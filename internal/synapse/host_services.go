package synapse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WorkerSpawner defines the interface for spawning Muscle (Docker) jobs.
// This decouples synapse from the concrete WorkerLifecycle implementation.
type WorkerSpawner interface {
	SpawnJob(ctx context.Context, spec domain.JobSpec) (domain.JobID, error)
}

// HostServices provides the bridge between Wasm plugins and Kernel capabilities.
type HostServices struct {
	logger  *slog.Logger
	spawner WorkerSpawner // The link to Muscle (Docker)
}

// NewHostServices creates a new HostServices instance.
func NewHostServices(logger *slog.Logger, spawner WorkerSpawner) *HostServices {
	return &HostServices{
		logger:  logger,
		spawner: spawner,
	}
}

// InstantiateHostFunctions registers the "aule" host module in the runtime.
func (h *HostServices) InstantiateHostFunctions(ctx context.Context, rt wazero.Runtime) error {
	_, err := rt.NewHostModuleBuilder("aule").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(h.fnLog), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{}).
		Export("log").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(h.fnDelegate), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).
		Export("delegate").
		Instantiate(ctx)

	return err
}

// fnLog: (level: i32, msg_ptr: i32, msg_len: i32)
func (h *HostServices) fnLog(ctx context.Context, mod api.Module, stack []uint64) {
	ptr := uint32(stack[0])
	size := uint32(stack[1])

	msg, err := readString(mod, ptr, size)
	if err != nil {
		h.logger.Error("synapse: failed to read log message from wasm", "error", err)
		return
	}

	h.logger.Info("WASMLOG: " + msg)
}

// fnDelegate: (spec_ptr: i32, spec_len: i32) -> job_id_ptr: i32
// Allows Wasm to request a heavy task (Muscle) execution.
func (h *HostServices) fnDelegate(ctx context.Context, mod api.Module, stack []uint64) {
	ptr := uint32(stack[0])
	size := uint32(stack[1])

	// 1. Read JSON spec from Wasm memory
	specJSON, err := readString(mod, ptr, size)
	if err != nil {
		h.logger.Error("synapse: failed to read delegate spec", "error", err)
		stack[0] = 0 // Return null pointer on error
		return
	}

	var spec domain.JobSpec
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		h.logger.Error("synapse: invalid delegate spec JSON", "error", err, "json", specJSON)
		stack[0] = 0
		return
	}

	// 2. Dispatch to Muscle (if spawner is configured)
	if h.spawner == nil {
		h.logger.Warn("synapse: delegate called but no spawner configured")
		stack[0] = 0
		return
	}

	jobID, err := h.spawner.SpawnJob(ctx, spec)
	if err != nil {
		h.logger.Error("synapse: failed to spawn job", "error", err)
		stack[0] = 0
		return
	}

	// 3. Write Job ID back to Wasm memory
	// Note: In a real implementation, we need a memory allocator in Wasm (malloc).
	// For simplicity here, we assume the Wasm module provides a buffer or we rely on the SDK.
	// This is a simplified "return pointer" logic. Real Wasm binding is more complex.
	// We'll log it for now and return 1 (success mock)
	h.logger.Info("synapse: delegated job", "job_id", jobID)
	stack[0] = 1 // Success
}

// Helper to read string from Wasm memory
func readString(mod api.Module, ptr, size uint32) (string, error) {
	bytes, ok := mod.Memory().Read(ptr, size)
	if !ok {
		return "", fmt.Errorf("memory read out of bounds: ptr=%d size=%d", ptr, size)
	}
	return string(bytes), nil
}
