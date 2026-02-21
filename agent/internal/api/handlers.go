package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"errors"
	"time"

	"github.com/driversti/hola/internal/api/respond"
	"github.com/driversti/hola/internal/docker"
	"github.com/driversti/hola/internal/metrics"
	"github.com/driversti/hola/internal/registry"
	"github.com/driversti/hola/internal/update"
	"gopkg.in/yaml.v3"
)

type handlers struct {
	version  string
	docker   *docker.Client
	registry *registry.Store
	updater  *update.Updater
}

// --- System endpoints ---

func (h *handlers) health(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) agentInfo(w http.ResponseWriter, _ *http.Request) {
	hostname, _ := os.Hostname()

	info := struct {
		Version       string `json:"version"`
		Hostname      string `json:"hostname"`
		OS            string `json:"os"`
		Arch          string `json:"arch"`
		DockerVersion string `json:"docker_version"`
	}{
		Version:       h.version,
		Hostname:      hostname,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		DockerVersion: dockerVersion(),
	}

	respond.JSON(w, http.StatusOK, info)
}

func (h *handlers) systemMetrics(w http.ResponseWriter, r *http.Request) {
	m, err := metrics.Collect(r.Context())
	if err != nil {
		slog.Error("failed to collect metrics", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to collect system metrics", "METRICS_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, m)
}

// --- Update endpoints ---

func (h *handlers) checkUpdate(w http.ResponseWriter, r *http.Request) {
	check, err := h.updater.CheckLatest(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, update.ErrNoReleases):
			respond.Error(w, http.StatusNotFound, "no releases available", "NO_RELEASES")
		case errors.Is(err, update.ErrRateLimited):
			respond.Error(w, http.StatusTooManyRequests, "GitHub API rate limit exceeded, try again later", "RATE_LIMITED")
		case errors.Is(err, update.ErrAssetNotFound):
			respond.Error(w, http.StatusNotFound,
				fmt.Sprintf("no binary available for %s/%s", runtime.GOOS, runtime.GOARCH),
				"PLATFORM_NOT_AVAILABLE")
		default:
			slog.Error("failed to check for updates", "error", err)
			respond.Error(w, http.StatusBadGateway, "failed to check for updates", "GITHUB_ERROR")
		}
		return
	}
	respond.JSON(w, http.StatusOK, check)
}

func (h *handlers) applyUpdate(w http.ResponseWriter, r *http.Request) {
	err := h.updater.Apply(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, update.ErrAlreadyLatest):
			respond.JSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": "already running the latest version",
			})
		case errors.Is(err, update.ErrNoReleases):
			respond.Error(w, http.StatusNotFound, "no releases available", "NO_RELEASES")
		case errors.Is(err, update.ErrRateLimited):
			respond.Error(w, http.StatusTooManyRequests, "GitHub API rate limit exceeded", "RATE_LIMITED")
		case errors.Is(err, update.ErrAssetNotFound):
			respond.Error(w, http.StatusNotFound,
				fmt.Sprintf("no binary available for %s/%s", runtime.GOOS, runtime.GOARCH),
				"PLATFORM_NOT_AVAILABLE")
		case errors.Is(err, update.ErrChecksumsNotFound):
			respond.Error(w, http.StatusUnprocessableEntity,
				"release is missing checksums.txt, refusing to update", "CHECKSUMS_MISSING")
		case errors.Is(err, update.ErrChecksumMismatch):
			respond.Error(w, http.StatusUnprocessableEntity,
				"downloaded binary failed checksum verification", "CHECKSUM_MISMATCH")
		default:
			slog.Error("failed to apply update", "error", err)
			respond.Error(w, http.StatusInternalServerError, "update failed: "+err.Error(), "UPDATE_FAILED")
		}
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "update applied successfully, agent is restarting",
	})

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		slog.Info("agent updated, exiting for restart")
		os.Exit(0)
	}()
}

// --- Stack read endpoints ---

