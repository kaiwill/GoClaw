// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// WebSearchTool searches the web for information.
type WebSearchTool struct {
	BaseTool
	provider   string
	maxResults int
	timeoutSecs int
}

// NewWebSearchTool creates a new WebSearchTool.
func NewWebSearchTool() *WebSearchTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "The search query. Be specific for better results."
			},
			"num_results": {
				"type": "integer",
				"description": "Number of results to return (default: 5)",
				"default": 5
			}
		},
		"required": ["query"]
	}`)
	return &WebSearchTool{
		BaseTool: *NewBaseTool(
			"web_search",
			"搜索网络获取信息。返回相关搜索结果，包括标题、URL 和描述。用于查找最新信息、新闻或研究主题。",
			schema,
		),
		provider:    "duckduckgo",
		maxResults:  5,
		timeoutSecs: 30,
	}
}

// Execute executes the web search tool.
func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "query parameter is required",
		}, nil
	}

	maxResults := t.maxResults
	if n, ok := args["num_results"].(float64); ok {
		maxResults = int(n)
		if maxResults > 10 {
			maxResults = 10
		}
		if maxResults < 1 {
			maxResults = 1
		}
	}

	// Use DuckDuckGo HTML search (no API key required)
	return t.searchDuckDuckGo(ctx, query, maxResults)
}

func (t *WebSearchTool) searchDuckDuckGo(ctx context.Context, query string, maxResults int) (*ToolResult, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	client := &http.Client{
		Timeout: time.Duration(t.timeoutSecs) * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to create request: %v", err),
		}, nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Search request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Search failed with status: %d", resp.StatusCode),
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to read response: %v", err),
		}, nil
	}

	results := t.parseDuckDuckGoResults(string(body), maxResults)

	var output strings.Builder
	fmt.Fprintf(&output, "Search results for: %s (via DuckDuckGo)\n\n", query)
	output.WriteString(results)

	return &ToolResult{
		Success: true,
		Output:  output.String(),
	}, nil
}

func (t *WebSearchTool) parseDuckDuckGoResults(html string, maxResults int) string {
	// Extract result links
	linkRegex := regexp.MustCompile(`<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)
	snippetRegex := regexp.MustCompile(`<a class="result__snippet[^"]*"[^>]*>([\s\S]*?)</a>`)

	linkMatches := linkRegex.FindAllStringSubmatch(html, maxResults+2)
	snippetMatches := snippetRegex.FindAllStringSubmatch(html, maxResults+2)

	if len(linkMatches) == 0 {
		return "No results found."
	}

	var results []string
	count := len(linkMatches)
	if count > maxResults {
		count = maxResults
	}

	for i := 0; i < count; i++ {
		caps := linkMatches[i]
		urlStr := decodeDDGRedirectURL(caps[1])
		title := stripTags(caps[2])

		var lines []string
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(title)))
		lines = append(lines, fmt.Sprintf("   %s", strings.TrimSpace(urlStr)))

		// Add snippet if available
		if i < len(snippetMatches) {
			snippet := stripTags(snippetMatches[i][1])
			snippet = strings.TrimSpace(snippet)
			if snippet != "" {
				lines = append(lines, fmt.Sprintf("   %s", snippet))
			}
		}

		results = append(results, strings.Join(lines, "\n"))
	}

	return strings.Join(results, "\n\n")
}

func decodeDDGRedirectURL(rawURL string) string {
	if idx := strings.Index(rawURL, "uddg="); idx != -1 {
		encoded := rawURL[idx+5:]
		if end := strings.Index(encoded, "&"); end != -1 {
			encoded = encoded[:end]
		}
		if decoded, err := url.QueryUnescape(encoded); err == nil {
			return decoded
		}
	}
	return rawURL
}

func stripTags(content string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(content, "")
}
