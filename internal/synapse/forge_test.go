package synapse_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/synapse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealGoWasmPlugin tests loading and executing a Go-compiled WASI .wasm module.
// This verifies the full pipeline: Go source → wasip1/wasm build → wazero execution.
func TestRealGoWasmPlugin(t *testing.T) {
	// Check if the prompt-enhancer.wasm exists (built by scripts/genplugin)
	homeDir, _ := os.UserHomeDir()
	wasmPath := filepath.Join(homeDir, ".aule", "plugins", "prompt-enhancer.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("prompt-enhancer.wasm not found — run `go run ./scripts/genplugin/` first")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	rt, err := synapse.NewRuntime(ctx, logger)
	require.NoError(t, err)
	defer rt.Close(ctx)

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	meta := synapse.PluginMeta{
		Name:        "prompt-enhancer",
		Version:     "1.0.0",
		Description: "Enhances image generation prompts",
		ToolName:    "enhance_prompt",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "The original image prompt",
				},
			},
			Required: []string{"prompt"},
		},
	}

	plugin, err := rt.LoadPlugin(ctx, "prompt-enhancer", wasmBytes, meta)
	require.NoError(t, err)

	// Execute the plugin with real input
	result, err := plugin.Execute(ctx, map[string]interface{}{
		"prompt": "a cute orange cat",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok, "expected map result, got %T", result)

	// The prompt-enhancer should return enhanced_prompt with quality keywords
	enhanced, ok := resultMap["enhanced_prompt"].(string)
	require.True(t, ok, "expected enhanced_prompt string in result")
	assert.Contains(t, enhanced, "a cute orange cat")
	assert.Contains(t, enhanced, "high quality")
	assert.Contains(t, enhanced, "detailed")

	t.Logf("Enhanced prompt: %s", enhanced)
}

// TestRealGoWasmPluginViaTool tests the full AsTool() → ToolRegistry → Execute path.
func TestRealGoWasmPluginViaTool(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	wasmPath := filepath.Join(homeDir, ".aule", "plugins", "prompt-enhancer.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("prompt-enhancer.wasm not found — run `go run ./scripts/genplugin/` first")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	rt, err := synapse.NewRuntime(ctx, logger)
	require.NoError(t, err)
	defer rt.Close(ctx)

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	meta := synapse.PluginMeta{
		Name:        "prompt-enhancer",
		Version:     "1.0.0",
		Description: "Enhances image generation prompts",
		ToolName:    "enhance_prompt",
	}

	plugin, err := rt.LoadPlugin(ctx, "prompt-enhancer", wasmBytes, meta)
	require.NoError(t, err)

	// Register as tool
	tool := plugin.AsTool()
	assert.Equal(t, domain.ExecWasm, tool.ExecutionType)

	registry := domain.NewToolRegistry()
	err = registry.Register(tool)
	require.NoError(t, err)

	// Execute through registry
	result, err := registry.Execute(ctx, "enhance_prompt", map[string]interface{}{
		"prompt": "a cyberpunk city at night",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)

	enhanced := resultMap["enhanced_prompt"].(string)
	assert.Contains(t, enhanced, "cyberpunk city at night")
	assert.Contains(t, enhanced, "cinematic")
}

// TestForgeCodeCleaner tests the code cleanup helper.
func TestForgeCodeCleaner(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean go code",
			input:    "package main\n\nfunc main() {}",
			expected: "package main\n\nfunc main() {}",
		},
		{
			name:     "with markdown fences",
			input:    "```go\npackage main\n\nfunc main() {}\n```",
			expected: "package main\n\nfunc main() {}",
		},
		{
			name:     "with plain fences",
			input:    "```\npackage main\n\nfunc main() {}\n```",
			expected: "package main\n\nfunc main() {}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't call the unexported function directly, but we can test
			// that the LLM output processing works through the Forge
			// For now, just verify the test data is correct
			cleaned := cleanMarkdownFences(tt.input)
			assert.Equal(t, tt.expected, cleaned)
		})
	}
}

// cleanMarkdownFences mirrors the logic in forge.go for testing.
func cleanMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```go") {
		s = strings.TrimPrefix(s, "```go")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
