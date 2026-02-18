package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/manthysbr/auleOS/internal/core/domain"
)

func NewCreateWorkflowTool(repo WorkflowRepository) *domain.Tool {
	return &domain.Tool{
		Name:        "create_workflow",
		Description: "Creates a new multi-step workflow definition. Returns the Workflow ID.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
				"description": map[string]interface{}{
					"type": "string",
				},
				"project_id": map[string]interface{}{
					"type": "string",
				},
				"steps": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type": "string",
							},
							"persona_id": map[string]interface{}{
								"type": "string",
							},
							"prompt": map[string]interface{}{
								"type": "string",
							},
							"depends_on": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
							},
							"tools": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
							},
						},
						"required": []string{"id", "prompt"},
					},
				},
			},
			Required: []string{"name", "steps"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			name, _ := params["name"].(string)
			desc, _ := params["description"].(string)
			projectID, _ := params["project_id"].(string)

			// Resolve Project ID from context if missing
			if projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
			}

			// Parse steps manually from interface{} map
			stepsRaw, ok := params["steps"].([]interface{})
			if !ok {
				return nil, fmt.Errorf("steps must be an array")
			}

			var steps []domain.WorkflowStep
			for _, sRaw := range stepsRaw {
				sMap, ok := sRaw.(map[string]interface{})
				if !ok {
					continue
				}

				id, _ := sMap["id"].(string)
				prompt, _ := sMap["prompt"].(string)
				personaID, _ := sMap["persona_id"].(string)

				var dependsOn []string
				if deps, ok := sMap["depends_on"].([]interface{}); ok {
					for _, d := range deps {
						dependsOn = append(dependsOn, fmt.Sprint(d))
					}
				}

				var tools []string
				if ts, ok := sMap["tools"].([]interface{}); ok {
					for _, t := range ts {
						tools = append(tools, fmt.Sprint(t))
					}
				}

				steps = append(steps, domain.WorkflowStep{
					ID:        id,
					PersonaID: domain.PersonaID(personaID),
					Prompt:    prompt,
					Tools:     tools,
					DependsOn: dependsOn,
					Status:    domain.StepStatusPending,
				})
			}

			wf := &domain.Workflow{
				ID:          domain.WorkflowID(uuid.New().String()),
				ProjectID:   domain.ProjectID(projectID),
				Name:        name,
				Description: desc,
				Steps:       steps,
				Status:      domain.WorkflowStatusPending,
				CreatedAt:   time.Now(),
				State:       make(map[string]any),
			}

			if err := repo.SaveWorkflow(ctx, wf); err != nil {
				return nil, fmt.Errorf("failed to save workflow: %w", err)
			}

			return fmt.Sprintf("Workflow created with ID: %s", wf.ID), nil
		},
	}
}

func NewListWorkflowsTool(repo WorkflowRepository) *domain.Tool {
	return &domain.Tool{
		Name:        "list_workflows",
		Description: "Lists all workflows. Returns name, status, and step count for each workflow.",
		Parameters: domain.ToolParameters{
			Type:       "object",
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			workflows, err := repo.ListWorkflows(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list workflows: %w", err)
			}

			if len(workflows) == 0 {
				return "No workflows found.", nil
			}

			var lines []string
			for _, wf := range workflows {
				lines = append(lines, fmt.Sprintf("- %s (ID: %s) [%s] — %d steps", wf.Name, wf.ID, wf.Status, len(wf.Steps)))
			}
			return fmt.Sprintf("%d workflows:\n%s", len(workflows), strings.Join(lines, "\n")), nil
		},
	}
}

func NewRunWorkflowTool(exec *WorkflowExecutor, repo WorkflowRepository) *domain.Tool {
	return &domain.Tool{
		Name:        "run_workflow",
		Description: "Starts execution of a workflow by ID.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"workflow_id": map[string]interface{}{
					"type": "string",
				},
			},
			Required: []string{"workflow_id"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			wfIDStr, _ := params["workflow_id"].(string)
			if wfIDStr == "" {
				return nil, fmt.Errorf("workflow_id required")
			}
			wfID := domain.WorkflowID(wfIDStr)

			wf, err := repo.GetWorkflow(ctx, wfID)
			if err != nil {
				return nil, fmt.Errorf("workflow not found: %w", err)
			}

			if err := exec.Start(ctx, wf); err != nil {
				return nil, fmt.Errorf("failed to start workflow: %w", err)
			}

			return fmt.Sprintf("Workflow %s started", wfID), nil
		},
	}
}
