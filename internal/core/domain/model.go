package domain

// ModelRole classifies models by their primary use case.
// The orchestrator picks the best model for each sub-agent task based on role.
type ModelRole string

const (
	ModelRoleGeneral  ModelRole = "general"  // balanced assistant
	ModelRoleCode     ModelRole = "code"     // code generation
	ModelRoleCreative ModelRole = "creative" // writing, image prompts
	ModelRoleFast     ModelRole = "fast"     // summarisation, routing, quick tasks
)

// ModelSpec describes a model available in the system, either local (Ollama) or remote (LiteLLM/OpenAI).
type ModelSpec struct {
	ID       string    `json:"id"`       // unique key: "qwen2.5:3b", "gpt-4o-mini"
	Name     string    `json:"name"`     // human-readable: "Qwen 2.5 3B"
	Provider string    `json:"provider"` // "ollama", "litellm", "openai", "azure"
	Role     ModelRole `json:"role"`     // primary use case
	Size     string    `json:"size"`     // parameter count: "3B", "7B", "70B"
	BaseURL  string    `json:"base_url"` // endpoint override; empty = use provider default
	IsLocal  bool      `json:"is_local"` // true = Ollama / local inference
}

// RecommendedLocalModels returns small models suitable for local Ollama testing.
// Each serves a different role so sub-agents can pick the best fit.
// All are ≤6B parameters → run comfortably on consumer GPUs.
func RecommendedLocalModels() []ModelSpec {
	return []ModelSpec{
		{
			ID:       "qwen2.5:3b",
			Name:     "Qwen 2.5 3B",
			Provider: "ollama",
			Role:     ModelRoleGeneral,
			Size:     "3B",
			IsLocal:  true,
		},
		{
			ID:       "qwen2.5-coder:3b",
			Name:     "Qwen 2.5 Coder 3B",
			Provider: "ollama",
			Role:     ModelRoleCode,
			Size:     "3B",
			IsLocal:  true,
		},
		{
			ID:       "gemma2:2b",
			Name:     "Gemma 2 2B",
			Provider: "ollama",
			Role:     ModelRoleCreative,
			Size:     "2B",
			IsLocal:  true,
		},
		{
			ID:       "phi4-mini:3.8b",
			Name:     "Phi-4 Mini 3.8B",
			Provider: "ollama",
			Role:     ModelRoleFast,
			Size:     "3.8B",
			IsLocal:  true,
		},
		{
			ID:       "llama3.2:3b",
			Name:     "Llama 3.2 3B",
			Provider: "ollama",
			Role:     ModelRoleGeneral,
			Size:     "3B",
			IsLocal:  true,
		},
	}
}
