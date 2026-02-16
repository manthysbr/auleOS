package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// ReActAgentService implements agentic reasoning with tool use
type ReActAgentService struct {
	logger   *slog.Logger
	llm      domain.LLMProvider
	tools    *domain.ToolRegistry
	convs    *ConversationStore
	maxIters int
}

// NewReActAgentService creates a new ReAct-enabled agent
func NewReActAgentService(
	logger *slog.Logger,
	llm domain.LLMProvider,
	tools *domain.ToolRegistry,
	convs *ConversationStore,
) *ReActAgentService {
	return &ReActAgentService{
		logger:   logger,
		llm:      llm,
		tools:    tools,
		convs:    convs,
		maxIters: 5,
	}
}

// Chat processes a user message using ReAct reasoning, within a conversation context.
// If convID is empty, it creates a new conversation automatically.
func (s *ReActAgentService) Chat(ctx context.Context, convID domain.ConversationID, message string) (*domain.AgentResponse, domain.ConversationID, error) {
	s.logger.Info("starting ReAct loop", "message", message, "conversation_id", string(convID))

	// Auto-create conversation if needed
	if convID == "" {
		// Generate title from first ~50 chars of message
		title := message
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		conv, err := s.convs.CreateConversation(ctx, title)
		if err != nil {
			return nil, "", fmt.Errorf("create conversation: %w", err)
		}
		convID = conv.ID
		s.logger.Info("auto-created conversation", "conversation_id", string(convID))
	}

	// Persist user message
	now := time.Now()
	userMsg := domain.Message{
		ID:             domain.NewMessageID(),
		ConversationID: convID,
		Role:           domain.RoleUser,
		Content:        message,
		CreatedAt:      now,
	}
	if err := s.convs.AddMessage(ctx, userMsg); err != nil {
		return nil, convID, fmt.Errorf("persist user message: %w", err)
	}

	// Build context: system prompt + conversation history + new user message
	history, err := s.convs.BuildContextWindow(ctx, convID, 20)
	if err != nil {
		return nil, convID, fmt.Errorf("build context: %w", err)
	}

	conversationHistory := []string{
		s.buildReActPrompt(history, message),
	}
	steps := []domain.ReActStep{}

	for i := 0; i < s.maxIters; i++ {
		s.logger.Info("ReAct iteration", "iteration", i+1)

		// 1. Call LLM
		prompt := strings.Join(conversationHistory, "\n\n")
		response, err := s.llm.GenerateText(ctx, prompt)
		if err != nil {
			return nil, convID, fmt.Errorf("llm generate: %w", err)
		}

		s.logger.Info("LLM response", "response", response[:min(200, len(response))])

		// 2. Parse output
		step := s.parseReActOutput(response)
		steps = append(steps, step)

		// 3. Check if final answer
		if step.IsFinalAnswer {
			s.logger.Info("final answer reached", "answer", step.FinalAnswer)

			agentResp := &domain.AgentResponse{
				Response: step.FinalAnswer,
				Thought:  step.Thought,
				Steps:    steps,
			}

			// Persist assistant message
			assistantMsg := domain.Message{
				ID:             domain.NewMessageID(),
				ConversationID: convID,
				Role:           domain.RoleAssistant,
				Content:        step.FinalAnswer,
				Thought:        step.Thought,
				Steps:          steps,
				CreatedAt:      time.Now(),
			}
			if err := s.convs.AddMessage(ctx, assistantMsg); err != nil {
				s.logger.Error("failed to persist assistant message", "error", err)
			}

			return agentResp, convID, nil
		}

		// 4. Execute tool
		s.logger.Info("executing tool", "tool", step.Action, "params", step.ActionInput)
		result, err := s.tools.Execute(ctx, step.Action, step.ActionInput)
		if err != nil {
			step.Observation = fmt.Sprintf("Error: %v", err)
		} else {
			// Format observation
			resultJSON, _ := json.Marshal(result)
			step.Observation = string(resultJSON)
		}

		s.logger.Info("tool executed", "observation", step.Observation[:min(200, len(step.Observation))])

		// 5. Add to conversation
		conversationHistory = append(conversationHistory, response)
		conversationHistory = append(conversationHistory, fmt.Sprintf("Observation: %s", step.Observation))
	}

	return nil, convID, fmt.Errorf("max iterations (%d) reached without final answer", s.maxIters)
}

// buildReActPrompt creates the initial prompt with tool descriptions and conversation history
func (s *ReActAgentService) buildReActPrompt(history string, userMessage string) string {
	toolsDesc := s.tools.FormatToolsForPrompt()

	var historyBlock string
	if history != "" {
		historyBlock = fmt.Sprintf(`
Previous conversation:
%s
---
`, history)
	}

	return fmt.Sprintf(`You are an AI assistant with access to tools. Use the ReAct (Reasoning + Acting) pattern to solve tasks.

IMPORTANT FORMAT:
Thought: [your reasoning about what to do next]
Action: [tool name]
Action Input: [JSON object with tool parameters]

After receiving an Observation, continue thinking or provide Final Answer.

%s

RULES:
1. Always start with "Thought:"
2. Use "Action:" for tool name only
3. Use "Action Input:" with valid JSON
4. When done, use "Final Answer:" instead of Action

Example:
User: Generate an image of a sunset
Thought: I need to create an image using the generate_image tool
Action: generate_image
Action Input: {"prompt": "beautiful sunset over ocean"}

[System provides Observation]

Thought: I have successfully generated the image
Final Answer: Here's your sunset image: [observation shows URL]
%s
Now respond to this request:
User: %s`, toolsDesc, historyBlock, userMessage)
}

// parseReActOutput extracts Thought/Action/ActionInput or FinalAnswer from LLM response
func (s *ReActAgentService) parseReActOutput(response string) domain.ReActStep {
	step := domain.ReActStep{}

	// Check for Final Answer first
	finalAnswerRe := regexp.MustCompile(`(?i)Final Answer:\s*(.*)`)
	if matches := finalAnswerRe.FindStringSubmatch(response); len(matches) > 1 {
		step.IsFinalAnswer = true
		step.FinalAnswer = strings.TrimSpace(matches[1])

		// Extract thought if present
		thoughtRe := regexp.MustCompile(`(?i)Thought:\s*([^\n]+)`)
		if thoughtMatches := thoughtRe.FindStringSubmatch(response); len(thoughtMatches) > 1 {
			step.Thought = strings.TrimSpace(thoughtMatches[1])
		}

		return step
	}

	// Extract Thought
	thoughtRe := regexp.MustCompile(`(?i)Thought:\s*([^\n]+)`)
	if matches := thoughtRe.FindStringSubmatch(response); len(matches) > 1 {
		step.Thought = strings.TrimSpace(matches[1])
	}

	// Extract Action
	actionRe := regexp.MustCompile(`(?i)Action:\s*([a-z_]+)`)
	if matches := actionRe.FindStringSubmatch(response); len(matches) > 1 {
		step.Action = strings.TrimSpace(matches[1])
	}

	// Extract Action Input (JSON)
	actionInputRe := regexp.MustCompile(`(?i)Action Input:\s*(\{[^}]*\})`)
	if matches := actionInputRe.FindStringSubmatch(response); len(matches) > 1 {
		jsonStr := strings.TrimSpace(matches[1])
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &params); err == nil {
			step.ActionInput = params
		} else {
			s.logger.Warn("failed to parse action input JSON", "error", err, "json", jsonStr)
			step.ActionInput = map[string]interface{}{"raw": jsonStr}
		}
	}

	return step
}
