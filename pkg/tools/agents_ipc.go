// Package tools - Inter-process communication tools for independent GoClaw agents.
//
// Provides 5 LLM-callable tools backed by a shared SQLite database, allowing
// independent GoClaw processes on the same host to discover each other and
// exchange messages.
package tools

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ── IpcDb core ──────────────────────────────────────────────────

const pragmaSQL = `
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA busy_timeout=5000;`

const schemaSQL = `
CREATE TABLE IF NOT EXISTS agents (
    agent_id  TEXT PRIMARY KEY,
    role      TEXT,
    status    TEXT DEFAULT 'online',
    metadata  TEXT,
    last_seen INTEGER
);
CREATE TABLE IF NOT EXISTS messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    from_agent TEXT NOT NULL,
    to_agent   TEXT NOT NULL,
    payload    TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    read       INTEGER DEFAULT 0
);
CREATE TABLE IF NOT EXISTS shared_state (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    owner      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);`

// IpcDb 共享 SQLite IPC 数据库
type IpcDb struct {
	conn          *sql.DB
	agentID       string
	stalenessSecs int64
	mu            sync.Mutex
	closed        bool
}

// AgentsIpcConfig IPC 配置
type AgentsIpcConfig struct {
	Enabled       bool   `json:"enabled"`
	DbPath        string `json:"db_path"`
	StalenessSecs int64  `json:"staleness_secs"`
}

// DefaultAgentsIpcConfig 返回默认配置
func DefaultAgentsIpcConfig() AgentsIpcConfig {
	return AgentsIpcConfig{
		Enabled:       false,
		DbPath:        "",
		StalenessSecs: 300,
	}
}

// 全局 IPC 数据库单例
var (
	globalIpcDb *IpcDb
	globalIpcMu sync.Mutex
)

// nowEpoch 返回当前 Unix 时间戳
func nowEpoch() int64 {
	return time.Now().Unix()
}

// InitIpcDb 初始化 IPC 数据库
func InitIpcDb(workspaceDir string, config AgentsIpcConfig) (*IpcDb, error) {
	globalIpcMu.Lock()
	defer globalIpcMu.Unlock()

	// 如果已经初始化，直接返回
	if globalIpcDb != nil && !globalIpcDb.closed {
		return globalIpcDb, nil
	}

	dbPath := config.DbPath
	if dbPath == "" {
		dbPath = filepath.Join(workspaceDir, "ipc.db")
	}

	// 展开 ~ 路径
	if strings.HasPrefix(dbPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("获取用户目录失败: %w", err)
		}
		dbPath = filepath.Join(home, dbPath[2:])
	}

	// 确保目录存在
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据库目录失败: %w", err)
		}
	}

	// 打开数据库
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开 IPC 数据库失败: %w", err)
	}

	// 设置 pragmas
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := conn.Exec(pragma); err != nil {
			conn.Close()
			return nil, fmt.Errorf("设置 pragma 失败: %w", err)
		}
	}

	// 创建 schema
	if _, err := conn.Exec(schemaSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建 schema 失败: %w", err)
	}

	// 从工作目录哈希派生 agent_id
	absPath, err := filepath.Abs(workspaceDir)
	if err != nil {
		absPath = workspaceDir
	}
	hash := sha256.Sum256([]byte(absPath))
	agentID := fmt.Sprintf("%x", hash)

	staleness := config.StalenessSecs
	if staleness == 0 {
		staleness = 300 // 默认 5 分钟
	}

	now := nowEpoch()

	// 注册或更新代理 (UPDATE + INSERT 模式保留现有 role/metadata)
	res, err := conn.Exec(
		"UPDATE agents SET status = 'online', last_seen = ? WHERE agent_id = ?",
		now, agentID,
	)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("更新代理失败: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		if _, err := conn.Exec(
			"INSERT INTO agents (agent_id, status, last_seen) VALUES (?, 'online', ?)",
			agentID, now,
		); err != nil {
			conn.Close()
			return nil, fmt.Errorf("注册代理失败: %w", err)
		}
	}

	db := &IpcDb{
		conn:          conn,
		agentID:       agentID,
		stalenessSecs: staleness,
	}

	globalIpcDb = db
	return db, nil
}

