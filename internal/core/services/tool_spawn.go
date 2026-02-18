package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// NewSpawnTool creates a tool that launches an async sub-agent in the background.
// Unlike "delegate" (which blocks until all sub-tasks finish), "spawn" returns
// immediately with a sub-agent ID. The spawned agent runs its own ReAct loop and
// reports results back via the EventBus broadcast channel.
//
// Inspired by PicoClaw's spawn tool — fire-and-forget background agents that
// can work on long-running tasks while the main agent continues.
func NewSpawnTool(orchestrator *SubAgentOrchestrator, eventBus *EventBus, logger *slog.Logger) *domain.Tool {
	return &domain.Tool{
		Name:        "spawn",
		Description: "Spawn an async background agent to handle a task independently. The agent runs in the background and sends results back via message. Use for long-running tasks, research, or work that doesn't need immediate results. Returns a task ID to track the spawned agent.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "The task/prompt for the background agent to accomplish.",
				},
				"persona": map[string]interface{}{
					"type":        "string",
					"description": "Optional persona name or ID for the sub-agent. Defaults to 'assistant'.",
				},
			},
			Required: []string{"task"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			task, ok := params["task"].(string)
			if !ok || task == "" {
				return nil, fmt.Errorf("task is required")
			}

			persona := "assistant"
			if p, ok := params["persona"].(string); ok && p != "" {
				persona = p
			}

			// Extract conversation context
			convID, _ := ctx.Value(ctxKeyConversationID).(domain.ConversationID)
			parentID, _ := ctx.Value(ctxKeySubAgentID).(domain.SubAgentID)

			saID := domain.NewSubAgentID()

			// Fire off the sub-agent in a background goroutine
			go func() {
				bgCtx := context.Background() // detached from parent — outlives the request
				bgCtx = ContextWithConversation(bgCtx, convID)
				bgCtx = ContextWithSubAgent(bgCtx, saID)

				logger.Info("spawn: background agent started",
					"sa_id", string(saID),
					"persona", persona,
					"task", task[:min(80, len(task))],
				)

				spec := domain.DelegateTaskSpec{
					Persona: persona,
					Prompt:  task,
				}

				results, err := orchestrator.Delegate(bgCtx, convID, parentID, []domain.DelegateTaskSpec{spec})

				// Report results back via EventBus broadcast
				var payload map[string]interface{}
				if err != nil {
					payload = map[string]interface{}{
						"type":      "spawn_result",
						"sa_id":     string(saID),
						"status":    "failed",
						"error":     err.Error(),
						"task":      task,
						"persona":   persona,
						"timestamp": time.Now().UnixMilli(),
					}
					logger.Error("spawn: background agent failed", "sa_id", string(saID), "error", err)
				} else if len(results) > 0 && results[0].Status == domain.SubAgentStatusDone {
					payload = map[string]interface{}{
						"type":      "spawn_result",
						"sa_id":     string(saID),
						"status":    "done",
						"result":    results[0].Result,
						"task":      task,
						"persona":   results[0].PersonaName,
						"model":     results[0].ModelID,
						"timestamp": time.Now().UnixMilli(),
					}
					logger.Info("spawn: background agent completed", "sa_id", string(saID))
				} else {
					errMsg := "unknown error"
					if len(results) > 0 {
						errMsg = results[0].Error
					}
					payload = map[string]interface{}{
						"type":      "spawn_result",
						"sa_id":     string(saID),
						"status":    "failed",
						"error":     errMsg,
						"task":      task,
						"persona":   persona,
						"timestamp": time.Now().UnixMilli(),
					}
					logger.Warn("spawn: background agent finished with error", "sa_id", string(saID), "error", errMsg)
				}

				data, _ := json.Marshal(payload)
				eventBus.Publish(Event{
					JobID:     BroadcastChannel,
					Type:      EventTypeNewMessage,
					Data:      string(data),
					Timestamp: time.Now().UnixMilli(),
				})
			}()

			return map[string]interface{}{
				"status":  "spawned",
				"sa_id":   string(saID),
				"task":    task,
				"persona": persona,
				"message": fmt.Sprintf("Background agent %s spawned. Results will be delivered via message when complete.", string(saID)),
			}, nil
		},
	}
}
