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

### M9.5 — Synapse Runtime + Tool Forge ✅ DONE

**IMPACTO: ESTRUTURAL** — Wasm plugin system + criação de tools em runtime via LLM.

**Entregue**:

- **Synapse Wasm Runtime** (wazero v1.11.0): AOT compiler, stdin/stdout JSON protocol, plugin discovery em `~/.aule/plugins/`
- **Tool Forge**: LLM → Go source → `GOOS=wasip1 GOARCH=wasm go build` → hot-load no registry
  - Tools `create_tool` e `list_forged_tools` registradas no ReAct agent
  - `cleanCodeResponse()`, `extractParamsFromSource()`, `sanitizeToolName()`
- **Host Functions seguras**: `aule.log`, `aule.metric`, `aule.secret_get` (zeroização), `aule.http_fetch` (SSRF protection + domain allowlist), `aule.kv_get/set/delete` (namespaced per plugin)
- **Capability Router**: GPU→Muscle (Docker), Logic→Synapse (Wasm), 9 rotas default
- **API**: `GET /v1/plugins`, `GET /v1/capabilities`
- **Frontend**: ToolsView com dados live de plugins e capabilities
- **Sub-agents Wasm**: Fast-path `runtime=synapse` no SubAgentOrchestrator

**Evidência**: `internal/synapse/` (runtime.go, plugin.go, forge.go, host_services.go, host_functions.go, registry.go, capability_router.go, config_vault.go), `internal/core/services/forge_tool.go`, `web/src/components/shell/ToolsView.tsx`

**Validação**: 15 packages, 17+ testes passando, plugins Wasm executam E2E (celsius_to_fahrenheit, text_to_base64, address_to_json criados via chat)

---

## FASE 3 — AGENT OS: TOOLS FUNDAMENTAIS + AUTONOMIA

> **Inspiração**: PicoClaw (file ops, exec, web search, cron, heartbeat, spawn, memory — tudo em Go), Genkit (Tool Interrupts, Sessions), Cogito (Goal Planning)
>
> **Objetivo**: Transformar o auleOS de "agente que conversa" para **agente que FAZ coisas**. Sem tools de file system, exec e web search, o agente é um chatbot glorificado. Com elas, vira um SO agêntico real — o usuário pede "crie um projeto Node.js com testes" e o agente cria arquivos, instala deps, roda testes.
>
> **Princípio**: Agentes precisam de **mãos** (tools de ação), **olhos** (web search, file read) e **memória** (long-term memory). PicoClaw provou que file ops + exec + search bastam para 90% dos use cases. Nós adicionamos Forge + Wasm + Desktop visual por cima.
>
> **Referência direta**: PicoClaw — `read_file`, `write_file`, `edit_file`, `list_dir`, `exec` (sandboxed), `web_search`, `spawn`, `cron`, `MEMORY.md`, `HEARTBEAT.md`. Mesmo stack Go, mesma filosofia de agent-as-OS.

### M10 — Core Agent Tools (TODO)

**IMPACTO: MÁXIMO** — Sem ferramentas fundamentais, agentes não fazem nada útil. Isso desbloqueia tudo.

Implementar as tools que tornam o agente capaz de **operar no sistema** — ler/escrever arquivos, executar comandos, buscar na web. Tudo sandboxed por projeto/workspace.

**Escopo**:

- **File System Tools** (nativas no ToolRegistry):
  - `read_file` — lê conteúdo de arquivo dentro do workspace do projeto
  - `write_file` — cria/sobrescreve arquivo no workspace
  - `edit_file` — edição cirúrgica (search & replace) num arquivo existente
  - `list_dir` — lista diretório no workspace
  - `append_file` — adiciona conteúdo ao final de um arquivo
- **Exec Tool** (nativa, sandboxed):
  - `exec` — executa comando shell dentro do workspace
  - **Security sandbox** (inspirado PicoClaw):
    - `restrict_to_workspace: true` por padrão — todos os paths resolvidos dentro de `workspace/projects/{project_id}/`
    - Blocklist de comandos perigosos: `rm -rf`, `format`, `mkfs`, `dd if=`, `shutdown`, `reboot`, fork bomb
    - Timeout configurável (default: 30s)
    - Stdout/stderr capturados e retornados como observation
    - Sub-agents herdam mesma restrição de sandbox
  - **Implementação**: `os/exec.CommandContext` com `Dir` setado para workspace, environment limpo
- **Web Search Tool** (nativa):
  - `web_search` — busca na web via DuckDuckGo (sem API key) ou Brave Search API (com key)
  - Retorna top N resultados (título, snippet, URL)
  - Fallback automático DuckDuckGo → Brave
  - **Implementação**: HTTP scraping do DuckDuckGo HTML ou Brave Search API REST
