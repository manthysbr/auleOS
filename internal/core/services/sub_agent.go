package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// SubAgentOrchestrator manages the execution of sub-agent tasks.
// It is used by the "delegate" tool: the orchestrator LLM outputs a list of tasks,
// each is dispatched to a persona-specific mini-ReAct loop in a goroutine.
// Events are published on the EventBus so the UI can visualise the sub-agent tree.
type SubAgentOrchestrator struct {
	logger *slog.Logger
	router *ModelRouter
	tools  *domain.ToolRegistry
	repo   personaReader
	bus    *EventBus

	mu       sync.RWMutex
	active   map[domain.SubAgentID]*domain.SubAgentTask // currently running
	maxIters int
}

// NewSubAgentOrchestrator creates a new orchestrator.
func NewSubAgentOrchestrator(
	logger *slog.Logger,
	router *ModelRouter,
	tools *domain.ToolRegistry,
	repo personaReader,
	bus *EventBus,
) *SubAgentOrchestrator {
	return &SubAgentOrchestrator{
		logger:   logger,
		router:   router,
		tools:    tools,
		repo:     repo,
		bus:      bus,
		active:   make(map[domain.SubAgentID]*domain.SubAgentTask),
		maxIters: 3, // sub-agents are focused — fewer iterations
	}
}

// Delegate runs multiple sub-agent tasks in parallel and returns combined results.
// This is the core function called by the "delegate" tool's Execute.
func (o *SubAgentOrchestrator) Delegate(
	ctx context.Context,
	convID domain.ConversationID,
	parentID domain.SubAgentID,
	tasks []domain.DelegateTaskSpec,
) ([]domain.SubAgentTask, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks to delegate")
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make([]domain.SubAgentTask, len(tasks))
	)

	for i, spec := range tasks {
		wg.Add(1)
		go func(idx int, ts domain.DelegateTaskSpec) {
			defer wg.Done()
			task := o.runSubAgent(ctx, convID, parentID, ts)
			mu.Lock()
			results[idx] = task
			mu.Unlock()
		}(i, spec)
	}

	wg.Wait()
	return results, nil
}

// runSubAgent executes a single sub-agent mini-ReAct loop.
func (o *SubAgentOrchestrator) runSubAgent(
	ctx context.Context,
	convID domain.ConversationID,
	parentID domain.SubAgentID,
	spec domain.DelegateTaskSpec,
) domain.SubAgentTask {
	saID := domain.NewSubAgentID()
	now := time.Now()

	task := domain.SubAgentTask{
		ID:             saID,
		ParentID:       parentID,
		ConversationID: convID,
		Prompt:         spec.Prompt,
		Status:         domain.SubAgentStatusPending,
		StartedAt:      now,
	}

	// Resolve persona
	persona, err := o.resolvePersona(ctx, spec.Persona)
	if err != nil {
		task.Status = domain.SubAgentStatusFailed
		task.Error = fmt.Sprintf("resolve persona %q: %v", spec.Persona, err)
		o.publishEvent(task, persona)
		return task
	}

	task.PersonaID = persona.ID
	task.PersonaName = persona.Name

	// Resolve model via router
	role := o.router.inferRoleFromPersona(persona)
	modelID := o.router.ResolveModel(persona, role)
	task.ModelID = modelID

	// Register as active
	o.mu.Lock()
	o.active[saID] = &task
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		delete(o.active, saID)
		o.mu.Unlock()
	}()

	// Emit "running" event
	task.Status = domain.SubAgentStatusRunning
	o.publishEvent(task, persona)
	o.logger.Info("sub-agent started",
		"sa_id", string(saID),
		"persona", persona.Name,
		"model", modelID,
		"prompt", spec.Prompt[:min(80, len(spec.Prompt))],
	)

	// Build effective tool set
	effectiveTools := o.tools
	if len(persona.AllowedTools) > 0 {
		effectiveTools = o.tools.FilterByNames(persona.AllowedTools)
	}

	// Build prompt
	prompt := o.buildSubAgentPrompt(persona, effectiveTools, spec.Prompt)
	conversation := []string{prompt}
	steps := []domain.ReActStep{}

	// Mini-ReAct loop
	for i := 0; i < o.maxIters; i++ {
		fullPrompt := strings.Join(conversation, "\n\n")
		response, err := o.router.GenerateText(ctx, fullPrompt, modelID)
		if err != nil {
			task.Status = domain.SubAgentStatusFailed
			task.Error = fmt.Sprintf("llm error on iter %d: %v", i+1, err)
			fin := time.Now()
			task.FinishedAt = &fin
			o.publishEvent(task, persona)
			return task
		}

		step := o.parseReActOutput(response)
		steps = append(steps, step)

		// Emit thought event for live visualization
		o.publishThought(task, persona, step.Thought)

		if step.IsFinalAnswer {
			task.Status = domain.SubAgentStatusDone
			task.Result = step.FinalAnswer
			task.Steps = steps
			fin := time.Now()
			task.FinishedAt = &fin
			o.publishEvent(task, persona)
			o.logger.Info("sub-agent completed",
				"sa_id", string(saID),
				"persona", persona.Name,
				"result_len", len(step.FinalAnswer),
			)
			return task
		}

		// Execute tool if action present
		if step.Action != "" {
			result, err := effectiveTools.Execute(ctx, step.Action, step.ActionInput)
			if err != nil {
				step.Observation = fmt.Sprintf("Error: %v", err)
			} else {
				resultJSON, _ := json.Marshal(result)
				step.Observation = string(resultJSON)
			}
			conversation = append(conversation, response)
			conversation = append(conversation, fmt.Sprintf("Observation: %s", step.Observation))
		} else {
			// No action, no final answer — treat as final answer
			task.Status = domain.SubAgentStatusDone
			task.Result = response
			task.Steps = steps
			fin := time.Now()
			task.FinishedAt = &fin
			o.publishEvent(task, persona)
			return task
		}
	}

	// Max iterations reached
	task.Status = domain.SubAgentStatusFailed
	task.Error = fmt.Sprintf("max iterations (%d) reached", o.maxIters)
	task.Steps = steps
	fin := time.Now()
	task.FinishedAt = &fin
	o.publishEvent(task, persona)
	return task
}

