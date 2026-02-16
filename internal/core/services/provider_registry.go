package services

import (
	"github.com/manthysbr/auleOS/internal/core/domain"
)

// ProviderRegistry manages and provides access to LLM and Image providers
type ProviderRegistry struct {
	config *domain.AppConfig
	
	// Providers set via dependency injection
	llmProvider   domain.LLMProvider
	imageProvider domain.ImageProvider
}

// NewProviderRegistry creates a new provider registry with injected providers
func NewProviderRegistry(config *domain.AppConfig, llm domain.LLMProvider, image domain.ImageProvider) *ProviderRegistry {
	if config == nil {
		config = domain.DefaultConfig()
	}
	
	return &ProviderRegistry{
		config:        config,
		llmProvider:   llm,
		imageProvider: image,
	}
}

// GetLLMProvider returns the configured LLM provider
func (r *ProviderRegistry) GetLLMProvider() domain.LLMProvider {
	return r.llmProvider
}

// GetImageProvider returns the configured image provider
func (r *ProviderRegistry) GetImageProvider() domain.ImageProvider {
	return r.imageProvider
}

// UpdateProviders updates the providers
func (r *ProviderRegistry) UpdateProviders(llm domain.LLMProvider, image domain.ImageProvider) {
	r.llmProvider = llm
	r.imageProvider = image
}

// UpdateConfig updates the configuration
func (r *ProviderRegistry) UpdateConfig(config *domain.AppConfig) {
	r.config = config
}

// GetConfig returns the current configuration
func (r *ProviderRegistry) GetConfig() *domain.AppConfig {
	return r.config
}
