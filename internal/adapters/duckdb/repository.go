package duckdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/ports"
	_ "github.com/marcboeker/go-duckdb"
)

type Repository struct {
	db *sql.DB
}

// NewRepository creates a new DuckDB repository and runs migrations
func NewRepository(path string) (*Repository, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping duckdb: %w", err)
	}

	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to migrate duckdb: %w", err)
	}

	return repo, nil
}

// migrate creates necessary tables
func (r *Repository) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS workers (
			id TEXT PRIMARY KEY,
			spec JSON,
			status TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			metadata JSON
		);`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			result TEXT,
			error TEXT,
			status TEXT,
			worker_id TEXT,
			spec JSON,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			metadata JSON
		);`,
	}
	
	for _, q := range queries {
		if _, err := r.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// Ensure Repository implements Repository interface
var _ ports.Repository = (*Repository)(nil)

func (r *Repository) SaveWorker(ctx context.Context, worker domain.Worker) error {
	specJSON, err := json.Marshal(worker.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}
	metaJSON, err := json.Marshal(worker.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
	INSERT INTO workers (id, spec, status, created_at, updated_at, metadata)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT (id) DO UPDATE SET
		spec = excluded.spec,
		status = excluded.status,
		updated_at = excluded.updated_at,
		metadata = excluded.metadata;
	`
	_, err = r.db.ExecContext(ctx, query,
		worker.ID,
		string(specJSON),
		worker.Status,
		worker.CreatedAt,
		worker.UpdatedAt,
		string(metaJSON),
	)
	return err
}

func (r *Repository) GetWorker(ctx context.Context, id domain.WorkerID) (domain.Worker, error) {
	query := `SELECT id, spec, status, created_at, updated_at, metadata FROM workers WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var w domain.Worker
	var specJSON, metaJSON string
	var idStr string

	if err := row.Scan(&idStr, &specJSON, &w.Status, &w.CreatedAt, &w.UpdatedAt, &metaJSON); err != nil {
		if err == sql.ErrNoRows {
			return domain.Worker{}, domain.ErrWorkerNotFound
		}
		return domain.Worker{}, err
	}

	w.ID = domain.WorkerID(idStr)
	if err := json.Unmarshal([]byte(specJSON), &w.Spec); err != nil {
		return domain.Worker{}, fmt.Errorf("failed to unmarshal spec: %w", err)
	}
	if err := json.Unmarshal([]byte(metaJSON), &w.Metadata); err != nil {
		return domain.Worker{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return w, nil
}

func (r *Repository) ListWorkers(ctx context.Context) ([]domain.Worker, error) {
	query := `SELECT id, spec, status, created_at, updated_at, metadata FROM workers`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workers []domain.Worker
	for rows.Next() {
		var w domain.Worker
		var specJSON, metaJSON string
		var idStr string
		if err := rows.Scan(&idStr, &specJSON, &w.Status, &w.CreatedAt, &w.UpdatedAt, &metaJSON); err != nil {
			return nil, err
		}
		w.ID = domain.WorkerID(idStr)
		_ = json.Unmarshal([]byte(specJSON), &w.Spec)
		_ = json.Unmarshal([]byte(metaJSON), &w.Metadata)
		workers = append(workers, w)
	}
	return workers, nil
}

func (r *Repository) UpdateWorkerStatus(ctx context.Context, id domain.WorkerID, status domain.HealthStatus) error {
	query := `UPDATE workers SET status = ?, updated_at = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrWorkerNotFound
	}
	return nil
}

// Job Management

func (r *Repository) SaveJob(ctx context.Context, job domain.Job) error {
	specJSON, err := json.Marshal(job.Spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}
	metaJSON, err := json.Marshal(job.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
	INSERT INTO jobs (id, result, error, status, worker_id, spec, created_at, updated_at, metadata)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT (id) DO UPDATE SET
		result = excluded.result,
		error = excluded.error,
		status = excluded.status,
		worker_id = excluded.worker_id,
		updated_at = excluded.updated_at,
		metadata = excluded.metadata;
	`
	
	// Handle nullable fields
	var workerID *string
	if job.WorkerID != nil {
		s := string(*job.WorkerID)
		workerID = &s
	}

	_, err = r.db.ExecContext(ctx, query,
		job.ID,
		job.Result,
		job.Error,
		job.Status,
		workerID,
		string(specJSON),
		job.CreatedAt,
		job.UpdatedAt,
		string(metaJSON),
	)
	return err
}

func (r *Repository) GetJob(ctx context.Context, id domain.JobID) (domain.Job, error) {
	query := `SELECT id, result, error, status, worker_id, spec, created_at, updated_at, metadata FROM jobs WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var j domain.Job
	var specJSON, metaJSON string
	var workerIDStr *string
	var idStr string

	if err := row.Scan(&idStr, &j.Result, &j.Error, &j.Status, &workerIDStr, &specJSON, &j.CreatedAt, &j.UpdatedAt, &metaJSON); err != nil {
		if err == sql.ErrNoRows {
			return domain.Job{}, domain.ErrJobNotFound
		}
		return domain.Job{}, err
	}

	j.ID = domain.JobID(idStr)
	if workerIDStr != nil {
		wid := domain.WorkerID(*workerIDStr)
		j.WorkerID = &wid
	}

	if err := json.Unmarshal([]byte(specJSON), &j.Spec); err != nil {
		return domain.Job{}, fmt.Errorf("failed to unmarshal spec: %w", err)
	}
	if err := json.Unmarshal([]byte(metaJSON), &j.Metadata); err != nil {
		return domain.Job{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return j, nil
}

func (r *Repository) ListJobs(ctx context.Context) ([]domain.Job, error) {
	query := `SELECT id, result, error, status, worker_id, spec, created_at, updated_at, metadata FROM jobs ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		var j domain.Job
		var specJSON, metaJSON string
		var workerIDStr *string
		var idStr string

		if err := rows.Scan(&idStr, &j.Result, &j.Error, &j.Status, &workerIDStr, &specJSON, &j.CreatedAt, &j.UpdatedAt, &metaJSON); err != nil {
			return nil, err
		}

		j.ID = domain.JobID(idStr)
		if workerIDStr != nil {
			wid := domain.WorkerID(*workerIDStr)
			j.WorkerID = &wid
		}
		_ = json.Unmarshal([]byte(specJSON), &j.Spec)
		_ = json.Unmarshal([]byte(metaJSON), &j.Metadata)

		jobs = append(jobs, j)
	}
	return jobs, nil
}
