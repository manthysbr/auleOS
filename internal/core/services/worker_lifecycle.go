package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/ports"
)

const (
	CapabilityImageGenerate = "image.generate"
	CapabilityTextGenerate  = "text.generate"
)

type capabilityJobHandler func(context.Context, domain.Job)

type WorkerLifecycle struct {
	logger    *slog.Logger
	scheduler *JobScheduler
	workerMgr ports.WorkerManager
	repo      ports.Repository
	workspace *WorkspaceManager
	eventBus  *EventBus
	llm       domain.LLMProvider
	image     domain.ImageProvider
	convStore *ConversationStore // optional: enables async job → chat push
	publicURL string

	handlerMu          sync.RWMutex
	capabilityHandlers map[string]capabilityJobHandler
}

func NewWorkerLifecycle(
	logger *slog.Logger,
	scheduler *JobScheduler,
	mgr ports.WorkerManager,
	repo ports.Repository,
	ws *WorkspaceManager,
	eventBus *EventBus,
	llmProvider domain.LLMProvider,
	imageProvider domain.ImageProvider,
) *WorkerLifecycle {
	publicBaseURL := os.Getenv("AULE_PUBLIC_BASE_URL")
	if publicBaseURL == "" {
		publicBaseURL = "http://localhost:8080"
	}

	lifecycle := &WorkerLifecycle{
		logger:             logger,
		scheduler:          scheduler,
		workerMgr:          mgr,
		repo:               repo,
		workspace:          ws,
		eventBus:           eventBus,
		llm:                llmProvider,
		image:              imageProvider,
		publicURL:          strings.TrimRight(publicBaseURL, "/"),
		capabilityHandlers: map[string]capabilityJobHandler{},
	}

	lifecycle.RegisterCapabilityHandler(CapabilityImageGenerate, lifecycle.executeImageJob)
	lifecycle.RegisterCapabilityHandler(CapabilityTextGenerate, lifecycle.executeTextJob)

	return lifecycle
}

// RegisterCapabilityHandler registers a capability execution handler.
func (s *WorkerLifecycle) RegisterCapabilityHandler(capability string, handler capabilityJobHandler) {
	capability = strings.TrimSpace(capability)
	if capability == "" || handler == nil {
		return
	}

	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()
	s.capabilityHandlers[capability] = handler
}

func (s *WorkerLifecycle) dispatchCapabilityJob(ctx context.Context, job domain.Job) bool {
	if job.Metadata == nil {
		return false
	}

	capability := strings.TrimSpace(job.Metadata["capability"])
	if capability == "" {
		return false
	}

	s.handlerMu.RLock()
	handler, ok := s.capabilityHandlers[capability]
	s.handlerMu.RUnlock()

	if !ok {
		s.failJob(ctx, job, fmt.Errorf("unsupported capability: %s", capability))
		return true
	}

	handler(ctx, job)
	return true
}

// Run starts the scheduler loop
func (s *WorkerLifecycle) Run(ctx context.Context) error {
	s.scheduler.Start(ctx, s.executeJob)
	return nil
}

func (s *WorkerLifecycle) publishStatus(jobID string, status string) {
	s.publishStatusWithProgress(jobID, status, nil)
}

func (s *WorkerLifecycle) publishStatusWithProgress(jobID string, status string, progress *int) {
	payload := map[string]interface{}{
		"status": status,
	}
	if progress != nil {
		payload["progress"] = *progress
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		payloadBytes = []byte(fmt.Sprintf(`{"status": "%s"}`, status))
	}

	s.eventBus.Publish(Event{
		JobID:     jobID,
		Type:      EventTypeStatus,
		Data:      string(payloadBytes),
		Timestamp: time.Now().Unix(),
	})
}

func (s *WorkerLifecycle) publishLog(jobID string, data string) {
	s.eventBus.Publish(Event{
		JobID:     jobID,
		Type:      EventTypeLog,
		Data:      data,
		Timestamp: time.Now().Unix(),
	})
}

