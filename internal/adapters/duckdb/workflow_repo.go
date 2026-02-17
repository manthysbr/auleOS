package duckdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

func (r *Repository) SaveWorkflow(ctx context.Context, wf *domain.Workflow) error {
	stepsJSON, err := json.Marshal(wf.Steps)
	if err != nil {
		return fmt.Errorf("failed to marshal steps: %w", err)
	}
	stateJSON, err := json.Marshal(wf.State)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	query := `
	INSERT INTO workflows (id, project_id, name, description, steps, state, status, created_at, started_at, completed_at, error)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT (id) DO UPDATE SET
		steps = excluded.steps,
		state = excluded.state,
		status = excluded.status,
		started_at = excluded.started_at,
		completed_at = excluded.completed_at,
		error = excluded.error;
	`

	// Handle Nullable Timestamps
	var startedAt, completedAt *time.Time
	startedAt = wf.StartedAt
	completedAt = wf.CompletedAt

	_, err = r.db.ExecContext(ctx, query,
		wf.ID, wf.ProjectID, wf.Name, wf.Description,
		string(stepsJSON), string(stateJSON), wf.Status,
		wf.CreatedAt, startedAt, completedAt, wf.Error,
	)
	return err
}

func (r *Repository) GetWorkflow(ctx context.Context, id domain.WorkflowID) (*domain.Workflow, error) {
	query := `SELECT id, project_id, name, description, CAST(steps AS TEXT), CAST(state AS TEXT), status, created_at, started_at, completed_at, error FROM workflows WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var wf domain.Workflow
	var stepsJSON, stateJSON string
	var idStr, projectIDStr string
	var statusStr string
	var errStr *string

	if err := row.Scan(&idStr, &projectIDStr, &wf.Name, &wf.Description, &stepsJSON, &stateJSON, &statusStr, &wf.CreatedAt, &wf.StartedAt, &wf.CompletedAt, &errStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workflow not found: %s", id)
		}
		return nil, err
	}

	wf.ID = domain.WorkflowID(idStr)
	wf.ProjectID = domain.ProjectID(projectIDStr)
	wf.Status = domain.WorkflowStatus(statusStr)
	if errStr != nil {
		wf.Error = errStr
	}

	if err := json.Unmarshal([]byte(stepsJSON), &wf.Steps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal steps: %w", err)
	}
	if err := json.Unmarshal([]byte(stateJSON), &wf.State); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &wf, nil
}

func (r *Repository) ListWorkflows(ctx context.Context) ([]domain.Workflow, error) {
	query := `SELECT id, project_id, name, description, CAST(steps AS TEXT), CAST(state AS TEXT), status, created_at, started_at, completed_at, error FROM workflows ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []domain.Workflow
	for rows.Next() {
		var wf domain.Workflow
		var stepsJSON, stateJSON string
		var idStr, projectIDStr string
		var statusStr string
		var errStr *string

		if err := rows.Scan(&idStr, &projectIDStr, &wf.Name, &wf.Description, &stepsJSON, &stateJSON, &statusStr, &wf.CreatedAt, &wf.StartedAt, &wf.CompletedAt, &errStr); err != nil {
			return nil, err
		}

		wf.ID = domain.WorkflowID(idStr)
		wf.ProjectID = domain.ProjectID(projectIDStr)
		wf.Status = domain.WorkflowStatus(statusStr)
		if errStr != nil {
			wf.Error = errStr
		}

		// Ignoring errors on unmarshal for list view could be safer, but rule says "NEVER swallow errors".
		// But fail-fast on one bad record blocks the whole list.
		// I will log error (if I had logger) or return error. I'll return error to be safe.
		if err := json.Unmarshal([]byte(stepsJSON), &wf.Steps); err != nil {
			return nil, fmt.Errorf("failed to unmarshal steps for wf %s: %w", idStr, err)
		}
		if err := json.Unmarshal([]byte(stateJSON), &wf.State); err != nil {
			return nil, fmt.Errorf("failed to unmarshal state for wf %s: %w", idStr, err)
		}
		workflows = append(workflows, wf)
	}
	return workflows, nil
}
