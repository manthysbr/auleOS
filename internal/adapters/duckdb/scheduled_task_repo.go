package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

func (r *Repository) SaveScheduledTask(ctx context.Context, task *domain.ScheduledTask) error {
	var personaID *string
	if task.PersonaID != nil {
		s := string(*task.PersonaID)
		personaID = &s
	}

	query := `
	INSERT INTO scheduled_tasks (id, project_id, name, prompt, persona_id, type, cron_expr, interval_sec, next_run, last_run, last_result, run_count, status, created_at, created_by)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT (id) DO UPDATE SET
		next_run = excluded.next_run,
		last_run = excluded.last_run,
		last_result = excluded.last_result,
		run_count = excluded.run_count,
		status = excluded.status;
	`

	_, err := r.db.ExecContext(ctx, query,
		task.ID, task.ProjectID, task.Name, task.Prompt, personaID,
		task.Type, task.CronExpr, task.IntervalSec,
		task.NextRun, task.LastRun, task.LastResult,
		task.RunCount, task.Status, task.CreatedAt, task.CreatedBy,
	)
	return err
}

func (r *Repository) GetScheduledTask(ctx context.Context, id domain.ScheduledTaskID) (*domain.ScheduledTask, error) {
	query := `SELECT id, project_id, name, prompt, persona_id, type, cron_expr, interval_sec, next_run, last_run, last_result, run_count, status, created_at, created_by FROM scheduled_tasks WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	task, err := scanScheduledTask(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("scheduled task not found: %s", id)
		}
		return nil, err
	}
	return task, nil
}

func (r *Repository) ListScheduledTasks(ctx context.Context) ([]domain.ScheduledTask, error) {
	query := `SELECT id, project_id, name, prompt, persona_id, type, cron_expr, interval_sec, next_run, last_run, last_result, run_count, status, created_at, created_by FROM scheduled_tasks ORDER BY next_run ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []domain.ScheduledTask
	for rows.Next() {
		task, err := scanScheduledTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, nil
}

func (r *Repository) DeleteScheduledTask(ctx context.Context, id domain.ScheduledTaskID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_tasks WHERE id = ?`, id)
	return err
}

func (r *Repository) GetDueTasks(ctx context.Context, now time.Time) ([]domain.ScheduledTask, error) {
	query := `SELECT id, project_id, name, prompt, persona_id, type, cron_expr, interval_sec, next_run, last_run, last_result, run_count, status, created_at, created_by FROM scheduled_tasks WHERE status = 'active' AND next_run <= ? ORDER BY next_run ASC`
	rows, err := r.db.QueryContext(ctx, query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []domain.ScheduledTask
	for rows.Next() {
		task, err := scanScheduledTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, nil
}

// scanScheduledTask scans a single row into a ScheduledTask
func scanScheduledTask(row *sql.Row) (*domain.ScheduledTask, error) {
	var t domain.ScheduledTask
	var idStr, projectIDStr, typeStr, statusStr string
	var personaIDStr *string

	err := row.Scan(
		&idStr, &projectIDStr, &t.Name, &t.Prompt, &personaIDStr,
		&typeStr, &t.CronExpr, &t.IntervalSec,
		&t.NextRun, &t.LastRun, &t.LastResult,
		&t.RunCount, &statusStr, &t.CreatedAt, &t.CreatedBy,
	)
	if err != nil {
		return nil, err
	}

	t.ID = domain.ScheduledTaskID(idStr)
	t.ProjectID = domain.ProjectID(projectIDStr)
	t.Type = domain.ScheduledTaskType(typeStr)
	t.Status = domain.ScheduledTaskStatus(statusStr)
	if personaIDStr != nil {
		pid := domain.PersonaID(*personaIDStr)
		t.PersonaID = &pid
	}
	return &t, nil
}

// scanScheduledTaskRows scans from sql.Rows (same logic, different interface)
func scanScheduledTaskRows(rows *sql.Rows) (*domain.ScheduledTask, error) {
	var t domain.ScheduledTask
	var idStr, projectIDStr, typeStr, statusStr string
	var personaIDStr *string

	err := rows.Scan(
		&idStr, &projectIDStr, &t.Name, &t.Prompt, &personaIDStr,
		&typeStr, &t.CronExpr, &t.IntervalSec,
		&t.NextRun, &t.LastRun, &t.LastResult,
		&t.RunCount, &statusStr, &t.CreatedAt, &t.CreatedBy,
	)
	if err != nil {
		return nil, err
	}

	t.ID = domain.ScheduledTaskID(idStr)
	t.ProjectID = domain.ProjectID(projectIDStr)
	t.Type = domain.ScheduledTaskType(typeStr)
	t.Status = domain.ScheduledTaskStatus(statusStr)
	if personaIDStr != nil {
		pid := domain.PersonaID(*personaIDStr)
		t.PersonaID = &pid
	}
	return &t, nil
}
