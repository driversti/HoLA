package ws

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/driversti/hola/internal/docker"
	"github.com/driversti/hola/internal/metrics"
)

// streamMetrics sends system metrics at a regular interval until the context is cancelled.
func streamMetrics(ctx context.Context, c *client, intervalSeconds int) {
	if intervalSeconds < 1 {
		intervalSeconds = 3
	}
	if intervalSeconds > 30 {
		intervalSeconds = 30
	}

	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	// Send an initial snapshot immediately.
	sendMetrics(ctx, c)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendMetrics(ctx, c)
		}
	}
}

func sendMetrics(ctx context.Context, c *client) {
	m, err := metrics.Collect(ctx)
	if err != nil {
		slog.Warn("metrics collect failed", "error", err)
		return
	}

	if err := c.send(ctx, Message{
		Type:    "metrics",
		Payload: mustMarshal(m),
	}); err != nil {
		slog.Debug("metrics send failed", "error", err)
	}
}

// LogLine is the payload for individual log lines sent over WebSocket.
type LogLine struct {
	ContainerID string `json:"container_id"`
	Timestamp   string `json:"timestamp"`
	Stream      string `json:"stream"`
	Message     string `json:"message"`
}

// streamLogs follows container logs and sends each line over the WebSocket.
func streamLogs(ctx context.Context, c *client, dockerClient *docker.Client, containerID string) {
	reader, err := dockerClient.StreamContainerLogs(ctx, containerID, "50")
	if err != nil {
		slog.Warn("log stream open failed", "container", containerID, "error", err)
		_ = c.send(ctx, Message{
			Type:    "error",
			Payload: mustMarshal(ErrorPayload{Error: "failed to open log stream: " + err.Error(), Code: "LOG_STREAM_ERROR"}),
		})
		return
	}
	defer reader.Close()

	// Docker logs use an 8-byte header per frame:
	// [stream_type(1)][0(3)][size(4)][payload]
	header := make([]byte, 8)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, err := io.ReadFull(reader, header)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled — clean shutdown.
			}
			slog.Debug("log stream ended", "container", containerID, "error", err)
			return
		}

		streamType := header[0]
		frameSize := int(binary.BigEndian.Uint32(header[4:8]))

		if frameSize <= 0 || frameSize > 1<<20 { // Skip frames > 1MB.
			continue
		}

		payload := make([]byte, frameSize)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Debug("log frame read failed", "container", containerID, "error", err)
			return
		}

		line := strings.TrimRight(string(payload), "\n")
		stream := "stdout"
		if streamType == 2 {
			stream = "stderr"
		}

		var timestamp, message string
		if idx := strings.IndexByte(line, ' '); idx > 0 {
			timestamp = line[:idx]
			message = line[idx+1:]
		} else {
			message = line
		}

		logLine := LogLine{
			ContainerID: containerID,
			Timestamp:   timestamp,
			Stream:      stream,
			Message:     message,
		}

		if err := c.send(ctx, Message{
			Type:    "log_line",
			Payload: mustMarshal(logLine),
		}); err != nil {
			slog.Debug("log send failed", "container", containerID, "error", err)
			return
		}
	}
}

// ContainerStatsPayload is the payload for per-container resource stats.
type ContainerStatsPayload struct {
	ContainerID   string  `json:"container_id"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemUsedBytes  uint64  `json:"mem_used_bytes"`
	MemLimitBytes uint64  `json:"mem_limit_bytes"`
	MemPercent    float64 `json:"mem_percent"`
}

// streamContainerStats reads Docker container stats and sends CPU/memory snapshots at a regular interval.
func streamContainerStats(ctx context.Context, c *client, dockerClient *docker.Client, containerID string, intervalSeconds int) {
	if intervalSeconds < 1 {
		intervalSeconds = 3
	}
	if intervalSeconds > 30 {
		intervalSeconds = 30
	}

	reader, err := dockerClient.ContainerStats(ctx, containerID)
	if err != nil {
		slog.Warn("container stats open failed", "container", containerID, "error", err)
		_ = c.send(ctx, Message{
			Type:    "error",
			Payload: mustMarshal(ErrorPayload{Error: "failed to open stats stream: " + err.Error(), Code: "STATS_STREAM_ERROR"}),
		})
		return
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	var latest *ContainerStatsPayload

	// Read stats in a separate goroutine to avoid blocking the ticker.
	statsCh := make(chan ContainerStatsPayload, 1)
	go func() {
		for {
			var stats container.StatsResponse
			if err := decoder.Decode(&stats); err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Debug("container stats decode failed", "container", containerID, "error", err)
				return
			}

			cpuPercent := calculateCPUPercent(&stats)
			memUsed := stats.MemoryStats.Usage
			if cache, ok := stats.MemoryStats.Stats["cache"]; ok {
				memUsed -= cache
			}
			memLimit := stats.MemoryStats.Limit
			var memPercent float64
			if memLimit > 0 {
				memPercent = float64(memUsed) / float64(memLimit) * 100.0
			}

			payload := ContainerStatsPayload{
				ContainerID:   containerID,
				CPUPercent:    cpuPercent,
				MemUsedBytes:  memUsed,
				MemLimitBytes: memLimit,
				MemPercent:    memPercent,
			}

			// Non-blocking send — drop old value if not consumed yet.
			select {
			case statsCh <- payload:
			default:
				<-statsCh
				statsCh <- payload
			}
		}
	}()

	// Send initial snapshot as soon as available.
	select {
	case <-ctx.Done():
		return
	case p := <-statsCh:
		latest = &p
		if err := c.send(ctx, Message{
			Type:    "container_stats",
			Payload: mustMarshal(p),
		}); err != nil {
			slog.Debug("container stats send failed", "container", containerID, "error", err)
			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case p := <-statsCh:
			latest = &p
		case <-ticker.C:
			if latest == nil {
				continue
			}
			if err := c.send(ctx, Message{
				Type:    "container_stats",
				Payload: mustMarshal(*latest),
			}); err != nil {
				slog.Debug("container stats send failed", "container", containerID, "error", err)
				return
			}
		}
	}
}

func calculateCPUPercent(stats *container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta <= 0 || cpuDelta < 0 {
		return 0.0
	}
	numCPUs := float64(stats.CPUStats.OnlineCPUs)
	if numCPUs == 0 {
		numCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if numCPUs == 0 {
		numCPUs = 1
	}
	return (cpuDelta / systemDelta) * numCPUs * 100.0
}
