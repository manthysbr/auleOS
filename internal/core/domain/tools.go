package domain

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool represents an executable capability available to the agent
type Tool struct {
	Name        string
	Description string
	Parameters  ToolParameters
	Execute     ToolExecutor
}

// ToolParameters defines the schema for tool inputs
type ToolParameters struct {
	Type       string                 `json:"type"`       // "object"
	Properties map[string]interface{} `json:"properties"` // param definitions
	Required   []string               `json:"required"`   // required param names
}

// ToolExecutor is the function signature for tool execution
type ToolExecutor func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// ToolRegistry manages available tools
type ToolRegistry struct {
	tools map[string]*Tool
}

// NewToolRegistry creates a new empty registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool *Tool) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	r.tools[tool.Name] = tool
	return nil
}

// Execute runs a tool with given parameters
func (r *ToolRegistry) Execute(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	
	return tool.Execute(ctx, params)
}

// GetTool returns a tool by name
func (r *ToolRegistry) GetTool(name string) (*Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// ListTools returns all registered tools
func (r *ToolRegistry) ListTools() []*Tool {
	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// FormatToolsForPrompt generates a description of available tools for LLM prompt
func (r *ToolRegistry) FormatToolsForPrompt() string {
	result := "Available Tools:\n"
	for _, tool := range r.tools {
		paramsJSON, _ := json.MarshalIndent(tool.Parameters, "  ", "  ")
		result += fmt.Sprintf("- %s: %s\n  Parameters: %s\n", tool.Name, tool.Description, string(paramsJSON))
	}
	return result
}
