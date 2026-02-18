package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"golang.org/x/sync/errgroup"
)

// WorkflowRepository interface for persistence
type WorkflowRepository interface {
	GetWorkflow(ctx context.Context, id domain.WorkflowID) (*domain.Workflow, error)
	SaveWorkflow(ctx context.Context, wf *domain.Workflow) error
	ListWorkflows(ctx context.Context) ([]domain.Workflow, error)
}

// WorkflowExecutor manages the execution of workflows
type WorkflowExecutor struct {
	logger   *slog.Logger
	repo     WorkflowRepository
	agent    *ReActAgentService
	eventBus *EventBus
	tracer   *TraceCollector // optional; nil-safe
	mu       sync.Mutex      // protects concurrent step execution writes

	// resumeCh is used to signal resume after interrupt, keyed by workflow ID
	resumeChans   map[domain.WorkflowID]chan struct{}
	resumeChansMu sync.Mutex
}

func NewWorkflowExecutor(logger *slog.Logger, repo WorkflowRepository, agent *ReActAgentService, eventBus *EventBus, tracer *TraceCollector) *WorkflowExecutor {
	return &WorkflowExecutor{
		logger:      logger,
		repo:        repo,
		agent:       agent,
		eventBus:    eventBus,
		tracer:      tracer,
		resumeChans: make(map[domain.WorkflowID]chan struct{}),
	}
}

// Start initiates a workflow execution
func (e *WorkflowExecutor) Start(ctx context.Context, wf *domain.Workflow) error {
	wf.Status = domain.WorkflowStatusRunning
	now := time.Now()
	wf.StartedAt = &now
	if err := e.repo.SaveWorkflow(ctx, wf); err != nil {
		return fmt.Errorf("failed to start workflow: %w", err)
	}

	e.emitEvent(wf.ID, "workflow.started", map[string]any{
		"workflow_id": wf.ID,
		"name":        wf.Name,
		"steps":       len(wf.Steps),
	})

	// Start a trace for the whole workflow execution (background context so it outlives request)
	runCtx := context.Background()
	if e.tracer != nil {
		var traceID domain.TraceID
		runCtx, traceID, _ = e.tracer.StartTrace(runCtx, "workflow: "+wf.Name, map[string]string{
			"workflow_id": string(wf.ID),
		})
		_ = traceID
	}

	go e.runLoop(runCtx, wf.ID)

	return nil
}

// Resume resumes a paused workflow after human approval
func (e *WorkflowExecutor) Resume(ctx context.Context, wfID domain.WorkflowID) error {
	wf, err := e.repo.GetWorkflow(ctx, wfID)
	if err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}
	if wf.Status != domain.WorkflowStatusPaused {
		return fmt.Errorf("workflow is not paused (status: %s)", wf.Status)
	}

	// Find the interrupted step and advance it
	for i := range wf.Steps {
		step := &wf.Steps[i]
		if step.Status == domain.StepStatusPending && step.Interrupt != nil && step.Interrupt.Before {
			// Before-interrupt: mark step ready to run, the runLoop will pick it up
			// We leave status as Pending — runLoop will re-evaluate
			break
		}
		if step.Status == domain.StepStatusDone && step.Interrupt != nil && step.Interrupt.After {
			// After-interrupt: step already done, just need to continue the loop
			break
		}
	}

	wf.Status = domain.WorkflowStatusRunning
	if err := e.repo.SaveWorkflow(ctx, wf); err != nil {
		return err
	}

	e.emitEvent(wfID, "workflow.resumed", map[string]any{"workflow_id": wfID})

	// Signal the resume channel if a goroutine is waiting
	e.resumeChansMu.Lock()
	ch, ok := e.resumeChans[wfID]
	e.resumeChansMu.Unlock()
	if ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	} else {
		// No goroutine waiting — restart the loop
		go e.runLoop(context.Background(), wfID)
	}

	return nil
}

// Cancel cancels a running or paused workflow
func (e *WorkflowExecutor) Cancel(ctx context.Context, wfID domain.WorkflowID) error {
	wf, err := e.repo.GetWorkflow(ctx, wfID)
	if err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	wf.Status = domain.WorkflowStatusCancelled
	now := time.Now()
	wf.CompletedAt = &now

	// Cancel pending steps
	for i := range wf.Steps {
		if wf.Steps[i].Status == domain.StepStatusPending || wf.Steps[i].Status == domain.StepStatusRunning {
			wf.Steps[i].Status = domain.StepStatusCancelled
		}
	}

	if err := e.repo.SaveWorkflow(ctx, wf); err != nil {
		return err
	}

	e.emitEvent(wfID, "workflow.cancelled", map[string]any{"workflow_id": wfID})

	// Signal resume channel to unblock any waiting goroutine
	e.resumeChansMu.Lock()
	if ch, ok := e.resumeChans[wfID]; ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	e.resumeChansMu.Unlock()

	return nil
}

