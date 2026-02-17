# Pesquisa: Padrões de Orquestração Multi-Agent / Sub-Agent

> Data: 2026-02-17  
> Contexto: auleOS — local-first Agentic OS, Go kernel, ReAct loop existente, Personas existentes

---

## 1. Frameworks Analisados — Resumo de Padrões

### 1.1 Frameworks Go

#### **langchaingo** (Go, ~8.7k stars)
Implementa chains composáveis e agents como interfaces. O padrão de agent usa `Agent` interface com método `Plan()` que retorna `[]tool.Tool` actions. Sub-agents não são nativos — a composição é via chains: um chain pode chamar outro chain como step. Paralelismo é manual via goroutines. Não há orquestração multi-agent nativa, mas o pattern de `AgentExecutor` (loop de plan→act→observe) é sólido e familiar.

#### **Genkit Go** (Google, ~10k stars)
Usa `DefineTool()` com struct tags e `DefineFlow()` composável. **Tool Interrupts** permitem human-in-the-loop nativo — um flow pode pausar e pedir input humano. Sessions são tipadas com state persistente. Flows podem chamar sub-flows, e cada sub-step é automaticamente traced. Streaming é nativo. Não tem multi-agent explícito, mas a composição flow-in-flow com tracing é elegante e production-ready.

#### **Cogito** (Go lib, ~36 stars)
Implementa **Goal Planning** com TODOs — o agent decompõe um objetivo em sub-tarefas trackeáveis. **Content Refinement** usa um padrão worker + reviewer (dois roles LLM em loop). Suporta **parallel execution** de tools. Tool args usam struct tags, e Guidelines permitem seleção inteligente de tools por contexto. Session state é tipado. É o framework Go mais próximo de multi-agent real.

#### **Bubo** (Go, ~1 star, mas padrão interessante)
Implementa **Agent Handoff** — um agent pode "entregar" a conversa para outro agent, similar a um transfer de chamada. `bubo.Steps()` define orquestração como pipeline de steps. O pattern **agent-as-function** é chave: cada agent é uma função que recebe contexto e retorna resultado. Integração com Temporal para durabilidade. Handoff pattern é limpo e composável.

#### **LocalAGI** (Go, ~1.6k stars)
Permite **criação de agents via Web UI** (no-code). Agents podem formar "teams" a partir de um prompt — o framework decompõe e distribui. Custom Go actions são interpretadas em runtime. Tem connectors e memória curta/longa. O pattern de "teaming" é o mais próximo de swarm/crew.

### 1.2 Frameworks Python (padrões para inspiração)

#### **CrewAI**
Define três conceitos centrais: **Agent** (role + goal + backstory + tools + llm), **Task** (description + expected_output + agent), e **Crew** (agents + tasks + process). O `process` pode ser `sequential` (tarefas em ordem) ou `hierarchical` (um manager agent delega para workers). Comunicação é via task output → próxima task input. Cada agent pode ter seu próprio LLM (`llm` parameter). Eventos de progresso são emitidos via callbacks (`task_started`, `task_completed`, `agent_action`). **Padrão chave**: decomposição é declarativa — o usuário define tasks e o crew engine executa.

#### **AutoGen** (Microsoft)
Multi-agent conversations: agents são atores que trocam mensagens em um `GroupChat`. Um `GroupChatManager` roteia mensagens para o próximo speaker (round-robin, LLM-selected, ou custom). Agents podem ser `AssistantAgent`, `UserProxyAgent` (human-in-the-loop), ou custom. Cada agent tem seu próprio `llm_config`. Comunicação é via mensagens no chat compartilhado. Paralelismo é manual. **Padrão chave**: orquestração conversacional — agents conversam entre si, o manager decide quem fala.

#### **LangGraph**
Modela agent workflows como **grafos direcionados com ciclos** (StateGraph). Cada nó é uma função (ou agent) que recebe/modifica um state compartilhado. Edges definem transições (condicionais ou fixas). `START → planner → [tools/sub_agents] → evaluator → END/loop_back`. State é tipado (TypedDict/Pydantic). Suporta **subgraphs** (grafo dentro de grafo). Paralelismo via fan-out/fan-in (múltiplas edges saindo de um nó). **Checkpointing** persiste state para retomada. **Padrão chave**: composição como grafo, state explícito, determinismo na orquestração.