- **Web Fetch Tool** (nativa ou Wasm):
  - `web_fetch` — faz GET numa URL e retorna conteúdo (text/HTML com readability extraction)
  - Usa mesma infra de SSRF protection do `aule.http_fetch` (deny list: localhost, metadata endpoints)
  - Limite de 1MB response, timeout 30s
- **Integração com ReAct**:
  - Todas as tools registradas no ToolRegistry com `ExecType: ExecNative`
  - Aparecem no prompt do ReAct agent automaticamente
  - Personas podem filtrar via `AllowedTools` (ex: persona "Coder" tem exec+file tools, "Researcher" tem web_search+web_fetch)
- **Contrato OpenAPI**: Atualizar spec com tools listadas em `/v1/capabilities`

**Exit Criteria**:

- Agente consegue: criar arquivo, ler arquivo, executar `ls`, buscar na web — tudo via chat
- Sandbox previne acesso fora do workspace e comandos perigosos
- Build/test passa com testes unitários para cada tool + sandbox

---

### M11 — Memória Persistente + Tarefas Agendadas (TODO)

**IMPACTO: ALTO** — Agentes com memória cross-session e capacidade de agir autonomamente no tempo.

Sem memória de longo prazo, o agente esquece tudo entre conversas. Sem cron, o agente só age quando o usuário fala. Um SO real precisa de ambos.

**Escopo**:

- **Long-Term Memory**:
  - Banco de memória por projeto: `workspace/projects/{id}/MEMORY.md`
  - Tool `memory_save` — agente salva fatos importantes ("o usuário prefere Python", "o deploy é na AWS")
  - Tool `memory_search` — busca na memória por keyword/relevância (simples grep primeiro, embeddings depois)
  - O agente AUTOMATICAMENTE consulta memória no início de cada conversa (inject no context window)
  - DuckDB: tabela `memories` (id, project_id, content, category, created_at) para busca estruturada
  - Categorias: `fact`, `preference`, `decision`, `context`
  - **Padrão absorvido**: PicoClaw `MEMORY.md` (simples e eficaz); Open WebUI long-term memory
- **Cron / Scheduled Tasks**:
  - Tool `schedule_task` — agente cria tarefa agendada ("me lembre em 10 minutos", "a cada 2 horas verifique X")
  - `CronScheduler` no kernel: goroutine que verifica a cada 1 minuto se há tasks para executar
  - DuckDB: tabela `scheduled_tasks` (id, project_id, cron_expr, prompt, next_run, status, created_by)
  - Tipos: one-shot ("daqui 10min"), recorrente ("a cada 2h"), cron expression ("0 9 * * *")
  - Execução: cria uma conversa temporária, roda o ReAct agent com o prompt da task, salva resultado
  - API: `GET/POST /v1/tasks/scheduled`, `DELETE /v1/tasks/scheduled/{id}`
  - **Padrão absorvido**: PicoClaw `cron` tool + `HEARTBEAT.md`
- **Heartbeat** (periodic check):
  - Arquivo `HEARTBEAT.md` por projeto com checklist de tarefas periódicas
  - Kernel lê a cada N minutos (configurável, default 30min) e executa via ReAct
  - Tasks longas rodam como sub-agent (spawn) para não bloquear
  - **Padrão absorvido**: PicoClaw heartbeat com spawn async
- **Frontend**:
  - Seção "Memory" na view de projeto (lista de memórias com filtro por categoria)
  - Seção "Scheduled Tasks" na view de jobs (lista com next_run, status, delete)

**Exit Criteria**:

- Agente lembra informações entre sessões ("qual é minha linguagem preferida?" → responde corretamente)
- Task agendada executa no horário e resultado aparece na conversa
- Heartbeat funcional com pelo menos 1 task periódica
- Build/test passa

---

## FASE 4 — WORKFLOW ENGINE + AGENT DEFINITIONS

> **Inspiração**: LangGraph (state graph com DAG), CrewAI (crews + tasks sequenciais), Cogito (Goal Planning), PicoClaw (spawn, subagents), Genkit (Tool Interrupts, DefineFlow)
>
> **Objetivo**: O usuário **cria seus próprios fluxos agênticos**. Não é um fluxo editorial hardcoded — é uma plataforma onde qualquer workflow multi-step com personas, tools e dependências pode ser montado, executado e monitorado. O agente vira o orquestrador de um pipeline que o usuário desenha.
>
> **Princípio**: Um workflow é um **DAG de steps** onde cada step é um sub-agent (persona + tools + prompt). Steps podem ter dependências (`DependsOn`), compartilhar estado, e pausar para aprovação humana. O usuário define o workflow via chat ("crie um fluxo que...") OU via visual builder — ambos geram a mesma representação.

### M12 — Workflow DAG Engine (TODO)

