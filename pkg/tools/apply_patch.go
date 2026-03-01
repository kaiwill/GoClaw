package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ApplyPatchTool 安全地检查/应用统一差异到当前 git 存储库
type ApplyPatchTool struct {
	BaseTool
	maxPatchSize int
}

// NewApplyPatchTool 创建新的 ApplyPatchTool
func NewApplyPatchTool() *ApplyPatchTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"patch": {
				"type": "string",
				"description": "统一差异文本 (例如 git diff 的输出)"
			},
			"dry_run": {
				"type": "boolean",
				"description": "如果为 true，只检查补丁是否能干净地应用 (不做任何更改)",
				"default": true
			},
			"commit_message": {
				"type": "string",
				"description": "如果提供 (且 dry_run=false)，暂存所有更改并创建 git 提交"
			}
		},
		"required": ["patch"]
	}`)
	return &ApplyPatchTool{
		BaseTool:     *NewBaseTool("apply_patch", "安全地检查/应用统一差异到当前 git 存储库，可选地暂存和提交。", schema),
		maxPatchSize: 1_000_000, // 1MB
	}
}

// Execute 执行工具
func (t *ApplyPatchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	patch, _ := args["patch"].(string)

	dryRun := true
	if dr, ok := args["dry_run"].(bool); ok {
		dryRun = dr
	}

	commitMessage, _ := args["commit_message"].(string)

	if patch == "" {
		return &ToolResult{
			Success: false,
			Error:   "缺少必需字段: patch (string)",
		}, nil
	}

	// 大小限制
	if len(patch) > t.maxPatchSize {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("补丁太大 (%d 字节)。拒绝处理 (> %d 字节)", len(patch), t.maxPatchSize),
		}, nil
	}

	// 获取 git 仓库根目录
	repoRoot, err := t.gitRepoRoot()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("获取 git 仓库根目录失败: %v", err),
		}, nil
	}

	var log strings.Builder
	fmt.Fprintf(&log, "Repo root: %s\n", repoRoot)
	if dryRun {
		fmt.Fprintf(&log, "Mode: dry-run\n")
	} else {
		fmt.Fprintf(&log, "Mode: apply\n")
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "patch-*.diff")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(patch); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("写入补丁文件失败: %w", err)
	}
	tmpFile.Close()

	// 先检查补丁
	log.WriteString("\n# git apply --check\n")
	checkCode, checkOut, checkErr := t.runCmd(repoRoot, "git", "apply", "--check", tmpPath)
	fmt.Fprintf(&log, "exit_code: %d\n", checkCode)
	if checkOut != "" {
		log.WriteString("stdout:\n" + checkOut + "\n")
	}
	if checkErr != "" {
		log.WriteString("stderr:\n" + checkErr + "\n")
	}

	if checkCode != 0 {
		return &ToolResult{
			Success: false,
			Output:  log.String(),
			Error:   "补丁检查失败 (git apply --check)。未做任何更改。",
		}, nil
	}

	if dryRun {
		log.WriteString("\n补丁检查通过。dry-run 模式，未应用任何更改。\n")
		return &ToolResult{
			Success: true,
			Output:  log.String(),
		}, nil
	}

	// 应用补丁
	log.WriteString("\n# git apply\n")
	applyCode, applyOut, applyErr := t.runCmd(repoRoot, "git", "apply", tmpPath)
	fmt.Fprintf(&log, "exit_code: %d\n", applyCode)
	if applyOut != "" {
		log.WriteString("stdout:\n" + applyOut + "\n")
	}
	if applyErr != "" {
		log.WriteString("stderr:\n" + applyErr + "\n")
	}

	if applyCode != 0 {
		return &ToolResult{
			Success: false,
			Output:  log.String(),
			Error:   "git apply 失败。补丁可能未应用。",
		}, nil
	}

	// 显示状态
	log.WriteString("\n# git status --porcelain\n")
	_, statusOut, _ := t.runCmd(repoRoot, "git", "status", "--porcelain")
	if strings.TrimSpace(statusOut) == "" {
		log.WriteString("(no changes)\n")
	} else {
		log.WriteString(statusOut)
		if !strings.HasSuffix(statusOut, "\n") {
			log.WriteString("\n")
		}
	}

	// 可选提交
	if commitMessage != "" {
		// git add -A
		log.WriteString("\n# git add -A\n")
		addCode, _, addErr := t.runCmd(repoRoot, "git", "add", "-A")
		fmt.Fprintf(&log, "exit_code: %d\n", addCode)
		if addErr != "" {
			log.WriteString("stderr:\n" + addErr + "\n")
		}
		if addCode != 0 {
			return &ToolResult{
				Success: false,
				Output:  log.String(),
				Error:   "git add 失败",
			}, nil
		}

		// git commit -m
		log.WriteString("\n# git commit -m <msg>\n")
		commitCode, commitOut, commitErr := t.runCmd(repoRoot, "git", "commit", "-m", commitMessage)
		fmt.Fprintf(&log, "exit_code: %d\n", commitCode)
		if commitOut != "" {
			log.WriteString("stdout:\n" + commitOut + "\n")
		}
		if commitErr != "" {
			log.WriteString("stderr:\n" + commitErr + "\n")
		}

		if commitCode != 0 {
			return &ToolResult{
				Success: false,
				Output:  log.String(),
				Error:   "git commit 失败 (可能没有内容可提交，或 hooks 拒绝)",
			}, nil
		}

		// 显示最后提交摘要
		log.WriteString("\n# git show --stat --oneline -1\n")
		_, showOut, _ := t.runCmd(repoRoot, "git", "show", "--stat", "--oneline", "-1")
		if showOut != "" {
			log.WriteString(showOut)
			if !strings.HasSuffix(showOut, "\n") {
				log.WriteString("\n")
			}
		}
	}

	return &ToolResult{
		Success: true,
		Output:  log.String(),
	}, nil
}

// gitRepoRoot 获取 git 仓库根目录
func (t *ApplyPatchTool) gitRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	code, out, errStr := t.runCmd(cwd, "git", "rev-parse", "--show-toplevel")
	if code != 0 {
		return "", fmt.Errorf("不是 git 仓库 (git rev-parse 失败): %s", strings.TrimSpace(errStr))
	}

	root := strings.TrimSpace(out)
	if root == "" {
		return "", fmt.Errorf("git rev-parse 返回空仓库根目录")
	}

	return root, nil
}

// runCmd 执行命令并返回退出码、stdout、stderr
func (t *ApplyPatchTool) runCmd(dir, name string, args ...string) (int, string, string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return exitCode, stdout.String(), stderr.String()
}