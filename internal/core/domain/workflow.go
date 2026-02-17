package domain

import (
	"time"
)

type WorkflowID string
type WorkflowStatus string
type StepStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusPaused    WorkflowStatus = "paused"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusCancelled WorkflowStatus = "cancelled"

	StepStatusPending StepStatus = "pending"
	StepStatusRunning StepStatus = "running"
	StepStatusDone    StepStatus = "done"
	StepStatusFailed  StepStatus = "failed"
	StepStatusSkipped StepStatus = "skipped"
	StepStatusCancelled StepStatus = "cancelled"
)

// Workflow represents a multi-step agentic process
type Workflow struct {
	ID          WorkflowID     `json:"id"`
	ProjectID   ProjectID      `json:"project_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
	State       map[string]any `json:"state"` // Shared state accessible by steps via {{state.key}}
	Status      WorkflowStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Error       *string        `json:"error,omitempty"`
}

// WorkflowStep is a single unit of work in the DAG
type WorkflowStep struct {
	ID          string         `json:"id"`         // Unique ID within the workflow (e.g. "research")
	PersonaID   PersonaID      `json:"persona_id"` // The agent persona to execute this step
	Prompt      string         `json:"prompt"`     // The instruction (can use {{state.x}})
	Tools       []string       `json:"tools"`      // List of allowed tool names for this step
	DependsOn   []string       `json:"depends_on"` // IDs of steps that must complete first
	Interrupt   *InterruptRule `json:"interrupt,omitempty"`
	Status      StepStatus     `json:"status"`
	Result      *StepResult    `json:"result,omitempty"`
	MaxIters    int            `json:"max_iters"` // ReAct loop limit (default 5)
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Error       *string        `json:"error,omitempty"`
}

// InterruptRule defines conditions to pause the workflow for human input
type InterruptRule struct {
	Before  bool   `json:"before"`  // Pause before executing this step?
	After   bool   `json:"after"`   // Pause after executing (to review output)?
	Message string `json:"message"` // Message to show the user
}

// StepResult captures the output of a step
type StepResult struct {
	Output    string                 `json:"output"`
	Artifacts []string               `json:"artifacts,omitempty"` // IDs/Paths of generated artifacts
	Metadata  map[string]interface{} `json:"metadata,omitempty"`  // Extra data (tokens, duration)
}
