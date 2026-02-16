package domain

// ReActStep represents one step in the ReAct reasoning chain
type ReActStep struct {
	Thought      string                 `json:"thought"`
	Action       string                 `json:"action"`        // Tool name
	ActionInput  map[string]interface{} `json:"action_input"`  // Tool parameters
	Observation  string                 `json:"observation"`   // Tool result
	IsFinalAnswer bool                  `json:"is_final_answer"`
	FinalAnswer  string                 `json:"final_answer"`
}

// AgentResponse wraps the agent's response with metadata
type AgentResponse struct {
	Response string     `json:"response"`
	Thought  string     `json:"thought"`
	ToolCall *ToolCall  `json:"tool_call,omitempty"`
	Steps    []ReActStep `json:"steps"`
}
