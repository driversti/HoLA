package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Message is the envelope for all WebSocket messages.
type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// SubscribePayload is sent by the client to start/stop a stream.
type SubscribePayload struct {
	Stream          string `json:"stream"`
	ContainerID     string `json:"container_id,omitempty"`
	IntervalSeconds int    `json:"interval_seconds,omitempty"`
}

// client represents a single WebSocket connection.
type client struct {
	conn          *websocket.Conn
	mu            sync.Mutex
	subscriptions map[string]context.CancelFunc // key: "metrics", "events", "logs:<container_id>"
}

func (c *client) send(ctx context.Context, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return wsjson.Write(ctx, c.conn, msg)
}

func (c *client) cancelAll() {
	for key, cancel := range c.subscriptions {
		cancel()
		delete(c.subscriptions, key)
	}
}

// Handler accepts WebSocket connections and manages subscriptions.
type Handler struct {
	eventHub *EventHub
}

// NewHandler creates a WebSocket handler.
func NewHandler(eventHub *EventHub) *Handler {
	return &Handler{eventHub: eventHub}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Allow all origins — agent runs on a trusted network.
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Error("websocket accept failed", "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	slog.Info("websocket client connected", "remote", r.RemoteAddr)

	c := &client{
		conn:          conn,
		subscriptions: make(map[string]context.CancelFunc),
	}
	defer c.cancelAll()

	h.readLoop(r.Context(), c)
}

func (h *Handler) readLoop(ctx context.Context, c *client) {
	for {
		var msg Message
		err := wsjson.Read(ctx, c.conn, &msg)
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				slog.Info("websocket client disconnected", "status", websocket.CloseStatus(err))
			} else {
				slog.Warn("websocket read error", "error", err)
			}
			return
		}

		switch msg.Type {
		case "subscribe":
			h.handleSubscribe(ctx, c, msg)
		case "unsubscribe":
			h.handleUnsubscribe(ctx, c, msg)
		case "ping":
			_ = c.send(ctx, Message{Type: "pong"})
		default:
			_ = c.send(ctx, Message{
				Type:    "error",
				Payload: mustMarshal(ErrorPayload{Error: "unknown message type: " + msg.Type, Code: "UNKNOWN_TYPE"}),
			})
		}
	}
}

func (h *Handler) handleSubscribe(ctx context.Context, c *client, msg Message) {
	var payload SubscribePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		_ = c.send(ctx, Message{
			Type:    "error",
			Payload: mustMarshal(ErrorPayload{Error: "invalid subscribe payload", Code: "BAD_PAYLOAD"}),
		})
		return
	}

	switch payload.Stream {
	case "metrics":
		subKey := "metrics"
		if _, exists := c.subscriptions[subKey]; exists {
			_ = c.send(ctx, Message{
				Type:    "error",
				Payload: mustMarshal(ErrorPayload{Error: "already subscribed to metrics", Code: "ALREADY_SUBSCRIBED"}),
			})
			return
		}

		subCtx, cancel := context.WithCancel(ctx)
		c.subscriptions[subKey] = cancel
		go streamMetrics(subCtx, c, payload.IntervalSeconds)

		_ = c.send(ctx, Message{
			Type:    "subscribed",
			ID:      msg.ID,
			Payload: mustMarshal(SubscribePayload{Stream: "metrics"}),
		})

	case "events":
		subKey := "events"
		if _, exists := c.subscriptions[subKey]; exists {
			_ = c.send(ctx, Message{
				Type:    "error",
				Payload: mustMarshal(ErrorPayload{Error: "already subscribed to events", Code: "ALREADY_SUBSCRIBED"}),
			})
			return
		}

		if h.eventHub == nil {
			_ = c.send(ctx, Message{
				Type:    "error",
				Payload: mustMarshal(ErrorPayload{Error: "event hub not available", Code: "NOT_AVAILABLE"}),
			})
			return
		}

		subCtx, cancel := context.WithCancel(ctx)
		c.subscriptions[subKey] = cancel
		h.eventHub.Subscribe(subCtx, c)

		_ = c.send(ctx, Message{
			Type:    "subscribed",
			ID:      msg.ID,
			Payload: mustMarshal(SubscribePayload{Stream: "events"}),
		})

	case "logs":
		if payload.ContainerID == "" {
			_ = c.send(ctx, Message{
				Type:    "error",
				Payload: mustMarshal(ErrorPayload{Error: "container_id required for logs stream", Code: "MISSING_CONTAINER_ID"}),
			})
			return
		}

		subKey := "logs:" + payload.ContainerID

		// Enforce max 3 concurrent log subscriptions.
		logCount := 0
		for key := range c.subscriptions {
			if len(key) > 5 && key[:5] == "logs:" {
				logCount++
			}
		}
		if logCount >= 3 {
			_ = c.send(ctx, Message{
				Type:    "error",
				Payload: mustMarshal(ErrorPayload{Error: "max 3 concurrent log subscriptions", Code: "LIMIT_EXCEEDED"}),
			})
			return
		}

		if _, exists := c.subscriptions[subKey]; exists {
			_ = c.send(ctx, Message{
				Type:    "error",
				Payload: mustMarshal(ErrorPayload{Error: "already subscribed to logs for this container", Code: "ALREADY_SUBSCRIBED"}),
			})
			return
		}

		subCtx, cancel := context.WithCancel(ctx)
		c.subscriptions[subKey] = cancel
		go streamLogs(subCtx, c, h.eventHub.dockerClient, payload.ContainerID)

		_ = c.send(ctx, Message{
			Type:    "subscribed",
			ID:      msg.ID,
			Payload: mustMarshal(SubscribePayload{Stream: "logs", ContainerID: payload.ContainerID}),
		})

	default:
		_ = c.send(ctx, Message{
			Type:    "error",
			Payload: mustMarshal(ErrorPayload{Error: "unknown stream: " + payload.Stream, Code: "UNKNOWN_STREAM"}),
		})
	}
}

func (h *Handler) handleUnsubscribe(ctx context.Context, c *client, msg Message) {
	var payload SubscribePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		_ = c.send(ctx, Message{
			Type:    "error",
			Payload: mustMarshal(ErrorPayload{Error: "invalid unsubscribe payload", Code: "BAD_PAYLOAD"}),
		})
		return
	}

	subKey := payload.Stream
	if payload.Stream == "logs" && payload.ContainerID != "" {
		subKey = "logs:" + payload.ContainerID
	}

	cancel, exists := c.subscriptions[subKey]
	if !exists {
		_ = c.send(ctx, Message{
			Type:    "error",
			Payload: mustMarshal(ErrorPayload{Error: "not subscribed to " + subKey, Code: "NOT_SUBSCRIBED"}),
		})
		return
	}

	cancel()
	delete(c.subscriptions, subKey)

	_ = c.send(ctx, Message{
		Type:    "subscribed", // reuse as ack
		ID:      msg.ID,
		Payload: mustMarshal(map[string]string{"stream": payload.Stream, "status": "unsubscribed"}),
	})
}

// ErrorPayload is the payload for error messages.
type ErrorPayload struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("ws: marshal failed: " + err.Error())
	}
	return data
}
