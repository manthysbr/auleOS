# auleOS Milestones

Última atualização: 2026-02-16

## Visão do Produto

**auleOS** (Aulë, o Ferreiro e Criador) é um **Sistema Operacional Agêntico local-first** para criação de conteúdo profissional. **NÃO é um chatbot** — é um SO onde agentes inteligentes são cidadãos de primeira classe. O chat é apenas **um dos componentes** de interação, não a interface central.

Pense num **desktop criativo** onde você abre "apps" (agentes especializados), arrasta ferramentas (tools), visualiza pipelines, e o SO se vira para criar o que você precisa — usando workers locais com Docker, modelos de IA generativa, e ferramentas composíveis.

**Referências de experiência**: NotebookLM (projetos + fontes + chat lateral), macOS (desktop limpo, dock, command palette), Figma (canvas + colaboração), Langflow/Flowise (visual agent builder).

### Pilares

| Pilar | Descrição |
|-------|-----------|
| **Desktop-first, não Chat-first** | A interface principal é um workspace/desktop. Chat é um painel lateral, não a tela inteira |
| **Glass Box** | Todo raciocínio, uso de tool e consumo de recurso é visível em tempo real |
| **Orquestração por Workers** | Kernel NUNCA processa mídia — delega para workers efêmeros via Docker |
| **Local-first + Cloud-burst** | Funciona 100% com Ollama + modelos leves, com opção de APIs para qualidade superior |
| **Agentes como Apps** | Cada agente é como um "app" do SO — tem ícone, capabilities, persona, e pode ser criado/editado via chat OU visual builder |
| **Tool Marketplace** | Tools são plugins instaláveis. Usuário pode criar tools via chat ("crie uma tool que...") ou via builder gráfico |
| **Multi-modal nativo** | Texto, imagem, áudio e vídeo como cidadãos de primeira classe |

### Referências arquiteturais pesquisadas e absorvidas

| Projeto OSS | Stars | O que absorvemos |
|------------|-------|------------------|
| **Ollama** (Go) | 163k | Model management, REST API simples, adaptabilidade de hardware |
| **LocalAI** (Go) | 42.8k | Backend Gallery (OCI), multi-modal pipelines, MCP, gallery system |
| **Genkit Go** (Go, Google) | 10k | `DefineTool` com struct tags, `DefineFlow` composável, **Tool Interrupts** (human-in-the-loop nativo!), Sessions c/ typed state, streaming flows, traced sub-steps. Go puro, MIT |
| **Cogito** (Go lib) | 36 | Tool args com struct tags, Guidelines p/ seleção inteligente, Goal Planning com TODOs, Content Refinement (worker+reviewer), parallel execution, session state |
| **Bubo** (Go) | ~1 | Agent handoff entre agentes, `bubo.Steps()` para orquestração, agent-as-function pattern, Temporal integration |
| **LocalAGI** (Go) | 1.6k | No-code agent creation via Web UI, agent teaming from a prompt, custom Go actions interpreted at runtime, connectors, short/long-term memory |
| **Gitea** (Go) | 53.7k | Clean architecture models/modules/services/routers, module-driven |
| **Grafana** (Go+TS) | 72.1k | Plugin architecture, data sources dinâmicos, observabilidade |
| **React Flow / @xyflow/react** | 35k | Lib React MIT para node-based UI. **Usado pelo Langflow e Flowise** como engine visual. TypeScript, infinitamente customizável |
| **Langflow** (Py+React) | 145k | Visual builder + code access. Prova que chat + visual flow funciona. Referência de UX para agent building |
| **Flowise** (TS) | 49k | "Build AI Agents, Visually". Node-based agent builder. Prova que visual agent building atrai developers |
| **Open WebUI** (Py+Svelte) | 124k | RAG c/ vector DBs, web search, artifact storage, Pipelines plugin, RBAC |
| **LangChainGo** (Go) | 8.7k | Chains composáveis, vector stores, document loaders, text splitters |

---

## Critério de status

- **DONE**: Entregue e validado (build + teste funcional)
- **IN_PROGRESS**: Parcialmente entregue, gaps para produção
- **TODO**: Não iniciado

## Princípios de implementação (diretriz permanente)

1. **Go idiomático** — composição, interfaces pequenas, erro explícito, zero "framework caseiro"
2. **Genkit Go como core** — `DefineTool`, `DefineFlow`, Tool Interrupts, Sessions são primitivas do kernel. Genkit é a camada agêntica, não reinventamos
3. **Reuse-first** — reaproveitar padrões do projeto; buscar libs maduras na web
4. **Referências sólidas** — pesquisar OSS maduro antes de criar; adaptar não copiar
5. **Spec-driven** — atualizar OpenAPI/schema antes de ampliar; gerar código/tipos do contrato
6. **Fatia vertical** — back + front + contrato por ciclo; fechar com build e smoke test
7. **Worker-first** — computação pesada SEMPRE em worker efêmero; kernel orquestra e persiste

---

