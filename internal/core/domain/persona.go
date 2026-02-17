package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// PersonaID uniquely identifies a persona
type PersonaID string

// Persona defines an agent personality with system prompt, style, and tool filtering
type Persona struct {
	ID            PersonaID `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	SystemPrompt  string    `json:"system_prompt"`
	Icon          string    `json:"icon"`           // lucide icon name
	Color         string    `json:"color"`          // tailwind color token, e.g. "blue", "emerald"
	AllowedTools  []string  `json:"allowed_tools"`  // empty = all tools allowed
	ModelOverride string    `json:"model_override"` // empty = use default model; e.g. "qwen2.5-coder:3b"
	IsBuiltin     bool      `json:"is_builtin"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

var (
	ErrPersonaNotFound = errors.New("persona not found")
)

// NewPersonaID generates a compact random persona ID (pers-<12 hex>)
func NewPersonaID() PersonaID {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return PersonaID("pers-" + hex.EncodeToString(b))
}

// BuiltinPersonas returns the default set of personas seeded on first run
func BuiltinPersonas() []Persona {
	now := time.Now()
	return []Persona{
		{
			ID:          "pers-assistant",
			Name:        "Assistant",
			Description: "General-purpose AI assistant. Balanced, clear, and helpful.",
			SystemPrompt: `You are auleOS Assistant — a capable, clear, and helpful AI.
Provide concise answers unless the user asks for details.
Use tools when the task requires them.
Be direct, avoid unnecessary preamble.`,
			Icon:         "bot",
			Color:        "blue",
			AllowedTools: nil, // all tools
			IsBuiltin:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:          "pers-researcher",
			Name:        "Researcher",
			Description: "Deep analysis and structured reasoning. Cites sources, explores nuance.",
			SystemPrompt: `You are auleOS Researcher — an analytical, thorough AI.
When answering:
- Break complex topics into structured sections
- Cite sources and reasoning steps explicitly
- Consider multiple perspectives before concluding
- Use numbered lists and headers for clarity
- Prefer depth over brevity
Use tools to gather data before synthesizing an answer.`,
			Icon:         "search",
			Color:        "emerald",
			AllowedTools: nil,
			IsBuiltin:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:          "pers-creator",
			Name:        "Creator",
			Description: "Creative content generation — images, stories, copywriting.",
			SystemPrompt: `You are auleOS Creator — a creative, expressive AI focused on content generation.
When creating:
- Prioritize vivid, engaging output
- Use rich descriptive language for image prompts
- For text: match the tone the user requests (formal, casual, poetic, etc.)
- Proactively use generate_image and generate_text tools
- Suggest creative variations and alternatives
Be bold and imaginative.`,
			Icon:         "palette",
			Color:        "violet",
			AllowedTools: []string{"generate_image", "generate_text"},
			IsBuiltin:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:          "pers-coder",
			Name:        "Coder",
			Description: "Software engineering assistant. Code-first, precise, opinionated.",
			SystemPrompt: `You are auleOS Coder — a senior software engineer AI.
When helping with code:
- Write idiomatic, production-ready code
- Prefer concise solutions over verbose explanations
- Include error handling and edge cases
- Use code blocks with language tags
- Suggest tests when relevant
- Be opinionated about best practices
Respect the user's stack and conventions.`,
			Icon:         "code",
			Color:        "amber",
			AllowedTools: []string{"generate_text"},
			IsBuiltin:    true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
}
