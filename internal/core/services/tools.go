package services

import (
	"context"
	"fmt"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// NewGenerateImageTool creates the image generation tool
func NewGenerateImageTool(imageProvider domain.ImageProvider) *domain.Tool {
	return &domain.Tool{
		Name:        "generate_image",
		Description: "Generates an image from a text prompt using ComfyUI",
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
			
			// Generate image
			imageURL, err := imageProvider.GenerateImage(ctx, prompt)
			if err != nil {
				return nil, fmt.Errorf("image generation failed: %w", err)
			}
			
			return map[string]interface{}{
				"status": "success",
				"url":    imageURL,
				"prompt": prompt,
			}, nil
		},
	}
}
