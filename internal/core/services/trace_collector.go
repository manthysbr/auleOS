package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/manthysbr/auleOS/internal/core/domain"
)

const (
	maxTraces      = 500  // ring buffer size
	maxInputOutput = 2000 // truncate input/output at 2KB
)

// TraceRepository is the minimal persistence interface needed by TraceCollector.
type TraceRepository interface {
	SaveTrace(ctx context.Context, trace *domain.Trace) error
}

// TraceCollector gathers, stores, and exposes traces and spans.
// Thread-safe. Operates as a ring buffer of recent traces.
type TraceCollector struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	eventBus *EventBus
	repo     TraceRepository // optional; if non-nil, completed traces are persisted

	// Ring buffer of traces, keyed by TraceID
	traces     map[domain.TraceID]*domain.Trace
	spans      map[domain.SpanID]*domain.Span
	traceOrder []domain.TraceID // for eviction
}

// NewTraceCollector creates a new collector with optional EventBus for real-time events.
// repo may be nil; when provided, traces are persisted to DB on completion.
func NewTraceCollector(logger *slog.Logger, eventBus *EventBus, repo TraceRepository) *TraceCollector {
	return &TraceCollector{
		logger:   logger,
		eventBus: eventBus,
		repo:     repo,
		traces:   make(map[domain.TraceID]*domain.Trace, maxTraces),
		spans:    make(map[domain.SpanID]*domain.Span, maxTraces*10),
	}
}

// --- Context propagation ---

type traceCtxKey struct{}
type spanCtxKey struct{}

// ContextWithTrace stores trace and span IDs in context for propagation.
func ContextWithTrace(ctx context.Context, traceID domain.TraceID, spanID domain.SpanID) context.Context {
	ctx = context.WithValue(ctx, traceCtxKey{}, traceID)
	ctx = context.WithValue(ctx, spanCtxKey{}, spanID)
	return ctx
}

// TraceFromContext extracts trace and current span ID from context.
func TraceFromContext(ctx context.Context) (domain.TraceID, domain.SpanID, bool) {
	traceID, ok1 := ctx.Value(traceCtxKey{}).(domain.TraceID)
	spanID, ok2 := ctx.Value(spanCtxKey{}).(domain.SpanID)
	return traceID, spanID, ok1 && ok2
}

// --- Trace lifecycle ---

// StartTrace begins a new trace. Returns updated context with trace/span.
func (tc *TraceCollector) StartTrace(ctx context.Context, name string, attrs map[string]string) (context.Context, domain.TraceID, domain.SpanID) {
	traceID := domain.TraceID(uuid.New().String())
	rootSpanID := domain.SpanID(uuid.New().String())
	now := time.Now()

	rootSpan := &domain.Span{
		ID:         rootSpanID,
		TraceID:    traceID,
		Name:       name,
		Kind:       domain.SpanKindAgent,
		Status:     domain.SpanStatusRunning,
		Attributes: attrs,
		StartTime:  now,
	}

	trace := &domain.Trace{
		ID:         traceID,
		RootSpanID: rootSpanID,
		Name:       name,
		Status:     domain.SpanStatusRunning,
		StartTime:  now,
		SpanCount:  1,
	}

	tc.mu.Lock()
	tc.evictIfNeeded()
	tc.traces[traceID] = trace
	tc.spans[rootSpanID] = rootSpan
	tc.traceOrder = append(tc.traceOrder, traceID)
	tc.mu.Unlock()

	tc.publishEvent(traceID, "trace_start", map[string]interface{}{
		"trace_id": traceID,
		"name":     name,
	})

	tc.logger.Debug("trace started", "trace_id", string(traceID), "name", name)

	return ContextWithTrace(ctx, traceID, rootSpanID), traceID, rootSpanID
}

// EndTrace finalizes a trace.
func (tc *TraceCollector) EndTrace(traceID domain.TraceID, status domain.SpanStatus, errMsg string) {
	tc.mu.Lock()

	trace, ok := tc.traces[traceID]
	if !ok {
		tc.mu.Unlock()
		return
	}

	now := time.Now()
	trace.Status = status
	trace.EndTime = &now
	trace.DurationMs = now.Sub(trace.StartTime).Milliseconds()

	// Also end root span
	if root, ok := tc.spans[trace.RootSpanID]; ok {
		root.Status = status
		root.EndTime = &now
		root.DurationMs = now.Sub(root.StartTime).Milliseconds()
		if errMsg != "" {
			root.Error = errMsg
		}
	}

	tc.publishEvent(traceID, "trace_end", map[string]interface{}{
		"trace_id":    traceID,
		"status":      status,
		"duration_ms": trace.DurationMs,
	})

	// Build a copy for persistence (while still holding the lock for safe span iteration)
	var persistCopy *domain.Trace
	if tc.repo != nil {
		cp := *trace
		for _, span := range tc.spans {
			if span.TraceID == traceID {
				cp.Spans = append(cp.Spans, *span)
			}
		}
		persistCopy = &cp
	}

	tc.mu.Unlock()

	// Persist asynchronously to avoid blocking callers
	if persistCopy != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := tc.repo.SaveTrace(ctx, persistCopy); err != nil {
				tc.logger.Warn("failed to persist trace", "trace_id", traceID, "error", err)
			}
		}()
	}
}

// --- Span lifecycle ---

