package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/manthysbr/auleOS/internal/synapse"
)

// RuntimeKind identifies which execution backend handles a capability.
type RuntimeKind string

const (
	// RuntimeSynapse runs in the Wasm sandbox (fast, CPU-only, < 10ms startup).
	RuntimeSynapse RuntimeKind = "synapse"
	// RuntimeMuscle runs via Docker containers (GPU, heavy I/O).
	RuntimeMuscle RuntimeKind = "muscle"
)

// CapabilityRoute describes how a specific capability should be executed.
type CapabilityRoute struct {
	Runtime     RuntimeKind
	Description string
}

// CapabilityRouter decides whether a capability runs via Synapse (Wasm)
// or Muscle (Docker). It centralizes the routing logic so that callers
// (ReAct agent, WorkerLifecycle, tools) don't need to know which backend
// handles each task.
type CapabilityRouter struct {
	mu      sync.RWMutex
	logger  *slog.Logger
	routes  map[string]CapabilityRoute
	synapse *synapse.Runtime
}

// NewCapabilityRouter creates a router with default capability mappings.
func NewCapabilityRouter(logger *slog.Logger, synapseRT *synapse.Runtime) *CapabilityRouter {
	router := &CapabilityRouter{
		logger:  logger,
		routes:  make(map[string]CapabilityRoute),
		synapse: synapseRT,
	}

	// Default routes — GPU/heavy tasks go to Muscle (Docker)
	router.routes["image.generate"] = CapabilityRoute{
		Runtime:     RuntimeMuscle,
		Description: "Image generation via ComfyUI (requires GPU)",
	}
	router.routes["text.generate"] = CapabilityRoute{
		Runtime:     RuntimeMuscle,
		Description: "LLM text generation via Ollama (requires GPU)",
	}
	router.routes["video.transcode"] = CapabilityRoute{
		Runtime:     RuntimeMuscle,
		Description: "Video transcoding (heavy I/O)",
	}
	router.routes["audio.transcribe"] = CapabilityRoute{
		Runtime:     RuntimeMuscle,
		Description: "Speech-to-text via Whisper (requires GPU)",
	}

	// Logic/transform tasks go to Synapse (Wasm)
	router.routes["prompt.enhance"] = CapabilityRoute{
		Runtime:     RuntimeSynapse,
		Description: "Enhance prompts with quality keywords",
	}
	router.routes["prompt.validate"] = CapabilityRoute{
		Runtime:     RuntimeSynapse,
		Description: "Validate prompt structure",
	}
	router.routes["json.transform"] = CapabilityRoute{
		Runtime:     RuntimeSynapse,
		Description: "Transform JSON data",
	}
	router.routes["text.format"] = CapabilityRoute{
		Runtime:     RuntimeSynapse,
		Description: "Format text (markdown, html, etc.)",
	}
	router.routes["data.validate"] = CapabilityRoute{
		Runtime:     RuntimeSynapse,
		Description: "Validate data against schema",
	}

	logger.Info("capability router initialized",
		"muscle_routes", router.countByRuntime(RuntimeMuscle),
		"synapse_routes", router.countByRuntime(RuntimeSynapse),
	)

	return router
}

// Resolve determines which runtime should handle the given capability.
// If the capability is unknown, it defaults to Muscle (Docker) for safety.
func (r *CapabilityRouter) Resolve(capability string) RuntimeKind {
	r.mu.RLock()
	defer r.mu.RUnlock()

	capability = strings.TrimSpace(strings.ToLower(capability))

	if route, ok := r.routes[capability]; ok {
		return route.Runtime
	}

	// Check if any loaded Synapse plugin handles this capability
	if r.synapse != nil {
		if _, ok := r.synapse.GetPlugin(capability); ok {
			return RuntimeSynapse
		}
	}

	// Unknown capabilities default to Muscle (safer for potentially heavy tasks)
	r.logger.Debug("unknown capability, defaulting to muscle", "capability", capability)
	return RuntimeMuscle
}

// RegisterRoute adds or overrides a capability route.
func (r *CapabilityRouter) RegisterRoute(capability string, route CapabilityRoute) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[strings.TrimSpace(strings.ToLower(capability))] = route
}

// ListRoutes returns all registered capability routes.
func (r *CapabilityRouter) ListRoutes() map[string]CapabilityRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]CapabilityRoute, len(r.routes))
	for k, v := range r.routes {
		result[k] = v
	}
	return result
}

// ExecuteSynapse runs a capability via the Synapse Wasm runtime.
// The plugin must exist and be loaded in the synapse.Runtime.
func (r *CapabilityRouter) ExecuteSynapse(ctx context.Context, pluginName string, params map[string]interface{}) (interface{}, error) {
	if r.synapse == nil {
		return nil, fmt.Errorf("synapse runtime is not available")
	}

	plugin, ok := r.synapse.GetPlugin(pluginName)
	if !ok {
		return nil, fmt.Errorf("synapse plugin %q not found", pluginName)
	}

	return plugin.Execute(ctx, params)
}

// Stats returns router statistics.
func (r *CapabilityRouter) Stats() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]int{
		"total":   len(r.routes),
		"muscle":  r.countByRuntime(RuntimeMuscle),
		"synapse": r.countByRuntime(RuntimeSynapse),
	}
}

func (r *CapabilityRouter) countByRuntime(kind RuntimeKind) int {
	count := 0
	for _, route := range r.routes {
		if route.Runtime == kind {
			count++
		}
	}
	return count
}