**IMPACTO: MÁXIMO** — O motor que permite qualquer fluxo agêntico customizado. Núcleo da plataforma.

Evoluir o `SubAgentOrchestrator` de fan-out paralelo flat para um **executor de DAG** com dependências, estado compartilhado e interrupts.

**Escopo**:

- **Domain Types**:
  ```go
  type WorkflowID string

  type Workflow struct {
      ID          WorkflowID         `json:"id"`
      ProjectID   ProjectID          `json:"project_id"`
      Name        string             `json:"name"`
      Description string             `json:"description"`
      Steps       []WorkflowStep     `json:"steps"`
      State       map[string]any     `json:"state"`       // shared state entre steps
      Status      WorkflowStatus     `json:"status"`      // pending/running/paused/completed/failed
      CreatedAt   time.Time          `json:"created_at"`
      StartedAt   *time.Time         `json:"started_at"`
      CompletedAt *time.Time         `json:"completed_at"`
  }

  type WorkflowStep struct {
      ID          string             `json:"id"`          // "research", "write", "review"
      PersonaID   PersonaID          `json:"persona_id"`  // qual persona roda este step
      Prompt      string             `json:"prompt"`      // o que fazer (pode interpolar {{state.research_result}})
      Tools       []string           `json:"tools"`       // tools permitidas neste step
      DependsOn   []string           `json:"depends_on"`  // IDs de steps que precisam completar antes
      Interrupt   *InterruptRule     `json:"interrupt"`   // pausa para aprovação humana (opcional)
      Status      StepStatus         `json:"status"`
      Result      *StepResult        `json:"result"`
      MaxIters    int                `json:"max_iters"`   // default 5
  }

  type InterruptRule struct {
      Before  bool   `json:"before"`  // pausar ANTES de executar o step
      After   bool   `json:"after"`   // pausar DEPOIS (para review do output)
      Message string `json:"message"` // mensagem para o humano ("Revise o draft antes de continuar")
  }

  type StepResult struct {
      Output    string         `json:"output"`
      Artifacts []ArtifactID   `json:"artifacts"`
      Duration  time.Duration  `json:"duration"`
      TokensIn  int            `json:"tokens_in"`
      TokensOut int            `json:"tokens_out"`
  }
  ```
- **DAG Executor**:
  - Topological sort dos steps baseado em `DependsOn`
  - Steps sem dependências rodam em paralelo (goroutines, como já fazemos)
  - Quando step completa, libera dependentes (channel-based signaling)
  - **SharedState**: `Workflow.State` é um `map[string]any` que cada step pode ler e escrever
    - Step "research" escreve `state["research_facts"] = "..."`, step "write" lê `{{state.research_facts}}`
    - Template interpolation no prompt antes de executar
  - **Interrupts**: quando step tem `InterruptRule`, executor pausa e emite evento SSE `workflow.interrupt`
    - Frontend mostra card de aprovação com o output do step
    - API `POST /v1/workflows/{id}/resume` retoma execução (opcionalmente com edição do state)
    - API `POST /v1/workflows/{id}/cancel` cancela o workflow
  - **Error handling**: step falha → marca workflow como `failed` (configurável: retry, skip, abort)
  - Eventos SSE por step: `step.started`, `step.completed`, `step.failed`, `step.interrupted`
- **Workflow Tool**:
  - Tool `create_workflow` — o agente ReAct gera a definição do workflow a partir do pedido do usuário
  - Tool `run_workflow` — executa um workflow definido
  - Tool `list_workflows` — lista workflows do projeto
  - O agente pode criar workflows complexos via conversa natural:
    - User: "crie um fluxo que pesquisa sobre IA, escreve um artigo, gera imagens e revisa antes de publicar"
    - Agent: cria workflow com 4 steps, dependências sequenciais, interrupt no review
- **Persistência**: DuckDB tabelas `workflows`, `workflow_steps`
- **API**:
  - `POST /v1/workflows` — cria workflow
  - `GET /v1/workflows` — lista por projeto
  - `GET /v1/workflows/{id}` — status + steps + state
  - `POST /v1/workflows/{id}/run` — inicia execução
  - `POST /v1/workflows/{id}/resume` — retoma após interrupt
  - `POST /v1/workflows/{id}/cancel` — cancela
  - `GET /v1/workflows/{id}/events` — SSE real-time

**Exit Criteria**:

- Workflow de 3+ steps com dependências executa corretamente (A→B, A→C, [B,C]→D)
- SharedState flui entre steps
- Interrupt funcional (pausa, mostra no frontend, resume retoma)
- Criação de workflow via chat (tool `create_workflow`)
- Build/test passa

---

### M13 — Agent Definitions + Teams + Templates (TODO)

**IMPACTO: ALTO** — Agentes como entidades configuráveis + fluxos reutilizáveis.

