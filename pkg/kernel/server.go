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
	"time"

	"github.com/google/uuid"
	"github.com/manthysbr/auleOS/internal/config"
	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/services"
	"github.com/manthysbr/auleOS/internal/synapse"
)

type Server struct {
	logger       *slog.Logger
	lifecycle    *services.WorkerLifecycle
	reactAgent   *services.ReActAgentService
	eventBus     *services.EventBus
	settings     *config.SettingsStore
	convStore    *services.ConversationStore
	modelRouter  *services.ModelRouter
	discovery    *services.ModelDiscovery
	capRouter    *services.CapabilityRouter
	synapseRT    *synapse.Runtime
	workflowExec *services.WorkflowExecutor
	tracer       *services.TraceCollector
	toolRegistry *domain.ToolRegistry
	systemChat   *services.SystemChat // optional proactive notification channel
	workerMgr    interface {
		GetLogs(ctx context.Context, id domain.WorkerID) (io.ReadCloser, error)
	}
	repo interface {
		GetJob(ctx context.Context, id domain.JobID) (domain.Job, error)
		ListJobs(ctx context.Context) ([]domain.Job, error)
		SaveJob(ctx context.Context, job domain.Job) error
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
		// Workflows
		GetWorkflow(ctx context.Context, id domain.WorkflowID) (*domain.Workflow, error)
		SaveWorkflow(ctx context.Context, wf *domain.Workflow) error
		ListWorkflows(ctx context.Context) ([]domain.Workflow, error)
		// Scheduled Tasks
		SaveScheduledTask(ctx context.Context, task *domain.ScheduledTask) error
		GetScheduledTask(ctx context.Context, id domain.ScheduledTaskID) (*domain.ScheduledTask, error)
		ListScheduledTasks(ctx context.Context) ([]domain.ScheduledTask, error)
		DeleteScheduledTask(ctx context.Context, id domain.ScheduledTaskID) error
		// Workers
		ListWorkers(ctx context.Context) ([]domain.Worker, error)
	}
}

// Ensure Server implements StrictServerInterface
var _ StrictServerInterface = (*Server)(nil)

func NewServer(
	logger *slog.Logger,
	lifecycle *services.WorkerLifecycle,
	reactAgent *services.ReActAgentService,
	eventBus *services.EventBus,
	settings *config.SettingsStore,
	convStore *services.ConversationStore,
	modelRouter *services.ModelRouter,
	discovery *services.ModelDiscovery,
	capRouter *services.CapabilityRouter,
	synapseRT *synapse.Runtime,
	workflowExec *services.WorkflowExecutor,
	tracer *services.TraceCollector,
	toolRegistry *domain.ToolRegistry,
	workerMgr interface {
		GetLogs(ctx context.Context, id domain.WorkerID) (io.ReadCloser, error)
	},
	repo interface {
		GetJob(ctx context.Context, id domain.JobID) (domain.Job, error)
		ListJobs(ctx context.Context) ([]domain.Job, error)
		SaveJob(ctx context.Context, job domain.Job) error
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
		// Workflows
		GetWorkflow(ctx context.Context, id domain.WorkflowID) (*domain.Workflow, error)
		SaveWorkflow(ctx context.Context, wf *domain.Workflow) error
		ListWorkflows(ctx context.Context) ([]domain.Workflow, error)
		// Scheduled Tasks
		SaveScheduledTask(ctx context.Context, task *domain.ScheduledTask) error
		GetScheduledTask(ctx context.Context, id domain.ScheduledTaskID) (*domain.ScheduledTask, error)
		ListScheduledTasks(ctx context.Context) ([]domain.ScheduledTask, error)
		DeleteScheduledTask(ctx context.Context, id domain.ScheduledTaskID) error
		// Workers
		ListWorkers(ctx context.Context) ([]domain.Worker, error)
	}) *Server {
	return &Server{
		logger:       logger,
		lifecycle:    lifecycle,
		reactAgent:   reactAgent,
		eventBus:     eventBus,
		settings:     settings,
		convStore:    convStore,
		modelRouter:  modelRouter,
		discovery:    discovery,
		capRouter:    capRouter,
		synapseRT:    synapseRT,
		workflowExec: workflowExec,
		tracer:       tracer,
		toolRegistry: toolRegistry,
		workerMgr:    workerMgr,
		repo:         repo,
	}
}

