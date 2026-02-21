package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

const (
	labelProject    = "com.docker.compose.project"
	labelWorkingDir = "com.docker.compose.project.working_dir"
	labelService    = "com.docker.compose.service"
)

// Client wraps the Docker SDK client for stack/container operations.
type Client struct {
	cli *client.Client
}

// NewClient creates a Docker client connected to the local socket.
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Close closes the underlying Docker client.
func (c *Client) Close() error {
	return c.cli.Close()
}

// Ping checks if the Docker daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

// Stack represents a Docker Compose stack discovered from container labels.
type Stack struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	ServiceCount int    `json:"service_count"`
	RunningCount int    `json:"running_count"`
	WorkingDir   string `json:"working_dir"`
	Registered   bool   `json:"registered"`
}

// StackDetail includes the container list for a stack.
type StackDetail struct {
	Name       string           `json:"name"`
	Status     string           `json:"status"`
	WorkingDir string           `json:"working_dir"`
	Containers []ContainerInfo  `json:"containers"`
}

// ContainerInfo represents a container within a compose stack.
type ContainerInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Service   string `json:"service"`
	Image     string `json:"image"`
	Status    string `json:"status"`
	State     string `json:"state"`
	CreatedAt int64  `json:"created_at"`
}

// ListStacks discovers compose stacks by grouping containers by project label.
func (c *Client) ListStacks(ctx context.Context) ([]Stack, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	type stackData struct {
		workingDir   string
		serviceCount int
		runningCount int
		services     map[string]bool
	}

	stacks := make(map[string]*stackData)

	for _, ctr := range containers {
		project := ctr.Labels[labelProject]
		if project == "" {
			continue
		}

		sd, ok := stacks[project]
		if !ok {
			sd = &stackData{
				workingDir: ctr.Labels[labelWorkingDir],
				services:   make(map[string]bool),
			}
			stacks[project] = sd
		}

		service := ctr.Labels[labelService]
		if service != "" && !sd.services[service] {
			sd.services[service] = true
			sd.serviceCount++
		}

		if ctr.State == "running" {
			sd.runningCount++
		}
	}

	result := make([]Stack, 0, len(stacks))
	for name, sd := range stacks {
		result = append(result, Stack{
			Name:         name,
			Status:       stackStatus(sd.serviceCount, sd.runningCount),
			ServiceCount: sd.serviceCount,
			RunningCount: sd.runningCount,
			WorkingDir:   sd.workingDir,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// GetStack returns detailed info for a named stack including its containers.
func (c *Client) GetStack(ctx context.Context, name string) (*StackDetail, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	detail := &StackDetail{
		Name:       name,
		Containers: []ContainerInfo{},
	}
	runningCount := 0

	for _, ctr := range containers {
		if ctr.Labels[labelProject] != name {
			continue
		}

		if detail.WorkingDir == "" {
			detail.WorkingDir = ctr.Labels[labelWorkingDir]
		}

		containerName := strings.TrimPrefix(ctr.Names[0], "/")

		detail.Containers = append(detail.Containers, ContainerInfo{
			ID:        ctr.ID[:12],
			Name:      containerName,
			Service:   ctr.Labels[labelService],
			Image:     ctr.Image,
			Status:    ctr.Status,
			State:     ctr.State,
			CreatedAt: ctr.Created,
		})

		if ctr.State == "running" {
			runningCount++
		}
	}

	if len(detail.Containers) == 0 {
		return nil, fmt.Errorf("stack %q not found", name)
	}

	detail.Status = stackStatus(len(detail.Containers), runningCount)

	sort.Slice(detail.Containers, func(i, j int) bool {
		return detail.Containers[i].Service < detail.Containers[j].Service
	})

	return detail, nil
}

// ComposeFile reads the compose file for a named stack.
type ComposeFile struct {
	Content string `json:"content"`
	Path    string `json:"path"`
}

// GetComposeFile reads the compose file from the stack's working directory.
func (c *Client) GetComposeFile(ctx context.Context, stackName string) (*ComposeFile, error) {
	detail, err := c.GetStack(ctx, stackName)
	if err != nil {
		return nil, err
	}

	if detail.WorkingDir == "" {
		return nil, fmt.Errorf("no working directory found for stack %q", stackName)
	}

	// Try common compose file names
	candidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, name := range candidates {
		path := filepath.Join(detail.WorkingDir, name)
		data, err := os.ReadFile(path)
		if err == nil {
			return &ComposeFile{
				Content: string(data),
				Path:    path,
			}, nil
		}
	}

	return nil, fmt.Errorf("compose file not found in %s", detail.WorkingDir)
}

// GetComposeFileFromDir reads the compose file from a given directory
// without requiring a running stack.
func (c *Client) GetComposeFileFromDir(workingDir string) (*ComposeFile, error) {
	if workingDir == "" {
		return nil, fmt.Errorf("working directory is empty")
	}

	candidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, name := range candidates {
		path := filepath.Join(workingDir, name)
		data, err := os.ReadFile(path)
		if err == nil {
			return &ComposeFile{
				Content: string(data),
				Path:    path,
			}, nil
		}
	}

	return nil, fmt.Errorf("compose file not found in %s", workingDir)
}

// ContainerLogs returns the last N lines of logs for a container.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"`
	Message   string `json:"message"`
}

// GetContainerLogs retrieves logs from a container.
func (c *Client) GetContainerLogs(ctx context.Context, containerID string, lines int, since string) ([]LogEntry, string, string, error) {
	if lines <= 0 {
		lines = 100
	}
	if lines > 1000 {
		lines = 1000
	}

	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       fmt.Sprintf("%d", lines),
	}
	if since != "" {
		opts.Since = since
	}

	reader, err := c.cli.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return nil, "", "", fmt.Errorf("get logs: %w", err)
	}
	defer reader.Close()

	// Docker log output has an 8-byte header per frame:
	// [stream_type(1)][0(3)][size(4)][payload]
	// stream_type: 1=stdout, 2=stderr
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", "", fmt.Errorf("read logs: %w", err)
	}

	// Resolve container name and ID for response
	inspect, inspErr := c.cli.ContainerInspect(ctx, containerID)
	var cName, cID string
	if inspErr == nil {
		cName = strings.TrimPrefix(inspect.Name, "/")
		cID = inspect.ID[:12]
	}

	entries := parseLogFrames(raw)
	return entries, cID, cName, nil
}