O modelo `AgentDefinition` define um agente completo (persona + tools + guidelines + workflow graph). `AgentTeam` agrupa agentes com handoff. `WorkflowTemplate` oferece fluxos prontos como ponto de partida.

**Escopo**:

- **AgentDefinition**:
  ```go
  type AgentDefinition struct {
      ID           string            `json:"id"`
      Name         string            `json:"name"`
      Description  string            `json:"description"`
      PersonaID    PersonaID         `json:"persona_id"`
      SystemPrompt string            `json:"system_prompt"`
      Tools        []string          `json:"tools"`
      Guidelines   []string          `json:"guidelines"`    // Cogito-style rules
      Interrupts   []InterruptRule   `json:"interrupts"`
      FlowGraph    *FlowGraph        `json:"flow_graph"`    // visual representation
      CreatedAt    time.Time         `json:"created_at"`
      UpdatedAt    time.Time         `json:"updated_at"`
  }
  ```
- **AgentTeam**:
  ```go
  type AgentTeam struct {
      ID      string            `json:"id"`
      Name    string            `json:"name"`
      Agents  []TeamMember      `json:"agents"`
      Router  RoutingStrategy   `json:"router"`     // round-robin, skill-based, LLM-decided
      Handoff []HandoffRule     `json:"handoff_rules"`
  }

  type HandoffRule struct {
      FromAgent  string   `json:"from_agent"`
      ToAgent    string   `json:"to_agent"`
      Condition  string   `json:"condition"`  // "topic=code", "confidence<0.5"
      Strategy   string   `json:"strategy"`   // "auto" | "interrupt"
  }
  ```
- **Workflow Templates** (built-in, como personas built-in):
  - **Research & Report**: Researcher → Writer → Reviewer (3 steps, interrupt no review)
  - **Code Project**: Architect → Coder → Tester (3 steps, exec + file tools)
  - **Content Pipeline**: Research → Draft → Images → Edit (4 steps, multi-modal)
  - **Data Analysis**: Fetch → Process → Visualize → Summarize (4 steps)
  - Templates são `Workflow` com steps pré-configurados, usuário pode clonar e customizar
- **Backend**:
  - CRUD `/v1/agents` (AgentDefinition)
  - CRUD `/v1/agent-teams`
  - `GET /v1/workflow-templates` (built-in templates)
  - `POST /v1/workflow-templates/{id}/instantiate` (clona template como workflow)
  - Tabelas DuckDB: `agent_definitions`, `agent_teams`, `workflow_templates`
  - Tools `create_agent`, `edit_agent` para criação via chat
- **Handoff Protocol**:
  - Tool function `handoff` que retorna `{handoff_to: "agent_id", context: {...}}`
  - O kernel redireciona para o agente destino com contexto transferido
  - Estratégia `auto` (agente decide) ou `interrupt` (humano confirma)
- **Frontend**:
  - Agents view: lista de AgentDefinitions com create/edit/delete
  - Teams view: visualização do time como cards conectados
  - Template gallery: grid de templates com "Use Template" button

**Exit Criteria**:

- CRUD de AgentDefinition funcional via API e via chat
- Handoff entre 2 agentes funciona
- Pelo menos 2 templates built-in usáveis
- Build/test passa

---

## FASE 5 — OBSERVABILIDADE + VISUAL BUILDER

> **Inspiração**: Grafana (dashboards de observabilidade), Genkit (traced sub-steps), Langflow/Flowise (visual builder com React Flow)
>
> **Objetivo**: Primeiro você **vê** o que os agentes fazem (observabilidade). Depois você **desenha** visualmente os fluxos (visual builder). A ordem importa: observabilidade sem visual builder é útil, visual builder sem observabilidade é perigoso.

### M14 — Glass Box Observability (TODO)

**IMPACTO: ALTO** — Visibilidade total do que os agentes fazem. Core da proposta "Glass Box".

Métricas, tracing e dashboards para monitorar workflows, steps, token usage e performance em tempo real.

**Escopo**:

- **Métricas por step/workflow**:
  - `tokens_in`, `tokens_out` por chamada LLM (extrair do response da Ollama/OpenAI)
  - `duration_ms` por step, por tool call, por workflow total
  - `model_used` por step (qual modelo rodou)
  - `tool_calls_count` por step
  - Custo estimado (tokens × preço por token configurável)
- **Storage**: DuckDB tabela `metrics` (workflow_id, step_id, metric_name, value, timestamp)
- **Tracing por Workflow**:
  - Trace completo: workflow → steps → tool calls → LLM calls
  - Cada nível com timestamps e durations
  - Visualizável como waterfall/timeline no frontend
- **API**:
  - `GET /v1/workflows/{id}/metrics` — métricas agregadas
  - `GET /v1/metrics/summary` — dashboard global (total tokens, workflows executados, success rate)
