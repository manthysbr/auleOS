package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"golang.org/x/sync/errgroup"
)

// WorkflowRepository interface for persistence
type WorkflowRepository interface {
	GetWorkflow(ctx context.Context, id domain.WorkflowID) (*domain.Workflow, error)
	SaveWorkflow(ctx context.Context, wf *domain.Workflow) error
}

// WorkflowExecutor manages the execution of workflows
type WorkflowExecutor struct {
	logger *slog.Logger
	repo   WorkflowRepository
	agent  *ReActAgentService // To execute steps
}

func NewWorkflowExecutor(logger *slog.Logger, repo WorkflowRepository, agent *ReActAgentService) *WorkflowExecutor {
	return &WorkflowExecutor{
		logger: logger,
		repo:   repo,
		agent:  agent,
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

	// Launch execution in background (or foreground if synchronous)
	// For M12, let's run synchronously in the request content for simplicity,
	// or ideally launch a goroutine. Given the complexity, a goroutine is better,
	// but we need to handle context.
	// We'll run it synchronously for the PoC, or return "Started" and run async.
	// Let's go with ASYNC to allow interrupts.
	go e.runLoop(context.Background(), wf.ID)

	return nil
}

// runLoop is the main DAG execution loop
func (e *WorkflowExecutor) runLoop(ctx context.Context, id domain.WorkflowID) {
	e.logger.Info("starting workflow execution loop", "workflow_id", id)

	wf, err := e.repo.GetWorkflow(ctx, id)
	if err != nil {
		e.logger.Error("failed to load workflow", "error", err)
		return
	}

	// 1. Check for Pending Steps that depend only on Completed steps
	for {
		// Reload state on each iteration
		wf, err = e.repo.GetWorkflow(ctx, id)
		if err != nil {
			return
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
			if step.Status == domain.StepStatusFailed || step.Status == domain.StepStatusCancelled {
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
			// No runnable steps? Check if we are waiting for running steps
			runningCount := 0
			for _, step := range wf.Steps {
				if step.Status == domain.StepStatusRunning {
					runningCount++
				}
			}
			if runningCount > 0 {
				// Wait a bit and retry (polling for now, ideally channels)
				time.Sleep(1 * time.Second)
				continue
			}
			// Deadlock or finished?
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
		
		// Wait for this batch triggers to *start* or complete?
		// Actually, if we launch them, they become RUNNING.
		// We should mark them RUNNING synchronously before launching.
		// Refactor: executeStep handles the transition.
		// For simplicity in this loop, we wait for the batch to finish? 
		// No, true DAG engine allows pipelining. 
		// BUT `executeStep` updates the DB.
		// Let's just wait for the group to ensure state consistency for this iteration.
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
			// Dependency not found? Fail safe
			return false
		}
	}
	return true
}

func (e *WorkflowExecutor) executeStep(ctx context.Context, wfID domain.WorkflowID, stepIdx int) error {
	// 1. Load Workflow (to modify safe copy)
	// Needs mutex/locking in real impl. For now reliance on atomic updates or loose consistency.
	wf, err := e.repo.GetWorkflow(ctx, wfID)
	if err != nil {
		return err
	}
	step := &wf.Steps[stepIdx] // Ptr to mutable

	// 2. Check Interrupt (Before)
	if step.Interrupt != nil && step.Interrupt.Before {
		wf.Status = domain.WorkflowStatusPaused
		// We'd need to emit an event here.
		e.logger.Info("workflow interrupted before step", "step", step.ID)
		e.repo.SaveWorkflow(ctx, wf)
		return nil // Stop execution path
	}

	// 3. Mark Running
	step.Status = domain.StepStatusRunning
	now := time.Now()
	step.StartedAt = &now
	if err := e.repo.SaveWorkflow(ctx, wf); err != nil {
		return err
	}

	// 4. Interpolate Prompt
	prompt := interpolate(step.Prompt, wf.State)

	// 5. Execute Agent
	// Create a temporary conversation for this step context?
	// Or use a "headless" execution mode?
	// ReActAgentService.Chat expects a conversation.
	// We'll create a transient conversation for this step.
	// Step ID as Conversation ID? Or new UUID.
	convID := domain.ConversationID(fmt.Sprintf("%s-%s", wf.ID, step.ID))
	
	// We need to inject the "Shared State" into the prompt context too?
	// "Shared State: ..."
	
	resp, _, err := e.agent.Chat(ctx, convID, prompt, &step.PersonaID)
	
	// Reload WF to minimize race condition window
	wf, _ = e.repo.GetWorkflow(ctx, wfID)
	step = &wf.Steps[stepIdx] // Reset ptr

	if err != nil {
		step.Status = domain.StepStatusFailed
		msg := err.Error()
		step.Error = &msg
		e.repo.SaveWorkflow(ctx, wf)
		return err
	}

	// 6. Update Result
	step.Result = &domain.StepResult{
		Output: resp.Response,
	}
	
	// 7. Update Shared State (naive: put output into state["stepID"])
	if wf.State == nil {
		wf.State = make(map[string]any)
	}
	wf.State[step.ID] = resp.Response // Simple string output
	// If output is JSON, could parse it? For now string.

	step.Status = domain.StepStatusDone
	finished := time.Now()
	step.CompletedAt = &finished

	return e.repo.SaveWorkflow(ctx, wf)
}

func (e *WorkflowExecutor) failWorkflow(ctx context.Context, wf *domain.Workflow, reason string) {
	wf.Status = domain.WorkflowStatusFailed
	wf.Error = &reason
	finished := time.Now()
	wf.CompletedAt = &finished
	e.repo.SaveWorkflow(ctx, wf)
}

func (e *WorkflowExecutor) completeWorkflow(ctx context.Context, wf *domain.Workflow) {
	wf.Status = domain.WorkflowStatusCompleted
	finished := time.Now()
	wf.CompletedAt = &finished
	e.repo.SaveWorkflow(ctx, wf)
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
