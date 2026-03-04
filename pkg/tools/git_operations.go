package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var dangerousGitPatterns = []*regexp.Regexp{
	regexp.MustCompile(`--exec=`),
	regexp.MustCompile(`--upload-pack=`),
	regexp.MustCompile(`--receive-pack=`),
	regexp.MustCompile(`--pager=`),
	regexp.MustCompile(`--editor=`),
	regexp.MustCompile(`--no-verify`),
	regexp.MustCompile(`\$\(`),
	regexp.MustCompile("`"),
	regexp.MustCompile(`\|`),
	regexp.MustCompile(`;`),
	regexp.MustCompile(`>`),
	regexp.MustCompile(`^-c$`),
	regexp.MustCompile(`^-c=`),
}

type GitOperationsTool struct {
	workspaceDir string
}

func NewGitOperationsTool(workspaceDir string) *GitOperationsTool {
	return &GitOperationsTool{
		workspaceDir: workspaceDir,
	}
}

func (t *GitOperationsTool) Name() string {
	return "git_operations"
}

func (t *GitOperationsTool) Description() string {
	return "Git 操作工具，用于仓库管理，包括 status、diff、log、commit、branch 等操作"
}

func (t *GitOperationsTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"operation": {
				"type": "string",
				"enum": ["status", "diff", "log", "commit", "add", "branch", "checkout", "stash", "reset", "remote"],
				"description": "The git operation to perform"
			},
			"args": {
				"type": "object",
				"description": "Arguments for the operation"
			}
		},
		"required": ["operation"]
	}`)
}

func (t *GitOperationsTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.ParametersSchema(),
	}
}

func (t *GitOperationsTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "operation is required",
		}, nil
	}

	opArgs, _ := args["args"].(map[string]interface{})

	switch operation {
	case "status":
		return t.gitStatus(ctx, opArgs)
	case "diff":
		return t.gitDiff(ctx, opArgs)
	case "log":
		return t.gitLog(ctx, opArgs)
	case "commit":
		return t.gitCommit(ctx, opArgs)
	case "add":
		return t.gitAdd(ctx, opArgs)
	case "branch":
		return t.gitBranch(ctx, opArgs)
	case "checkout":
		return t.gitCheckout(ctx, opArgs)
	case "stash":
		return t.gitStash(ctx, opArgs)
	case "reset":
		return t.gitReset(ctx, opArgs)
	case "remote":
		return t.gitRemote(ctx, opArgs)
	default:
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("unknown operation: %s", operation),
		}, nil
	}
}

func (t *GitOperationsTool) sanitizeGitArgs(args string) error {
	for _, pattern := range dangerousGitPatterns {
		if pattern.MatchString(args) {
			return fmt.Errorf("blocked potentially dangerous git argument: %s", args)
		}
	}
	return nil
}

func (t *GitOperationsTool) runGitCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = t.workspaceDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s", stderr.String())
	}

	return stdout.String(), nil
}

func (t *GitOperationsTool) gitStatus(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	output, err := t.runGitCommand(ctx, "status", "--porcelain=2", "--branch")
	if err != nil {
		return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
	}

	var result map[string]interface{}
	result = make(map[string]interface{})

	branch := ""
	staged := []interface{}{}
	unstaged := []interface{}{}
	untracked := []string{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# branch.head ") {
			branch = strings.TrimPrefix(line, "# branch.head ")
		} else if strings.HasPrefix(line, "1 ") {
			rest := strings.TrimPrefix(line, "1 ")
			parts := strings.SplitN(rest, " ", 3)
			if len(parts) >= 2 {
				staging := parts[0]
				path := parts[len(parts)-1]
				if len(staging) > 0 && staging[0] != '.' && staging[0] != ' ' {
					staged = append(staged, map[string]interface{}{"path": path, "status": string(staging[0])})
				}
				if len(staging) > 1 && staging[1] != '.' && staging[1] != ' ' {
					unstaged = append(unstaged, map[string]interface{}{"path": path, "status": string(staging[1])})
				}
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

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return &ToolResult{Success: true, Output: string(jsonBytes)}, nil
}

func (t *GitOperationsTool) gitDiff(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	files := "."
	if f, ok := args["files"].(string); ok {
		if err := t.sanitizeGitArgs(f); err != nil {
			return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
		}
		files = f
	}

	cached, _ := args["cached"].(bool)

	gitArgs := []string{"diff", "--unified=3"}
	if cached {
		gitArgs = append(gitArgs, "--cached")
	}
	gitArgs = append(gitArgs, "--", files)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
	}

	return &ToolResult{Success: true, Output: output}, nil
}

func (t *GitOperationsTool) gitLog(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	maxCount := 10
	if mc, ok := args["max_count"].(float64); ok {
		maxCount = int(mc)
	}

	format := "%H|%an|%ae|%at|%s"
	if f, ok := args["format"].(string); ok {
		format = f
	}

	gitArgs := []string{"log", fmt.Sprintf("--max-count=%d", maxCount), fmt.Sprintf("--format=%s", format)}

	if since, ok := args["since"].(string); ok {
		gitArgs = append(gitArgs, fmt.Sprintf("--since=%s", since))
	}
	if until, ok := args["until"].(string); ok {
		gitArgs = append(gitArgs, fmt.Sprintf("--until=%s", until))
	}

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
	}

	commits := []map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) >= 5 {
			commits = append(commits, map[string]string{
				"hash":    parts[0],
				"author":  parts[1],
				"email":   parts[2],
				"date":    parts[3],
				"message": parts[4],
			})
		}
	}

	jsonBytes, _ := json.MarshalIndent(commits, "", "  ")
	return &ToolResult{Success: true, Output: string(jsonBytes)}, nil
}

func (t *GitOperationsTool) gitCommit(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return &ToolResult{Success: false, Output: "", Error: "commit message is required"}, nil
	}

	all := true
	if a, ok := args["all"].(bool); ok {
		all = a
	}

	gitArgs := []string{"commit"}
	if all {
		gitArgs = append(gitArgs, "-a")
	}
	gitArgs = append(gitArgs, "-m", message)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
	}

	return &ToolResult{Success: true, Output: output}, nil
}

func (t *GitOperationsTool) gitAdd(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	files := "."
	if f, ok := args["files"].(string); ok {
		if err := t.sanitizeGitArgs(f); err != nil {
			return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
		}
		files = f
	}

	gitArgs := []string{"add"}
	if all, _ := args["all"].(bool); all {
		gitArgs = append(gitArgs, "-A")
	}
	gitArgs = append(gitArgs, "--", files)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
	}

	return &ToolResult{Success: true, Output: output}, nil
}

func (t *GitOperationsTool) gitBranch(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	list, _ := args["list"].(bool)
	if list || (args["operation"] == nil) {
		output, err := t.runGitCommand(ctx, "branch", "-a")
		if err != nil {
			return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
		}
		branches := strings.Split(strings.TrimSpace(output), "\n")
		return &ToolResult{Success: true, Output: strings.Join(branches, "\n")}, nil
	}

	name, ok := args["name"].(string)
	if !ok || name == "" {
		return &ToolResult{Success: false, Output: "", Error: "branch name is required"}, nil
	}

	createNew, _ := args["create"].(bool)
	if createNew {
		output, err := t.runGitCommand(ctx, "branch", name)
		if err != nil {
			return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
		}
		return &ToolResult{Success: true, Output: output}, nil
	}

	return &ToolResult{Success: false, Output: "", Error: "unsupported branch operation"}, nil
}

func (t *GitOperationsTool) gitCheckout(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	branch, ok := args["branch"].(string)
	if !ok || branch == "" {
		return &ToolResult{Success: false, Output: "", Error: "branch is required"}, nil
	}

	if err := t.sanitizeGitArgs(branch); err != nil {
		return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
	}

	newBranch, _ := args["new"].(bool)

	gitArgs := []string{"checkout"}
	if newBranch {
		gitArgs = append(gitArgs, "-b")
	}
	gitArgs = append(gitArgs, branch)

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
	}

	return &ToolResult{Success: true, Output: output}, nil
}

func (t *GitOperationsTool) gitStash(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		action = "push"
	}

	gitArgs := []string{"stash"}

	switch action {
	case "push":
		gitArgs = append(gitArgs, "push")
		if msg, ok := args["message"].(string); ok {
			gitArgs = append(gitArgs, "-m", msg)
		}
	case "pop":
		gitArgs = append(gitArgs, "pop")
	case "list":
		gitArgs = append(gitArgs, "list")
	case "drop":
		if index, ok := args["index"].(float64); ok {
			gitArgs = append(gitArgs, "drop", fmt.Sprintf("stash@{%d}", int(index)))
		}
	case "clear":
		gitArgs = append(gitArgs, "clear")
	default:
		return &ToolResult{Success: false, Output: "", Error: fmt.Sprintf("unknown stash action: %s", action)}, nil
	}

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
	}

	return &ToolResult{Success: true, Output: output}, nil
}

func (t *GitOperationsTool) gitReset(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	mode := "mixed"
	if m, ok := args["mode"].(string); ok {
		mode = m
	}

	commit := "HEAD"
	if c, ok := args["commit"].(string); ok {
		commit = c
	}

	gitArgs := []string{"reset", "--" + mode, commit}

	output, err := t.runGitCommand(ctx, gitArgs...)
	if err != nil {
		return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
	}

	return &ToolResult{Success: true, Output: output}, nil
}

func (t *GitOperationsTool) gitRemote(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	operation, ok := args["operation"].(string)
	if !ok || operation == "" {
		operation = "list"
	}

	switch operation {
	case "list":
		output, err := t.runGitCommand(ctx, "remote", "-v")
		if err != nil {
			return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
		}
		return &ToolResult{Success: true, Output: output}, nil

	case "add":
		name, ok := args["name"].(string)
		url, ok2 := args["url"].(string)
		if !ok || !ok2 {
			return &ToolResult{Success: false, Output: "", Error: "name and url are required"}, nil
		}
		output, err := t.runGitCommand(ctx, "remote", "add", name, url)
		if err != nil {
			return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
		}
		return &ToolResult{Success: true, Output: output}, nil

	case "remove":
		name, ok := args["name"].(string)
		if !ok {
			return &ToolResult{Success: false, Output: "", Error: "name is required"}, nil
		}
		output, err := t.runGitCommand(ctx, "remote", "remove", name)
		if err != nil {
			return &ToolResult{Success: false, Output: "", Error: err.Error()}, nil
		}
		return &ToolResult{Success: true, Output: output}, nil

	default:
		return &ToolResult{Success: false, Output: "", Error: fmt.Sprintf("unknown remote operation: %s", operation)}, nil
	}
}