// runLoop is the main DAG execution loop
func (e *WorkflowExecutor) runLoop(ctx context.Context, id domain.WorkflowID) {
	e.logger.Info("starting workflow execution loop", "workflow_id", id)

	// Create resume channel for this workflow
	resumeCh := make(chan struct{}, 1)
	e.resumeChansMu.Lock()
	e.resumeChans[id] = resumeCh
	e.resumeChansMu.Unlock()

	defer func() {
		e.resumeChansMu.Lock()
		delete(e.resumeChans, id)
		e.resumeChansMu.Unlock()
	}()

	for {
		wf, err := e.repo.GetWorkflow(ctx, id)
		if err != nil {
			e.logger.Error("failed to load workflow", "error", err)
			return
		}

		if wf.Status == domain.WorkflowStatusPaused {
			e.logger.Info("workflow paused, waiting for resume", "workflow_id", id)
			// Block until resume signal
			<-resumeCh
			// Re-check status
			wf, err = e.repo.GetWorkflow(ctx, id)
			if err != nil {
				return
			}
			if wf.Status != domain.WorkflowStatusRunning {
				e.logger.Info("workflow not running after resume signal", "status", wf.Status)
				return
			}
		}

		if wf.Status != domain.WorkflowStatusRunning {
			e.logger.Info("workflow not running", "status", wf.Status)
			return
		}

		// Identify runnable steps
		runnableSteps := []int{}
		allDone := true
		anyFailed := false

		for i, step := range wf.Steps {
			if step.Status == domain.StepStatusFailed {
				anyFailed = true
			}
			if step.Status != domain.StepStatusDone && step.Status != domain.StepStatusSkipped {
				allDone = false
			}

			if step.Status == domain.StepStatusPending {
				if e.canRun(step, wf) {
					runnableSteps = append(runnableSteps, i)
				}
			}
		}

		if anyFailed {
			e.failWorkflow(ctx, wf, "One or more steps failed")
			return
		}

		if allDone {
			e.completeWorkflow(ctx, wf)
			return
		}

		if len(runnableSteps) == 0 {
			// Check if we are waiting for running steps
			runningCount := 0
			for _, step := range wf.Steps {
				if step.Status == domain.StepStatusRunning {
					runningCount++
				}
			}
			if runningCount > 0 {
				time.Sleep(1 * time.Second)
				continue
			}
			if !allDone {
				e.failWorkflow(ctx, wf, "Deadlock detected: no runnable steps and not all done")
				return
			}
		}

		// Run runnable steps in parallel
		var g errgroup.Group
		for _, idx := range runnableSteps {
			idx := idx // capture
			g.Go(func() error {
				return e.executeStep(ctx, wf.ID, idx)
			})
		}
		_ = g.Wait()
	}
}

