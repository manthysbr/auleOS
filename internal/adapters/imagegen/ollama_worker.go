package imagegen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/services"
)

// OllamaImageWorkerProvider implements domain.ImageProvider using ephemeral Ollama workers
type OllamaImageWorkerProvider struct {
	lifecycle *services.WorkerLifecycle
	client    *http.Client
}

func NewOllamaImageWorkerProvider(lifecycle *services.WorkerLifecycle) *OllamaImageWorkerProvider {
	return &OllamaImageWorkerProvider{
		lifecycle: lifecycle,
		client:    &http.Client{Timeout: 300 * time.Second},
	}
}

// GenerateImage spawns an Ollama worker and generates an image
func (p *OllamaImageWorkerProvider) GenerateImage(ctx context.Context, prompt string) (string, error) {
	// 1. Submit job to spawn Ollama image worker
	spec := domain.WorkerSpec{
		Image:   "aule/image-gen:latest",
		Command: []string{}, // Entrypoint handles startup
		Env:     map[string]string{},
		Tags: map[string]string{
			"type":  "image-generation",
			"model": "z-image-turbo",
		},
	}

	job, err := p.lifecycle.SubmitJob(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("failed to submit image generation job: %w", err)
	}
	_ = job // TODO: Use job ID for tracking

	// 2. Wait for worker to become healthy (Ollama daemon + watchdog ready)
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var workerIP string
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for worker to be ready")
		case <-ticker.C:
			// Try to get worker IP
			ip, err := p.lifecycle.GetWorkerIP(ctx, job)
			if err != nil {
				continue // Worker not ready yet
			}
			
			// Test if watchdog is responding
			healthURL := fmt.Sprintf("http://%s:8080/health", ip)
			req, _ := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
			resp, err := p.client.Do(req)
			if err != nil {
				continue // Not ready
			}
			resp.Body.Close()
			
			if resp.StatusCode == 200 {
				workerIP = ip
				goto WORKER_READY
			}
		}
	}

WORKER_READY:
	// 3. Call Ollama API via watchdog to generate image
	watchdogURL := fmt.Sprintf("http://%s:8080/v1/exec", workerIP)
	
	// Build Ollama generate command for z-image-turbo
	ollamaPayload := map[string]interface{}{
		"model":  "z-image-turbo",
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"num_predict": 1, // Single image
		},
	}
	ollamaJSON, _ := json.Marshal(ollamaPayload)

	watchdogReq := map[string]interface{}{
		"command": "curl",
		"args": []string{
			"-X", "POST",
			"http://localhost:11434/api/generate",
			"-H", "Content-Type: application/json",
			"-d", string(ollamaJSON),
		},
		"timeout_ms": 120000, // 2 min timeout
	}

	reqBody, _ := json.Marshal(watchdogReq)
	req, _ := http.NewRequestWithContext(ctx, "POST", watchdogURL, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call watchdog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("watchdog returned %d: %s", resp.StatusCode, body)
	}

	// 4. Parse Ollama response (contains base64 image)
	var watchdogResp struct {
		ExitCode int    `json:"exit_code"`
		Output   string `json:"output"`
		Error    *string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&watchdogResp); err != nil {
		return "", fmt.Errorf("failed to parse watchdog response: %w", err)
	}

	if watchdogResp.ExitCode != 0 {
		return "", fmt.Errorf("ollama generation failed: %s", watchdogResp.Output)
	}

	// Parse Ollama's JSON response to extract image
	var ollamaResp struct {
		Response string `json:"response"` // base64 image
		Done     bool   `json:"done"`
	}
	if err := json.Unmarshal([]byte(watchdogResp.Output), &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse ollama response: %w", err)
	}

	// 5. Save image to workspace and return URL
	// For M8, return base64 directly (frontend can display)
	// In production, we'd save to /mnt/aule/workspace and return /v1/jobs/{id}/files/output.png
	
	return fmt.Sprintf("data:image/png;base64,%s", ollamaResp.Response), nil
}
