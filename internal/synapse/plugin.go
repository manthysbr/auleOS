package synapse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// PluginMeta describes a Wasm plugin's identity and tool interface.
// This is loaded from the plugin manifest (metadata.json) or embedded
// in the .wasm custom section.
type PluginMeta struct {
	Name        string                `json:"name"`
	Version     string                `json:"version"`
	Description string                `json:"description"`
	ToolName    string                `json:"tool_name"`
	Parameters  domain.ToolParameters `json:"parameters"`
	Timeout     time.Duration         `json:"timeout,omitempty"` // max execution time (default 5s)
}

// Plugin represents a compiled Wasm module that can be executed as a Tool.
// Each invocation instantiates a fresh module (no shared state between calls),
// providing strong isolation guarantees.
type Plugin struct {
	name     string
	meta     PluginMeta
	compiled wazero.CompiledModule
	rt       wazero.Runtime
	logger   *slog.Logger
}

// Execute runs the Wasm plugin with the given JSON input.
// It creates a fresh module instance per call (sandboxed), pipes input via stdin,
// and captures output from stdout. Stderr is captured for logging.
//
// Protocol:
//   - Input:  JSON object written to stdin
//   - Output: JSON object read from stdout
//   - Errors: Text written to stderr (logged, not returned to caller)
//
// The module's _start (main) function is called, similar to a CLI tool.
func (p *Plugin) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	timeout := p.meta.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Serialize input as JSON → stdin
	inputJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("synapse: failed to marshal input for %q: %w", p.name, err)
	}

	stdin := bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer

	// Each call gets a fresh module instance — no shared mutable state.
	// This is fast because the module is already AOT-compiled.
	moduleCfg := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithStartFunctions("_start"). // WASI convention: calls main()
		WithName("")                  // anonymous instance (allows concurrent calls)

	mod, err := p.rt.InstantiateModule(ctx, p.compiled, moduleCfg)
	if err != nil {
		stderrMsg := stderr.String()
		if stderrMsg != "" {
			p.logger.Warn("synapse: plugin stderr", "plugin", p.name, "stderr", stderrMsg)
		}
		return nil, fmt.Errorf("synapse: execution failed for %q: %w", p.name, err)
	}
	defer mod.Close(ctx)

	// Log stderr if any (useful for plugin debugging)
	if stderrMsg := stderr.String(); stderrMsg != "" {
		p.logger.Debug("synapse: plugin stderr", "plugin", p.name, "stderr", stderrMsg)
	}

	// Parse stdout as JSON result
	outBytes := stdout.Bytes()
	if len(outBytes) == 0 {
		return map[string]interface{}{
			"status": "ok",
			"plugin": p.name,
		}, nil
	}

	var result interface{}
	if err := json.Unmarshal(outBytes, &result); err != nil {
		// If output isn't JSON, return as raw string
		return map[string]interface{}{
			"status": "ok",
			"plugin": p.name,
			"output": stdout.String(),
		}, nil
	}

	return result, nil
}

// AsTool converts this Plugin into a domain.Tool that can be registered
// in the agent's ToolRegistry. This bridges the Synapse (Wasm) world
// with the existing auleOS tool system.
func (p *Plugin) AsTool() *domain.Tool {
	return &domain.Tool{
		Name:        p.meta.ToolName,
		Description: p.meta.Description,
		Parameters:  p.meta.Parameters,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return p.Execute(ctx, params)
		},
	}
}

// ExportedFunctions returns the list of exported function names from the compiled module.
// Useful for introspection and debugging.
func (p *Plugin) ExportedFunctions() []string {
	defs := p.compiled.ExportedFunctions()
	names := make([]string, 0, len(defs))
	for name := range defs {
		names = append(names, name)
	}
	return names
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return p.name
}

// Meta returns the plugin metadata.
func (p *Plugin) Meta() PluginMeta {
	return p.meta
}

// Close frees the compiled module resources.
func (p *Plugin) Close(ctx context.Context) {
	if p.compiled != nil {
		p.compiled.Close(ctx)
	}
}

// Ensure api.Module is used (for future host function bindings)
var _ api.Module = (api.Module)(nil)
