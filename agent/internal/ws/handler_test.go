package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestPingPong(t *testing.T) {
	h := NewHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	err = wsjson.Write(ctx, conn, Message{Type: "ping"})
	if err != nil {
		t.Fatal(err)
	}

	var resp Message
	err = wsjson.Read(ctx, conn, &resp)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Type != "pong" {
		t.Fatalf("want type pong, got %q", resp.Type)
	}
}

func TestUnknownMessageType(t *testing.T) {
	h := NewHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	err = wsjson.Write(ctx, conn, Message{Type: "foobar"})
	if err != nil {
		t.Fatal(err)
	}

	var resp Message
	err = wsjson.Read(ctx, conn, &resp)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Type != "error" {
		t.Fatalf("want type error, got %q", resp.Type)
	}

	var errPayload ErrorPayload
	json.Unmarshal(resp.Payload, &errPayload)
	if errPayload.Code != "UNKNOWN_TYPE" {
		t.Fatalf("want code UNKNOWN_TYPE, got %q", errPayload.Code)
	}
}

func TestSubscribeMetrics(t *testing.T) {
	h := NewHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Subscribe to metrics with a 1s interval.
	sub := Message{
		Type:    "subscribe",
		ID:      "sub-1",
		Payload: mustMarshal(SubscribePayload{Stream: "metrics", IntervalSeconds: 1}),
	}
	err = wsjson.Write(ctx, conn, sub)
	if err != nil {
		t.Fatal(err)
	}

	// First message should be the subscription ack.
	var ack Message
	err = wsjson.Read(ctx, conn, &ack)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Type != "subscribed" {
		t.Fatalf("want type subscribed, got %q", ack.Type)
	}
	if ack.ID != "sub-1" {
		t.Fatalf("want id sub-1, got %q", ack.ID)
	}

	// Next message should be the immediate metrics snapshot.
	var metrics Message
	err = wsjson.Read(ctx, conn, &metrics)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Type != "metrics" {
		t.Fatalf("want type metrics, got %q", metrics.Type)
	}
	if len(metrics.Payload) == 0 {
		t.Fatal("metrics payload is empty")
	}
}

func TestSubscribeDuplicate(t *testing.T) {
	h := NewHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	sub := Message{
		Type:    "subscribe",
		Payload: mustMarshal(SubscribePayload{Stream: "metrics", IntervalSeconds: 30}),
	}

	// First subscribe.
	err = wsjson.Write(ctx, conn, sub)
	if err != nil {
		t.Fatal(err)
	}
	var ack Message
	wsjson.Read(ctx, conn, &ack)       // ack
	wsjson.Read(ctx, conn, &Message{}) // initial metrics

	// Second subscribe — should get error.
	err = wsjson.Write(ctx, conn, sub)
	if err != nil {
		t.Fatal(err)
	}
	var errMsg Message
	wsjson.Read(ctx, conn, &errMsg)

	if errMsg.Type != "error" {
		t.Fatalf("want type error, got %q", errMsg.Type)
	}

	var errPayload ErrorPayload
	json.Unmarshal(errMsg.Payload, &errPayload)
	if errPayload.Code != "ALREADY_SUBSCRIBED" {
		t.Fatalf("want code ALREADY_SUBSCRIBED, got %q", errPayload.Code)
	}
}

func TestSubscribeUnknownStream(t *testing.T) {
	h := NewHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	sub := Message{
		Type:    "subscribe",
		Payload: mustMarshal(SubscribePayload{Stream: "nope"}),
	}
	err = wsjson.Write(ctx, conn, sub)
	if err != nil {
		t.Fatal(err)
	}

	var resp Message
	wsjson.Read(ctx, conn, &resp)

	if resp.Type != "error" {
		t.Fatalf("want type error, got %q", resp.Type)
	}

	var errPayload ErrorPayload
	json.Unmarshal(resp.Payload, &errPayload)
	if errPayload.Code != "UNKNOWN_STREAM" {
		t.Fatalf("want code UNKNOWN_STREAM, got %q", errPayload.Code)
	}
}

func TestLogsRequiresContainerID(t *testing.T) {
	h := NewHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	sub := Message{
		Type:    "subscribe",
		Payload: mustMarshal(SubscribePayload{Stream: "logs"}),
	}
	err = wsjson.Write(ctx, conn, sub)
	if err != nil {
		t.Fatal(err)
	}

	var resp Message
	wsjson.Read(ctx, conn, &resp)

	if resp.Type != "error" {
		t.Fatalf("want type error, got %q", resp.Type)
	}

	var errPayload ErrorPayload
	json.Unmarshal(resp.Payload, &errPayload)
	if errPayload.Code != "MISSING_CONTAINER_ID" {
		t.Fatalf("want code MISSING_CONTAINER_ID, got %q", errPayload.Code)
	}
}

func TestInvalidPayload(t *testing.T) {
	h := NewHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Send subscribe with invalid payload (not JSON object).
	raw := `{"type":"subscribe","payload":"not-an-object"}`
	err = conn.Write(ctx, websocket.MessageText, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}

	var resp Message
	wsjson.Read(ctx, conn, &resp)

	if resp.Type != "error" {
		t.Fatalf("want type error, got %q", resp.Type)
	}

	var errPayload ErrorPayload
	json.Unmarshal(resp.Payload, &errPayload)
	if errPayload.Code != "BAD_PAYLOAD" {
		t.Fatalf("want code BAD_PAYLOAD, got %q", errPayload.Code)
	}
}

func TestUnsubscribeNotSubscribed(t *testing.T) {
	h := NewHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	unsub := Message{
		Type:    "unsubscribe",
		Payload: mustMarshal(SubscribePayload{Stream: "metrics"}),
	}
	err = wsjson.Write(ctx, conn, unsub)
	if err != nil {
		t.Fatal(err)
	}

	var resp Message
	wsjson.Read(ctx, conn, &resp)

	if resp.Type != "error" {
		t.Fatalf("want type error, got %q", resp.Type)
	}

	var errPayload ErrorPayload
	json.Unmarshal(resp.Payload, &errPayload)
	if errPayload.Code != "NOT_SUBSCRIBED" {
		t.Fatalf("want code NOT_SUBSCRIBED, got %q", errPayload.Code)
	}
}

func TestSubscribeEventsWithNilHub(t *testing.T) {
	h := NewHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	sub := Message{
		Type:    "subscribe",
		Payload: mustMarshal(SubscribePayload{Stream: "events"}),
	}
	err = wsjson.Write(ctx, conn, sub)
	if err != nil {
		t.Fatal(err)
	}

	var resp Message
	wsjson.Read(ctx, conn, &resp)

	if resp.Type != "error" {
		t.Fatalf("want type error, got %q", resp.Type)
	}

	var errPayload ErrorPayload
	json.Unmarshal(resp.Payload, &errPayload)
	if errPayload.Code != "NOT_AVAILABLE" {
		t.Fatalf("want code NOT_AVAILABLE, got %q", errPayload.Code)
	}
}

// helpers

func testServer(h http.Handler) (*httptest.Server, *websocket.Conn, func()) {
	srv := httptest.NewServer(h)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn, _, err := websocket.Dial(ctx, "ws"+srv.URL[4:], nil)
	if err != nil {
		panic(err)
	}
	return srv, conn, func() {
		conn.Close(websocket.StatusNormalClosure, "done")
		cancel()
		srv.Close()
	}
}
