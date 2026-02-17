package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Provider abstracts the LLM backend
type Provider interface {
	Generate(ctx context.Context, prompt string, model string) (string, error)
}

// OllamaProvider implements Provider for local Ollama instance
type OllamaProvider struct {
	baseURL string
	client  *http.Client
}

func NewOllamaProvider(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func (p *OllamaProvider) Generate(ctx context.Context, prompt string, model string) (string, error) {
	reqBody := generateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status: %d", resp.StatusCode)
	}

	var genResp generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return genResp.Response, nil
}

// GenerateText implements domain.LLMProvider interface using the default model
func (p *OllamaProvider) GenerateText(ctx context.Context, prompt string) (string, error) {
	return p.Generate(ctx, prompt, "qwen2.5:latest")
}

// GenerateTextWithModel implements domain.LLMProvider — uses a specific model, falls back to default if empty
func (p *OllamaProvider) GenerateTextWithModel(ctx context.Context, prompt string, modelID string) (string, error) {
	if modelID == "" {
		return p.GenerateText(ctx, prompt)
	}
	return p.Generate(ctx, prompt, modelID)
}
