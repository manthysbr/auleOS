package synapse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// ManifestFile is the expected filename for plugin manifests.
const ManifestFile = "plugins.json"

// PluginManifest describes the on-disk format of the plugin registry.
type PluginManifest struct {
	Plugins []PluginEntry `json:"plugins"`
}

// PluginEntry is a single plugin definition in the manifest.
type PluginEntry struct {
	Name        string                `json:"name"`
	Version     string                `json:"version"`
	File        string                `json:"file"` // relative path to .wasm
	Description string                `json:"description"`
	ToolName    string                `json:"tool_name"`
	Parameters  domain.ToolParameters `json:"parameters"`
	Runtime     string                `json:"runtime"` // "synapse" or "muscle"
	Enabled     bool                  `json:"enabled"`
}

// Registry discovers, loads, and manages Wasm plugins from a directory.
// It reads a plugins.json manifest and loads the referenced .wasm files
// into the Synapse Runtime, registering each as a Tool.
type Registry struct {
	logger    *slog.Logger
	runtime   *Runtime
	pluginDir string
}

// NewRegistry creates a plugin registry that scans pluginDir for plugins.
// The directory should contain .wasm files and optionally a plugins.json manifest.
func NewRegistry(logger *slog.Logger, runtime *Runtime, pluginDir string) *Registry {
	return &Registry{
		logger:    logger,
		runtime:   runtime,
		pluginDir: pluginDir,
	}
}

// DiscoverAndLoad scans the plugin directory and loads all enabled plugins.
// It returns the list of domain.Tool instances ready for registration.
//
// Discovery strategy:
//  1. If plugins.json exists → load manifest and use declared entries
//  2. Else → scan for *.wasm files, generate default metadata
func (r *Registry) DiscoverAndLoad(ctx context.Context) ([]*domain.Tool, error) {
	// Ensure plugin directory exists
	if err := os.MkdirAll(r.pluginDir, 0755); err != nil {
		return nil, fmt.Errorf("synapse: failed to create plugin dir %q: %w", r.pluginDir, err)
	}

	manifestPath := filepath.Join(r.pluginDir, ManifestFile)
	if _, err := os.Stat(manifestPath); err == nil {
		return r.loadFromManifest(ctx, manifestPath)
	}

	// No manifest — scan for .wasm files with auto-generated metadata
	return r.loadFromDirectory(ctx)
}

// loadFromManifest reads plugins.json and loads declared plugins.
func (r *Registry) loadFromManifest(ctx context.Context, manifestPath string) ([]*domain.Tool, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("synapse: failed to read manifest: %w", err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("synapse: failed to parse manifest: %w", err)
	}

	var tools []*domain.Tool

	for _, entry := range manifest.Plugins {
		if !entry.Enabled {
			r.logger.Debug("synapse: skipping disabled plugin", "name", entry.Name)
			continue
		}

		if entry.Runtime != "" && entry.Runtime != "synapse" {
			r.logger.Debug("synapse: skipping non-synapse plugin", "name", entry.Name, "runtime", entry.Runtime)
			continue
		}

		wasmPath := filepath.Join(r.pluginDir, entry.File)
		wasmBytes, err := os.ReadFile(wasmPath)
		if err != nil {
			r.logger.Error("synapse: failed to read plugin wasm", "name", entry.Name, "path", wasmPath, "error", err)
			continue
		}

		meta := PluginMeta{
			Name:        entry.Name,
			Version:     entry.Version,
			Description: entry.Description,
			ToolName:    entry.ToolName,
			Parameters:  entry.Parameters,
		}

		plugin, err := r.runtime.LoadPlugin(ctx, entry.Name, wasmBytes, meta)
		if err != nil {
			r.logger.Error("synapse: failed to load plugin", "name", entry.Name, "error", err)
			continue
		}

		tools = append(tools, plugin.AsTool())
		r.logger.Info("synapse: registered plugin as tool",
			"plugin", entry.Name,
			"tool", entry.ToolName,
		)
	}

	return tools, nil
}

// loadFromDirectory scans for .wasm files and creates default metadata.
func (r *Registry) loadFromDirectory(ctx context.Context) ([]*domain.Tool, error) {
	entries, err := os.ReadDir(r.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("synapse: failed to read plugin dir: %w", err)
	}

	var tools []*domain.Tool

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".wasm")
		wasmPath := filepath.Join(r.pluginDir, entry.Name())
		wasmBytes, err := os.ReadFile(wasmPath)
		if err != nil {
			r.logger.Error("synapse: failed to read wasm file", "file", entry.Name(), "error", err)
			continue
		}

		// Auto-generated metadata for manifest-less plugins
		meta := PluginMeta{
			Name:        name,
			Version:     "0.0.0",
			Description: fmt.Sprintf("Wasm plugin: %s", name),
			ToolName:    name,
			Parameters: domain.ToolParameters{
				Type: "object",
				Properties: map[string]interface{}{
					"input": map[string]interface{}{
						"type":        "string",
						"description": "Input text for the plugin",
					},
				},
				Required: []string{"input"},
			},
		}

		plugin, err := r.runtime.LoadPlugin(ctx, name, wasmBytes, meta)
		if err != nil {
			r.logger.Error("synapse: failed to load wasm", "name", name, "error", err)
			continue
		}

		tools = append(tools, plugin.AsTool())
		r.logger.Info("synapse: auto-discovered plugin", "name", name)
	}

	return tools, nil
}

// PluginDir returns the configured plugin directory path.
func (r *Registry) PluginDir() string {
	return r.pluginDir
}
