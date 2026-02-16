package domain

import (
	"context"
	"errors"
	"time"
)

// ID types to prevent stringly-typed confusion
type WorkerID string

// HealthStatus represents the current state of a worker
type HealthStatus string

const (
	HealthStatusUnknown   HealthStatus = "UNKNOWN"
	HealthStatusStarting  HealthStatus = "STARTING"
	HealthStatusHealthy   HealthStatus = "HEALTHY"
	HealthStatusUnhealthy HealthStatus = "UNHEALTHY"
	HealthStatusExited    HealthStatus = "EXITED"
)

// WorkerSpec defines how a worker should be spawned
type WorkerSpec struct {
	Image       string            `json:"image"`
	Command     []string          `json:"command"`
	Env         map[string]string `json:"env"`
	ResourceCPU float64           `json:"resource_cpu"` // 0.5 = 50% core
	ResourceMem int64             `json:"resource_mem"` // in bytes
	Tags        map[string]string `json:"tags"`
	BindMounts  map[string]string `json:"bind_mounts"` // HostPath -> ContainerPath
}

// Worker represents a running instance
type Worker struct {
	ID        WorkerID          `json:"id"`
	Spec      WorkerSpec        `json:"spec"`
	Status    HealthStatus      `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Metadata  map[string]string `json:"metadata"`
}

var (
	ErrWorkerNotFound = errors.New("worker not found")
)

// ToolCall represents an intent execution by the agent
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// ImageProvider defines the interface for image generation services
type ImageProvider interface {
	GenerateImage(ctx context.Context, prompt string) (string, error)
}

// LLMProvider defines the interface for LLM services
type LLMProvider interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
}

