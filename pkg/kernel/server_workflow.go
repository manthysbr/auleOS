package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/manthysbr/auleOS/internal/core/domain"
)

// CreateWorkflow implements StrictServerInterface
func (s *Server) CreateWorkflow(ctx context.Context, request CreateWorkflowRequestObject) (CreateWorkflowResponseObject, error) {
	req := request.Body
	id := domain.WorkflowID(uuid.New().String())

	steps := make([]domain.WorkflowStep, len(req.Steps))
	for i, stepReq := range req.Steps {
		stepID := uuid.New().String()
		if stepReq.Id != nil {
			stepID = *stepReq.Id
		}
		
		var personaID domain.PersonaID
		if stepReq.PersonaId != nil {
			personaID = domain.PersonaID(*stepReq.PersonaId)
		}

		var tools []string
		if stepReq.Tools != nil {
			tools = *stepReq.Tools
		}

		var dependsOn []string
		if stepReq.DependsOn != nil {
			dependsOn = *stepReq.DependsOn
		}

		var interrupt *domain.InterruptRule
		if stepReq.Interrupt != nil {
			interrupt = &domain.InterruptRule{
				Before:  false,
				After:   false,
				Message: "",
			}
			if stepReq.Interrupt.Before != nil {
				interrupt.Before = *stepReq.Interrupt.Before
			}
			if stepReq.Interrupt.After != nil {
				interrupt.After = *stepReq.Interrupt.After
			}
			if stepReq.Interrupt.Message != nil {
				interrupt.Message = *stepReq.Interrupt.Message
			}
		}

		steps[i] = domain.WorkflowStep{
			ID:        stepID,
			PersonaID: personaID,
			Prompt:    *stepReq.Prompt,
			Tools:     tools,
			DependsOn: dependsOn,
			Interrupt: interrupt,
			Status:    domain.StepStatusPending,
		}
	}

	wf := &domain.Workflow{
		ID:          id,
		Name:        req.Name,
		Steps:       steps,
		Status:      domain.WorkflowStatusPending,
		CreatedAt:   time.Now(),
		State:       make(map[string]any),
	}

	if req.ProjectId != nil {
		wf.ProjectID = domain.ProjectID(*req.ProjectId)
	}
	if req.Description != nil {
		wf.Description = *req.Description
	}

	if err := s.workflowExec.Start(ctx, wf); err != nil { // Start saves it
		s.logger.Error("failed to create workflow", "error", err)
		return nil, fmt.Errorf("internal error")
	}

	status := WorkflowStatus(wf.Status)
	return CreateWorkflow201JSONResponse{
		Id:          toPtr(string(wf.ID)),
		Name:        toPtr(wf.Name),
		Status:      &status,
		CreatedAt:   &wf.CreatedAt,
	}, nil
}

// GetWorkflow implements StrictServerInterface
func (s *Server) GetWorkflow(ctx context.Context, request GetWorkflowRequestObject) (GetWorkflowResponseObject, error) {
	wf, err := s.repo.GetWorkflow(ctx, domain.WorkflowID(request.Id))
	if err != nil {
		return GetWorkflow404Response{}, nil
	}

	// Map domain to API response...
	apiSteps := make([]WorkflowStep, len(wf.Steps))
	for i, step := range wf.Steps {
		stepStatus := WorkflowStepStatus(step.Status)
		apiSteps[i] = WorkflowStep{
			Id:        toPtr(step.ID),
			Prompt:    toPtr(step.Prompt),
			Status:    &stepStatus,
			DependsOn: &step.DependsOn,
			Tools:     &step.Tools,
		}
		if step.PersonaID != "" {
			pid := string(step.PersonaID)
			apiSteps[i].PersonaId = &pid
		}
		if step.Result != nil {
			apiSteps[i].Result = &struct {
				Metadata *map[string]interface{} `json:"metadata,omitempty"`
				Output   *string                 `json:"output,omitempty"`
			}{
				Output:   &step.Result.Output,
				Metadata: &step.Result.Metadata,
			}
		}
	}

	wfStatus := WorkflowStatus(wf.Status)
	return GetWorkflow200JSONResponse{
		Id:          toPtr(string(wf.ID)),
		Name:        toPtr(wf.Name),
		Description: toPtr(wf.Description),
		Status:      &wfStatus,
		State:       &wf.State,
		Steps:       &apiSteps,
		CreatedAt:   &wf.CreatedAt,
		StartedAt:   wf.StartedAt,
		CompletedAt: wf.CompletedAt,
		Error:       wf.Error,
	}, nil
}