func (h *handlers) listStacks(w http.ResponseWriter, r *http.Request) {
	stacks, err := h.docker.ListStacks(r.Context())
	if err != nil {
		slog.Error("failed to list stacks", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to list stacks", "DOCKER_ERROR")
		return
	}

	// Merge with registry: enrich discovered stacks + add downed registered stacks.
	byName := make(map[string]int, len(stacks))
	for i := range stacks {
		byName[stacks[i].Name] = i
	}

	for _, rs := range h.registry.All() {
		if idx, ok := byName[rs.Name]; ok {
			stacks[idx].Registered = true
		} else {
			stacks = append(stacks, docker.Stack{
				Name:       rs.Name,
				Status:     "down",
				WorkingDir: rs.WorkingDir,
				Registered: true,
			})
		}
	}

	sort.Slice(stacks, func(i, j int) bool {
		return stacks[i].Name < stacks[j].Name
	})

	respond.JSON(w, http.StatusOK, map[string]any{"stacks": stacks})
}

func (h *handlers) getStack(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	detail, err := h.docker.GetStack(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// Fall back to registry for downed registered stacks.
			if rs := h.registry.Get(name); rs != nil {
				respond.JSON(w, http.StatusOK, docker.StackDetail{
					Name:       rs.Name,
					Status:     "down",
					WorkingDir: rs.WorkingDir,
					Containers: []docker.ContainerInfo{},
				})
				return
			}
			respond.Error(w, http.StatusNotFound, err.Error(), "STACK_NOT_FOUND")
			return
		}
		slog.Error("failed to get stack", "name", name, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to get stack", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, detail)
}

func (h *handlers) getComposeFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	cf, err := h.docker.GetComposeFile(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// Fall back to registry for downed registered stacks.
			if rs := h.registry.Get(name); rs != nil {
				cf2, err2 := h.docker.GetComposeFileFromDir(rs.WorkingDir)
				if err2 == nil {
					respond.JSON(w, http.StatusOK, cf2)
					return
				}
			}
			respond.Error(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
			return
		}
		slog.Error("failed to get compose file", "name", name, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to read compose file", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, cf)
}

func (h *handlers) updateComposeFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		respond.Error(w, http.StatusBadRequest, "content must not be empty", "BAD_REQUEST")
		return
	}

	// Validate YAML syntax.
	var parsed any
	if err := yaml.Unmarshal([]byte(body.Content), &parsed); err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("invalid YAML syntax: %s", err),
		})
		return
	}

	// Resolve compose file path.
	composePath := h.resolveComposeFilePath(r.Context(), name)
	if composePath == "" {
		respond.Error(w, http.StatusNotFound, fmt.Sprintf("compose file not found for stack %q", name), "NOT_FOUND")
		return
	}

	dir := filepath.Dir(composePath)

	// Write content to a temp file in the same directory for docker compose validation.
	tmpFile, err := os.CreateTemp(dir, ".compose-validate-*.yml")
	if err != nil {
		slog.Error("failed to create temp file", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to create temp file", "IO_ERROR")
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(body.Content); err != nil {
		tmpFile.Close()
		slog.Error("failed to write temp file", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to write temp file", "IO_ERROR")
		return
	}
	tmpFile.Close()

	// Validate with docker compose.
	cmd := exec.CommandContext(r.Context(), "docker", "compose", "-f", tmpPath, "config", "-q")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		respond.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("docker compose validation failed: %s", detail),
		})
		return
	}

	// Preserve original file permissions.
	fileInfo, err := os.Stat(composePath)
	if err != nil {
		slog.Error("failed to stat compose file", "path", composePath, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to read compose file info", "IO_ERROR")
		return
	}
	perm := fileInfo.Mode().Perm()

	// Create .bak backup of original.
	originalData, err := os.ReadFile(composePath)
	if err != nil {
		slog.Error("failed to read original compose file", "path", composePath, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to read original compose file", "IO_ERROR")
		return
	}
	if err := os.WriteFile(composePath+".bak", originalData, perm); err != nil {
		slog.Error("failed to create backup", "path", composePath+".bak", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to create backup", "IO_ERROR")
		return
	}

	// Write new content to the compose file.
	if err := os.WriteFile(composePath, []byte(body.Content), perm); err != nil {
		slog.Error("failed to write compose file", "path", composePath, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to write compose file", "IO_ERROR")
		return
	}

	slog.Info("compose file updated", "stack", name, "path", composePath)
	respond.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Compose file for stack '%s' updated successfully", name),
	})
}

// resolveComposeFilePath tries to find the compose file path for a stack.
// It first checks the running stack via docker, then falls back to the registry.
func (h *handlers) resolveComposeFilePath(ctx context.Context, stackName string) string {
	cf, err := h.docker.GetComposeFile(ctx, stackName)
	if err == nil && cf.Path != "" {
		return cf.Path
	}

	// Fall back to registry for downed/registered stacks.
	if rs := h.registry.Get(stackName); rs != nil {
		if path := findComposeFile(rs.WorkingDir); path != "" {
			return path
		}
	}

	return ""
}

// --- Container logs ---

func (h *handlers) containerLogs(w http.ResponseWriter, r *http.Request) {
	containerID := r.PathValue("id")

	lines := 100
	if v := r.URL.Query().Get("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lines = n
		}
	}

	since := r.URL.Query().Get("since")

	entries, cID, cName, err := h.docker.GetContainerLogs(r.Context(), containerID, lines, since)
	if err != nil {
		slog.Error("failed to get container logs", "container", containerID, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to get container logs", "DOCKER_ERROR")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"container_id":   cID,
		"container_name": cName,
		"lines":          entries,
	})
}

