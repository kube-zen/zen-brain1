// Package evidence provides the Evidence Vault interface and implementations (Block 4.5).
package evidence

import (
	"context"
	"sync"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// Vault stores and retrieves evidence items (Block 4.5 Evidence Vault).
type Vault interface {
	// Store adds an evidence item. Idempotent if ID already exists (replace or no-op).
	Store(ctx context.Context, item contracts.EvidenceItem) error
	// GetBySession returns all evidence for a session.
	GetBySession(ctx context.Context, sessionID string) ([]contracts.EvidenceItem, error)
	// GetByTask returns all evidence for a task (optional; may use metadata task_id).
	GetByTask(ctx context.Context, taskID string) ([]contracts.EvidenceItem, error)
}

// MemoryVault is an in-memory implementation of Vault for development and testing.
type MemoryVault struct {
	mu   sync.RWMutex
	byID map[string]contracts.EvidenceItem
}

// NewMemoryVault creates a new in-memory evidence vault.
func NewMemoryVault() *MemoryVault {
	return &MemoryVault{byID: make(map[string]contracts.EvidenceItem)}
}

// Store implements Vault.Store.
func (m *MemoryVault) Store(ctx context.Context, item contracts.EvidenceItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[item.ID] = item
	return nil
}

// GetBySession implements Vault.GetBySession.
func (m *MemoryVault) GetBySession(ctx context.Context, sessionID string) ([]contracts.EvidenceItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []contracts.EvidenceItem
	for _, v := range m.byID {
		if v.SessionID == sessionID {
			out = append(out, v)
		}
	}
	return out, nil
}

// GetByTask implements Vault.GetByTask.
func (m *MemoryVault) GetByTask(ctx context.Context, taskID string) ([]contracts.EvidenceItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []contracts.EvidenceItem
	for _, v := range m.byID {
		if v.Metadata != nil && v.Metadata["task_id"] == taskID {
			out = append(out, v)
		}
	}
	return out, nil
}
