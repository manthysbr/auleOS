package services

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventBus_PubSub(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	bus := NewEventBus(logger)

	jobID := "job-123"
	
	// 1. Subscribe
	ch, unsub := bus.Subscribe(jobID)
	defer unsub()

	// 2. Publish
	event := Event{
		JobID:     jobID,
		Type:      EventTypeStatus,
		Data:      "test-data",
		Timestamp: time.Now().Unix(),
	}
	bus.Publish(event)

	// 3. Verify
	select {
	case received := <-ch:
		assert.Equal(t, event.JobID, received.JobID)
		assert.Equal(t, event.Data, received.Data)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	bus := NewEventBus(logger)
	jobID := "job-456"

	ch, unsub := bus.Subscribe(jobID)
	unsub() // Unsubscribe immediately

	bus.Publish(Event{JobID: jobID, Type: EventTypeLog, Data: "should not receive"})

	select {
	case e, ok := <-ch:
		if ok {
			t.Fatalf("received event after unsubscribe: %v", e)
		}
		// logic: channel is closed, which corresponds to unsubscribe
	case <-time.After(100 * time.Millisecond):
		// This path is actually ambiguous if channel isn't closed.
		// Unsubscribe closes the channel, so we Expect it to be closed.
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	bus := NewEventBus(logger)
	jobID := "job-multi"

	ch1, unsub1 := bus.Subscribe(jobID)
	defer unsub1()
	ch2, unsub2 := bus.Subscribe(jobID)
	defer unsub2()

	bus.Publish(Event{JobID: jobID, Data: "broadcast"})

	// Both should receive
	timeout := time.After(1 * time.Second)
	
	got1 := false
	got2 := false

	for i := 0; i < 2; i++ {
		select {
		case <-ch1:
			got1 = true
		case <-ch2:
			got2 = true
		case <-timeout:
			t.Fatal("timeout")
		}
	}

	assert.True(t, got1)
	assert.True(t, got2)
}