## FASE 1 — FUNDAÇÃO (DONE)

### M1 — Fundação Agêntica (DONE)

**Objetivo**: Núcleo de raciocínio + abstração de providers + primeira tool.

**Entregue**: Abstração providers LLM/Image (local/remoto), Factory central, ToolRegistry, ReActAgentService com loop Thought→Action→Observation→Final Answer, tool `generate_image`.

**Evidência**: `react_agent.go`, `tools.go`, `providers/factory.go`, `main.go`

---

### M2 — ReAct no Kernel API (DONE)

**Objetivo**: Motor ReAct exposto via API.

**Entregue**: Handler de chat via `ReActAgentService`, contrato OpenAPI com `steps`, tipos Go/TS regenerados.

---

### M3 — UI do Pensamento ReAct (DONE)

**Objetivo**: Frontend mostra raciocínio do agente, não só resposta.

**Entregue**: Chat renderiza steps ReAct (pensamento, ação, observação), `tool_call` com payload.

---

### M4 — Jobs assíncronos + workspace persistente (DONE*)

**Objetivo**: Geração de imagem/texto como Job persistido com artefato.

**Entregue**: Jobs `image.generate` e `text.generate` no pipeline assíncrono, capability handlers extensíveis, transições `QUEUED→RUNNING→COMPLETED/FAILED`, persistência de artefatos, URLs servidas pelo kernel, sidebar com status real.

*\*Gaps: testes de integração, handlers extraídos em módulos dedicados*

---

### M5 — SSE + Streaming de Jobs (DONE*)

**Objetivo**: Progressão de jobs em tempo real via SSE.

**Entregue**: Eventos `status`/`log`/`progress` via SSE, `AgentStream` no frontend, auto-seleção de job no stream, preview/download de artefatos.

*\*Gaps: merge eventos tool↔worker lifecycle, reconexão SSE robusta*

---

### M5.5 — Settings & Secret Management (DONE)

**Objetivo**: Configuração de providers com armazenamento seguro de API keys.

**Entregue**: AES-256-GCM para secrets, SettingsStore com DuckDB, API endpoints GET/PUT/test, hot-reload de providers, UI completa de settings com local/remote toggle, test connection.

**Evidência**: `internal/config/crypto.go`, `internal/config/store.go`, `SettingsPanel.tsx`

---

## FASE 2 — CONVERSAS & DESKTOP SHELL

> **Objetivo**: Sair do layout "chatbot" para uma experiência de **SO criativo**. Conversas são a espinha dorsal, mas a interface principal é um desktop/workspace — não uma tela de chat.
>
> **Princípio**: O chat é um **painel lateral** (como Spotlight/Copilot sidebar), não a área central. A área central mostra artefatos, projetos, pipelines, galeria.

### M6 — Conversations & Memory (DONE)

**IMPACTO: MÁXIMO** — Espinha dorsal de todo o produto. Tudo passa por conversas.

**Entregue**: Conversas persistentes com histórico de mensagens e memória de contexto. Domain types (Conversation, Message, ConversationID, MessageID), DuckDB persistence (conversations + messages tables), CRUD endpoints completo, ConversationStore com LRU cache (64 convs), sliding window (20 msgs), ReActAgentService refatorado para multi-turn com conversation_id, frontend com Zustand store, sidebar de conversas, chat com persistência.

**Evidência**: `domain/conversation.go`, `services/conversation_store.go`, `duckdb/repository.go`, `pkg/kernel/conversations.go`, `store/conversations.ts`, `ChatInterface.tsx`, `Sidebar.tsx`

---

### M7 — Desktop Shell & Workspace ✅ DONE

**IMPACTO: CRÍTICO** — Define a identidade do produto como SO, não chatbot.

Transformar a UI de "tela de chat com sidebar" para um **desktop criativo** onde o chat é um painel lateral e a área principal mostra artefatos, projetos e agentes.

**Por que agora**: A tela atual é 90% chat — parece um ChatGPT clone. O produto é um SO agêntico. A experiência de "desktop" precisa existir ANTES de adicionar personas, tools e agent studio, porque define onde cada feature vai morar visualmente.

**Escopo**:

- **Frontend — Layout Desktop**:
  - **Top Bar**: Logo auleOS + breadcrumb (Workspace > Projeto > ...) + Command Palette trigger (⌘K) + Settings
  - **Left Dock**: Ícones verticais para navegação (Home, Projetos, Agentes, Tools, Jobs) — estilo VS Code / macOS Dock
  - **Center Stage**: Área principal que muda conforme contexto:
    - **Home/Dashboard**: Grid de artefatos recentes (imagens, textos, docs), stats de uso, quick actions
    - **Projeto view**: Conversas + documentos + artefatos do projeto
    - **Artifact Viewer**: Preview de imagem/texto/PDF fullscreen com ações (download, re-generate, share)
    - **Jobs Monitor**: Lista detalhada de jobs com logs, progresso, artefatos
  - **Right Panel (collapsible)**: Chat/Agent — painel lateral que abre/fecha com ⌘J ou clicando no dock
    - Mesmo ChatInterface atual, mas como sidebar, não como tela inteira
    - Conversas listadas dentro do painel
  - **Bottom Bar**: Status do sistema (workers ativos, modelo carregado, uso de recursos)
