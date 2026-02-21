package ws

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"strings"
	"time"

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
