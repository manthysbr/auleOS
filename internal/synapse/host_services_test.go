package synapse_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/synapse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── HTTPProxy Tests ─────────────────────────────────────────────────────

func TestHTTPProxyDenyList(t *testing.T) {
	proxy := synapse.NewHTTPProxy(testLogger())
	proxy.SetPluginPermissions("test-plugin", []string{"example.com"})

	ctx := context.Background()

	// SSRF protection: localhost is always denied
	for _, blocked := range []string{
		"http://localhost/foo",
		"http://127.0.0.1/bar",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/baz",
	} {
		_, _, err := proxy.Fetch(ctx, "test-plugin", "GET", blocked, "")
		require.Error(t, err, "should block %s", blocked)
		assert.Contains(t, err.Error(), "denied", "error for %s should mention denied", blocked)
	}
}

func TestHTTPProxyNoPermissions(t *testing.T) {
	proxy := synapse.NewHTTPProxy(testLogger())
	// Don't set any permissions for "no-perm-plugin"

	ctx := context.Background()
	_, _, err := proxy.Fetch(ctx, "no-perm-plugin", "GET", "https://example.com", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no HTTP permissions")
}

func TestHTTPProxyAllowlistEnforcement(t *testing.T) {
	proxy := synapse.NewHTTPProxy(testLogger())
	proxy.SetPluginPermissions("scoped-plugin", []string{"api.example.com"})

	ctx := context.Background()

	// Not in allowlist
	_, _, err := proxy.Fetch(ctx, "scoped-plugin", "GET", "https://evil.com/steal", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in plugin allowlist")
}

// ── MemKVStore Tests ────────────────────────────────────────────────────

func TestMemKVStoreNamespaceIsolation(t *testing.T) {
	store := synapse.NewMemKVStore()
	ctx := context.Background()

	// Plugin A writes
	require.NoError(t, store.Set(ctx, "plugin-a", "key1", []byte("value-a")))

	// Plugin B writes same key
	require.NoError(t, store.Set(ctx, "plugin-b", "key1", []byte("value-b")))

	// Plugin A reads its own value
	val, err := store.Get(ctx, "plugin-a", "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value-a"), val)

	// Plugin B reads its own value
	val, err = store.Get(ctx, "plugin-b", "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value-b"), val)

	// Plugin A can't read plugin-b's namespace (different key exists)
	_, err = store.Get(ctx, "plugin-a", "nonexistent")
	require.Error(t, err)
}

func TestMemKVStoreDelete(t *testing.T) {
	store := synapse.NewMemKVStore()
	ctx := context.Background()

	require.NoError(t, store.Set(ctx, "ns", "k", []byte("v")))
	require.NoError(t, store.Delete(ctx, "ns", "k"))

	_, err := store.Get(ctx, "ns", "k")
	require.Error(t, err)
}

func TestMemKVStoreReturnsCopy(t *testing.T) {
	store := synapse.NewMemKVStore()
	ctx := context.Background()

	require.NoError(t, store.Set(ctx, "ns", "k", []byte("original")))

	val, err := store.Get(ctx, "ns", "k")
	require.NoError(t, err)

	// Mutate the returned slice
	val[0] = 'X'

	// Original should be unchanged
	val2, err := store.Get(ctx, "ns", "k")
	require.NoError(t, err)
	assert.Equal(t, []byte("original"), val2, "store should return copies, not references")
}

// ── ConfigVault Tests ───────────────────────────────────────────────────

func TestConfigVaultResolve(t *testing.T) {
	vault := synapse.NewConfigVault(mockConfigFunc("sk-test-123", ""))
	ctx := context.Background()

	// Known key
	val, ok := vault.ResolveSecret(ctx, "llm_api_key")
	assert.True(t, ok)
	assert.Equal(t, "sk-test-123", val)

	// Unknown key — should not leak
	_, ok = vault.ResolveSecret(ctx, "nonexistent_key")
	assert.False(t, ok)

	// Case insensitive
	val, ok = vault.ResolveSecret(ctx, "LLM_API_KEY")
	assert.True(t, ok)
	assert.Equal(t, "sk-test-123", val)
}

func TestConfigVaultEmptyValue(t *testing.T) {
	vault := synapse.NewConfigVault(mockConfigFunc("", ""))
	ctx := context.Background()

	_, ok := vault.ResolveSecret(ctx, "llm_api_key")
	assert.False(t, ok, "empty values should return false")
}

// ── ExtractHost Tests ───────────────────────────────────────────────────

func TestExtractHost(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://api.openai.com/v1/chat", "api.openai.com"},
		{"http://localhost:8080/test", "localhost"},
		{"https://user:pass@example.com/path", "example.com"},
		{"ftp://files.example.com:21/pub", "files.example.com"},
		{"invalid-no-scheme", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := synapse.ExtractHost(tt.url)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func mockConfigFunc(llmAPIKey, imageAPIKey string) func() *domain.AppConfig {
	return func() *domain.AppConfig {
		return &domain.AppConfig{
			Providers: domain.ProviderConfig{
				LLM: domain.LLMProviderConfig{
					APIKey:   llmAPIKey,
					LocalURL: "http://localhost:11434",
				},
				Image: domain.ImageProviderConfig{
					APIKey: imageAPIKey,
				},
			},
		}
	}
}