// --- Stack write endpoints ---

func (h *handlers) stackAction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Extract action from the last path segment
	parts := strings.Split(r.URL.Path, "/")
	action := parts[len(parts)-1]

	// Resolve working directory from the stack (or registry for downed stacks).
	detail, err := h.docker.GetStack(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			if rs := h.registry.Get(name); rs != nil {
				detail = &docker.StackDetail{
					Name:       rs.Name,
					Status:     "down",
					WorkingDir: rs.WorkingDir,
				}
			} else {
				respond.Error(w, http.StatusNotFound, err.Error(), "STACK_NOT_FOUND")
				return
			}
		} else {
			slog.Error("failed to get stack for action", "name", name, "error", err)
			respond.Error(w, http.StatusInternalServerError, "failed to get stack", "DOCKER_ERROR")
			return
		}
	}

	var args []string
	switch action {
	case "start":
		args = []string{"compose", "up", "-d"}
	case "stop":
		args = []string{"compose", "stop"}
	case "restart":
		args = []string{"compose", "restart"}
	case "down":
		args = []string{"compose", "down"}
	case "pull":
		args = []string{"compose", "pull"}
	default:
		respond.Error(w, http.StatusBadRequest, fmt.Sprintf("unknown action: %s", action), "BAD_REQUEST")
		return
	}

	// Find compose file in working dir
	composeFile := findComposeFile(detail.WorkingDir)

	if composeFile != "" {
		args = append(args[:1], append([]string{"-f", composeFile}, args[1:]...)...)
	}

	cmd := exec.CommandContext(r.Context(), "docker", args...)
	cmd.Dir = detail.WorkingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("stack action failed", "name", name, "action", action, "error", err, "output", string(output))
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		respond.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to %s stack: %s", action, detail),
		})
		return
	}

	slog.Info("stack action succeeded", "name", name, "action", action)
	respond.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Stack '%s' %s successfully", name, actionPastTense(action)),
	})
}

// --- Container write endpoints ---

func (h *handlers) containerAction(w http.ResponseWriter, r *http.Request) {
	containerID := r.PathValue("id")

	parts := strings.Split(r.URL.Path, "/")
	action := parts[len(parts)-1]

	var err error
	switch action {
	case "start":
		err = h.docker.StartContainer(r.Context(), containerID)
	case "stop":
		err = h.docker.StopContainer(r.Context(), containerID)
	case "restart":
		err = h.docker.RestartContainer(r.Context(), containerID)
	default:
		respond.Error(w, http.StatusBadRequest, fmt.Sprintf("unknown action: %s", action), "BAD_REQUEST")
		return
	}

	if err != nil {
		slog.Error("container action failed", "container", containerID, "action", action, "error", err)
		respond.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to %s container: %s", action, err.Error()),
		})
		return
	}

	slog.Info("container action succeeded", "container", containerID, "action", action)
	respond.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Container %s %s successfully", containerID, actionPastTense(action)),
	})
}

// --- Filesystem browse ---

type fsEntry struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	IsDir          bool   `json:"is_dir"`
	HasComposeFile bool   `json:"has_compose_file"`
}

func (h *handlers) browsePath(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "/"
	}

	cleanPath := filepath.Clean(reqPath)
	if !filepath.IsAbs(cleanPath) {
		respond.Error(w, http.StatusBadRequest, "path must be absolute", "BAD_REQUEST")
		return
	}

	dirEntries, err := os.ReadDir(cleanPath)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, fmt.Sprintf("cannot read path: %s", err), "BAD_REQUEST")
		return
	}

	entries := make([]fsEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		name := de.Name()
		// Skip hidden entries (dot-prefixed).
		if strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(cleanPath, name)
		entry := fsEntry{
			Name:  name,
			Path:  fullPath,
			IsDir: de.IsDir(),
		}

		if de.IsDir() {
			entry.HasComposeFile = findComposeFile(fullPath) != ""
		}

		entries = append(entries, entry)
	}

	// Sort: directories first, then alphabetical.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	respond.JSON(w, http.StatusOK, map[string]any{
		"path":    cleanPath,
		"parent":  filepath.Dir(cleanPath),
		"entries": entries,
	})
}

// --- Stack registration ---