#### **Agency Swarm**
Cada agent é definido com `name`, `description`, `instructions`, `tools`, e `model`. Um `Agency` define uma topologia de comunicação entre agents (quem pode falar com quem). A comunicação usa uma tool especial `SendMessage` que permite um agent delegar para outro. O agent de entrada (CEO) recebe o prompt e decide quem acionar. **Padrão chave**: tool-based delegation — chamar outro agent é literalmente executar uma tool.

#### **Pydantic AI**
Foca em **outputs estruturados** — o agent retorna dados tipados (Pydantic model), não texto livre. `Agent[DepsType, ResultType]` é genérico. Tools são funções Python com type hints. Suporta `model` per-agent e dependency injection. Não tem multi-agent nativo, mas o pattern de result typing é essencial para composição: AgentA retorna `PlanOutput`, AgentB consome `PlanOutput`. **Padrão chave**: structured I/O entre agents.

#### **smolagents** (HuggingFace)
Extremamente leve. Um `ManagedAgent` wrapa um agent e o expõe como tool para outro agent. O orchestrator agent chama `managed_agent_summarizer(task="resumir X")` como uma tool call normal. O managed agent executa internamente seu próprio loop e retorna texto. **Padrão chave**: agent-as-tool — o mais simples e elegante pattern de delegação.

---

## 2. Comparação de Padrões Chave

| Dimensão | CrewAI | AutoGen | LangGraph | Agency Swarm | smolagents |
|----------|--------|---------|-----------|-------------|------------|
| **Decomposição** | Declarativa (user define tasks) | Conversacional (agents decidem) | Grafo explícito (dev define nós/edges) | Hierárquica (CEO delega) | Agent-as-tool (orchestrator decide) |
| **Comunicação** | Task output → next input | Messages em chat | State compartilhado | SendMessage tool | Return value da tool |
| **Paralelismo** | Crew.kickoff_async, tasks paralelas | Manual | Fan-out/fan-in nativo | Não nativo | Não nativo |
| **Eventos/UI** | Callbacks por step | Print/logging | State snapshots | Logging | Step callbacks |
| **Model per agent** | ✅ `agent.llm` | ✅ `llm_config` | ✅ per-node | ✅ `agent.model` | ✅ per-agent |
| **Composição** | Tasks em sequência/hierarquia | GroupChat | Subgraphs | Agent topology | ManagedAgent wrapper |

---

## 3. MELHOR PADRÃO PARA auleOS — Agent-as-Tool + DAG Leve

### Raciocínio

Dado que auleOS:
1. Já tem `ToolRegistry` e `ReActAgentService` com loop funcional
2. Já tem `Personas` com `AllowedTools` filtrado
3. Já tem `EventBus` para SSE
4. Usa arquitetura Worker-first (delegação é nativa)
5. É Go (concorrência com goroutines + channels é trivial)

O padrão ideal é **uma combinação de smolagents (agent-as-tool) + LangGraph (DAG leve)**:

- **Agent-as-Tool**: Cada sub-agent (Persona) é registrado como uma tool no ToolRegistry. O orchestrator ReAct agent pode chamar `delegate_to_coder(task="implementar função X")` como uma tool call normal. Internamente, isso cria um sub-ReAct loop com a persona "Coder", seu modelo, e suas tools.
- **DAG Leve**: Para tarefas complexas, um `Planner` agent decompõe em sub-tasks com dependências (DAG). Tasks independentes rodam em paralelo via goroutines. O DAG é simples — não precisa de LangGraph completo, só uma struct `Plan` com tasks e dependências.
- **Structured Output**: Sub-agents retornam resultado tipado (JSON schema), não texto livre. Isso garante composição confiável.

### Por que NÃO outros padrões:

- **CrewAI (declarativo)**: Exige que o user define tasks upfront. auleOS quer decomposição automática pelo LLM.
- **AutoGen (conversacional)**: Over-engineered para chat entre agents. auleOS já tem conversa com o user; agents são workers internos, não debatedores.
- **LangGraph completo**: Grafo Turing-completo é overkill. Um DAG simples com fan-out/fan-in resolve 90% dos casos.
- **Agency Swarm (topologia)**: Complexidade desnecessária de definir quem fala com quem.

---

## 4. Interfaces e Structs Go Propostas

```go
package domain

import (
    "context"
    "time"
)

// ──────────────────────────────────────────────
// Sub-Agent / Multi-Agent Domain Types
// ──────────────────────────────────────────────

// TaskID identifies a sub-task within a plan
type TaskID string

// PlanID identifies an execution plan
type PlanID string

// SubTaskStatus tracks the lifecycle of a sub-task
type SubTaskStatus string

const (
    SubTaskPending   SubTaskStatus = "PENDING"
    SubTaskRunning   SubTaskStatus = "RUNNING"
    SubTaskCompleted SubTaskStatus = "COMPLETED"
    SubTaskFailed    SubTaskStatus = "FAILED"
    SubTaskSkipped   SubTaskStatus = "SKIPPED"
)

// SubTask represents one unit of work in a decomposed plan.
// Each sub-task is assigned to a specific persona (sub-agent) with its own model.
type SubTask struct {
    ID           TaskID            `json:"id"`
    Description  string            `json:"description"`
    PersonaID    PersonaID         `json:"persona_id"`     // Which sub-agent handles this
    Model        string            `json:"model,omitempty"` // Override model (empty = persona default)
    DependsOn    []TaskID          `json:"depends_on"`     // DAG edges — must complete before this starts
    Status       SubTaskStatus     `json:"status"`
    Result       *SubTaskResult    `json:"result,omitempty"`
    StartedAt    *time.Time        `json:"started_at,omitempty"`
    CompletedAt  *time.Time        `json:"completed_at,omitempty"`
    Metadata     map[string]string `json:"metadata,omitempty"`
}

// SubTaskResult holds the structured output of a sub-task
type SubTaskResult struct {
    Summary   string                 `json:"summary"`         // Human-readable summary
    Data      map[string]interface{} `json:"data,omitempty"`  // Structured output
    Artifacts []ArtifactID           `json:"artifacts,omitempty"` // Generated artifacts
    Error     string                 `json:"error,omitempty"`
}

// Plan represents a decomposed task as a DAG of sub-tasks
type Plan struct {
    ID          PlanID        `json:"id"`
    ParentConvID ConversationID `json:"parent_conversation_id"`
    OriginalTask string       `json:"original_task"`
    Tasks       []SubTask     `json:"tasks"`
    Status      SubTaskStatus `json:"status"` // Overall plan status
    CreatedAt   time.Time     `json:"created_at"`
    CompletedAt *time.Time    `json:"completed_at,omitempty"`
}

// ──────────────────────────────────────────────
// Model Router — Multi-Model Support
// ──────────────────────────────────────────────

// ModelCapability categorizes what a model is good at
type ModelCapability string

const (
    CapGeneral    ModelCapability = "general"
    CapCode       ModelCapability = "code"
    CapCreative   ModelCapability = "creative"
    CapSummarize  ModelCapability = "summarize"
    CapReasoning  ModelCapability = "reasoning"
    CapVision     ModelCapability = "vision"
)

// ModelProfile describes an available model and its strengths
type ModelProfile struct {
    Tag          string            `json:"tag"`          // "qwen2.5:3b"
    Provider     string            `json:"provider"`     // "ollama", "openai"
    Parameters   string            `json:"parameters"`   // "3B", "7B"
    Capabilities []ModelCapability  `json:"capabilities"`
    MaxContext   int               `json:"max_context"`  // tokens
    Speed        string            `json:"speed"`        // "fast", "medium", "slow"
    VRAMRequired int               `json:"vram_mb"`      // approximate VRAM in MB
}

// ModelRouter selects the best model for a given task
type ModelRouter interface {
    // SelectModel picks the best available model for the capability
    SelectModel(ctx context.Context, cap ModelCapability) (ModelProfile, error)
    
    // ListModels returns all available models
    ListModels(ctx context.Context) ([]ModelProfile, error)
    
    // GetModel returns a specific model by tag
    GetModel(ctx context.Context, tag string) (ModelProfile, error)
}

// ──────────────────────────────────────────────
// Sub-Agent Events (for SSE/UI)
// ──────────────────────────────────────────────

// SubAgentEventType enumerates events the UI can observe
type SubAgentEventType string

const (
    EventPlanCreated     SubAgentEventType = "plan.created"
    EventTaskStarted     SubAgentEventType = "task.started"
    EventTaskThought     SubAgentEventType = "task.thought"    // ReAct thought from sub-agent
    EventTaskToolCall    SubAgentEventType = "task.tool_call"
    EventTaskObservation SubAgentEventType = "task.observation"
    EventTaskCompleted   SubAgentEventType = "task.completed"
    EventTaskFailed      SubAgentEventType = "task.failed"
    EventPlanCompleted   SubAgentEventType = "plan.completed"
)

// SubAgentEvent is emitted during multi-agent execution
type SubAgentEvent struct {
    Type      SubAgentEventType      `json:"type"`
    PlanID    PlanID                 `json:"plan_id"`
    TaskID    TaskID                 `json:"task_id,omitempty"`
    PersonaID PersonaID              `json:"persona_id,omitempty"`
    Model     string                 `json:"model,omitempty"`
    Data      map[string]interface{} `json:"data,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
}
```

```go
package ports