func parseLogFrames(raw []byte) []LogEntry {
	var entries []LogEntry
	pos := 0

	for pos+8 <= len(raw) {
		streamType := raw[pos]
		frameSize := int(raw[pos+4])<<24 | int(raw[pos+5])<<16 | int(raw[pos+6])<<8 | int(raw[pos+7])
		pos += 8

		if pos+frameSize > len(raw) {
			break
		}

		line := string(raw[pos : pos+frameSize])
		pos += frameSize

		line = strings.TrimRight(line, "\n")

		stream := "stdout"
		if streamType == 2 {
			stream = "stderr"
		}

		// Timestamp is the first space-separated token
		var timestamp, message string
		if idx := strings.IndexByte(line, ' '); idx > 0 {
			timestamp = line[:idx]
			message = line[idx+1:]
		} else {
			message = line
		}

		entries = append(entries, LogEntry{
			Timestamp: timestamp,
			Stream:    stream,
			Message:   message,
		})
	}

	return entries
}

// StartContainer starts a stopped container.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer stops a running container.
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerStop(ctx, containerID, container.StopOptions{})
}

// RestartContainer restarts a container.
func (c *Client) RestartContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerRestart(ctx, containerID, container.StopOptions{})
}

// Events returns channels for Docker container events.
func (c *Client) Events(ctx context.Context) (<-chan events.Message, <-chan error) {
	return c.cli.Events(ctx, events.ListOptions{
		Filters: filters.NewArgs(filters.Arg("type", "container")),
	})
}

