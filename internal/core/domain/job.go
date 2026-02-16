package domain

import (
	"errors"
	"time"
)

type JobID string

type JobStatus string

const (
	JobStatusPending   JobStatus = "QUEUED"
	JobStatusRunning   JobStatus = "RUNNING"
	JobStatusCompleted JobStatus = "COMPLETED"
	JobStatusFailed    JobStatus = "FAILED"
	JobStatusCancelled JobStatus = "CANCELLED"
)

// Job represents a unit of work (AWU - Agentic Work Unit)
type Job struct {
	ID        JobID             `json:"id"`
	Result    *string           `json:"result,omitempty"` // Path to result or output
	Error     *string           `json:"error,omitempty"`
	Status    JobStatus         `json:"status"`
	WorkerID  *WorkerID         `json:"worker_id,omitempty"`
	Spec      WorkerSpec        `json:"spec"` // The spec needed to run this job
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Metadata  map[string]string `json:"metadata"`
}

var (
	ErrJobNotFound = errors.New("job not found")
)