import (
    "context"
    "github.com/manthysbr/auleOS/internal/core/domain"
)

// Planner decomposes a user request into a DAG of sub-tasks
type Planner interface {
    // Decompose analyzes a user message and creates an execution plan.
    // Returns nil plan if the task is simple enough for a single agent.
    Decompose(ctx context.Context, message string, availablePersonas []domain.Persona) (*domain.Plan, error)
}

// SubAgentRunner executes a single sub-task using a specific persona
type SubAgentRunner interface {
    // Run executes a sub-task and streams events via the callback.
    // The runner creates an internal ReAct loop with the persona's config.
    Run(ctx context.Context, task domain.SubTask, onEvent func(domain.SubAgentEvent)) (*domain.SubTaskResult, error)
}

// Orchestrator coordinates the execution of a Plan
type Orchestrator interface {
    // Execute runs a plan, handling parallelism and dependencies.
    // Streams events for UI observability.
    Execute(ctx context.Context, plan *domain.Plan, onEvent func(domain.SubAgentEvent)) error
}
```

```go
package services

import (
    "context"
    "fmt"
    "log/slog"
    "sync"

    "github.com/manthysbr/auleOS/internal/core/domain"
)

// DAGOrchestrator executes a Plan respecting dependency order and parallelism
type DAGOrchestrator struct {
    logger  *slog.Logger
    runner  SubAgentRunner
    eventBus *EventBus
}

func NewDAGOrchestrator(logger *slog.Logger, runner SubAgentRunner, bus *EventBus) *DAGOrchestrator {
    return &DAGOrchestrator{logger: logger, runner: runner, eventBus: bus}
}

