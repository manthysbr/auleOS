package synapse

import (
	"context"
	"strings"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// ConfigVault adapts the auleOS SettingsStore into a SecretVault for Wasm plugins.
// It resolves logical secret keys to their plaintext values from the app config.
//
// Supported keys:
//   - "llm_api_key"     → Providers.LLM.APIKey
//   - "image_api_key"   → Providers.Image.APIKey
//   - "llm_base_url"    → Providers.LLM.LocalURL
//   - "image_base_url"  → Providers.Image.LocalURL
//
// This ensures plugins never see raw encrypted values — the SettingsStore
// handles decryption transparently. The vault only exposes semantic keys,
// never internal config paths.
type ConfigVault struct {
	getConfig func() *domain.AppConfig
}

// NewConfigVault creates a vault backed by the settings store's GetConfig method.
// The function is passed (not the store directly) to avoid tight coupling.
func NewConfigVault(getConfig func() *domain.AppConfig) *ConfigVault {
	return &ConfigVault{getConfig: getConfig}
}

// ResolveSecret maps a logical key to a config value.
// Returns ("", false) for unknown keys — plugins can't enumerate available secrets.
func (v *ConfigVault) ResolveSecret(_ context.Context, key string) (string, bool) {
	cfg := v.getConfig()
	if cfg == nil {
		return "", false
	}

	key = strings.ToLower(strings.TrimSpace(key))

	switch key {
	case "llm_api_key":
		if cfg.Providers.LLM.APIKey != "" {
			return cfg.Providers.LLM.APIKey, true
		}
	case "image_api_key":
		if cfg.Providers.Image.APIKey != "" {
			return cfg.Providers.Image.APIKey, true
		}
	case "llm_base_url":
		if cfg.Providers.LLM.LocalURL != "" {
			return cfg.Providers.LLM.LocalURL, true
		}
	case "image_base_url":
		if cfg.Providers.Image.LocalURL != "" {
			return cfg.Providers.Image.LocalURL, true
		}
	}

	return "", false
}
