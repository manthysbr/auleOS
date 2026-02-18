package domain

import "time"

// TraceID uniquely identifies a trace (one per agent chat or workflow execution).
type TraceID string

// SpanID uniquely identifies a span within a trace.
type SpanID string

// SpanKind classifies the type of operation a span represents.
type SpanKind string

const (
	SpanKindAgent    SpanKind = "agent"     // Top-level agent invocation
	SpanKindLLM      SpanKind = "llm"       // LLM call (generate text)
	SpanKindTool     SpanKind = "tool"      // Tool execution
	SpanKindSubAgent SpanKind = "sub_agent" // Delegated sub-agent
	SpanKindWorkflow SpanKind = "workflow"  // Workflow execution
	SpanKindStep     SpanKind = "step"      // Workflow step
)

// SpanStatus indicates completion state of a span.
type SpanStatus string

const (
	SpanStatusRunning   SpanStatus = "running"
	SpanStatusOK        SpanStatus = "ok"
	SpanStatusError     SpanStatus = "error"
	SpanStatusCancelled SpanStatus = "cancelled"
)

// Span represents a single unit of work within a trace.
// Spans form a tree: an agent span contains LLM + tool child spans.
type Span struct {
	ID         SpanID                 `json:"id"`
	ParentID   SpanID                 `json:"parent_id,omitempty"` // empty = root
	TraceID    TraceID                `json:"trace_id"`
	Name       string                 `json:"name"` // e.g., "llm.generate", "tool.exec", "agent.chat"
	Kind       SpanKind               `json:"kind"` // agent, llm, tool, sub_agent, workflow, step
	Status     SpanStatus             `json:"status"`
	Input      string                 `json:"input,omitempty"`  // truncated input
	Output     string                 `json:"output,omitempty"` // truncated output
	Error      string                 `json:"error,omitempty"`
	Model      string                 `json:"model,omitempty"` // LLM model used (for llm spans)
	Attributes map[string]string      `json:"attributes,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    *time.Time             `json:"end_time,omitempty"`
	DurationMs int64                  `json:"duration_ms,omitempty"`
	Children   []SpanID               `json:"children,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Trace groups all spans of a single operation (chat, workflow run, etc.).
type Trace struct {
	ID             TraceID    `json:"id"`
	RootSpanID     SpanID     `json:"root_span_id"`
	Name           string     `json:"name"` // e.g., "chat: hello world", "workflow: pipeline-x"
	Status         SpanStatus `json:"status"`
	ConversationID string     `json:"conversation_id,omitempty"`
	PersonaID      string     `json:"persona_id,omitempty"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	DurationMs     int64      `json:"duration_ms,omitempty"`
	SpanCount      int        `json:"span_count"`
	Spans          []Span     `json:"spans,omitempty"` // populated only on detail view
}

// TraceSummary is a lightweight view for listing traces.
type TraceSummary struct {
	ID         TraceID    `json:"id"`
	Name       string     `json:"name"`
	Status     SpanStatus `json:"status"`
	StartTime  time.Time  `json:"start_time"`
	DurationMs int64      `json:"duration_ms"`
	SpanCount  int        `json:"span_count"`
}
