package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPTool struct {
	name        string
	description string
	client      *http.Client
	timeout     time.Duration
}

func NewHTTPTool() *HTTPTool {
	return &HTTPTool{
		name:        "http",
		description: "发送 HTTP 请求获取数据",
		client:      &http.Client{Timeout: 30 * time.Second},
		timeout:     30 * time.Second,
	}
}

func (t *HTTPTool) Name() string {
	return t.name
}

func (t *HTTPTool) Description() string {
	return t.description
}

func (t *HTTPTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The URL to request"
			},
			"method": {
				"type": "string",
				"enum": ["GET", "POST", "PUT", "DELETE", "PATCH"],
				"description": "HTTP method to use"
			},
			"headers": {
				"type": "object",
				"description": "HTTP headers to send"
			},
			"body": {
				"type": "string",
				"description": "Request body for POST/PUT/PATCH"
			}
		},
		"required": ["url"]
	}`)
}

func (t *HTTPTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.ParametersSchema(),
	}
}

func (t *HTTPTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	urlStr, ok := args["url"].(string)
	if !ok || urlStr == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "url is required",
		}, nil
	}

	method := "GET"
	if m, ok := args["method"].(string); ok {
		method = strings.ToUpper(m)
	}

	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
	if !validMethods[method] {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid method: %s", method),
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	if headers, ok := args["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Status: %d\n", resp.StatusCode),
	}, nil
}

type FetchTool struct {
	name        string
	description string
	client      *http.Client
}

func NewFetchTool() *FetchTool {
	return &FetchTool{
		name:        "fetch",
		description: "获取并解析网页内容",
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *FetchTool) Name() string {
	return t.name
}

func (t *FetchTool) Description() string {
	return t.description
}

func (t *FetchTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The URL to fetch"
			},
			"selector": {
				"type": "string",
				"description": "CSS selector to extract specific content"
			}
		},
		"required": ["url"]
	}`)
}

func (t *FetchTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.ParametersSchema(),
	}
}

func (t *FetchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	urlStr, ok := args["url"].(string)
	if !ok || urlStr == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "url is required",
		}, nil
	}

	resp, err := t.client.Get(urlStr)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("fetch failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Fetched %s, status: %d", urlStr, resp.StatusCode),
	}, nil
}

type SearchTool struct {
	name        string
	description string
	client      *http.Client
}

func NewSearchTool() *SearchTool {
	return &SearchTool{
		name:        "search",
		description: "Search the web for information",
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *SearchTool) Name() string {
	return t.name
}

func (t *SearchTool) Description() string {
	return t.description
}

func (t *SearchTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Search query"
			},
			"num_results": {
				"type": "number",
				"description": "Number of results to return"
			}
		},
		"required": ["query"]
	}`)
}

func (t *SearchTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.ParametersSchema(),
	}
}

func (t *SearchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "query is required",
		}, nil
	}

	searchURL := fmt.Sprintf("https://duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	resp, err := t.client.Get(searchURL)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("search failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Search completed for: %s", query),
	}, nil
}
