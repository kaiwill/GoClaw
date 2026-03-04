// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultMaxResponseSize = 500000
	defaultTimeoutSecs     = 30
)

// WebFetchTool fetches web pages and converts HTML to plain text.
type WebFetchTool struct {
	BaseTool
	maxResponseSize int
	timeoutSecs     int
}

// NewWebFetchTool creates a new WebFetchTool.
func NewWebFetchTool() *WebFetchTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The HTTP or HTTPS URL to fetch"
			},
			"prompt": {
				"type": "string",
				"description": "Instructions on how to process the fetched content"
			}
		},
		"required": ["url"]
	}`)
	return &WebFetchTool{
		BaseTool: *NewBaseTool(
			"web_fetch",
			"获取网页并返回纯文本内容。HTML 页面自动转换为可读文本，JSON 和纯文本响应原样返回。仅支持 GET 请求，自动跟随重定向。",
			schema,
		),
		maxResponseSize: defaultMaxResponseSize,
		timeoutSecs:     defaultTimeoutSecs,
	}
}

// Execute executes the web fetch tool.
func (t *WebFetchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	urlStr, ok := args["url"].(string)
	if !ok || urlStr == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "url parameter is required",
		}, nil
	}

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Invalid URL: %v", err),
		}, nil
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "Only http:// and https:// URLs are allowed",
		}, nil
	}

	// Block private/local hosts for security
	host := parsedURL.Hostname()
	if isPrivateHost(host) {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Blocked local/private host: %s", host),
		}, nil
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(t.timeoutSecs) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to create request: %v", err),
		}, nil
	}
	req.Header.Set("User-Agent", "GoClaw/0.1 (web_fetch)")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("HTTP request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status),
		}, nil
	}

	// Read response body with limit
	limitedReader := io.LimitReader(resp.Body, int64(t.maxResponseSize+1))
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to read response: %v", err),
		}, nil
	}

	// Check if truncated
	truncated := len(body) > t.maxResponseSize
	if truncated {
		body = body[:t.maxResponseSize]
	}

	// Determine content type
	contentType := resp.Header.Get("Content-Type")
	var output string

	if strings.Contains(contentType, "text/html") {
		// Convert HTML to plain text
		output = htmlToText(string(body))
	} else if strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "text/markdown") {
		output = string(body)
	} else {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Unsupported content type: %s", contentType),
		}, nil
	}

	if truncated {
		output += "\n\n[Response truncated due to size limit]"
	}

	return &ToolResult{
		Success: true,
		Output:  output,
	}, nil
}

// isPrivateHost checks if a host is private/local.
func isPrivateHost(host string) bool {
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") {
		return true
	}
	if strings.HasSuffix(lowerHost, ".local") {
		return true
	}
	if strings.HasPrefix(lowerHost, "127.") ||
		strings.HasPrefix(lowerHost, "10.") ||
		strings.HasPrefix(lowerHost, "192.168.") ||
		strings.HasPrefix(lowerHost, "172.16.") ||
		strings.HasPrefix(lowerHost, "172.17.") ||
		strings.HasPrefix(lowerHost, "172.18.") ||
		strings.HasPrefix(lowerHost, "172.19.") ||
		strings.HasPrefix(lowerHost, "172.20.") ||
		strings.HasPrefix(lowerHost, "172.21.") ||
		strings.HasPrefix(lowerHost, "172.22.") ||
		strings.HasPrefix(lowerHost, "172.23.") ||
		strings.HasPrefix(lowerHost, "172.24.") ||
		strings.HasPrefix(lowerHost, "172.25.") ||
		strings.HasPrefix(lowerHost, "172.26.") ||
		strings.HasPrefix(lowerHost, "172.27.") ||
		strings.HasPrefix(lowerHost, "172.28.") ||
		strings.HasPrefix(lowerHost, "172.29.") ||
		strings.HasPrefix(lowerHost, "172.30.") ||
		strings.HasPrefix(lowerHost, "172.31.") {
		return true
	}
	return false
}

// htmlToText converts HTML to plain text.
func htmlToText(html string) string {
	// Simple HTML to text conversion
	// Remove script and style blocks
	result := html

	// Remove script tags
	for {
		start := strings.Index(result, "<script")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "</script>")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+9:]
	}

	// Remove style tags
	for {
		start := strings.Index(result, "<style")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "</style>")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+8:]
	}

	// Replace common block elements with newlines
	replacements := []struct {
		from, to string
	}{
		{"<br>", "\n"},
		{"<br/>", "\n"},
		{"<br />", "\n"},
		{"</p>", "\n\n"},
		{"</div>", "\n"},
		{"</h1>", "\n\n"},
		{"</h2>", "\n\n"},
		{"</h3>", "\n\n"},
		{"</h4>", "\n"},
		{"</h5>", "\n"},
		{"</h6>", "\n"},
		{"</li>", "\n"},
		{"</tr>", "\n"},
		{"</td>", " "},
		{"</th>", " "},
	}

	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.from, r.to)
		result = strings.ReplaceAll(strings.ToLower(result), strings.ToLower(r.from), r.to)
	}

	// Remove remaining HTML tags
	for {
		start := strings.Index(result, "<")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}

	// Decode HTML entities
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", "\"")
	result = strings.ReplaceAll(result, "&#39;", "'")

	// Clean up whitespace
	lines := strings.Split(result, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		} else if len(cleanLines) > 0 && cleanLines[len(cleanLines)-1] != "" {
			cleanLines = append(cleanLines, "")
		}
	}

	return strings.Join(cleanLines, "\n")
}