// RunWorkflow implements StrictServerInterface
func (s *Server) RunWorkflow(ctx context.Context, request RunWorkflowRequestObject) (RunWorkflowResponseObject, error) {
	wf, err := s.repo.GetWorkflow(ctx, domain.WorkflowID(request.Id))
	if err != nil {
		return RunWorkflow404Response{}, nil
	}

	if err := s.workflowExec.Start(ctx, wf); err != nil {
		return nil, err
	}

	// This endpoint returns generic object with status string, not enum type in spec?
	// Spec:
	// properties:
	//   id: type: string
	//   status: type: string
	// So toPtr(string(wf.Status)) is correct here since schema says type: string, not $ref: '#/components/schemas/WorkflowStatus'
	// Wait, let's check spec again.
	// Generated code types?
	// RunWorkflow200JSONResponse content...
	
	// Let's assume it generated an anonymous struct. 
	// If generated struct has Status *string, then toPtr(string(wf.Status)) is correct.
	// Build error said: "cannot use toPtr(string(wf.Status)) (value of type *string) as *WorkflowStatus value in struct literal" at line 136?
	// Line 136 was likely GetWorkflow return or CreateWorkflow return. 
	// RunWorkflow was later.
	
	// I'll stick to string pointer here if spec defined it as string.
	// If it defined ref, I need cast.
	// Spec: "status: type: string". So it's string.
	
	return RunWorkflow200JSONResponse{
		Id:     toPtr(string(wf.ID)),
		Status: toPtr(string(wf.Status)),
	}, nil
}

// ResumeWorkflow implements StrictServerInterface
func (s *Server) ResumeWorkflow(ctx context.Context, request ResumeWorkflowRequestObject) (ResumeWorkflowResponseObject, error) {
	// Not implemented in executor yet, but handler placeholder
	return ResumeWorkflow200JSONResponse{
		Status: toPtr("resumed"), // Mock
	}, nil
}

func toPtr(s string) *string {
	return &s
}

// ListWorkflows implements StrictServerInterface
func (s *Server) ListWorkflows(ctx context.Context, request ListWorkflowsRequestObject) (ListWorkflowsResponseObject, error) {
	workflows, err := s.repo.ListWorkflows(ctx)
	if err != nil {
		s.logger.Error("failed to list workflows", "error", err)
		return nil, fmt.Errorf("internal error")
	}

	response := make([]Workflow, len(workflows))
	for i, wf := range workflows {
		wfStatus := WorkflowStatus(wf.Status)
		
		// Simplify steps for list view? Spec says return Workflow full object.
		apiSteps := make([]WorkflowStep, len(wf.Steps))
		for j, step := range wf.Steps {
			stepStatus := WorkflowStepStatus(step.Status)
			apiSteps[j] = WorkflowStep{
				Id:        toPtr(step.ID),
				Prompt:    toPtr(step.Prompt),
				Status:    &stepStatus,
				DependsOn: &step.DependsOn,
				Tools:     &step.Tools,
			}
			if step.PersonaID != "" {
				pid := string(step.PersonaID)
				apiSteps[j].PersonaId = &pid
			}
			// Result skipped for list view brevity? Or include?
			// Spec requires Workflow object which has steps.
		}

		response[i] = Workflow{
			Id:          toPtr(string(wf.ID)),
			Name:        toPtr(wf.Name),
			Description: toPtr(wf.Description),
			Status:      &wfStatus,
			State:       &wf.State,
			Steps:       &apiSteps,
			CreatedAt:   &wf.CreatedAt,
			StartedAt:   wf.StartedAt,
			CompletedAt: wf.CompletedAt,
			Error:       wf.Error,
		}
	}

	return ListWorkflows200JSONResponse(response), nil
}

