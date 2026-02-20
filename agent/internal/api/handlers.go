package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/driversti/hola/internal/api/respond"
	"github.com/driversti/hola/internal/docker"
	"github.com/driversti/hola/internal/metrics"
)

type handlers struct {
	version string
	docker  *docker.Client
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

// --- Stack read endpoints ---

func (h *handlers) listStacks(w http.ResponseWriter, r *http.Request) {
	stacks, err := h.docker.ListStacks(r.Context())
	if err != nil {
		slog.Error("failed to list stacks", "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to list stacks", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"stacks": stacks})
}

func (h *handlers) getStack(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	detail, err := h.docker.GetStack(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
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
			respond.Error(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
			return
		}
		slog.Error("failed to get compose file", "name", name, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to read compose file", "DOCKER_ERROR")
		return
	}
	respond.JSON(w, http.StatusOK, cf)
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

	// Resolve working directory from the stack
	detail, err := h.docker.GetStack(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respond.Error(w, http.StatusNotFound, err.Error(), "STACK_NOT_FOUND")
			return
		}
		slog.Error("failed to get stack for action", "name", name, "error", err)
		respond.Error(w, http.StatusInternalServerError, "failed to get stack", "DOCKER_ERROR")
		return
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