- **Backend — Projetos**:
  - struct `Project` com `ID`, `Name`, `Description`, `CreatedAt`, `UpdatedAt`
  - Tabela `projects` no DuckDB
  - `conversations` ganha `project_id` (nullable) para agrupar
  - CRUD `/v1/projects` (list, get, create, update, delete)
  - `GET /v1/projects/{id}/conversations`
  - `GET /v1/projects/{id}/artifacts` (artefatos gerados nos jobs das conversas do projeto)
- **Frontend — Artifact Gallery**:
  - Grid responsivo de artefatos com thumbnail
  - Filtro por tipo (imagem, texto, documento, áudio)
  - Preview inline (imagem = lightbox, texto = reader, PDF = embed)
  - Actions: download, delete, re-generate (submete novo job com mesmo prompt)
- **Frontend — Command Palette** (⌘K):
  - Busca fuzzy em: conversas, projetos, artefatos, agentes, tools, settings
  - Quick actions: "New Project", "New Chat", "Generate Image", "Open Settings"
  - Padrão: VS Code Command Palette, Raycast, macOS Spotlight
- **Contrato OpenAPI**: Schemas `Project`, endpoints `/v1/projects`, `/v1/artifacts`
- **Router**: wouter com rotas: `/`, `/project/:id`, `/agents`, `/tools`, `/jobs`, `/settings`

**Exit Criteria**:

- Layout de desktop com dock + center stage + chat como sidebar
- Projetos organizam conversas e artefatos
- Artefatos são visíveis numa galeria (imagens aparecem como thumbnails!)
- Command Palette funcional
- Chat continua funcionando, agora como painel lateral
- Build/test passa

---

### M8 — Sistema de Personas ✅ DONE

**IMPACTO: ALTO** — Diferenciador do produto. Transforma como o agente se comporta.

Agentes com personalidade definida que adaptam estilo, profundidade e formato de output. Integrado ao Desktop Shell (M7).

**Implementado**:

- **Domínio**: `Persona` struct (ID, Name, Description, SystemPrompt, Icon, Color, AllowedTools, IsBuiltin, CreatedAt, UpdatedAt)
  - 4 personas built-in: `assistant` (blue/bot), `researcher` (emerald/search), `creator` (violet/palette), `coder` (amber/code)
  - `BuiltinPersonas()` retorna as 4 personas, seed idempotente no boot
  - `ToolRegistry.FilterByNames()` filtra tools por persona
- **Backend**: CRUD completo `/v1/personas` (GET/POST list, GET/PATCH/DELETE individual)
  - DuckDB: tabela `personas`, `conversations.persona_id` FK
  - `ReActAgentService.Chat()` recebe `personaID`, resolve persona, injeta SystemPrompt dinâmico em `buildReActPrompt()`
  - Tool filtering: persona com `AllowedTools` restringe o tool registry efetivo
  - Proteção: built-in personas não podem ser deletadas
- **Frontend**: PersonaChip selector no ChatPanel, `persona_id` enviado no POST `/v1/agent/chat`
  - AgentsView: grid de PersonaCards com create/edit/delete (protege builtins)
  - Seletor visual com ícones (bot/search/palette/code) e cores (blue/emerald/violet/amber/cyan/rose)
  - Zustand store `personas.ts` com CRUD completo
- **Padrão absorvido**: Cogito Guidelines, LocalAGI personas

**Exit Criteria**: ✅ ALL MET

- ✅ Quatro personas alterando comportamento do agente (system prompt dinâmico + tool filtering)
- ✅ Persona vinculada à conversa, visível no chat panel
- ✅ Build/test passa (`go build`, `go test`, `tsc --noEmit`, `npm run build`)

---

### M9 — Sub-Agents Visíveis + Multi-Model + Model Discovery ✅ DONE

**IMPACTO: ALTO** — Arquitetura de sub-agents paralelos com visibilidade em tempo real.

Implementação completa de sub-agents assíncronos visíveis na UI, roteamento multi-modelo, e discovery de modelos locais/remotos. Inspirado em padrões de CrewAI, smolagents e LangGraph.

**Implementado**:

- **Domain Layer**:
  - `ModelSpec` / `ModelRole` (general/code/creative/fast) + `RecommendedLocalModels()`
  - `SubAgentTask` / `SubAgentEvent` / `SubAgentStatus` (pending/running/done/failed)
  - `DelegateRequest` / `DelegateTaskSpec` — input para orquestração
  - `Persona.ModelOverride` — override de modelo por persona
  - `LLMProvider.GenerateTextWithModel()` — geração com modelo específico