// GetActive returns currently running sub-agents.
func (o *SubAgentOrchestrator) GetActive() []domain.SubAgentTask {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]domain.SubAgentTask, 0, len(o.active))
	for _, t := range o.active {
		out = append(out, *t)
	}
	return out
}

// --- internal helpers ---

func (o *SubAgentOrchestrator) resolvePersona(ctx context.Context, personaRef string) (*domain.Persona, error) {
	// Try by ID first
	p, err := o.repo.GetPersona(ctx, domain.PersonaID(personaRef))
	if err == nil {
		return &p, nil
	}

	// Try by name (case-insensitive scan through builtins)
	for _, bp := range domain.BuiltinPersonas() {
		if strings.EqualFold(bp.Name, personaRef) {
			return &bp, nil
		}
	}

	return nil, fmt.Errorf("persona not found: %s", personaRef)
}

func (o *SubAgentOrchestrator) buildSubAgentPrompt(persona *domain.Persona, tools *domain.ToolRegistry, userPrompt string) string {
	identity := "You are a helpful AI sub-agent."
	if persona != nil && persona.SystemPrompt != "" {
		identity = persona.SystemPrompt
	}

	toolsDesc := tools.FormatToolsForPrompt()

	return fmt.Sprintf(`%s

You are a SUB-AGENT executing a focused task. Be concise and direct.
Complete the task and provide a Final Answer.

%s

RULES:
1. Start with "Thought:" — reason briefly
2. Use "Action:" + "Action Input:" to call a tool if needed
3. End with "Final Answer:" when done — this is REQUIRED

Task: %s`, identity, toolsDesc, userPrompt)
}

func (o *SubAgentOrchestrator) publishEvent(task domain.SubAgentTask, persona *domain.Persona) {
	evt := domain.SubAgentEvent{
		SubAgentID:     task.ID,
		ParentID:       task.ParentID,
		ConversationID: task.ConversationID,
		PersonaName:    task.PersonaName,
		ModelID:        task.ModelID,
		Status:         task.Status,
		Result:         task.Result,
		Error:          task.Error,
	}
	if persona != nil {
		evt.PersonaColor = persona.Color
		evt.PersonaIcon = persona.Icon
	}

	data, _ := json.Marshal(evt)
	o.bus.Publish(Event{
		JobID:     string(task.ConversationID), // key by conversation so chat UI receives it
		Type:      EventTypeSubAgent,
		Data:      string(data),
		Timestamp: time.Now().Unix(),
	})
}

func (o *SubAgentOrchestrator) publishThought(task domain.SubAgentTask, persona *domain.Persona, thought string) {
	if thought == "" {
		return
	}
	evt := domain.SubAgentEvent{
		SubAgentID:     task.ID,
		ParentID:       task.ParentID,
		ConversationID: task.ConversationID,
		PersonaName:    task.PersonaName,
		ModelID:        task.ModelID,
		Status:         domain.SubAgentStatusRunning,
		Thought:        thought,
	}
	if persona != nil {
		evt.PersonaColor = persona.Color
		evt.PersonaIcon = persona.Icon
	}

	data, _ := json.Marshal(evt)
	o.bus.Publish(Event{
		JobID:     string(task.ConversationID),
		Type:      EventTypeSubAgent,
		Data:      string(data),
		Timestamp: time.Now().Unix(),
	})
}

// parseReActOutput extracts Thought/Action/ActionInput or FinalAnswer from LLM response
// (same logic as ReActAgentService but local to the sub-agent)
func (o *SubAgentOrchestrator) parseReActOutput(response string) domain.ReActStep {
	step := domain.ReActStep{}

	finalAnswerRe := regexp.MustCompile(`(?i)Final Answer:\s*(.*)`)
	if matches := finalAnswerRe.FindStringSubmatch(response); len(matches) > 1 {
		step.IsFinalAnswer = true
		step.FinalAnswer = strings.TrimSpace(matches[1])

		thoughtRe := regexp.MustCompile(`(?i)Thought:\s*([^\n]+)`)
		if tMatches := thoughtRe.FindStringSubmatch(response); len(tMatches) > 1 {
			step.Thought = strings.TrimSpace(tMatches[1])
		}
		return step
	}

	thoughtRe := regexp.MustCompile(`(?i)Thought:\s*([^\n]+)`)
	if matches := thoughtRe.FindStringSubmatch(response); len(matches) > 1 {
		step.Thought = strings.TrimSpace(matches[1])
	}

	actionRe := regexp.MustCompile(`(?i)Action:\s*([a-z_]+)`)
	if matches := actionRe.FindStringSubmatch(response); len(matches) > 1 {
		step.Action = strings.TrimSpace(matches[1])
	}

	actionInputRe := regexp.MustCompile(`(?i)Action Input:\s*(\{[^}]*\})`)
	if matches := actionInputRe.FindStringSubmatch(response); len(matches) > 1 {
		jsonStr := strings.TrimSpace(matches[1])
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &params); err == nil {
			step.ActionInput = params
		}
	}

	return step
}
