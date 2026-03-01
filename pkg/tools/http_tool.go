// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// HTTPRequestTool makes HTTP requests.
type HTTPRequestTool struct {
	BaseTool
}

// NewHTTPRequestTool creates a new HTTPRequestTool.
func NewHTTPRequestTool() *HTTPRequestTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": { "type": "string", "description": "URL to make request to" },
			"method": { "type": "string", "description": "HTTP method (GET, POST, etc.)", "default": "GET" },
			"headers": { "type": "object", "description": "HTTP headers" },
			"body": { "type": "string", "description": "HTTP request body" }
		},
		"required": ["url"]
	}`)
	return &HTTPRequestTool{
		BaseTool: *NewBaseTool(
			"http_request",
			"发送 HTTP 请求",
			schema,
		),
	}
}

// Execute executes the HTTP request tool.
func (t *HTTPRequestTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	url, ok := args["url"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Output:  "url is required",
			Error:   "url parameter is missing or invalid",
		}, nil
	}

	method, ok := args["method"].(string)
	if !ok {
		method = "GET"
	}

	headers, _ := args["headers"].(map[string]interface{})
	body, _ := args["body"].(string)

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  fmt.Sprintf("Failed to create request: %v", err),
			Error:   err.Error(),
		}, nil
	}

	// Set headers
	for key, value := range headers {
		if v, ok := value.(string); ok {
			req.Header.Set(key, v)
		}
	}

	// Set default Content-Type for POST requests
	if method == "POST" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  fmt.Sprintf("Request failed: %v", err),
			Error:   err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  fmt.Sprintf("Failed to read response: %v", err),
			Error:   err.Error(),
		}, nil
	}

	// Build result
	result := fmt.Sprintf("Status: %d %s\nHeaders: %v\nBody: %s",
		resp.StatusCode, resp.Status, resp.Header, string(respBody))

	return &ToolResult{
		Success: true,
		Output:  result,
	}, nil
}
