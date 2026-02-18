package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// SystemChat is the kernel's proactive messaging channel.
// It writes to a fixed conversation (conv-kernel-system) that appears in the UI
// as the "Kernel" inbox. The frontend subscribes to SSE for that conversation ID
// and receives real-time notifications, suggestions, and questions.
type SystemChat struct {
	logger    *slog.Logger
	convStore *ConversationStore
	eventBus  *EventBus
	llm       domain.LLMProvider // optional: used to generate suggestions

	initOnce sync.Once
	initErr  error
}

// KernelInboxStatus is the payload for GET /v1/system/inbox.
type KernelInboxStatus struct {
	ConversationID string          `json:"conversation_id"`
	UnreadCount    int             `json:"unread_count"`
	LastMessage    *domain.Message `json:"last_message,omitempty"`
}

// NewSystemChat creates the SystemChat service.
// llm may be nil — suggestions will be skipped if not provided.
func NewSystemChat(
	logger *slog.Logger,
	convStore *ConversationStore,
	eventBus *EventBus,
	llm domain.LLMProvider,
) *SystemChat {
	return &SystemChat{
		logger:    logger,
		convStore: convStore,
		eventBus:  eventBus,
		llm:       llm,
	}
}

// ensureConv creates the system conversation if it doesn't exist yet.
func (s *SystemChat) ensureConv(ctx context.Context) error {
	s.initOnce.Do(func() {
		s.initErr = s.convStore.EnsureConversation(ctx, domain.SystemConversationID, "Kernel")
	})
	return s.initErr
}

// post writes a kernel-role message and pushes a real-time event.
func (s *SystemChat) post(ctx context.Context, content string, meta map[string]interface{}) {
	if err := s.ensureConv(ctx); err != nil {
		s.logger.Error("system_chat: failed to ensure conv", "error", err)
		return
	}

	now := time.Now()
	msg := domain.Message{
		ID:             domain.NewMessageID(),
		ConversationID: domain.SystemConversationID,
		Role:           domain.RoleKernel,
		Content:        content,
		Metadata:       meta,
		CreatedAt:      now,
	}
	if err := s.convStore.AddMessage(ctx, msg); err != nil {
		s.logger.Error("system_chat: failed to persist message", "error", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"role":       "kernel",
		"content":    content,
		"message_id": string(msg.ID),
		"metadata":   meta,
	})
	s.eventBus.Publish(Event{
		JobID:     string(domain.SystemConversationID),
		Type:      "kernel_message",
		Data:      string(payload),
		Timestamp: now.Unix(),
	})
}

// Notify posts an informational message to the kernel inbox.
func (s *SystemChat) Notify(ctx context.Context, content string) {
	s.post(ctx, content, map[string]interface{}{"kind": "info"})
}

// NotifyJobResult posts a job completion or failure notification.
func (s *SystemChat) NotifyJobResult(ctx context.Context, jobID string, status string, detail string) {
	var icon string
	switch status {
	case "COMPLETED":
		icon = "✅"
	case "FAILED":
		icon = "❌"
	default:
		icon = "ℹ️"
	}
	content := fmt.Sprintf("%s Job `%s` **%s**", icon, jobID, status)
	if detail != "" {
		content += "\n\n" + detail
	}
	s.post(ctx, content, map[string]interface{}{
		"kind":   "job_result",
		"job_id": jobID,
		"status": status,
	})
}

// Ask posts a question to the user.
// The user's reply goes back as a normal chat message in the system conversation.
func (s *SystemChat) Ask(ctx context.Context, question string) {
	s.post(ctx, "❓ "+question, map[string]interface{}{"kind": "question"})
}

// Suggest posts an AI-generated suggestion based on recent context.
// If llm is nil, posts a pre-canned hint instead.
func (s *SystemChat) Suggest(ctx context.Context, contextSummary string) {
	if s.llm == nil {
		s.Notify(ctx, "💡 Tip: try asking me to delegate tasks to specialized agents using the `delegate` tool.")
		return
	}
	prompt := fmt.Sprintf(`You are a proactive AI assistant embedded in auleOS.
Based on the following recent activity summary, write ONE short, actionable suggestion for the user.
Be concise (1-2 sentences), friendly, and specific. Start with a relevant emoji.

Activity:
%s

Suggestion:`, contextSummary)

	result, err := s.llm.GenerateText(ctx, prompt)
	if err != nil {
		s.logger.Warn("system_chat: failed to generate suggestion", "error", err)
		return
	}
	s.post(ctx, result, map[string]interface{}{"kind": "suggestion"})
}

// GetStatus returns unread count and last message for the inbox badge.
func (s *SystemChat) GetStatus(ctx context.Context) KernelInboxStatus {
	_ = s.ensureConv(ctx)

	msgs, err := s.convStore.GetMessages(ctx, domain.SystemConversationID, 50)
	if err != nil {
		return KernelInboxStatus{ConversationID: string(domain.SystemConversationID)}
	}

	// Unread = kernel messages after last user message
	lastUserIdx := -1
	for i, m := range msgs {
		if m.Role == domain.RoleUser {
			lastUserIdx = i
		}
	}
	unread := 0
	for i := lastUserIdx + 1; i < len(msgs); i++ {
		if msgs[i].Role == domain.RoleKernel {
			unread++
		}
	}

	status := KernelInboxStatus{
		ConversationID: string(domain.SystemConversationID),
		UnreadCount:    unread,
	}
	if len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		status.LastMessage = &last
	}
	return status
}

// WelcomeIfNew posts a welcome message on first startup (no prior messages).
func (s *SystemChat) WelcomeIfNew(ctx context.Context) {
	if err := s.ensureConv(ctx); err != nil {
		return
	}
	msgs, _ := s.convStore.GetMessages(ctx, domain.SystemConversationID, 1)
	if len(msgs) > 0 {
		return // already initialized
	}
	s.post(ctx,
		"👋 **Kernel online.** I'll notify you here about job completions, agent activity, and suggestions.\n\nYou can also talk to me — I'm watching your activity.",
		map[string]interface{}{"kind": "welcome"},
	)
}
