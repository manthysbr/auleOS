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
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			job_id TEXT,
			conversation_id TEXT,
			type TEXT NOT NULL DEFAULT 'other',
			name TEXT NOT NULL DEFAULT '',
			file_path TEXT NOT NULL,
			mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
			size_bytes BIGINT NOT NULL DEFAULT 0,
			prompt TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS personas (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			system_prompt TEXT NOT NULL DEFAULT '',
			icon TEXT NOT NULL DEFAULT 'bot',
			color TEXT NOT NULL DEFAULT 'blue',
			allowed_tools JSON,
			is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			steps JSON,
			state JSON,
			status TEXT,
			created_at TIMESTAMP,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			error TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			name TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL DEFAULT '',
			persona_id TEXT,
			type TEXT NOT NULL DEFAULT 'one_shot',
			cron_expr TEXT NOT NULL DEFAULT '',
			interval_sec INTEGER NOT NULL DEFAULT 0,
			next_run TIMESTAMP NOT NULL,
			last_run TIMESTAMP,
			last_result TEXT NOT NULL DEFAULT '',
			run_count INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMP NOT NULL,
			created_by TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			content TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'fact',
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS traces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'running',
			conversation_id TEXT NOT NULL DEFAULT '',
			persona_id TEXT NOT NULL DEFAULT '',
			root_span_id TEXT NOT NULL DEFAULT '',
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP,
			duration_ms BIGINT NOT NULL DEFAULT 0,
			span_count INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS spans (
			id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL,
			parent_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL DEFAULT 'agent',
			status TEXT NOT NULL DEFAULT 'running',
			input TEXT NOT NULL DEFAULT '',
			output TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			attributes JSON,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP,
			duration_ms BIGINT NOT NULL DEFAULT 0
		);`,
	}

	for _, q := range queries {
		if _, err := r.db.Exec(q); err != nil {
			return err
		}
	}

	// Additive migrations — safe to re-run
	migrations := []string{
		`ALTER TABLE conversations ADD COLUMN IF NOT EXISTS project_id TEXT`,
		`ALTER TABLE conversations ADD COLUMN IF NOT EXISTS persona_id TEXT`,
		`ALTER TABLE personas ADD COLUMN IF NOT EXISTS model_override TEXT DEFAULT ''`,
	}
	for _, m := range migrations {
		_, _ = r.db.Exec(m) // ignore errors; DuckDB may not support IF NOT EXISTS on ALTER
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
	var personaID *string
	if conv.PersonaID != nil {
		s := string(*conv.PersonaID)
		personaID = &s
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO conversations (id, title, persona_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		conv.ID, conv.Title, personaID, conv.CreatedAt, conv.UpdatedAt,
	)
	return err
}

func (r *Repository) GetConversation(ctx context.Context, id domain.ConversationID) (domain.Conversation, error) {
	var c domain.Conversation
	var idStr string
	var personaID *string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, persona_id, created_at, updated_at FROM conversations WHERE id = ?`, id,
	).Scan(&idStr, &c.Title, &personaID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Conversation{}, domain.ErrConversationNotFound
		}
		return domain.Conversation{}, err
	}
	c.ID = domain.ConversationID(idStr)
	if personaID != nil {
		pid := domain.PersonaID(*personaID)
		c.PersonaID = &pid
	}
	return c, nil
}

