package services

import (
	"context"
	"fmt"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// NewGenerateImageTool creates the image generation tool
func NewGenerateImageTool(lifecycle *WorkerLifecycle) *domain.Tool {
	return &domain.Tool{
		Name:        "generate_image",
		Description: "Queues an image generation job using ComfyUI and returns the job id",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "The text description of the image to generate",
				},
			},
			Required: []string{"prompt"},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			// Extract prompt
			promptRaw, ok := params["prompt"]
			if !ok {
				return nil, fmt.Errorf("missing required parameter: prompt")
			}

			prompt, ok := promptRaw.(string)
			if !ok {
				return nil, fmt.Errorf("prompt must be a string")
			}
			if lifecycle == nil {
				return nil, fmt.Errorf("worker lifecycle is not configured")
			}

			jobID, err := lifecycle.SubmitImageJob(ctx, prompt)
			if err != nil {
				return nil, fmt.Errorf("failed to queue image job: %w", err)
			}

			return map[string]interface{}{
				"status": "queued",
				"job_id": string(jobID),
				"prompt": prompt,
			}, nil
		},
	}
}

// NewGenerateTextTool creates the text generation tool
func NewGenerateTextTool(lifecycle *WorkerLifecycle) *domain.Tool {
	return &domain.Tool{
		Name:        "generate_text",
		Description: "Queues a text generation job and returns the job id",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "The prompt for text generation",
				},
			},
			Required: []string{"prompt"},
		},
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			promptRaw, ok := params["prompt"]
			if !ok {
				return nil, fmt.Errorf("missing required parameter: prompt")
			}

			prompt, ok := promptRaw.(string)
			if !ok {
				return nil, fmt.Errorf("prompt must be a string")
			}
			if lifecycle == nil {
				return nil, fmt.Errorf("worker lifecycle is not configured")
			}

			jobID, err := lifecycle.SubmitTextJob(ctx, prompt)
			if err != nil {
				return nil, fmt.Errorf("failed to queue text job: %w", err)
			}

			return map[string]interface{}{
				"status": "queued",
				"job_id": string(jobID),
				"prompt": prompt,
			}, nil
		},
	}
}
