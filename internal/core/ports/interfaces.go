package ports

import (
	"context"
	"io"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// WorkerManager abstracts the container runtime (Docker, Podman, etc.)
type WorkerManager interface {
	// Spawn creates and starts a new worker container based on the spec.
	// Returns the WorkerID on success.
	Spawn(ctx context.Context, spec domain.WorkerSpec) (domain.WorkerID, error)

	// HealthCheck pings the worker to determine its current state.
	HealthCheck(ctx context.Context, id domain.WorkerID) (domain.HealthStatus, error)

	// Kill wraps up the worker execution forcefully or gracefully.
	Kill(ctx context.Context, id domain.WorkerID) error

	// List returns all known workers in the runtime.
	List(ctx context.Context) ([]domain.Worker, error)

	// GetLogs searches for logs associated with a worker
	GetLogs(ctx context.Context, id domain.WorkerID) (io.ReadCloser, error)
}

// Repository abstracts the persistent storage (DuckDB)
type Repository interface {
	// SaveWorker persists the worker state.
	SaveWorker(ctx context.Context, worker domain.Worker) error

	// GetWorker retrieves a worker by ID.
	GetWorker(ctx context.Context, id domain.WorkerID) (domain.Worker, error)

	// ListWorkers returns all workers, optionally filtered.
	ListWorkers(ctx context.Context) ([]domain.Worker, error)

	// UpdateWorkerStatus updates just the status of a worker.
	UpdateWorkerStatus(ctx context.Context, id domain.WorkerID, status domain.HealthStatus) error

	// Job Management
	SaveJob(ctx context.Context, job domain.Job) error
	GetJob(ctx context.Context, id domain.JobID) (domain.Job, error)
	ListJobs(ctx context.Context) ([]domain.Job, error)
}
