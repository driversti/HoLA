package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
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