- **Services**:
  - `ModelRouter` — resolve modelo: PersonaOverride > RoleDefault > ProviderDefault
  - `ModelDiscovery` — descobre modelos do Ollama (`/api/tags`) e LiteLLM (`/v1/models`)
  - `SubAgentOrchestrator` — executa tasks em paralelo com goroutines, mini-ReAct loop (3 iters), SSE events por sub-agent
  - `delegate` tool — o agente principal chama `delegate` com array de tasks, cada uma rodando como sub-agent com persona/modelo/tools próprios
  - `EventBus` com `EventTypeSubAgent` — publica eventos por conversation ID

- **API (OpenAPI)**:
  - `GET /v1/models` — catálogo de modelos disponíveis
  - `POST /v1/models/discover` — descobre modelos do Ollama/LiteLLM
  - `GET /v1/conversations/{id}/events` — SSE real-time para sub-agent activity
  - `model_override` em Persona create/update/response

- **Adapters**:
  - Ollama `GenerateTextWithModel()` — default llama3.2:3b
  - OpenAI `GenerateTextWithModel()` — thread-safe via método interno

- **Frontend**:
  - `useSubAgentStream` hook — SSE para `/v1/conversations/{id}/events`
  - `useModelStore` — catálogo de modelos com fetch + discover
  - `useSubAgentStore` — Map de sub-agents ativos por ID
  - `SubAgentCard` / `SubAgentTree` — cards visuais com cor da persona, status animado, thought bubble, resultado
  - `ChatInterface` integrado com sub-agent tree em tempo real
  - `Persona` store atualizado com `model_override`

- **DuckDB**: Migration `model_override TEXT` na tabela personas

**Validação**:

- ✅ `GET /v1/models` → 5 modelos recomendados
- ✅ `POST /v1/models/discover` → encontra llama3.2:latest (3.2B) do Ollama local
- ✅ `POST /v1/personas` com `model_override` → retorna corretamente
- ✅ `GET /v1/conversations/{id}/events` → SSE conecta e espera eventos
- ✅ `go build ./...` ✅ `go test ./...` ✅ `tsc --noEmit` ✅ `vite build`

---

## FASE 3 — AGENT STUDIO & TOOL BUILDER

> **Inspiração**: Langflow (visual builder + code), Flowise (node-based agents), React Flow (engine visual), Genkit (Tool Interrupts, DefineFlow), Bubo (agent handoff), LocalAGI (no-code agent creation)
>
> **Objetivo**: O usuário projeta agentes e tools via interface híbrida — chat OU visual OU ambos. Inclui **Tool Builder** (criar tools via chat ou componentes gráficos) e **Tool Marketplace** (instalar tools da comunidade).
>
> **Princípio**: Chat e Visual são **views** da mesma entidade. O que o chat cria, o visual exibe e vice-versa.

### M10 — Agent Definition Model + Tool Builder (TODO)

**IMPACTO: ESTRUTURAL** — Define o modelo central + Tool Builder para criar tools via chat.

**Escopo** (inclui Tool Builder MVP):

- **Domínio**: `AgentDefinition` (persona, tools, guidelines, interrupts, flow_graph)
  ```go
  type AgentDefinition struct {
      ID           string            `json:"id"`
      Name         string            `json:"name"`
      Description  string            `json:"description"`
      PersonaID    string            `json:"persona_id"`     // Persona base
      SystemPrompt string            `json:"system_prompt"`  // Override ou complemento
      Tools        []string          `json:"tools"`          // Tools permitidas
      Guidelines   []string          `json:"guidelines"`     // Regras de comportamento (Cogito-style)
      Interrupts   []InterruptRule   `json:"interrupts"`     // Human-in-the-loop checkpoints (Genkit-style)
      FlowGraph    *FlowGraph        `json:"flow_graph"`     // Representação visual (nodes + edges)
      CreatedAt    time.Time         `json:"created_at"`
      UpdatedAt    time.Time         `json:"updated_at"`
  }

  type AgentTeam struct {
      ID      string            `json:"id"`
      Name    string            `json:"name"`
      Agents  []TeamMember      `json:"agents"`         // Agentes do time
      Router  RoutingStrategy   `json:"router"`         // Como rotear (round-robin, skill-based, LLM-decided)
      Handoff []HandoffRule     `json:"handoff_rules"`  // Regras de transferência entre agentes
  }

  // Handoff protocol — nosso, construído sobre Genkit primitives
  type HandoffRule struct {
      FromAgent  string   `json:"from_agent"`   // Agente de origem
      ToAgent    string   `json:"to_agent"`     // Agente destino
      Condition  string   `json:"condition"`    // Condição trigger ("language=es", "topic=code", "confidence<0.5")
      Strategy   string   `json:"strategy"`     // "auto" | "interrupt" (pede confirmação humana via Tool Interrupt)
  }

  type FlowGraph struct {
      Nodes []FlowNode `json:"nodes"` // Nós visuais (persona, tool, condition, output)
      Edges []FlowEdge `json:"edges"` // Conexões entre nós
  }
  ```
