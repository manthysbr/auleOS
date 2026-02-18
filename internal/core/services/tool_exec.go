package services

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// dangerousCommands is a blocklist of commands that could damage the host system.
var dangerousCommands = []string{
	"rm -rf /",
	"rm -rf /*",
	"mkfs",
	"dd if=",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"init 0",
	"init 6",
	":(){ :|:& };:", // fork bomb
	"format c:",
	"> /dev/sda",
	"mv / ",
	"chmod -R 777 /",
	"chown -R ",
}

// isDangerousCommand checks if a command matches the blocklist.
func isDangerousCommand(cmd string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	for _, dangerous := range dangerousCommands {
		if strings.Contains(lower, strings.ToLower(dangerous)) {
			return true
		}
	}
	return false
}

// NewExecTool creates the exec tool — local sandboxed execution using os/exec.
// Commands run inside the project workspace directory with a 30s timeout.
func NewExecTool(ws *WorkspaceManager) *domain.Tool {
	return &domain.Tool{
		Name:          "exec",
		Description:   "Executes a shell command inside the project workspace. Sandboxed to the project directory with a 30-second timeout. Use for npm install, ls, cat, grep, git, python, etc.",
		ExecutionType: domain.ExecNative,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute (e.g., 'ls -la', 'npm install', 'python main.py').",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the project/workspace. If not provided, inferred from context.",
				},
				"timeout_seconds": map[string]interface{}{
					"type":        "number",
					"description": "Optional timeout in seconds (default: 30, max: 120).",
				},
			},
			Required: []string{"command"},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			command, ok := params["command"].(string)
			if !ok || strings.TrimSpace(command) == "" {
				return nil, fmt.Errorf("command is required and must be a non-empty string")
			}

			projectID, _ := params["project_id"].(string)
			if projectID == "" {
				if pID, found := GetProjectFromContext(ctx); found {
					projectID = string(pID)
				}
				// No project — will use home directory as fallback
			}

			// Parse timeout
			timeoutSec := 30.0
			if t, ok := params["timeout_seconds"].(float64); ok && t > 0 {
				timeoutSec = t
			}
			if timeoutSec > 120 {
				timeoutSec = 120 // Hard cap
			}

			// Security: check blocklist
			if isDangerousCommand(command) {
				return nil, fmt.Errorf("command blocked: matches dangerous command blocklist")
			}

			// Resolve workspace directory
			var workDir string
			if projectID != "" {
				workDir = ws.GetProjectPath(projectID)
			} else {
				// Fallback to home directory when no project context
				workDir, _ = os.UserHomeDir()
				if workDir == "" {
					workDir = "/tmp"
				}
			}

			// Create context with timeout
			execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
			defer cancel()

			// Execute command in workspace directory
			cmd := exec.CommandContext(execCtx, "/bin/sh", "-c", command)
			cmd.Dir = workDir

			// Clean environment — only pass safe vars
			cmd.Env = []string{
				fmt.Sprintf("HOME=%s", workDir),
				fmt.Sprintf("PWD=%s", workDir),
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"LANG=en_US.UTF-8",
				"TERM=xterm",
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			// Build result
			result := strings.Builder{}
			if stdout.Len() > 0 {
				outStr := stdout.String()
				// Truncate excessive output
				if len(outStr) > 8192 {
					outStr = outStr[:8192] + "\n... (output truncated at 8KB)"
				}
				result.WriteString(outStr)
			}
			if stderr.Len() > 0 {
				errStr := stderr.String()
				if len(errStr) > 4096 {
					errStr = errStr[:4096] + "\n... (stderr truncated at 4KB)"
				}
				if result.Len() > 0 {
					result.WriteString("\n")
				}
				result.WriteString("STDERR: ")
				result.WriteString(errStr)
			}

			if err != nil {
				if execCtx.Err() == context.DeadlineExceeded {
					return nil, fmt.Errorf("command timed out after %.0fs", timeoutSec)
				}
				// Command failed but produced output — return output + error
				if result.Len() > 0 {
					return nil, fmt.Errorf("command failed (exit %v):\n%s", err, result.String())
				}
				return nil, fmt.Errorf("command failed: %v", err)
			}

			if result.Len() == 0 {
				return "(command completed with no output)", nil
			}
			return result.String(), nil
		},
	}
}
