// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ShellTool executes shell commands.
type ShellTool struct {
	BaseTool
}

// NewShellTool creates a new ShellTool.
func NewShellTool() *ShellTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": { "type": "string", "description": "Shell command to execute" }
		},
		"required": ["command"]
	}`)
	return &ShellTool{
		BaseTool: *NewBaseTool(
			"shell",
			"执行 shell 命令",
			schema,
		),
	}
}

// Execute executes the shell tool.
func (t *ShellTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	command, ok := args["command"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Output:  "command is required",
			Error:   "command parameter is missing or invalid",
		}, nil
	}

	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return &ToolResult{
			Success: false,
			Output:  "empty command",
			Error:   "command is empty",
		}, nil
	}

	// Execute command
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  fmt.Sprintf("Command failed: %s\nOutput: %s", err.Error(), string(output)),
			Error:   err.Error(),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}
