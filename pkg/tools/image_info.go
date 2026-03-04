// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxImageBytes = 5 * 1024 * 1024 // 5 MB

// ImageInfoTool reads image metadata and optionally returns base64 data.
type ImageInfoTool struct {
	BaseTool
	workspaceDir string
}

// NewImageInfoTool creates a new ImageInfoTool.
func NewImageInfoTool(workspaceDir string) *ImageInfoTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the image file (absolute or relative to workspace)"
			},
			"include_base64": {
				"type": "boolean",
				"description": "Include base64-encoded image data in output (default: false)"
			}
		},
		"required": ["path"]
	}`)
	return &ImageInfoTool{
		BaseTool: *NewBaseTool(
			"image_info",
			"读取图像文件元数据（格式、尺寸、大小），可选返回 base64 编码数据。",
			schema,
		),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the image info tool.
func (t *ImageInfoTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	pathStr, ok := args["path"].(string)
	if !ok || pathStr == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "path parameter is required",
		}, nil
	}

	includeBase64 := false
	if v, ok := args["include_base64"].(bool); ok {
		includeBase64 = v
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

	if info.IsDir() {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Not a file: %s", pathStr),
		}, nil
	}

	fileSize := info.Size()
	if fileSize > maxImageBytes {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Image too large: %d bytes (max %d bytes)", fileSize, maxImageBytes),
		}, nil
	}

	// Read file
	bytes, err := os.ReadFile(fullPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to read file: %v", err),
		}, nil
	}

	// Detect format
	format := detectImageFormat(bytes)
	dimensions := extractDimensions(bytes, format)

	// Build output
	var output strings.Builder
	fmt.Fprintf(&output, "File: %s\n", pathStr)
	fmt.Fprintf(&output, "Format: %s\n", format)
	fmt.Fprintf(&output, "Size: %d bytes", fileSize)

	if dimensions != nil {
		fmt.Fprintf(&output, "\nDimensions: %dx%d", dimensions.Width, dimensions.Height)
	}

	if includeBase64 {
		mime := formatToMIME(format)
		encoded := base64.StdEncoding.EncodeToString(bytes)
		fmt.Fprintf(&output, "\ndata:%s;base64,%s", mime, encoded)
	}

	return &ToolResult{
		Success: true,
		Output:  output.String(),
	}, nil
}

type dimensions struct {
	Width, Height int
}

func detectImageFormat(bytes []byte) string {
	if len(bytes) < 4 {
		return "unknown"
	}

	// PNG
	if bytes[0] == 0x89 && bytes[1] == 0x50 && bytes[2] == 0x4E && bytes[3] == 0x47 {
		return "png"
	}

	// JPEG
	if bytes[0] == 0xFF && bytes[1] == 0xD8 && bytes[2] == 0xFF {
		return "jpeg"
	}

	// GIF
	if bytes[0] == 0x47 && bytes[1] == 0x49 && bytes[2] == 0x46 {
		return "gif"
	}

	// WEBP
	if len(bytes) >= 12 && bytes[0] == 0x52 && bytes[1] == 0x49 && bytes[2] == 0x46 && bytes[3] == 0x46 &&
		bytes[8] == 0x57 && bytes[9] == 0x45 && bytes[10] == 0x42 && bytes[11] == 0x50 {
		return "webp"
	}

	// BMP
	if bytes[0] == 0x42 && bytes[1] == 0x4D {
		return "bmp"
	}

	return "unknown"
}

func extractDimensions(bytes []byte, format string) *dimensions {
	switch format {
	case "png":
		if len(bytes) >= 24 {
			w := int(binary.BigEndian.Uint32(bytes[16:20]))
			h := int(binary.BigEndian.Uint32(bytes[20:24]))
			return &dimensions{w, h}
		}
	case "gif":
		if len(bytes) >= 10 {
			w := int(binary.LittleEndian.Uint16(bytes[6:8]))
			h := int(binary.LittleEndian.Uint16(bytes[8:10]))
			return &dimensions{w, h}
		}
	case "bmp":
		if len(bytes) >= 26 {
			w := int(binary.LittleEndian.Uint32(bytes[18:22]))
			h := int(binary.LittleEndian.Uint32(bytes[22:26]))
			return &dimensions{w, h}
		}
	case "jpeg":
		return extractJPEGDimensions(bytes)
	}
	return nil
}

func extractJPEGDimensions(bytes []byte) *dimensions {
	if len(bytes) < 4 {
		return nil
	}

	i := 2 // Skip SOI marker
	for i+1 < len(bytes) {
		if bytes[i] != 0xFF {
			return nil
		}

		marker := bytes[i+1]
		i += 2

		// SOF0..SOF3 markers contain dimensions
		if marker >= 0xC0 && marker <= 0xC3 {
			if i+7 <= len(bytes) {
				h := int(binary.BigEndian.Uint16(bytes[i+3 : i+5]))
				w := int(binary.BigEndian.Uint16(bytes[i+5 : i+7]))
				return &dimensions{w, h}
			}
			return nil
		}

		// Skip this segment
		if i+1 < len(bytes) {
			segLen := int(binary.BigEndian.Uint16(bytes[i : i+2]))
			if segLen < 2 {
				return nil
			}
			i += segLen
		} else {
			return nil
		}
	}
	return nil
}

func formatToMIME(format string) string {
	switch format {
	case "png":
		return "image/png"
	case "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}
