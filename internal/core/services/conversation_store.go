package services

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/ports"
)

// ConversationStore manages conversations with an in-memory cache backed by DuckDB.
// Hot conversations stay in memory; cold ones are loaded on-demand.
type ConversationStore struct {
	mu   sync.RWMutex
	repo ports.Repository

	// In-memory LRU cache: conversationID -> messages (ordered by time)
	cache    map[domain.ConversationID][]domain.Message
	order    []domain.ConversationID // LRU order, most recent last
	maxCache int                     // max conversations in memory
}

// NewConversationStore creates a new store with the given cache capacity.
func NewConversationStore(repo ports.Repository, maxCache int) *ConversationStore {
	if maxCache <= 0 {
		maxCache = 64
	}
	return &ConversationStore{
		repo:     repo,
		cache:    make(map[domain.ConversationID][]domain.Message, maxCache),
		order:    make([]domain.ConversationID, 0, maxCache),
		maxCache: maxCache,
	}
}

// CreateConversation initializes a new conversation. Returns its ID.
func (s *ConversationStore) CreateConversation(ctx context.Context, title string) (domain.Conversation, error) {
	now := time.Now()
	conv := domain.Conversation{
		ID:        domain.NewConversationID(),
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.CreateConversation(ctx, conv); err != nil {
		return domain.Conversation{}, err
	}

	s.mu.Lock()
	s.cache[conv.ID] = nil // empty messages slice
	s.touchLocked(conv.ID)
	s.evictLocked()
	s.mu.Unlock()

	return conv, nil
}

// GetConversation returns conversation metadata.
func (s *ConversationStore) GetConversation(ctx context.Context, id domain.ConversationID) (domain.Conversation, error) {
	return s.repo.GetConversation(ctx, id)
}

// ListConversations returns all conversations, most recently updated first.
func (s *ConversationStore) ListConversations(ctx context.Context) ([]domain.Conversation, error) {
	return s.repo.ListConversations(ctx)
}

// DeleteConversation removes conversation and its messages.
func (s *ConversationStore) DeleteConversation(ctx context.Context, id domain.ConversationID) error {
	if err := s.repo.DeleteConversation(ctx, id); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.cache, id)
	s.removeLRULocked(id)
	s.mu.Unlock()

	return nil
}

// UpdateTitle updates the conversation title.
func (s *ConversationStore) UpdateTitle(ctx context.Context, id domain.ConversationID, title string) error {
	return s.repo.UpdateConversationTitle(ctx, id, title)
}

// AddMessage persists a message and updates the in-memory cache.
func (s *ConversationStore) AddMessage(ctx context.Context, msg domain.Message) error {
	if err := s.repo.AddMessage(ctx, msg); err != nil {
		return err
	}

	s.mu.Lock()
	if msgs, ok := s.cache[msg.ConversationID]; ok {
		s.cache[msg.ConversationID] = append(msgs, msg)
	}
	// If not cached, don't load — will be fetched on next GetMessages
	s.touchLocked(msg.ConversationID)
	s.mu.Unlock()

	return nil
}

// GetMessages returns messages for a conversation, using cache when available.
// limit=0 means all messages.
func (s *ConversationStore) GetMessages(ctx context.Context, convID domain.ConversationID, limit int) ([]domain.Message, error) {
	s.mu.RLock()
	if msgs, ok := s.cache[convID]; ok && limit == 0 {
		// Return cached copy
		result := make([]domain.Message, len(msgs))
		copy(result, msgs)
		s.mu.RUnlock()
		return result, nil
	}
	s.mu.RUnlock()

	// Load from DB
	msgs, err := s.repo.ListMessages(ctx, convID, limit)
	if err != nil {
		return nil, err
	}

	// Populate cache (full set only)
	if limit == 0 {
		s.mu.Lock()
		s.cache[convID] = msgs
		s.touchLocked(convID)
		s.evictLocked()
		s.mu.Unlock()
	}

	return msgs, nil
}

// BuildContextWindow returns the last N messages formatted as a prompt context string.
// Uses a sliding window approach — keeps system + last N user/assistant turns.
func (s *ConversationStore) BuildContextWindow(ctx context.Context, convID domain.ConversationID, maxMessages int) (string, error) {
	if maxMessages <= 0 {
		maxMessages = 20
	}

	msgs, err := s.repo.ListMessages(ctx, convID, maxMessages)
	if err != nil {
		return "", err
	}

	if len(msgs) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.Grow(len(msgs) * 200) // pre-allocate rough estimate

	for _, msg := range msgs {
		switch msg.Role {
		case domain.RoleUser:
			sb.WriteString("User: ")
			sb.WriteString(msg.Content)
		case domain.RoleAssistant:
			sb.WriteString("Assistant: ")
			sb.WriteString(msg.Content)
		case domain.RoleTool:
			sb.WriteString("Observation: ")
			sb.WriteString(msg.Content)
		case domain.RoleSystem:
			sb.WriteString("System: ")
			sb.WriteString(msg.Content)
		}
		sb.WriteByte('\n')
	}

	return sb.String(), nil
}

// --- LRU helpers (must be called with mu held) ---

func (s *ConversationStore) touchLocked(id domain.ConversationID) {
	// Remove from current position
	s.removeLRULocked(id)
	// Add to end (most recent)
	s.order = append(s.order, id)
}

func (s *ConversationStore) removeLRULocked(id domain.ConversationID) {
	for i, v := range s.order {
		if v == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			return
		}
	}
}

func (s *ConversationStore) evictLocked() {
	for len(s.order) > s.maxCache {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.cache, oldest)
	}
}
