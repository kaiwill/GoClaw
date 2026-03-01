// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"sync"
)

// Tool represents a tool that can be executed by the agent.
type Tool interface {
	// Name returns the name of the tool.
	Name() string

	// Description returns the description of the tool.
	Description() string

	// ParametersSchema returns the JSON schema for the tool's parameters.
	ParametersSchema() json.RawMessage

	// Execute executes the tool with the given arguments.
	Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)

	// Spec returns the tool specification.
	Spec() ToolSpec
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

// ToolSpec represents the specification of a tool.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a tool call requested by the LLM.
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolDispatcher dispatches tool calls.
type ToolDispatcher interface {
	// ExecuteTool executes a single tool call.
	ExecuteTool(ctx context.Context, call ToolCall, tools []Tool) (*ToolResult, error)

	// ExecuteTools executes multiple tool calls.
	ExecuteTools(ctx context.Context, calls []ToolCall, tools []Tool) ([]ToolExecutionResult, error)
}

// ToolExecutionResult represents the result of a tool execution with tool call ID.
type ToolExecutionResult struct {
	ToolCallID string     `json:"tool_call_id"`
	Result     *ToolResult `json:"result"`
}

// BaseTool provides a base implementation for Tool.
type BaseTool struct {
	name        string
	description string
	parameters  json.RawMessage
}

// NewBaseTool creates a new BaseTool.
func NewBaseTool(name, description string, parameters json.RawMessage) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
		parameters:  parameters,
	}
}

// Name returns the name of the tool.
func (t *BaseTool) Name() string {
	return t.name
}

// Description returns the description of the tool.
func (t *BaseTool) Description() string {
	return t.description
}

// ParametersSchema returns the JSON schema for the tool's parameters.
func (t *BaseTool) ParametersSchema() json.RawMessage {
	return t.parameters
}

// Spec returns the tool specification.
func (t *BaseTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        t.name,
		Description: t.description,
		Parameters:  t.parameters,
	}
}

// Execute executes the tool with the given arguments.
func (t *BaseTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Default implementation returns an error
	return &ToolResult{
		Success: false,
		Output:  "not implemented",
		Error:   "method Execute not implemented",
	}, nil
}

// ToolRegistry is a registry for tools.
type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewToolRegistry creates a new ToolRegistry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// RegisterTool registers a tool in the registry.
func (r *ToolRegistry) RegisterTool(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// GetTool retrieves a tool from the registry.
func (r *ToolRegistry) GetTool(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
}

// ListTools returns all tools in the registry.
func (r *ToolRegistry) ListTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ToolManager manages tools for the agent.
type ToolManager struct {
	registry *ToolRegistry
}

// NewToolManager creates a new ToolManager.
func NewToolManager() *ToolManager {
	return &ToolManager{
		registry: NewToolRegistry(),
	}
}

// AddTool adds a tool to the manager.
func (m *ToolManager) AddTool(tool Tool) {
	m.registry.RegisterTool(tool)
}

// GetTool retrieves a tool from the manager.
func (m *ToolManager) GetTool(name string) (Tool, bool) {
	return m.registry.GetTool(name)
}

// ListTools returns all tools in the manager.
func (m *ToolManager) ListTools() []Tool {
	return m.registry.ListTools()
}

// ExecuteTool executes a tool with the given arguments.
func (m *ToolManager) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	tool, exists := m.registry.GetTool(name)
	if !exists {
		return &ToolResult{
			Success: false,
			Output:  "tool not found",
			Error:   "tool not found",
		}, nil
	}

	return tool.Execute(ctx, args)
}
