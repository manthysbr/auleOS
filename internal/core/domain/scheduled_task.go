package domain

import "time"

// ScheduledTaskID is the unique identifier for a scheduled task
type ScheduledTaskID string

// ScheduledTaskStatus represents the current state of a scheduled task
type ScheduledTaskStatus string

const (
	TaskStatusActive    ScheduledTaskStatus = "active"
	TaskStatusPaused    ScheduledTaskStatus = "paused"
	TaskStatusCompleted ScheduledTaskStatus = "completed" // one-shot that has run
	TaskStatusFailed    ScheduledTaskStatus = "failed"
)

// ScheduledTaskType differentiates one-shot from recurring tasks
type ScheduledTaskType string

const (
	TaskTypeOneShot   ScheduledTaskType = "one_shot"  // run once at a specific time
	TaskTypeRecurring ScheduledTaskType = "recurring" // repeat on interval
	TaskTypeCron      ScheduledTaskType = "cron"      // cron expression
)

// ScheduledTask represents a task to be executed at scheduled times
type ScheduledTask struct {
	ID          ScheduledTaskID     `json:"id"`
	ProjectID   ProjectID           `json:"project_id"`
	Name        string              `json:"name"`
	Prompt      string              `json:"prompt"`            // The instruction to execute via ReAct agent
	Command     string              `json:"command,omitempty"` // Direct command (bypasses LLM if set)
	Deliver     bool                `json:"deliver"`           // If true, send result to user via message tool
	PersonaID   *PersonaID          `json:"persona_id,omitempty"`
	Type        ScheduledTaskType   `json:"type"`
	CronExpr    string              `json:"cron_expr,omitempty"`    // cron expression (for Type=cron)
	IntervalSec int                 `json:"interval_sec,omitempty"` // interval in seconds (for Type=recurring)
	NextRun     time.Time           `json:"next_run"`
	LastRun     *time.Time          `json:"last_run,omitempty"`
	LastResult  string              `json:"last_result,omitempty"`
	RunCount    int                 `json:"run_count"`
	Status      ScheduledTaskStatus `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	CreatedBy   string              `json:"created_by,omitempty"` // "agent" or "user"
}