// Execute runs the plan's DAG: tasks with no unmet dependencies run in parallel.
func (o *DAGOrchestrator) Execute(ctx context.Context, plan *domain.Plan, onEvent func(domain.SubAgentEvent)) error {
    completed := make(map[domain.TaskID]bool)
    results := make(map[domain.TaskID]*domain.SubTaskResult)
    var mu sync.Mutex

    onEvent(domain.SubAgentEvent{
        Type:   domain.EventPlanCreated,
        PlanID: plan.ID,
        Data:   map[string]interface{}{"task_count": len(plan.Tasks)},
    })

    for {
        // Find ready tasks (all deps completed, not yet run)
        var ready []domain.SubTask
        mu.Lock()
        for i := range plan.Tasks {
            t := &plan.Tasks[i]
            if t.Status != domain.SubTaskPending {
                continue
            }
            allDepsMet := true
            for _, dep := range t.DependsOn {
                if !completed[dep] {
                    allDepsMet = false
                    break
                }
            }
            if allDepsMet {
                ready = append(ready, *t)
                t.Status = domain.SubTaskRunning
            }
        }
        allDone := len(ready) == 0
        mu.Unlock()

        if allDone {
            break
        }

        // Execute ready tasks in parallel
        var wg sync.WaitGroup
        for _, task := range ready {
            wg.Add(1)
            go func(t domain.SubTask) {
                defer wg.Done()
                
                result, err := o.runner.Run(ctx, t, onEvent)
                
                mu.Lock()
                defer mu.Unlock()
                
                // Find and update task in plan
                for i := range plan.Tasks {
                    if plan.Tasks[i].ID == t.ID {
                        if err != nil {
                            plan.Tasks[i].Status = domain.SubTaskFailed
                            plan.Tasks[i].Result = &domain.SubTaskResult{
                                Error: err.Error(),
                            }
                        } else {
                            plan.Tasks[i].Status = domain.SubTaskCompleted
                            plan.Tasks[i].Result = result
                            results[t.ID] = result
                        }
                        completed[t.ID] = true
                        break
                    }
                }
            }(task)
        }
        wg.Wait()
    }

    onEvent(domain.SubAgentEvent{
        Type:   domain.EventPlanCompleted,
        PlanID: plan.ID,
    })

    return nil
}

// ──────────────────────────────────────────────
// Agent-as-Tool: registra personas como tools delegáveis
// ──────────────────────────────────────────────

