package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

const HeartbeatFileName = "HEARTBEAT.md"

// HeartbeatService periodically reads HEARTBEAT.md from each active project
// and executes pending checklist items via the ReAct agent.
type HeartbeatService struct {
	logger   *slog.Logger
	ws       *WorkspaceManager
	agent    *ReActAgentService
	repo     heartbeatProjectLister
	interval time.Duration // default 30 minutes
}

// heartbeatProjectLister is the minimal interface to get active projects
type heartbeatProjectLister interface {
	ListProjects(ctx context.Context) ([]domain.Project, error)
}

func NewHeartbeatService(logger *slog.Logger, ws *WorkspaceManager, agent *ReActAgentService, repo heartbeatProjectLister, interval time.Duration) *HeartbeatService {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &HeartbeatService{
		logger:   logger,
		ws:       ws,
		agent:    agent,
		repo:     repo,
		interval: interval,
	}
}

// Run starts the heartbeat loop. Blocks until ctx is cancelled.
func (h *HeartbeatService) Run(ctx context.Context) error {
	h.logger.Info("heartbeat service started", "interval", h.interval)
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("heartbeat service stopped")
			return nil
		case <-ticker.C:
			h.checkAllProjects(ctx)
		}
	}
}

func (h *HeartbeatService) checkAllProjects(ctx context.Context) {
	projects, err := h.repo.ListProjects(ctx)
	if err != nil {
		h.logger.Error("heartbeat: failed to list projects", "error", err)
		return
	}

	for _, proj := range projects {
		h.processProject(ctx, proj)
	}
}

func (h *HeartbeatService) processProject(ctx context.Context, proj domain.Project) {
	projectPath := h.ws.GetProjectPath(string(proj.ID))
	heartbeatPath := filepath.Join(projectPath, HeartbeatFileName)

	data, err := os.ReadFile(heartbeatPath)
	if err != nil {
		// No HEARTBEAT.md is normal — most projects won't have one
		return
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return
	}

	// Parse checklist items: lines starting with "- [ ]" (unchecked)
	lines := strings.Split(content, "\n")
	var pendingTasks []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") {
			task := strings.TrimPrefix(trimmed, "- [ ]")
			task = strings.TrimSpace(task)
			if task != "" {
				pendingTasks = append(pendingTasks, task)
			}
		}
	}

	if len(pendingTasks) == 0 {
		return
	}

	h.logger.Info("heartbeat: found pending tasks", "project", proj.Name, "count", len(pendingTasks))

	// Execute each pending task via the agent
	for _, task := range pendingTasks {
		convID := domain.ConversationID(fmt.Sprintf("heartbeat-%s-%d", proj.ID, time.Now().Unix()))
		prompt := fmt.Sprintf("Heartbeat task for project '%s': %s", proj.Name, task)

		go func(t string) {
			resp, _, err := h.agent.Chat(ctx, convID, prompt, nil)
			if err != nil {
				h.logger.Error("heartbeat task failed", "project", proj.Name, "task", t, "error", err)
				return
			}

			h.logger.Info("heartbeat task completed", "project", proj.Name, "task", t, "result_len", len(resp.Response))

			// Mark task as done in HEARTBEAT.md
			h.markTaskDone(heartbeatPath, t)
		}(task)
	}
}

// markTaskDone replaces "- [ ] task" with "- [x] task" in the heartbeat file
func (h *HeartbeatService) markTaskDone(path string, task string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	content := string(data)
	unchecked := "- [ ] " + task
	checked := "- [x] " + task
	updated := strings.Replace(content, unchecked, checked, 1)

	if updated != content {
		_ = os.WriteFile(path, []byte(updated), 0644)
	}
}
