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
	router   *ModelRouter
	tools    *domain.ToolRegistry
	convs    *ConversationStore
	repo     personaReader
	maxIters int
}

// personaReader is the minimal interface needed to fetch personas
type personaReader interface {
	GetPersona(ctx context.Context, id domain.PersonaID) (domain.Persona, error)
}

// NewReActAgentService creates a new ReAct-enabled agent
func NewReActAgentService(
	logger *slog.Logger,
	llm domain.LLMProvider,
	router *ModelRouter,
	tools *domain.ToolRegistry,
	convs *ConversationStore,
	repo personaReader,
) *ReActAgentService {
	return &ReActAgentService{
		logger:   logger,
		llm:      llm,
		router:   router,
		tools:    tools,
		convs:    convs,
		repo:     repo,
		maxIters: 5,
	}
}

// Chat processes a user message using ReAct reasoning, within a conversation context.
// If convID is empty, it creates a new conversation automatically.
// If personaID is provided, the agent uses the persona's system prompt and tool filter.
func (s *ReActAgentService) Chat(ctx context.Context, convID domain.ConversationID, message string, personaID *domain.PersonaID) (*domain.AgentResponse, domain.ConversationID, error) {
	s.logger.Info("starting ReAct loop", "message", message, "conversation_id", string(convID))

	// Resolve persona
	var persona *domain.Persona
	if personaID != nil {
		p, err := s.repo.GetPersona(ctx, *personaID)
		if err == nil {
			persona = &p
			s.logger.Info("using persona", "persona_id", string(p.ID), "persona_name", p.Name)
		} else {
			s.logger.Warn("persona not found, using default", "persona_id", string(*personaID), "error", err)
		}
	}

	// Auto-create conversation if needed
	if convID == "" {
		// Generate title from first ~50 chars of message
		title := message
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		conv, err := s.convs.CreateConversationWithPersona(ctx, title, personaID)
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
		s.buildReActPrompt(history, message, persona),
	}
	steps := []domain.ReActStep{}

	// Build effective tool registry (filtered by persona if applicable)
	effectiveTools := s.tools
	if persona != nil && len(persona.AllowedTools) > 0 {
		effectiveTools = s.tools.FilterByNames(persona.AllowedTools)
	}

	// Resolve model: persona override > default
	modelID := ""
	if s.router != nil && persona != nil {
		role := s.router.inferRoleFromPersona(persona)
		modelID = s.router.ResolveModel(persona, role)
	}

	// Inject conversation ID into context for sub-agent tools
	ctx = ContextWithConversation(ctx, convID)

	// Inject ProjectID into context if available
	// Ensure we have the latest conversation state (although convID is usually sufficient, project might be loaded)
	// We need to fetch the conversation metadata to get the ProjectID
	if currentConv, err := s.convs.GetConversation(ctx, convID); err == nil && currentConv.ProjectID != nil {
		ctx = ContextWithProject(ctx, *currentConv.ProjectID)
		s.logger.Info("context injected with project_id", "project_id", string(*currentConv.ProjectID))
	}

	for i := 0; i < s.maxIters; i++ {
		s.logger.Info("ReAct iteration", "iteration", i+1)

		// 1. Call LLM (with model override if available)
		prompt := strings.Join(conversationHistory, "\n\n")
		var response string
		var err error
		if s.router != nil && modelID != "" {
			response, err = s.router.GenerateText(ctx, prompt, modelID)
		} else {
			response, err = s.llm.GenerateText(ctx, prompt)
		}
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
		result, err := effectiveTools.Execute(ctx, step.Action, step.ActionInput)
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
func (s *ReActAgentService) buildReActPrompt(history string, userMessage string, persona *domain.Persona) string {
	// Choose effective tool set for prompt
	var toolsDesc string
	if persona != nil && len(persona.AllowedTools) > 0 {
		toolsDesc = s.tools.FilterByNames(persona.AllowedTools).FormatToolsForPrompt()
	} else {
		toolsDesc = s.tools.FormatToolsForPrompt()
	}

	// Build system identity from persona or default
	systemIdentity := "You are an AI assistant with access to tools."
	if persona != nil && persona.SystemPrompt != "" {
		systemIdentity = persona.SystemPrompt
	}

	var historyBlock string
	if history != "" {
		historyBlock = fmt.Sprintf(`
Previous conversation:
%s
---
`, history)
	}

	return fmt.Sprintf(`%s

You use the ReAct pattern: Thought → Action → Observation → ... → Final Answer.

FORMAT (tool call):
Thought: <reasoning>
Action: <tool_name>
Action Input: <JSON params>

FORMAT (direct answer):
Thought: <reasoning>
Final Answer: <response>

%s

RULES:
1. Always start with "Thought:"
2. For simple chat (greetings, questions, conversation), go DIRECTLY to "Final Answer:" — no tools needed.
3. Only use tools when the user explicitly asks for something requiring them.
4. Tools generate_image and generate_text are ASYNC. When "status":"queued", tell user to wait.
5. When "status":"unavailable", the service is down — tell user clearly.
6. Action Input must be valid JSON on one line.

Example 1 — simple chat:
User: Hello!
Thought: Simple greeting, no tool needed.
Final Answer: Hello! How can I help you today?

Example 2 — image generation:
User: Generate an image of a sunset
Thought: I need to use generate_image for this request.
Action: generate_image
Action Input: {"prompt": "beautiful sunset over ocean"}
%s
Now respond to:
User: %s`, systemIdentity, toolsDesc, historyBlock, userMessage)
}

// parseReActOutput extracts Thought/Action/ActionInput or FinalAnswer from LLM response
func (s *ReActAgentService) parseReActOutput(response string) domain.ReActStep {
	step := domain.ReActStep{}

	// Check for Final Answer first
	finalAnswerRe := regexp.MustCompile(`(?is)Final\s*Answer:\s*(.*)`)
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
	actionRe := regexp.MustCompile(`(?i)Action:\s*([a-z][a-z0-9_]*)`)
	if matches := actionRe.FindStringSubmatch(response); len(matches) > 1 {
		step.Action = strings.TrimSpace(matches[1])
	}

	// Extract Action Input — use brace-matching for nested JSON
	step.ActionInput = s.extractActionInput(response)

	return step
}

// extractActionInput extracts the JSON object from "Action Input: {...}" using brace-depth counting
// to handle nested JSON objects correctly.
func (s *ReActAgentService) extractActionInput(response string) map[string]interface{} {
	// Find "Action Input:" prefix (case-insensitive)
	aiRe := regexp.MustCompile(`(?i)Action\s*Input:\s*`)
	loc := aiRe.FindStringIndex(response)
	if loc == nil {
		return nil
	}

	// Find the first '{' after the prefix
	rest := response[loc[1]:]
	start := strings.Index(rest, "{")
	if start < 0 {
		return nil
	}

	// Count braces to find matching '}'
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(rest); i++ {
		ch := rest[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inStr {
			escaped = true
			continue
		}
		if ch == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				jsonStr := rest[start : i+1]
				var params map[string]interface{}
				if parseErr := json.Unmarshal([]byte(jsonStr), &params); parseErr == nil {
					return params
				} else {
					s.logger.Warn("failed to parse action input JSON", "error", parseErr, "json", jsonStr)
					return map[string]interface{}{"raw": jsonStr}
				}
			}
		}
	}

	// Fallback: try a simple regex as last resort
	simpleRe := regexp.MustCompile(`\{[^}]+\}`)
	if match := simpleRe.FindString(rest); match != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(match), &params); err == nil {
			return params
		}
	}

	return nil
}
