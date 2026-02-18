package services

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestEventBusPublish(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bus := NewEventBus(logger)

	ch, unsub := bus.Subscribe("job-1")
	defer unsub()

	bus.Publish(Event{
		JobID:     "job-1",
		Type:      EventTypeStatus,
		Data:      `{"status":"running"}`,
		Timestamp: time.Now().UnixMilli(),
	})

	select {
	case evt := <-ch:
		if evt.Type != EventTypeStatus {
			t.Errorf("expected EventTypeStatus, got %s", evt.Type)
		}
		if evt.Data != `{"status":"running"}` {
			t.Errorf("unexpected data: %s", evt.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBusPublishNoSubscriber(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bus := NewEventBus(logger)

	// Publishing with no subscriber should not panic
	bus.Publish(Event{
		JobID:     "no-such-job",
		Type:      EventTypeLog,
		Data:      "test",
		Timestamp: time.Now().UnixMilli(),
	})
}

func TestEventBusGlobalSubscriber(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bus := NewEventBus(logger)

	// Subscribe globally
	globalCh, unsub := bus.SubscribeGlobal()
	defer unsub()

	// Publish to a specific job — global should still receive it
	bus.Publish(Event{
		JobID:     "job-abc",
		Type:      EventTypeNewMessage,
		Data:      `{"msg":"hello"}`,
		Timestamp: time.Now().UnixMilli(),
	})

	select {
	case evt := <-globalCh:
		if evt.JobID != "job-abc" {
			t.Errorf("expected job-abc, got %s", evt.JobID)
		}
		if evt.Type != EventTypeNewMessage {
			t.Errorf("expected new_message, got %s", evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for global event")
	}
}

func TestEventBusBroadcastChannel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bus := NewEventBus(logger)

	// Subscribe to the broadcast channel specifically
	broadcastCh, unsub := bus.Subscribe(BroadcastChannel)
	defer unsub()

	// Also subscribe globally
	globalCh, unsub2 := bus.SubscribeGlobal()
	defer unsub2()

	bus.Publish(Event{
		JobID:     BroadcastChannel,
		Type:      EventTypeNewMessage,
		Data:      `{"content":"proactive message"}`,
		Timestamp: time.Now().UnixMilli(),
	})

	// Both channels should receive the event
	select {
	case evt := <-broadcastCh:
		if evt.Data != `{"content":"proactive message"}` {
			t.Errorf("unexpected broadcast data: %s", evt.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast event")
	}

	select {
	case evt := <-globalCh:
		if evt.Data != `{"content":"proactive message"}` {
			t.Errorf("unexpected global data: %s", evt.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for global event from broadcast")
	}
}

func TestEventBusUnsubscribe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bus := NewEventBus(logger)

	ch, unsub := bus.Subscribe("job-x")

	// Unsubscribe immediately
	unsub()

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}

func TestEventBusGlobalUnsubscribe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bus := NewEventBus(logger)

	ch, unsub := bus.SubscribeGlobal()
	unsub()

	_, ok := <-ch
	if ok {
		t.Error("expected global channel to be closed after unsubscribe")
	}
}

func TestMessageTool(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bus := NewEventBus(logger)

	tool := NewMessageTool(bus)

	if tool.Name != "message" {
		t.Errorf("expected tool name 'message', got %s", tool.Name)
	}

	// Subscribe to broadcast channel to capture the message
	ch, unsub := bus.Subscribe(BroadcastChannel)
	defer unsub()

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"content": "Hello from heartbeat!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	if resultStr != "Message sent to user." {
		t.Errorf("unexpected result: %s", resultStr)
	}

	// Verify the event was published
	select {
	case evt := <-ch:
		if evt.Type != EventTypeNewMessage {
			t.Errorf("expected new_message type, got %s", evt.Type)
		}
		if evt.JobID != BroadcastChannel {
			t.Errorf("expected broadcast channel, got %s", evt.JobID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message event")
	}
}

func TestMessageToolRequiresContent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bus := NewEventBus(logger)

	tool := NewMessageTool(bus)

	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}
