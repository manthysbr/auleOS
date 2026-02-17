package services

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// ModelRouter resolves which LLMProvider + model to use for a given request.
// It supports:
//   - A default provider (Ollama local or remote OpenAI-compatible)
//   - Per-model-role routing (code → qwen2.5-coder:3b, creative → gemma2:2b, etc.)
//   - Persona-level model override (persona.ModelOverride)
//   - Catalog of available models (discovered from Ollama / LiteLLM)
type ModelRouter struct {
	logger   *slog.Logger
	mu       sync.RWMutex
	provider domain.LLMProvider // The single provider that handles all requests

	// roleDefaults maps ModelRole → preferred model ID
	roleDefaults map[domain.ModelRole]string

	// catalog of available models (populated by discovery)
	catalog []domain.ModelSpec
}

// NewModelRouter creates a router with the given base LLM provider.
func NewModelRouter(logger *slog.Logger, provider domain.LLMProvider) *ModelRouter {
	return &ModelRouter{
		logger:   logger,
		provider: provider,
		roleDefaults: map[domain.ModelRole]string{
			domain.ModelRoleGeneral:  "qwen2.5:latest",
			domain.ModelRoleCode:     "qwen2.5:latest",
			domain.ModelRoleCreative: "qwen2.5:latest",
			domain.ModelRoleFast:     "llama3.2:latest",
		},
		catalog: domain.RecommendedLocalModels(),
	}
}

// ResolveModel picks the best model ID for a sub-agent task.
// Priority: persona.ModelOverride > role default > empty (use provider default).
func (r *ModelRouter) ResolveModel(persona *domain.Persona, role domain.ModelRole) string {
	// 1. Persona-level override wins
	if persona != nil && persona.ModelOverride != "" {
		return persona.ModelOverride
	}

	// 2. Role-based default
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.roleDefaults[role]; ok {
		return m
	}

	// 3. Fallback — empty means use provider default
	return ""
}

// GenerateText delegates to the underlying provider with an optional model override.
func (r *ModelRouter) GenerateText(ctx context.Context, prompt string, modelID string) (string, error) {
	r.logger.Debug("model router generating text", "model", modelID)
	return r.provider.GenerateTextWithModel(ctx, prompt, modelID)
}

// UpdateProvider hot-swaps the underlying LLM provider (called on settings change).
func (r *ModelRouter) UpdateProvider(p domain.LLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.provider = p
}

// SetRoleDefault overrides the default model for a role.
func (r *ModelRouter) SetRoleDefault(role domain.ModelRole, modelID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roleDefaults[role] = modelID
}

// SetCatalog replaces the known model catalog (called after discovery).
func (r *ModelRouter) SetCatalog(models []domain.ModelSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.catalog = models
}

// GetCatalog returns the current model catalog.
func (r *ModelRouter) GetCatalog() []domain.ModelSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.ModelSpec, len(r.catalog))
	copy(out, r.catalog)
	return out
}

// GetRoleDefaults returns the current role → model mapping.
func (r *ModelRouter) GetRoleDefaults() map[domain.ModelRole]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[domain.ModelRole]string, len(r.roleDefaults))
	for k, v := range r.roleDefaults {
		out[k] = v
	}
	return out
}

// inferRoleFromPersona guesses the best model role from a persona's name.
func (r *ModelRouter) inferRoleFromPersona(persona *domain.Persona) domain.ModelRole {
	if persona == nil {
		return domain.ModelRoleGeneral
	}
	lower := strings.ToLower(persona.Name)
	switch {
	case strings.Contains(lower, "coder") || strings.Contains(lower, "code"):
		return domain.ModelRoleCode
	case strings.Contains(lower, "creator") || strings.Contains(lower, "creative"):
		return domain.ModelRoleCreative
	case strings.Contains(lower, "researcher") || strings.Contains(lower, "research"):
		return domain.ModelRoleFast
	default:
		return domain.ModelRoleGeneral
	}
}
