package domain

import (
	"context"
	"fmt"
	"strings"
)

// ExecType identifies how a tool is executed.
type ExecType string

const (
	// ExecNative runs in the Go kernel process (default for built-in tools).
	ExecNative ExecType = "native"
	// ExecWasm runs inside the Synapse Wasm sandbox (forged/plugin tools).
	ExecWasm ExecType = "wasm"
	// ExecDocker runs inside a Docker container (heavy/GPU tools).
	ExecDocker ExecType = "docker"
)

// Tool represents an executable capability available to the agent
type Tool struct {
	Name          string
	Description   string
	Parameters    ToolParameters
	Execute       ToolExecutor
	ExecutionType ExecType // "native", "wasm", or "docker" (default: native)
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

// Execute runs a tool with given parameters.
// If the exact name is not found, it attempts fuzzy matching to handle LLM hallucinated names.
func (r *ToolRegistry) Execute(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	tool, ok := r.tools[name]
	if !ok {
		// Fuzzy match: find the closest tool name
		if match := r.fuzzyMatch(name); match != "" {
			tool = r.tools[match]
			// Log the correction for observability
			fmt.Printf("[tool-fuzzy] corrected %q → %q\n", name, match)
		} else {
			return nil, fmt.Errorf("tool not found: %s", name)
		}
	}

	return tool.Execute(ctx, params)
}

// fuzzyMatch finds the best matching tool name for a hallucinated/wrong name.
// It uses word-overlap scoring + Levenshtein distance as tiebreaker.
// Returns empty string if no reasonable match is found.
func (r *ToolRegistry) fuzzyMatch(input string) string {
	// Normalize: split by underscore into words
	inputWords := splitToolWords(input)

	bestName := ""
	bestScore := 0

	for name := range r.tools {
		nameWords := splitToolWords(name)
		score := wordOverlapScore(inputWords, nameWords)

		// Require at least 1 common word to consider
		if score > bestScore {
			bestScore = score
			bestName = name
		} else if score == bestScore && score > 0 {
			// Tiebreak: prefer shorter Levenshtein distance
			if levenshtein(input, name) < levenshtein(input, bestName) {
				bestName = name
			}
		}
	}

	// Only accept if score >= 1 (at least 1 word overlap)
	if bestScore >= 1 {
		return bestName
	}
	return ""
}

func splitToolWords(name string) []string {
	parts := []string{}
	for _, p := range strings.Split(strings.ToLower(name), "_") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func wordOverlapScore(a, b []string) int {
	set := make(map[string]bool, len(b))
	for _, w := range b {
		set[w] = true
	}
	score := 0
	for _, w := range a {
		if set[w] {
			score++
		}
	}
	return score
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
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

// FormatToolsForPrompt generates a concise description of available tools for LLM prompt.
// Uses compact format: name — description (required params) to reduce token usage.
func (r *ToolRegistry) FormatToolsForPrompt() string {
	result := "Available Tools:\n"
	for _, tool := range r.tools {
		// Compact required params list
		reqParams := ""
		if len(tool.Parameters.Required) > 0 {
			reqParams = " | required: " + strings.Join(tool.Parameters.Required, ", ")
		}

		// List all param names with types
		paramsList := ""
		if len(tool.Parameters.Properties) > 0 {
			parts := make([]string, 0, len(tool.Parameters.Properties))
			for pName, pDef := range tool.Parameters.Properties {
				pType := "any"
				if pm, ok := pDef.(map[string]interface{}); ok {
					if t, ok := pm["type"].(string); ok {
						pType = t
					}
				}
				parts = append(parts, pName+":"+pType)
			}
			paramsList = " | params: {" + strings.Join(parts, ", ") + "}"
		}

		execTag := ""
		if tool.ExecutionType == ExecWasm {
			execTag = " [wasm]"
		} else if tool.ExecutionType == ExecDocker {
			execTag = " [docker]"
		}
		result += fmt.Sprintf("- %s%s: %s%s%s\n", tool.Name, execTag, tool.Description, paramsList, reqParams)
	}
	return result
}

// FilterByNames returns a new ToolRegistry containing only the tools whose names match the given list.
// The new registry shares Tool pointers with the original (same Execute funcs).
func (r *ToolRegistry) FilterByNames(names []string) *ToolRegistry {
	allowed := make(map[string]struct{}, len(names))
	for _, n := range names {
		allowed[n] = struct{}{}
	}
	filtered := NewToolRegistry()
	for name, tool := range r.tools {
		if _, ok := allowed[name]; ok {
			filtered.tools[name] = tool
		}
	}
	return filtered
}
