package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// ModelDiscovery discovers available models from Ollama and LiteLLM endpoints.
type ModelDiscovery struct {
	logger *slog.Logger
	client *http.Client
}

// NewModelDiscovery creates a new model discovery service.
func NewModelDiscovery(logger *slog.Logger) *ModelDiscovery {
	return &ModelDiscovery{
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// ollamaTagsResponse is the Ollama /api/tags JSON structure.
type ollamaTagsResponse struct {
	Models []struct {
		Name    string `json:"name"`
		Model   string `json:"model"`
		Details struct {
			ParameterSize     string `json:"parameter_size"`
			QuantizationLevel string `json:"quantization_level"`
			Family            string `json:"family"`
		} `json:"details"`
	} `json:"models"`
}

// litellmModelsResponse is the LiteLLM /models or /v1/models response.
type litellmModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// DiscoverOllama queries the Ollama instance at baseURL for installed models.
func (d *ModelDiscovery) DiscoverOllama(ctx context.Context, baseURL string) ([]domain.ModelSpec, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	url := baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama not reachable at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama returned %d", resp.StatusCode)
	}

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("decode ollama tags: %w", err)
	}

	models := make([]domain.ModelSpec, 0, len(tags.Models))
	for _, m := range tags.Models {
		role := inferRole(m.Name, m.Details.Family)
		models = append(models, domain.ModelSpec{
			ID:       m.Name,
			Name:     m.Name,
			Provider: "ollama",
			Role:     role,
			Size:     m.Details.ParameterSize,
			BaseURL:  baseURL,
			IsLocal:  true,
		})
	}

	d.logger.Info("discovered ollama models", "count", len(models), "base_url", baseURL)
	return models, nil
}

// DiscoverLiteLLM queries a LiteLLM proxy for available models via OpenAI-compatible /v1/models.
func (d *ModelDiscovery) DiscoverLiteLLM(ctx context.Context, baseURL string, apiKey string) ([]domain.ModelSpec, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("litellm base URL is required")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	url := baseURL + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("litellm not reachable at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("litellm returned %d", resp.StatusCode)
	}

	var result litellmModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode litellm models: %w", err)
	}

	models := make([]domain.ModelSpec, 0, len(result.Data))
	for _, m := range result.Data {
		role := inferRole(m.ID, m.OwnedBy)
		models = append(models, domain.ModelSpec{
			ID:       m.ID,
			Name:     m.ID,
			Provider: "litellm",
			Role:     role,
			Size:     "", // LiteLLM doesn't expose param size
			BaseURL:  baseURL,
			IsLocal:  false,
		})
	}

	d.logger.Info("discovered litellm models", "count", len(models), "base_url", baseURL)
	return models, nil
}

// inferRole guesses a ModelRole from the model name / family.
func inferRole(name string, extra string) domain.ModelRole {
	lower := strings.ToLower(name + " " + extra)
	switch {
	case strings.Contains(lower, "coder") || strings.Contains(lower, "code") || strings.Contains(lower, "starcoder") || strings.Contains(lower, "deepseek-coder"):
		return domain.ModelRoleCode
	case strings.Contains(lower, "phi") || strings.Contains(lower, "mini") || strings.Contains(lower, "tiny"):
		return domain.ModelRoleFast
	case strings.Contains(lower, "gemma") || strings.Contains(lower, "creative") || strings.Contains(lower, "mistral"):
		return domain.ModelRoleCreative
	default:
		return domain.ModelRoleGeneral
	}
}
