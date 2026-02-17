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

	// GetWorkerIP returns the IP address of a worker container
	GetWorkerIP(ctx context.Context, id domain.WorkerID) (string, error)
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

	// Conversations
	CreateConversation(ctx context.Context, conv domain.Conversation) error
	GetConversation(ctx context.Context, id domain.ConversationID) (domain.Conversation, error)
	ListConversations(ctx context.Context) ([]domain.Conversation, error)
	UpdateConversationTitle(ctx context.Context, id domain.ConversationID, title string) error
	DeleteConversation(ctx context.Context, id domain.ConversationID) error

	// Messages
	AddMessage(ctx context.Context, msg domain.Message) error
	ListMessages(ctx context.Context, convID domain.ConversationID, limit int) ([]domain.Message, error)

	// Projects
	CreateProject(ctx context.Context, proj domain.Project) error
	GetProject(ctx context.Context, id domain.ProjectID) (domain.Project, error)
	ListProjects(ctx context.Context) ([]domain.Project, error)
	UpdateProject(ctx context.Context, proj domain.Project) error
	DeleteProject(ctx context.Context, id domain.ProjectID) error
	ListProjectConversations(ctx context.Context, projectID domain.ProjectID) ([]domain.Conversation, error)

	// Artifacts
	SaveArtifact(ctx context.Context, art domain.Artifact) error
	GetArtifact(ctx context.Context, id domain.ArtifactID) (domain.Artifact, error)
	ListArtifacts(ctx context.Context) ([]domain.Artifact, error)
	ListProjectArtifacts(ctx context.Context, projectID domain.ProjectID) ([]domain.Artifact, error)
	DeleteArtifact(ctx context.Context, id domain.ArtifactID) error

	// Personas
	CreatePersona(ctx context.Context, p domain.Persona) error
	GetPersona(ctx context.Context, id domain.PersonaID) (domain.Persona, error)
	ListPersonas(ctx context.Context) ([]domain.Persona, error)
	UpdatePersona(ctx context.Context, p domain.Persona) error
	DeletePersona(ctx context.Context, id domain.PersonaID) error

	// Settings
	GetSetting(ctx context.Context, key string) (string, error)
	SaveSetting(ctx context.Context, key string, value string) error
}
