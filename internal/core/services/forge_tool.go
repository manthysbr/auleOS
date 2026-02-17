package services

import (
	"context"
	"fmt"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/synapse"
)

// NewCreateToolTool returns the "create_tool" tool that allows the ReAct agent
// to forge new tools from natural language descriptions at runtime.
//
// When a user says "I need a tool that converts CSV to JSON", the agent can
// call create_tool and a new Wasm-sandboxed tool will be compiled and loaded
// in real time — no restart needed.
func NewCreateToolTool(forge *synapse.Forge) *domain.Tool {
	return &domain.Tool{
		Name: "create_tool",
		Description: "Creates a new tool from a natural language description. " +
			"The tool will be compiled to WebAssembly and loaded instantly. " +
			"Use this when the user needs a custom data transformation, text processing, " +
			"or any logic that doesn't require GPU/network. " +
			"The created tool reads JSON from stdin and writes JSON to stdout.",
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "A short snake_case name for the tool (e.g. csv_to_json, word_counter, slug_generator)",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "A detailed description of what the tool should do, including input/output format",
				},
			},
			Required: []string{"name", "description"},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			name, _ := params["name"].(string)
			if name == "" {
				return nil, fmt.Errorf("missing required parameter: name")
			}

			description, _ := params["description"].(string)
			if description == "" {
				return nil, fmt.Errorf("missing required parameter: description")
			}

			result, err := forge.Create(ctx, name, description)
			if err != nil {
				return map[string]interface{}{
					"status":  "error",
					"message": fmt.Sprintf("Failed to create tool: %v", err),
				}, nil
			}

			return map[string]interface{}{
				"status":       result.Status,
				"tool_name":    result.ToolName,
				"description":  result.Description,
				"wasm_size":    result.WasmSize,
				"compile_time": result.CompileTime,
				"message": fmt.Sprintf(
					"Tool '%s' created and loaded successfully! It's now available for use. "+
						"Compiled to %d bytes of WebAssembly in %s.",
					result.ToolName, result.WasmSize, result.CompileTime,
				),
			}, nil
		},
	}
}

// NewListForgedToolsTool returns a tool that lists all tools created by the Forge.
func NewListForgedToolsTool(forge *synapse.Forge) *domain.Tool {
	return &domain.Tool{
		Name:        "list_forged_tools",
		Description: "Lists all custom tools that were created by the Tool Forge",
		Parameters: domain.ToolParameters{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
		ExecutionType: domain.ExecNative,
		Execute: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			tools, err := forge.ListForgedTools()
			if err != nil {
				return nil, fmt.Errorf("failed to list forged tools: %w", err)
			}

			items := make([]map[string]interface{}, len(tools))
			for i, t := range tools {
				items[i] = map[string]interface{}{
					"name":      t.ToolName,
					"wasm_size": t.WasmSize,
					"status":    t.Status,
				}
			}

			return map[string]interface{}{
				"tools": items,
				"count": len(items),
			}, nil
		},
	}
}