// GetIpcDb 获取全局 IPC 数据库实例
func GetIpcDb() *IpcDb {
	globalIpcMu.Lock()
	defer globalIpcMu.Unlock()
	return globalIpcDb
}

// Heartbeat 更新 last_seen 时间戳
func (db *IpcDb) Heartbeat() {
	if db == nil || db.closed {
		return
	}
	now := nowEpoch()
	db.mu.Lock()
	defer db.mu.Unlock()
	db.conn.Exec("UPDATE agents SET last_seen = ? WHERE agent_id = ?", now, db.agentID)
}

// AgentID 返回代理 ID
func (db *IpcDb) AgentID() string {
	if db == nil {
		return ""
	}
	return db.agentID
}

// StalenessSecs 返回陈旧窗口秒数
func (db *IpcDb) StalenessSecs() int64 {
	if db == nil {
		return 300
	}
	return db.stalenessSecs
}

// Close 关闭数据库并清理代理记录
func (db *IpcDb) Close() error {
	if db == nil || db.closed {
		return nil
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// 删除代理记录
	db.conn.Exec("DELETE FROM agents WHERE agent_id = ?", db.agentID)

	// 关闭连接
	err := db.conn.Close()
	db.closed = true
	return err
}

// SetRole 设置代理角色
func (db *IpcDb) SetRole(role string) error {
	if db == nil {
		return fmt.Errorf("IPC 数据库未初始化")
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.conn.Exec("UPDATE agents SET role = ? WHERE agent_id = ?", role, db.agentID)
	return err
}

// SetMetadata 设置代理元数据
func (db *IpcDb) SetMetadata(metadata string) error {
	if db == nil {
		return fmt.Errorf("IPC 数据库未初始化")
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.conn.Exec("UPDATE agents SET metadata = ? WHERE agent_id = ?", metadata, db.agentID)
	return err
}

// ── AgentsListTool ──────────────────────────────────────────────

// AgentsListTool 列出在线代理
type AgentsListTool struct {
	BaseTool
	ipcDb *IpcDb
}

// NewAgentsListTool 创建代理列表工具
func NewAgentsListTool(ipcDb *IpcDb) *AgentsListTool {
	schema := json.RawMessage(`{"type": "object", "properties": {}, "required": []}`)
	return &AgentsListTool{
		BaseTool: *NewBaseTool("agents_list", "列出此主机上的在线 IPC 代理。返回在陈旧窗口内的代理的代理 ID、角色和最后一次看到的时间戳。", schema),
		ipcDb:    ipcDb,
	}
}

// Execute 执行工具
func (t *AgentsListTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if t.ipcDb == nil {
		return &ToolResult{
			Success: false,
			Error:   "IPC 数据库未初始化",
		}, nil
	}

	t.ipcDb.Heartbeat()

	cutoff := nowEpoch() - t.ipcDb.StalenessSecs()

	t.ipcDb.mu.Lock()
	defer t.ipcDb.mu.Unlock()

	rows, err := t.ipcDb.conn.Query(
		"SELECT agent_id, role, status, last_seen FROM agents WHERE last_seen >= ?",
		cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("查询代理失败: %w", err)
	}
	defer rows.Close()

	var agents []map[string]interface{}
	for rows.Next() {
		var agentID, status string
		var role sql.NullString
		var lastSeen int64
		if err := rows.Scan(&agentID, &role, &status, &lastSeen); err != nil {
			continue
		}
		agents = append(agents, map[string]interface{}{
			"agent_id":  agentID,
			"role":      role.String,
			"status":    status,
			"last_seen": lastSeen,
		})
	}

	output, _ := json.MarshalIndent(agents, "", "  ")
	return &ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ── AgentsSendTool ──────────────────────────────────────────────

// AgentsSendTool 发送消息给其他代理
type AgentsSendTool struct {
	BaseTool
	ipcDb    *IpcDb
	security SecurityPolicyChecker
}

// SecurityPolicyChecker 安全策略检查接口
type SecurityPolicyChecker interface {
	CanAct(operation string) bool
}

// NewAgentsSendTool 创建发送消息工具
func NewAgentsSendTool(ipcDb *IpcDb, security SecurityPolicyChecker) *AgentsSendTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"to_agent": {
				"type": "string",
				"description": "目标代理 ID 或 '*' 广播"
			},
			"payload": {
				"type": "string",
				"description": "消息内容 (推荐 JSON 字符串)"
			}
		},
		"required": ["to_agent", "payload"]
	}`)
	return &AgentsSendTool{
		BaseTool: *NewBaseTool("agents_send", "通过 ID 向另一个代理发送消息，或使用 to_agent=\"*\" 广播给所有代理。", schema),
		ipcDb:    ipcDb,
		security: security,
	}
}

// Execute 执行工具
func (t *AgentsSendTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if t.ipcDb == nil {
		return &ToolResult{
			Success: false,
			Error:   "IPC 数据库未初始化",
		}, nil
	}

	// 安全检查
	if t.security != nil && !t.security.CanAct("agents_send") {
		return &ToolResult{
			Success: false,
			Error:   "安全策略禁止此操作",
		}, nil
	}

	toAgent, _ := args["to_agent"].(string)
	payload, _ := args["payload"].(string)

	if toAgent == "" {
		return &ToolResult{
			Success: false,
			Error:   "缺少 'to_agent' 参数",
		}, nil
	}
	if payload == "" {
		return &ToolResult{
			Success: false,
			Error:   "缺少 'payload' 参数",
		}, nil
	}

	t.ipcDb.Heartbeat()
	now := nowEpoch()

	t.ipcDb.mu.Lock()
	defer t.ipcDb.mu.Unlock()

	_, err := t.ipcDb.conn.Exec(
		"INSERT INTO messages (from_agent, to_agent, payload, created_at) VALUES (?, ?, ?, ?)",
		t.ipcDb.agentID, toAgent, payload, now,
	)
	if err != nil {
		return nil, fmt.Errorf("插入消息失败: %w", err)
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("消息已发送到 %s", toAgent),
	}, nil
}

// ── AgentsInboxTool ─────────────────────────────────────────────

// AgentsInboxTool 读取收件箱消息
type AgentsInboxTool struct {
	BaseTool
	ipcDb *IpcDb
}

// NewAgentsInboxTool 创建收件箱工具
func NewAgentsInboxTool(ipcDb *IpcDb) *AgentsInboxTool {
	schema := json.RawMessage(`{"type": "object", "properties": {}, "required": []}`)
	return &AgentsInboxTool{
		BaseTool: *NewBaseTool("agents_inbox", "读取此代理收件箱中的未读消息（包括广播到 '*' 的消息）。直接消息在检索后标记为已读；广播消息保持未读状态。", schema),
		ipcDb:    ipcDb,
	}
}

// Execute 执行工具
func (t *AgentsInboxTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if t.ipcDb == nil {
		return &ToolResult{
			Success: false,
			Error:   "IPC 数据库未初始化",
		}, nil
	}

	t.ipcDb.Heartbeat()
	agentID := t.ipcDb.AgentID()

	t.ipcDb.mu.Lock()
	defer t.ipcDb.mu.Unlock()

	// 获取发给此代理或广播的未读消息
	rows, err := t.ipcDb.conn.Query(
		"SELECT id, from_agent, payload, created_at FROM messages WHERE (to_agent = ? OR to_agent = '*') AND read = 0 ORDER BY created_at ASC",
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询消息失败: %w", err)
	}
	defer rows.Close()

	var messages []map[string]interface{}
	for rows.Next() {
		var id int64
		var fromAgent, payload string
		var createdAt int64
		if err := rows.Scan(&id, &fromAgent, &payload, &createdAt); err != nil {
			continue
		}
		messages = append(messages, map[string]interface{}{
			"id":         id,
			"from_agent": fromAgent,
			"payload":    payload,
			"created_at": createdAt,
		})
	}

	// 标记直接消息为已读（广播消息保持未读）
	if _, err := t.ipcDb.conn.Exec(
		"UPDATE messages SET read = 1 WHERE to_agent = ? AND read = 0",
		agentID,
	); err != nil {
		// 记录错误但不影响返回结果
	}

	output, _ := json.MarshalIndent(messages, "", "  ")
	return &ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ── StateGetTool ────────────────────────────────────────────────

// StateGetTool 获取共享状态
type StateGetTool struct {
	BaseTool
	ipcDb *IpcDb
}

// NewStateGetTool 创建状态获取工具
func NewStateGetTool(ipcDb *IpcDb) *StateGetTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"key": {
				"type": "string",
				"description": "要查找的键名"
			}
		},
		"required": ["key"]
	}`)
	return &StateGetTool{
		BaseTool: *NewBaseTool("state_get", "从共享的代理间键值存储中获取值。", schema),
		ipcDb:    ipcDb,
	}
}

