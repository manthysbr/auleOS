package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// ensurePathIsSafe strictly validates that the requested path is within the workspace root.
func ensurePathIsSafe(root, requestedPath string) (string, error) {
	fullPath := filepath.Join(root, requestedPath)
	cleanPath := filepath.Clean(fullPath)

	if !strings.HasPrefix(cleanPath, filepath.Clean(root)) {
		return "", fmt.Errorf("security violation: path %q is outside workspace root", requestedPath)
	}
	return cleanPath, nil
}

// NewReadFileTool creates the read_file tool
func NewReadFileTool(ws *WorkspaceManager) *domain.Tool {
	return &domain.Tool{
		Name:        "read_file",
		Description: "Reads the content of a file within the workspace. Returns text content.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path to the file (e.g., 'src/main.go').",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the project/workspace.",
				},
			},
			Required: []string{"path", "project_id"},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, ok := params["path"].(string)
			if !ok {
				return nil, fmt.Errorf("path must be a string")
			}
			projectID, ok := params["project_id"].(string)
			if !ok || projectID == "" {
				// Try context
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				} else {
					return nil, fmt.Errorf("project_id must be provided (or context missing)")
				}
			}

			// 1. Resolve Workspace Root
			root := ws.GetProjectPath(projectID)

			// 2. Security Check
			safePath, err := ensurePathIsSafe(root, path)
			if err != nil {
				return nil, err
			}

			// 3. Read File
			content, err := os.ReadFile(safePath)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("file not found: %s", path)
				}
				return nil, fmt.Errorf("failed to read file: %w", err)
			}

			return string(content), nil
		},
	}
}

// NewWriteFileTool creates the write_file tool
func NewWriteFileTool(ws *WorkspaceManager) *domain.Tool {
	return &domain.Tool{
		Name:        "write_file",
		Description: "Writes content to a file. Overwrites if exists, creates directories if needed.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path to the file.",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Text content to write.",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the project/workspace.",
				},
			},
			Required: []string{"path", "content", "project_id"},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, ok := params["path"].(string)
			if !ok {
				return nil, fmt.Errorf("path must be a string")
			}
			content, ok := params["content"].(string)
			if !ok {
				return nil, fmt.Errorf("content must be a string")
			}
			projectID, ok := params["project_id"].(string)
			if !ok || projectID == "" {
				// Try context
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				} else {
					return nil, fmt.Errorf("project_id must be provided (or context missing)")
				}
			}

			root := ws.GetPath(projectID)
			safePath, err := ensurePathIsSafe(root, path)
			if err != nil {
				return nil, err
			}

			// Ensure parent dir exists
			if err := os.MkdirAll(filepath.Dir(safePath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create directories: %w", err)
			}

			if err := os.WriteFile(safePath, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("failed to write file: %w", err)
			}

			return fmt.Sprintf("Successfully wrote to %s (%d bytes)", path, len(content)), nil
		},
	}
}

// NewListDirTool creates the list_dir tool
func NewListDirTool(ws *WorkspaceManager) *domain.Tool {
	return &domain.Tool{
		Name:        "list_dir",
		Description: "Lists files and directories in a path.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path to list (default: root).",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the project/workspace.",
				},
			},
			Required: []string{"project_id"},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, _ := params["path"].(string) // Optional
			projectID, ok := params["project_id"].(string)
			if !ok || projectID == "" {
				// Try context
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				} else {
					return nil, fmt.Errorf("project_id must be provided (or context missing)")
				}
			}

			root := ws.GetPath(projectID)
			targetPath := path
			if targetPath == "" {
				targetPath = "."
			}

			safePath, err := ensurePathIsSafe(root, targetPath)
			if err != nil {
				return nil, err
			}

			entries, err := os.ReadDir(safePath)
			if err != nil {
				return nil, fmt.Errorf("failed to list directory: %w", err)
			}

			var results []string
			for _, e := range entries {
				suffix := ""
				if e.IsDir() {
					suffix = "/"
				}
				results = append(results, e.Name()+suffix)
			}

			if len(results) == 0 {
				return "(empty directory)", nil
			}
			return strings.Join(results, "\n"), nil
		},
	}
}
