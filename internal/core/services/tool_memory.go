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
				}
			}

			// Resolve Path
			var projectPath string
			if projectID != "" {
				projectPath = ws.GetProjectPath(projectID)
			} else {
				home, _ := os.UserHomeDir()
				projectPath = filepath.Join(home, ".aule", "global")
				_ = os.MkdirAll(projectPath, 0755)
			}
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
				}
			}

			// Resolve Path
			var projectPath string
			if projectID != "" {
				projectPath = ws.GetProjectPath(projectID)
			} else {
				home, _ := os.UserHomeDir()
				projectPath = filepath.Join(home, ".aule", "global")
				_ = os.MkdirAll(projectPath, 0755)
			}
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

// NewMemorySearchTool returns a tool that searches the project's long-term memory by keyword.
func NewMemorySearchTool(ws *WorkspaceManager) *domain.Tool {
	return &domain.Tool{
		Name:        "memory_search",
		Description: "Searches the project's long-term memory for entries matching a keyword or phrase. Returns matching lines from MEMORY.md. Use this to find specific preferences, decisions, or facts.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Keyword or phrase to search for (case-insensitive).",
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Optional: filter by category (preference, decision, fact, context).",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "The project ID. If not provided, inferred from context.",
				},
			},
			Required: []string{"query"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			query, _ := params["query"].(string)
			category, _ := params["category"].(string)
			projectID, _ := params["project_id"].(string)

			if query == "" {
				return nil, fmt.Errorf("query is required")
			}

			if projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
			}

			var projectPath string
			if projectID != "" {
				projectPath = ws.GetProjectPath(projectID)
			} else {
				home, _ := os.UserHomeDir()
				projectPath = filepath.Join(home, ".aule", "global")
				_ = os.MkdirAll(projectPath, 0755)
			}
			memoryPath := filepath.Join(projectPath, MemoryFileName)

			data, err := os.ReadFile(memoryPath)
			if err != nil {
				if os.IsNotExist(err) {
					return "No memories found (memory is empty).", nil
				}
				return nil, fmt.Errorf("failed to read memory: %w", err)
			}

			queryLower := strings.ToLower(query)
			categoryUpper := strings.ToUpper(category)

			lines := strings.Split(string(data), "\n")
			var matches []string

			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}

				// Category filter
				if category != "" && !strings.Contains(trimmed, "**"+categoryUpper+"**") {
					continue
				}

				// Keyword match (case-insensitive)
				if strings.Contains(strings.ToLower(trimmed), queryLower) {
					matches = append(matches, trimmed)
				}
			}

			if len(matches) == 0 {
				return fmt.Sprintf("No memories matching '%s' found.", query), nil
			}

			return fmt.Sprintf("Found %d matching memories:\n%s", len(matches), strings.Join(matches, "\n")), nil
		},
	}
}
