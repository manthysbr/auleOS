package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/manthysbr/auleOS/internal/config"
	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/services"
)

type Server struct {
	logger       *slog.Logger
	lifecycle    *services.WorkerLifecycle
	reactAgent   *services.ReActAgentService
	agentService *services.AgentService
	eventBus     *services.EventBus
	settings     *config.SettingsStore
	convStore    *services.ConversationStore
	modelRouter  *services.ModelRouter
	discovery    *services.ModelDiscovery
	workerMgr    interface {
		GetLogs(ctx context.Context, id domain.WorkerID) (io.ReadCloser, error)
	}
	repo interface {
		GetJob(ctx context.Context, id domain.JobID) (domain.Job, error)
		ListJobs(ctx context.Context) ([]domain.Job, error)
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
	}
}

// Ensure Server implements StrictServerInterface
var _ StrictServerInterface = (*Server)(nil)

func NewServer(
	logger *slog.Logger,
	lifecycle *services.WorkerLifecycle,
	reactAgent *services.ReActAgentService,
	agentService *services.AgentService,
	eventBus *services.EventBus,
	settings *config.SettingsStore,
	convStore *services.ConversationStore,
	modelRouter *services.ModelRouter,
	discovery *services.ModelDiscovery,
	workerMgr interface {
		GetLogs(ctx context.Context, id domain.WorkerID) (io.ReadCloser, error)
	},
	repo interface {
		GetJob(ctx context.Context, id domain.JobID) (domain.Job, error)
		ListJobs(ctx context.Context) ([]domain.Job, error)
		CreateProject(ctx context.Context, proj domain.Project) error
		GetProject(ctx context.Context, id domain.ProjectID) (domain.Project, error)
		ListProjects(ctx context.Context) ([]domain.Project, error)
		UpdateProject(ctx context.Context, proj domain.Project) error
		DeleteProject(ctx context.Context, id domain.ProjectID) error
		ListProjectConversations(ctx context.Context, projectID domain.ProjectID) ([]domain.Conversation, error)
		SaveArtifact(ctx context.Context, art domain.Artifact) error
		GetArtifact(ctx context.Context, id domain.ArtifactID) (domain.Artifact, error)
		ListArtifacts(ctx context.Context) ([]domain.Artifact, error)
		ListProjectArtifacts(ctx context.Context, projectID domain.ProjectID) ([]domain.Artifact, error)
		DeleteArtifact(ctx context.Context, id domain.ArtifactID) error
		CreatePersona(ctx context.Context, p domain.Persona) error
		GetPersona(ctx context.Context, id domain.PersonaID) (domain.Persona, error)
		ListPersonas(ctx context.Context) ([]domain.Persona, error)
		UpdatePersona(ctx context.Context, p domain.Persona) error
		DeletePersona(ctx context.Context, id domain.PersonaID) error
	}) *Server {
	return &Server{
		logger:       logger,
		lifecycle:    lifecycle,
		reactAgent:   reactAgent,
		agentService: agentService,
		eventBus:     eventBus,
		settings:     settings,
		convStore:    convStore,
		modelRouter:  modelRouter,
		discovery:    discovery,
		workerMgr:    workerMgr,
		repo:         repo,
	}
}