- **Frontend — Dashboard de Observabilidade**:
  - Stat cards: Total Workflows / Success Rate / Avg Duration / Total Tokens / Estimated Cost
  - Timeline view: workflow em execução com steps como barra de progresso (SSE real-time)
  - Token usage chart: consumo por dia/semana
  - Model usage breakdown: qual modelo é mais usado
  - Integração no Dashboard existente (novos cards)
- **Host Function `aule.metric`**: Já existe no host_services.go — evoluir de log-only para persistência real

**Exit Criteria**:

- Workflow completado mostra métricas (tokens, duração, custo)
- Dashboard no frontend com pelo menos 4 stat cards + timeline de workflow
- Build/test passa

---

### M15 — Agent Studio Visual Builder (TODO)

**IMPACTO: DIFERENCIADOR** — Interface visual para criar fluxos agênticos com drag & drop.

Canvas node-based usando React Flow para projetar workflows e agent definitions visualmente, com sincronização bidirecional com o chat.

**Escopo**:

- **Frontend — Visual Builder** (React Flow / `@xyflow/react`):
  - Canvas node-based com tipos de nó:
    - `PersonaNode` — seleciona persona para o step
    - `ToolNode` — configura tool com parâmetros
    - `ConditionNode` — if/else baseado em state
    - `InterruptNode` — checkpoint human-in-the-loop
    - `OutputNode` — resultado final / artifact
  - Edges conectando nós (fluxo de execução = DependsOn)
  - Sidebar de componentes (drag & drop)
  - Panel lateral para configurar propriedades de cada nó
  - Preview do `Workflow` JSON resultante
  - **Live execution overlay**: quando workflow roda, nós mudam de cor conforme status (pending→running→done/failed)
- **Tela Híbrida**:
  - Layout split: Chat à esquerda, Canvas à direita (redimensionável)
  - **Bidirecional**: Criar step no canvas → workflow updated. Pedir no chat "adicione um step de review" → nó aparece
  - Toggle: modo chat-only / visual-only / split
  - Mini-mapa do flow no canto
- **Template Gallery Visual**: Grid de templates com preview do grafo, "Use Template" instancia no canvas
- **Sincronização**: FlowGraph no backend é single source of truth. Chat e Canvas leem/escrevem a mesma entidade
- **Padrão absorvido**: React Flow (Langflow/Flowise usam), Langflow dual-mode UX

**Exit Criteria**:

- Workflow criado visualmente executa identicamente a um criado via chat
- Live execution: nós mudam de cor durante execução
- Template gallery com preview
- Build/test passa

---

## FASE 6 — RAG + WORKER INFRASTRUCTURE

> **Objetivo**: Knowledge base para agentes informados + workers Docker reais para tasks pesadas (TTS, PDF, video).

### M16 — Document Ingestion + RAG (TODO)

**IMPACTO: MÉDIO** — Agentes que consultam documentos do usuário para gerar conteúdo informado.

Upload, processamento e busca semântica em documentos.

**Escopo**:

- **Upload API**: `POST /v1/documents` (multipart) — PDF, Markdown, TXT, DOCX
- **Processing**: Extraction → Text splitting → Embedding (Ollama `nomic-embed-text`)
- **Vector store**: chromem-go (in-process, Go puro) ou DuckDB VSS extension
- **RAG no agente**: Tool `search_knowledge` que busca chunks relevantes por similaridade
- **Frontend**: `#` prefix para buscar em docs, indicador "grounded in documents", painel de documentos
- **Padrão absorvido**: LangChainGo loaders/splitters, Open WebUI RAG, chromem-go

**Exit Criteria**:

- Upload de PDF → pergunta sobre conteúdo → resposta com citação da fonte
- Funciona com Ollama embeddings local

---

### M17 — Worker Registry + Watchdog + Pipeline (TODO)

**IMPACTO: MÉDIO** — Workers Docker reais, isolados, com registry de capabilities e pipelines multi-step.

Consolida M14+M15+M16 do plano anterior. Workers containerizados com manifest, sidecar watchdog, e pipelines de steps sequenciais.

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
- **Watchdog sidecar** (já iniciado em `pkg/watchdog/`): HTTP server dentro do container, `POST /execute`, progress SSE
- **Lifecycle**: Spawn com `--network none`, mount workspace volume, timeout + kill, zombie reaping
- **Pipeline Multi-step**: struct `Pipeline` com `Steps[]` (capability + input mapping) — integrado com Workflow Engine (M12)
- **Workers planejados**:
  - Piper/Kokoro TTS → `audio.generate`
  - Pandoc → `document.generate` (Markdown→PDF)
  - FFmpeg → `video.compose`
  - Tika/Docling → `document.extract`
- **Padrão absorvido**: LocalAI Backend Gallery, Ollama lifecycle, Docker SDK

