package imagegen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIImageProvider implements image generation via OpenAI-compatible API.
// Expected endpoint: POST {baseURL}/images/generations
// Expected response: {"data":[{"url":"https://..."}]}
type OpenAIImageProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
}

func NewOpenAIImageProvider(baseURL, apiKey, model string) *OpenAIImageProvider {
	if model == "" {
		model = "gpt-image-1"
	}
	return &OpenAIImageProvider{
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
	}
}

func (p *OpenAIImageProvider) GenerateImage(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf("%s/images/generations", p.baseURL)

	payload := map[string]interface{}{
		"model":  p.model,
		"prompt": prompt,
		"size":   "1024x1024",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call image API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("image API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode image API response: %w", err)
	}

	if len(result.Data) == 0 || strings.TrimSpace(result.Data[0].URL) == "" {
		return "", fmt.Errorf("image API returned no image URL")
	}

	return result.Data[0].URL, nil
}