// StreamContainerLogs returns a streaming reader for a container's logs.
// The caller is responsible for closing the returned reader.
func (c *Client) StreamContainerLogs(ctx context.Context, containerID string, tail string) (io.ReadCloser, error) {
	if tail == "" {
		tail = "50"
	}

	return c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Follow:     true,
		Tail:       tail,
	})
}

// --- Docker resource management ---

// DiskUsage returns an aggregated summary of Docker resource usage.
func (c *Client) DiskUsage(ctx context.Context) (*DiskUsageSummary, error) {
	du, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		return nil, fmt.Errorf("disk usage: %w", err)
	}

	// Build container image ID set for in-use detection.
	usedImageIDs := make(map[string]bool)
	for _, ctr := range du.Containers {
		usedImageIDs[ctr.ImageID] = true
	}

	var imgSummary ResourceSummary
	for _, img := range du.Images {
		imgSummary.TotalCount++
		imgSummary.TotalSize += img.Size
		if usedImageIDs[img.ID] {
			imgSummary.InUseCount++
		} else {
			imgSummary.ReclaimableSize += img.Size
		}
	}

	var volSummary ResourceSummary
	for _, vol := range du.Volumes {
		volSummary.TotalCount++
		var sz int64
		if vol.UsageData != nil && vol.UsageData.Size > 0 {
			sz = vol.UsageData.Size
		}
		volSummary.TotalSize += sz
		if vol.UsageData != nil && vol.UsageData.RefCount > 0 {
			volSummary.InUseCount++
		} else {
			volSummary.ReclaimableSize += sz
		}
	}

	// Networks: fetch separately since DiskUsage doesn't include them.
	nets, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("network list: %w", err)
	}
	var netSummary NetworkSummary
	for _, n := range nets {
		netSummary.TotalCount++
		if len(n.Containers) > 0 {
			netSummary.InUseCount++
		} else if !isBuiltinNetwork(n.Name) {
			netSummary.ReclaimableCount++
		}
	}

	var cacheSummary CacheSummary
	for _, bc := range du.BuildCache {
		cacheSummary.TotalSize += bc.Size
	}

	return &DiskUsageSummary{
		Images:     imgSummary,
		Volumes:    volSummary,
		Networks:   netSummary,
		BuildCache: cacheSummary,
	}, nil
}

// ListImages returns all Docker images with container usage info.
func (c *Client) ListImages(ctx context.Context) ([]ImageInfo, error) {
	images, err := c.cli.ImageList(ctx, image.ListOptions{All: false})
	if err != nil {
		return nil, fmt.Errorf("image list: %w", err)
	}

	// Build a map of image ID → container names.
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("container list: %w", err)
	}
	imageContainers := make(map[string][]string)
	for _, ctr := range containers {
		name := strings.TrimPrefix(ctr.Names[0], "/")
		imageContainers[ctr.ImageID] = append(imageContainers[ctr.ImageID], name)
	}

	result := make([]ImageInfo, 0, len(images))
	for _, img := range images {
		tags := img.RepoTags
		if tags == nil {
			tags = []string{}
		}
		ctrs := imageContainers[img.ID]
		if ctrs == nil {
			ctrs = []string{}
		}
		result = append(result, ImageInfo{
			ID:         img.ID,
			Tags:       tags,
			Size:       img.Size,
			Created:    img.Created,
			InUse:      len(ctrs) > 0,
			Containers: ctrs,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Created > result[j].Created
	})

	return result, nil
}

// RemoveImage removes a Docker image by ID.
func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	_, err := c.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force, PruneChildren: true})
	if err != nil {
		return fmt.Errorf("remove image: %w", err)
	}
	return nil
}

// PruneImages removes unused images. If dryRun is true, returns what would be removed.
func (c *Client) PruneImages(ctx context.Context, dryRun bool) (*PruneResult, error) {
	if dryRun {
		return c.pruneImagesDryRun(ctx)
	}

	report, err := c.cli.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "false")))
	if err != nil {
		return nil, fmt.Errorf("prune images: %w", err)
	}

	items := make([]string, 0, len(report.ImagesDeleted))
	for _, d := range report.ImagesDeleted {
		if d.Deleted != "" {
			items = append(items, d.Deleted)
		}
	}

	return &PruneResult{
		DryRun:         false,
		ItemsToRemove:  items,
		Count:          len(items),
		SpaceReclaimed: int64(report.SpaceReclaimed),
	}, nil
}

