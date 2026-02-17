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
		Description: "Queues an image generation job using ComfyUI and returns the job id. The result will be delivered asynchronously to the conversation.",
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

			// Quick health check: is the image provider reachable?
			if err := lifecycle.TestImageProvider(ctx); err != nil {
				return map[string]interface{}{
					"status":  "unavailable",
					"error":   fmt.Sprintf("Image generation service is not available: %v", err),
					"message": "The image generation backend (ComfyUI) is not running. Please start it or configure a remote provider in Settings.",
				}, nil
			}

			// Extract conversation ID so the job result can be pushed back to the chat
			convID, _ := ctx.Value(ctxKeyConversationID).(domain.ConversationID)

			jobID, err := lifecycle.SubmitImageJobWithConv(ctx, prompt, string(convID))
			if err != nil {
				return nil, fmt.Errorf("failed to queue image job: %w", err)
			}

			return map[string]interface{}{
				"status":  "queued",
				"job_id":  string(jobID),
				"prompt":  prompt,
				"message": "Image is being generated asynchronously. The result will appear in this chat when ready.",
			}, nil
		},
	}
}

// NewGenerateTextTool creates the text generation tool
func NewGenerateTextTool(lifecycle *WorkerLifecycle) *domain.Tool {
	return &domain.Tool{
		Name:        "generate_text",
		Description: "Queues a text generation job and returns the job id. The result will be delivered asynchronously to the conversation.",
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

			// Extract conversation ID so the job result can be pushed back to the chat
			convID, _ := ctx.Value(ctxKeyConversationID).(domain.ConversationID)

			jobID, err := lifecycle.SubmitTextJobWithConv(ctx, prompt, string(convID))
			if err != nil {
				return nil, fmt.Errorf("failed to queue text job: %w", err)
			}

			return map[string]interface{}{
				"status":  "queued",
				"job_id":  string(jobID),
				"prompt":  prompt,
				"message": "Text is being generated asynchronously. The result will appear in this chat when ready.",
			}, nil
		},
	}
}
