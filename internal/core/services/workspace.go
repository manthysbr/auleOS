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
	baseDir := os.Getenv("AULE_WORKSPACE_DIR")
	if baseDir == "" {
		baseDir = "/home/gohan/auleOS/workspace"
	}
	return &WorkspaceManager{
		baseDir: baseDir,
	}
}

// PrepareWorkspace creates the directory structure for a job/worker (ephemeral)
// Path: baseDir/jobs/{id}
func (s *WorkspaceManager) PrepareWorkspace(id string) (string, error) {
	path := filepath.Join(s.baseDir, "jobs", id)
	return s.ensureDir(path)
}

// PrepareProject creates the directory structure for a persistent project
// Path: baseDir/projects/{id}
func (s *WorkspaceManager) PrepareProject(id string) (string, error) {
	path := filepath.Join(s.baseDir, "projects", id)
	return s.ensureDir(path)
}

func (s *WorkspaceManager) ensureDir(path string) (string, error) {
	if err := os.MkdirAll(path, 0777); err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}
	if err := os.Chmod(path, 0777); err != nil {
		return "", fmt.Errorf("failed to chmod workspace: %w", err)
	}
	return path, nil
}

// CleanupWorkspace removes the job workspace directory
func (s *WorkspaceManager) CleanupWorkspace(id string) error {
	path := filepath.Join(s.baseDir, "jobs", id)
	return os.RemoveAll(path)
}

// GetPath returns the absolute path for a job's workspace
func (s *WorkspaceManager) GetPath(id string) string {
	return filepath.Join(s.baseDir, "jobs", id)
}

// GetProjectPath returns the absolute path for a project's workspace
// If path doesn't exist, it creates it (lazy init for default project)
func (s *WorkspaceManager) GetProjectPath(id string) string {
	path := filepath.Join(s.baseDir, "projects", id)
	// Build it just in case
	_ = os.MkdirAll(path, 0777) 
	return path
}
