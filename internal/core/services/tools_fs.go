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
			Required: []string{"path"},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, ok := params["path"].(string)
			if !ok {
				return nil, fmt.Errorf("path must be a string")
			}
			projectID, _ := params["project_id"].(string)
			if projectID == "" {
				// Try context
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
			}

			// 1. Resolve Workspace Root
			var root string
			if projectID != "" {
				root = ws.GetProjectPath(projectID)
			} else {
				root, _ = os.UserHomeDir()
				if root == "" {
					root = "/tmp"
				}
			}

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
			Required: []string{"path", "content"},
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
			projectID, _ := params["project_id"].(string)
			if projectID == "" {
				// Try context
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
			}

			var root string
			if projectID != "" {
				root = ws.GetProjectPath(projectID)
			} else {
				root, _ = os.UserHomeDir()
				if root == "" {
					root = "/tmp"
				}
			}
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

			// Return project_id in response so the agent can use it in subsequent read_file calls
			if projectID != "" {
				return fmt.Sprintf("Written to %s (%d bytes) @ path %s | project_id: %s", path, len(content), safePath, projectID), nil
			}
			return fmt.Sprintf("Written to %s (%d bytes) @ path %s", path, len(content), safePath), nil
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
			Required: []string{},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, _ := params["path"].(string) // Optional
			projectID, _ := params["project_id"].(string)
			if projectID == "" {
				// Try context
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
			}

			var root string
			if projectID != "" {
				root = ws.GetProjectPath(projectID)
			} else {
				root, _ = os.UserHomeDir()
				if root == "" {
					root = "/tmp"
				}
			}
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

// NewEditFileTool creates the edit_file tool (search & replace)
func NewEditFileTool(ws *WorkspaceManager) *domain.Tool {
	return &domain.Tool{
		Name:        "edit_file",
		Description: "Performs a search-and-replace edit in an existing file. Finds the exact 'search' string and replaces it with the 'replace' string. Only replaces the first occurrence.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path to the file to edit.",
				},
				"search": map[string]interface{}{
					"type":        "string",
					"description": "The exact text to find in the file.",
				},
				"replace": map[string]interface{}{
					"type":        "string",
					"description": "The text to replace the search string with.",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the project/workspace.",
				},
			},
			Required: []string{"path", "search", "replace"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, _ := params["path"].(string)
			search, _ := params["search"].(string)
			replace, _ := params["replace"].(string)
			projectID, _ := params["project_id"].(string)

			if path == "" || search == "" {
				return nil, fmt.Errorf("path and search are required")
			}

			if projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
			}

			var root string
			if projectID != "" {
				root = ws.GetProjectPath(projectID)
			} else {
				root, _ = os.UserHomeDir()
				if root == "" {
					root = "/tmp"
				}
			}
			safePath, err := ensurePathIsSafe(root, path)
			if err != nil {
				return nil, err
			}

			content, err := os.ReadFile(safePath)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("file not found: %s", path)
				}
				return nil, fmt.Errorf("failed to read file: %w", err)
			}

			original := string(content)
			if !strings.Contains(original, search) {
				return nil, fmt.Errorf("search string not found in file %s", path)
			}

			// Replace first occurrence only
			edited := strings.Replace(original, search, replace, 1)

			if err := os.WriteFile(safePath, []byte(edited), 0644); err != nil {
				return nil, fmt.Errorf("failed to write edited file: %w", err)
			}

			return fmt.Sprintf("Successfully edited %s (replaced %d bytes with %d bytes)", path, len(search), len(replace)), nil
		},
	}
}

// NewAppendFileTool creates the append_file tool
func NewAppendFileTool(ws *WorkspaceManager) *domain.Tool {
	return &domain.Tool{
		Name:        "append_file",
		Description: "Appends content to the end of a file. Creates the file if it doesn't exist.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path to the file.",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Text content to append.",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the project/workspace.",
				},
			},
			Required: []string{"path", "content"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			path, _ := params["path"].(string)
			content, _ := params["content"].(string)
			projectID, _ := params["project_id"].(string)

			if path == "" || content == "" {
				return nil, fmt.Errorf("path and content are required")
			}

			if projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
			}

			var root string
			if projectID != "" {
				root = ws.GetProjectPath(projectID)
			} else {
				root, _ = os.UserHomeDir()
				if root == "" {
					root = "/tmp"
				}
			}
			safePath, err := ensurePathIsSafe(root, path)
			if err != nil {
				return nil, err
			}

			// Ensure parent dir exists
			if err := os.MkdirAll(filepath.Dir(safePath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create directories: %w", err)
			}

			f, err := os.OpenFile(safePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			if _, err := f.WriteString(content); err != nil {
				return nil, fmt.Errorf("failed to append: %w", err)
			}

			return fmt.Sprintf("Appended %d bytes to %s", len(content), path), nil
		},
	}
}
