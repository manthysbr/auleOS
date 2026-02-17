package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIProvider implements LLM provider using OpenAI-compatible API
// Works with: OpenAI, Azure OpenAI, Together AI, local Ollama /v1, etc.
type OpenAIProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
}

// NewOpenAIProvider creates a new OpenAI-compatible provider
func NewOpenAIProvider(baseURL, apiKey, model string) *OpenAIProvider {
	if model == "" {
		model = "gpt-4"
	}

	return &OpenAIProvider{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
	}
}

// GenerateText generates text using OpenAI chat completions API
func (p *OpenAIProvider) GenerateText(ctx context.Context, prompt string) (string, error) {
	return p.generate(ctx, prompt, p.model)
}

// GenerateTextWithModel uses a specific model override. If modelID is empty, uses the configured default.
func (p *OpenAIProvider) GenerateTextWithModel(ctx context.Context, prompt string, modelID string) (string, error) {
	if modelID == "" {
		return p.GenerateText(ctx, prompt)
	}
	return p.generate(ctx, prompt, modelID)
}

// generate is the internal implementation that accepts an explicit model parameter (thread-safe).
func (p *OpenAIProvider) generate(ctx context.Context, prompt string, model string) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", p.baseURL)

	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}
