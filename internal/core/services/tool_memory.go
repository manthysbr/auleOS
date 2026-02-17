package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

const MemoryFileName = "MEMORY.md"

// NewMemorySaveTool returns a tool that saves a fact/memory to the project's long-term memory.
func NewMemorySaveTool(ws *WorkspaceManager) *domain.Tool {
	return &domain.Tool{
		Name:        "memory_save",
		Description: "Saves a significant fact, preference, or decision to the project's long-term memory. Use this when the user mentions something important that should be remembered for future conversations (e.g., architectural choices, preferred language, deployment details).",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Category of the memory: 'preference', 'decision', 'fact', 'context'",
					"enum":        []string{"preference", "decision", "fact", "context"},
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The concise content to remember.",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "The project ID to associate this memory with. If not provided, tries to infer from context.",
				},
			},
			Required: []string{"category", "content"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			category, _ := params["category"].(string)
			content, _ := params["content"].(string)
			projectID, _ := params["project_id"].(string)

			if category == "" || content == "" {
				return nil, fmt.Errorf("category and content are required")
			}

			// Resolve Project ID
			if projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				} else {
					return nil, fmt.Errorf("project_id required (memory is project-scoped)")
				}
			}

			// Resolve Path
			projectPath := ws.GetProjectPath(projectID)
			memoryPath := filepath.Join(projectPath, MemoryFileName)

			// Format Entry
			timestamp := time.Now().Format("2006-01-02")
			entry := fmt.Sprintf("- [%s] **%s**: %s\n", timestamp, strings.ToUpper(category), content)

			// Append to file
			f, err := os.OpenFile(memoryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to open memory file: %w", err)
			}
			defer f.Close()

			if _, err := f.WriteString(entry); err != nil {
				return nil, fmt.Errorf("failed to write to memory: %w", err)
			}

			return fmt.Sprintf("Memory saved to %s", MemoryFileName), nil
		},
	}
}

// NewMemoryReadTool returns a tool that reads the project's long-term memory.
func NewMemoryReadTool(ws *WorkspaceManager) *domain.Tool {
	return &domain.Tool{
		Name:        "memory_read",
		Description: "Reads the project's long-term memory. Use this to recall past decisions, user preferences, or project context.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "The project ID. If not provided, tries to infer from context.",
				},
			},
			Required: []string{},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			projectID, _ := params["project_id"].(string)

			// Resolve Project ID
			if projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				} else {
					return nil, fmt.Errorf("project_id required")
				}
			}

			// Resolve Path
			projectPath := ws.GetProjectPath(projectID)
			memoryPath := filepath.Join(projectPath, MemoryFileName)

			// Read file
			data, err := os.ReadFile(memoryPath)
			if err != nil {
				if os.IsNotExist(err) {
					return "Memory is empty.", nil
				}
				return nil, fmt.Errorf("failed to read memory: %w", err)
			}

			return string(data), nil
		},
	}
}
