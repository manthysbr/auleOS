package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

type AgentService struct {
	logger        *slog.Logger
	llm           domain.LLMProvider
	imageProvider domain.ImageProvider
	lifecycle     *WorkerLifecycle
}

func NewAgentService(logger *slog.Logger, llm domain.LLMProvider, imageProvider domain.ImageProvider, lifecycle *WorkerLifecycle) *AgentService {
	return &AgentService{
		logger:        logger,
		llm:           llm,
		imageProvider: imageProvider,
		lifecycle:     lifecycle,
	}
}

type ChatResponse struct {
	Response string
	Thought  string
	ToolCall *domain.ToolCall
}

func (s *AgentService) Chat(ctx context.Context, message string, model string) (*ChatResponse, error) {
	s.logger.Info("processing agent chat", "message", message)

	if model == "" {
		model = "llama3.2" // Updated default
	}

	// 1. Construct System Prompt (The "Identity")
	systemPrompt := `You are auleOS, an autonomous operating system agent. 
You can execute code by generating a JSON tool call block.
Result format must be valid JSON if calling a tool.
Available Tools:
- submit_job(image, command): Runs a container.
- generate_image(prompt): Generates an image using ComfyUI.
`
	fullPrompt := fmt.Sprintf("%s\n\nUser: %s\n\nAssistant:", systemPrompt, message)

	// 2. Call LLM
	rawResponse, err := s.llm.GenerateText(ctx, fullPrompt)
	if err != nil {
		s.logger.Error("llm generation failed", "error", err)
		return &ChatResponse{
			Response: "I'm having trouble connecting to my brain (Ollama). Please ensure it's running!",
			Thought:  "Connection refused to localhost:11434",
		}, nil
	}

	// 3. Simple Heuristic Parse (For M8)
	// If the LLM output explicitly asks to generate an image
	lowerResp := strings.ToLower(rawResponse)
	s.logger.Info("checking for image generation", "response_snippet", rawResponse[:min(100, len(rawResponse))])
	
	if strings.Contains(lowerResp, "generate_image") || (strings.Contains(lowerResp, "image") && strings.Contains(lowerResp, "generate")) {
		s.logger.Info("image generation detected, calling provider", "prompt", message)
		// Mock parsing the prompt from the response
		// In a real agent, we parse the JSON tool call.
		// Here we just take the user message as the prompt for simplicity if the LLM agrees.
		imgURL, err := s.imageProvider.GenerateImage(ctx, message)
		if err != nil {
			s.logger.Error("image generation failed", "error", err)
			return &ChatResponse{
				Response: fmt.Sprintf("Failed to generate image: %v", err),
				Thought:  "Image generation error",
			}, nil
		}
		s.logger.Info("image generated successfully", "url", imgURL)
		return &ChatResponse{
			Response: fmt.Sprintf("Here is your image: %s", imgURL),
			Thought:  "Invoked generate_image tool",
			ToolCall: &domain.ToolCall{
				Name: "generate_image",
				Args: map[string]interface{}{"url": imgURL},
			},
		}, nil
	}
	
	return &ChatResponse{
		Response: rawResponse,
		Thought:  "Processed via " + model,
	}, nil
}
