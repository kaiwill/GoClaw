// Package tools provides tool functionality for GoClaw.
package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxSearchResults  = 1000
	maxSearchOutputMB = 1
)

// ContentSearchTool searches file contents by regex pattern.
type ContentSearchTool struct {
	BaseTool
	workspaceDir string
	hasRg        bool
}

// NewContentSearchTool creates a new ContentSearchTool.
func NewContentSearchTool(workspaceDir string) *ContentSearchTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Regular expression pattern to search for"
			},
			"path": {
				"type": "string",
				"description": "Directory to search in, relative to workspace root. Defaults to '.'",
				"default": "."
			},
			"output_mode": {
				"type": "string",
				"description": "Output format: 'content' (matching lines), 'files_with_matches' (paths only), 'count' (match counts)",
				"enum": ["content", "files_with_matches", "count"],
				"default": "content"
			},
			"include": {
				"type": "string",
				"description": "File glob filter, e.g. '*.go', '*.{ts,tsx}'"
			},
			"case_sensitive": {
				"type": "boolean",
				"description": "Case-sensitive matching. Defaults to true",
				"default": true
			},
			"context_before": {
				"type": "integer",
				"description": "Lines of context before each match (content mode only)",
				"default": 0
			},
			"context_after": {
				"type": "integer",
				"description": "Lines of context after each match (content mode only)",
				"default": 0
			},
			"max_results": {
				"type": "integer",
				"description": "Maximum number of results to return. Defaults to 1000",
				"default": 1000
			}
		},
		"required": ["pattern"]
	}`)

	// Check if ripgrep is available
	_, err := exec.LookPath("rg")
	hasRg := err == nil

	return &ContentSearchTool{
		BaseTool: *NewBaseTool(
			"content_search",
			"使用正则表达式搜索工作区内的文件内容。优先使用 ripgrep (rg)，回退到 grep。输出模式：'content' (匹配行)，'files_with_matches' (仅路径)，'count' (每个文件的匹配数)。",
			schema,
		),
		workspaceDir: workspaceDir,
		hasRg:        hasRg,
	}
}

// Execute executes the content search tool.
func (t *ContentSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Parse pattern
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "pattern parameter is required",
		}, nil
	}

	// Parse optional parameters
	searchPath, _ := args["path"].(string)
	if searchPath == "" {
		searchPath = "."
	}

	outputMode, _ := args["output_mode"].(string)
	if outputMode == "" {
		outputMode = "content"
	}
	if !map[string]bool{"content": true, "files_with_matches": true, "count": true}[outputMode] {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Invalid output_mode '%s'. Allowed: content, files_with_matches, count", outputMode),
		}, nil
	}

	include, _ := args["include"].(string)
	caseSensitive := true
	if v, ok := args["case_sensitive"].(bool); ok {
		caseSensitive = v
	}

	contextBefore := 0
	if v, ok := args["context_before"].(float64); ok {
		contextBefore = int(v)
	}
	contextAfter := 0
	if v, ok := args["context_after"].(float64); ok {
		contextAfter = int(v)
	}

	maxResults := maxSearchResults
	if v, ok := args["max_results"].(float64); ok {
		maxResults = int(v)
		if maxResults > maxSearchResults {
			maxResults = maxSearchResults
		}
	}

	// Security: reject absolute paths
	if filepath.IsAbs(searchPath) {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "Absolute paths are not allowed. Use a relative path.",
		}, nil
	}

	// Security: reject path traversal
	if strings.Contains(searchPath, "../") || strings.Contains(searchPath, "..\\") || searchPath == ".." {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "Path traversal ('..') is not allowed.",
		}, nil
	}

	// Resolve workspace
	workspace := t.workspaceDir
	if workspace == "" {
		wd, _ := os.Getwd()
		workspace = wd
	}

	resolvedPath := filepath.Join(workspace, searchPath)

	// Build and execute command
	var cmd *exec.Cmd
	if t.hasRg {
		cmd = t.buildRgCommand(pattern, resolvedPath, outputMode, include, caseSensitive, contextBefore, contextAfter)
	} else {
		cmd = t.buildGrepCommand(pattern, resolvedPath, outputMode, include, caseSensitive, contextBefore, contextAfter)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Exit code 1 means no matches found, which is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &ToolResult{
				Success: true,
				Output:  "No matches found.",
			}, nil
		}
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Search failed: %v", err),
		}, nil
	}

	// Format output
	result := t.formatOutput(string(output), workspace, outputMode, maxResults)

	return &ToolResult{
		Success: true,
		Output:  result,
	}, nil
}

func (t *ContentSearchTool) buildRgCommand(pattern, searchPath, outputMode, include string, caseSensitive bool, contextBefore, contextAfter int) *exec.Cmd {
	args := []string{"--no-heading", "--line-number", "--with-filename"}

	switch outputMode {
	case "files_with_matches":
		args = append(args, "--files-with-matches")
	case "count":
		args = append(args, "--count")
	default:
		if contextBefore > 0 {
			args = append(args, "-B", fmt.Sprintf("%d", contextBefore))
		}
		if contextAfter > 0 {
			args = append(args, "-A", fmt.Sprintf("%d", contextAfter))
		}
	}

	if !caseSensitive {
		args = append(args, "-i")
	}

	if include != "" {
		args = append(args, "--glob", include)
	}

	args = append(args, "--", pattern, searchPath)

	return exec.Command("rg", args...)
}

func (t *ContentSearchTool) buildGrepCommand(pattern, searchPath, outputMode, include string, caseSensitive bool, contextBefore, contextAfter int) *exec.Cmd {
	args := []string{"-r", "-n", "-E", "--binary-files=without-match"}

	switch outputMode {
	case "files_with_matches":
		args = append(args, "-l")
	case "count":
		args = append(args, "-c")
	default:
		if contextBefore > 0 {
			args = append(args, "-B", fmt.Sprintf("%d", contextBefore))
		}
		if contextAfter > 0 {
			args = append(args, "-A", fmt.Sprintf("%d", contextAfter))
		}
	}

	if !caseSensitive {
		args = append(args, "-i")
	}

	if include != "" {
		args = append(args, "--include", include)
	}

	args = append(args, "--", pattern, searchPath)

	return exec.Command("grep", args...)
}

func (t *ContentSearchTool) formatOutput(raw, workspace, outputMode string, maxResults int) string {
	if strings.TrimSpace(raw) == "" {
		return "No matches found."
	}

	lines := strings.Split(raw, "\n")
	var results []string
	fileSet := make(map[string]bool)
	totalMatches := 0
	truncated := false

	workspacePrefix := workspace + string(filepath.Separator)

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Relativize path
		relPath := strings.TrimPrefix(line, workspacePrefix)

		switch outputMode {
		case "files_with_matches":
			path := strings.TrimSpace(relPath)
			if path != "" && !fileSet[path] {
				fileSet[path] = true
				results = append(results, path)
				if len(results) >= maxResults {
					truncated = true
					break
				}
			}
		case "count":
			if parts := strings.SplitN(relPath, ":", 2); len(parts) == 2 {
				path := parts[0]
				fileSet[path] = true
				results = append(results, relPath)
				if len(results) >= maxResults {
					truncated = true
					break
				}
			}
		default:
			// Content mode
			if parts := strings.SplitN(relPath, ":", 3); len(parts) >= 2 {
				fileSet[parts[0]] = true
				totalMatches++
			}
			results = append(results, relPath)
			if len(results) >= maxResults {
				truncated = true
				break
			}
		}
	}

	if len(results) == 0 {
		return "No matches found."
	}

	output := strings.Join(results, "\n")

	if truncated {
		output += fmt.Sprintf("\n\n[Results truncated: showing first %d results]", maxResults)
	}

	switch outputMode {
	case "files_with_matches":
		output += fmt.Sprintf("\n\nTotal: %d files", len(fileSet))
	case "count":
		output += fmt.Sprintf("\n\nTotal: %d files with matches", len(fileSet))
	default:
		output += fmt.Sprintf("\n\nTotal: %d matching lines in %d files", totalMatches, len(fileSet))
	}

	return output
}

// Native content search implementation (fallback when no grep/rg available)
type nativeContentSearch struct {
	workspaceDir string
}

func searchContentNative(workspace, pattern, searchPath, outputMode, include string, caseSensitive bool, contextBefore, contextAfter, maxResults int) (string, error) {
	fullPath := filepath.Join(workspace, searchPath)

	regexFlags := ""
	if !caseSensitive {
		regexFlags = "(?i)"
	}
	re, err := regexp.Compile(regexFlags + pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	var results []string
	fileSet := make(map[string]bool)
	totalMatches := 0

	err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check include filter
		if include != "" {
			matched, _ := filepath.Match(include, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		relPath, _ := filepath.Rel(workspace, path)

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var lines []string

		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		for i, line := range lines {
			_ = i + 1 // line number for future use
			if re.MatchString(line) {
				fileSet[relPath] = true

				switch outputMode {
				case "files_with_matches":
					results = append(results, relPath)
					return nil // One match is enough for this mode
				case "count":
					totalMatches++
				default:
					// Content mode with context
					start := i - contextBefore
					if start < 0 {
						start = 0
					}
					end := i + contextAfter + 1
					if end > len(lines) {
						end = len(lines)
					}

					for j := start; j < end; j++ {
						prefix := " "
						if j == i {
							prefix = ">"
						}
						results = append(results, fmt.Sprintf("%s:%d:%s%s", relPath, j+1, prefix, lines[j]))
					}
				}

				if len(results) >= maxResults {
					return fmt.Errorf("stop")
				}
			}
		}

		if outputMode == "count" && totalMatches > 0 {
			results = append(results, fmt.Sprintf("%s:%d", relPath, totalMatches))
		}

		return nil
	})

	if err != nil && err.Error() != "stop" {
		return "", err
	}

	if len(results) == 0 {
		return "No matches found.", nil
	}

	output := strings.Join(results, "\n")
	if len(fileSet) > 0 {
		output += fmt.Sprintf("\n\nTotal: %d files", len(fileSet))
	}

	return output, nil
}
