package kernel

import (
	"context"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// --- StrictServerInterface implementations for Conversations ---

// ListConversations implements StrictServerInterface.
func (s *Server) ListConversations(ctx context.Context, _ ListConversationsRequestObject) (ListConversationsResponseObject, error) {
	convs, err := s.convStore.ListConversations(ctx)
	if err != nil {
		s.logger.Error("failed to list conversations", "error", err)
		return ListConversations200JSONResponse{}, nil
	}

	result := make(ListConversations200JSONResponse, len(convs))
	for i, c := range convs {
		result[i] = domainConvToAPI(c)
	}

	return result, nil
}

// CreateConversation implements StrictServerInterface.
func (s *Server) CreateConversation(ctx context.Context, request CreateConversationRequestObject) (CreateConversationResponseObject, error) {
	title := "New Chat"
	if request.Body != nil && request.Body.Title != nil {
		title = *request.Body.Title
	}

	conv, err := s.convStore.CreateConversation(ctx, title)
	if err != nil {
		s.logger.Error("failed to create conversation", "error", err)
		return nil, err
	}

	return CreateConversation201JSONResponse(domainConvToAPI(conv)), nil
}

// GetConversation implements StrictServerInterface.
func (s *Server) GetConversation(ctx context.Context, request GetConversationRequestObject) (GetConversationResponseObject, error) {
	conv, err := s.convStore.GetConversation(ctx, domain.ConversationID(request.Id))
	if err != nil {
		if err == domain.ErrConversationNotFound {
			msg := "conversation not found"
			return GetConversation404JSONResponse{Error: &msg}, nil
		}
		s.logger.Error("failed to get conversation", "error", err)
		return nil, err
	}

	return GetConversation200JSONResponse(domainConvToAPI(conv)), nil
}

// DeleteConversation implements StrictServerInterface.
func (s *Server) DeleteConversation(ctx context.Context, request DeleteConversationRequestObject) (DeleteConversationResponseObject, error) {
	if err := s.convStore.DeleteConversation(ctx, domain.ConversationID(request.Id)); err != nil {
		if err == domain.ErrConversationNotFound {
			return DeleteConversation404Response{}, nil
		}
		s.logger.Error("failed to delete conversation", "error", err)
		return nil, err
	}

	return DeleteConversation204Response{}, nil
}

// UpdateConversation implements StrictServerInterface.
func (s *Server) UpdateConversation(ctx context.Context, request UpdateConversationRequestObject) (UpdateConversationResponseObject, error) {
	id := domain.ConversationID(request.Id)

	if request.Body != nil && request.Body.Title != nil {
		if err := s.convStore.UpdateTitle(ctx, id, *request.Body.Title); err != nil {
			if err == domain.ErrConversationNotFound {
				return nil, err
			}
			s.logger.Error("failed to update conversation", "error", err)
			return nil, err
		}
	}

	conv, err := s.convStore.GetConversation(ctx, id)
	if err != nil {
		return nil, err
	}

	return UpdateConversation200JSONResponse(domainConvToAPI(conv)), nil
}

// ListMessages implements StrictServerInterface.
func (s *Server) ListMessages(ctx context.Context, request ListMessagesRequestObject) (ListMessagesResponseObject, error) {
	limit := 50
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}

	msgs, err := s.convStore.GetMessages(ctx, domain.ConversationID(request.Id), limit)
	if err != nil {
		s.logger.Error("failed to list messages", "error", err)
		return ListMessages200JSONResponse{}, nil
	}

	result := make(ListMessages200JSONResponse, len(msgs))
	for i, m := range msgs {
		result[i] = domainMsgToAPI(m)
	}

	return result, nil
}

// --- Mapping helpers ---

func domainConvToAPI(c domain.Conversation) Conversation {
	id := string(c.ID)
	title := c.Title
	createdAt := c.CreatedAt
	updatedAt := c.UpdatedAt
	conv := Conversation{
		Id:        &id,
		Title:     &title,
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	if c.ProjectID != nil {
		pid := string(*c.ProjectID)
		conv.ProjectId = &pid
	}
	if c.PersonaID != nil {
		pid := string(*c.PersonaID)
		conv.PersonaId = &pid
	}
	return conv
}

func domainMsgToAPI(m domain.Message) Message {
	id := string(m.ID)
	convID := string(m.ConversationID)
	role := MessageRole(m.Role)
	content := m.Content
	createdAt := m.CreatedAt

	msg := Message{
		Id:             &id,
		ConversationId: &convID,
		Role:           &role,
		Content:        &content,
		CreatedAt:      &createdAt,
	}

	if m.Thought != "" {
		msg.Thought = &m.Thought
	}

	if len(m.Steps) > 0 {
		apiSteps := make([]ReActStep, len(m.Steps))
		for j, step := range m.Steps {
			apiSteps[j] = ReActStep{}
			if step.Thought != "" {
				v := step.Thought
				apiSteps[j].Thought = &v
			}
			if step.Action != "" {
				v := step.Action
				apiSteps[j].Action = &v
			}
			if step.ActionInput != nil {
				v := step.ActionInput
				apiSteps[j].ActionInput = &v
			}
			if step.Observation != "" {
				v := step.Observation
				apiSteps[j].Observation = &v
			}
			if step.FinalAnswer != "" {
				v := step.FinalAnswer
				apiSteps[j].FinalAnswer = &v
			}
			if step.IsFinalAnswer {
				v := step.IsFinalAnswer
				apiSteps[j].IsFinalAnswer = &v
			}
		}
		msg.Steps = &apiSteps
	}

	if m.ToolCall != nil {
		msg.ToolCall = &struct {
			Args *map[string]interface{} `json:"args,omitempty"`
			Name *string                 `json:"name,omitempty"`
		}{
			Name: &m.ToolCall.Name,
			Args: &m.ToolCall.Args,
		}
	}

	return msg
}
