package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/ports"
)

type WorkerLifecycle struct {
	logger    *slog.Logger
	scheduler *JobScheduler
	workerMgr ports.WorkerManager
	repo      ports.Repository
	workspace *WorkspaceManager
	eventBus  *EventBus
}

func NewWorkerLifecycle(
	logger *slog.Logger,
	scheduler *JobScheduler,
	mgr ports.WorkerManager,
	repo ports.Repository,
	ws *WorkspaceManager,
	eventBus *EventBus,
) *WorkerLifecycle {
	return &WorkerLifecycle{
		logger:    logger,
		scheduler: scheduler,
		workerMgr: mgr,
		repo:      repo,
		workspace: ws,
		eventBus:  eventBus,
	}
}

// Run starts the scheduler loop
func (s *WorkerLifecycle) Run(ctx context.Context) error {
	s.scheduler.Start(ctx, s.executeJob)
	return nil
}

func (s *WorkerLifecycle) publishStatus(jobID string, status string) {
	s.eventBus.Publish(Event{
		JobID:     jobID,
		Type:      EventTypeStatus,
		Data:      fmt.Sprintf(`{"status": "%s"}`, status),
		Timestamp: time.Now().Unix(),
	})
}

// executeJob is the callback for the scheduler
func (s *WorkerLifecycle) executeJob(ctx context.Context, job domain.Job) {
	s.logger.Info("executing job", "job_id", job.ID)
	
	// Publish RUNNING
	s.publishStatus(string(job.ID), string(domain.JobStatusRunning))

	// 1. Create Workspace
	wsPath, err := s.workspace.PrepareWorkspace(string(job.ID))
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("workspace prep failed: %w", err))
		return
	}
	s.logger.Info("workspace prepared", "path", wsPath)
	// Defer cleanup if we want ephemeral workspaces (POLICY: do we keep them? Yes for debugging, maybe reap later)
	// For now, keep them.

	// 2. persist job status RUNNING
	job.Status = domain.JobStatusRunning
	// TODO: repo.SaveJob(ctx, job) -> We need to update Repository interface to support Jobs!

	// 3. Spawn Worker
	workerID, err := s.workerMgr.Spawn(ctx, job.Spec)
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("spawn failed: %w", err))
		return
	}
	s.logger.Info("worker spawned", "worker_id", workerID, "job_id", job.ID)

	// 4. Watch Loop (Wait for completion)
	// In a real system, we'd use the Watchdog API here to poll status or wait for SSE.
	// For this milestone, let's poll HealthCheck until it exits.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute) // Safety timeout

	for {
		select {
		case <-ctx.Done():
			_ = s.workerMgr.Kill(context.Background(), workerID)
			return
		case <-timeout:
			s.logger.Warn("job timed out", "job_id", job.ID)
			_ = s.workerMgr.Kill(ctx, workerID)
			s.failJob(ctx, job, fmt.Errorf("timeout"))
			return
		case <-ticker.C:
			status, err := s.workerMgr.HealthCheck(ctx, workerID)
			if err != nil {
				s.logger.Error("health check failed", "error", err)
				continue
			}

			if status == domain.HealthStatusExited {
				s.logger.Info("job completed", "job_id", job.ID)
				
				// 5. Cleanup
				_ = s.workerMgr.Kill(ctx, workerID) // Ensure it's gone
				
				job.Status = domain.JobStatusCompleted
				s.publishStatus(string(job.ID), string(domain.JobStatusCompleted))
				// repo.SaveJob(ctx, job)
				return
			}
		}
	}
}

func (s *WorkerLifecycle) failJob(ctx context.Context, job domain.Job, err error) {
	s.logger.Error("job failed", "job_id", job.ID, "error", err)
	job.Status = domain.JobStatusFailed
	msg := err.Error()
	job.Error = &msg
	s.publishStatus(string(job.ID), string(domain.JobStatusFailed))
	// repo.SaveJob(ctx, job)
}

// SubmitJob creates a job record and submits it
func (s *WorkerLifecycle) SubmitJob(ctx context.Context, spec domain.WorkerSpec) (domain.JobID, error) {
	id := domain.JobID(uuid.New().String())
	job := domain.Job{
		ID:        id,
		Spec:      spec,
		Status:    domain.JobStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// repo.SaveJob(ctx, job)

	if err := s.scheduler.SubmitJob(ctx, job); err != nil {
		return "", err
	}
	return id, nil
}