func (e *WorkflowExecutor) canRun(step domain.WorkflowStep, wf *domain.Workflow) bool {
	if len(step.DependsOn) == 0 {
		return true
	}
	for _, depID := range step.DependsOn {
		found := false
		for _, other := range wf.Steps {
			if other.ID == depID {
				found = true
				if other.Status != domain.StepStatusDone {
					return false
				}
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (e *WorkflowExecutor) executeStep(ctx context.Context, wfID domain.WorkflowID, stepIdx int) error {
	// Lock for safe read-modify-write
	e.mu.Lock()
	wf, err := e.repo.GetWorkflow(ctx, wfID)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	step := &wf.Steps[stepIdx]

	// Check Before-Interrupt
	if step.Interrupt != nil && step.Interrupt.Before {
		wf.Status = domain.WorkflowStatusPaused
		e.repo.SaveWorkflow(ctx, wf)
		e.mu.Unlock()

		e.emitEvent(wfID, "step.interrupted", map[string]any{
			"step_id": step.ID,
			"phase":   "before",
			"message": step.Interrupt.Message,
		})
		e.logger.Info("workflow interrupted before step", "step", step.ID)
		return nil
	}

	// --- Tracing: start a span for this step execution ---
	var spanID domain.SpanID
	if e.tracer != nil {
		ctx, spanID = e.tracer.StartSpan(ctx, "step."+step.ID, domain.SpanKindStep, map[string]string{
			"workflow_id": string(wfID),
			"step_id":     step.ID,
		})
		e.tracer.SetSpanInput(spanID, step.Prompt)
	}

	// Mark Running
	step.Status = domain.StepStatusRunning
	now := time.Now()
	step.StartedAt = &now
	e.repo.SaveWorkflow(ctx, wf)
	e.mu.Unlock()

	e.emitEvent(wfID, "step.started", map[string]any{
		"step_id":    step.ID,
		"step_index": stepIdx,
	})

	// Interpolate Prompt
	prompt := interpolate(step.Prompt, wf.State)

	// Execute Agent
	convID := domain.ConversationID(fmt.Sprintf("wf-%s-%s", wfID, step.ID))

	startTime := time.Now()
	resp, _, agentErr := e.agent.Chat(ctx, convID, prompt, &step.PersonaID)
	duration := time.Since(startTime)

	// Lock again for result write
	e.mu.Lock()
	wf, _ = e.repo.GetWorkflow(ctx, wfID)
	step = &wf.Steps[stepIdx]

	if agentErr != nil {
		step.Status = domain.StepStatusFailed
		msg := agentErr.Error()
		step.Error = &msg
		e.repo.SaveWorkflow(ctx, wf)
		e.mu.Unlock()

		if e.tracer != nil {
			e.tracer.EndSpan(spanID, domain.SpanStatusError, "", agentErr.Error())
		}
		e.emitEvent(wfID, "step.failed", map[string]any{
			"step_id": step.ID,
			"error":   msg,
		})
		return agentErr
	}

	// Update Result
	step.Result = &domain.StepResult{
		Output: resp.Response,
		Metadata: map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
			"iterations":  len(resp.Steps),
		},
	}

	// Update Shared State
	if wf.State == nil {
		wf.State = make(map[string]any)
	}
	wf.State[step.ID] = resp.Response

	step.Status = domain.StepStatusDone
	finished := time.Now()
	step.CompletedAt = &finished

	// Check After-Interrupt
	if step.Interrupt != nil && step.Interrupt.After {
		wf.Status = domain.WorkflowStatusPaused
		e.repo.SaveWorkflow(ctx, wf)
		e.mu.Unlock()

		if e.tracer != nil {
			e.tracer.EndSpan(spanID, domain.SpanStatusOK, resp.Response, "")
		}
		e.emitEvent(wfID, "step.interrupted", map[string]any{
			"step_id": step.ID,
			"phase":   "after",
			"message": step.Interrupt.Message,
			"output":  resp.Response,
		})
		e.logger.Info("workflow interrupted after step", "step", step.ID)
		return nil
	}

	e.repo.SaveWorkflow(ctx, wf)
	e.mu.Unlock()

	if e.tracer != nil {
		e.tracer.EndSpan(spanID, domain.SpanStatusOK, resp.Response, "")
	}
	e.emitEvent(wfID, "step.completed", map[string]any{
		"step_id":     step.ID,
		"duration_ms": duration.Milliseconds(),
	})
	return nil
}

func (e *WorkflowExecutor) failWorkflow(ctx context.Context, wf *domain.Workflow, reason string) {
	wf.Status = domain.WorkflowStatusFailed
	wf.Error = &reason
	finished := time.Now()
	wf.CompletedAt = &finished
	e.repo.SaveWorkflow(ctx, wf)

	e.emitEvent(wf.ID, "workflow.failed", map[string]any{
		"workflow_id": wf.ID,
		"error":       reason,
	})
}

func (e *WorkflowExecutor) completeWorkflow(ctx context.Context, wf *domain.Workflow) {
	wf.Status = domain.WorkflowStatusCompleted
	finished := time.Now()
	wf.CompletedAt = &finished
	e.repo.SaveWorkflow(ctx, wf)

	e.emitEvent(wf.ID, "workflow.completed", map[string]any{
		"workflow_id": wf.ID,
		"state_keys":  len(wf.State),
	})
}

// emitEvent publishes a workflow/step event through the EventBus
func (e *WorkflowExecutor) emitEvent(wfID domain.WorkflowID, eventType string, data map[string]any) {
	if e.eventBus == nil {
		return
	}

	payload, _ := json.Marshal(data)
	e.eventBus.Publish(Event{
		JobID:     string(wfID),
		Type:      EventType(eventType),
		Data:      string(payload),
		Timestamp: time.Now().UnixMilli(),
	})
}

// interpolate replaces {{state.key}} with values
func interpolate(template string, state map[string]any) string {
	res := template
	for k, v := range state {
		placeholder := fmt.Sprintf("{{state.%s}}", k)
		valStr := fmt.Sprintf("%v", v)
		res = strings.ReplaceAll(res, placeholder, valStr)
	}
	return res
}
