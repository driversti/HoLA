package ws

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"

	"github.com/driversti/hola/internal/docker"
)

// ContainerEvent is the payload sent to clients for Docker container events.
type ContainerEvent struct {
	Action        string `json:"action"`
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	Image         string `json:"image"`
	Stack         string `json:"stack"`
	Status        string `json:"status"`
	Time          int64  `json:"time"`
}

// subscriber wraps a client with its cancellation context.
type subscriber struct {
	client *client
	ctx    context.Context
}

// EventHub listens to Docker events and fans out container events to subscribers.
type EventHub struct {
	dockerClient *docker.Client
	mu           sync.RWMutex
	subscribers  map[*client]subscriber
}

// NewEventHub creates an EventHub.
func NewEventHub(dockerClient *docker.Client) *EventHub {
	return &EventHub{
		dockerClient: dockerClient,
		subscribers:  make(map[*client]subscriber),
	}
}

// Subscribe adds a client to receive container events.
func (h *EventHub) Subscribe(ctx context.Context, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribers[c] = subscriber{client: c, ctx: ctx}
}

// Unsubscribe removes a client from the event hub.
func (h *EventHub) Unsubscribe(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers, c)
}

// Run starts listening for Docker events. It blocks until ctx is cancelled.
// It automatically reconnects if the Docker events stream breaks.
func (h *EventHub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			h.listenOnce(ctx)
		}
		// Brief pause before reconnecting.
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (h *EventHub) listenOnce(ctx context.Context) {
	msgCh, errCh := h.dockerClient.Events(ctx)
	slog.Info("event hub connected to Docker events")

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil {
				slog.Warn("docker events stream error", "error", err)
			}
			return
		case msg := <-msgCh:
			if msg.Type != events.ContainerEventType {
				continue
			}
			h.broadcast(ctx, msg)
		}
	}
}

// allowedActions filters container events to only meaningful state changes.
var allowedActions = map[string]bool{
	"start":   true,
	"stop":    true,
	"die":     true,
	"kill":    true,
	"restart": true,
	"create":  true,
	"destroy": true,
}

func (h *EventHub) broadcast(ctx context.Context, msg events.Message) {
	action := string(msg.Action)
	if !allowedActions[action] {
		return
	}

	evt := ContainerEvent{
		Action:        action,
		ContainerID:   msg.Actor.ID[:min(12, len(msg.Actor.ID))],
		ContainerName: msg.Actor.Attributes["name"],
		Image:         msg.Actor.Attributes["image"],
		Stack:         msg.Actor.Attributes["com.docker.compose.project"],
		Status:        action,
		Time:          msg.Time,
	}

	payload := mustMarshal(evt)

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subscribers {
		select {
		case <-sub.ctx.Done():
			continue
		default:
		}
		if err := sub.client.send(ctx, Message{Type: "container_event", Payload: payload}); err != nil {
			slog.Debug("event send failed", "error", err)
		}
	}
}
