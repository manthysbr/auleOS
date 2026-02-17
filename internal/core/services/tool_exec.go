package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// NewExecTool creates the exec tool
func NewExecTool(lifecycle *WorkerLifecycle, repo interface {
	GetJob(ctx context.Context, id domain.JobID) (domain.Job, error)
}) *domain.Tool {
	return &domain.Tool{
		Name:        "exec",
		Description: "Executes a shell command in a secure, isolated environment (Docker container). Use this for npm install, git clone, etc.",
		ExecutionType: domain.ExecDocker,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The command to execute (e.g., 'npm install').",
				},
				"image": map[string]interface{}{
					"type":        "string",
					"description": "Optional: Docker image to use (default: 'node:20-alpine' for generic work).",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the project/workspace to mount.",
				},
			},
			Required: []string{"command", "project_id"},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			command, ok := params["command"].(string)
			if !ok {
				return nil, fmt.Errorf("command must be a string")
			}
			image, _ := params["image"].(string)
			projectID, ok := params["project_id"].(string)
			if !ok || projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				} else {
					return nil, fmt.Errorf("project_id must be provided (or context missing)")
				}
			}

			// Defaults
			if image == "" {
				image = "node:20-alpine" // Safe default
			}
			// Sanitize command (basic)
			cmd := strings.TrimSpace(command)
			if cmd == "" {
				return nil, fmt.Errorf("command is required")
			}

			// 1. Submit Job
			jobSpec := domain.WorkerSpec{
				Image:   image,
				Command: []string{"/bin/sh", "-c", cmd},
				Env:     map[string]string{
					"PROJECT_ID": projectID,
				},
			}

			// Inject metadata so WorkerLifecycle knows context/project
			// We might need to handle this in WorkerLifecycle to mount the right volume
			// For now, we rely on the implementation detail that WorkerLifecycle creates ephemeral workspaces
			// unless we implement Project persistence.
			// WORKAROUND: We will assume future iterations will handle mounting based on projectID.

			jobID, err := lifecycle.SubmitJob(ctx, jobSpec)
			if err != nil {
				return nil, fmt.Errorf("failed to submit exec job: %w", err)
			}

			// 2. Wait for Completion (Synchronous block for ReAct)
			timeout := time.After(60 * time.Second)
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-timeout:
					return nil, fmt.Errorf("command timed out (job %s)", jobID)
				case <-ticker.C:
					job, err := repo.GetJob(ctx, jobID)
					if err != nil {
						continue
					}

					if job.Status == domain.JobStatusCompleted {
						return fmt.Sprintf("Command finished successfully. Job ID: %s", jobID), nil
					}
					if job.Status == domain.JobStatusFailed {
						msg := "Command failed"
						if job.Error != nil {
							msg += ": " + *job.Error
						}
						return nil, fmt.Errorf(msg)
					}
				}
			}
		},
	}
}
