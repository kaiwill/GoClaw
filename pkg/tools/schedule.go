// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ScheduleTool manages scheduled tasks.
type ScheduleTool struct {
	BaseTool
	workspaceDir string
}

// NewScheduleTool creates a new ScheduleTool.
func NewScheduleTool(workspaceDir string) *ScheduleTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["list", "create", "cancel", "pause", "resume"],
				"description": "Action to perform"
			},
			"expression": {
				"type": "string",
				"description": "Cron expression for recurring tasks (e.g. '*/5 * * * *')"
			},
			"delay": {
				"type": "string",
				"description": "Delay for one-shot tasks (e.g. '30m', '2h', '1d')"
			},
			"command": {
				"type": "string",
				"description": "Shell command to execute. Required for create."
			},
			"id": {
				"type": "string",
				"description": "Task ID. Required for cancel/pause/resume."
			}
		},
		"required": ["action"]
	}`)
	return &ScheduleTool{
		BaseTool: *NewBaseTool(
			"schedule",
			"管理计划任务。操作：create/add/once/list/get/cancel/remove/pause/resume。警告：shell 任务输出仅记录日志，不会发送到任何渠道。",
			schema,
		),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the schedule tool.
func (t *ScheduleTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "action parameter is required",
		}, nil
	}

	switch action {
	case "list":
		return t.handleList()
	case "create", "add", "once":
		return t.handleCreate(action, args)
	case "cancel", "remove":
		return t.handleCancel(args)
	case "pause":
		return t.handlePauseResume(args, true)
	case "resume":
		return t.handlePauseResume(args, false)
	default:
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Unknown action '%s'. Use list/create/cancel/pause/resume.", action),
		}, nil
	}
}

func (t *ScheduleTool) handleList() (*ToolResult, error) {
	// TODO: Implement actual schedule listing when cron package is ready
	return &ToolResult{
		Success: true,
		Output:  "No scheduled jobs. (Schedule storage not implemented yet)",
	}, nil
}

func (t *ScheduleTool) handleCreate(action string, args map[string]interface{}) (*ToolResult, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "command parameter is required for create",
		}, nil
	}

	expression, _ := args["expression"].(string)
	delay, _ := args["delay"].(string)

	if expression == "" && delay == "" && action != "once" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "expression or delay parameter is required",
		}, nil
	}

	// TODO: Implement actual schedule creation when cron package is ready
	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Scheduled job created (stub): cmd=%s, expr=%s, delay=%s", command, expression, delay),
	}, nil
}

func (t *ScheduleTool) handleCancel(args map[string]interface{}) (*ToolResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "id parameter is required for cancel",
		}, nil
	}

	// TODO: Implement actual schedule cancellation when cron package is ready
	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Cancelled job %s (stub)", id),
	}, nil
}

func (t *ScheduleTool) handlePauseResume(args map[string]interface{}, pause bool) (*ToolResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "id parameter is required",
		}, nil
	}

	action := "Resumed"
	if pause {
		action = "Paused"
	}

	// TODO: Implement actual pause/resume when cron package is ready
	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("%s job %s (stub)", action, id),
	}, nil
}

// TaskPlanTool helps plan and track tasks.
type TaskPlanTool struct {
	BaseTool
}

// NewTaskPlanTool creates a new TaskPlanTool.
func NewTaskPlanTool() *TaskPlanTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["create", "list", "update", "complete", "delete"],
				"description": "Action to perform"
			},
			"title": {
				"type": "string",
				"description": "Task title"
			},
			"description": {
				"type": "string",
				"description": "Task description"
			},
			"steps": {
				"type": "array",
				"items": { "type": "string" },
				"description": "List of steps to complete the task"
			},
			"id": {
				"type": "string",
				"description": "Task ID for update/complete/delete"
			},
			"step_index": {
				"type": "integer",
				"description": "Step index to mark as complete"
			}
		},
		"required": ["action"]
	}`)
	return &TaskPlanTool{
		BaseTool: *NewBaseTool(
			"task_plan",
			"规划和跟踪任务步骤。创建多步骤任务计划，跟踪进度，标记完成状态。",
			schema,
		),
	}
}

// Execute executes the task plan tool.
func (t *TaskPlanTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "action parameter is required",
		}, nil
	}

	switch action {
	case "create":
		title, _ := args["title"].(string)
		description, _ := args["description"].(string)
		stepsRaw, _ := args["steps"].([]interface{})
		var steps []string
		for _, s := range stepsRaw {
			if str, ok := s.(string); ok {
				steps = append(steps, str)
			}
		}

		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Created task plan: %s\nDescription: %s\nSteps: %v", title, description, steps),
		}, nil

	case "list":
		return &ToolResult{
			Success: true,
			Output:  "No task plans. (Task storage not implemented yet)",
		}, nil

	case "update", "complete", "delete":
		id, _ := args["id"].(string)
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("%s task %s (stub)", action, id),
		}, nil

	default:
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Unknown action: %s", action),
		}, nil
	}
}
