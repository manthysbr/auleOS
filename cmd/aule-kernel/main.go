package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/manthysbr/auleOS/internal/adapters/docker"
	"github.com/manthysbr/auleOS/internal/adapters/duckdb"
	"github.com/manthysbr/auleOS/internal/core/ports"
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
	repo, err := duckdb.NewRepository("aule.db")
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

	// Application Loop (using errgroup as per rules)
	g, gCtx := errgroup.WithContext(ctx)

	// Example background service or API server could be started here.
	// For now, we just wait for context cancel.
	g.Go(func() error {
		logger.Info("kernel is running")
		<-gCtx.Done()
		return nil
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
