package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDangerousCommand(t *testing.T) {
	dangerous := []string{
		"rm -rf /",
		"rm -rf /*",
		"sudo mkfs.ext4 /dev/sda",
		"dd if=/dev/zero of=/dev/sda",
		"shutdown -h now",
		"reboot",
		":(){ :|:& };:",
		"mv / /tmp",
		"chmod -R 777 /",
	}

	for _, cmd := range dangerous {
		t.Run("blocked_"+cmd[:min(len(cmd), 20)], func(t *testing.T) {
			assert.True(t, isDangerousCommand(cmd), "should block: %s", cmd)
		})
	}

	safe := []string{
		"ls -la",
		"cat file.txt",
		"npm install",
		"python main.py",
		"go build ./...",
		"grep -r pattern .",
		"git status",
		"echo hello",
		"mkdir -p src/components",
		"rm temp.txt",    // single file rm is fine
		"rm -rf ./build", // relative rm is fine (no root)
	}

	for _, cmd := range safe {
		t.Run("allowed_"+cmd[:min(len(cmd), 20)], func(t *testing.T) {
			assert.False(t, isDangerousCommand(cmd), "should allow: %s", cmd)
		})
	}
}

func TestExecTool_BlockedCommand(t *testing.T) {
	ws, _ := testWorkspaceManager(t)
	tool := NewExecTool(ws)

	ctx := testProjectCtx("proj1")
	_, err := tool.Execute(ctx, map[string]interface{}{
		"command":    "rm -rf /",
		"project_id": "proj1",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

func TestExecTool_RunsSimpleCommand(t *testing.T) {
	ws, _ := testWorkspaceManager(t)
	tool := NewExecTool(ws)

	ctx := testProjectCtx("proj1")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"command":    "echo hello_world",
		"project_id": "proj1",
	})
	assert.NoError(t, err)
	assert.Contains(t, result.(string), "hello_world")
}

func TestExecTool_RequiresCommand(t *testing.T) {
	ws, _ := testWorkspaceManager(t)
	tool := NewExecTool(ws)

	ctx := testProjectCtx("proj1")
	_, err := tool.Execute(ctx, map[string]interface{}{
		"command":    "",
		"project_id": "proj1",
	})
	assert.Error(t, err)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