// executeJob is the callback for the scheduler
func (s *WorkerLifecycle) executeJob(ctx context.Context, job domain.Job) {
	s.logger.Info("executing job", "job_id", job.ID)

	if s.dispatchCapabilityJob(ctx, job) {
		return
	}

	// Publish RUNNING
	progressStart := 10
	s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusRunning), &progressStart)

	// 1. Prepare Workspace (Project-based or Ephemeral)
	var wsPath string
	var err error

	// If job has project_id, we mount the persistent project workspace
	if projectID, ok := job.Metadata["project_id"]; ok && projectID != "" {
		wsPath, err = s.workspace.PrepareProject(projectID)
		if err != nil {
			s.failJob(ctx, job, fmt.Errorf("project workspace prep failed: %w", err))
			return
		}
		s.logger.Info("project workspace mounted", "path", wsPath, "project_id", projectID)
	} else {
		// Default to ephemeral job workspace
		wsPath, err = s.workspace.PrepareWorkspace(string(job.ID))
		if err != nil {
			s.failJob(ctx, job, fmt.Errorf("workspace prep failed: %w", err))
			return
		}
		s.logger.Info("ephemeral workspace prepared", "path", wsPath)
	}

	// 2. persist job status RUNNING
	job.Status = domain.JobStatusRunning
	if err := s.repo.SaveJob(ctx, job); err != nil {
		s.logger.Error("failed to save job status", "error", err)
	}

	// 3. Spawn Worker
	workerID, err := s.workerMgr.Spawn(ctx, job.Spec)
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("spawn failed: %w", err))
		return
	}
	s.logger.Info("worker spawned", "worker_id", workerID, "job_id", job.ID)

	// Persist worker record so the Workers view shows it immediately
	worker := domain.Worker{
		ID:        workerID,
		Spec:      job.Spec,
		Status:    domain.HealthStatusStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata: map[string]string{
			"job_id": string(job.ID),
		},
	}
	if err := s.repo.SaveWorker(ctx, worker); err != nil {
		s.logger.Warn("failed to persist worker record", "worker_id", workerID, "error", err)
	}

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
			_ = s.repo.UpdateWorkerStatus(ctx, workerID, domain.HealthStatusExited)
			return
		case <-timeout:
			s.logger.Warn("job timed out", "job_id", job.ID)
			_ = s.workerMgr.Kill(ctx, workerID)
			_ = s.repo.UpdateWorkerStatus(ctx, workerID, domain.HealthStatusExited)
			s.failJob(ctx, job, fmt.Errorf("timeout"))
			return
		case <-ticker.C:
			status, err := s.workerMgr.HealthCheck(ctx, workerID)
			if err != nil {
				s.logger.Error("health check failed", "error", err)
				continue
			}

			// Keep DB status in sync with container reality
			_ = s.repo.UpdateWorkerStatus(ctx, workerID, status)

			if status == domain.HealthStatusExited {
				s.logger.Info("job completed", "job_id", job.ID)

				// 5. Cleanup
				_ = s.workerMgr.Kill(ctx, workerID) // Ensure it's gone

				job.Status = domain.JobStatusCompleted
				progressDone := 100
				s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusCompleted), &progressDone)
				if err := s.repo.SaveJob(ctx, job); err != nil {
					s.logger.Error("failed to save job status", "error", err)
				}
				return
			}
		}
	}
}

func (s *WorkerLifecycle) executeImageJob(ctx context.Context, job domain.Job) {
	if s.image == nil {
		s.failJob(ctx, job, fmt.Errorf("image provider not configured"))
		return
	}

	prompt := ""
	if job.Metadata != nil {
		prompt = job.Metadata["prompt"]
	}
	if strings.TrimSpace(prompt) == "" {
		s.failJob(ctx, job, fmt.Errorf("missing prompt metadata for image job"))
		return
	}

	progressStart := 20
	s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusRunning), &progressStart)
	s.publishLog(string(job.ID), "image generation started")

	workspacePath, err := s.workspace.PrepareWorkspace(string(job.ID))
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("workspace prep failed: %w", err))
		return
	}

	job.Status = domain.JobStatusRunning
	job.UpdatedAt = time.Now()
	if err := s.repo.SaveJob(ctx, job); err != nil {
		s.logger.Error("failed to save image job running state", "job_id", job.ID, "error", err)
	}

	rawImageURL, err := s.image.GenerateImage(ctx, prompt)
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("image generation failed: %w", err))
		return
	}
	progressGenerated := 60
	s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusRunning), &progressGenerated)

	imageURLRegex := regexp.MustCompile(`https?://[^\s\)]+`)
	resolvedURL := rawImageURL
	if match := imageURLRegex.FindString(rawImageURL); match != "" {
		resolvedURL = match
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolvedURL, nil)
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("failed creating download request: %w", err))
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("failed downloading generated image: %w", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.failJob(ctx, job, fmt.Errorf("image download failed status=%d body=%s", resp.StatusCode, string(body)))
		return
	}
	progressDownloaded := 80
	s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusRunning), &progressDownloaded)

	attempt := "1"
	if job.Metadata != nil && strings.TrimSpace(job.Metadata["attempt"]) != "" {
		attempt = strings.TrimSpace(job.Metadata["attempt"])
	}
	resultFileName := fmt.Sprintf("result-v%s.png", attempt)
	resultPath := filepath.Join(workspacePath, resultFileName)
	file, err := os.Create(resultPath)
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("failed creating result file: %w", err))
		return
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		s.failJob(ctx, job, fmt.Errorf("failed writing result file: %w", err))
		return
	}

	servedURL := fmt.Sprintf("%s/v1/jobs/%s/files/%s", s.publicURL, job.ID, resultFileName)
	job.Status = domain.JobStatusCompleted
	job.Result = &servedURL
	job.Error = nil
	job.UpdatedAt = time.Now()
	if err := s.repo.SaveJob(ctx, job); err != nil {
		s.logger.Error("failed to save completed image job", "job_id", job.ID, "error", err)
	}

	progressDone := 100
	s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusCompleted), &progressDone)
	s.publishLog(string(job.ID), fmt.Sprintf("image saved: %s", resultPath))

	// Push result back into the originating conversation
	s.notifyConversation(ctx, job, fmt.Sprintf("Here is your generated image:\n\n![Generated Image](%s)", servedURL), &servedURL)
}

