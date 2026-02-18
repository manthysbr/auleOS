package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	ws       *WorkspaceManager
	tracer   *TraceCollector
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
	ws *WorkspaceManager,
	tracer *TraceCollector,
) *ReActAgentService {
	return &ReActAgentService{
		logger:   logger,
		llm:      llm,
		router:   router,
		tools:    tools,
		convs:    convs,
		repo:     repo,
		ws:       ws,
		tracer:   tracer,
		maxIters: 5,
	}
}

// Chat processes a user message using ReAct reasoning, within a conversation context.
// If convID is empty, it creates a new conversation automatically.
// If personaID is provided, the agent uses the persona's system prompt and tool filter.
func (s *ReActAgentService) Chat(ctx context.Context, convID domain.ConversationID, message string, personaID *domain.PersonaID) (*domain.AgentResponse, domain.ConversationID, error) {
	s.logger.Info("starting ReAct loop", "message", message, "conversation_id", string(convID))

	// --- Start Trace ---
	traceName := "chat: " + message
	if len(traceName) > 80 {
		traceName = traceName[:80] + "..."
	}
	traceAttrs := map[string]string{"conversation_id": string(convID)}
	if personaID != nil {
		traceAttrs["persona_id"] = string(*personaID)
	}
	ctx, traceID, _ := s.tracer.StartTrace(ctx, traceName, traceAttrs)
	defer func() {
		// EndTrace is called explicitly below — this is a safety net
	}()

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

	s.tracer.SetTraceConversation(traceID, string(convID), func() string {
		if personaID != nil {
			return string(*personaID)
		}
		return ""
	}())

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

	// Inject ProjectID into context and load workspace context (AGENT.md, USER.md, IDENTITY.md, MEMORY.md, skills)
	var wsCtx WorkspaceContext
	if currentConv, err := s.convs.GetConversation(ctx, convID); err == nil && currentConv.ProjectID != nil {
		projectID := *currentConv.ProjectID
		ctx = ContextWithProject(ctx, projectID)
		s.logger.Info("context injected with project_id", "project_id", string(projectID))

		// Load all workspace personality/context files
		wsCtx = LoadWorkspaceContext(s.ws, string(projectID), s.logger)

		// Load skills summary if skills directory exists
		if s.ws != nil {
			projPath := s.ws.GetProjectPath(string(projectID))
			homeDir, _ := os.UserHomeDir()
			globalSkills := filepath.Join(homeDir, ".aule", "skills")
			wsCtx.Skills = NewSkillsLoader(s.logger).BuildSkillsSummary(
				filepath.Join(projPath, "skills"),
				globalSkills,
				"",
			)
		}
	}

	// Build context: system prompt + conversation history + new user message
	history, err := s.convs.BuildContextWindow(ctx, convID, 20)
	if err != nil {
		return nil, convID, fmt.Errorf("build context: %w", err)
	}

	conversationHistory := []string{
		s.buildReActPrompt(history, message, persona, wsCtx),
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

	for i := 0; i < s.maxIters; i++ {
		s.logger.Info("ReAct iteration", "iteration", i+1)

		// 1. Call LLM (with model override if available) — traced
		prompt := strings.Join(conversationHistory, "\n\n")

		llmCtx, llmSpanID := s.tracer.StartSpan(ctx, fmt.Sprintf("llm.generate (iter %d)", i+1), domain.SpanKindLLM, map[string]string{
			"iteration": fmt.Sprintf("%d", i+1),
			"model":     modelID,
		})
		s.tracer.SetSpanInput(llmSpanID, prompt[max(0, len(prompt)-500):])
		s.tracer.SetSpanModel(llmSpanID, modelID)
		_ = llmCtx // llmCtx used for future nested calls

		var response string
		var err error
		if s.router != nil && modelID != "" {
			response, err = s.router.GenerateText(ctx, prompt, modelID)
		} else {
			response, err = s.llm.GenerateText(ctx, prompt)
		}
		if err != nil {
			s.tracer.EndSpan(llmSpanID, domain.SpanStatusError, "", err.Error())
			s.tracer.EndTrace(traceID, domain.SpanStatusError, err.Error())
			return nil, convID, fmt.Errorf("llm generate: %w", err)
		}
		s.tracer.EndSpan(llmSpanID, domain.SpanStatusOK, response[:min(500, len(response))], "")

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

			s.tracer.EndTrace(traceID, domain.SpanStatusOK, "")
			return agentResp, convID, nil
		}

		// 4. Execute tool — traced
		s.logger.Info("executing tool", "tool", step.Action, "params", step.ActionInput)

		toolCtx, toolSpanID := s.tracer.StartSpan(ctx, fmt.Sprintf("tool.%s", step.Action), domain.SpanKindTool, map[string]string{
			"tool": step.Action,
		})
		inputJSON, _ := json.Marshal(step.ActionInput)
		s.tracer.SetSpanInput(toolSpanID, string(inputJSON))

		result, err := effectiveTools.Execute(toolCtx, step.Action, step.ActionInput)
		if err != nil {
			step.Observation = fmt.Sprintf("Error: %v", err)
			s.tracer.EndSpan(toolSpanID, domain.SpanStatusError, step.Observation, err.Error())
		} else {
			// Format observation
			resultJSON, _ := json.Marshal(result)
			step.Observation = string(resultJSON)
			s.tracer.EndSpan(toolSpanID, domain.SpanStatusOK, step.Observation, "")
		}

		s.logger.Info("tool executed", "observation", step.Observation[:min(200, len(step.Observation))])

		// 5. Add to conversation
		conversationHistory = append(conversationHistory, response)
		conversationHistory = append(conversationHistory, fmt.Sprintf("Observation: %s", step.Observation))
	}

	s.tracer.EndTrace(traceID, domain.SpanStatusError, "max iterations reached")
	return nil, convID, fmt.Errorf("max iterations (%d) reached without final answer", s.maxIters)
}

