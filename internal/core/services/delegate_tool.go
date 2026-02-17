package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// NewDelegateTool creates the "delegate" tool that spawns sub-agents.
// The orchestrator LLM calls this tool with a JSON payload listing tasks + personas.
// Sub-agents run in parallel, each with their own persona/model/tools.
func NewDelegateTool(orchestrator *SubAgentOrchestrator) *domain.Tool {
	return &domain.Tool{
		Name:        "delegate",
		Description: "Delegate sub-tasks to specialized persona agents. Each task runs in parallel with its own model. Use when the request involves multiple distinct sub-tasks (e.g., research + code + creative).",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"tasks": map[string]interface{}{
					"type":        "array",
					"description": "List of tasks to delegate. Each has a 'persona' (name or id) and a 'prompt' (what the sub-agent should do).",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"persona": map[string]interface{}{
								"type":        "string",
								"description": "Persona name or ID: assistant, researcher, creator, coder, or a custom persona ID",
							},
							"prompt": map[string]interface{}{
								"type":        "string",
								"description": "The specific task for this sub-agent to accomplish",
							},
						},
						"required": []string{"persona", "prompt"},
					},
				},
			},
			Required: []string{"tasks"},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			// Parse tasks from params
			tasksRaw, ok := params["tasks"]
			if !ok {
				return nil, fmt.Errorf("missing required parameter: tasks")
			}

			// Convert to JSON and back to parse the array properly
			tasksJSON, err := json.Marshal(tasksRaw)
			if err != nil {
				return nil, fmt.Errorf("invalid tasks format: %w", err)
			}

			var taskSpecs []domain.DelegateTaskSpec
			if err := json.Unmarshal(tasksJSON, &taskSpecs); err != nil {
				return nil, fmt.Errorf("failed to parse tasks: %w", err)
			}

			if len(taskSpecs) == 0 {
				return nil, fmt.Errorf("tasks array is empty")
			}

			// Extract conversation ID from context (set by the ReAct loop)
			convID, _ := ctx.Value(ctxKeyConversationID).(domain.ConversationID)
			parentID, _ := ctx.Value(ctxKeySubAgentID).(domain.SubAgentID)

			// Execute all sub-agents in parallel
			results, err := orchestrator.Delegate(ctx, convID, parentID, taskSpecs)
			if err != nil {
				return nil, fmt.Errorf("delegation failed: %w", err)
			}

			// Format combined results
			var summary strings.Builder
			summary.WriteString(fmt.Sprintf("Delegated %d sub-tasks:\n\n", len(results)))

			for i, r := range results {
				summary.WriteString(fmt.Sprintf("--- Sub-Agent %d: %s (%s, model: %s) ---\n", i+1, r.PersonaName, string(r.Status), r.ModelID))
				if r.Status == domain.SubAgentStatusDone {
					summary.WriteString(fmt.Sprintf("Result: %s\n\n", r.Result))
				} else {
					summary.WriteString(fmt.Sprintf("Error: %s\n\n", r.Error))
				}
			}

			return map[string]interface{}{
				"status":    "completed",
				"sub_tasks": len(results),
				"summary":   summary.String(),
			}, nil
		},
	}
}

// Context keys for passing conversation/sub-agent IDs through the tool execution chain
type contextKey string

const (
	ctxKeyConversationID contextKey = "conversation_id"
	ctxKeySubAgentID     contextKey = "sub_agent_id"
)

// ContextWithConversation adds the conversation ID to context for tools to use.
func ContextWithConversation(ctx context.Context, id domain.ConversationID) context.Context {
	return context.WithValue(ctx, ctxKeyConversationID, id)
}

// ContextWithSubAgent adds the sub-agent ID to context for nested delegation.
func ContextWithSubAgent(ctx context.Context, id domain.SubAgentID) context.Context {
	return context.WithValue(ctx, ctxKeySubAgentID, id)
}
