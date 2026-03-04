package hooks

import (
	"context"
	"fmt"
	"sync"
)

type EventType string

const (
	EventPreToolExec   EventType = "pre_tool_exec"
	EventPostToolExec  EventType = "post_tool_exec"
	EventPreAgentLoop  EventType = "pre_agent_loop"
	EventPostAgentLoop EventType = "post_agent_loop"
	EventOnError       EventType = "on_error"
	EventOnMessage     EventType = "on_message"
)

type Context map[string]interface{}

type Handler func(ctx context.Context, eventType EventType, hookCtx Context) error

type Hook struct {
	Name     string
	Event    EventType
	Handler  Handler
	Priority int
}

type Manager struct {
	mu    sync.RWMutex
	hooks map[EventType][]Hook
}

func NewManager() *Manager {
	return &Manager{
		hooks: make(map[EventType][]Hook),
	}
}

func (m *Manager) Register(hook Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hooks[hook.Event] = append(m.hooks[hook.Event], hook)
}

func (m *Manager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for eventType, hooks := range m.hooks {
		var remaining []Hook
		for _, hook := range hooks {
			if hook.Name != name {
				remaining = append(remaining, hook)
			}
		}
		m.hooks[eventType] = remaining
	}
}

func (m *Manager) Emit(ctx context.Context, eventType EventType, hookCtx Context) error {
	m.mu.RLock()
	hooks := make([]Hook, len(m.hooks[eventType]))
	copy(hooks, m.hooks[eventType])
	m.mu.RUnlock()

	for _, hook := range hooks {
		if err := hook.Handler(ctx, eventType, hookCtx); err != nil {
			return fmt.Errorf("hook %s failed: %w", hook.Name, err)
		}
	}
	return nil
}

type PreToolContext struct {
	ToolName string
	Args     map[string]interface{}
}

type PostToolContext struct {
	ToolName string
	Args     map[string]interface{}
	Result   interface{}
	Error    error
}

type AgentLoopContext struct {
	Input    string
	Output   string
	Provider string
}

func PreToolHook(name string, handler func(ctx context.Context, tc PreToolContext) error) Hook {
	return Hook{
		Name:  name,
		Event: EventPreToolExec,
		Handler: func(ctx context.Context, eventType EventType, hookCtx Context) error {
			toolName, _ := hookCtx["tool_name"].(string)
			args, _ := hookCtx["args"].(map[string]interface{})
			return handler(ctx, PreToolContext{
				ToolName: toolName,
				Args:     args,
			})
		},
	}
}

func PostToolHook(name string, handler func(ctx context.Context, tc PostToolContext) error) Hook {
	return Hook{
		Name:  name,
		Event: EventPostToolExec,
		Handler: func(ctx context.Context, eventType EventType, hookCtx Context) error {
			toolName, _ := hookCtx["tool_name"].(string)
			args, _ := hookCtx["args"].(map[string]interface{})
			result, _ := hookCtx["result"]
			err, _ := hookCtx["error"].(error)
			return handler(ctx, PostToolContext{
				ToolName: toolName,
				Args:     args,
				Result:   result,
				Error:    err,
			})
		},
	}
}

func LoggingHook() Hook {
	return Hook{
		Name:  "logging",
		Event: EventOnMessage,
		Handler: func(ctx context.Context, eventType EventType, hookCtx Context) error {
			fmt.Printf("[hook:%s] %v\n", eventType, hookCtx)
			return nil
		},
		Priority: 100,
	}
}
