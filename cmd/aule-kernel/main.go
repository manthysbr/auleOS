package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"path/filepath"

	"github.com/manthysbr/auleOS/internal/adapters/docker"
	"github.com/manthysbr/auleOS/internal/adapters/duckdb"
	"github.com/manthysbr/auleOS/internal/adapters/providers"
	appconfig "github.com/manthysbr/auleOS/internal/config"
	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/ports"
	"github.com/manthysbr/auleOS/internal/core/services"
	"github.com/manthysbr/auleOS/internal/synapse"
	"github.com/manthysbr/auleOS/pkg/kernel"
	"github.com/rs/cors"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("starting auleOS kernel")

	if err := run(logger); err != nil {
		logger.Error("kernel startup failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		logger.Info("shutting down")
		cancel()
	}()

	// Initialize Adapters
	dbPath := os.Getenv("AULE_DB_PATH")
	if dbPath == "" {
		dbPath = "aule.db"
	}

	repo, err := duckdb.NewRepository(dbPath)
	if err != nil {
		return fmt.Errorf("failed to init repository: %w", err)
	}

	workerMgr, err := docker.NewManager()
	if err != nil {
		return fmt.Errorf("failed to init docker manager: %w", err)
	}

	// Run Zombie Reaping (Strategy Phase A)
	if err := reapZombies(ctx, logger, workerMgr, repo); err != nil {
		// Deciding whether to fail hard or log error. Rule says "NEVER swallow errors", usually means wrap/return.
		// For startup critical task, failing is appropriate.
		return fmt.Errorf("zombie reaping failed: %w", err)
	}

	// Initialize Core Services
	eventBus := services.NewEventBus(logger) // Telemetry
	workspaceMgr := services.NewWorkspaceManager()

	jobScheduler := services.NewJobScheduler(logger, services.SchedulerConfig{
		MaxConcurrentJobs: 10,
	})

	// Provider Registry - manages local/remote providers
	// Initialize encryption for API key storage
	secretKey, err := appconfig.NewSecretKey()
	if err != nil {
		return fmt.Errorf("failed to init secret key: %w", err)
	}

	// Settings store: loads persisted config from DuckDB with encrypted secrets
	settingsStore, err := appconfig.NewSettingsStore(logger, repo, secretKey)
	if err != nil {
		return fmt.Errorf("failed to init settings store: %w", err)
	}

	config := settingsStore.GetConfig()

	llmProvider, imageProvider, err := providers.Build(config)
	if err != nil {
		return fmt.Errorf("failed to build providers from config: %w", err)
	}

	lifecycle := services.NewWorkerLifecycle(logger, jobScheduler, workerMgr, repo, workspaceMgr, eventBus, llmProvider, imageProvider)

	// Tool Registry - register available tools
	toolRegistry := domain.NewToolRegistry()
	generateImageTool := services.NewGenerateImageTool(lifecycle)
	if err := toolRegistry.Register(generateImageTool); err != nil {
		logger.Error("failed to register generate_image tool", "error", err)
		return err
	}
	generateTextTool := services.NewGenerateTextTool(lifecycle)
	if err := toolRegistry.Register(generateTextTool); err != nil {
		logger.Error("failed to register generate_text tool", "error", err)
		return err
	}

	// Synapse Runtime — lightweight Wasm plugin engine
	wasmRT, err := synapse.NewRuntime(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to init synapse runtime: %w", err)
	}
	defer wasmRT.Close(ctx)

	// Host Services — The Bridge between Synapse (Wasm) and Muscle (Docker)
	// Allows plugins to call `aule.delegate` to spawn heavy tasks.
	hostServices := synapse.NewHostServices(logger, lifecycle)
	if err := wasmRT.RegisterHostServices(ctx, hostServices); err != nil {
		return fmt.Errorf("failed to register host services: %w", err)
	}

	// Discover and load Wasm plugins from ~/.aule/plugins/
	homeDir, _ := os.UserHomeDir()
	pluginDir := filepath.Join(homeDir, ".aule", "plugins")
	if envDir := os.Getenv("AULE_PLUGIN_DIR"); envDir != "" {
		pluginDir = envDir
	}
	pluginRegistry := synapse.NewRegistry(logger, wasmRT, pluginDir)
	wasmTools, err := pluginRegistry.DiscoverAndLoad(ctx)
	if err != nil {
		logger.Warn("synapse plugin discovery failed (non-fatal)", "error", err)
	} else if len(wasmTools) > 0 {
		for _, tool := range wasmTools {
			if err := toolRegistry.Register(tool); err != nil {
				logger.Error("failed to register wasm tool", "tool", tool.Name, "error", err)
			}
		}
		logger.Info("synapse plugins loaded", "count", len(wasmTools))
	}

	// Conversation Store - in-memory cache backed by DuckDB (64 conversations cached)
	convStore := services.NewConversationStore(repo, 64)

	// Wire conversation store into lifecycle for async job → chat notifications
	lifecycle.SetConversationStore(convStore)

	// Model Router - resolves which model to use per persona/role
	modelRouter := services.NewModelRouter(logger, llmProvider)

	// Trace Collector — observability engine (Genkit-style tracing)
	traceCollector := services.NewTraceCollector(logger, eventBus)

	// Hot-reload: when settings change, rebuild providers and swap in lifecycle + model router
	settingsStore.OnChange(func(cfg *domain.AppConfig) {
		newLLM, newImage, err := providers.Build(cfg)
		if err != nil {
			logger.Error("failed to rebuild providers on settings change", "error", err)
			return
		}
		lifecycle.UpdateProviders(newLLM, newImage)
		modelRouter.UpdateProvider(newLLM)
		logger.Info("providers hot-reloaded from settings change")
	})

	// Model Discovery - detect installed Ollama models on startup
	discovery := services.NewModelDiscovery(logger)
	ollamaURL := config.Providers.LLM.LocalURL
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	// Strip /v1 suffix — Ollama native API is at /api/tags, not /v1/api/tags
	ollamaURL = strings.TrimRight(strings.TrimSpace(ollamaURL), "/")
	if strings.HasSuffix(ollamaURL, "/v1") {
		ollamaURL = strings.TrimSuffix(ollamaURL, "/v1")
	}
	if discovered, err := discovery.DiscoverOllama(ctx, ollamaURL); err == nil && len(discovered) > 0 {
		modelRouter.SetCatalog(discovered)
		logger.Info("ollama models discovered", "count", len(discovered))
	} else if err != nil {
		logger.Warn("ollama model discovery failed (non-fatal)", "error", err)
	}

	// Sub-Agent Orchestrator - parallel delegation engine
	subOrchestrator := services.NewSubAgentOrchestrator(logger, modelRouter, toolRegistry, repo, eventBus, wasmRT)

	// Register delegate tool (must be after orchestrator creation)
	delegateTool := services.NewDelegateTool(subOrchestrator)
	if err := toolRegistry.Register(delegateTool); err != nil {
		logger.Error("failed to register delegate tool", "error", err)
		return err
	}

	// Tool Forge — LLM-driven tool creation (text → Go → Wasm → hot-load)
	forge := synapse.NewForge(logger, modelRouter, "qwen2.5:latest", wasmRT, toolRegistry, pluginDir)
	createToolTool := services.NewCreateToolTool(forge)
	if err := toolRegistry.Register(createToolTool); err != nil {
		logger.Error("failed to register create_tool tool", "error", err)
	}
	listForgedTool := services.NewListForgedToolsTool(forge)
	if err := toolRegistry.Register(listForgedTool); err != nil {
		logger.Error("failed to register list_forged_tools tool", "error", err)
	}
	logger.Info("tool forge initialized", "plugin_dir", pluginDir)

	// Core Agent Tools (M10)
	// FS Tools
	if err := toolRegistry.Register(services.NewReadFileTool(workspaceMgr)); err != nil {
		logger.Error("failed to register read_file tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewWriteFileTool(workspaceMgr)); err != nil {
		logger.Error("failed to register write_file tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewListDirTool(workspaceMgr)); err != nil {
		logger.Error("failed to register list_dir tool", "error", err)
	}
	// Exec Tool
	execTool := services.NewExecTool(workspaceMgr)
	if err := toolRegistry.Register(execTool); err != nil {
		logger.Error("failed to register exec tool", "error", err)
	}
	// Web Search Tool
	if err := toolRegistry.Register(services.NewWebSearchTool()); err != nil {
		logger.Error("failed to register web_search tool", "error", err)
	}
	// Web Fetch Tool
	if err := toolRegistry.Register(services.NewWebFetchTool()); err != nil {
		logger.Error("failed to register web_fetch tool", "error", err)
	}
	// Memory Tools
	if err := toolRegistry.Register(services.NewMemorySaveTool(workspaceMgr)); err != nil {
		logger.Error("failed to register memory_save tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewMemoryReadTool(workspaceMgr)); err != nil {
		logger.Error("failed to register memory_read tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewMemorySearchTool(workspaceMgr)); err != nil {
		logger.Error("failed to register memory_search tool", "error", err)
	}
	// FS Tools — edit_file, append_file
	if err := toolRegistry.Register(services.NewEditFileTool(workspaceMgr)); err != nil {
		logger.Error("failed to register edit_file tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewAppendFileTool(workspaceMgr)); err != nil {
		logger.Error("failed to register append_file tool", "error", err)
	}

	// ReAct Agent Service - agentic reasoning with tools + model routing + tracing
	reactAgent := services.NewReActAgentService(logger, llmProvider, modelRouter, toolRegistry, convStore, repo, workspaceMgr, traceCollector)

	// Seed built-in personas (idempotent — ON CONFLICT DO NOTHING)
	for _, p := range domain.BuiltinPersonas() {
		if err := repo.CreatePersona(ctx, p); err != nil {
			logger.Warn("failed to seed persona", "persona", p.Name, "error", err)
		}
	}
	logger.Info("built-in personas seeded")

	// Capability Router — decides Synapse vs Muscle per capability
	capRouter := services.NewCapabilityRouter(logger, wasmRT)

	// Workflow Engine (M12)
	workflowExec := services.NewWorkflowExecutor(logger, repo, reactAgent, eventBus)

	// Register Workflow Tools
	if err := toolRegistry.Register(services.NewCreateWorkflowTool(repo)); err != nil {
		logger.Error("failed to register create_workflow tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewRunWorkflowTool(workflowExec, repo)); err != nil {
		logger.Error("failed to register run_workflow tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewListWorkflowsTool(repo)); err != nil {
		logger.Error("failed to register list_workflows tool", "error", err)
	}

	// Scheduled Task Tools (M11)
	if err := toolRegistry.Register(services.NewScheduleTaskTool(repo)); err != nil {
		logger.Error("failed to register schedule_task tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewListScheduledTasksTool(repo)); err != nil {
		logger.Error("failed to register list_scheduled_tasks tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewCancelScheduledTaskTool(repo)); err != nil {
		logger.Error("failed to register cancel_scheduled_task tool", "error", err)
	}
	if err := toolRegistry.Register(services.NewToggleScheduledTaskTool(repo)); err != nil {
		logger.Error("failed to register toggle_scheduled_task tool", "error", err)
	}

	// Message Tool — proactive communication from agent to user (PicoClaw pattern)
	if err := toolRegistry.Register(services.NewMessageTool(eventBus)); err != nil {
		logger.Error("failed to register message tool", "error", err)
	}

	// Spawn Tool — async background sub-agent (PicoClaw pattern)
	if err := toolRegistry.Register(services.NewSpawnTool(subOrchestrator, eventBus, logger)); err != nil {
		logger.Error("failed to register spawn tool", "error", err)
	}

	// Initialize Kernel API Server
	apiServer := kernel.NewServer(logger, lifecycle, reactAgent, eventBus, settingsStore, convStore, modelRouter, discovery, capRouter, wasmRT, workflowExec, traceCollector, workerMgr, repo)

	// CronScheduler — executes scheduled tasks (M11)
	cronScheduler := services.NewCronScheduler(logger, repo, reactAgent, eventBus)

	// HeartbeatService — processes HEARTBEAT.md checklists (M11)
	heartbeatSvc := services.NewHeartbeatService(logger, workspaceMgr, reactAgent, repo, 30*time.Minute)

	// Setup HTTP Server
	// CORS Configuration
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:5174"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	handler := c.Handler(apiServer.Handler())

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// Application Loop (using errgroup as per rules)
	g, gCtx := errgroup.WithContext(ctx)

	// 1. Start Worker Lifecycle (Scheduler Loop)
	g.Go(func() error {
		return lifecycle.Run(gCtx)
	})

	// 2. Start API Server
	g.Go(func() error {
		logger.Info("starting user api server", "addr", ":8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("api server failed: %w", err)
		}
		return nil
	})

	// 3. Graceful Shutdown for API Server
	g.Go(func() error {
		<-gCtx.Done()
		logger.Info("shutting down api server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	})

	// 4. CronScheduler loop (M11)
	g.Go(func() error {
		return cronScheduler.Run(gCtx)
	})

	// 5. HeartbeatService loop (M11)
	g.Go(func() error {
		return heartbeatSvc.Run(gCtx)
	})

	return g.Wait()
}

// reapZombies implements the startup cleanup strategy
func reapZombies(ctx context.Context, logger *slog.Logger, mgr ports.WorkerManager, repo ports.Repository) error {
	logger.Info("running zombie reaper")

	// TODO: Implement the full reconciliation loop defined in PLAN.md
	// 1. Fetch running containers from Docker (mgr.List)
	// 2. Fetch running workers from DB (repo.ListWorkers)
	// 3. Compare and cleanup

	return nil
}
