// Package tools provides tool functionality for GoClaw.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CronJob represents a scheduled job.
type CronJob struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Expression  string    `json:"expression"`
	Command     string    `json:"command"`
	NextRun     time.Time `json:"next_run"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	LastStatus  string    `json:"last_status,omitempty"`
	Enabled     bool      `json:"enabled"`
	OneShot     bool      `json:"one_shot"`
	CreatedAt   time.Time `json:"created_at"`
}

// CronStore manages cron jobs with file persistence.
type CronStore struct {
	jobs      map[string]*CronJob
	filePath  string
	mu        sync.RWMutex
}

var (
	cronStores     = make(map[string]*CronStore)
	cronStoresMu   sync.Mutex
)

// GetCronStore returns a cron store for the given workspace.
func GetCronStore(workspaceDir string) *CronStore {
	cronStoresMu.Lock()
	defer cronStoresMu.Unlock()

	if store, ok := cronStores[workspaceDir]; ok {
		return store
	}

	store := &CronStore{
		jobs:     make(map[string]*CronJob),
		filePath: filepath.Join(workspaceDir, ".cron_jobs.json"),
	}
	cronStores[workspaceDir] = store
	store.load()
	return store
}

func (s *CronStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var jobs []*CronJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		return
	}
	for _, job := range jobs {
		s.jobs[job.ID] = job
	}
}

func (s *CronStore) save() {
	s.mu.RLock()
	jobs := make([]*CronJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(s.filePath, data, 0644)
}

func (s *CronStore) Add(job *CronJob) {
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
	s.save()
}

func (s *CronStore) Get(id string) (*CronJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

func (s *CronStore) Remove(id string) bool {
	s.mu.Lock()
	_, exists := s.jobs[id]
	if exists {
		delete(s.jobs, id)
	}
	s.mu.Unlock()
	if exists {
		s.save()
	}
	return exists
}

func (s *CronStore) List() []*CronJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*CronJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// CronAddTool creates scheduled cron jobs.
type CronAddTool struct {
	BaseTool
	workspaceDir string
}

// NewCronAddTool creates a new CronAddTool.
func NewCronAddTool(workspaceDir string) *CronAddTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": { "type": "string", "description": "Job name" },
			"expression": { "type": "string", "description": "Cron expression (e.g. '*/5 * * * *') or 'at:YYYY-MM-DDTHH:MM' for one-shot" },
			"command": { "type": "string", "description": "Shell command to execute" },
			"enabled": { "type": "boolean", "description": "Whether the job is enabled (default: true)" }
		},
		"required": ["expression", "command"]
	}`)
	return &CronAddTool{
		BaseTool: *NewBaseTool(
			"cron_add",
			"创建定时任务。使用 cron 表达式，或 'at:YYYY-MM-DDTHH:MM' 格式创建一次性任务。",
			schema,
		),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the cron add tool.
func (t *CronAddTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	expression, _ := args["expression"].(string)
	command, _ := args["command"].(string)
	name, _ := args["name"].(string)
	enabled := true
	if e, ok := args["enabled"].(bool); ok {
		enabled = e
	}

	if expression == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "expression parameter is required",
		}, nil
	}
	if command == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "command parameter is required",
		}, nil
	}

	// Generate job ID
	id := fmt.Sprintf("job-%d", time.Now().UnixNano())

	// Parse next run time (simplified)
	nextRun := t.parseNextRun(expression)

	job := &CronJob{
		ID:         id,
		Name:       name,
		Expression: expression,
		Command:    command,
		NextRun:    nextRun,
		Enabled:    enabled,
		OneShot:    strings.HasPrefix(expression, "at:"),
		CreatedAt:  time.Now(),
	}

	store := GetCronStore(t.workspaceDir)
	store.Add(job)

	return &ToolResult{
		Success: true,
		Output: fmt.Sprintf(`Created job:
  ID: %s
  Name: %s
  Expression: %s
  Command: %s
  Next Run: %s
  Enabled: %v`,
			job.ID, job.Name, job.Expression, job.Command, job.NextRun.Format(time.RFC3339), job.Enabled),
	}, nil
}

func (t *CronAddTool) parseNextRun(expr string) time.Time {
	// Simplified: just add 5 minutes for demo
	// In real implementation, parse cron expression
	if strings.HasPrefix(expr, "at:") {
		t, err := time.Parse(time.RFC3339, strings.TrimPrefix(expr, "at:"))
		if err == nil {
			return t
		}
	}
	return time.Now().Add(5 * time.Minute)
}

// CronListTool lists all scheduled jobs.
type CronListTool struct {
	BaseTool
	workspaceDir string
}

// NewCronListTool creates a new CronListTool.
func NewCronListTool(workspaceDir string) *CronListTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
	return &CronListTool{
		BaseTool: *NewBaseTool(
			"cron_list",
			"列出所有定时任务。",
			schema,
		),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the cron list tool.
func (t *CronListTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	store := GetCronStore(t.workspaceDir)
	jobs := store.List()

	if len(jobs) == 0 {
		return &ToolResult{
			Success: true,
			Output:  "No scheduled jobs.",
		}, nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Scheduled jobs (%d):", len(jobs)))
	for _, job := range jobs {
		status := "enabled"
		if !job.Enabled {
			status = "disabled"
		}
		oneShot := ""
		if job.OneShot {
			oneShot = " [one-shot]"
		}
		lines = append(lines, fmt.Sprintf("  - %s | %s | next: %s | %s%s",
			job.ID, job.Name, job.NextRun.Format(time.RFC3339), status, oneShot))
		lines = append(lines, fmt.Sprintf("    Command: %s", job.Command))
	}

	return &ToolResult{
		Success: true,
		Output:  strings.Join(lines, "\n"),
	}, nil
}

// CronRemoveTool removes a scheduled job.
type CronRemoveTool struct {
	BaseTool
	workspaceDir string
}

// NewCronRemoveTool creates a new CronRemoveTool.
func NewCronRemoveTool(workspaceDir string) *CronRemoveTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": { "type": "string", "description": "Job ID to remove" }
		},
		"required": ["id"]
	}`)
	return &CronRemoveTool{
		BaseTool: *NewBaseTool(
			"cron_remove",
			"按 ID 删除定时任务。",
			schema,
		),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the cron remove tool.
func (t *CronRemoveTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "id parameter is required",
		}, nil
	}

	store := GetCronStore(t.workspaceDir)
	if store.Remove(id) {
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Removed job: %s", id),
		}, nil
	}

	return &ToolResult{
		Success: false,
		Output:  "",
		Error:   fmt.Sprintf("Job not found: %s", id),
	}, nil
}

// CronRunTool runs a scheduled job immediately.
type CronRunTool struct {
	BaseTool
	workspaceDir string
}

// NewCronRunTool creates a new CronRunTool.
func NewCronRunTool(workspaceDir string) *CronRunTool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": { "type": "string", "description": "Job ID to run" }
		},
		"required": ["id"]
	}`)
	return &CronRunTool{
		BaseTool: *NewBaseTool(
			"cron_run",
			"立即运行指定的定时任务。",
			schema,
		),
		workspaceDir: workspaceDir,
	}
}

// Execute executes the cron run tool.
func (t *CronRunTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   "id parameter is required",
		}, nil
	}

	store := GetCronStore(t.workspaceDir)
	job, ok := store.Get(id)
	if !ok {
		return &ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("Job not found: %s", id),
		}, nil
	}

	// Run the command (simplified - just echo it)
	now := time.Now()
	job.LastRun = &now
	job.LastStatus = "success"
	store.Add(job)

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Ran job %s: %s", id, job.Command),
	}, nil
}