func (r *Repository) ListConversations(ctx context.Context) ([]domain.Conversation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, title, persona_id, created_at, updated_at FROM conversations ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []domain.Conversation
	for rows.Next() {
		var c domain.Conversation
		var idStr string
		var personaID *string
		if err := rows.Scan(&idStr, &c.Title, &personaID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.ID = domain.ConversationID(idStr)
		if personaID != nil {
			pid := domain.PersonaID(*personaID)
			c.PersonaID = &pid
		}
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

// --- Project Management ---

func (r *Repository) CreateProject(ctx context.Context, proj domain.Project) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		proj.ID, proj.Name, proj.Description, proj.CreatedAt, proj.UpdatedAt,
	)
	return err
}

func (r *Repository) GetProject(ctx context.Context, id domain.ProjectID) (domain.Project, error) {
	var p domain.Project
	var idStr string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM projects WHERE id = ?`, id,
	).Scan(&idStr, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Project{}, domain.ErrProjectNotFound
		}
		return domain.Project{}, err
	}
	p.ID = domain.ProjectID(idStr)
	return p, nil
}

func (r *Repository) ListProjects(ctx context.Context) ([]domain.Project, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM projects ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		var idStr string
		if err := rows.Scan(&idStr, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.ID = domain.ProjectID(idStr)
		projects = append(projects, p)
	}
	return projects, nil
}

func (r *Repository) UpdateProject(ctx context.Context, proj domain.Project) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		proj.Name, proj.Description, proj.UpdatedAt, proj.ID,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrProjectNotFound
	}
	return nil
}

func (r *Repository) DeleteProject(ctx context.Context, id domain.ProjectID) error {
	// Unlink conversations from project
	if _, err := r.db.ExecContext(ctx, `UPDATE conversations SET project_id = NULL WHERE project_id = ?`, id); err != nil {
		return err
	}
	// Unlink artifacts from project
	if _, err := r.db.ExecContext(ctx, `UPDATE artifacts SET project_id = NULL WHERE project_id = ?`, id); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrProjectNotFound
	}
	return nil
}

func (r *Repository) ListProjectConversations(ctx context.Context, projectID domain.ProjectID) ([]domain.Conversation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, title, project_id, created_at, updated_at FROM conversations WHERE project_id = ? ORDER BY updated_at DESC`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []domain.Conversation
	for rows.Next() {
		var c domain.Conversation
		var idStr string
		var projID *string
		if err := rows.Scan(&idStr, &c.Title, &projID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.ID = domain.ConversationID(idStr)
		if projID != nil {
			pid := domain.ProjectID(*projID)
			c.ProjectID = &pid
		}
		convs = append(convs, c)
	}
	return convs, nil
}

// --- Artifact Management ---

func (r *Repository) SaveArtifact(ctx context.Context, art domain.Artifact) error {
	var projectID, jobID, convID *string
	if art.ProjectID != nil {
		s := string(*art.ProjectID)
		projectID = &s
	}
	if art.JobID != nil {
		s := string(*art.JobID)
		jobID = &s
	}
	if art.ConversationID != nil {
		s := string(*art.ConversationID)
		convID = &s
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO artifacts (id, project_id, job_id, conversation_id, type, name, file_path, mime_type, size_bytes, prompt, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET
		 	project_id = excluded.project_id,
		 	name = excluded.name,
		 	prompt = excluded.prompt`,
		art.ID, projectID, jobID, convID, art.Type, art.Name, art.FilePath, art.MimeType, art.SizeBytes, art.Prompt, art.CreatedAt,
	)
	return err
}

func (r *Repository) GetArtifact(ctx context.Context, id domain.ArtifactID) (domain.Artifact, error) {
	var a domain.Artifact
	var idStr string
	var projectID, jobID, convID *string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, job_id, conversation_id, type, name, file_path, mime_type, size_bytes, prompt, created_at
		 FROM artifacts WHERE id = ?`, id,
	).Scan(&idStr, &projectID, &jobID, &convID, &a.Type, &a.Name, &a.FilePath, &a.MimeType, &a.SizeBytes, &a.Prompt, &a.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Artifact{}, domain.ErrArtifactNotFound
		}
		return domain.Artifact{}, err
	}
	a.ID = domain.ArtifactID(idStr)
	if projectID != nil {
		pid := domain.ProjectID(*projectID)
		a.ProjectID = &pid
	}
	if jobID != nil {
		jid := domain.JobID(*jobID)
		a.JobID = &jid
	}
	if convID != nil {
		cid := domain.ConversationID(*convID)
		a.ConversationID = &cid
	}
	return a, nil
}

func (r *Repository) scanArtifacts(rows *sql.Rows) ([]domain.Artifact, error) {
	var arts []domain.Artifact
	for rows.Next() {
		var a domain.Artifact
		var idStr string
		var projectID, jobID, convID *string

		if err := rows.Scan(&idStr, &projectID, &jobID, &convID, &a.Type, &a.Name, &a.FilePath, &a.MimeType, &a.SizeBytes, &a.Prompt, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.ID = domain.ArtifactID(idStr)
		if projectID != nil {
			pid := domain.ProjectID(*projectID)
			a.ProjectID = &pid
		}
		if jobID != nil {
			jid := domain.JobID(*jobID)
			a.JobID = &jid
		}
		if convID != nil {
			cid := domain.ConversationID(*convID)
			a.ConversationID = &cid
		}
		arts = append(arts, a)
	}
	return arts, nil
}

func (r *Repository) ListArtifacts(ctx context.Context) ([]domain.Artifact, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, job_id, conversation_id, type, name, file_path, mime_type, size_bytes, prompt, created_at
		 FROM artifacts ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanArtifacts(rows)
}

func (r *Repository) ListProjectArtifacts(ctx context.Context, projectID domain.ProjectID) ([]domain.Artifact, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, job_id, conversation_id, type, name, file_path, mime_type, size_bytes, prompt, created_at
		 FROM artifacts WHERE project_id = ? ORDER BY created_at DESC`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanArtifacts(rows)
}

func (r *Repository) DeleteArtifact(ctx context.Context, id domain.ArtifactID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM artifacts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrArtifactNotFound
	}
	return nil
}

// --- Persona Management ---

func (r *Repository) CreatePersona(ctx context.Context, p domain.Persona) error {
	allowedJSON, _ := json.Marshal(p.AllowedTools)

	// For builtin personas, upsert to keep them up-to-date across versions.
	// User-created personas use ON CONFLICT DO NOTHING.
	var query string
	if p.IsBuiltin {
		query = `INSERT INTO personas (id, name, description, system_prompt, icon, color, allowed_tools, model_override, is_builtin, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			system_prompt = excluded.system_prompt,
			icon = excluded.icon,
			color = excluded.color,
			allowed_tools = excluded.allowed_tools,
			model_override = excluded.model_override,
			updated_at = excluded.updated_at`
	} else {
		query = `INSERT INTO personas (id, name, description, system_prompt, icon, color, allowed_tools, model_override, is_builtin, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO NOTHING`
	}

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.Name, p.Description, p.SystemPrompt, p.Icon, p.Color, string(allowedJSON), p.ModelOverride, p.IsBuiltin, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *Repository) GetPersona(ctx context.Context, id domain.PersonaID) (domain.Persona, error) {
	var p domain.Persona
	var idStr, allowedJSON string
	var modelOverride sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, description, system_prompt, icon, color, CAST(allowed_tools AS TEXT), model_override, is_builtin, created_at, updated_at
		 FROM personas WHERE id = ?`, id,
	).Scan(&idStr, &p.Name, &p.Description, &p.SystemPrompt, &p.Icon, &p.Color, &allowedJSON, &modelOverride, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Persona{}, domain.ErrPersonaNotFound
		}
		return domain.Persona{}, err
	}
	p.ID = domain.PersonaID(idStr)
	_ = json.Unmarshal([]byte(allowedJSON), &p.AllowedTools)
	if modelOverride.Valid {
		p.ModelOverride = modelOverride.String
	}
	return p, nil
}

func (r *Repository) ListPersonas(ctx context.Context) ([]domain.Persona, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, system_prompt, icon, color, CAST(allowed_tools AS TEXT), model_override, is_builtin, created_at, updated_at
		 FROM personas ORDER BY is_builtin DESC, name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var personas []domain.Persona
	for rows.Next() {
		var p domain.Persona
		var idStr, allowedJSON string
		var modelOverride sql.NullString
		if err := rows.Scan(&idStr, &p.Name, &p.Description, &p.SystemPrompt, &p.Icon, &p.Color, &allowedJSON, &modelOverride, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.ID = domain.PersonaID(idStr)
		_ = json.Unmarshal([]byte(allowedJSON), &p.AllowedTools)
		if modelOverride.Valid {
			p.ModelOverride = modelOverride.String
		}
		personas = append(personas, p)
	}
	return personas, nil
}

func (r *Repository) UpdatePersona(ctx context.Context, p domain.Persona) error {
	allowedJSON, _ := json.Marshal(p.AllowedTools)
	result, err := r.db.ExecContext(ctx,
		`UPDATE personas SET name = ?, description = ?, system_prompt = ?, icon = ?, color = ?, allowed_tools = ?, model_override = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Description, p.SystemPrompt, p.Icon, p.Color, string(allowedJSON), p.ModelOverride, p.UpdatedAt, p.ID,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrPersonaNotFound
	}
	return nil
}

func (r *Repository) DeletePersona(ctx context.Context, id domain.PersonaID) error {
	// Unlink conversations from persona
	if _, err := r.db.ExecContext(ctx, `UPDATE conversations SET persona_id = NULL WHERE persona_id = ?`, id); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM personas WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrPersonaNotFound
	}
	return nil
}