- **Backend**: CRUD `/v1/agents` (list, get, create, update, delete, clone)
  - CRUD `/v1/agent-teams` (list, get, create, update, delete)
  - Persistir em DuckDB (tabelas `agent_definitions`, `agent_teams`)
  - Conversação com persona `builder` cria/edita `AgentDefinition` via tool `create_agent` / `edit_agent`
  - Validação: tools referenciadas existem no registry, persona existe
- **Padrão absorvido**: Genkit `DefineFlow` (composição), Genkit Tool Interrupts (human-in-the-loop), Cogito Guidelines
- **Agent Handoff Protocol** (próprio, sobre Genkit):
  - Cada agente é um Genkit Flow com tools e persona
  - Handoff = Tool Interrupt especial que transfere contexto para outro agente-flow
  - O kernel mantém a sessão (Genkit Sessions) e roteia entre flows
  - Estratégia `auto` (agente decide) ou `interrupt` (humano confirma via Tool Interrupt)
  - Não usamos Bubo/Temporal — nosso handoff é leve: tool function que retorna `{handoff_to: "agent_id", context: {...}}`
  - O `AgentTeam.Router` decide qual agente recebe a próxima mensagem

**Exit Criteria**:

- CRUD de AgentDefinition funcional
- Agente pode ser criado via API e via chat (tool `create_agent`)
- FlowGraph persiste a representação visual
- Handoff entre 2 agentes funciona (auto + interrupt)
- Build/test passa

---

### M11 — Agent Studio Visual + Tool Marketplace (TODO)

**IMPACTO: DIFERENCIADOR** — A tela híbrida chat + visual que define o produto.

Interface visual para projetar agentes usando React Flow, com sincronização bidirecional com o chat.

**Escopo**:

- **Frontend — Visual Builder** (React Flow / `@xyflow/react`):
  - Canvas node-based com tipos de nó:
    - `PersonaNode` — seleciona persona base
    - `ToolNode` — configura tool com parâmetros
    - `ConditionNode` — if/else baseado em output
    - `InterruptNode` — checkpoint human-in-the-loop
    - `HandoffNode` — transferência para outro agente
    - `OutputNode` — resultado final
  - Drag & drop de nós da sidebar de componentes
  - Edges conectando nós (fluxo de execução)
  - Panel lateral para configurar propriedades de cada nó
  - Preview do `AgentDefinition` JSON resultante
- **Frontend — Tela Híbrida**:
  - Layout split: Chat à esquerda, Canvas à direita (redimensionável)
  - **Bidirecional**: Criar tool no canvas → aparece no chat context. Pedir no chat "adicione web_search" → nó aparece no canvas
  - Toggle: modo chat-only / visual-only / split
  - Mini-mapa do flow no canto
- **Frontend — Preview de Time**:
  - Visualização de `AgentTeam` como grafo de agentes conectados
  - Setas de handoff entre agentes
  - Status de cada agente (idle, running, completed)
- **Sincronização**:
  - Chat command → API → FlowGraph updated → React Flow re-renders
  - Visual edit → FlowGraph updated → API → (opcional) Chat mostra log da mudança
  - Single source of truth: `AgentDefinition.FlowGraph` no backend
- **Padrão absorvido**: React Flow (Langflow/Flowise usam), Langflow dual-mode UX, LocalAGI no-code creation

**Exit Criteria**:

- Agente criado via visual builder funciona identicamente a um criado via chat
- Edição no chat reflete no canvas e vice-versa
- Time de agentes visível como grafo
- Build/test passa

---

## FASE 4 — RAG & KNOWLEDGE BASE

> **Inspiração**: LangChainGo (document loaders, text splitters, vector stores), Open WebUI (RAG), LocalRecall
>
> **Objetivo**: O agente pode consumir documentos do usuário para gerar conteúdo informado.

### M12 — Document Ingestion Pipeline (TODO)

Upload e processamento de documentos para indexação.

**Escopo**:

- **Upload API**: `POST /v1/documents` (multipart)
  - Tipos suportados: PDF, Markdown, TXT, DOCX
  - Armazenamento em `workspace/documents/{doc_id}/`
- **Processing Worker**:
  - Extraction (Tika ou Docling via worker container)
  - Text splitting (chunk por parágrafo/seção, configurável)
  - Embedding generation (Ollama `nomic-embed-text` local ou API)
- **Domínio**: struct `Document` com `ID`, `Name`, `Type`, `ChunkCount`, `Status`
- **Padrão absorvido**: LangChainGo `documentloaders` + `textsplitter` patterns

**Exit Criteria**:

- Upload → extração → chunks criados
- Pipeline funciona como job assíncrono com progresso SSE

---

### M13 — Vector Store & RAG Query (TODO)

Busca semântica nos documentos indexados para alimentar o agente.

**Escopo**:

- **Vector store**: Embeddings em DuckDB c/ extensão VSS (ou chromem-go como fallback — in-process, Go puro)
- **RAG no agente**:
  - Tool `search_knowledge` que faz similarity search nos chunks
  - Injeta contexto relevante no prompt antes do ReAct loop
  - Citação de fonte (documento + chunk) na resposta
