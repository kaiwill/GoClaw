// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxGlobResults = 1000

// GlobSearchTool searches for files by glob pattern.
type GlobSearchTool struct {
	BaseTool
	workspaceDir string
}

// NewGlobSearchTool creates a new GlobSearchTool.
func NewGlobSearchTool(workspaceDir string) *GlobSearchTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Glob pattern to match files, e.g. '**/*.go', 'src/**/main.go'"
			}
		},
		"required": ["pattern"]
	}`)
	return &GlobSearchTool{
		BaseTool:     *NewBaseTool("glob_search", "使用 glob 模式搜索工作区内的文件，返回相对于工作区根目录的匹配文件路径列表。", schema),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the glob search tool.
func (t *GlobSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return &ToolResult{
			Success: false,
			Output:  "pattern is required",
			Error:   "pattern parameter is missing or empty",
		}, nil
	}

	// Security: reject absolute paths
	if strings.HasPrefix(pattern, "/") || strings.HasPrefix(pattern, "\\") {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "Absolute paths are not allowed. Use a relative glob pattern.",
		}, nil
	}

	// Security: reject path traversal
	if strings.Contains(pattern, "../") || strings.Contains(pattern, "..\\") || pattern == ".." {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "Path traversal ('..') is not allowed in glob patterns.",
		}, nil
	}

	// Resolve workspace
	workspace := t.workspaceDir
	if workspace == "" {
		wd, _ := os.Getwd()
		workspace = wd
	}

	// Use double-star glob matching
	var results []string
	truncated := false

	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}

		// Check if path matches pattern
		relPath, err := filepath.Rel(workspace, path)
		if err != nil {
			return nil
		}

		matched, err := filepath.Match(pattern, relPath)
		if err != nil {
			return nil
		}

		// Also try double-star matching
		if !matched {
			matched = matchGlob(pattern, relPath)
		}

		if matched {
			results = append(results, relPath)
			if len(results) >= maxGlobResults {
				truncated = true
				return fmt.Errorf("stop")
			}
		}
		return nil
	})

	if err != nil && err.Error() != "stop" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to search: %v", err),
		}, nil
	}

	sort.Strings(results)

	var output string
	if len(results) == 0 {
		output = fmt.Sprintf("No files matching pattern '%s' found in workspace.", pattern)
	} else {
		output = strings.Join(results, "\n")
		if truncated {
			output += fmt.Sprintf("\n\n[Results truncated: showing first %d of more matches]", maxGlobResults)
		}
		output += fmt.Sprintf("\n\nTotal: %d files", len(results))
	}

	return &ToolResult{
		Success: true,
		Output:  output,
	}, nil
}

// matchGlob performs double-star glob matching.
func matchGlob(pattern, path string) bool {
	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		// Convert glob to regex-like matching
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			// Check prefix
			if prefix != "" && !strings.HasPrefix(path, prefix) {
				// Allow prefix to match any directory depth
				if !strings.Contains(path, "/"+prefix) && path != prefix && !strings.HasPrefix(path, prefix+"/") {
					return false
				}
			}

			// Check suffix
			if suffix != "" {
				// Use filepath.Match for the suffix
				matched, _ := filepath.Match(suffix, filepath.Base(path))
				if !matched {
					// Try matching the whole suffix against end of path
					if !strings.HasSuffix(path, suffix) {
						matched, _ = filepath.Match(suffix, path)
					}
				}
				return matched
			}
			return true
		}
	}

	// Standard glob matching
	matched, _ := filepath.Match(pattern, path)
	return matched
}
