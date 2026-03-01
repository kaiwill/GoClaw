// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	maxPDFBytes    = 50 * 1024 * 1024 // 50 MB
	defaultMaxChars = 50000
	maxOutputChars  = 200000
)

// PDFReadTool extracts plain text from a PDF file.
type PDFReadTool struct {
	BaseTool
	workspaceDir string
}

// NewPDFReadTool creates a new PDFReadTool.
func NewPDFReadTool(workspaceDir string) *PDFReadTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the PDF file. Relative paths resolve from workspace."
			},
			"max_chars": {
				"type": "integer",
				"description": "Maximum characters to return (default: 50000, max: 200000)",
				"minimum": 1,
				"maximum": 200000
			}
		},
		"required": ["path"]
	}`)
	return &PDFReadTool{
		BaseTool: *NewBaseTool(
			"pdf_read",
			"从 PDF 文件中提取纯文本。返回所有可读文本。纯图像或加密的 PDF 返回空结果。",
			schema,
		),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the PDF read tool.
func (t *PDFReadTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	pathStr, ok := args["path"].(string)
	if !ok || pathStr == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "path parameter is required",
		}, nil
	}

	maxChars := defaultMaxChars
	if mc, ok := args["max_chars"].(float64); ok {
		mcInt := int(mc)
		if mcInt > 0 && mcInt <= maxOutputChars {
			maxChars = mcInt
		}
	}

	// Resolve path
	var fullPath string
	if filepath.IsAbs(pathStr) {
		fullPath = pathStr
	} else {
		if t.workspaceDir != "" {
			fullPath = filepath.Join(t.workspaceDir, pathStr)
		} else {
			wd, _ := os.Getwd()
			fullPath = filepath.Join(wd, pathStr)
		}
	}

	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("File not found: %s", pathStr),
		}, nil
	}

	if info.Size() > maxPDFBytes {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("PDF too large: %d bytes (max %d bytes)", info.Size(), maxPDFBytes),
		}, nil
	}

	// Try to extract text using pdftotext (poppler-utils)
	text, err := t.extractWithPDFToText(ctx, fullPath)
	if err == nil && text != "" {
		if len(text) > maxChars {
			text = text[:maxChars] + "\n\n[Text truncated due to size limit]"
		}
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("PDF: %s\n\n%s", pathStr, text),
		}, nil
	}

	// Fallback: try pdftk or python pdfplumber
	text, err = t.extractWithPython(ctx, fullPath)
	if err == nil && text != "" {
		if len(text) > maxChars {
			text = text[:maxChars] + "\n\n[Text truncated due to size limit]"
		}
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("PDF: %s\n\n%s", pathStr, text),
		}, nil
	}

	return &ToolResult{
		Success: false,
		Output:  "",
		Error:   "PDF extraction failed. Install poppler-utils (pdftotext) or python3 with pdfplumber.",
	}, nil
}

func (t *PDFReadTool) extractWithPDFToText(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-q", path, "-")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (t *PDFReadTool) extractWithPython(ctx context.Context, path string) (string, error) {
	script := `
import sys
try:
    import pdfplumber
    with pdfplumber.open(sys.argv[1]) as pdf:
        text = ""
        for page in pdf.pages:
            page_text = page.extract_text()
            if page_text:
                text += page_text + "\n"
        print(text)
except ImportError:
    sys.exit(1)
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
    sys.exit(1)
`
	cmd := exec.CommandContext(ctx, "python3", "-c", script, path)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
