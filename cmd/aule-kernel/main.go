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

	"github.com/manthysbr/auleOS/internal/adapters/docker"
	"github.com/manthysbr/auleOS/internal/adapters/duckdb"
	"github.com/manthysbr/auleOS/internal/adapters/providers"
	appconfig "github.com/manthysbr/auleOS/internal/config"
	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/ports"
	"github.com/manthysbr/auleOS/internal/core/services"
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

	// Conversation Store - in-memory cache backed by DuckDB (64 conversations cached)
	convStore := services.NewConversationStore(repo, 64)

	// ReAct Agent Service - agentic reasoning with tools
	reactAgent := services.NewReActAgentService(logger, llmProvider, toolRegistry, convStore)

	// Legacy Agent Service (for compatibility)
	agentService := services.NewAgentService(logger, llmProvider, imageProvider, lifecycle)

	// Initialize Kernel API Server
	apiServer := kernel.NewServer(logger, lifecycle, reactAgent, agentService, eventBus, settingsStore, convStore, workerMgr, repo)

	// Setup HTTP Server
	// CORS Configuration
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
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
