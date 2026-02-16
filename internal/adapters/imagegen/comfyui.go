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

// ComfyUIProvider implements domain.ImageProvider for local ComfyUI
type ComfyUIProvider struct {
	baseURL string
	client  *http.Client
}

func NewComfyUIProvider(baseURL string) *ComfyUIProvider {
	if baseURL == "" {
		baseURL = "http://localhost:8188"
	}
	return &ComfyUIProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// GenerateImage sends a text-to-image workflow to ComfyUI
// For simplicity, we use a minimal workflow with KSampler
// Returns a placeholder URL since full async implementation requires WebSocket
func (p *ComfyUIProvider) GenerateImage(ctx context.Context, prompt string) (string, error) {
	// Check if ComfyUI is running
	healthReq, _ := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/system_stats", nil)
	healthResp, err := p.client.Do(healthReq)
	if err != nil {
		return "", fmt.Errorf("comfyui unreachable (ensure docker-compose.comfyui.yml is running): %w", err)
	}
	defer healthResp.Body.Close()

	if healthResp.StatusCode != 200 {
		return "", fmt.Errorf("comfyui health check failed with status %d", healthResp.StatusCode)
	}

	// Simplified workflow payload (text2img with SD 1.5)
	// In production, this would be loaded from a JSON file
	workflow := map[string]interface{}{
		"prompt": map[string]interface{}{
			"3": map[string]interface{}{
				"class_type": "KSampler",
				"inputs": map[string]interface{}{
					"seed":   time.Now().Unix(),
					"steps":  20,
					"cfg":    7.0,
					"sampler_name": "euler",
					"scheduler":    "normal",
					"denoise":      1.0,
					"model":        []interface{}{"4", 0},
					"positive":     []interface{}{"6", 0},
					"negative":     []interface{}{"7", 0},
					"latent_image": []interface{}{"5", 0},
				},
			},
			"4": map[string]interface{}{
				"class_type": "CheckpointLoaderSimple",
				"inputs": map[string]interface{}{
					"ckpt_name": "v1-5-pruned-emaonly.safetensors",
				},
			},
			"5": map[string]interface{}{
				"class_type": "EmptyLatentImage",
				"inputs": map[string]interface{}{
					"width":  512,
					"height": 512,
					"batch_size": 1,
				},
			},
			"6": map[string]interface{}{
				"class_type": "CLIPTextEncode",
				"inputs": map[string]interface{}{
					"text": prompt,
					"clip":  []interface{}{"4", 1},
				},
			},
			"7": map[string]interface{}{
				"class_type": "CLIPTextEncode",
				"inputs": map[string]interface{}{
					"text": "ugly, bad quality, blurry",
					"clip": []interface{}{"4", 1},
				},
			},
			"8": map[string]interface{}{
				"class_type": "VAEDecode",
				"inputs": map[string]interface{}{
					"samples": []interface{}{"3", 0},
					"vae":     []interface{}{"4", 2},
				},
			},
			"9": map[string]interface{}{
				"class_type": "SaveImage",
				"inputs": map[string]interface{}{
					"filename_prefix": "aule_gen",
					"images":          []interface{}{"8", 0},
				},
			},
		},
	}

	payload, _ := json.Marshal(workflow)
	promptReq, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/prompt", bytes.NewReader(payload))
	promptReq.Header.Set("Content-Type", "application/json")

	promptResp, err := p.client.Do(promptReq)
	if err != nil {
		return "", fmt.Errorf("failed to submit prompt: %w", err)
	}
	defer promptResp.Body.Close()

	if promptResp.StatusCode != 200 {
		body, _ := io.ReadAll(promptResp.Body)
		return "", fmt.Errorf("comfyui prompt submission failed: %s", body)
	}

	// Parse response to get prompt ID
	var promptResult struct {
		PromptID string `json:"prompt_id"`
	}
	json.NewDecoder(promptResp.Body).Decode(&promptResult)

	// For M8, we return a placeholder since full implementation needs WebSocket client
	// In production, we'd poll /history/{prompt_id} or use WS to get the output filename
	// Then construct: http://localhost:8188/view?filename=aule_gen_00001_.png&subfolder=&type=output
	
	return fmt.Sprintf("http://localhost:8188/outputs/aule_gen_latest.png (job queued: %s)", promptResult.PromptID), nil
}