// RegisterDelegationTools adds a "delegate_to_<persona>" tool for each persona
func RegisterDelegationTools(registry *domain.ToolRegistry, personas []domain.Persona, runner SubAgentRunner) {
    for _, p := range personas {
        persona := p // capture loop var
        tool := &domain.Tool{
            Name:        fmt.Sprintf("delegate_to_%s", persona.ID),
            Description: fmt.Sprintf("Delegate a sub-task to the '%s' agent: %s", persona.Name, persona.Description),
            Parameters: domain.ToolParameters{
                Type: "object",
                Properties: map[string]interface{}{
                    "task": map[string]interface{}{
                        "type":        "string",
                        "description": "The specific task to delegate",
                    },
                },
                Required: []string{"task"},
            },
            Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
                taskDesc, _ := params["task"].(string)
                subTask := domain.SubTask{
                    ID:          domain.TaskID(domain.NewMessageID()), // reuse ID gen
                    Description: taskDesc,
                    PersonaID:   persona.ID,
                    Status:      domain.SubTaskPending,
                }
                result, err := runner.Run(ctx, subTask, func(e domain.SubAgentEvent) {
                    // Events flow through the runner's internal event handler
                })
                if err != nil {
                    return nil, err
                }
                return result, nil
            },
        }
        registry.Register(tool)
    }
}
```

---

## 5. LiteLLM — Model Discovery API

### Endpoint para listar modelos disponíveis

LiteLLM expõe um proxy OpenAI-compatible. O endpoint de model discovery é:

```
GET /v1/models
# ou com base URL customizada:
GET {LITELLM_BASE_URL}/models        # lista simplificada
GET {LITELLM_BASE_URL}/v1/models     # formato OpenAI-compatible
GET {LITELLM_BASE_URL}/model/info    # info detalhada (custo, max_tokens, etc.)
```

**Response format** (`/v1/models`):
```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4",
      "object": "model",
      "created": 1687882410,
      "owned_by": "openai"
    },
    {
      "id": "ollama/qwen2.5:3b",
      "object": "model",
      "created": 1687882410,
      "owned_by": "ollama"
    }
  ]
}
```

**Response format** (`/model/info`):
```json
{
  "data": [
    {
      "model_name": "qwen2.5:3b",
      "litellm_params": {
        "model": "ollama/qwen2.5:3b",
        "api_base": "http://localhost:11434"
      },
      "model_info": {
        "max_tokens": 32768,
        "max_input_tokens": 32768,
        "input_cost_per_token": 0.0,
        "output_cost_per_token": 0.0
      }
    }
  ]
}
```

### Ollama API nativa (sem LiteLLM)

Como auleOS é local-first e já usa Ollama diretamente, a API nativa é mais simples:

```
GET http://localhost:11434/api/tags    # lista modelos instalados
GET http://localhost:11434/api/ps      # modelos carregados em memória
```

**Response** (`/api/tags`):
```json
{
  "models": [
    {
      "name": "qwen2.5:3b",
      "model": "qwen2.5:3b",
      "size": 1928519168,
      "digest": "a6f19517...",
      "details": {
        "parent_model": "",
        "format": "gguf",
        "family": "qwen2",
        "families": ["qwen2"],
        "parameter_size": "3.1B",
        "quantization_level": "Q4_K_M"
      }
    }
  ]
}
```

**Recomendação para auleOS**: Use `/api/tags` do Ollama direto (já está conectado). Não precisa de LiteLLM a menos que queira unificar Ollama + OpenAI + Anthropic atrás de um proxy.

---

## 6. Modelos Ollama Recomendados (1B-6B) — Pull Commands

### Setup recomendado: 5 modelos, ~12GB total de disco

| Papel | Modelo | Params | VRAM (Q4) | Contexto | Por que este |
|-------|--------|--------|-----------|----------|-------------|
| **General / Reasoning** | `qwen2.5:3b` | 3.1B | ~2.5GB | 32K | Melhor tradeoff qualidade/velocidade na faixa 3B. Bate phi3 em benchmarks. Multilingual (PT-BR OK). |
| **Code Generation** | `qwen2.5-coder:3b` | 3.1B | ~2.5GB | 32K | Família Qwen2.5 com fine-tune para código. Supera DeepSeek-Coder-V2-Lite e StarCoder2:3b em HumanEval. Suporta 90+ linguagens. |
| **Creative / Writing** | `gemma2:2b` | 2.6B | ~2.0GB | 8K | Treinado com foco em qualidade de texto. Excelente para escrita criativa, emails, copy. Extremamente rápido. |
| **Fast Summarization** | `phi4-mini:3.8b` | 3.8B | ~2.8GB | 128K | Janela de contexto de 128K é ideal para summarizar documentos longos. MMLU competitivo com modelos 7B. |
| **Reasoning (fallback)** | `llama3.2:3b` | 3.2B | ~2.5GB | 128K | Modelo mais testado da faixa. Bom fallback geral. Forte em instruction following. |

### Pull Commands

```bash
# Core set — ~12GB total disk
ollama pull qwen2.5:3b          # General assistant / reasoning
ollama pull qwen2.5-coder:3b    # Code generation & analysis
ollama pull gemma2:2b            # Creative writing & copy
ollama pull phi4-mini:3.8b       # Summarization (128K context!)
ollama pull llama3.2:3b          # Fallback / reasoning
```

### Alternativas notáveis

```bash
# Se precisar de algo MUITO rápido (< 1B)
ollama pull qwen2.5:0.5b         # 0.5B — classificação, routing, entity extraction
ollama pull smollm2:1.7b         # 1.7B — HuggingFace, bom para tarefas simples

# Se tiver VRAM sobrando (6-8B)
ollama pull qwen2.5:7b           # Melhor qualidade geral acessível
ollama pull qwen2.5-coder:7b     # Coding com qualidade próxima de GPT-3.5
ollama pull deepseek-coder-v2:16b-lite-instruct-q4_0  # Se tiver >=10GB VRAM

