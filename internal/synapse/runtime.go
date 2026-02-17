// Package synapse provides the lightweight Wasm runtime for auleOS plugins.
// It uses wazero (zero-dependency, pure-Go WebAssembly runtime) to execute
// .wasm modules as sandboxed tools within the agent's ToolRegistry.
//
// Architecture: "Synapse" handles fast, CPU-only tasks (prompt transforms,
// JSON parsing, validation, formatting) while "Muscle" (Docker) handles
// GPU-bound tasks (LLM inference, image generation).
package synapse

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime manages the wazero WebAssembly runtime and loaded modules.
// It compiles .wasm binaries using AOT (Ahead-Of-Time) compilation
// for near-native performance on linux/amd64.
type Runtime struct {
	mu      sync.RWMutex
	logger  *slog.Logger
	rt      wazero.Runtime
	plugins map[string]*Plugin // name → loaded plugin
}

// NewRuntime creates a new Wasm runtime with AOT compilation and WASI support.
// Call Close() when done to free compiled module caches.
func NewRuntime(ctx context.Context, logger *slog.Logger) (*Runtime, error) {
	// AOT compiler: compiles Wasm→native on first load, ~10x faster than interpreter.
	// Falls back to interpreter on unsupported architectures.
	cfg := wazero.NewRuntimeConfigCompiler().
		WithCloseOnContextDone(true)

	rt := wazero.NewRuntimeWithConfig(ctx, cfg)

	// Instantiate WASI preview1 — provides stdout/stderr, args, env, basic FS.
	// Plugins use stdin/stdout for I/O with the host (kernel).
	_, err := wasi_snapshot_preview1.Instantiate(ctx, rt)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("synapse: failed to instantiate WASI: %w", err)
	}

	logger.Info("synapse runtime initialized", "compiler", "aot")

	return &Runtime{
		logger:  logger,
		rt:      rt,
		plugins: make(map[string]*Plugin),
	}, nil
}

// LoadPlugin compiles a .wasm binary and registers it as a named plugin.
// The plugin becomes available as a Tool in the agent's ToolRegistry.
// If a plugin with the same name exists, it is replaced (hot-reload).
func (r *Runtime) LoadPlugin(ctx context.Context, name string, wasmBytes []byte, meta PluginMeta) (*Plugin, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close existing plugin if hot-reloading
	if existing, ok := r.plugins[name]; ok {
		existing.Close(ctx)
		r.logger.Info("synapse: replacing existing plugin", "name", name)
	}

	// Compile: validates Wasm binary and AOT-compiles to native code.
	// This is expensive (~ms) but only done once per load.
	compiled, err := r.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("synapse: failed to compile %q: %w", name, err)
	}

	plugin := &Plugin{
		name:     name,
		meta:     meta,
		compiled: compiled,
		rt:       r.rt,
		logger:   r.logger,
	}

	r.plugins[name] = plugin
	r.logger.Info("synapse: plugin loaded",
		"name", name,
		"version", meta.Version,
		"description", meta.Description,
	)

	return plugin, nil
}

// GetPlugin returns a loaded plugin by name.
func (r *Runtime) GetPlugin(name string) (*Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// ListPlugins returns all loaded plugin names.
func (r *Runtime) ListPlugins() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}

// UnloadPlugin removes and closes a plugin by name.
func (r *Runtime) UnloadPlugin(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("synapse: plugin %q not found", name)
	}

	plugin.Close(ctx)
	delete(r.plugins, name)
	r.logger.Info("synapse: plugin unloaded", "name", name)
	return nil
}

// Close shuts down the entire runtime and all loaded plugins.
func (r *Runtime) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, plugin := range r.plugins {
		plugin.Close(ctx)
		r.logger.Debug("synapse: closed plugin", "name", name)
	}
	r.plugins = nil

	return r.rt.Close(ctx)
}