func (s *WorkerLifecycle) executeTextJob(ctx context.Context, job domain.Job) {
	if s.llm == nil {
		s.failJob(ctx, job, fmt.Errorf("llm provider not configured"))
		return
	}

	prompt := ""
	if job.Metadata != nil {
		prompt = job.Metadata["prompt"]
	}
	if strings.TrimSpace(prompt) == "" {
		s.failJob(ctx, job, fmt.Errorf("missing prompt metadata for text job"))
		return
	}

	progressStart := 20
	s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusRunning), &progressStart)
	s.publishLog(string(job.ID), "text generation started")

	workspacePath, err := s.workspace.PrepareWorkspace(string(job.ID))
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("workspace prep failed: %w", err))
		return
	}

	job.Status = domain.JobStatusRunning
	job.UpdatedAt = time.Now()
	if err := s.repo.SaveJob(ctx, job); err != nil {
		s.logger.Error("failed to save text job running state", "job_id", job.ID, "error", err)
	}

	resultText, err := s.llm.GenerateText(ctx, prompt)
	if err != nil {
		s.failJob(ctx, job, fmt.Errorf("text generation failed: %w", err))
		return
	}
	progressGenerated := 70
	s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusRunning), &progressGenerated)

	attempt := "1"
	if job.Metadata != nil && strings.TrimSpace(job.Metadata["attempt"]) != "" {
		attempt = strings.TrimSpace(job.Metadata["attempt"])
	}
	resultFileName := fmt.Sprintf("result-v%s.txt", attempt)
	resultPath := filepath.Join(workspacePath, resultFileName)
	if err := os.WriteFile(resultPath, []byte(resultText), 0644); err != nil {
		s.failJob(ctx, job, fmt.Errorf("failed writing result file: %w", err))
		return
	}

	servedURL := fmt.Sprintf("%s/v1/jobs/%s/files/%s", s.publicURL, job.ID, resultFileName)
	job.Status = domain.JobStatusCompleted
	job.Result = &servedURL
	job.Error = nil
	job.UpdatedAt = time.Now()
	if err := s.repo.SaveJob(ctx, job); err != nil {
		s.logger.Error("failed to save completed text job", "job_id", job.ID, "error", err)
	}

	progressDone := 100
	s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusCompleted), &progressDone)
	s.publishLog(string(job.ID), fmt.Sprintf("text saved: %s", resultPath))

	// Push result back into the originating conversation
	s.notifyConversation(ctx, job, fmt.Sprintf("Here is the generated text:\n\n%s", resultText), nil)
}

func (s *WorkerLifecycle) failJob(ctx context.Context, job domain.Job, err error) {
	s.logger.Error("job failed", "job_id", job.ID, "error", err)
	job.Status = domain.JobStatusFailed
	msg := err.Error()
	job.Error = &msg
	job.UpdatedAt = time.Now()
	s.publishStatusWithProgress(string(job.ID), string(domain.JobStatusFailed), nil)
	s.publishLog(string(job.ID), msg)
	if err := s.repo.SaveJob(ctx, job); err != nil {
		s.logger.Error("failed to save job status", "error", err)
	}

	// Notify conversation about the failure too
	s.notifyConversation(ctx, job, fmt.Sprintf("Job failed: %s", msg), nil)
}