**Exit Criteria**:

- Worker declarado via manifest, instalável via API
- Job executa em container isolado real com progress SSE
- Pipeline multi-step funcional end-to-end
- Build/test passa

---

## FASE 7 — EXPORT, CHANNELS & POLISH

> **Objetivo**: Output entregável + canais de comunicação além do web UI.

### M18 — Export & Publish Pipeline (TODO)

**Escopo**:

- Markdown → PDF (Pandoc worker)
- Markdown → Slides (Marp worker)
- TTS → Audio (Piper/Kokoro worker)
- Composição → Video (FFmpeg: narração + slides)
- Template system (relatório, apresentação, tutorial)
- One-click publish: conversa/projeto → artefato final

### M19 — Chat Channels (TODO)

**IMPACTO: MÉDIO** — Falar com o auleOS via Telegram, Discord, etc. (inspirado PicoClaw gateway).

**Escopo**:

- **Gateway service** (novo binário ou modo do kernel):
  - Telegram bot (token + long-polling)
  - Discord bot (token + intents)
  - Cada mensagem → `POST /v1/agent/chat` no kernel → resposta de volta no canal
  - `allowFrom` para controle de acesso por user ID
- **Implementação**: Go libs — `telegram-bot-api`, `discordgo`
- **Padrão absorvido**: PicoClaw gateway (Telegram, Discord, QQ, DingTalk, LINE)

**Exit Criteria**:

- Mandar mensagem no Telegram → agente responde via auleOS kernel
- Build/test passa

### M20 — Command Palette Avançado & Polish (TODO)

**Escopo**:

- ⌘K palette com busca federada (conversas, projetos, artefatos, agentes, tools, workflows)
- Keyboard shortcuts globais
- Drag & drop de arquivos para upload
- Theming (light/dark/system)
- Responsive layout

---

## FASE 8 — HARDENING

### M21 — Segurança Zero-Trust (TODO)

**Escopo**:

- `--network none` padrão em workers
- FS read-only + exceções explícitas
- Workers rodam como user `aule` (non-root)
- Zombie reaping na startup
- Rate limiting na API
- CORS restritivo em produção

### M22 — Testes, CI & Release (TODO)

**Escopo**:

- Suite de integração: chat → workflow → artefato → SSE
- E2E mínimo frontend (Playwright)
- Makefile: `test`, `lint`, `build`, `release`
- Docker Compose para deploy one-command

---

## FASE 9 — EXTENSIBILIDADE (FUTURO)

### M23 — MCP (Model Context Protocol) Support

Conectar ferramentas externas via protocolo padrão.

- Agente usa tools de MCP servers remotos ou locais
- **Padrão**: Cogito MCP integration, LocalAI MCP servers

### M24 — Advanced Agent Teaming

Coordenação sofisticada de times de agentes (handoff básico em M13).

- Routing LLM-decided, confidence scoring, fallback chains
- Agent pools com auto-scaling
- Reviewer pattern (worker + reviewer em loop — Cogito Content Refinement)
- **Padrão**: Bubo Steps(), Cogito reviewer judges, LocalAGI agent pooling

---

## Mapa de execução (ordenado por impacto)

```
                    GRAFO DE DEPENDÊNCIAS

    ┌──────────────────────────────────────────────────┐
    │  M1-M5.5 Fundação (DONE)                        │
    │  ReAct + API + UI + Jobs + SSE + Crypto          │
    └────────┬─────────────────────────────────────────┘
             │
             ▼
    ┌──────────────────────────────────────────────────┐
    │  M6-M9 Conversas & Desktop Shell (DONE)          │
    │  Conversations + Shell + Personas + Sub-Agents   │
    └────────┬─────────────────────────────────────────┘
             │
             ▼
    ┌──────────────────────────────────────────────────┐
    │  M9.5 Synapse + Forge (DONE)                     │
    │  Wasm Runtime + Tool Forge + Host Functions      │
    └────────┬─────────────────────────────────────────┘
             │
             ▼
    ┌──────────────────────────────────────────────────┐
    │  M10 Core Agent Tools ← FILE + EXEC + WEB       │
    │  (SEM ISSO, AGENTES NÃO FAZEM NADA)             │
    └─────┬────────────────────────────────────────────┘
          │
          ▼
    ┌──────────────────────────────────────────────────┐
    │  M11 Memória + Cron + Heartbeat                  │
    │  (agentes autônomos que lembram e agem no tempo) │
    └─────┬────────────────────────────────────────────┘
          │
          ▼
    ┌──────────────────────────────────────────────────┐
    │  M12 Workflow DAG Engine                         │
    │  (MOTOR PRA QUALQUER FLUXO CUSTOMIZADO)          │
    └─────┬────────────────────────────────────────────┘
          │
          ▼
    ┌───────────────────────────────────────────────────┐
    │  M13 Agent Definitions + Teams + Templates        │
    │  (entidades configuráveis + fluxos reutilizáveis) │
    └──────┬────────────────────────────────────────────┘
           │
     ┌─────┴──────┐
     ▼            ▼
  ┌────────┐  ┌──────────────┐
  │ M14    │  │ M15          │
  │ Glass  │  │ Visual       │
  │ Box    │  │ Builder      │
  └────────┘  └──────────────┘
           │
     ┌─────┴──────────┐
     ▼                ▼
  ┌──────────┐  ┌──────────────┐
  │ M16 RAG  │  │ M17 Workers  │
  └──────────┘  └──────────────┘
           │
           ▼
  ┌──────────────────────────────┐
  │ M18-M20 Export/Channels/UX   │
  └──────────────────────────────┘
           │
           ▼
  ┌──────────────────────────────┐
  │ M21-M24 Hardening + Future   │
  └──────────────────────────────┘
```

