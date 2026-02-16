package imagegen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DirectOllamaProvider calls persistent Ollama container directly (no worker spawn)
type DirectOllamaProvider struct {
	client     *http.Client
	ollamaHost string
	model      string
}

// NewDirectOllamaProvider creates provider that calls persistent Ollama
func NewDirectOllamaProvider(ollamaHost string) *DirectOllamaProvider {
	return &DirectOllamaProvider{
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		ollamaHost: ollamaHost,
		model:      "x/z-image-turbo:fp8",
	}
}

// GenerateImage calls Ollama's /api/generate endpoint directly
func (p *DirectOllamaProvider) GenerateImage(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf("%s/api/generate", p.ollamaHost)

	payload := map[string]interface{}{
		"model":  p.model,
		"prompt": prompt,
		"stream": false,
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

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Response string `json:"response"`
		Images   []string `json:"images"` // base64 encoded images
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Return first image if available
	if len(result.Images) > 0 {
		return result.Images[0], nil
	}

	// Otherwise return text response (for debugging)
	return result.Response, nil
}
