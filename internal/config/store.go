package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// SettingsRepository is the minimal DB interface for settings persistence.
type SettingsRepository interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SaveSetting(ctx context.Context, key string, value string) error
}

// OnChangeFunc is called when settings are updated.
type OnChangeFunc func(cfg *domain.AppConfig)

// SettingsStore manages persistent settings with encrypted secrets.
// Inspired by Gitea/Grafana settings architecture: categories stored as JSON,
// secrets encrypted at rest, masked on read.
type SettingsStore struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	secret   *SecretKey
	repo     SettingsRepository
	config   *domain.AppConfig
	onChange []OnChangeFunc
}

// NewSettingsStore creates a store that loads/saves settings from DB with AES-256-GCM encryption.
func NewSettingsStore(logger *slog.Logger, repo SettingsRepository, secret *SecretKey) (*SettingsStore, error) {
	store := &SettingsStore{
		logger: logger,
		secret: secret,
		repo:   repo,
	}

	ctx := context.Background()
	cfg, err := store.loadFromDB(ctx)
	if err != nil {
		logger.Warn("no saved settings found, using defaults", "error", err)
		cfg = domain.DefaultConfig()
		if err := store.saveToDB(ctx, cfg); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
	}

	store.config = cfg
	return store, nil
}

// OnChange registers a callback for when settings are updated.
// Used by lifecycle to hot-reload providers.
func (s *SettingsStore) OnChange(fn OnChangeFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = append(s.onChange, fn)
}

// GetConfig returns the current config with decrypted secrets.
func (s *SettingsStore) GetConfig() *domain.AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := *s.config
	cp.Providers.LLM = s.config.Providers.LLM
	cp.Providers.Image = s.config.Providers.Image
	return &cp
}

// GetMaskedConfig returns config safe for API response (secrets masked).
func (s *SettingsStore) GetMaskedConfig() *domain.AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := *s.config
	cp.Providers.LLM = s.config.Providers.LLM
	cp.Providers.LLM.APIKey = MaskSecret(s.config.Providers.LLM.APIKey)
	cp.Providers.Image = s.config.Providers.Image
	cp.Providers.Image.APIKey = MaskSecret(s.config.Providers.Image.APIKey)
	return &cp
}

// UpdateConfig validates, encrypts secrets, persists, and triggers onChange callbacks.
// Smart merge: if apiKey is empty or masked, keeps existing key.
func (s *SettingsStore) UpdateConfig(ctx context.Context, update *domain.AppConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Merge: preserve existing secrets if update sends empty/masked values
	if update.Providers.LLM.APIKey == "" || isMasked(update.Providers.LLM.APIKey) {
		update.Providers.LLM.APIKey = s.config.Providers.LLM.APIKey
	}
	if update.Providers.Image.APIKey == "" || isMasked(update.Providers.Image.APIKey) {
		update.Providers.Image.APIKey = s.config.Providers.Image.APIKey
	}

	// Validate required fields for remote mode
	if update.Providers.LLM.Mode == "remote" {
		if update.Providers.LLM.RemoteURL == "" {
			return fmt.Errorf("LLM remote_url is required when mode=remote")
		}
		if update.Providers.LLM.APIKey == "" {
			return fmt.Errorf("LLM api_key is required when mode=remote")
		}
	}
	if update.Providers.Image.Mode == "remote" {
		if update.Providers.Image.RemoteURL == "" {
			return fmt.Errorf("Image remote_url is required when mode=remote")
		}
	}

	// Defaults
	if update.Providers.LLM.Mode == "" {
		update.Providers.LLM.Mode = "local"
	}
	if update.Providers.Image.Mode == "" {
		update.Providers.Image.Mode = "local"
	}

	if err := s.saveToDB(ctx, update); err != nil {
		return err
	}

	s.config = update
	s.logger.Info("settings updated",
		"llm_mode", update.Providers.LLM.Mode,
		"image_mode", update.Providers.Image.Mode,
	)

	// Trigger callbacks (outside lock would deadlock if callback reads config)
	for _, fn := range s.onChange {
		fn(update)
	}

	return nil
}

func (s *SettingsStore) loadFromDB(ctx context.Context) (*domain.AppConfig, error) {
	raw, err := s.repo.GetSetting(ctx, "app_config")
	if err != nil {
		return nil, err
	}

	var stored storedConfig
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, fmt.Errorf("unmarshal settings: %w", err)
	}

	cfg := &domain.AppConfig{
		Providers: domain.ProviderConfig{
			LLM: domain.LLMProviderConfig{
				Mode:         stored.LLM.Mode,
				LocalURL:     stored.LLM.LocalURL,
				RemoteURL:    stored.LLM.RemoteURL,
				DefaultModel: stored.LLM.DefaultModel,
			},
			Image: domain.ImageProviderConfig{
				Mode:         stored.Image.Mode,
				LocalURL:     stored.Image.LocalURL,
				RemoteURL:    stored.Image.RemoteURL,
				DefaultModel: stored.Image.DefaultModel,
			},
		},
	}

	// Decrypt secrets
	if stored.LLM.EncryptedAPIKey != "" {
		key, err := s.secret.Decrypt(stored.LLM.EncryptedAPIKey)
		if err != nil {
			s.logger.Warn("failed to decrypt LLM API key", "error", err)
		} else {
			cfg.Providers.LLM.APIKey = key
		}
	}

	if stored.Image.EncryptedAPIKey != "" {
		key, err := s.secret.Decrypt(stored.Image.EncryptedAPIKey)
		if err != nil {
			s.logger.Warn("failed to decrypt Image API key", "error", err)
		} else {
			cfg.Providers.Image.APIKey = key
		}
	}

	return cfg, nil
}

func (s *SettingsStore) saveToDB(ctx context.Context, cfg *domain.AppConfig) error {
	stored := storedConfig{
		LLM: storedProviderConfig{
			Mode:         cfg.Providers.LLM.Mode,
			LocalURL:     cfg.Providers.LLM.LocalURL,
			RemoteURL:    cfg.Providers.LLM.RemoteURL,
			DefaultModel: cfg.Providers.LLM.DefaultModel,
		},
		Image: storedProviderConfig{
			Mode:         cfg.Providers.Image.Mode,
			LocalURL:     cfg.Providers.Image.LocalURL,
			RemoteURL:    cfg.Providers.Image.RemoteURL,
			DefaultModel: cfg.Providers.Image.DefaultModel,
		},
	}

	if cfg.Providers.LLM.APIKey != "" {
		enc, err := s.secret.Encrypt(cfg.Providers.LLM.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt LLM API key: %w", err)
		}
		stored.LLM.EncryptedAPIKey = enc
	}

	if cfg.Providers.Image.APIKey != "" {
		enc, err := s.secret.Encrypt(cfg.Providers.Image.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt Image API key: %w", err)
		}
		stored.Image.EncryptedAPIKey = enc
	}

	raw, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return s.repo.SaveSetting(ctx, "app_config", string(raw))
}

// storedConfig is the DB representation with encrypted fields
type storedConfig struct {
	LLM   storedProviderConfig `json:"llm"`
	Image storedProviderConfig `json:"image"`
}

type storedProviderConfig struct {
	Mode            string `json:"mode"`
	LocalURL        string `json:"local_url"`
	RemoteURL       string `json:"remote_url"`
	EncryptedAPIKey string `json:"encrypted_api_key,omitempty"`
	DefaultModel    string `json:"default_model"`
}

func isMasked(s string) bool {
	return len(s) >= 4 && s[:4] == "****"
}
