package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/manthysbr/auleOS/internal/core/domain"
)

// NewScheduleTaskTool creates a tool that allows the agent to schedule tasks
func NewScheduleTaskTool(repo ScheduledTaskRepository) *domain.Tool {
	return &domain.Tool{
		Name:        "schedule_task",
		Description: "Schedules a task to be executed later. Supports one-shot ('in 10 minutes'), recurring ('every 2 hours'), and cron expressions ('0 9 * * *'). Tasks can run through the ReAct agent (prompt) or execute a command directly.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Short name for the task (e.g., 'Check server status').",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "The instruction to execute via ReAct agent when the task runs. Required if command is not set.",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Optional: shell command to execute directly (bypasses LLM). Mutually exclusive with prompt for the execution path.",
				},
				"deliver": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, the task result is sent to the user via the broadcast channel. Default: false.",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Task type: 'one_shot' (run once), 'recurring' (repeat), or 'cron' (cron expression).",
					"enum":        []string{"one_shot", "recurring", "cron"},
				},
				"delay_minutes": map[string]interface{}{
					"type":        "number",
					"description": "For one_shot: minutes from now to execute. For recurring: interval in minutes.",
				},
				"cron_expr": map[string]interface{}{
					"type":        "string",
					"description": "For cron type: standard 5-field cron expression (e.g., '0 9 * * *' = every day at 9am).",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "Project ID to associate the task with.",
				},
				"persona_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional: persona to use when executing the task.",
				},
			},
			Required: []string{"name", "type"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			name, _ := params["name"].(string)
			prompt, _ := params["prompt"].(string)
			command, _ := params["command"].(string)
			taskType, _ := params["type"].(string)

			if name == "" || taskType == "" {
				return nil, fmt.Errorf("name and type are required")
			}
			if prompt == "" && command == "" {
				return nil, fmt.Errorf("either prompt or command is required")
			}

			deliver := false
			if d, ok := params["deliver"].(bool); ok {
				deliver = d
			}

			projectID, _ := params["project_id"].(string)
			if projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
			}

			task := &domain.ScheduledTask{
				ID:        domain.ScheduledTaskID(uuid.New().String()),
				ProjectID: domain.ProjectID(projectID),
				Name:      name,
				Prompt:    prompt,
				Command:   command,
				Deliver:   deliver,
				Status:    domain.TaskStatusActive,
				CreatedAt: time.Now(),
				CreatedBy: "agent",
			}

			// Parse persona_id
			if pid, ok := params["persona_id"].(string); ok && pid != "" {
				personaID := domain.PersonaID(pid)
				task.PersonaID = &personaID
			}

			now := time.Now()

			switch domain.ScheduledTaskType(taskType) {
			case domain.TaskTypeOneShot:
				delayMin := 10.0 // default 10 minutes
				if d, ok := params["delay_minutes"].(float64); ok && d > 0 {
					delayMin = d
				}
				task.Type = domain.TaskTypeOneShot
				task.NextRun = now.Add(time.Duration(delayMin) * time.Minute)

			case domain.TaskTypeRecurring:
				intervalMin := 60.0 // default 1 hour
				if d, ok := params["delay_minutes"].(float64); ok && d > 0 {
					intervalMin = d
				}
				task.Type = domain.TaskTypeRecurring
				task.IntervalSec = int(intervalMin * 60)
				task.NextRun = now.Add(time.Duration(intervalMin) * time.Minute)

			case domain.TaskTypeCron:
				cronExpr, _ := params["cron_expr"].(string)
				cronExpr = strings.TrimSpace(cronExpr)
				if cronExpr == "" {
					return nil, fmt.Errorf("cron_expr is required for cron type")
				}
				// Validate the expression
				nextRun, err := nextCronRun(cronExpr, now)
				if err != nil {
					return nil, fmt.Errorf("invalid cron expression '%s': %w", cronExpr, err)
				}
				task.Type = domain.TaskTypeCron
				task.CronExpr = cronExpr
				task.NextRun = nextRun

			default:
				return nil, fmt.Errorf("invalid task type: %s (use one_shot, recurring, or cron)", taskType)
			}

			if err := repo.SaveScheduledTask(ctx, task); err != nil {
				return nil, fmt.Errorf("failed to save scheduled task: %w", err)
			}

			return fmt.Sprintf("Task '%s' scheduled (ID: %s). Next run: %s", name, task.ID, task.NextRun.Format("2006-01-02 15:04:05")), nil
		},
	}
}