- **Frontend**:
  - `#` prefix para buscar em documentos (inspirado Open WebUI)
  - Indicador visual de "grounded in documents"
  - Painel de documentos na sidebar
- **Padrão absorvido**: Open WebUI RAG workflow; chromem-go para simplicidade Go-native

**Exit Criteria**:

- Upload de PDF → pergunta sobre conteúdo → resposta com citação
- Funciona com Ollama embeddings local

---

## FASE 5 — ORQUESTRAÇÃO REAL DE WORKERS

> **Inspiração**: Ollama (model management), LocalAI (backend gallery OCI), Docker
>
> **Objetivo**: Workers Docker reais, efêmeros, isolados. Kernel como plano de controle puro.

### M14 — Worker Registry & Manifest System (TODO)

Registry de workers disponíveis com capacidades declaradas.

**Escopo**:

- **Worker Manifest** (evolução do `worker-spec.json`):
  ```json
  {
    "name": "piper-tts",
    "version": "1.0.0",
    "capabilities": ["audio.generate"],
    "image": "ghcr.io/aule/worker-piper:latest",
    "resources": {"vram_mb": 0, "ram_mb": 512},
    "inputs": [{"name": "text", "type": "string"}],
    "outputs": [{"name": "audio", "type": "file", "format": "wav"}]
  }
  ```
- **Registry API**: `GET /v1/workers/catalog`, `POST /v1/workers/install`
- **Auto-discovery**: Scan de imagens Docker com label `io.aule.worker=true`
- **Padrão absorvido**: LocalAI Backend Gallery (OCI-based install/remove)

**Exit Criteria**:

- Workers declarados via manifest
- Kernel sabe quais capabilities estão disponíveis
- API para listar/instalar workers

---

### M15 — Watchdog Sidecar & Execução Isolada (TODO)

Worker containers reais com sidecar de comunicação.

**Escopo**:

- **Watchdog** (já iniciado em `pkg/watchdog/`):
  - HTTP server dentro do container
  - Recebe comando do kernel → executa task → reporta progresso
  - Protocolo: `POST /execute`, streaming progress via SSE
- **Lifecycle completo**:
  - Spawn container com `--network none` (default)
  - Mount volume compartilhado em `/mnt/aule/workspace/{job_id}`
  - Timeout + kill automático
  - Zombie reaping na startup do kernel
- **Padrão absorvido**: Ollama model lifecycle; Docker SDK já no projeto

**Exit Criteria**:

- Job de imagem executa em container isolado real
- Progresso real do worker → SSE → frontend
- Container destruído após conclusão

---

### M16 — Pipeline de Execução Multi-step (TODO)

Jobs compostos por múltiplos steps sequenciais ou paralelos.

**Escopo**:

- **Domínio**: struct `Pipeline` com `Steps[]` (capability + input mapping)
  - Ex: "Gere apresentação" → `generate_text` (outline) → `generate_image` (slides) → `generate_document` (PDF)
- **Agente como orquestrador**: ReAct decide pipeline, kernel executa steps
- **Parallel execution**: Steps independentes rodam em paralelo (inspirado Cogito)
- **Padrão absorvido**: Cogito Goal Planning com TODOs; Grafana pipeline composition

**Exit Criteria**:

- Um fluxo multi-step funcional end-to-end
- Steps paralelos quando possível
- Resultado final é artefato composto

---

## FASE 6 — EXPORT & PUBLISH

> **Objetivo**: O output do auleOS vira material entregável.

### M17 — Export & Publish Pipeline (TODO)

O output do auleOS vira material entregável.

**Escopo**:

- **Formatos de export**:
  - Markdown → PDF (Pandoc worker)
  - Markdown → Slides (reveal.js ou Marp worker)
  - TTS → Audio track (Piper/Kokoro worker)
  - Composição → Video (FFmpeg worker com narração + slides)
- **Template system**: Templates pré-definidos (relatório, apresentação, tutorial)
- **One-click publish**: Gera artefato final a partir da conversa/projeto

### M18 — Command Palette Avançado & Polish (TODO)

**Escopo**:

- ⌘K palette com busca federada (conversas, projetos, artefatos, agentes, tools)
- Keyboard shortcuts globais
- Drag & drop de arquivos para upload
- Theming (light/dark/system)
- Responsive layout para diferentes tamanhos de tela

**Exit Criteria**:

- Um flow conversa → PDF funcional
- Command palette funcional com todas as entidades
- UX polida e responsiva

---

## FASE 7 — HARDENING & OBSERVABILIDADE

### M19 — Segurança Zero-Trust (TODO)

**Escopo**:

- `--network none` padrão em workers
- FS read-only + exceções explícitas (workspace/tmp)
- Workers rodam como user `aule` (non-root)
- Zombie reaping na inicialização do kernel
- Rate limiting na API
- CORS restritivo em produção

---

### M20 — Observabilidade Glass Box (TODO)

**Escopo**:

- Métricas por job: latência LLM, tempo de tool, consumo de recursos
- Tracing por JobID (fluxo completo pedido→artefato)
- Painel de saúde no frontend (workers ativos, filas, erros)
- Structured logging com `slog` em todos os caminhos críticos
- **Padrão absorvido**: Grafana observability; Open WebUI OpenTelemetry

---

### M21 — Testes, CI & Release (TODO)

**Escopo**:

- Suite de integração: chat ReAct → job → artefato → SSE
- Testes por capability handler
- E2E mínimo no frontend (Playwright)
- Makefile: `test`, `lint`, `build`, `release`
- Runbook de operação
- Docker Compose para deploy local one-command

---

## FASE 8 — EXTENSIBILIDADE (FUTURO)

### M22 — MCP (Model Context Protocol) Support

Conectar ferramentas externas via protocolo padrão.

- **Padrão**: Cogito MCP integration, LocalAI MCP servers
- Agente usa tools de MCP servers remotos ou locais

### M23 — Plugin System / Custom Actions

Usuário adiciona tools sem recompilar.

- **Padrão**: LocalAGI interpreted Go actions; Open WebUI Pipelines
- Manifesto JSON + script → tool no registry

### M24 — Agent Teaming Avançado

Times de agentes com coordenação sofisticada (handoff básico já implementado em M10).

- **Handoff avançado**: routing LLM-decided (o modelo escolhe para quem transferir), confidence scoring, fallback chains
- **Agent pools**: pool de agentes disponíveis, auto-scaling baseado em demanda
- **Reviewer pattern**: agente reviewer julga output de agente worker (Cogito Content Refinement)
- **Parallel delegation**: gerente divide sub-tarefas e agrega resultados
- **Padrão**: LocalAGI agent pooling; Cogito reviewer judges; Bubo Steps() orchestration

---

## Mapa de execução (ordenado por impacto)

```
                    GRAFO DE DEPENDÊNCIAS

    ┌──────────────────────────────────────────────┐
    │  M6 Conversations (DONE) — ESPINHA DORSAL    │
    └────────┬─────────────────────────────────────┘
             │
             ▼
    ┌──────────────────────────────────────────────┐
    │  M7 Desktop Shell & Workspace                │
    │  (layout OS, dock, center stage, chat como   │
    │   sidebar, projetos, artifact gallery)        │
    └─────┬──────────┬──────────┬──────────────────┘
          │          │          │
          ▼          │          ▼
  ┌────────────┐     │  ┌──────────────────┐
  │ M8 Personas│     │  │ M9 Sub-Agents    │
  │ (na shell) │     │  │ (visíveis no dock│
  └─────┬──────┘     │  └──────────────────┘
        │            │
        ▼            ▼
  ┌─────────────────────────────────────────┐
  │ M10 Agent Definition + Tool Builder     │
  │ M11 Agent Studio Visual + Marketplace   │
  └──────────┬──────────────────────────────┘
             │
     ┌───────┴───────┐
     ▼               ▼
  ┌──────────┐  ┌──────────────┐
  │ M12+M13  │  │ M14-M16      │
  │ RAG      │  │ Workers      │
  └──────────┘  └──────────────┘
```

```
           ORDEM DE EXECUÇÃO LINEAR

  ┌───────────────────────────────────────────────┐
  │ FASE 1: Fundação (DONE)                       │
  │ M1-M5.5 Core + API + UI + Jobs + SSE + Crypto │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 2: Conversas & Desktop Shell             │
  │ M6  Conversations & Memory  ✅ DONE           │
  │ M7  Desktop Shell & Workspace  ✅ DONE        │
  │ M8  Sistema de Personas  ✅ DONE               │
  │ M9  Sub-Agents + Multi-Model  ✅ DONE          │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 3: Agent Studio & Tool Builder           │
  │ M10 Agent Definition + Tool Builder           │
  │ M11 Agent Studio Visual + Tool Marketplace    │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 4: RAG & Knowledge Base                  │
  │ M12 Document Ingestion Pipeline               │
  │ M13 Vector Store & RAG Query                  │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 5: Workers de Produção                   │
  │ M14 Worker Registry & Manifest                │
  │ M15 Watchdog Sidecar                          │
  │ M16 Multi-step Pipeline                       │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 6: Export & Publish                      │
  │ M17 Export Pipeline · M18 Polish              │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 7: Hardening                             │
  │ M19 Segurança · M20 Observabilidade · M21 CI  │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 8: Extensibilidade (futuro)              │
  │ M22 MCP · M23 Plugins · M24 Agent Teaming    │
  └───────────────────────────────────────────────┘
```

### Racional de impacto

