package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
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

	// Hot-reload: when settings change, rebuild providers and swap in lifecycle
	settingsStore.OnChange(func(cfg *domain.AppConfig) {
		newLLM, newImage, err := providers.Build(cfg)
		if err != nil {
			logger.Error("failed to rebuild providers on settings change", "error", err)
			return
		}
		lifecycle.UpdateProviders(newLLM, newImage)
		logger.Info("providers hot-reloaded from settings change")
	})

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

	// Model Discovery - detect installed Ollama models on startup
	discovery := services.NewModelDiscovery(logger)
	ollamaURL := config.Providers.LLM.LocalURL
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if discovered, err := discovery.DiscoverOllama(ctx, ollamaURL); err == nil && len(discovered) > 0 {
		modelRouter.SetCatalog(discovered)
		logger.Info("ollama models discovered", "count", len(discovered))
	} else if err != nil {
		logger.Warn("ollama model discovery failed (non-fatal)", "error", err)
	}

	// Sub-Agent Orchestrator - parallel delegation engine
	subOrchestrator := services.NewSubAgentOrchestrator(logger, modelRouter, toolRegistry, repo, eventBus)

	// Register delegate tool (must be after orchestrator creation)
	delegateTool := services.NewDelegateTool(subOrchestrator)
	if err := toolRegistry.Register(delegateTool); err != nil {
		logger.Error("failed to register delegate tool", "error", err)
		return err
	}

	// ReAct Agent Service - agentic reasoning with tools + model routing
	reactAgent := services.NewReActAgentService(logger, llmProvider, modelRouter, toolRegistry, convStore, repo)

	// Seed built-in personas (idempotent — ON CONFLICT DO NOTHING)
	for _, p := range domain.BuiltinPersonas() {
		if err := repo.CreatePersona(ctx, p); err != nil {
			logger.Warn("failed to seed persona", "persona", p.Name, "error", err)
		}
	}
	logger.Info("built-in personas seeded")

	// Capability Router — decides Synapse vs Muscle per capability
	capRouter := services.NewCapabilityRouter(logger, wasmRT)

	// Initialize Kernel API Server
	apiServer := kernel.NewServer(logger, lifecycle, reactAgent, eventBus, settingsStore, convStore, modelRouter, discovery, capRouter, wasmRT, workerMgr, repo)

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