// buildReActPrompt creates the initial prompt with tool descriptions and conversation history
func (s *ReActAgentService) buildReActPrompt(history string, userMessage string, persona *domain.Persona, wsCtx WorkspaceContext) string {
	// Choose effective tool set for prompt
	var toolsDesc string
	if persona != nil && len(persona.AllowedTools) > 0 {
		toolsDesc = s.tools.FilterByNames(persona.AllowedTools).FormatToolsForPrompt()
	} else {
		toolsDesc = s.tools.FormatToolsForPrompt()
	}

	// Build system identity from persona or workspace IDENTITY.md or default
	systemIdentity := "You are an AI assistant with access to tools."
	if persona != nil && persona.SystemPrompt != "" {
		systemIdentity = persona.SystemPrompt
	} else if wsCtx.Identity != "" {
		systemIdentity = wsCtx.Identity
	}

	// Append AGENT.md instructions if present
	if wsCtx.Agent != "" {
		systemIdentity += "\n\n" + wsCtx.Agent
	}

	var historyBlock string
	if history != "" {
		historyBlock = fmt.Sprintf(`
Previous conversation:
%s
---
`, history)
	}

	// Build workspace context block (memory, user prefs, skills, tools guide)
	workspaceBlock := wsCtx.FormatForPrompt()
	// For backwards compatibility, if wsCtx produced a block it replaces the old memory block
	var memoryBlock string
	if workspaceBlock != "" {
		memoryBlock = workspaceBlock
	}

	return fmt.Sprintf(`%s

You use the ReAct pattern: Thought → Action → Observation → ... → Final Answer.

FORMAT (tool call):
Thought: <reasoning>
Action: <EXACT tool name from list below>
Action Input: <JSON params>

FORMAT (direct answer):
Thought: <reasoning>
Final Answer: <response>

%s

%s

%s

RULES:
1. Always start with "Thought:"
2. For simple chat (greetings, questions, conversation), go DIRECTLY to "Final Answer:" — no tools needed.
3. Only use tools when the user explicitly asks for something requiring them.
4. CRITICAL: Use the EXACT tool name from the "Available Tools" list above. Do NOT invent tool names.
5. Tools generate_image and generate_text are ASYNC. When "status":"queued", tell user to wait.
6. When "status":"unavailable", the service is down — tell user clearly.
7. Action Input must be valid JSON on one line.
8. CHECK MEMORY: If the user asks about something stored in LONG-TERM MEMORY, use it!

EXAMPLES:

Example 1 — simple chat:
User: Hello!
Thought: Simple greeting, no tool needed.
Final Answer: Hello! How can I help you today?

Example 2 — image generation:
User: Generate an image of a sunset
Thought: I need to use generate_image for this request.
Action: generate_image
Action Input: {"prompt": "beautiful sunset over ocean"}

Example 3 — create a workflow:
User: Create a workflow to analyze and summarize a document
Thought: I need to use create_workflow to create a multi-step workflow.
Action: create_workflow
Action Input: {"name": "Document Analysis", "steps": [{"id": "analyze", "prompt": "Analyze the document structure and key points"}, {"id": "summarize", "prompt": "Write a concise summary based on the analysis", "depends_on": ["analyze"]}]}

Example 4 — run a command:
User: What is my current directory?
Thought: I need to use exec to run a shell command.
Action: exec
Action Input: {"command": "pwd"}

Example 5 — read a file:
User: Show me the contents of main.go
Thought: I need to use read_file to read this file.
Action: read_file
Action Input: {"path": "main.go"}

Now respond to:
User: %s`, systemIdentity, toolsDesc, memoryBlock, historyBlock, userMessage)
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
