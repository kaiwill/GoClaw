package heartbeat

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Event struct {
	Type    string
	Payload interface{}
	Time    time.Time
}

type Handler func(ctx context.Context, event Event)

type Engine struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	interval time.Duration
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
}

type Config struct {
	Interval time.Duration
}

func NewEngine(cfg Config) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		handlers: make(map[string][]Handler),
		interval: cfg.Interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (e *Engine) Register(eventType string, handler Handler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[eventType] = append(e.handlers[eventType], handler)
}

func (e *Engine) Unregister(eventType string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.handlers, eventType)
}

func (e *Engine) Emit(eventType string, payload interface{}) {
	event := Event{
		Type:    eventType,
		Payload: payload,
		Time:    time.Now(),
	}

	e.mu.RLock()
	handlers := make([]Handler, len(e.handlers[eventType]))
	copy(handlers, e.handlers[eventType])
	e.mu.RUnlock()

	for _, handler := range handlers {
		handler(e.ctx, event)
	}
}

func (e *Engine) Start() error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("heartbeat engine already running")
	}
	e.running = true
	e.mu.Unlock()

	go e.run()
	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		e.cancel()
		e.running = false
	}
}

func (e *Engine) run() {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.Emit("tick", nil)
		}
	}
}

func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

type HeartbeatPayload struct {
	Timestamp time.Time
	Uptime    time.Duration
	Status    string
}

func DefaultConfig() Config {
	return Config{
		Interval: 30 * time.Second,
	}
}
