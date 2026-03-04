package health

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

type Component struct {
	Name      string
	Status    Status
	Message   string
	LastCheck time.Time
}

type HealthChecker interface {
	Check(ctx context.Context) Component
}

type Manager struct {
	mu         sync.RWMutex
	components map[string]HealthChecker
	interval   time.Duration
}

func NewManager(interval time.Duration) *Manager {
	return &Manager{
		components: make(map[string]HealthChecker),
		interval:   interval,
	}
}

func (m *Manager) Register(name string, checker HealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components[name] = checker
}

func (m *Manager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.components, name)
}

func (m *Manager) CheckAll(ctx context.Context) map[string]Component {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]Component)
	for name, checker := range m.components {
		results[name] = checker.Check(ctx)
	}
	return results
}

func (m *Manager) GetStatus(ctx context.Context) (Status, map[string]Component) {
	results := m.CheckAll(ctx)

	overall := StatusHealthy
	for _, comp := range results {
		if comp.Status == StatusUnhealthy {
			overall = StatusUnhealthy
			break
		}
		if comp.Status == StatusDegraded {
			overall = StatusDegraded
		}
	}

	return overall, results
}

func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				m.CheckAll(ctx)
			}
		}
	}()
}

type SystemChecker struct{}

func NewSystemChecker() *SystemChecker {
	return &SystemChecker{}
}

func (s *SystemChecker) Check(ctx context.Context) Component {
	return Component{
		Name:      "system",
		Status:    StatusHealthy,
		Message:   "System operational",
		LastCheck: time.Now(),
	}
}

type MemoryChecker struct {
	threshold uint64
}

func NewMemoryChecker(threshold uint64) *MemoryChecker {
	return &MemoryChecker{threshold: threshold}
}

func (c *MemoryChecker) Check(ctx context.Context) Component {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	used := m.Alloc
	if used > c.threshold {
		return Component{
			Name:      "memory",
			Status:    StatusDegraded,
			Message:   fmt.Sprintf("Memory usage high: %d bytes", used),
			LastCheck: time.Now(),
		}
	}

	return Component{
		Name:      "memory",
		Status:    StatusHealthy,
		Message:   fmt.Sprintf("Memory usage: %d bytes", used),
		LastCheck: time.Now(),
	}
}
