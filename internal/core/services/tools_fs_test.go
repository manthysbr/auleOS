package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testWorkspaceManager creates a WorkspaceManager rooted in a temp dir.
func testWorkspaceManager(t *testing.T) (*WorkspaceManager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	ws := &WorkspaceManager{baseDir: tmpDir}
	return ws, tmpDir
}

// testProjectCtx returns a context with a project ID embedded.
func testProjectCtx(projectID string) context.Context {
	return ContextWithProject(context.Background(), domain.ProjectID(projectID))
}

// ── ensurePathIsSafe ────────────────────────────────────────────────────

func TestEnsurePathIsSafe(t *testing.T) {
	root := "/workspace/projects/abc"

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"normal file", "src/main.go", false},
		{"nested", "a/b/c/d.txt", false},
		{"dot current", "./foo.txt", false},
		{"traversal blocked", "../../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ensurePathIsSafe(root, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ── read_file ───────────────────────────────────────────────────────────

func TestReadFileTool(t *testing.T) {
	ws, tmpDir := testWorkspaceManager(t)

	// Setup: create a project dir with a file
	projDir := filepath.Join(tmpDir, "projects", "proj1")
	require.NoError(t, os.MkdirAll(projDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "hello.txt"), []byte("hello world"), 0644))

	tool := NewReadFileTool(ws)

	ctx := testProjectCtx("proj1")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"path":       "hello.txt",
		"project_id": "proj1",
	})
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestReadFileTool_Traversal(t *testing.T) {
	ws, _ := testWorkspaceManager(t)
	tool := NewReadFileTool(ws)

	ctx := testProjectCtx("proj1")
	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":       "../../etc/passwd",
		"project_id": "proj1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "security violation")
}

// ── write_file ──────────────────────────────────────────────────────────

func TestWriteFileTool(t *testing.T) {
	ws, tmpDir := testWorkspaceManager(t)

	projDir := filepath.Join(tmpDir, "projects", "proj1")
	require.NoError(t, os.MkdirAll(projDir, 0755))

	tool := NewWriteFileTool(ws)

	ctx := testProjectCtx("proj1")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"path":       "new_file.txt",
		"content":    "test content 123",
		"project_id": "proj1",
	})
	require.NoError(t, err)
	assert.Contains(t, result.(string), "wrote")

	// Verify file was created
	data, err := os.ReadFile(filepath.Join(projDir, "new_file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "test content 123", string(data))
}

func TestWriteFileTool_CreatesSubdirs(t *testing.T) {
	ws, tmpDir := testWorkspaceManager(t)

	projDir := filepath.Join(tmpDir, "projects", "proj1")
	require.NoError(t, os.MkdirAll(projDir, 0755))

	tool := NewWriteFileTool(ws)

	ctx := testProjectCtx("proj1")
	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":       "sub/dir/file.txt",
		"content":    "nested",
		"project_id": "proj1",
	})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projDir, "sub", "dir", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

// ── list_dir ────────────────────────────────────────────────────────────

func TestListDirTool(t *testing.T) {
	ws, tmpDir := testWorkspaceManager(t)

	projDir := filepath.Join(tmpDir, "projects", "proj1")
	require.NoError(t, os.MkdirAll(filepath.Join(projDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "b.txt"), []byte("b"), 0644))

	tool := NewListDirTool(ws)

	ctx := testProjectCtx("proj1")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"path":       ".",
		"project_id": "proj1",
	})
	require.NoError(t, err)

	listing := result.(string)
	assert.Contains(t, listing, "a.txt")
	assert.Contains(t, listing, "b.txt")
	assert.Contains(t, listing, "subdir")
}

// ── edit_file ───────────────────────────────────────────────────────────

func TestEditFileTool(t *testing.T) {
	ws, tmpDir := testWorkspaceManager(t)

	projDir := filepath.Join(tmpDir, "projects", "proj1")
	require.NoError(t, os.MkdirAll(projDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "code.go"), []byte("func main() {\n\tfmt.Println(\"old\")\n}"), 0644))

	tool := NewEditFileTool(ws)

	ctx := testProjectCtx("proj1")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"path":       "code.go",
		"search":     "fmt.Println(\"old\")",
		"replace":    "fmt.Println(\"new\")",
		"project_id": "proj1",
	})
	require.NoError(t, err)
	assert.Contains(t, result.(string), "edited")

	data, err := os.ReadFile(filepath.Join(projDir, "code.go"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "fmt.Println(\"new\")")
	assert.NotContains(t, string(data), "fmt.Println(\"old\")")
}

func TestEditFileTool_NotFound(t *testing.T) {
	ws, tmpDir := testWorkspaceManager(t)

	projDir := filepath.Join(tmpDir, "projects", "proj1")
	require.NoError(t, os.MkdirAll(projDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "code.go"), []byte("hello world"), 0644))

	tool := NewEditFileTool(ws)

	ctx := testProjectCtx("proj1")
	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":       "code.go",
		"search":     "NONEXISTENT STRING",
		"replace":    "replacement",
		"project_id": "proj1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ── append_file ─────────────────────────────────────────────────────────

func TestAppendFileTool(t *testing.T) {
	ws, tmpDir := testWorkspaceManager(t)

	projDir := filepath.Join(tmpDir, "projects", "proj1")
	require.NoError(t, os.MkdirAll(projDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "log.txt"), []byte("line1\n"), 0644))

	tool := NewAppendFileTool(ws)

	ctx := testProjectCtx("proj1")
	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":       "log.txt",
		"content":    "line2\n",
		"project_id": "proj1",
	})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projDir, "log.txt"))
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\n", string(data))
}

func TestAppendFileTool_CreatesFile(t *testing.T) {
	ws, tmpDir := testWorkspaceManager(t)

	projDir := filepath.Join(tmpDir, "projects", "proj1")
	require.NoError(t, os.MkdirAll(projDir, 0755))

	tool := NewAppendFileTool(ws)

	ctx := testProjectCtx("proj1")
	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":       "new.txt",
		"content":    "created via append",
		"project_id": "proj1",
	})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projDir, "new.txt"))
	require.NoError(t, err)
	assert.Equal(t, "created via append", string(data))
}