func (h *handlers) registerStack(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}

	cleanPath := filepath.Clean(body.Path)
	if !filepath.IsAbs(cleanPath) {
		respond.Error(w, http.StatusBadRequest, "path must be absolute", "BAD_REQUEST")
		return
	}

	composeFile := findComposeFile(cleanPath)
	if composeFile == "" {
		respond.Error(w, http.StatusBadRequest, "no compose file found in "+cleanPath, "NO_COMPOSE_FILE")
		return
	}

	name := filepath.Base(cleanPath)
	if err := h.registry.Register(name, cleanPath, composeFile); err != nil {
		slog.Error("failed to register stack", "name", name, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to register stack", "REGISTRY_ERROR")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"name":    name,
		"message": fmt.Sprintf("Stack '%s' registered", name),
	})
}

func (h *handlers) unregisterStack(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if h.registry.Get(name) == nil {
		respond.Error(w, http.StatusNotFound, fmt.Sprintf("stack %q is not registered", name), "NOT_FOUND")
		return
	}

	if err := h.registry.Unregister(name); err != nil {
		slog.Error("failed to unregister stack", "name", name, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to unregister stack", "REGISTRY_ERROR")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Stack '%s' unregistered", name),
	})
}

// --- Docker resources ---

func (h *handlers) dockerDiskUsage(w http.ResponseWriter, r *http.Request) {
	summary, err := h.docker.DiskUsage(r.Context())
	if err != nil {
		slog.Error("failed to get disk usage", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to get disk usage", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, summary)
}

func (h *handlers) listImages(w http.ResponseWriter, r *http.Request) {
	images, err := h.docker.ListImages(r.Context())
	if err != nil {
		slog.Error("failed to list images", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to list images", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"images": images})
}

func (h *handlers) removeImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true"

	if err := h.docker.RemoveImage(r.Context(), id, force); err != nil {
		slog.Error("failed to remove image", "id", id, "error", err)
		respond.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to remove image: %s", err),
		})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Image %s removed", id),
	})
}

func (h *handlers) pruneImages(w http.ResponseWriter, r *http.Request) {
	dryRun := r.URL.Query().Get("dry_run") == "true"

	result, err := h.docker.PruneImages(r.Context(), dryRun)
	if err != nil {
		slog.Error("failed to prune images", "dry_run", dryRun, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to prune images", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *handlers) listVolumes(w http.ResponseWriter, r *http.Request) {
	volumes, err := h.docker.ListVolumes(r.Context())
	if err != nil {
		slog.Error("failed to list volumes", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to list volumes", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"volumes": volumes})
}

func (h *handlers) removeVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	force := r.URL.Query().Get("force") == "true"

	if err := h.docker.RemoveVolume(r.Context(), name, force); err != nil {
		slog.Error("failed to remove volume", "name", name, "error", err)
		respond.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to remove volume: %s", err),
		})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Volume %s removed", name),
	})
}

func (h *handlers) pruneVolumes(w http.ResponseWriter, r *http.Request) {
	dryRun := r.URL.Query().Get("dry_run") == "true"

	result, err := h.docker.PruneVolumes(r.Context(), dryRun)
	if err != nil {
		slog.Error("failed to prune volumes", "dry_run", dryRun, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to prune volumes", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *handlers) listNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := h.docker.ListNetworks(r.Context())
	if err != nil {
		slog.Error("failed to list networks", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to list networks", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"networks": networks})
}

func (h *handlers) removeNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.docker.RemoveNetwork(r.Context(), id); err != nil {
		slog.Error("failed to remove network", "id", id, "error", err)
		respond.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("failed to remove network: %s", err),
		})
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Network %s removed", id),
	})
}

func (h *handlers) pruneNetworks(w http.ResponseWriter, r *http.Request) {
	dryRun := r.URL.Query().Get("dry_run") == "true"

	result, err := h.docker.PruneNetworks(r.Context(), dryRun)
	if err != nil {
		slog.Error("failed to prune networks", "dry_run", dryRun, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to prune networks", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *handlers) pruneBuildCache(w http.ResponseWriter, r *http.Request) {
	dryRun := r.URL.Query().Get("dry_run") == "true"

	result, err := h.docker.PruneBuildCache(r.Context(), dryRun)
	if err != nil {
		slog.Error("failed to prune build cache", "dry_run", dryRun, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to prune build cache", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

// --- Helpers ---

func dockerVersion() string {
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func findComposeFile(dir string) string {
	candidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func actionPastTense(action string) string {
	switch action {
	case "stop":
		return "stopped"
	case "start":
		return "started"
	case "restart":
		return "restarted"
	case "down":
		return "brought down"
	case "pull":
		return "pulled"
	default:
		return action + "ed"
	}
}
