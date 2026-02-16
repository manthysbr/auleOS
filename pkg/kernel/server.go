package kernel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/services"
)

type Server struct {
	logger    *slog.Logger
	lifecycle *services.WorkerLifecycle
	eventBus  *services.EventBus
	workerMgr interface {
		GetLogs(ctx context.Context, id domain.WorkerID) (io.ReadCloser, error)
	}
	repo      interface { // Minimal repo interface needed for queries
		GetJob(ctx context.Context, id domain.JobID) (domain.Job, error)
	}
}

// Ensure Server implements StrictServerInterface
var _ StrictServerInterface = (*Server)(nil)

func NewServer(
	logger *slog.Logger, 
	lifecycle *services.WorkerLifecycle,
	eventBus *services.EventBus,
	workerMgr interface {
		GetLogs(ctx context.Context, id domain.WorkerID) (io.ReadCloser, error)
	},
	repo interface {
	GetJob(ctx context.Context, id domain.JobID) (domain.Job, error)
}) *Server {
	return &Server{
		logger:    logger,
		lifecycle: lifecycle,
		eventBus:  eventBus,
		workerMgr: workerMgr,
		repo:      repo,
	}
}

// Handler returns the http.Handler for the server
func (s *Server) Handler() http.Handler {
	strictHandler := NewStrictHandler(s, nil)
	return Handler(strictHandler)
}

// SubmitJob implements StrictServerInterface
func (s *Server) SubmitJob(ctx context.Context, request SubmitJobRequestObject) (SubmitJobResponseObject, error) {
	req := request.Body

	// Map request to domain spec
	spec := domain.WorkerSpec{
		Image:   req.Image,
		Command: req.Command,
		Env:     make(map[string]string),
		// Resources: ... map resources if needed
	}
	
	if req.Env != nil {
		for k, v := range *req.Env {
			spec.Env[k] = v
		}
	}

	jobID, err := s.lifecycle.SubmitJob(ctx, spec)
	if err != nil {
		s.logger.Error("failed to submit job", "error", err)
		errMsg := "Failed to submit job: " + err.Error()
		return SubmitJob500JSONResponse{Error: &errMsg}, nil
	}

	// Helpers
	toPtr := func(s string) *string { return &s }

	return SubmitJob201JSONResponse{
		Id:     toPtr(string(jobID)),
		Status: toPtr(string(domain.JobStatusPending)),
	}, nil
}

// GetJob implements StrictServerInterface
func (s *Server) GetJob(ctx context.Context, request GetJobRequestObject) (GetJobResponseObject, error) {
	job, err := s.repo.GetJob(ctx, domain.JobID(request.Id))
	if err != nil {
		if err == domain.ErrJobNotFound {
			msg := "Job not found"
			return GetJob404JSONResponse{Error: &msg}, nil
		}
		s.logger.Error("failed to get job", "error", err)
		return nil, fmt.Errorf("internal error") // Will trigger 500 handler
	}

	toPtr := func(s string) *string { return &s }

	return GetJob200JSONResponse{
		Id:        toPtr(string(job.ID)),
		Status:    toPtr(string(job.Status)),
		Result:    job.Result,
		Error:     job.Error,
		CreatedAt: &job.CreatedAt,
	}, nil
}

// StreamJob implements StrictServerInterface
// StreamJob implements StrictServerInterface
// Note: We bypass the strict interface return type here essentially, 
// because we are hijacking the ResponseWriter for SSE.
// To do this cleanly with oapi-codegen strict middleware, we should return a body-less response or manage it manually.
// However, since we are inside the Strict interface implementation, we don't have direct access to the `http.ResponseWriter` 
// UNLESS we use the manually mounted handler method or middleware.
// 
// CORRECTION: The Generated code passes `ctx` but NOT `ResponseWriter` to strict handlers.
// We must modify `Server` struct to store `EventBus` and use it.
// The issue is: `StrictServerInterface` returns `(StreamJobResponseObject, error)`.
// We can define a custom ResponseObject that implements `VisitStreamJobResponse(w http.ResponseWriter) error`.
// Let's create `StreamJobSSEResponse` that holds the logic.

type StreamJobSSEResponse struct {
	EventBus *services.EventBus
	JobID    string
	WorkerMgr interface {
		GetLogs(ctx context.Context, id domain.WorkerID) (io.ReadCloser, error)
	}
	Repo interface {
		GetJob(ctx context.Context, id domain.JobID) (domain.Job, error)
	}
}

func (r StreamJobSSEResponse) VisitStreamJobResponse(w http.ResponseWriter) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)

	// Verify Job exists first
	ctx := context.Background() // TODO: inherit context from request if possible, but Visist doesn't have it easily.
	// Actually we should check existence BEFORE returning this response.
	// Assume existence checked in handler.

	// 1. Subscribe to Events
	eventCh, unsub := r.EventBus.Subscribe(r.JobID)
	defer unsub()

	// 2. Stream Logs (if running or completed recently)
	// For simplicity in this milestone, we only stream events from bus.
	// Real log streaming from docker logs would need a separate goroutine pumping to the SSE writer.
	
	// Sending initial "connected" event
	fmt.Fprintf(w, "event: connected\ndata: %s\n\n", r.JobID)
	flusher.Flush()

	// Loop
	// We need a context to know when client disconnects. Visit doesn't provide it directly in signature (standard net/http).
	// But `w` usually is linked to request.
	// Ideally we pass context in struct.
	
	for event := range eventCh {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
		flusher.Flush()
		
		// If job finished, we might want to close, but user might want logs.
		// For now, keep open until client disconnects.
	}
	
	return nil
}

func (s *Server) StreamJob(ctx context.Context, request StreamJobRequestObject) (StreamJobResponseObject, error) {
	// 1. Check if job exists
	job, err := s.repo.GetJob(ctx, domain.JobID(request.Id))
	if err != nil {
		if err == domain.ErrJobNotFound {
			return StreamJob404Response{}, nil
		}
		s.logger.Error("failed to get job", "error", err)
		return nil, fmt.Errorf("internal error")
	}

	// 2. Return SSE Response Object
	// We need to extend Server struct to hold EventBus and WorkerMgr to pass them here
	return StreamJobSSEResponse{
		EventBus:  s.eventBus,
		JobID:     request.Id,
		WorkerMgr: s.workerMgr,
		Repo:      s.repo,
	}, nil
}