// notifyConversation pushes a result message back into the originating conversation
// when a job completes. This enables async tool results to appear in the chat.
func (s *WorkerLifecycle) notifyConversation(ctx context.Context, job domain.Job, content string, imageURL *string) {
	if job.Metadata == nil {
		return
	}
	convID := strings.TrimSpace(job.Metadata["conversation_id"])
	if convID == "" {
		return
	}

	msgID := domain.NewMessageID()
	now := time.Now()

	// Persist the assistant message into the conversation
	if s.convStore != nil {
		msg := domain.Message{
			ID:             msgID,
			ConversationID: domain.ConversationID(convID),
			Role:           domain.RoleAssistant,
			Content:        content,
			CreatedAt:      now,
		}
		if err := s.convStore.AddMessage(ctx, msg); err != nil {
			s.logger.Error("failed to persist job result message", "job_id", job.ID, "conv_id", convID, "error", err)
		}
	}

	// Build SSE payload — includes enough data for the frontend to render immediately
	payload := map[string]interface{}{
		"id":              string(msgID),
		"conversation_id": convID,
		"role":            "assistant",
		"content":         content,
		"job_id":          string(job.ID),
		"created_at":      now.Format(time.RFC3339),
	}
	if imageURL != nil {
		payload["image_url"] = *imageURL
	}
	payloadJSON, _ := json.Marshal(payload)

	// Publish on the conversation channel so the SSE handler picks it up
	s.eventBus.Publish(Event{
		JobID:     convID, // EventBus key = conversation ID
		Type:      EventTypeNewMessage,
		Data:      string(payloadJSON),
		Timestamp: now.Unix(),
	})

	s.logger.Info("job result pushed to conversation", "job_id", job.ID, "conv_id", convID)
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

	if err := s.repo.SaveJob(ctx, job); err != nil {
		return "", fmt.Errorf("failed to save job: %w", err)
	}
	s.publishStatus(string(id), string(domain.JobStatusPending))

	if err := s.scheduler.SubmitJob(ctx, job); err != nil {
		return "", err
	}
	return id, nil
}

// SubmitImageJob creates a queued image job and delegates execution to scheduler/lifecycle.
func (s *WorkerLifecycle) SubmitImageJob(ctx context.Context, prompt string) (domain.JobID, error) {
	return s.SubmitImageJobWithConv(ctx, prompt, "")
}

// SubmitImageJobWithConv creates a queued image job with an optional conversation_id
// so the result can be pushed back into the originating chat.
func (s *WorkerLifecycle) SubmitImageJobWithConv(ctx context.Context, prompt string, convID string) (domain.JobID, error) {
	id := domain.JobID(uuid.New().String())
	now := time.Now()

	job := domain.Job{
		ID: id,
		Spec: domain.WorkerSpec{
			Image:   "comfyui",
			Command: []string{"generate_image"},
			Env:     map[string]string{},
		},
		Status:    domain.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: map[string]string{
			"capability":      CapabilityImageGenerate,
			"tool":            "generate_image",
			"prompt":          prompt,
			"attempt":         "1",
			"conversation_id": convID,
		},
	}

	if err := s.repo.SaveJob(ctx, job); err != nil {
		return "", fmt.Errorf("failed to save image job: %w", err)
	}

	s.publishStatus(string(id), string(domain.JobStatusPending))
	s.publishLog(string(id), "image job queued")

	if err := s.scheduler.SubmitJob(ctx, job); err != nil {
		return "", err
	}

	return id, nil
}

// SubmitTextJob creates a queued text generation job and delegates execution to scheduler/lifecycle.
func (s *WorkerLifecycle) SubmitTextJob(ctx context.Context, prompt string) (domain.JobID, error) {
	return s.SubmitTextJobWithConv(ctx, prompt, "")
}