```
           ORDEM DE EXECUÇÃO LINEAR

  ┌───────────────────────────────────────────────┐
  │ FASE 1: Fundação ✅ DONE                      │
  │ M1-M5.5 Core + API + UI + Jobs + SSE + Crypto │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 2: Conversas & Desktop Shell ✅ DONE     │
  │ M6 Conversations  ✅                          │
  │ M7 Desktop Shell   ✅                         │
  │ M8 Personas        ✅                         │
  │ M9 Sub-Agents      ✅                         │
  │ M9.5 Synapse+Forge ✅                         │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 3: Agent OS — Tools + Autonomia          │
  │ M10 Core Agent Tools (file/exec/web)          │
  │ M11 Memory + Cron + Heartbeat                 │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 4: Workflow Engine + Agent Definitions   │
  │ M12 Workflow DAG Engine                       │
  │ M13 Agent Definitions + Teams + Templates     │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 5: Observabilidade + Visual Builder      │
  │ M14 Glass Box Observability                   │
  │ M15 Agent Studio Visual Builder               │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 6: RAG + Workers                         │
  │ M16 Document Ingestion + RAG                  │
  │ M17 Worker Registry + Watchdog + Pipeline     │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 7: Export, Channels & Polish             │
  │ M18 Export Pipeline                           │
  │ M19 Chat Channels (Telegram/Discord)          │
  │ M20 Command Palette + Polish                  │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 8: Hardening                             │
  │ M21 Segurança Zero-Trust                      │
  │ M22 Testes, CI & Release                      │
  └──────────────────┬────────────────────────────┘
                     ▼
  ┌───────────────────────────────────────────────┐
  │ FASE 9: Extensibilidade (futuro)              │
  │ M23 MCP · M24 Advanced Agent Teaming          │
  └───────────────────────────────────────────────┘
```

### Racional de impacto — reordenado

| Prioridade | Milestone | Por que nesta posição |
|-----------|-----------|----------------------|
| 🔴 #1 | **M10 Core Agent Tools** | **SEM TOOLS DE AÇÃO, AGENTES NÃO FAZEM NADA.** File ops + exec + web search são o mínimo para ser um "OS". PicoClaw tem. auleOS precisa. |
| 🔴 #2 | **M11 Memory + Cron** | Agentes que esquecem e só agem quando falados não são um OS. Memory + scheduled tasks = autonomia real. PicoClaw tem. |
| 🔴 #3 | **M12 Workflow DAG** | O MOTOR que permite criar qualquer fluxo customizado. Sem isso, sub-agents são flat. Com isso, o usuário monta pipelines arbitrários. |
| 🟠 #4 | **M13 Agent Defs + Teams** | Entidades persistentes + templates reutilizáveis. Transforma de "workflow one-shot" para "plataforma de fluxos". |
| 🟠 #5 | **M14 Glass Box** | Observabilidade vem ANTES do visual builder porque você precisa VER o que acontece antes de DESENHAR. Token tracking, custos, timelines. |
| 🟡 #6 | **M15 Visual Builder** | React Flow canvas para montar fluxos visualmente. DIFERENCIADOR do produto. Mas só faz sentido após ter engine + observabilidade. |
| 🟢 #7 | **M16 RAG** | Knowledge base para agentes informados. Complementar, não blocking. |
| 🟢 #8 | **M17 Workers** | Infra Docker real. Sistema atual funciona para dev. |
| 🔵 #9 | **M18-M20 Export/Channels/Polish** | Layer de output + canais + UX. Requer features maduras. |
| ⚪ #10 | **M21-M24 Hardening + Future** | Para release público. |

### Princípio central — "Agent OS, não Chatbot"

