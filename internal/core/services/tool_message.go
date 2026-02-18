package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// BroadcastChannel is the well-known EventBus key for global agent messages.
// Clients subscribe to this channel to receive proactive agent messages.
const BroadcastChannel = "__broadcast__"

// NewMessageTool creates a tool that sends a message to the user via EventBus.
// Inspired by PicoClaw's message tool — lets heartbeat/cron/subagents communicate
// results back to the user without waiting for the main agent loop.
func NewMessageTool(eventBus *EventBus) *domain.Tool {
	return &domain.Tool{
		Name:        "message",
		Description: "Send a message to the user. Use this when you want to communicate results proactively (e.g., from heartbeat tasks, scheduled tasks, or subagent work). The message appears in real-time via SSE stream.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The message content to send to the user.",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional: project ID context for routing the message.",
				},
			},
			Required: []string{"content"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			content, ok := params["content"].(string)
			if !ok || content == "" {
				return nil, fmt.Errorf("content is required")
			}

			projectID, _ := params["project_id"].(string)
			if projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
			}

			// Emit as an SSE event on the broadcast channel
			payload, _ := json.Marshal(map[string]interface{}{
				"content":    content,
				"project_id": projectID,
				"source":     "agent",
			})

			eventBus.Publish(Event{
				JobID:     BroadcastChannel,
				Type:      EventTypeNewMessage,
				Data:      string(payload),
				Timestamp: time.Now().UnixMilli(),
			})

			return fmt.Sprintf("Message sent to user."), nil
		},
	}
}