func (c *Client) pruneImagesDryRun(ctx context.Context) (*PruneResult, error) {
	images, err := c.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	var items []string
	var reclaimable int64
	for _, img := range images {
		if !img.InUse {
			label := img.ID[:12]
			if len(img.Tags) > 0 && img.Tags[0] != "<none>:<none>" {
				label = img.Tags[0]
			}
			items = append(items, label)
			reclaimable += img.Size
		}
	}
	if items == nil {
		items = []string{}
	}

	return &PruneResult{
		DryRun:         true,
		ItemsToRemove:  items,
		Count:          len(items),
		SpaceReclaimed: reclaimable,
	}, nil
}

// ListVolumes returns all Docker volumes with container usage info.
func (c *Client) ListVolumes(ctx context.Context) ([]VolumeInfo, error) {
	resp, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("volume list: %w", err)
	}

	// Get disk usage data for volume sizes.
	du, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{
		Types: []types.DiskUsageObject{types.VolumeObject},
	})
	if err != nil {
		return nil, fmt.Errorf("disk usage for volumes: %w", err)
	}
	volUsage := make(map[string]*volume.UsageData, len(du.Volumes))
	for _, v := range du.Volumes {
		if v.UsageData != nil {
			volUsage[v.Name] = v.UsageData
		}
	}

	// Build volume → container name map via container list.
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("container list: %w", err)
	}
	volContainers := make(map[string][]string)
	for _, ctr := range containers {
		name := strings.TrimPrefix(ctr.Names[0], "/")
		for _, m := range ctr.Mounts {
			if m.Type == "volume" {
				volContainers[m.Name] = append(volContainers[m.Name], name)
			}
		}
	}

	result := make([]VolumeInfo, 0, len(resp.Volumes))
	for _, vol := range resp.Volumes {
		ctrs := volContainers[vol.Name]
		if ctrs == nil {
			ctrs = []string{}
		}
		var sz int64
		if usage, ok := volUsage[vol.Name]; ok && usage.Size > 0 {
			sz = usage.Size
		}
		result = append(result, VolumeInfo{
			Name:       vol.Name,
			Driver:     vol.Driver,
			Size:       sz,
			Created:    vol.CreatedAt,
			InUse:      len(ctrs) > 0,
			Containers: ctrs,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// RemoveVolume removes a Docker volume by name.
func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	if err := c.cli.VolumeRemove(ctx, name, force); err != nil {
		return fmt.Errorf("remove volume: %w", err)
	}
	return nil
}

// PruneVolumes removes unused volumes. If dryRun is true, returns what would be removed.
func (c *Client) PruneVolumes(ctx context.Context, dryRun bool) (*PruneResult, error) {
	if dryRun {
		return c.pruneVolumesDryRun(ctx)
	}

	report, err := c.cli.VolumesPrune(ctx, filters.NewArgs())
	if err != nil {
		return nil, fmt.Errorf("prune volumes: %w", err)
	}

	items := report.VolumesDeleted
	if items == nil {
		items = []string{}
	}

	return &PruneResult{
		DryRun:         false,
		ItemsToRemove:  items,
		Count:          len(items),
		SpaceReclaimed: int64(report.SpaceReclaimed),
	}, nil
}

func (c *Client) pruneVolumesDryRun(ctx context.Context) (*PruneResult, error) {
	volumes, err := c.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}

	var items []string
	var reclaimable int64
	for _, vol := range volumes {
		if !vol.InUse {
			items = append(items, vol.Name)
			reclaimable += vol.Size
		}
	}
	if items == nil {
		items = []string{}
	}

	return &PruneResult{
		DryRun:         true,
		ItemsToRemove:  items,
		Count:          len(items),
		SpaceReclaimed: reclaimable,
	}, nil
}

// ListNetworks returns all Docker networks with container usage info.
func (c *Client) ListNetworks(ctx context.Context) ([]NetworkInfo, error) {
	nets, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("network list: %w", err)
	}

	result := make([]NetworkInfo, 0, len(nets))
	for _, n := range nets {
		ctrs := make([]string, 0, len(n.Containers))
		for _, ep := range n.Containers {
			ctrs = append(ctrs, ep.Name)
		}

		result = append(result, NetworkInfo{
			ID:         n.ID,
			Name:       n.Name,
			Driver:     n.Driver,
			Scope:      n.Scope,
			Internal:   n.Internal,
			InUse:      len(n.Containers) > 0,
			Containers: ctrs,
			Builtin:    isBuiltinNetwork(n.Name),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// RemoveNetwork removes a Docker network by ID.
func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	if err := c.cli.NetworkRemove(ctx, id); err != nil {
		return fmt.Errorf("remove network: %w", err)
	}
	return nil
}

// PruneNetworks removes unused networks. If dryRun is true, returns what would be removed.
func (c *Client) PruneNetworks(ctx context.Context, dryRun bool) (*PruneResult, error) {
	if dryRun {
		return c.pruneNetworksDryRun(ctx)
	}

	report, err := c.cli.NetworksPrune(ctx, filters.NewArgs())
	if err != nil {
		return nil, fmt.Errorf("prune networks: %w", err)
	}

	items := report.NetworksDeleted
	if items == nil {
		items = []string{}
	}

	return &PruneResult{
		DryRun:         false,
		ItemsToRemove:  items,
		Count:          len(items),
		SpaceReclaimed: 0,
	}, nil
}

func (c *Client) pruneNetworksDryRun(ctx context.Context) (*PruneResult, error) {
	networks, err := c.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}

	var items []string
	for _, n := range networks {
		if !n.InUse && !n.Builtin {
			items = append(items, n.Name)
		}
	}
	if items == nil {
		items = []string{}
	}

	return &PruneResult{
		DryRun:         true,
		ItemsToRemove:  items,
		Count:          len(items),
		SpaceReclaimed: 0,
	}, nil
}

// PruneBuildCache clears the Docker build cache. If dryRun is true, returns what would be removed.
func (c *Client) PruneBuildCache(ctx context.Context, dryRun bool) (*PruneResult, error) {
	if dryRun {
		return c.pruneBuildCacheDryRun(ctx)
	}

	report, err := c.cli.BuildCachePrune(ctx, build.CachePruneOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("prune build cache: %w", err)
	}

	items := []string{}
	if report != nil {
		if report.CachesDeleted == nil {
			report.CachesDeleted = []string{}
		}
		items = report.CachesDeleted
		return &PruneResult{
			DryRun:         false,
			ItemsToRemove:  items,
			Count:          len(items),
			SpaceReclaimed: int64(report.SpaceReclaimed),
		}, nil
	}

	return &PruneResult{
		DryRun:         false,
		ItemsToRemove:  items,
		Count:          0,
		SpaceReclaimed: 0,
	}, nil
}

func (c *Client) pruneBuildCacheDryRun(ctx context.Context) (*PruneResult, error) {
	du, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{
		Types: []types.DiskUsageObject{types.BuildCacheObject},
	})
	if err != nil {
		return nil, fmt.Errorf("disk usage for build cache: %w", err)
	}

	var items []string
	var totalSize int64
	for _, bc := range du.BuildCache {
		if !bc.InUse {
			items = append(items, bc.Description)
			totalSize += bc.Size
		}
	}
	if items == nil {
		items = []string{}
	}

	return &PruneResult{
		DryRun:         true,
		ItemsToRemove:  items,
		Count:          len(items),
		SpaceReclaimed: totalSize,
	}, nil
}

func isBuiltinNetwork(name string) bool {
	return name == "bridge" || name == "host" || name == "none"
}

func stackStatus(serviceCount, runningCount int) string {
	switch {
	case runningCount == 0:
		return "stopped"
	case runningCount == serviceCount:
		return "running"
	default:
		return "partial"
	}
}
