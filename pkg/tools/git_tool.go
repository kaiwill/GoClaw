// Package tools provides built-in tools for the GoClaw agent.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

// GitTool provides git operations functionality.
type GitTool struct {
	workspaceDir string
}

// NewGitTool creates a new GitTool instance.
func NewGitTool(workspaceDir string) *GitTool {
	return &GitTool{
		workspaceDir: workspaceDir,
	}
}

// Name returns the tool name.
func (t *GitTool) Name() string {
	return "git"
}

// Description returns the tool description.
func (t *GitTool) Description() string {
	return "Perform git operations: status, diff, log, commit, push, pull, etc."
}

// Parameters returns the tool parameters.
func (t *GitTool) Parameters() json.RawMessage {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"description": "Git operation to perform: status, diff, log, commit, push, pull, branch, checkout, etc.",
				"enum": []string{"status", "diff", "log", "commit", "push", "pull", "branch", "checkout"},
			},
			"args": map[string]interface{}{
				"type": "string",
				"description": "Additional arguments for the git command",
			},
			"message": map[string]interface{}{
				"type": "string",
				"description": "Commit message for commit operation",
			},
			"branch": map[string]interface{}{
					"type": "string",
					"description": "Branch name for checkout or branch operations",
				},
		},
		"required": []string{"operation"},
	}
	jsonParams, _ := json.Marshal(params)
	return jsonParams
}

// Execute runs the git tool.
func (t *GitTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var params struct {
		Operation string `json:"operation"`
		Args      string `json:"args,omitempty"`
		Message   string `json:"message,omitempty"`
		Branch    string `json:"branch,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse git tool arguments: %w", err)
	}

	// Sanitize arguments to prevent injection
	if err := t.sanitizeArgs(params.Args); err != nil {
		return nil, fmt.Errorf("invalid git arguments: %w", err)
	}

	// Build git command
	var cmdArgs []string
	switch params.Operation {
	case "status":
		cmdArgs = []string{"status", "--porcelain=2", "--branch"}
	case "diff":
		cmdArgs = []string{"diff", "--unified=3"}
		if params.Args != "" {
			cmdArgs = append(cmdArgs, params.Args)
		}
	case "log":
		cmdArgs = []string{"log", "--oneline", "--graph", "--decorate", "-n", "20"}
	case "commit":
		if params.Message == "" {
			return nil, fmt.Errorf("commit message is required for commit operation")
		}
		cmdArgs = []string{"commit", "-m", params.Message}
	case "push":
		cmdArgs = []string{"push"}
	case "pull":
		cmdArgs = []string{"pull"}
	case "branch":
		cmdArgs = []string{"branch"}
	case "checkout":
		if params.Branch == "" {
			return nil, fmt.Errorf("branch name is required for checkout operation")
		}
		cmdArgs = []string{"checkout", params.Branch}
	default:
		return nil, fmt.Errorf("unsupported git operation: %s", params.Operation)
	}

	// Run git command
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = t.workspaceDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git command failed: %w, stderr: %s", err, stderr.String())
	}

	// Parse output for structured results
	output := stdout.String()
	if params.Operation == "status" {
		parsed, err := t.parseGitStatus(output)
		if err != nil {
			return nil, fmt.Errorf("failed to parse git status: %w", err)
		}
		jsonOutput, err := json.MarshalIndent(parsed, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal git status: %w", err)
		}
		result := types.NewToolResult(string(jsonOutput))
		return &result, nil
	}

	result := types.NewToolResult(output)
	return &result, nil
}

// sanitizeArgs prevents git command injection.
func (t *GitTool) sanitizeArgs(args string) error {
	// Block dangerous git options
	dangerousOptions := []string{"--exec=", "--upload-pack=", "--receive-pack=", "--pager=", "--editor=", "--no-verify", "-c"}
	for _, opt := range dangerousOptions {
		if strings.Contains(args, opt) {
			return fmt.Errorf("dangerous git option '%s' is blocked", opt)
		}
	}

	// Block shell injection characters
	dangerousChars := []string{"$", "`", "|", ";", ">"}
	for _, c := range dangerousChars {
		if strings.Contains(args, c) {
			return fmt.Errorf("dangerous character '%s' is blocked", c)
		}
	}

	return nil
}

// parseGitStatus parses git status output into structured format.
func (t *GitTool) parseGitStatus(output string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	var branch string
	var staged []map[string]interface{}
	var unstaged []map[string]interface{}
	var untracked []string

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "# branch.head ") {
			branch = strings.TrimPrefix(line, "# branch.head ")
		} else if strings.HasPrefix(line, "1 ") {
			parts := strings.SplitN(line[2:], " ", 3)
			if len(parts) < 2 {
				continue
			}

			staging := parts[0]
			path := parts[1]

			if len(staging) >= 1 && staging[0] != '.' && staging[0] != ' ' {
				staged = append(staged, map[string]interface{}{
					"path": path,
					"status": string(staging[0]),
				})
			}

			if len(staging) >= 2 && staging[1] != '.' && staging[1] != ' ' {
				unstaged = append(unstaged, map[string]interface{}{
					"path": path,
					"status": string(staging[1]),
				})
			}
		} else if strings.HasPrefix(line, "? ") {
			untracked = append(untracked, strings.TrimPrefix(line, "? "))
		}
	}

	result["branch"] = branch
	result["staged"] = staged
	result["unstaged"] = unstaged
	result["untracked"] = untracked
	result["clean"] = len(staged) == 0 && len(unstaged) == 0 && len(untracked) == 0

	return result, nil
}