// StartSpan creates a child span under the current context's span.
func (tc *TraceCollector) StartSpan(ctx context.Context, name string, kind domain.SpanKind, attrs map[string]string) (context.Context, domain.SpanID) {
	traceID, parentSpanID, ok := TraceFromContext(ctx)
	if !ok {
		// No trace in context — return a no-op span
		return ctx, ""
	}

	spanID := domain.SpanID(uuid.New().String())
	now := time.Now()

	span := &domain.Span{
		ID:         spanID,
		ParentID:   parentSpanID,
		TraceID:    traceID,
		Name:       name,
		Kind:       kind,
		Status:     domain.SpanStatusRunning,
		Attributes: attrs,
		StartTime:  now,
	}

	tc.mu.Lock()
	tc.spans[spanID] = span

	// Link child to parent
	if parent, ok := tc.spans[parentSpanID]; ok {
		parent.Children = append(parent.Children, spanID)
	}

	// Update trace span count
	if trace, ok := tc.traces[traceID]; ok {
		trace.SpanCount++
	}
	tc.mu.Unlock()

	tc.publishEvent(traceID, "span_start", map[string]interface{}{
		"span_id":   spanID,
		"parent_id": parentSpanID,
		"name":      name,
		"kind":      kind,
	})

	return ContextWithTrace(ctx, traceID, spanID), spanID
}

// EndSpan finalizes a span with output and status.
func (tc *TraceCollector) EndSpan(spanID domain.SpanID, status domain.SpanStatus, output string, errMsg string) {
	if spanID == "" {
		return
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	span, ok := tc.spans[spanID]
	if !ok {
		return
	}

	now := time.Now()
	span.Status = status
	span.Output = truncate(output, maxInputOutput)
	span.EndTime = &now
	span.DurationMs = now.Sub(span.StartTime).Milliseconds()
	if errMsg != "" {
		span.Error = errMsg
	}

	tc.publishEvent(span.TraceID, "span_end", map[string]interface{}{
		"span_id":     spanID,
		"name":        span.Name,
		"kind":        span.Kind,
		"status":      status,
		"duration_ms": span.DurationMs,
	})
}

// SetSpanInput sets the input for a span (call after creation to avoid locking overhead).
func (tc *TraceCollector) SetSpanInput(spanID domain.SpanID, input string) {
	if spanID == "" {
		return
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if span, ok := tc.spans[spanID]; ok {
		span.Input = truncate(input, maxInputOutput)
	}
}

// SetSpanModel sets the model ID for an LLM span.
func (tc *TraceCollector) SetSpanModel(spanID domain.SpanID, model string) {
	if spanID == "" {
		return
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if span, ok := tc.spans[spanID]; ok {
		span.Model = model
	}
}

// SetTraceConversation associates a conversation ID with the trace.
func (tc *TraceCollector) SetTraceConversation(traceID domain.TraceID, convID string, personaID string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if trace, ok := tc.traces[traceID]; ok {
		trace.ConversationID = convID
		trace.PersonaID = personaID
	}
}

// --- Query ---

// ListTraces returns summaries of recent traces (newest first).
func (tc *TraceCollector) ListTraces(limit int) []domain.TraceSummary {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if limit <= 0 || limit > len(tc.traceOrder) {
		limit = len(tc.traceOrder)
	}

	result := make([]domain.TraceSummary, 0, limit)
	// Iterate in reverse (newest first)
	for i := len(tc.traceOrder) - 1; i >= 0 && len(result) < limit; i-- {
		tid := tc.traceOrder[i]
		if trace, ok := tc.traces[tid]; ok {
			result = append(result, domain.TraceSummary{
				ID:         trace.ID,
				Name:       trace.Name,
				Status:     trace.Status,
				StartTime:  trace.StartTime,
				DurationMs: trace.DurationMs,
				SpanCount:  trace.SpanCount,
			})
		}
	}
	return result
}

// GetTrace returns a full trace with all spans.
func (tc *TraceCollector) GetTrace(traceID domain.TraceID) (*domain.Trace, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	trace, ok := tc.traces[traceID]
	if !ok {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	// Collect all spans for this trace
	result := *trace // copy
	for _, span := range tc.spans {
		if span.TraceID == traceID {
			result.Spans = append(result.Spans, *span)
		}
	}

	return &result, nil
}

// --- Internal helpers ---

func (tc *TraceCollector) evictIfNeeded() {
	for len(tc.traceOrder) >= maxTraces {
		// Remove oldest trace and its spans
		oldID := tc.traceOrder[0]
		tc.traceOrder = tc.traceOrder[1:]

		if oldTrace, ok := tc.traces[oldID]; ok {
			// Remove all spans for this trace
			for sid, span := range tc.spans {
				if span.TraceID == oldTrace.ID {
					delete(tc.spans, sid)
				}
			}
			delete(tc.traces, oldID)
		}
	}
}

func (tc *TraceCollector) publishEvent(traceID domain.TraceID, eventType string, data map[string]interface{}) {
	if tc.eventBus == nil {
		return
	}

	payload, _ := json.Marshal(data)
	tc.eventBus.Publish(Event{
		JobID:     "trace:" + string(traceID),
		Type:      EventType("trace_" + eventType),
		Data:      string(payload),
		Timestamp: time.Now().UnixMilli(),
	})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}
