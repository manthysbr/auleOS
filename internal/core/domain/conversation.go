package domain

import (
	"errors"
	"time"

	"crypto/rand"
	"encoding/hex"
)

// ConversationID uniquely identifies a conversation
type ConversationID string

// MessageID uniquely identifies a message within a conversation
type MessageID string

// MessageRole defines who authored a message
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
	RoleKernel    MessageRole = "kernel" // system-initiated proactive messages
)

// SystemConversationID is the fixed ID for the kernel's proactive chat with the user
const SystemConversationID ConversationID = "conv-kernel-system"

// Conversation represents a multi-turn chat session
type Conversation struct {
	ID        ConversationID `json:"id"`
	ProjectID *ProjectID     `json:"project_id,omitempty"`
	PersonaID *PersonaID     `json:"persona_id,omitempty"`
	Title     string         `json:"title"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// Message represents a single turn in a conversation
type Message struct {
	ID             MessageID              `json:"id"`
	ConversationID ConversationID         `json:"conversation_id"`
	Role           MessageRole            `json:"role"`
	Content        string                 `json:"content"`
	Thought        string                 `json:"thought,omitempty"`
	Steps          []ReActStep            `json:"steps,omitempty"`
	ToolCall       *ToolCall              `json:"tool_call,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrMessageNotFound      = errors.New("message not found")
)

// NewConversationID generates a compact random conversation ID (conv-<12 hex>)
func NewConversationID() ConversationID {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return ConversationID("conv-" + hex.EncodeToString(b))
}

// NewMessageID generates a compact random message ID (msg-<12 hex>)
func NewMessageID() MessageID {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return MessageID("msg-" + hex.EncodeToString(b))
}
