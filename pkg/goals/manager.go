package goals

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type Goal struct {
	ID          string
	Title       string
	Description string
	Status      Status
	Priority    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DueDate     *time.Time
	CompletedAt *time.Time
	Metadata    map[string]interface{}
}

type Manager struct {
	mu    sync.RWMutex
	goals map[string]*Goal
}

func NewManager() *Manager {
	return &Manager{
		goals: make(map[string]*Goal),
	}
}

func (m *Manager) Create(ctx context.Context, title, description string, priority int) (*Goal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	goal := &Goal{
		ID:          generateID(),
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	m.goals[goal.ID] = goal
	return goal, nil
}

func (m *Manager) Get(ctx context.Context, id string) (*Goal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	goal, ok := m.goals[id]
	if !ok {
		return nil, fmt.Errorf("goal not found: %s", id)
	}
	return goal, nil
}

func (m *Manager) List(ctx context.Context, status Status) []*Goal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Goal
	for _, goal := range m.goals {
		if status == "" || goal.Status == status {
			results = append(results, goal)
		}
	}
	return results
}

func (m *Manager) Update(ctx context.Context, id string, title, description string, priority int) (*Goal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	goal, ok := m.goals[id]
	if !ok {
		return nil, fmt.Errorf("goal not found: %s", id)
	}

	if title != "" {
		goal.Title = title
	}
	if description != "" {
		goal.Description = description
	}
	if priority > 0 {
		goal.Priority = priority
	}
	goal.UpdatedAt = time.Now()

	return goal, nil
}

func (m *Manager) Complete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	goal, ok := m.goals[id]
	if !ok {
		return fmt.Errorf("goal not found: %s", id)
	}

	now := time.Now()
	goal.Status = StatusCompleted
	goal.CompletedAt = &now
	goal.UpdatedAt = now

	return nil
}

func (m *Manager) Cancel(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	goal, ok := m.goals[id]
	if !ok {
		return fmt.Errorf("goal not found: %s", id)
	}

	goal.Status = StatusCancelled
	goal.UpdatedAt = time.Now()

	return nil
}

func (m *Manager) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.goals[id]; !ok {
		return fmt.Errorf("goal not found: %s", id)
	}

	delete(m.goals, id)
	return nil
}

func (m *Manager) Activate(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	goal, ok := m.goals[id]
	if !ok {
		return fmt.Errorf("goal not found: %s", id)
	}

	goal.Status = StatusActive
	goal.UpdatedAt = time.Now()

	return nil
}

func generateID() string {
	return fmt.Sprintf("goal-%d", time.Now().UnixNano())
}