| Prioridade | Milestone | Por que nesta posição |
|-----------|-----------|----------------------|
| 🔴 #1 | **M6 Conversations** ✅ | Tudo depende de conversas persistentes |
| 🔴 #2 | **M7 Desktop Shell** | O produto é um SO, não um chatbot. Define a identidade visual AGORA |
| 🟠 #3 | **M8 Personas** ✅ | System prompt dinâmico dentro da nova shell. Baixo esforço, alto impacto |
| 🟡 #4 | **M9 Tools** | Incremental. Cada tool é independente. Aparece no dock do Desktop Shell |
| 🟠 #5 | **M10 Agent Definition + Tool Builder** | Modelo central + criar tools via chat |
| 🔴 #6 | **M11 Agent Studio** | Visual builder com React Flow. DIFERENCIADOR do produto |
| 🟢 #7 | **M12+M13 RAG** | Upload doc → pergunta → resposta com citação |
| 🔵 #8 | **M14-M16 Workers** | Infra de produção. Sistema atual funciona para dev |
| 🟣 #9 | **M17-M18 Export & Polish** | Requer conteúdo e tools maduros |
| ⚪ #10 | **M19-M24 Hardening & Extensibilidade** | Para release |

### Princípio central — "Desktop Agêntico"

```
  ┌─────────────────────────────────────────────────────────────┐
  │  auleOS Desktop Shell                                       │
  │  ┌──────┐  ┌──────────────────────────────┐  ┌──────────┐  │
  │  │ Dock │  │     CENTER STAGE              │  │  Chat    │  │
  │  │      │  │                               │  │  Panel   │  │
  │  │ 🏠   │  │  Dashboard / Project /        │  │  (⌘J)   │  │
  │  │ 📁   │  │  Artifact Gallery /           │  │          │  │
  │  │ 🤖   │  │  Agent Studio Canvas /        │  │ [Agent]  │  │
  │  │ 🔧   │  │  Job Monitor                  │  │ [Chats]  │  │
  │  │ 📊   │  │                               │  │ [History]│  │
  │  │ ⚙️   │  │  (muda conforme contexto)     │  │          │  │
  │  │      │  │                               │  │          │  │
  │  └──────┘  └──────────────────────────────┘  └──────────┘  │
  │  ┌──────────────────────────────────────────────────────┐   │
  │  │ Status Bar: workers • modelo • recursos • ⌘K search │   │
  │  └──────────────────────────────────────────────────────┘   │
  └─────────────────────────────────────────────────────────────┘
```

**O chat é um painel lateral (⌘J para toggle), não a tela inteira.**
**O center stage mostra artefatos, projetos, agent studio canvas — o "conteúdo" do SO.**

### Arquitetura "Agent Studio" (M10+M11)

```
  ┌─────────────────────────────────────────────────────┐
  │              AgentDefinition (única fonte de verdade)│
  │  {persona, tools[], guidelines[], flow_graph,       │
  │   interrupts[], system_prompt}                      │
  └────────────┬────────────────────┬───────────────────┘
               │                    │
       ┌───────▼───────┐    ┌──────▼────────┐
       │  Chat View    │    │ Visual View   │
       │  (conversa    │◄──►│ (React Flow   │
       │   natural)    │    │  canvas)      │
       └───────────────┘    └───────────────┘
```

**O que o chat faz, o visual mostra. O que o visual edita, o chat sabe.**

## Stack de Workers planejada

| Worker | Capability | Modelo Local | API Remota | Status |
|--------|-----------|-------------|------------|--------|
| ComfyUI/sd.cpp | `image.generate` | SD 1.5 GGML, FLUX GGML | OpenAI DALL-E | ✅ Funcional |
| Ollama | `text.generate` | Llama 3.2, Gemma 3 | OpenAI GPT-4 | ✅ Funcional |
| Piper/Kokoro | `audio.generate` | Piper ONNX voices | ElevenLabs API | 🔜 M9 |
| Pandoc | `document.generate` | N/A (local tool) | N/A | 🔜 M9 |
| Moondream2 | `image.analyze` | Moondream2 2B | GPT-4V API | 🔜 M9 |
| Tika/Docling | `document.extract` | N/A (local tool) | N/A | 🔜 M12 |
| nomic-embed | `text.embed` | nomic-embed-text | OpenAI embeddings | 🔜 M13 |
| SearXNG | `web.search` | N/A (self-hosted) | N/A | 🔜 M9 |
| FFmpeg | `video.compose` | N/A (local tool) | N/A | 📋 M17 |

## Dependências de frontend planejadas

| Pacote | Versão | Para que | Milestone |
|--------|--------|---------|-----------|
| `@xyflow/react` | latest | Engine visual node-based (Agent Studio canvas) | M11 |
| `zustand` | (já instalado) | State management local | - |
| `@tanstack/react-query` | (já instalado) | Data fetching | - |

---

## Comandos de validação rápida

```bash
# Backend
cd /home/gohan/auleOS/auleOS && go build ./... && go vet ./... && go test ./...

# Frontend
cd /home/gohan/auleOS/auleOS/web && npm run build

# Smoke API
curl -s http://localhost:8080/v1/agent/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"gere uma imagem de cidade futurista","model":"llama3.2"}' | jq

# Settings
curl -s http://localhost:8080/v1/settings | jq
```