# Modelos especializados
ollama pull nomic-embed-text:latest  # Embeddings para RAG (137M params!)
ollama pull moondream:1.8b           # Vision model — lê imagens
```

### Mapeamento Modelo → Capability para ModelRouter

```go
var DefaultModelProfiles = []ModelProfile{
    {
        Tag: "qwen2.5:3b", Provider: "ollama", Parameters: "3.1B",
        Capabilities: []ModelCapability{CapGeneral, CapReasoning},
        MaxContext: 32768, Speed: "fast", VRAMRequired: 2500,
    },
    {
        Tag: "qwen2.5-coder:3b", Provider: "ollama", Parameters: "3.1B",
        Capabilities: []ModelCapability{CapCode},
        MaxContext: 32768, Speed: "fast", VRAMRequired: 2500,
    },
    {
        Tag: "gemma2:2b", Provider: "ollama", Parameters: "2.6B",
        Capabilities: []ModelCapability{CapCreative},
        MaxContext: 8192, Speed: "fast", VRAMRequired: 2000,
    },
    {
        Tag: "phi4-mini:3.8b", Provider: "ollama", Parameters: "3.8B",
        Capabilities: []ModelCapability{CapSummarize, CapReasoning},
        MaxContext: 131072, Speed: "medium", VRAMRequired: 2800,
    },
    {
        Tag: "llama3.2:3b", Provider: "ollama", Parameters: "3.2B",
        Capabilities: []ModelCapability{CapGeneral},
        MaxContext: 131072, Speed: "fast", VRAMRequired: 2500,
    },
}
```

---

## 7. Plano de Integração com auleOS existente

### O que já existe e pode ser reutilizado

| Componente existente | Como usar no multi-agent |
|---------------------|-------------------------|
| `ReActAgentService` | Vira o `SubAgentRunner` — cada sub-task roda um ReAct loop interno |
| `ToolRegistry` | Delegation tools (`delegate_to_coder`) são registradas como tools normais |
| `Persona` | Cada persona = um sub-agent com SystemPrompt + AllowedTools + Model |
| `EventBus` | Extended com `SubAgentEvent` types para streaming multi-agent |
| `ProviderRegistry` | Evolui para `ModelRouter` — múltiplos modelos por provider |
| `LLMProvider` interface | Precisa aceitar `model` como parâmetro (hoje usa `DefaultModel` fixo) |

### Mudanças mínimas necessárias

1. **`LLMProvider.GenerateText`** precisa aceitar model como param:
   ```go
   // De:
   GenerateText(ctx context.Context, prompt string) (string, error)
   // Para:
   GenerateText(ctx context.Context, prompt string, opts ...GenerateOption) (string, error)
   ```

2. **`Persona`** ganha campo `PreferredModel`:
   ```go
   type Persona struct {
       // ... existing fields ...
       PreferredModel string `json:"preferred_model"` // "qwen2.5-coder:3b"
   }
   ```

3. **`EventBus`** ganha novos event types para sub-agent tracking (mas mesma infra).

4. **Nova tool `delegate_to_<persona>`** para cada persona — registrada no boot.

5. **Novo `Planner` service** que usa o LLM para decidir se decompõe ou não.

### Fluxo de execução proposto

```
User Message
    │
    ▼
┌─────────────┐    "tarefa simples?"     ┌──────────────┐
│   Planner   │ ──────── sim ──────────► │ ReAct Agent  │ (single agent, como hoje)
│  (LLM call) │                          │   (current)  │
└──────┬──────┘                          └──────────────┘
       │ não — cria Plan com N subtasks
       ▼
┌──────────────┐
│    DAG       │
│ Orchestrator │
└──────┬───────┘
       │
       ├──── goroutine ──► SubAgent(Coder, qwen2.5-coder:3b) ──► ReAct loop ──► Result
       │
       ├──── goroutine ──► SubAgent(Researcher, qwen2.5:3b)  ──► ReAct loop ──► Result
       │                                                              │
       │          ◄──── depends_on ────────────────────────────────────┘
       │
       └──── sequential ──► SubAgent(Writer, gemma2:2b) ──► ReAct loop ──► Final Result
                                                                            │
                                                                            ▼
                                                                    Merged into conversation
```

---

## 8. Conclusão

**Padrão adotado**: **Agent-as-Tool + DAG Leve** (inspirado em smolagents + LangGraph simplificado).

**Razões**:
1. **Incremental** — estende o que já existe (`ToolRegistry`, `ReActAgent`, `Persona`, `EventBus`) sem rewrite
2. **Go-idiomatic** — goroutines para parallelismo, channels para eventos, interfaces pequenas
3. **Observable** — cada step de cada sub-agent emite eventos para o `EventBus` → SSE → UI mostra a árvore de agentes trabalhando
4. **Model-aware** — `ModelRouter` + `Persona.PreferredModel` permite que cada agente use o modelo mais adequado
5. **Simples** — sem framework externo pesado. ~300 linhas de Go para o DAGOrchestrator + RegisterDelegationTools
