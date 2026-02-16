package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"golang.org/x/sync/semaphore"
)

// SchedulerConfig defines concurrency limits
type SchedulerConfig struct {
	MaxConcurrentJobs int64
	MaxCPU            float64
	MaxMemory         int64
}

type JobScheduler struct {
	logger        *slog.Logger
	pendingQueue  chan domain.Job
	semaphore     *semaphore.Weighted
	
	// Real implementation would track resource usage more granularly
	// For now, we use a simple weighted semaphore based on "1 job = 1 unit"
	// or we can map CPU/Mem to weight. Let's keep it simple for M2: Global Concurrency.
}

func NewJobScheduler(logger *slog.Logger, cfg SchedulerConfig) *JobScheduler {
	// Default to 10 concurrent jobs if not set
	limit := cfg.MaxConcurrentJobs
	if limit <= 0 {
		limit = 10
	}

	return &JobScheduler{
		logger:       logger,
		pendingQueue: make(chan domain.Job, 100), // Buffer
		semaphore:    semaphore.NewWeighted(limit),
	}
}

// SubmitJob adds a job to the scheduling queue
func (s *JobScheduler) SubmitJob(ctx context.Context, job domain.Job) error {
	select {
	case s.pendingQueue <- job:
		s.logger.Info("job submitted", "job_id", job.ID)
		return nil
	default:
		return errors.New("scheduling queue full")
	}
}

// StartWorkerPool consumes jobs and executes them using the provided handler
// handler is a function that spawns the worker and waits for it
func (s *JobScheduler) Start(ctx context.Context, handler func(context.Context, domain.Job)) {
	s.logger.Info("starting job scheduler")
	
	// We use a long-running goroutine to consume the queue
	go func() {
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("stopping scheduler")
				return
			case job := <-s.pendingQueue:
				// Acquire semaphore
				if err := s.semaphore.Acquire(ctx, 1); err != nil {
					s.logger.Error("failed to acquire semaphore", "error", err)
					return
				}

				// Launch job in background so we don't block the consumer loop
				go func(j domain.Job) {
					defer s.semaphore.Release(1)
					handler(ctx, j)
				}(job)
			}
		}
	}()
}
