package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// ScheduledTaskRepository defines persistence for scheduled tasks
type ScheduledTaskRepository interface {
	SaveScheduledTask(ctx context.Context, task *domain.ScheduledTask) error
	GetScheduledTask(ctx context.Context, id domain.ScheduledTaskID) (*domain.ScheduledTask, error)
	ListScheduledTasks(ctx context.Context) ([]domain.ScheduledTask, error)
	DeleteScheduledTask(ctx context.Context, id domain.ScheduledTaskID) error
	GetDueTasks(ctx context.Context, now time.Time) ([]domain.ScheduledTask, error)
}

// CronScheduler is a goroutine that checks for due tasks every minute
type CronScheduler struct {
	logger   *slog.Logger
	repo     ScheduledTaskRepository
	agent    *ReActAgentService
	eventBus *EventBus
	tick     time.Duration // check interval (1 minute default)
}

func NewCronScheduler(logger *slog.Logger, repo ScheduledTaskRepository, agent *ReActAgentService, eventBus *EventBus) *CronScheduler {
	return &CronScheduler{
		logger:   logger,
		repo:     repo,
		agent:    agent,
		eventBus: eventBus,
		tick:     1 * time.Minute,
	}
}

// Run starts the scheduler loop. Blocks until ctx is cancelled.
func (s *CronScheduler) Run(ctx context.Context) error {
	s.logger.Info("cron scheduler started", "check_interval", s.tick)
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("cron scheduler stopped")
			return nil
		case <-ticker.C:
			s.checkAndExecute(ctx)
		}
	}
}

func (s *CronScheduler) checkAndExecute(ctx context.Context) {
	now := time.Now()
	tasks, err := s.repo.GetDueTasks(ctx, now)
	if err != nil {
		s.logger.Error("failed to get due tasks", "error", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	s.logger.Info("executing due tasks", "count", len(tasks))

	for _, task := range tasks {
		task := task // capture
		go s.executeTask(ctx, &task)
	}
}

func (s *CronScheduler) executeTask(ctx context.Context, task *domain.ScheduledTask) {
	s.logger.Info("executing scheduled task", "task_id", task.ID, "name", task.Name)

	var result string
	var execErr error

	if task.Command != "" {
		// Direct command execution — bypass LLM entirely (PicoClaw pattern)
		result, execErr = s.executeCommand(ctx, task.Command)
	} else {
		// LLM-based execution via ReAct agent
		convID := domain.ConversationID(fmt.Sprintf("cron-%s-%d", task.ID, time.Now().Unix()))
		resp, _, err := s.agent.Chat(ctx, convID, task.Prompt, task.PersonaID)
		if err != nil {
			execErr = err
		} else {
			result = resp.Response
		}
	}

	now := time.Now()
	task.LastRun = &now
	task.RunCount++

	if execErr != nil {
		task.LastResult = fmt.Sprintf("ERROR: %v", execErr)
		s.logger.Error("scheduled task failed", "task_id", task.ID, "error", execErr)
	} else {
		task.LastResult = result
		// Truncate result for storage
		if len(task.LastResult) > 4096 {
			task.LastResult = task.LastResult[:4096] + "... (truncated)"
		}
		s.logger.Info("scheduled task completed", "task_id", task.ID)
	}

	// Deliver result to user via EventBus if requested
	if task.Deliver && s.eventBus != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"type":      "scheduled_task_result",
			"task_id":   string(task.ID),
			"task_name": task.Name,
			"result":    task.LastResult,
			"status":    string(task.Status),
			"timestamp": now.UnixMilli(),
		})
		s.eventBus.Publish(Event{
			JobID:     BroadcastChannel,
			Type:      EventTypeNewMessage,
			Data:      string(payload),
			Timestamp: now.UnixMilli(),
		})
	}

	// Update next_run based on type
	switch task.Type {
	case domain.TaskTypeOneShot:
		task.Status = domain.TaskStatusCompleted
	case domain.TaskTypeRecurring:
		if task.IntervalSec > 0 {
			task.NextRun = now.Add(time.Duration(task.IntervalSec) * time.Second)
		}
	case domain.TaskTypeCron:
		next, parseErr := nextCronRun(task.CronExpr, now)
		if parseErr != nil {
			s.logger.Error("invalid cron expression", "task_id", task.ID, "expr", task.CronExpr, "error", parseErr)
			task.Status = domain.TaskStatusFailed
			task.LastResult = fmt.Sprintf("Invalid cron expression: %v", parseErr)
		} else {
			task.NextRun = next
		}
	}

	if saveErr := s.repo.SaveScheduledTask(ctx, task); saveErr != nil {
		s.logger.Error("failed to save task after execution", "task_id", task.ID, "error", saveErr)
	}
}

// executeCommand runs a shell command directly and returns stdout.
// Only non-dangerous commands are allowed (reuses exec tool's blocklist).
func (s *CronScheduler) executeCommand(ctx context.Context, command string) (string, error) {
	if isDangerousCommand(command) {
		return "", fmt.Errorf("command blocked by security policy: %s", command)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}
	return string(output), nil
}

// nextCronRun parses a simple cron expression and returns the next run time.
// Supports: "minute hour day month weekday" (standard 5-field cron)
// For simplicity, only handles basic patterns: *, specific number, and intervals (*/N)
func nextCronRun(expr string, from time.Time) (time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("expected 5 fields (min hour day month weekday), got %d", len(fields))
	}

	// Simple approach: scan forward from 'from' minute by minute up to 48 hours
	// This is brute-force but correct for a PoC. Production would use a proper cron lib.
	candidate := from.Truncate(time.Minute).Add(time.Minute)
	limit := from.Add(48 * time.Hour)

	for candidate.Before(limit) {
		if matchesCronField(fields[0], candidate.Minute()) &&
			matchesCronField(fields[1], candidate.Hour()) &&
			matchesCronField(fields[2], candidate.Day()) &&
			matchesCronField(fields[3], int(candidate.Month())) &&
			matchesCronField(fields[4], int(candidate.Weekday())) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching time found within 48 hours for expression: %s", expr)
}

// matchesCronField checks if a value matches a cron field pattern
func matchesCronField(pattern string, value int) bool {
	if pattern == "*" {
		return true
	}

	// Handle */N (every N)
	if strings.HasPrefix(pattern, "*/") {
		n := 0
		if _, err := fmt.Sscanf(pattern, "*/%d", &n); err == nil && n > 0 {
			return value%n == 0
		}
		return false
	}

	// Handle comma-separated list (check BEFORE single number)
	if strings.Contains(pattern, ",") {
		for _, part := range strings.Split(pattern, ",") {
			pn := 0
			if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &pn); err == nil && pn == value {
				return true
			}
		}
		return false
	}

	// Handle specific number
	n := 0
	if _, err := fmt.Sscanf(pattern, "%d", &n); err == nil {
		return value == n
	}

	return false
}
