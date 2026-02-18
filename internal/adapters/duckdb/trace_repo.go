package duckdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// SaveTrace persists a completed trace and all its spans to DuckDB.
func (r *Repository) SaveTrace(ctx context.Context, trace *domain.Trace) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Upsert trace row
	_, err = tx.ExecContext(ctx, `
		INSERT INTO traces (id, name, status, conversation_id, persona_id, root_span_id,
		                    start_time, end_time, duration_ms, span_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			status       = excluded.status,
			end_time     = excluded.end_time,
			duration_ms  = excluded.duration_ms,
			span_count   = excluded.span_count`,
		string(trace.ID),
		trace.Name,
		string(trace.Status),
		trace.ConversationID,
		trace.PersonaID,
		string(trace.RootSpanID),
		trace.StartTime,
		trace.EndTime,
		trace.DurationMs,
		trace.SpanCount,
	)
	if err != nil {
		return fmt.Errorf("upsert trace: %w", err)
	}

	// Upsert spans
	for _, span := range trace.Spans {
		attrJSON, _ := json.Marshal(span.Attributes)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO spans (id, trace_id, parent_id, name, kind, status,
			                   input, output, error, model, attributes, start_time, end_time, duration_ms)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				status      = excluded.status,
				output      = excluded.output,
				error       = excluded.error,
				end_time    = excluded.end_time,
				duration_ms = excluded.duration_ms`,
			string(span.ID),
			string(span.TraceID),
			string(span.ParentID),
			span.Name,
			string(span.Kind),
			string(span.Status),
			span.Input,
			span.Output,
			span.Error,
			span.Model,
			string(attrJSON),
			span.StartTime,
			span.EndTime,
			span.DurationMs,
		)
		if err != nil {
			return fmt.Errorf("upsert span %s: %w", span.ID, err)
		}
	}

	return tx.Commit()
}

// ListTraces returns summaries of the most recent traces (newest first).
func (r *Repository) ListTraces(ctx context.Context, limit int) ([]domain.TraceSummary, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, status, start_time, end_time, duration_ms, span_count
		FROM traces
		ORDER BY start_time DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list traces: %w", err)
	}
	defer rows.Close()

	var out []domain.TraceSummary
	for rows.Next() {
		var s domain.TraceSummary
		var statusStr string
		var endTime *time.Time
		err := rows.Scan(&s.ID, &s.Name, &statusStr, &s.StartTime, &endTime, &s.DurationMs, &s.SpanCount)
		if err != nil {
			return nil, err
		}
		s.Status = domain.SpanStatus(statusStr)
		out = append(out, s)
	}
	if out == nil {
		out = []domain.TraceSummary{}
	}
	return out, rows.Err()
}

// GetTrace returns a full trace with all its spans.
func (r *Repository) GetTrace(ctx context.Context, id domain.TraceID) (*domain.Trace, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, status, conversation_id, persona_id, root_span_id,
		       start_time, end_time, duration_ms, span_count
		FROM traces WHERE id = ?`, string(id))

	var t domain.Trace
	var statusStr, convID, personaID, rootSpanID string
	err := row.Scan(
		&t.ID, &t.Name, &statusStr, &convID, &personaID, &rootSpanID,
		&t.StartTime, &t.EndTime, &t.DurationMs, &t.SpanCount,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("trace not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get trace: %w", err)
	}
	t.Status = domain.SpanStatus(statusStr)
	t.ConversationID = convID
	t.PersonaID = personaID
	t.RootSpanID = domain.SpanID(rootSpanID)

	// Load spans
	spans, err := r.loadSpansForTrace(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Spans = spans
	return &t, nil
}

func (r *Repository) loadSpansForTrace(ctx context.Context, traceID domain.TraceID) ([]domain.Span, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, trace_id, parent_id, name, kind, status,
		       input, output, error, model, attributes, start_time, end_time, duration_ms
		FROM spans WHERE trace_id = ?
		ORDER BY start_time ASC`, string(traceID))
	if err != nil {
		return nil, fmt.Errorf("load spans: %w", err)
	}
	defer rows.Close()

	var out []domain.Span
	for rows.Next() {
		var s domain.Span
		var kindStr, statusStr, attrJSON string
		err := rows.Scan(
			&s.ID, &s.TraceID, &s.ParentID,
			&s.Name, &kindStr, &statusStr,
			&s.Input, &s.Output, &s.Error, &s.Model,
			&attrJSON, &s.StartTime, &s.EndTime, &s.DurationMs,
		)
		if err != nil {
			return nil, err
		}
		s.Kind = domain.SpanKind(kindStr)
		s.Status = domain.SpanStatus(statusStr)
		if attrJSON != "" && attrJSON != "null" {
			_ = json.Unmarshal([]byte(attrJSON), &s.Attributes)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