// SetSystemChat wires the proactive kernel notification channel.
func (s *Server) SetSystemChat(sc *services.SystemChat) {
	s.systemChat = sc
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
		// Intercept SSE endpoint for workflow events
		if r.Method == "GET" && isWorkflowEventsPath(r.URL.Path) {
			s.handleWorkflowSSE(w, r)
			return
		}
		// Intercept SSE endpoint for broadcast/global agent events
		if r.Method == "GET" && r.URL.Path == "/v1/events" {
			s.handleBroadcastSSE(w, r)
			return
		}
		// Tracing API — Genkit-style observability
		if r.Method == "GET" && r.URL.Path == "/v1/traces" {
			s.handleListTraces(w, r)
			return
		}
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/traces/") {
			s.handleGetTrace(w, r)
			return
		}
		// Scheduled Tasks API
		if r.Method == "GET" && r.URL.Path == "/v1/tasks" {
			s.handleListTasks(w, r)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/tasks" {
			s.handleCreateTask(w, r)
			return
		}
		if r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/v1/tasks/") && !strings.Contains(strings.TrimPrefix(r.URL.Path, "/v1/tasks/"), "/") {
			s.handleDeleteTask(w, r)
			return
		}
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/tasks/") && strings.HasSuffix(r.URL.Path, "/toggle") {
			s.handleToggleTask(w, r)
			return
		}
		// Workers API
		if r.Method == "GET" && r.URL.Path == "/v1/workers" {
			s.handleListWorkers(w, r)
			return
		}
		// Models API
		if r.Method == "GET" && r.URL.Path == "/v1/models" {
			s.handleListModels(w, r)
			return
		}
		// Tools API — list and execute
		if r.Method == "GET" && r.URL.Path == "/v1/tools" {
			s.handleListTools(w, r)
			return
		}
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/tools/") && strings.HasSuffix(r.URL.Path, "/run") {
			s.handleRunTool(w, r)
			return
		}
		// System inbox — kernel proactive notification channel
		if r.Method == "GET" && r.URL.Path == "/v1/system/inbox" {
			s.handleKernelInbox(w, r)
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

// isWorkflowEventsPath checks if an URL path matches /v1/workflows/{id}/events
func isWorkflowEventsPath(path string) bool {
	const prefix = "/v1/workflows/"
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

// ListPlugins implements StrictServerInterface
func (s *Server) ListPlugins(ctx context.Context, request ListPluginsRequestObject) (ListPluginsResponseObject, error) {
	var plugins []Plugin

	if s.synapseRT != nil {
		for _, name := range s.synapseRT.ListPlugins() {
			if p, ok := s.synapseRT.GetPlugin(name); ok {
				meta := p.Meta()
				plugins = append(plugins, Plugin{
					Name:        &meta.Name,
					Version:     &meta.Version,
					Description: &meta.Description,
					ToolName:    &meta.ToolName,
					Runtime:     toPtr("synapse"),
				})
			}
		}
	}

	if plugins == nil {
		plugins = []Plugin{}
	}

	return ListPlugins200JSONResponse{
		Plugins: &plugins,
		Count:   toPtrInt(len(plugins)),
	}, nil
}

// ListCapabilities implements StrictServerInterface
func (s *Server) ListCapabilities(ctx context.Context, request ListCapabilitiesRequestObject) (ListCapabilitiesResponseObject, error) {
	var caps []Capability

	if s.capRouter != nil {
		for name, route := range s.capRouter.ListRoutes() {
			n := name
			r := string(route.Runtime)
			d := route.Description
			rt := CapabilityRuntime(r) // assuming enum match
			caps = append(caps, Capability{
				Capability:  &n,
				Runtime:     &rt,
				Description: &d,
			})
		}
	}

	if caps == nil {
		caps = []Capability{}
	}

	stats := map[string]int{"total": 0, "muscle": 0, "synapse": 0}
	if s.capRouter != nil {
		stats = s.capRouter.Stats()
	}

	m := stats["muscle"]
	syn := stats["synapse"]
	t := stats["total"]

	return ListCapabilities200JSONResponse{
		Capabilities: &caps,
		Stats: &CapabilityStats{
			Muscle:  &m,
			Synapse: &syn,
			Total:   &t,
		},
	}, nil
}

func toPtrInt(i int) *int {
	return &i
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
		hasToolCalls := false
		for _, step := range reactResp.Steps {
			if step.Action != "" {
				hasToolCalls = true
			}
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

		// Create a Job record for chat operations that involved tool calls
		// This makes agentic work visible in the Jobs view
		if hasToolCalls {
			jobID := domain.JobID("chat-" + conversationID[:min(8, len(conversationID))] + "-" + fmt.Sprintf("%d", time.Now().UnixMilli()))
			toolNames := []string{}
			for _, step := range reactResp.Steps {
				if step.Action != "" {
					toolNames = append(toolNames, step.Action)
				}
			}
			resultStr := response
			if len(resultStr) > 200 {
				resultStr = resultStr[:200] + "..."
			}
			chatJob := domain.Job{
				ID:     jobID,
				Status: domain.JobStatusCompleted,
				Result: &resultStr,
				Spec: domain.WorkerSpec{
					Image: "agent-chat",
					Tags: map[string]string{
						"type": "chat",
					},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Metadata: map[string]string{
					"type":            "agent_chat",
					"conversation_id": conversationID,
					"tools_used":      strings.Join(toolNames, ","),
					"message":         msg,
				},
			}
			if err := s.repo.SaveJob(ctx, chatJob); err != nil {
				s.logger.Warn("failed to save chat job record", "error", err)
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

// handleListPlugins returns the list of loaded Synapse (Wasm) plugins.
// GET /v1/plugins
func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	type pluginInfo struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		ToolName    string `json:"tool_name"`
		Runtime     string `json:"runtime"`
	}

	var plugins []pluginInfo

	if s.synapseRT != nil {
		for _, name := range s.synapseRT.ListPlugins() {
			if p, ok := s.synapseRT.GetPlugin(name); ok {
				meta := p.Meta()
				plugins = append(plugins, pluginInfo{
					Name:        meta.Name,
					Version:     meta.Version,
					Description: meta.Description,
					ToolName:    meta.ToolName,
					Runtime:     "synapse",
				})
			}
		}
	}

	if plugins == nil {
		plugins = []pluginInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// handleListCapabilities returns all registered capability routes.
// GET /v1/capabilities
func (s *Server) handleListCapabilities(w http.ResponseWriter, r *http.Request) {
	type capInfo struct {
		Capability  string `json:"capability"`
		Runtime     string `json:"runtime"`
		Description string `json:"description"`
	}

	var caps []capInfo

	if s.capRouter != nil {
		for name, route := range s.capRouter.ListRoutes() {
			caps = append(caps, capInfo{
				Capability:  name,
				Runtime:     string(route.Runtime),
				Description: route.Description,
			})
		}
	}

	if caps == nil {
		caps = []capInfo{}
	}

	stats := map[string]int{"total": 0, "muscle": 0, "synapse": 0}
	if s.capRouter != nil {
		stats = s.capRouter.Stats()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"capabilities": caps,
		"stats":        stats,
	})
}

// --- Tracing API (Genkit-style observability) ---

// handleListTraces returns recent traces.
// GET /v1/traces?limit=50
func (s *Server) handleListTraces(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := fmt.Sscanf(l, "%d", &limit); n == 1 && err == nil && limit > 0 {
			if limit > 500 {
				limit = 500
			}
		}
	}

	traces := s.tracer.ListTraces(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"traces": traces,
		"count":  len(traces),
	})
}

// handleGetTrace returns a single trace with all spans.
// GET /v1/traces/{id}
func (s *Server) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /v1/traces/{id}
	path := strings.TrimPrefix(r.URL.Path, "/v1/traces/")
	if path == "" || strings.Contains(path, "/") {
		http.Error(w, "invalid trace id", http.StatusBadRequest)
		return
	}

	trace, err := s.tracer.GetTrace(domain.TraceID(path))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trace)
}

// --- Scheduled Tasks API ---

// handleListTasks returns all scheduled tasks.
// GET /v1/tasks
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.repo.ListScheduledTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []domain.ScheduledTask{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// handleCreateTask creates a new scheduled task.
// POST /v1/tasks
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req domain.ScheduledTask
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	req.ID = domain.ScheduledTaskID(uuid.New().String())
	req.CreatedAt = time.Now()
	req.RunCount = 0
	if req.Status == "" {
		req.Status = domain.TaskStatusActive
	}
	if req.Type == "" {
		req.Type = domain.TaskTypeOneShot
	}
	if req.NextRun.IsZero() {
		req.NextRun = time.Now()
	}

	if err := s.repo.SaveScheduledTask(r.Context(), &req); err != nil {
		s.logger.Error("failed to create scheduled task", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

// handleDeleteTask deletes a scheduled task.
// DELETE /v1/tasks/{id}
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
	if id == "" {
		http.Error(w, "missing task id", http.StatusBadRequest)
		return
	}
	if err := s.repo.DeleteScheduledTask(r.Context(), domain.ScheduledTaskID(id)); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleToggleTask toggles a task between active and paused.
// POST /v1/tasks/{id}/toggle
func (s *Server) handleToggleTask(w http.ResponseWriter, r *http.Request) {
	// Path: /v1/tasks/{id}/toggle
	withoutPrefix := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
	id := strings.TrimSuffix(withoutPrefix, "/toggle")
	if id == "" {
		http.Error(w, "missing task id", http.StatusBadRequest)
		return
	}

	task, err := s.repo.GetScheduledTask(r.Context(), domain.ScheduledTaskID(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if task.Status == domain.TaskStatusActive {
		task.Status = domain.TaskStatusPaused
	} else {
		task.Status = domain.TaskStatusActive
	}

	if err := s.repo.SaveScheduledTask(r.Context(), task); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// --- Workers API ---

// handleListWorkers returns all workers from the DB.
// GET /v1/workers
func (s *Server) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	workers, err := s.repo.ListWorkers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workers == nil {
		workers = []domain.Worker{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workers": workers,
		"count":   len(workers),
	})
}

// --- Models API ---

// handleListModels returns the discovered model catalog.
// GET /v1/models
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	models := s.modelRouter.GetCatalog()
	if models == nil {
		models = []domain.ModelSpec{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models": models,
		"count":  len(models),
	})
}

// --- Tools API ---

// toolDTO is the JSON representation of a tool (Execute func is excluded).
type toolDTO struct {
	Name          string                `json:"name"`
	Description   string                `json:"description"`
	Parameters    domain.ToolParameters `json:"parameters"`
	ExecutionType string                `json:"execution_type"`
}

// handleListTools returns all registered tools with their schemas.
// GET /v1/tools
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	if s.toolRegistry == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"tools": []struct{}{}, "count": 0})
		return
	}
	tools := s.toolRegistry.ListTools()
	dtos := make([]toolDTO, 0, len(tools))
	for _, t := range tools {
		dtos = append(dtos, toolDTO{
			Name:          t.Name,
			Description:   t.Description,
			Parameters:    t.Parameters,
			ExecutionType: string(t.ExecutionType),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": dtos,
		"count": len(dtos),
	})
}

// handleRunTool executes a tool by name with the provided JSON params.
// POST /v1/tools/{name}/run
// Body: {"params": {...}}
func (s *Server) handleRunTool(w http.ResponseWriter, r *http.Request) {
	if s.toolRegistry == nil {
		http.Error(w, `{"error":"tool registry not available"}`, http.StatusServiceUnavailable)
		return
	}

	// Parse tool name: /v1/tools/{name}/run
	withoutPrefix := strings.TrimPrefix(r.URL.Path, "/v1/tools/")
	toolName := strings.TrimSuffix(withoutPrefix, "/run")
	if toolName == "" {
		http.Error(w, `{"error":"missing tool name"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		Params map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.Params == nil {
		body.Params = map[string]interface{}{}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	result, err := s.toolRegistry.Execute(ctx, toolName, body.Params)
	elapsed := time.Since(startTime).Milliseconds()

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":          false,
			"tool":        toolName,
			"error":       err.Error(),
			"duration_ms": elapsed,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":          true,
		"tool":        toolName,
		"result":      result,
		"duration_ms": elapsed,
	})
}

// handleKernelInbox returns the system inbox status (conversation ID + unread badge).
// GET /v1/system/inbox
func (s *Server) handleKernelInbox(w http.ResponseWriter, r *http.Request) {
	if s.systemChat == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"conversation_id": string(domain.SystemConversationID),
			"unread_count":    0,
			"last_message":    nil,
		})
		return
	}
	status := s.systemChat.GetStatus(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
