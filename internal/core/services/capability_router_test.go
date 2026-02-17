package services

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/manthysbr/auleOS/internal/synapse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityRouterDefaults(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	router := NewCapabilityRouter(logger, nil)

	// GPU tasks → Muscle
	assert.Equal(t, RuntimeMuscle, router.Resolve("image.generate"))
	assert.Equal(t, RuntimeMuscle, router.Resolve("text.generate"))
	assert.Equal(t, RuntimeMuscle, router.Resolve("video.transcode"))

	// Logic tasks → Synapse
	assert.Equal(t, RuntimeSynapse, router.Resolve("prompt.enhance"))
	assert.Equal(t, RuntimeSynapse, router.Resolve("json.transform"))
	assert.Equal(t, RuntimeSynapse, router.Resolve("data.validate"))

	// Unknown → defaults to Muscle
	assert.Equal(t, RuntimeMuscle, router.Resolve("something.unknown"))
}

func TestCapabilityRouterRegisterRoute(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	router := NewCapabilityRouter(logger, nil)

	// Register custom route
	router.RegisterRoute("custom.task", CapabilityRoute{
		Runtime:     RuntimeSynapse,
		Description: "Custom synapse task",
	})

	assert.Equal(t, RuntimeSynapse, router.Resolve("custom.task"))

	// Override existing route
	router.RegisterRoute("image.generate", CapabilityRoute{
		Runtime:     RuntimeSynapse,
		Description: "Override for testing",
	})
	assert.Equal(t, RuntimeSynapse, router.Resolve("image.generate"))
}

func TestCapabilityRouterWithSynapse(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	wasmRT, err := synapse.NewRuntime(ctx, logger)
	require.NoError(t, err)
	defer wasmRT.Close(ctx)

	// Load a test plugin
	noopWasm := []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x04, 0x01, 0x60, 0x00, 0x00,
		0x03, 0x02, 0x01, 0x00,
		0x05, 0x03, 0x01, 0x00, 0x01,
		0x07, 0x13, 0x02,
		0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00,
		0x06, 0x5f, 0x73, 0x74, 0x61, 0x72, 0x74, 0x00, 0x00,
		0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b,
	}

	_, err = wasmRT.LoadPlugin(ctx, "my-transform", noopWasm, synapse.PluginMeta{
		Name:     "my-transform",
		Version:  "0.1.0",
		ToolName: "my_transform",
	})
	require.NoError(t, err)

	router := NewCapabilityRouter(logger, wasmRT)

	// Plugin name resolves to Synapse even without explicit route
	assert.Equal(t, RuntimeSynapse, router.Resolve("my-transform"))
}

func TestCapabilityRouterStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	router := NewCapabilityRouter(logger, nil)

	stats := router.Stats()
	assert.Greater(t, stats["total"], 0)
	assert.Greater(t, stats["muscle"], 0)
	assert.Greater(t, stats["synapse"], 0)
	assert.Equal(t, stats["total"], stats["muscle"]+stats["synapse"])
}

func TestCapabilityRouterListRoutes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	router := NewCapabilityRouter(logger, nil)

	routes := router.ListRoutes()
	assert.Contains(t, routes, "image.generate")
	assert.Contains(t, routes, "prompt.enhance")
	assert.Equal(t, RuntimeMuscle, routes["image.generate"].Runtime)
	assert.Equal(t, RuntimeSynapse, routes["prompt.enhance"].Runtime)
}
