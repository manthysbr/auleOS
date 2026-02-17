// Package synapse — forge.go implements the Tool Forge: an LLM-driven
// code generator that turns natural language descriptions into compiled
// Wasm plugins, hot-loaded into the Synapse runtime.
//
// Flow:
//  1. User describes a tool in plain text ("I need a tool that converts CSV to JSON")
//  2. The LLM generates a Go WASI program (stdin JSON → stdout JSON)
//  3. Forge compiles it with `GOOS=wasip1 GOARCH=wasm go build`
//  4. The resulting .wasm is hot-loaded into the Synapse Runtime
//  5. The tool is immediately available in the ToolRegistry
//
// Security: All generated code runs inside the Wasm sandbox — no filesystem,
// no network, no syscalls beyond stdin/stdout. Even malicious code is contained.
package synapse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
)

// LLMProvider is the minimal interface the Forge needs to generate code.
// Compatible with services.ModelRouter.GenerateText(ctx, prompt, model).
type LLMProvider interface {
	GenerateText(ctx context.Context, prompt string, modelID string) (string, error)
}

// ForgeResult contains the output of a forge operation.
type ForgeResult struct {
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	WasmPath    string `json:"wasm_path"`
	WasmSize    int64  `json:"wasm_size_bytes"`
	SourceHash  string `json:"source_hash"`
	CompileTime string `json:"compile_time"`
	Status      string `json:"status"` // "ok" or "error"
	Error       string `json:"error,omitempty"`
}

// Forge generates, compiles, and loads Wasm tools from natural language.
type Forge struct {
	logger    *slog.Logger
	llm       LLMProvider
	model     string
	runtime   *Runtime
	registry  *domain.ToolRegistry
	pluginDir string
}

// NewForge creates a Tool Forge.
// - llm: the LLM provider used to generate Go code
// - model: which model to use (e.g. "qwen2.5:latest")
// - runtime: Synapse Wasm runtime for hot-loading
// - registry: ToolRegistry where the new tool will be registered
// - pluginDir: directory to persist .wasm and source files
func NewForge(
	logger *slog.Logger,
	llm LLMProvider,
	model string,
	runtime *Runtime,
	registry *domain.ToolRegistry,
	pluginDir string,
) *Forge {
	return &Forge{
		logger:    logger,
		llm:       llm,
		model:     model,
		runtime:   runtime,
		registry:  registry,
		pluginDir: pluginDir,
	}
}

// Create generates a tool from a natural language description.
// It asks the LLM to produce Go code, compiles it to Wasm, and hot-loads it.
func (f *Forge) Create(ctx context.Context, toolName, description string) (*ForgeResult, error) {
	start := time.Now()

	// Sanitize tool name
	toolName = sanitizeToolName(toolName)
	if toolName == "" {
		return nil, fmt.Errorf("forge: tool name cannot be empty")
	}

	f.logger.Info("forge: creating tool", "name", toolName, "description", description[:min(80, len(description))])

	// Step 1: Ask LLM to generate Go code
	goSource, err := f.generateCode(ctx, toolName, description)
	if err != nil {
		return &ForgeResult{
			ToolName: toolName,
			Status:   "error",
			Error:    fmt.Sprintf("code generation failed: %v", err),
		}, err
	}

	// Step 2: Write source to disk
	toolDir := filepath.Join(f.pluginDir, "forge", toolName)
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		return nil, fmt.Errorf("forge: failed to create tool dir: %w", err)
	}

	sourcePath := filepath.Join(toolDir, "main.go")
	if err := os.WriteFile(sourcePath, []byte(goSource), 0644); err != nil {
		return nil, fmt.Errorf("forge: failed to write source: %w", err)
	}

	// Write go.mod for the tool
	goMod := fmt.Sprintf("module %s\n\ngo 1.21\n", toolName)
	if err := os.WriteFile(filepath.Join(toolDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return nil, fmt.Errorf("forge: failed to write go.mod: %w", err)
	}

	// Step 3: Compile to Wasm
	wasmPath := filepath.Join(f.pluginDir, toolName+".wasm")
	if err := f.compile(ctx, toolDir, wasmPath); err != nil {
		return &ForgeResult{
			ToolName: toolName,
			Status:   "error",
			Error:    fmt.Sprintf("compilation failed: %v", err),
		}, err
	}

	// Step 4: Hot-load into Synapse
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("forge: failed to read compiled wasm: %w", err)
	}

	meta := PluginMeta{
		Name:        toolName,
		Version:     "1.0.0",
		Description: description,
		ToolName:    toolName,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input for the tool",
				},
			},
			Required: []string{"input"},
		},
	}

	// Parse parameters from LLM-generated code comments if possible
	if params := extractParamsFromSource(goSource); params != nil {
		meta.Parameters = *params
	}

	plugin, err := f.runtime.LoadPlugin(ctx, toolName, wasmBytes, meta)
	if err != nil {
		return &ForgeResult{
			ToolName: toolName,
			Status:   "error",
			Error:    fmt.Sprintf("wasm load failed: %v", err),
		}, err
	}

	// Step 5: Register in ToolRegistry
	tool := plugin.AsTool()
	tool.ExecutionType = domain.ExecWasm
	if err := f.registry.Register(tool); err != nil {
		return nil, fmt.Errorf("forge: failed to register tool: %w", err)
	}

	// Step 6: Update manifest
	if err := f.updateManifest(toolName, description); err != nil {
		f.logger.Warn("forge: failed to update manifest (non-fatal)", "error", err)
	}

	stat, _ := os.Stat(wasmPath)
	hash := sha256.Sum256([]byte(goSource))

	result := &ForgeResult{
		ToolName:    toolName,
		Description: description,
		WasmPath:    wasmPath,
		WasmSize:    stat.Size(),
		SourceHash:  hex.EncodeToString(hash[:8]),
		CompileTime: time.Since(start).Round(time.Millisecond).String(),
		Status:      "ok",
	}

	f.logger.Info("forge: tool created successfully",
		"name", toolName,
		"wasm_size", result.WasmSize,
		"compile_time", result.CompileTime,
	)

	return result, nil
}

