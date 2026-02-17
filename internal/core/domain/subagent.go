package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// SubAgentID uniquely identifies a sub-agent execution
type SubAgentID string

// NewSubAgentID generates a compact random sub-agent ID (sa-<12 hex>)
func NewSubAgentID() SubAgentID {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return SubAgentID("sa-" + hex.EncodeToString(b))
}

// SubAgentStatus represents the lifecycle state of a sub-agent task
type SubAgentStatus string

const (
	SubAgentStatusPending SubAgentStatus = "pending"
	SubAgentStatusRunning SubAgentStatus = "running"
	SubAgentStatusDone    SubAgentStatus = "done"
	SubAgentStatusFailed  SubAgentStatus = "failed"
)

// SubAgentTask is a unit of work delegated by the orchestrator to a sub-agent.
// Each task runs its own mini-ReAct loop on a specific persona + model.
type SubAgentTask struct {
	ID             SubAgentID     `json:"id"`
	ParentID       SubAgentID     `json:"parent_id"` // orchestrator's SA ID (empty if root)
	ConversationID ConversationID `json:"conversation_id"`
	PersonaID      PersonaID      `json:"persona_id"`
	PersonaName    string         `json:"persona_name"`
	ModelID        string         `json:"model_id"` // resolved model: "qwen2.5:3b"
	Prompt         string         `json:"prompt"`
	Status         SubAgentStatus `json:"status"`
	Result         string         `json:"result,omitempty"`
	Error          string         `json:"error,omitempty"`
	Steps          []ReActStep    `json:"steps,omitempty"`
	StartedAt      time.Time      `json:"started_at"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
}

// SubAgentEvent is emitted on the EventBus so the UI can show sub-agent activity in real time.
type SubAgentEvent struct {
	SubAgentID     SubAgentID     `json:"sub_agent_id"`
	ParentID       SubAgentID     `json:"parent_id"`
	ConversationID ConversationID `json:"conversation_id"`
	PersonaName    string         `json:"persona_name"`
	PersonaColor   string         `json:"persona_color"`
	PersonaIcon    string         `json:"persona_icon"`
	ModelID        string         `json:"model_id"`
	Status         SubAgentStatus `json:"status"`
	Thought        string         `json:"thought,omitempty"` // current thought (streaming)
	Result         string         `json:"result,omitempty"`  // final result
	Error          string         `json:"error,omitempty"`
}

// DelegateRequest is the structured input to the "delegate" tool.
// The orchestrator LLM outputs this JSON as Action Input when it wants to spawn sub-agents.
type DelegateRequest struct {
	Tasks []DelegateTaskSpec `json:"tasks"`
}

// DelegateTaskSpec describes one sub-task to delegate.
type DelegateTaskSpec struct {
	Persona string `json:"persona"` // persona ID or name
	Prompt  string `json:"prompt"`  // what the sub-agent should do
}
