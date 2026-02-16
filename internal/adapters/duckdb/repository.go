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
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			thought TEXT NOT NULL DEFAULT '',
			steps JSON,
			tool_call JSON,
			metadata JSON,
			created_at TIMESTAMP NOT NULL
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
	query := `SELECT id, CAST(spec AS TEXT), status, created_at, updated_at, CAST(metadata AS TEXT) FROM workers WHERE id = ?`
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
	query := `SELECT id, CAST(spec AS TEXT), status, created_at, updated_at, CAST(metadata AS TEXT) FROM workers`
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
	query := `SELECT id, result, error, status, worker_id, CAST(spec AS TEXT), created_at, updated_at, CAST(metadata AS TEXT) FROM jobs WHERE id = ?`
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

// Settings Management

func (r *Repository) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("setting not found: %s", key)
		}
		return "", err
	}
	return value, nil
}

func (r *Repository) SaveSetting(ctx context.Context, key string, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at;
	`, key, value, time.Now())
	return err
}

func (r *Repository) ListJobs(ctx context.Context) ([]domain.Job, error) {
	query := `SELECT id, result, error, status, worker_id, CAST(spec AS TEXT), created_at, updated_at, CAST(metadata AS TEXT) FROM jobs ORDER BY created_at DESC`
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

// Conversation Management

func (r *Repository) CreateConversation(ctx context.Context, conv domain.Conversation) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO conversations (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		conv.ID, conv.Title, conv.CreatedAt, conv.UpdatedAt,
	)
	return err
}

func (r *Repository) GetConversation(ctx context.Context, id domain.ConversationID) (domain.Conversation, error) {
	var c domain.Conversation
	var idStr string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, created_at, updated_at FROM conversations WHERE id = ?`, id,
	).Scan(&idStr, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Conversation{}, domain.ErrConversationNotFound
		}
		return domain.Conversation{}, err
	}
	c.ID = domain.ConversationID(idStr)
	return c, nil
}

func (r *Repository) ListConversations(ctx context.Context) ([]domain.Conversation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []domain.Conversation
	for rows.Next() {
		var c domain.Conversation
		var idStr string
		if err := rows.Scan(&idStr, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.ID = domain.ConversationID(idStr)
		convs = append(convs, c)
	}
	return convs, nil
}

func (r *Repository) UpdateConversationTitle(ctx context.Context, id domain.ConversationID, title string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`, title, time.Now(), id,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrConversationNotFound
	}
	return nil
}

func (r *Repository) DeleteConversation(ctx context.Context, id domain.ConversationID) error {
	// Delete messages first, then conversation
	if _, err := r.db.ExecContext(ctx, `DELETE FROM messages WHERE conversation_id = ?`, id); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrConversationNotFound
	}
	return nil
}

// Message Management

func (r *Repository) AddMessage(ctx context.Context, msg domain.Message) error {
	stepsJSON, _ := json.Marshal(msg.Steps)
	toolCallJSON, _ := json.Marshal(msg.ToolCall)
	metaJSON, _ := json.Marshal(msg.Metadata)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO messages (id, conversation_id, role, content, thought, steps, tool_call, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ConversationID, msg.Role, msg.Content, msg.Thought,
		string(stepsJSON), string(toolCallJSON), string(metaJSON), msg.CreatedAt,
	)
	return err
}

func (r *Repository) ListMessages(ctx context.Context, convID domain.ConversationID, limit int) ([]domain.Message, error) {
	query := `SELECT id, conversation_id, role, content, thought,
	          CAST(steps AS TEXT), CAST(tool_call AS TEXT), CAST(metadata AS TEXT), created_at
	          FROM messages WHERE conversation_id = ? ORDER BY created_at ASC`
	if limit > 0 {
		// Get last N messages: subquery to get latest, then order ASC
		query = fmt.Sprintf(`SELECT * FROM (
			SELECT id, conversation_id, role, content, thought,
			       CAST(steps AS TEXT), CAST(tool_call AS TEXT), CAST(metadata AS TEXT), created_at
			FROM messages WHERE conversation_id = ? ORDER BY created_at DESC LIMIT %d
		) sub ORDER BY created_at ASC`, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []domain.Message
	for rows.Next() {
		var m domain.Message
		var idStr, convIDStr, roleStr string
		var stepsJSON, toolCallJSON, metaJSON string

		if err := rows.Scan(&idStr, &convIDStr, &roleStr, &m.Content, &m.Thought,
			&stepsJSON, &toolCallJSON, &metaJSON, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.ID = domain.MessageID(idStr)
		m.ConversationID = domain.ConversationID(convIDStr)
		m.Role = domain.MessageRole(roleStr)

		_ = json.Unmarshal([]byte(stepsJSON), &m.Steps)
		_ = json.Unmarshal([]byte(toolCallJSON), &m.ToolCall)
		_ = json.Unmarshal([]byte(metaJSON), &m.Metadata)

		msgs = append(msgs, m)
	}
	return msgs, nil
}
