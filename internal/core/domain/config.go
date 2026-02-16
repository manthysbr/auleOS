package domain

// ProviderConfig holds configuration for all AI providers
type ProviderConfig struct {
	LLM   LLMProviderConfig   `json:"llm"`
	Image ImageProviderConfig `json:"image"`
}

// LLMProviderConfig configures the LLM provider
type LLMProviderConfig struct {
	Mode         string `json:"mode"`          // "local" or "remote"
	LocalURL     string `json:"local_url"`     // "http://localhost:11434/v1"
	RemoteURL    string `json:"remote_url"`    // "https://api.openai.com/v1"
	APIKey       string `json:"api_key"`       // Encrypted in storage
	DefaultModel string `json:"default_model"` // "gemma3:12b" or "gpt-4"
}

// ImageProviderConfig configures the image generation provider
type ImageProviderConfig struct {
	Mode         string `json:"mode"`          // "local" or "remote"
	LocalURL     string `json:"local_url"`     // "http://localhost:8188"
	RemoteURL    string `json:"remote_url"`    // "https://api.replicate.com/v1"
	APIKey       string `json:"api_key"`       // Encrypted in storage
	DefaultModel string `json:"default_model"` // "sd-1.5" or "sdxl-turbo"
}

// AppConfig is the main application configuration
type AppConfig struct {
	Providers ProviderConfig `json:"providers"`
}

// DefaultConfig returns safe defaults
func DefaultConfig() *AppConfig {
	return &AppConfig{
		Providers: ProviderConfig{
			LLM: LLMProviderConfig{
				Mode:         "local",
				LocalURL:     "http://localhost:11434/v1",
				DefaultModel: "gemma3:12b",
			},
			Image: ImageProviderConfig{
				Mode:         "local",
				LocalURL:     "http://localhost:8188",
				DefaultModel: "sd-1.5",
			},
		},
	}
}
