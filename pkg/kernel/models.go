package kernel

import (
	"context"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// ListModels returns the current model catalog.
func (s *Server) ListModels(_ context.Context, _ ListModelsRequestObject) (ListModelsResponseObject, error) {
	catalog := s.modelRouter.GetCatalog()
	result := make(ListModels200JSONResponse, 0, len(catalog))
	for _, m := range catalog {
		result = append(result, domainModelToAPI(m))
	}
	return result, nil
}

// DiscoverModels runs model discovery against Ollama and/or LiteLLM.
func (s *Server) DiscoverModels(ctx context.Context, req DiscoverModelsRequestObject) (DiscoverModelsResponseObject, error) {
	var allModels []domain.ModelSpec

	// Discover from Ollama
	ollamaURL := ""
	if req.Body != nil && req.Body.OllamaUrl != nil {
		ollamaURL = *req.Body.OllamaUrl
	}
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if discovered, err := s.discovery.DiscoverOllama(ctx, ollamaURL); err == nil {
		allModels = append(allModels, discovered...)
	}

	// Discover from LiteLLM
	if req.Body != nil && req.Body.LitellmUrl != nil && *req.Body.LitellmUrl != "" {
		apiKey := ""
		if req.Body.LitellmApiKey != nil {
			apiKey = *req.Body.LitellmApiKey
		}
		if discovered, err := s.discovery.DiscoverLiteLLM(ctx, *req.Body.LitellmUrl, apiKey); err == nil {
			allModels = append(allModels, discovered...)
		}
	}

	// Update router catalog
	if len(allModels) > 0 {
		s.modelRouter.SetCatalog(allModels)
	}

	result := make(DiscoverModels200JSONResponse, 0, len(allModels))
	for _, m := range allModels {
		result = append(result, domainModelToAPI(m))
	}
	return result, nil
}

func domainModelToAPI(m domain.ModelSpec) ModelSpec {
	role := ModelSpecRole(m.Role)
	return ModelSpec{
		Id:       strPtr(m.ID),
		Name:     strPtr(m.Name),
		Provider: strPtr(m.Provider),
		Role:     &role,
		Size:     strPtr(m.Size),
		BaseUrl:  strPtr(m.BaseURL),
		IsLocal:  &m.IsLocal,
	}
}
