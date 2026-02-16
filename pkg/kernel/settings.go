package kernel

import (
	"context"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// GetSettings implements StrictServerInterface.
func (s *Server) GetSettings(ctx context.Context, _ GetSettingsRequestObject) (GetSettingsResponseObject, error) {
	cfg := s.settings.GetMaskedConfig()
	return GetSettings200JSONResponse(domainCfgToAPI(cfg)), nil
}

// UpdateSettings implements StrictServerInterface.
func (s *Server) UpdateSettings(ctx context.Context, request UpdateSettingsRequestObject) (UpdateSettingsResponseObject, error) {
	if request.Body == nil {
		msg := "empty request body"
		return UpdateSettings400JSONResponse{Error: &msg}, nil
	}

	// Convert API config to domain config
	update := apiCfgToDomain(request.Body)

	if err := s.settings.UpdateConfig(ctx, update); err != nil {
		msg := err.Error()
		return UpdateSettings400JSONResponse{Error: &msg}, nil
	}

	cfg := s.settings.GetMaskedConfig()
	return UpdateSettings200JSONResponse(domainCfgToAPI(cfg)), nil
}

// TestConnection implements StrictServerInterface.
func (s *Server) TestConnection(ctx context.Context, request TestConnectionRequestObject) (TestConnectionResponseObject, error) {
	if request.Body == nil {
		return nil, nil
	}

	provider := string(request.Body.Provider)
	cfg := s.settings.GetConfig()

	result := ConnectionTestResult{
		Provider: &provider,
	}

	switch provider {
	case "llm":
		mode := cfg.Providers.LLM.Mode
		url := cfg.Providers.LLM.LocalURL
		if mode == "remote" {
			url = cfg.Providers.LLM.RemoteURL
		}
		model := cfg.Providers.LLM.DefaultModel
		result.Mode = &mode
		result.Url = &url
		result.Model = &model

		_, err := s.lifecycle.TestLLM(ctx)
		if err != nil {
			status := ConnectionTestResultStatus("error")
			msg := err.Error()
			result.Status = &status
			result.Message = &msg
		} else {
			status := ConnectionTestResultStatus("ok")
			msg := "Connection successful"
			result.Status = &status
			result.Message = &msg
		}
	case "image":
		mode := cfg.Providers.Image.Mode
		url := cfg.Providers.Image.LocalURL
		if mode == "remote" {
			url = cfg.Providers.Image.RemoteURL
		}
		result.Mode = &mode
		result.Url = &url

		status := ConnectionTestResultStatus("ok")
		msg := "Image provider configured (test requires actual generation)"
		result.Status = &status
		result.Message = &msg
	}

	return TestConnection200JSONResponse(result), nil
}

// --- Config mapping helpers ---

func domainCfgToAPI(cfg *domain.AppConfig) AppConfig {
	if cfg == nil {
		return AppConfig{}
	}

	llmMode := cfg.Providers.LLM.Mode
	llmLocalURL := cfg.Providers.LLM.LocalURL
	llmRemoteURL := cfg.Providers.LLM.RemoteURL
	llmAPIKey := cfg.Providers.LLM.APIKey
	llmModel := cfg.Providers.LLM.DefaultModel
	imgMode := cfg.Providers.Image.Mode
	imgLocalURL := cfg.Providers.Image.LocalURL
	imgRemoteURL := cfg.Providers.Image.RemoteURL
	imgAPIKey := cfg.Providers.Image.APIKey
	imgModel := cfg.Providers.Image.DefaultModel

	llmModePtr := ProviderConfigMode(llmMode)
	imgModePtr := ProviderConfigMode(imgMode)

	return AppConfig{
		Providers: &struct {
			Image *ProviderConfig `json:"image,omitempty"`
			Llm   *ProviderConfig `json:"llm,omitempty"`
		}{
			Llm: &ProviderConfig{
				Mode:         &llmModePtr,
				LocalUrl:     &llmLocalURL,
				RemoteUrl:    &llmRemoteURL,
				ApiKey:       &llmAPIKey,
				DefaultModel: &llmModel,
			},
			Image: &ProviderConfig{
				Mode:         &imgModePtr,
				LocalUrl:     &imgLocalURL,
				RemoteUrl:    &imgRemoteURL,
				ApiKey:       &imgAPIKey,
				DefaultModel: &imgModel,
			},
		},
	}
}

func apiCfgToDomain(api *AppConfig) *domain.AppConfig {
	cfg := domain.DefaultConfig()

	if api.Providers != nil {
		if api.Providers.Llm != nil {
			llm := api.Providers.Llm
			if llm.Mode != nil {
				cfg.Providers.LLM.Mode = string(*llm.Mode)
			}
			if llm.LocalUrl != nil {
				cfg.Providers.LLM.LocalURL = *llm.LocalUrl
			}
			if llm.RemoteUrl != nil {
				cfg.Providers.LLM.RemoteURL = *llm.RemoteUrl
			}
			if llm.ApiKey != nil {
				cfg.Providers.LLM.APIKey = *llm.ApiKey
			}
			if llm.DefaultModel != nil {
				cfg.Providers.LLM.DefaultModel = *llm.DefaultModel
			}
		}
		if api.Providers.Image != nil {
			img := api.Providers.Image
			if img.Mode != nil {
				cfg.Providers.Image.Mode = string(*img.Mode)
			}
			if img.LocalUrl != nil {
				cfg.Providers.Image.LocalURL = *img.LocalUrl
			}
			if img.RemoteUrl != nil {
				cfg.Providers.Image.RemoteURL = *img.RemoteUrl
			}
			if img.ApiKey != nil {
				cfg.Providers.Image.APIKey = *img.ApiKey
			}
			if img.DefaultModel != nil {
				cfg.Providers.Image.DefaultModel = *img.DefaultModel
			}
		}
	}

	return cfg
}
