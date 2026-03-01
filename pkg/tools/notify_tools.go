// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// PushoverTool sends push notifications via Pushover API.
type PushoverTool struct {
	BaseTool
	apiToken string
}

// NewPushoverTool creates a new PushoverTool.
func NewPushoverTool(apiToken string) *PushoverTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"user_key": {
				"type": "string",
				"description": "Pushover user key"
			},
			"message": {
				"type": "string",
				"description": "Message to send"
			},
			"title": {
				"type": "string",
				"description": "Notification title (optional)"
			},
			"priority": {
				"type": "integer",
				"description": "Message priority (-2 to 2, default: 0)",
				"minimum": -2,
				"maximum": 2
			}
		},
		"required": ["user_key", "message"]
	}`)
	return &PushoverTool{
		BaseTool: *NewBaseTool(
			"pushover",
			"通过 Pushover API 发送推送通知。需要配置 api_token。",
			schema,
		),
		apiToken: apiToken,
	}
}

// Execute executes the pushover tool.
func (t *PushoverTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	userKey, _ := args["user_key"].(string)
	message, _ := args["message"].(string)
	title, _ := args["title"].(string)
	priority := 0
	if p, ok := args["priority"].(float64); ok {
		priority = int(p)
	}

	if userKey == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "user_key parameter is required",
		}, nil
	}
	if message == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "message parameter is required",
		}, nil
	}

	// Use environment variable if no token configured
	apiToken := t.apiToken
	if apiToken == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "Pushover API token not configured. Set PUSHOVER_API_TOKEN environment variable or configure in config.",
		}, nil
	}

	// Send to Pushover API
	formData := url.Values{}
	formData.Set("token", apiToken)
	formData.Set("user", userKey)
	formData.Set("message", message)
	if title != "" {
		formData.Set("title", title)
	}
	formData.Set("priority", fmt.Sprintf("%d", priority))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm("https://api.pushover.net/1/messages.json", formData)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Pushover API error: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Pushover API returned %d: %s", resp.StatusCode, string(body)),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Push notification sent successfully to user %s", userKey[:8]+"..."),
	}, nil
}

// DelegateTool delegates tasks to sub-agents.
type DelegateTool struct {
	BaseTool
	workspaceDir string
}

// NewDelegateTool creates a new DelegateTool.
func NewDelegateTool(workspaceDir string) *DelegateTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"agent": {
				"type": "string",
				"description": "Name of the delegate agent to use"
			},
			"prompt": {
				"type": "string",
				"description": "Task prompt for the delegate agent"
			},
			"max_depth": {
				"type": "integer",
				"description": "Maximum delegation depth (default: 3)",
				"default": 3
			}
		},
		"required": ["prompt"]
	}`)
	return &DelegateTool{
		BaseTool: *NewBaseTool(
			"delegate",
			"将任务委托给专门的子代理。适用于需要专业知识的复杂任务。",
			schema,
		),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the delegate tool.
func (t *DelegateTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	agent, _ := args["agent"].(string)
	prompt, _ := args["prompt"].(string)
	maxDepth := 3
	if md, ok := args["max_depth"].(float64); ok {
		maxDepth = int(md)
	}

	if prompt == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "prompt parameter is required",
		}, nil
	}

	// For now, return a stub response
	// In a real implementation, this would invoke a sub-agent
	var agentInfo string
	if agent != "" {
		agentInfo = fmt.Sprintf("Agent: %s\n", agent)
	}

	return &ToolResult{
		Success: true,
		Output: fmt.Sprintf(`Delegation initiated (stub):
%sMax Depth: %d
Prompt: %s

Note: Full delegation requires sub-agent configuration. This is a placeholder response.`,
			agentInfo, maxDepth, truncate(prompt, 200)),
	}, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ModelRoutingConfigTool manages model routing configuration.
type ModelRoutingConfigTool struct {
	BaseTool
}

// NewModelRoutingConfigTool creates a new ModelRoutingConfigTool.
func NewModelRoutingConfigTool() *ModelRoutingConfigTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["list", "set", "remove"],
				"description": "Action to perform"
			},
			"hint": {
				"type": "string",
				"description": "Routing hint (e.g., 'code', 'math', 'creative')"
			},
			"model": {
				"type": "string",
				"description": "Model to route to for the hint"
			}
		},
		"required": ["action"]
	}`)
	return &ModelRoutingConfigTool{
		BaseTool: *NewBaseTool(
			"model_routing_config",
			"配置基于任务提示的模型路由。将特定任务映射到专门的模型。",
			schema,
		),
	}
}

// Execute executes the model routing config tool.
func (t *ModelRoutingConfigTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	action, _ := args["action"].(string)
	hint, _ := args["hint"].(string)
	model, _ := args["model"].(string)

	switch action {
	case "list":
		return &ToolResult{
			Success: true,
			Output:  "Model routing rules: (not implemented - stub)",
		}, nil
	case "set":
		if hint == "" || model == "" {
			return &ToolResult{
				Success: false,
				Output:  "",
				Error:   "hint and model parameters are required for set action",
			}, nil
		}
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Set routing: hint '%s' -> model '%s' (stub)", hint, model),
		}, nil
	case "remove":
		if hint == "" {
			return &ToolResult{
				Success: false,
				Output:  "",
				Error:   "hint parameter is required for remove action",
			}, nil
		}
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Removed routing for hint '%s' (stub)", hint),
		}, nil
	default:
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Unknown action: %s", action),
		}, nil
	}
}

// ProxyConfigTool manages proxy configuration.
type ProxyConfigTool struct {
	BaseTool
}

// NewProxyConfigTool creates a new ProxyConfigTool.
func NewProxyConfigTool() *ProxyConfigTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["get", "set", "clear"],
				"description": "Action to perform"
			},
			"http_proxy": {
				"type": "string",
				"description": "HTTP proxy URL"
			},
			"https_proxy": {
				"type": "string",
				"description": "HTTPS proxy URL"
			}
		},
		"required": ["action"]
	}`)
	return &ProxyConfigTool{
		BaseTool: *NewBaseTool(
			"proxy_config",
			"配置代理的 HTTP/HTTPS 代理设置。",
			schema,
		),
	}
}

// Execute executes the proxy config tool.
func (t *ProxyConfigTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	action, _ := args["action"].(string)

	switch action {
	case "get":
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Proxy settings: (not configured - stub)"),
		}, nil
	case "set":
		httpProxy, _ := args["http_proxy"].(string)
		httpsProxy, _ := args["https_proxy"].(string)
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Set proxy: http=%s, https=%s (stub)", httpProxy, httpsProxy),
		}, nil
	case "clear":
		return &ToolResult{
			Success: true,
			Output:  "Proxy settings cleared (stub)",
		}, nil
	default:
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Unknown action: %s", action),
		}, nil
	}
}
