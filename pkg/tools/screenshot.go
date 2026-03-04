// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	screenshotTimeoutSecs = 15
	maxBase64Bytes        = 2 * 1024 * 1024 // 2 MB
)

// ScreenshotTool captures screenshots using platform-native commands.
type ScreenshotTool struct {
	BaseTool
	workspaceDir string
}

// NewScreenshotTool creates a new ScreenshotTool.
func NewScreenshotTool(workspaceDir string) *ScreenshotTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"output": {
				"type": "string",
				"description": "Output filename for the screenshot (saved in workspace). If not specified, returns base64 data."
			},
			"display": {
				"type": "integer",
				"description": "Display number to capture (Linux only, default: 0)"
			},
			"window": {
				"type": "string",
				"description": "Window ID or name to capture (platform-specific)"
			}
		}
	}`)
	return &ScreenshotTool{
		BaseTool: *NewBaseTool(
			"screenshot",
			"使用平台原生命令截取屏幕。macOS: screencapture，Linux: gnome-screenshot/scrot/import。返回 base64 数据或保存到文件。",
			schema,
		),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the screenshot tool.
func (t *ScreenshotTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	outputFile, _ := args["output"].(string)
	display := 0
	if d, ok := args["display"].(float64); ok {
		display = int(d)
	}

	// Create temp file for screenshot
	tempDir := os.TempDir()
	timestamp := time.Now().Format("20060102-150405")
	tempFile := filepath.Join(tempDir, fmt.Sprintf("screenshot-%s.png", timestamp))

	// Build screenshot command based on platform
	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(ctx, time.Duration(screenshotTimeoutSecs)*time.Second)
	defer cancel()

	switch runtime.GOOS {
	case "darwin":
		// macOS: screencapture
		cmdArgs := []string{"-x"} // no sound
		if display > 0 {
			cmdArgs = append(cmdArgs, "-D", fmt.Sprintf("%d", display))
		}
		cmdArgs = append(cmdArgs, tempFile)
		cmd = exec.CommandContext(ctx, "screencapture", cmdArgs...)

	case "linux":
		// Linux: try gnome-screenshot, scrot, or import (ImageMagick)
		if t.hasCommand("gnome-screenshot") {
			cmd = exec.CommandContext(ctx, "gnome-screenshot", "-f", tempFile)
		} else if t.hasCommand("scrot") {
			cmd = exec.CommandContext(ctx, "scrot", tempFile)
		} else if t.hasCommand("import") {
			cmd = exec.CommandContext(ctx, "import", "-window", "root", tempFile)
		} else {
			return &ToolResult{
				Success: false,
				Output:  "",
				Error:   "No screenshot command found. Install gnome-screenshot, scrot, or ImageMagick.",
			}, nil
		}

	default:
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Screenshot not supported on %s", runtime.GOOS),
		}, nil
	}

	// Execute screenshot command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Screenshot failed: %v\nOutput: %s", err, string(output)),
		}, nil
	}

	// Read screenshot file
	data, err := os.ReadFile(tempFile)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to read screenshot: %v", err),
		}, nil
	}
	defer os.Remove(tempFile)

	// If output file specified, save to workspace
	if outputFile != "" {
		if t.workspaceDir == "" {
			t.workspaceDir, _ = os.Getwd()
		}
		outputPath := filepath.Join(t.workspaceDir, outputFile)

		// Ensure directory exists
		os.MkdirAll(filepath.Dir(outputPath), 0755)

		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return &ToolResult{
				Success: false,
				Output:  "",
				Error:   fmt.Sprintf("Failed to save screenshot: %v", err),
			}, nil
		}

		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Screenshot saved to: %s (%d bytes)", outputFile, len(data)),
		}, nil
	}

	// Return base64 data
	encoded := base64.StdEncoding.EncodeToString(data)
	if len(encoded) > maxBase64Bytes {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Screenshot too large: %d bytes (max %d)", len(encoded), maxBase64Bytes),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Screenshot captured (%d bytes)\ndata:image/png;base64,%s", len(data), encoded),
	}, nil
}

func (t *ScreenshotTool) hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
