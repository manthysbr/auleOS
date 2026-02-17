package synapse_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/synapse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Minimal valid Wasm module: exports memory + _start (no-op).
// Equivalent WAT:
//
//	(module
//	  (memory (export "memory") 1)
//	  (func (export "_start"))
//	)
//
// Hand-encoded with verified section lengths.
var noopWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, // magic: \0asm
	0x01, 0x00, 0x00, 0x00, // version: 1

	// Type section: 1 type — () -> ()
	0x01, 0x04,
	0x01, 0x60, 0x00, 0x00,

	// Function section: 1 func → type 0
	0x03, 0x02,
	0x01, 0x00,

	// Memory section: 1 memory, min=1 page
	0x05, 0x03,
	0x01, 0x00, 0x01,

	// Export section: "memory" (mem 0) + "_start" (func 0)
	0x07, 0x13,
	0x02,
	0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00, // "memory" -> mem 0
	0x06, 0x5f, 0x73, 0x74, 0x61, 0x72, 0x74, 0x00, 0x00, // "_start" -> func 0

	// Code section: 1 body — empty (just end)
	0x0a, 0x04,
	0x01, 0x02, 0x00, 0x0b,
}

func testMeta() synapse.PluginMeta {
	return synapse.PluginMeta{
		Name:        "test-plugin",
		Version:     "0.1.0",
		Description: "A test no-op plugin",
		ToolName:    "test_noop",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input text",
				},
			},
			Required: []string{"input"},
		},
	}
}

func TestRuntimeLifecycle(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	rt, err := synapse.NewRuntime(ctx, logger)
	require.NoError(t, err)
	defer rt.Close(ctx)

	// Initially empty
	assert.Empty(t, rt.ListPlugins())

	// Load a plugin
	meta := testMeta()
	plugin, err := rt.LoadPlugin(ctx, "test-plugin", noopWasm, meta)
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", plugin.Name())

	// Plugin is listed
	assert.Equal(t, []string{"test-plugin"}, rt.ListPlugins())

	// Can retrieve
	got, ok := rt.GetPlugin("test-plugin")
	assert.True(t, ok)
	assert.Equal(t, "test-plugin", got.Name())

	// Can unload
	err = rt.UnloadPlugin(ctx, "test-plugin")
	require.NoError(t, err)
	assert.Empty(t, rt.ListPlugins())
}

func TestPluginExecuteNoopReturnsDefault(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	rt, err := synapse.NewRuntime(ctx, logger)
	require.NoError(t, err)
	defer rt.Close(ctx)

	meta := testMeta()
	plugin, err := rt.LoadPlugin(ctx, "test-plugin", noopWasm, meta)
	require.NoError(t, err)

	// Execute: noop module produces empty stdout → default response
	result, err := plugin.Execute(ctx, map[string]interface{}{
		"input": "hello synapse",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ok", resultMap["status"])
	assert.Equal(t, "test-plugin", resultMap["plugin"])
}

func TestPluginAsTool(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	rt, err := synapse.NewRuntime(ctx, logger)
	require.NoError(t, err)
	defer rt.Close(ctx)

	meta := testMeta()
	plugin, err := rt.LoadPlugin(ctx, "test-plugin", noopWasm, meta)
	require.NoError(t, err)

	// Convert to tool
	tool := plugin.AsTool()
	assert.Equal(t, "test_noop", tool.Name)
	assert.Equal(t, "A test no-op plugin", tool.Description)

	// Register in ToolRegistry and execute via it
	registry := domain.NewToolRegistry()
	err = registry.Register(tool)
	require.NoError(t, err)

	result, err := registry.Execute(ctx, "test_noop", map[string]interface{}{
		"input": "from registry",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ok", resultMap["status"])
}

func TestPluginHotReload(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	rt, err := synapse.NewRuntime(ctx, logger)
	require.NoError(t, err)
	defer rt.Close(ctx)

	meta := testMeta()

	// Load original
	_, err = rt.LoadPlugin(ctx, "test-plugin", noopWasm, meta)
	require.NoError(t, err)

	// Load again (hot-reload) — should not error
	meta.Version = "0.2.0"
	_, err = rt.LoadPlugin(ctx, "test-plugin", noopWasm, meta)
	require.NoError(t, err)

	// Still only one plugin
	assert.Len(t, rt.ListPlugins(), 1)

	got, ok := rt.GetPlugin("test-plugin")
	require.True(t, ok)
	assert.Equal(t, "0.2.0", got.Meta().Version)
}

func TestRegistryDiscoverEmpty(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	rt, err := synapse.NewRuntime(ctx, logger)
	require.NoError(t, err)
	defer rt.Close(ctx)

	// Use a temp dir with no plugins
	tmpDir := t.TempDir()
	reg := synapse.NewRegistry(logger, rt, tmpDir)

	tools, err := reg.DiscoverAndLoad(ctx)
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestRegistryDiscoverWasmFile(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	rt, err := synapse.NewRuntime(ctx, logger)
	require.NoError(t, err)
	defer rt.Close(ctx)

	// Create a temp dir with a .wasm file
	tmpDir := t.TempDir()
	wasmPath := tmpDir + "/my-tool.wasm"
	require.NoError(t, os.WriteFile(wasmPath, noopWasm, 0644))

	reg := synapse.NewRegistry(logger, rt, tmpDir)
	tools, err := reg.DiscoverAndLoad(ctx)
	require.NoError(t, err)

	// Should auto-discover the .wasm file
	require.Len(t, tools, 1)
	assert.Equal(t, "my-tool", tools[0].Name)
}
