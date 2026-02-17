package kernel

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// StreamConversationEvents serves SSE events for a conversation (sub-agent activity, etc.)
// NOTE: The strict server pattern doesn't work well with SSE streaming.
// We return a placeholder here and handle the real SSE in the raw HTTP wrapper.
// See server.go for the actual SSE handler registered separately.
func (s *Server) StreamConversationEvents(_ context.Context, req StreamConversationEventsRequestObject) (StreamConversationEventsResponseObject, error) {
	// This is a no-op — the actual SSE is handled by the raw HTTP handler
	// registered in server.go's Handler() method, which bypasses the strict wrapper.
	_ = req.Id
	return StreamConversationEvents200TexteventStreamResponse{
		Body: nil,
	}, nil
}

// handleConversationSSE is the raw HTTP handler for SSE streaming of conversation events.
// It subscribes to the EventBus using the conversation ID as the key, and streams
// sub_agent and agent events in real-time.
func (s *Server) handleConversationSSE(w http.ResponseWriter, r *http.Request) {
	// Extract conversation ID from path: /v1/conversations/{id}/events
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	var convID string
	if len(parts) >= 3 {
		convID = parts[2] // v1/conversations/{id}/events → index 2
	}
	if convID == "" {
		http.Error(w, "missing conversation id", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Subscribe to events for this conversation (keyed by conv ID in the EventBus)
	ch, unsub := s.eventBus.Subscribe(convID)
	defer unsub()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			// evt.Type tells us if it's sub_agent, status, log, etc.
			// evt.Data is the JSON payload
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, evt.Data)
			flusher.Flush()
		}
	}
}