```
  ┌────────────────────────────────────────────────────────────────┐
  │  auleOS Desktop Shell                                          │
  │  ┌──────┐  ┌───────────────────────────────┐  ┌────────────┐  │
  │  │ Dock │  │     CENTER STAGE               │  │   Chat     │  │
  │  │      │  │                                │  │   Panel    │  │
  │  │ 🏠   │  │  Dashboard / Project /         │  │   (⌘J)    │  │
  │  │ 📁   │  │  Workflow Builder (React Flow) │  │            │  │
  │  │ 🤖   │  │  Workflow Monitor (Glass Box)  │  │ [Agent]    │  │
  │  │ 🔧   │  │  Artifact Gallery /            │  │ [Memory]   │  │
  │  │ 📊   │  │  Agent Studio Canvas           │  │ [Scheduled]│  │
  │  │ ⚙️   │  │                                │  │ [History]  │  │
  │  │      │  │  (muda conforme contexto)      │  │            │  │
  │  └──────┘  └───────────────────────────────┘  └────────────┘  │
  │  ┌──────────────────────────────────────────────────────────┐  │
  │  │ Status: workers • modelo • tokens/day • workflows • ⌘K  │  │
  │  └──────────────────────────────────────────────────────────┘  │
  └────────────────────────────────────────────────────────────────┘
```

**O agente FAZ coisas** (file ops, exec, web search), **LEMBRA** (memory), **AGE SOZINHO** (cron/heartbeat), e **ORQUESTRA FLUXOS** (workflow DAG).
**O chat é um painel lateral.** O center stage mostra workflows em execução, resultados, métricas.
**O usuário cria seus fluxos** — via chat ("crie um workflow que...") ou via visual builder (drag & drop).

### Comparação: PicoClaw vs auleOS

| Dimensão | PicoClaw | auleOS |
|----------|----------|--------|
| **Interface** | CLI only | Desktop Shell + Visual Builder + Chat sidebar |
| **File/Exec** | ✅ builtin | M10 (próximo) |
| **Web Search** | ✅ Brave/DDG | M10 (próximo) |
| **Subagents** | ✅ spawn | ✅ delegate + SubAgentOrchestrator |
| **Memory** | MEMORY.md (file) | M11: DuckDB + categorias + busca |
| **Cron** | ✅ cron tool | M11 (próximo) |
| **Heartbeat** | ✅ HEARTBEAT.md | M11 (próximo) |
| **Custom Tools** | Não | ✅ Forge (LLM→Go→Wasm→hot-load) |
| **Workflow DAG** | Não | M12: SharedState + DependsOn + Interrupts |
| **Visual Builder** | Não | M15: React Flow canvas |
| **Observability** | Logs only | M14: Glass Box (tokens, custos, timelines) |
| **Personas** | SOUL.md (1 only) | ✅ Multi-persona com ModelOverride + AllowedTools |
| **Wasm Plugins** | Não | ✅ Synapse (wazero, host functions, hot-load) |
| **Chat Channels** | Telegram/Discord/etc | M19 (futuro) |
| **Templates** | Não | M13: Workflow templates reutilizáveis |
| **Hardware** | $10 / 10MB | Desktop-class (workers Docker, GPU optional) |

**auleOS = PicoClaw capabilities + Desktop UX + Workflow Engine + Visual Builder + Forge + Wasm + Observability.**

## Stack de Workers planejada

| Worker | Capability | Modelo Local | API Remota | Status |
|--------|-----------|-------------|------------|--------|
| ComfyUI/sd.cpp | `image.generate` | SD 1.5 GGML, FLUX GGML | OpenAI DALL-E | ✅ Funcional |
| Ollama | `text.generate` | qwen2.5, llama3.2, gemma3 | OpenAI GPT-4 | ✅ Funcional |
| Piper/Kokoro | `audio.generate` | Piper ONNX voices | ElevenLabs API | 📋 M17 |
| Pandoc | `document.generate` | N/A (local tool) | N/A | 📋 M17 |
| Moondream2 | `image.analyze` | Moondream2 2B | GPT-4V API | 📋 M17 |
| Tika/Docling | `document.extract` | N/A (local tool) | N/A | 📋 M16 |
| nomic-embed | `text.embed` | nomic-embed-text | OpenAI embeddings | 📋 M16 |
| SearXNG | `web.search` | N/A (self-hosted) | N/A | 📋 Alternativa M10 |
| FFmpeg | `video.compose` | N/A (local tool) | N/A | 📋 M18 |

## Dependências de frontend planejadas

| Pacote | Versão | Para que | Milestone |
|--------|--------|---------|-----------|
| `@xyflow/react` | latest | Engine visual node-based (Workflow Builder) | M15 |
| `zustand` | (já instalado) | State management local | - |
| `@tanstack/react-query` | (já instalado) | Data fetching | - |
| `recharts` ou `nivo` | latest | Gráficos de observabilidade (tokens, custos) | M14 |

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