// Handler returns the http.Handler for the server.
// Mounts generated API routes + custom settings routes on a shared mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Mount generated OpenAPI routes (includes conversations, settings, jobs, agent chat)
	strictHandler := NewStrictHandler(s, nil)
	HandlerFromMux(strictHandler, mux)

	// Wrap with SSE interceptor — our raw HTTP handler takes priority
	// over the generated strict handler for the SSE endpoint.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Intercept SSE endpoint for conversation events
		if r.Method == "GET" && isConversationEventsPath(r.URL.Path) {
			s.handleConversationSSE(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// isConversationEventsPath checks if an URL path matches /v1/conversations/{id}/events
func isConversationEventsPath(path string) bool {
	// Pattern: /v1/conversations/<uuid>/events
	const prefix = "/v1/conversations/"
	const suffix = "/events"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return false
	}
	middle := path[len(prefix) : len(path)-len(suffix)]
	return len(middle) > 0 && !strings.Contains(middle, "/")
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
	EventBus  *services.EventBus
	JobID     string
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
	// ctx := context.Background()

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
	_, err := s.repo.GetJob(ctx, domain.JobID(request.Id))
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

// ListJobs implements StrictServerInterface
func (s *Server) ListJobs(ctx context.Context, request ListJobsRequestObject) (ListJobsResponseObject, error) {
	jobs, err := s.repo.ListJobs(ctx)
	if err != nil {
		s.logger.Error("failed to list jobs", "error", err)
		errMsg := "Internal server error"
		return ListJobs500JSONResponse{Error: &errMsg}, nil
	}

	toPtr := func(s string) *string { return &s }

	response := make([]Job, len(jobs))
	for i, job := range jobs {
		response[i] = Job{
			Id:        toPtr(string(job.ID)),
			Status:    toPtr(string(job.Status)),
			Result:    job.Result,
			Error:     job.Error,
			CreatedAt: &job.CreatedAt,
		}
	}

	return ListJobs200JSONResponse(response), nil
}

// AgentChat implements StrictServerInterface
func (s *Server) AgentChat(ctx context.Context, request AgentChatRequestObject) (AgentChatResponseObject, error) {
	msg := request.Body.Message
	model := "llama3"
	if request.Body.Model != nil {
		model = *request.Body.Model
	}

	var convID domain.ConversationID
	if request.Body.ConversationId != nil {
		convID = domain.ConversationID(*request.Body.ConversationId)
	}

	var personaID *domain.PersonaID
	if request.Body.PersonaId != nil {
		pid := domain.PersonaID(*request.Body.PersonaId)
		personaID = &pid
	}

	var (
		thought        string
		response       string
		conversationID string
		steps          *[]ReActStep
		toolCall       *struct {
			Args *map[string]interface{} `json:"args,omitempty"`
			Name *string                 `json:"name,omitempty"`
		}
	)

	if s.reactAgent != nil {
		reactResp, retConvID, err := s.reactAgent.Chat(ctx, convID, msg, personaID)
		if err != nil {
			s.logger.Error("react agent chat failed", "error", err)
			errMsg := err.Error()
			return AgentChat500JSONResponse{Error: &errMsg}, nil
		}

		response = reactResp.Response
		thought = reactResp.Thought
		conversationID = string(retConvID)

		apiSteps := make([]ReActStep, 0, len(reactResp.Steps))
		for _, step := range reactResp.Steps {
			apiStep := ReActStep{}
			if step.Thought != "" {
				value := step.Thought
				apiStep.Thought = &value
			}
			if step.Action != "" {
				value := step.Action
				apiStep.Action = &value
			}
			if step.ActionInput != nil {
				value := step.ActionInput
				apiStep.ActionInput = &value
			}
			if step.Observation != "" {
				value := step.Observation
				apiStep.Observation = &value
			}
			if step.FinalAnswer != "" {
				value := step.FinalAnswer
				apiStep.FinalAnswer = &value
			}
			if step.IsFinalAnswer {
				value := step.IsFinalAnswer
				apiStep.IsFinalAnswer = &value
			}
			apiSteps = append(apiSteps, apiStep)
		}
		steps = &apiSteps

		if len(reactResp.Steps) > 0 {
			lastStep := reactResp.Steps[len(reactResp.Steps)-1]
			if lastStep.Action != "" {
				args := lastStep.ActionInput
				if lastStep.Observation != "" {
					var observed map[string]interface{}
					if err := json.Unmarshal([]byte(lastStep.Observation), &observed); err == nil {
						args = observed
					}
				}
				name := lastStep.Action
				toolCall = &struct {
					Args *map[string]interface{} `json:"args,omitempty"`
					Name *string                 `json:"name,omitempty"`
				}{
					Name: &name,
					Args: &args,
				}
			}
		}
	} else if s.agentService != nil {
		resp, err := s.agentService.Chat(ctx, msg, model)
		if err != nil {
			s.logger.Error("agent chat failed", "error", err)
			errMsg := err.Error()
			return AgentChat500JSONResponse{Error: &errMsg}, nil
		}

		response = resp.Response
		thought = resp.Thought

		if resp.ToolCall != nil {
			name := resp.ToolCall.Name
			args := resp.ToolCall.Args
			toolCall = &struct {
				Args *map[string]interface{} `json:"args,omitempty"`
				Name *string                 `json:"name,omitempty"`
			}{
				Name: &name,
				Args: &args,
			}
		}
	} else {
		errMsg := "no agent service configured"
		return AgentChat500JSONResponse{Error: &errMsg}, nil
	}

	chatResponse := AgentChat200JSONResponse{
		Response:       &response,
		Steps:          steps,
		Thought:        &thought,
		ToolCall:       toolCall,
		ConversationId: &conversationID,
	}

	return chatResponse, nil
}

// ServeJobFile implements StrictServerInterface
func (s *Server) ServeJobFile(ctx context.Context, request ServeJobFileRequestObject) (ServeJobFileResponseObject, error) {
	path, err := s.lifecycle.GetJobFilePath(request.Id, request.Filename)
	if err != nil {
		s.logger.Error("failed to get job file path", "error", err)
		return ServeJobFile404Response{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		s.logger.Error("failed to open job file", "error", err)
		return ServeJobFile404Response{}, nil
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return ServeJobFile500Response{}, nil
	}

	return ServeJobFile200ApplicationoctetStreamResponse{
		Body:          file,
		ContentLength: stat.Size(),
	}, nil
}
