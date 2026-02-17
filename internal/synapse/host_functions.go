package synapse

import (
	"context"
	"log/slog"

	"github.com/tetratelabs/wazero"
)

// hostModuleName is the namespace for auleOS host functions exported to Wasm.
// Plugins call these via `(import "aule" "log")` in their Wasm binary.
const hostModuleName = "aule"

// InstantiateHostFunctions registers host-provided functions into the Wasm runtime.
// This is a convenience wrapper that creates minimal HostServices (log + metric only)
// when the full host services layer isn't needed (e.g., tests, simple plugins).
//
// For production use with secrets, HTTP proxy, and KV store, use HostServices directly:
//
//	hs := synapse.NewHostServices(logger, vault, kvStore)
//	hs.InstantiateHostFunctions(ctx, rt)
func InstantiateHostFunctions(ctx context.Context, rt wazero.Runtime, logger *slog.Logger) error {
	// Create minimal host services — no vault, no KV, just log + metric
	hs := NewHostServices(logger, nil, nil)
	return hs.InstantiateHostFunctions(ctx, rt)
}
