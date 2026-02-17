package synapse

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// hostModuleName is the namespace for auleOS host functions exported to Wasm.
// Plugins call these via `(import "aule" "log")` in their Wasm binary.
const hostModuleName = "aule"

// InstantiateHostFunctions registers host-provided functions into the Wasm runtime.
// These functions are callable from any plugin module via the "aule" import namespace.
//
// Available host functions:
//   - aule.log(ptr, len)  — logs a message from the plugin to the kernel's slog
//   - aule.kv_get(key_ptr, key_len) → val_ptr  — (future) read from shared KV store
//   - aule.kv_set(key_ptr, key_len, val_ptr, val_len)  — (future) write to shared KV store
//
// For Phase 1, only `aule.log` is implemented. The KV store and HTTP functions
// will be added in Phase 3 when the plugin system matures.
func InstantiateHostFunctions(ctx context.Context, rt wazero.Runtime, logger *slog.Logger) error {
	_, err := rt.NewHostModuleBuilder(hostModuleName).
		// aule.log(ptr i32, len i32)
		// Logs a UTF-8 message from the plugin to the kernel's structured logger.
		// The plugin writes the message to its linear memory, then calls this
		// function with a pointer and length.
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, ptr, length uint32) {
			// Read the message from the plugin's linear memory
			if msg, ok := mod.Memory().Read(ptr, length); ok {
				logger.Info("synapse: plugin log", "message", string(msg))
			}
		}).
		WithParameterNames("ptr", "len").
		Export("log").

		// aule.metric(name_ptr, name_len, value f64)
		// Records a numeric metric from the plugin. Useful for performance monitoring.
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, namePtr, nameLen uint32, value float64) {
			if name, ok := mod.Memory().Read(namePtr, nameLen); ok {
				logger.Debug("synapse: plugin metric",
					"metric", string(name),
					"value", value,
				)
			}
		}).
		WithParameterNames("name_ptr", "name_len", "value").
		Export("metric").
		Instantiate(ctx)

	if err != nil {
		return fmt.Errorf("synapse: failed to instantiate host functions: %w", err)
	}

	logger.Debug("synapse: host functions registered", "module", hostModuleName)
	return nil
}