// Execute 执行工具
func (t *StateGetTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if t.ipcDb == nil {
		return &ToolResult{
			Success: false,
			Error:   "IPC 数据库未初始化",
		}, nil
	}

	t.ipcDb.Heartbeat()

	key, _ := args["key"].(string)

	if key == "" {
		return &ToolResult{
			Success: false,
			Error:   "缺少 'key' 参数",
		}, nil
	}

	t.ipcDb.mu.Lock()
	defer t.ipcDb.mu.Unlock()

	var value, owner string
	var updatedAt int64
	err := t.ipcDb.conn.QueryRow(
		"SELECT value, owner, updated_at FROM shared_state WHERE key = ?",
		key,
	).Scan(&value, &owner, &updatedAt)

	if err == sql.ErrNoRows {
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("键 '%s' 未找到", key),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询状态失败: %w", err)
	}

	result := map[string]interface{}{
		"key":        key,
		"value":      value,
		"owner":      owner,
		"updated_at": updatedAt,
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	return &ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ── StateSetTool ────────────────────────────────────────────────

// StateSetTool 设置共享状态
type StateSetTool struct {
	BaseTool
	ipcDb    *IpcDb
	security SecurityPolicyChecker
}

// NewStateSetTool 创建状态设置工具
func NewStateSetTool(ipcDb *IpcDb, security SecurityPolicyChecker) *StateSetTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"key": {
				"type": "string",
				"description": "要设置的键名"
			},
			"value": {
				"type": "string",
				"description": "要存储的值"
			}
		},
		"required": ["key", "value"]
	}`)
	return &StateSetTool{
		BaseTool: *NewBaseTool("state_set", "在共享的代理间状态存储中设置键值对。覆盖键的任何现有值。", schema),
		ipcDb:    ipcDb,
		security: security,
	}
}

// Execute 执行工具
func (t *StateSetTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if t.ipcDb == nil {
		return &ToolResult{
			Success: false,
			Error:   "IPC 数据库未初始化",
		}, nil
	}

	// 安全检查
	if t.security != nil && !t.security.CanAct("state_set") {
		return &ToolResult{
			Success: false,
			Error:   "安全策略禁止此操作",
		}, nil
	}

	t.ipcDb.Heartbeat()

	key, _ := args["key"].(string)
	value, _ := args["value"].(string)

	if key == "" {
		return &ToolResult{
			Success: false,
			Error:   "缺少 'key' 参数",
		}, nil
	}
	if value == "" {
		return &ToolResult{
			Success: false,
			Error:   "缺少 'value' 参数",
		}, nil
	}

	now := nowEpoch()

	t.ipcDb.mu.Lock()
	defer t.ipcDb.mu.Unlock()

	// SQLite INSERT OR REPLACE 等同于 UPSERT
	_, err := t.ipcDb.conn.Exec(
		"INSERT OR REPLACE INTO shared_state (key, value, owner, updated_at) VALUES (?, ?, ?, ?)",
		key, value, t.ipcDb.agentID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("设置状态失败: %w", err)
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("状态 '%s' 已更新", key),
	}, nil
}

// ── 工厂函数 ────────────────────────────────────────────────────

// CreateIpcTools 创建所有 IPC 工具
// 返回 5 个工具：agents_list, agents_send, agents_inbox, state_get, state_set
func CreateIpcTools(ipcDb *IpcDb, security SecurityPolicyChecker) []Tool {
	return []Tool{
		NewAgentsListTool(ipcDb),
		NewAgentsSendTool(ipcDb, security),
		NewAgentsInboxTool(ipcDb),
		NewStateGetTool(ipcDb),
		NewStateSetTool(ipcDb, security),
	}
}

// DefaultSecurityPolicy 默认安全策略（允许所有操作）
type DefaultSecurityPolicy struct{}

// CanAct 默认允许所有操作
func (p *DefaultSecurityPolicy) CanAct(operation string) bool {
	return true
}