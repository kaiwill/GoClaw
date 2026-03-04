package approval

import (
	"fmt"
	"sync"
	"time"
)

type ApprovalRequest struct {
	ID          string
	Action      string
	Details     string
	RequestedBy string
	Status      ApprovalStatus
	CreatedAt   time.Time
	ResolvedAt  *time.Time
	Resolver    *string
}

type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
	ApprovalStatusExpired  ApprovalStatus = "expired"
)

type ApprovalPolicy struct {
	AutoApprovePatterns     []string
	RequireApprovalPatterns []string
	MaxAutoApproveAge       time.Duration
}

type ApprovalManager struct {
	mu         sync.RWMutex
	requests   map[string]*ApprovalRequest
	policy     ApprovalPolicy
	approvers  map[string]bool
	notifyChan chan *ApprovalRequest
}

func NewApprovalManager(policy ApprovalPolicy) *ApprovalManager {
	return &ApprovalManager{
		requests:   make(map[string]*ApprovalRequest),
		policy:     policy,
		approvers:  make(map[string]bool),
		notifyChan: make(chan *ApprovalRequest, 100),
	}
}

func (m *ApprovalManager) RequestApproval(action, details, requestedBy string) (*ApprovalRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldAutoApprove(action, details) {
		return &ApprovalRequest{
			ID:          generateApprovalID(),
			Action:      action,
			Details:     details,
			RequestedBy: requestedBy,
			Status:      ApprovalStatusApproved,
			CreatedAt:   time.Now(),
		}, nil
	}

	req := &ApprovalRequest{
		ID:          generateApprovalID(),
		Action:      action,
		Details:     details,
		RequestedBy: requestedBy,
		Status:      ApprovalStatusPending,
		CreatedAt:   time.Now(),
	}

	m.requests[req.ID] = req
	m.notifyChan <- req

	return req, nil
}

func (m *ApprovalManager) Approve(requestID, resolver string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.requests[requestID]
	if !exists {
		return fmt.Errorf("approval request not found: %s", requestID)
	}

	if req.Status != ApprovalStatusPending {
		return fmt.Errorf("approval request already resolved: %s", requestID)
	}

	now := time.Now()
	req.Status = ApprovalStatusApproved
	req.ResolvedAt = &now
	req.Resolver = &resolver

	return nil
}

func (m *ApprovalManager) Reject(requestID, resolver, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.requests[requestID]
	if !exists {
		return fmt.Errorf("approval request not found: %s", requestID)
	}

	if req.Status != ApprovalStatusPending {
		return fmt.Errorf("approval request already resolved: %s", requestID)
	}

	now := time.Now()
	req.Status = ApprovalStatusRejected
	req.ResolvedAt = &now
	req.Resolver = &resolver
	req.Details = req.Details + "\n\nRejection reason: " + reason

	return nil
}

func (m *ApprovalManager) GetRequest(id string) (*ApprovalRequest, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, exists := m.requests[id]
	return req, exists
}

func (m *ApprovalManager) ListPending() []*ApprovalRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pending []*ApprovalRequest
	for _, req := range m.requests {
		if req.Status == ApprovalStatusPending {
			pending = append(pending, req)
		}
	}

	return pending
}

func (m *ApprovalManager) AddApprover(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.approvers[userID] = true
}

func (m *ApprovalManager) RemoveApprover(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.approvers, userID)
}

func (m *ApprovalManager) IsApprover(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.approvers[userID]
}

func (m *ApprovalManager) NotificationChan() <-chan *ApprovalRequest {
	return m.notifyChan
}

func (m *ApprovalManager) shouldAutoApprove(action, details string) bool {
	for _, pattern := range m.policy.RequireApprovalPatterns {
		if matchesPattern(action+" "+details, pattern) {
			return false
		}
	}

	for _, pattern := range m.policy.AutoApprovePatterns {
		if matchesPattern(action+" "+details, pattern) {
			return true
		}
	}

	return false
}

func matchesPattern(text, pattern string) bool {
	return contains(text, pattern)
}

func contains(text, substr string) bool {
	return len(text) >= len(substr) &&
		(text == substr ||
			len(text) > len(substr) &&
				(text[:len(substr)] == substr ||
					text[len(text)-len(substr):] == substr ||
					containsHelper(text, substr)))
}

func containsHelper(text, substr string) bool {
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func generateApprovalID() string {
	return fmt.Sprintf("apr_%d", time.Now().UnixNano())
}

func (r *ApprovalRequest) IsPending() bool {
	return r.Status == ApprovalStatusPending
}

func (r *ApprovalRequest) IsApproved() bool {
	return r.Status == ApprovalStatusApproved
}

func (r *ApprovalRequest) IsRejected() bool {
	return r.Status == ApprovalStatusRejected
}
