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
			"capability": CapabilityImageGenerate,
			"tool":       "generate_image",
			"prompt":     prompt,
			"attempt":    "1",
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
			"capability": CapabilityTextGenerate,
			"tool":       "generate_text",
			"prompt":     prompt,
			"attempt":    "1",
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

// TestLLM sends a minimal request to verify LLM connectivity.
func (wl *WorkerLifecycle) TestLLM(ctx context.Context) (string, error) {
	return wl.llm.GenerateText(ctx, "Reply with exactly: ok")
}
