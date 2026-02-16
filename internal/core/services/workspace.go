package services

import (
	"fmt"
	"os"
	"path/filepath"
)

type WorkspaceManager struct {
	baseDir string
}

func NewWorkspaceManager() *WorkspaceManager {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to tmp if home fails (unlikely)
		return &WorkspaceManager{baseDir: "/tmp/aule/workspaces"}
	}
	return &WorkspaceManager{
		baseDir: filepath.Join(home, ".aule", "workspaces"),
	}
}

// PrepareWorkspace creates the directory structure for a job/worker
// It ensures /mnt/aule/workspace/{id} exists and is writable
func (s *WorkspaceManager) PrepareWorkspace(id string) (string, error) {
	path := filepath.Join(s.baseDir, id)
	
	// Create with broad permissions so the container user 'aule' can write to it
	// In a production env, we'd be more specific with ownership (chown), but for now 0777 works for shared volume.
	if err := os.MkdirAll(path, 0777); err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}
	
	// Explicit chmod ensuring permissions apply if directory already existed with different perms
	if err := os.Chmod(path, 0777); err != nil {
		return "", fmt.Errorf("failed to chmod workspace: %w", err)
	}

	return path, nil
}

// CleanupWorkspace removes the workspace directory
func (s *WorkspaceManager) CleanupWorkspace(id string) error {
	path := filepath.Join(s.baseDir, id)
	return os.RemoveAll(path)
}

// GetPath returns the absolute path for a job's workspace
func (s *WorkspaceManager) GetPath(id string) string {
	return filepath.Join(s.baseDir, id)
}
