package providers

import (
	"fmt"
	"os"
	"strings"

	"github.com/manthysbr/auleOS/internal/adapters/imagegen"
	"github.com/manthysbr/auleOS/internal/adapters/llm"
	"github.com/manthysbr/auleOS/internal/core/domain"
)

// Build creates LLM and Image providers from app configuration.
// It hides local/remote provider selection from callers.
func Build(config *domain.AppConfig) (domain.LLMProvider, domain.ImageProvider, error) {
	if config == nil {
		config = domain.DefaultConfig()
	}

	llmProvider, err := buildLLMProvider(config)
	if err != nil {
		return nil, nil, err
	}

	imageProvider, err := buildImageProvider(config)
	if err != nil {
		return nil, nil, err
	}

	return llmProvider, imageProvider, nil
}

func buildLLMProvider(config *domain.AppConfig) (domain.LLMProvider, error) {
	mode := strings.ToLower(strings.TrimSpace(config.Providers.LLM.Mode))
	switch mode {
	case "", "local":
		baseURL := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
		if baseURL == "" {
			baseURL = strings.TrimSpace(config.Providers.LLM.LocalURL)
		}
		baseURL = normalizeOllamaBaseURL(baseURL)
		return llm.NewOllamaProvider(baseURL), nil
	case "remote":
		if strings.TrimSpace(config.Providers.LLM.RemoteURL) == "" {
			return nil, fmt.Errorf("llm remote_url is required when mode=remote")
		}
		return llm.NewOpenAIProvider(
			strings.TrimSpace(config.Providers.LLM.RemoteURL),
			strings.TrimSpace(config.Providers.LLM.APIKey),
			strings.TrimSpace(config.Providers.LLM.DefaultModel),
		), nil
	default:
		return nil, fmt.Errorf("unsupported llm provider mode: %s", config.Providers.LLM.Mode)
	}
}

func buildImageProvider(config *domain.AppConfig) (domain.ImageProvider, error) {
	mode := strings.ToLower(strings.TrimSpace(config.Providers.Image.Mode))
	switch mode {
	case "", "local":
		comfyHost := strings.TrimSpace(os.Getenv("COMFYUI_HOST"))
		if comfyHost == "" {
			comfyHost = strings.TrimSpace(config.Providers.Image.LocalURL)
		}
		if comfyHost == "" {
			comfyHost = "http://localhost:8188"
		}
		return imagegen.NewDirectComfyUIProvider(comfyHost), nil
	case "remote":
		if strings.TrimSpace(config.Providers.Image.RemoteURL) == "" {
			return nil, fmt.Errorf("image remote_url is required when mode=remote")
		}
		return imagegen.NewOpenAIImageProvider(
			strings.TrimSpace(config.Providers.Image.RemoteURL),
			strings.TrimSpace(config.Providers.Image.APIKey),
			strings.TrimSpace(config.Providers.Image.DefaultModel),
		), nil
	default:
		return nil, fmt.Errorf("unsupported image provider mode: %s", config.Providers.Image.Mode)
	}
}

func normalizeOllamaBaseURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(trimmed, "/v1") {
		return strings.TrimSuffix(trimmed, "/v1")
	}
	return trimmed
}
