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

// DirectComfyUIProvider calls persistent ComfyUI container
type DirectComfyUIProvider struct {
	client     *http.Client
	comfyHost  string
	checkpoint string
}

// NewDirectComfyUIProvider creates provider that calls persistent ComfyUI
func NewDirectComfyUIProvider(comfyHost string) *DirectComfyUIProvider {
	return &DirectComfyUIProvider{
		client: &http.Client{
			Timeout: 180 * time.Second, // Image generation can take longer
		},
		comfyHost:  comfyHost,
		checkpoint: "v1-5-pruned-emaonly.safetensors",
	}
}

// GenerateImage calls ComfyUI's /prompt endpoint with workflow
func (p *DirectComfyUIProvider) GenerateImage(ctx context.Context, prompt string) (string, error) {
	// Build simple workflow for SD 1.5
	workflow := p.buildWorkflow(prompt)

	url := fmt.Sprintf("%s/prompt", p.comfyHost)

	payloadBytes, err := json.Marshal(workflow)
	if err != nil {
		return "", fmt.Errorf("failed to marshal workflow: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call ComfyUI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ComfyUI returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		PromptID string `json:"prompt_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.PromptID == "" {
		return "", fmt.Errorf("no prompt_id returned")
	}

	// Wait for generation to complete and fetch image
	imageURL, err := p.waitAndFetchImage(ctx, result.PromptID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch image: %w", err)
	}

	return imageURL, nil
}

// waitAndFetchImage polls /history until generation completes and returns image URL
func (p *DirectComfyUIProvider) waitAndFetchImage(ctx context.Context, promptID string) (string, error) {
	maxAttempts := 60 // 60 attempts * 2s = 120s max wait
	
	for i := 0; i < maxAttempts; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Check history
		url := fmt.Sprintf("%s/history/%s", p.comfyHost, promptID)
		resp, err := p.client.Get(url)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		var history map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
			resp.Body.Close()
			time.Sleep(2 * time.Second)
			continue
		}
		resp.Body.Close()

		// Check if prompt exists in history
		promptData, ok := history[promptID].(map[string]interface{})
		if !ok {
			time.Sleep(2 * time.Second)
			continue
		}

		// Check if generation is complete
		outputs, ok := promptData["outputs"].(map[string]interface{})
		if !ok {
			time.Sleep(2 * time.Second)
			continue
		}

		// Find SaveImage node output (node "9" in our workflow)
		saveImageOutput, ok := outputs["9"].(map[string]interface{})
		if !ok {
			time.Sleep(2 * time.Second)
			continue
		}

		// Get images array
		images, ok := saveImageOutput["images"].([]interface{})
		if !ok || len(images) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		// Get first image metadata
		imageData, ok := images[0].(map[string]interface{})
		if !ok {
			time.Sleep(2 * time.Second)
			continue
		}

		filename, ok := imageData["filename"].(string)
		if !ok {
			time.Sleep(2 * time.Second)
			continue
		}

		// Return URL to view the image
		imageURL := fmt.Sprintf("%s/view?filename=%s&type=output", p.comfyHost, filename)
		return imageURL, nil
	}

	return "", fmt.Errorf("timeout waiting for image generation")
}

// buildWorkflow creates a simple SD 1.5 workflow
func (p *DirectComfyUIProvider) buildWorkflow(prompt string) map[string]interface{} {
	return map[string]interface{}{
		"prompt": map[string]interface{}{
			// KSampler
			"3": map[string]interface{}{
				"inputs": map[string]interface{}{
					"seed":         42,
					"steps":        20,
					"cfg":          7.0,
					"sampler_name": "euler",
					"scheduler":    "normal",
					"denoise":      1.0,
					"model":        []interface{}{"4", 0},
					"positive":     []interface{}{"6", 0},
					"negative":     []interface{}{"7", 0},
					"latent_image": []interface{}{"5", 0},
				},
				"class_type": "KSampler",
			},
			// CheckpointLoader
			"4": map[string]interface{}{
				"inputs": map[string]interface{}{
					"ckpt_name": p.checkpoint,
				},
				"class_type": "CheckpointLoaderSimple",
			},
			// EmptyLatentImage
			"5": map[string]interface{}{
				"inputs": map[string]interface{}{
					"width":      512,
					"height":     512,
					"batch_size": 1,
				},
				"class_type": "EmptyLatentImage",
			},
			// Positive prompt
			"6": map[string]interface{}{
				"inputs": map[string]interface{}{
					"text": prompt,
					"clip": []interface{}{"4", 1},
				},
				"class_type": "CLIPTextEncode",
			},
			// Negative prompt
			"7": map[string]interface{}{
				"inputs": map[string]interface{}{
					"text": "bad quality, blurry, ugly",
					"clip": []interface{}{"4", 1},
				},
				"class_type": "CLIPTextEncode",
			},
			// VAEDecode
			"8": map[string]interface{}{
				"inputs": map[string]interface{}{
					"samples": []interface{}{"3", 0},
					"vae":     []interface{}{"4", 2},
				},
				"class_type": "VAEDecode",
			},
			// SaveImage
			"9": map[string]interface{}{
				"inputs": map[string]interface{}{
					"filename_prefix": "auleOS",
					"images":          []interface{}{"8", 0},
				},
				"class_type": "SaveImage",
			},
		},
	}
}