// generateCode asks the LLM to produce a Go WASI program.
func (f *Forge) generateCode(ctx context.Context, toolName, description string) (string, error) {
	prompt := fmt.Sprintf(`You are a Go code generator for the auleOS Tool Forge.
Generate a complete, compilable Go program that implements the following tool.

TOOL NAME: %s
DESCRIPTION: %s

REQUIREMENTS:
1. Package must be "main" with a main() function
2. Read JSON input from stdin using io.ReadAll(os.Stdin)
3. Parse input as map[string]interface{} using encoding/json
4. Process the input according to the tool description
5. Write JSON output to stdout using fmt.Print(string(jsonBytes))
6. Use ONLY Go standard library (no external dependencies)
7. Handle errors gracefully - return {"error": "message"} on failure
8. The program will be compiled with GOOS=wasip1 GOARCH=wasm

OUTPUT FORMAT:
Return ONLY the Go source code, no markdown fences, no explanation.
Start with "package main" and end with the closing brace of main().

Add a comment at the top describing the parameters in this exact format:
// @params {"type":"object","properties":{"input_field":{"type":"string","description":"desc"}},"required":["input_field"]}

EXAMPLE TEMPLATE:
package main

// @params {"type":"object","properties":{"text":{"type":"string","description":"The text to process"}},"required":["text"]}

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Print(`+"`"+`{"error":"failed to read input"}`+"`"+`)
		return
	}

	var params map[string]interface{}
	if err := json.Unmarshal(input, &params); err != nil {
		fmt.Print(`+"`"+`{"error":"invalid JSON input"}`+"`"+`)
		return
	}

	// Process params here...
	text, _ := params["text"].(string)

	result := map[string]interface{}{
		"result": text,
		"status": "ok",
	}

	output, _ := json.Marshal(result)
	fmt.Print(string(output))
}`, toolName, description)

	response, err := f.llm.GenerateText(ctx, prompt, f.model)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// Clean up response — strip markdown fences if present
	code := cleanCodeResponse(response)
	if !strings.HasPrefix(strings.TrimSpace(code), "package main") {
		return "", fmt.Errorf("LLM did not produce valid Go code (missing 'package main')")
	}

	return code, nil
}

// compile runs `go build` with GOOS=wasip1 GOARCH=wasm.
func (f *Forge) compile(ctx context.Context, sourceDir, outputPath string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, ".")
	cmd.Dir = sourceDir
	cmd.Env = append(os.Environ(),
		"GOOS=wasip1",
		"GOARCH=wasm",
		"CGO_ENABLED=0",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build failed: %s\n%s", err, string(output))
	}

	f.logger.Debug("forge: compilation successful", "output", outputPath)
	return nil
}

// updateManifest adds the new tool to plugins.json.
func (f *Forge) updateManifest(toolName, description string) error {
	manifestPath := filepath.Join(f.pluginDir, ManifestFile)

	var manifest PluginManifest
	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}

	// Remove existing entry with same name
	filtered := make([]PluginEntry, 0, len(manifest.Plugins))
	for _, p := range manifest.Plugins {
		if p.Name != toolName {
			filtered = append(filtered, p)
		}
	}

	filtered = append(filtered, PluginEntry{
		Name:        toolName,
		Version:     "1.0.0",
		File:        toolName + ".wasm",
		Description: description,
		ToolName:    toolName,
		Runtime:     "synapse",
		Enabled:     true,
	})

	manifest.Plugins = filtered

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, data, 0644)
}

// ListForgedTools returns all tools created by the Forge.
func (f *Forge) ListForgedTools() ([]ForgeResult, error) {
	forgeDir := filepath.Join(f.pluginDir, "forge")
	entries, err := os.ReadDir(forgeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ForgeResult{}, nil
		}
		return nil, err
	}

	var results []ForgeResult
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		wasmPath := filepath.Join(f.pluginDir, name+".wasm")
		stat, err := os.Stat(wasmPath)
		if err != nil {
			continue
		}
		results = append(results, ForgeResult{
			ToolName: name,
			WasmPath: wasmPath,
			WasmSize: stat.Size(),
			Status:   "ok",
		})
	}
	return results, nil
}

// --- helpers ---

func sanitizeToolName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	// Keep only alphanumeric and underscore
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func cleanCodeResponse(response string) string {
	// Strip markdown code fences
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```go") {
		response = strings.TrimPrefix(response, "```go")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}
	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}
	return strings.TrimSpace(response)
}

func extractParamsFromSource(source string) *domain.ToolParameters {
	// Look for: // @params {"type":"object",...}
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "// @params ") {
			jsonStr := strings.TrimPrefix(line, "// @params ")
			var params domain.ToolParameters
			if err := json.Unmarshal([]byte(jsonStr), &params); err == nil {
				return &params
			}
		}
	}
	return nil
}