// SubmitTextJobWithConv creates a queued text job with an optional conversation_id
// so the result can be pushed back into the originating chat.
func (s *WorkerLifecycle) SubmitTextJobWithConv(ctx context.Context, prompt string, convID string) (domain.JobID, error) {
	id := domain.JobID(uuid.New().String())
	now := time.Now()

	job := domain.Job{
		ID: id,
		Spec: domain.WorkerSpec{
			Image:   "llm",
			Command: []string{"generate_text"},
			Env:     map[string]string{},
		},
		Status:    domain.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: map[string]string{
			"capability":      CapabilityTextGenerate,
			"tool":            "generate_text",
			"prompt":          prompt,
			"attempt":         "1",
			"conversation_id": convID,
		},
	}

	if err := s.repo.SaveJob(ctx, job); err != nil {
		return "", fmt.Errorf("failed to save text job: %w", err)
	}

	s.publishStatus(string(id), string(domain.JobStatusPending))
	s.publishLog(string(id), "text job queued")

	if err := s.scheduler.SubmitJob(ctx, job); err != nil {
		return "", err
	}

	return id, nil
}

// GetJobFilePath returns the absolute path to a file in the job's workspace.
// It prevents directory traversal.
func (s *WorkerLifecycle) GetJobFilePath(jobID string, filename string) (string, error) {
	wsPath := s.workspace.GetPath(jobID)
	fullPath := filepath.Join(wsPath, filename)

	// Security check: ensure strictly within workspace
	cleanPath := filepath.Clean(fullPath)
	if filepath.Dir(cleanPath) != filepath.Clean(wsPath) {
		// Strict check: we only allow files in the root of the workspace for now?
		// Or strictly verify prefix.
		// Let's allow subdirs but verify prefix.
	}

	// Better check:
	rel, err := filepath.Rel(wsPath, cleanPath)
	if err != nil || filepath.IsAbs(rel) || (len(rel) > 2 && rel[:2] == "..") {
		return "", fmt.Errorf("invalid file path: directory traversal detected")
	}

	// Verify file exists
	if _, err := os.Stat(cleanPath); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	return cleanPath, nil
}

// GetWorkerIP returns the IP address of the worker for a given job
func (wl *WorkerLifecycle) GetWorkerIP(ctx context.Context, jobID domain.JobID) (string, error) {
	job, err := wl.repo.GetJob(ctx, jobID)
	if err != nil {
		return "", fmt.Errorf("failed to get job: %w", err)
	}

	if job.WorkerID == nil {
		return "", fmt.Errorf("job has no worker assigned")
	}

	return wl.workerMgr.GetWorkerIP(ctx, *job.WorkerID)
}

// UpdateProviders hot-swaps the LLM and Image providers.
// Called when settings change to apply new configuration without restart.
func (wl *WorkerLifecycle) UpdateProviders(llm domain.LLMProvider, img domain.ImageProvider) {
	wl.handlerMu.Lock()
	defer wl.handlerMu.Unlock()
	wl.llm = llm
	wl.image = img
	wl.logger.Info("providers hot-reloaded")
}

// SetConversationStore wires the conversation store so completed jobs
// can push result messages back into the originating chat.
func (wl *WorkerLifecycle) SetConversationStore(cs *ConversationStore) {
	wl.convStore = cs
}

// TestLLM sends a minimal request to verify LLM connectivity.
func (wl *WorkerLifecycle) TestLLM(ctx context.Context) (string, error) {
	return wl.llm.GenerateText(ctx, "Reply with exactly: ok")
}

// TestImageProvider performs a quick connectivity check on the image backend.
// Returns nil if reachable, error otherwise. Does NOT generate an image.
func (wl *WorkerLifecycle) TestImageProvider(ctx context.Context) error {
	if wl.image == nil {
		return fmt.Errorf("no image provider configured")
	}
	// Try a simple TCP-level check by using the image provider's host
	// We'll try a tiny HTTP HEAD to the comfyui/remote endpoint
	// For ComfyUI: GET / returns HTML; for remote APIs: health endpoint
	// Quick approach: attempt to generate with an empty prompt will fail fast
	// but that's heavy. Instead, check if the provider struct has a known host.
	//
	// Simplest: try a 2-second timeout HEAD request to the configured host
	client := &http.Client{Timeout: 2 * time.Second}
	// Detect provider type by checking the configured URL
	comfyHost := os.Getenv("COMFYUI_HOST")
	if comfyHost == "" {
		comfyHost = "http://localhost:8188"
	}
	resp, err := client.Head(comfyHost)
	if err != nil {
		return fmt.Errorf("image backend unreachable at %s: %w", comfyHost, err)
	}
	resp.Body.Close()
	return nil
}
