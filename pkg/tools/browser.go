// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// BrowserOpenTool opens URLs in the default browser.
type BrowserOpenTool struct {
	BaseTool
	allowedDomains []string
}

// NewBrowserOpenTool creates a new BrowserOpenTool.
func NewBrowserOpenTool(allowedDomains []string) *BrowserOpenTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "URL to open in the browser"
			}
		},
		"required": ["url"]
	}`)
	return &BrowserOpenTool{
		BaseTool: *NewBaseTool(
			"browser_open",
			"在默认浏览器中打开 URL。",
			schema,
		),
		allowedDomains: allowedDomains,
	}
}

// Execute executes the browser open tool.
func (t *BrowserOpenTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	urlStr, ok := args["url"].(string)
	if !ok || urlStr == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "url parameter is required",
		}, nil
	}

	// Validate URL
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "URL must start with http:// or https://",
		}, nil
	}

	// Check domain allowlist
	if len(t.allowedDomains) > 0 {
		allowed := false
		for _, domain := range t.allowedDomains {
			if domain == "*" || strings.Contains(urlStr, domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return &ToolResult{
				Success: false,
				Output:  "",
				Error:   fmt.Sprintf("URL domain not in allowlist: %s", urlStr),
			}, nil
		}
	}

	// Open browser based on platform
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", urlStr)
	case "linux":
		cmd = exec.CommandContext(ctx, "xdg-open", urlStr)
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd", "/c", "start", urlStr)
	default:
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Unsupported platform: %s", runtime.GOOS),
		}, nil
	}

	if err := cmd.Run(); err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to open browser: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Opened URL in browser: %s", urlStr),
	}, nil
}

// BrowserTool provides browser automation capabilities.
type BrowserTool struct {
	BaseTool
	allowedDomains []string
}

// NewBrowserTool creates a new BrowserTool.
func NewBrowserTool(allowedDomains []string) *BrowserTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["open", "screenshot", "click", "fill", "scroll", "wait"],
				"description": "Browser action to perform"
			},
			"url": { "type": "string", "description": "URL for open action" },
			"selector": { "type": "string", "description": "CSS selector for click/fill actions" },
			"value": { "type": "string", "description": "Value for fill action" },
			"timeout_ms": { "type": "integer", "description": "Timeout for wait action" }
		},
		"required": ["action"]
	}`)
	return &BrowserTool{
		BaseTool: *NewBaseTool(
			"browser",
			"浏览器自动化工具，用于网页交互。支持 open、screenshot、click、fill、scroll、wait 等操作。",
			schema,
		),
		allowedDomains: allowedDomains,
	}
}

// Execute executes the browser tool.
func (t *BrowserTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "action parameter is required",
		}, nil
	}

	switch action {
	case "open":
		urlStr, _ := args["url"].(string)
		if urlStr == "" {
			return &ToolResult{
				Success: false,
				Output:  "",
				Error:   "url parameter is required for open action",
			}, nil
		}
		return t.handleOpen(ctx, urlStr)

	case "screenshot":
		return &ToolResult{
			Success: true,
			Output:  "Screenshot captured (stub - requires browser automation)",
		}, nil

	case "click":
		selector, _ := args["selector"].(string)
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Clicked element: %s (stub - requires browser automation)", selector),
		}, nil

	case "fill":
		selector, _ := args["selector"].(string)
		value, _ := args["value"].(string)
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Filled %s with: %s (stub - requires browser automation)", selector, value),
		}, nil

	default:
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Unknown action: %s", action),
		}, nil
	}
}

func (t *BrowserTool) handleOpen(ctx context.Context, urlStr string) (*ToolResult, error) {
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "URL must start with http:// or https://",
		}, nil
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", urlStr)
	case "linux":
		cmd = exec.CommandContext(ctx, "xdg-open", urlStr)
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd", "/c", "start", urlStr)
	default:
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Unsupported platform: %s", runtime.GOOS),
		}, nil
	}

	if err := cmd.Run(); err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to open browser: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Opened: %s", urlStr),
	}, nil
}