// NewListScheduledTasksTool creates a tool to list scheduled tasks
func NewListScheduledTasksTool(repo ScheduledTaskRepository) *domain.Tool {
	return &domain.Tool{
		Name:        "list_scheduled_tasks",
		Description: "Lists all scheduled tasks with their status and next run time.",
		Parameters: domain.ToolParameters{
			Type:       "object",
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			tasks, err := repo.ListScheduledTasks(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list tasks: %w", err)
			}

			if len(tasks) == 0 {
				return "No scheduled tasks.", nil
			}

			var lines []string
			for _, t := range tasks {
				nextRun := t.NextRun.Format("2006-01-02 15:04")
				lastRun := "never"
				if t.LastRun != nil {
					lastRun = t.LastRun.Format("2006-01-02 15:04")
				}
				lines = append(lines, fmt.Sprintf("- %s (ID: %s) [%s] type=%s next=%s last=%s runs=%d",
					t.Name, t.ID, t.Status, t.Type, nextRun, lastRun, t.RunCount))
			}
			return fmt.Sprintf("%d tasks:\n%s", len(tasks), strings.Join(lines, "\n")), nil
		},
	}
}

// NewCancelScheduledTaskTool creates a tool to cancel/delete a scheduled task
func NewCancelScheduledTaskTool(repo ScheduledTaskRepository) *domain.Tool {
	return &domain.Tool{
		Name:        "cancel_scheduled_task",
		Description: "Cancels and deletes a scheduled task by ID.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"task_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the scheduled task to cancel.",
				},
			},
			Required: []string{"task_id"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			taskID, _ := params["task_id"].(string)
			if taskID == "" {
				return nil, fmt.Errorf("task_id is required")
			}

			if err := repo.DeleteScheduledTask(ctx, domain.ScheduledTaskID(taskID)); err != nil {
				return nil, fmt.Errorf("failed to cancel task: %w", err)
			}

			return fmt.Sprintf("Task %s cancelled.", taskID), nil
		},
	}
}

// NewToggleScheduledTaskTool creates a tool to enable/disable (pause/resume) a scheduled task
func NewToggleScheduledTaskTool(repo ScheduledTaskRepository) *domain.Tool {
	return &domain.Tool{
		Name:        "toggle_scheduled_task",
		Description: "Enable or disable a scheduled task. Paused tasks are skipped by the scheduler. Resuming recalculates the next run time.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"task_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the scheduled task to toggle.",
				},
				"enable": map[string]interface{}{
					"type":        "boolean",
					"description": "true to enable (resume), false to disable (pause).",
				},
			},
			Required: []string{"task_id", "enable"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			taskID, _ := params["task_id"].(string)
			if taskID == "" {
				return nil, fmt.Errorf("task_id is required")
			}

			enable, ok := params["enable"].(bool)
			if !ok {
				return nil, fmt.Errorf("enable (bool) is required")
			}

			task, err := repo.GetScheduledTask(ctx, domain.ScheduledTaskID(taskID))
			if err != nil {
				return nil, fmt.Errorf("task not found: %w", err)
			}

			if enable {
				task.Status = domain.TaskStatusActive
				// Recalculate next_run from now
				now := time.Now()
				switch task.Type {
				case domain.TaskTypeRecurring:
					if task.IntervalSec > 0 {
						task.NextRun = now.Add(time.Duration(task.IntervalSec) * time.Second)
					}
				case domain.TaskTypeCron:
					if next, err := nextCronRun(task.CronExpr, now); err == nil {
						task.NextRun = next
					}
				case domain.TaskTypeOneShot:
					task.NextRun = now.Add(10 * time.Minute) // default 10 min from resume
				}
			} else {
				task.Status = domain.TaskStatusPaused
			}

			if err := repo.SaveScheduledTask(ctx, task); err != nil {
				return nil, fmt.Errorf("failed to update task: %w", err)
			}

			action := "paused"
			if enable {
				action = "resumed"
			}
			return fmt.Sprintf("Task '%s' %s. Status: %s", task.Name, action, task.Status), nil
		},
	}
}
