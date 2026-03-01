// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zeroclaw-labs/goclaw/pkg/memory"
)

// MemoryStoreTool stores memories in the memory backend.
type MemoryStoreTool struct {
	BaseTool
	backend memory.MemoryBackend
}

// NewMemoryStoreTool creates a new MemoryStoreTool.
func NewMemoryStoreTool(backend memory.MemoryBackend) *MemoryStoreTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"key": {
				"type": "string",
				"description": "Unique key for this memory (e.g. 'user_lang', 'project_stack')"
			},
			"content": {
				"type": "string",
				"description": "The information to remember"
			},
			"category": {
				"type": "string",
				"description": "Memory category: 'core' (permanent), 'daily' (session), 'conversation' (chat), or a custom category name. Defaults to 'core'."
			}
		},
		"required": ["key", "content"]
	}`)
	return &MemoryStoreTool{
		BaseTool: *NewBaseTool(
			"memory_store",
			"将事实、偏好或笔记存储到长期记忆中。类别 'core' 用于永久事实，'daily' 用于会话笔记，'conversation' 用于对话上下文，或使用自定义类别名称。",
			schema,
		),
		backend: backend,
	}
}

// Execute executes the memory store tool.
func (t *MemoryStoreTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	key, ok := args["key"].(string)
	if !ok || key == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "key parameter is required",
		}, nil
	}

	content, ok := args["content"].(string)
	if !ok || content == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "content parameter is required",
		}, nil
	}

	category := "core"
	if c, ok := args["category"].(string); ok && c != "" {
		category = c
	}

	err := t.backend.Store(ctx, key, content, &category, nil)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to store memory: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Stored memory: %s", key),
	}, nil
}

// MemoryRecallTool recalls memories from the memory backend.
type MemoryRecallTool struct {
	BaseTool
	backend memory.MemoryBackend
}

// NewMemoryRecallTool creates a new MemoryRecallTool.
func NewMemoryRecallTool(backend memory.MemoryBackend) *MemoryRecallTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Keywords or phrase to search for in memory"
			},
			"limit": {
				"type": "integer",
				"description": "Max results to return (default: 5)"
			},
			"category": {
				"type": "string",
				"description": "Filter by category (optional)"
			}
		},
		"required": ["query"]
	}`)
	return &MemoryRecallTool{
		BaseTool: *NewBaseTool(
			"memory_recall",
			"搜索长期记忆中相关的事实、偏好或上下文。返回按相关性排序的评分结果。",
			schema,
		),
		backend: backend,
	}
}

// Execute executes the memory recall tool.
func (t *MemoryRecallTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "query parameter is required",
		}, nil
	}

	limit := 5
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	var category *string
	if c, ok := args["category"].(string); ok && c != "" {
		category = &c
	}

	entries, err := t.backend.Recall(ctx, query, limit, category)
	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Memory recall failed: %v", err),
		}, nil
	}

	if len(entries) == 0 {
		return &ToolResult{
			Success: true,
			Output:  "No memories found matching that query.",
		}, nil
	}

	output := fmt.Sprintf("Found %d memories:\n", len(entries))
	for _, entry := range entries {
		cat := "unknown"
		if entry.Category != nil {
			cat = *entry.Category
		}
		output += fmt.Sprintf("- [%s] %s: %s\n", cat, entry.Key, entry.Content)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
	}, nil
}

// MemoryForgetTool deletes a memory entry.
type MemoryForgetTool struct {
	BaseTool
	backend memory.MemoryBackend
}

// NewMemoryForgetTool creates a new MemoryForgetTool.
func NewMemoryForgetTool(backend memory.MemoryBackend) *MemoryForgetTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"key": {
				"type": "string",
				"description": "The key of the memory to forget"
			}
		},
		"required": ["key"]
	}`)
	return &MemoryForgetTool{
		BaseTool: *NewBaseTool(
			"memory_forget",
			"按键删除记忆。用于删除过时事实或敏感数据。返回是否找到并删除了记忆。",
			schema,
		),
		backend: backend,
	}
}

// Execute executes the memory forget tool.
func (t *MemoryForgetTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	key, ok := args["key"].(string)
	if !ok || key == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "key parameter is required",
		}, nil
	}

	err := t.backend.Forget(ctx, key)
	if err != nil {
		if err == memory.ErrNotFound {
			return &ToolResult{
				Success: true,
				Output:  fmt.Sprintf("No memory found with key: %s", key),
			}, nil
		}
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Failed to forget memory: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Forgot memory: %s", key),
	}, nil
}
