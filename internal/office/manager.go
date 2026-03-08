// Package office provides the Office abstraction layer for zen-brain.
// The Office is responsible for interfacing with external planning systems
// (Jira, Linear, Slack, etc.) and converting their native items to canonical
// WorkItem types.
package office

import (
	"context"
	"fmt"
	"sync"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	pkgoffice "github.com/kube-zen/zen-brain1/pkg/office"
)

// Manager manages multiple Office connectors and routes requests appropriately.
type Manager struct {
	mu         sync.RWMutex
	connectors map[string]pkgoffice.ZenOffice // key: connector name
	byCluster  map[string]string              // clusterID -> connector name
}

// NewManager creates a new Office manager.
func NewManager() *Manager {
	return &Manager{
		connectors: make(map[string]pkgoffice.ZenOffice),
		byCluster:  make(map[string]string),
	}
}

// Register adds a connector to the manager.
func (m *Manager) Register(name string, connector pkgoffice.ZenOffice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.connectors[name]; exists {
		return fmt.Errorf("connector %q already registered", name)
	}
	
	m.connectors[name] = connector
	return nil
}

// RegisterForCluster associates a connector with a specific cluster.
func (m *Manager) RegisterForCluster(clusterID, connectorName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.connectors[connectorName]; !exists {
		return fmt.Errorf("connector %q not registered", connectorName)
	}
	
	m.byCluster[clusterID] = connectorName
	return nil
}

// GetConnector returns a connector by name.
func (m *Manager) GetConnector(name string) (pkgoffice.ZenOffice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	connector, exists := m.connectors[name]
	if !exists {
		return nil, fmt.Errorf("connector %q not found", name)
	}
	
	return connector, nil
}

// GetConnectorForCluster returns the connector associated with a cluster.
func (m *Manager) GetConnectorForCluster(clusterID string) (pkgoffice.ZenOffice, error) {
	m.mu.RLock()
	connectorName, exists := m.byCluster[clusterID]
	if !exists {
		m.mu.RUnlock()
		// Fall back to default connector if cluster not explicitly mapped
		return m.GetConnector("default")
	}
	
	connector, exists := m.connectors[connectorName]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("connector %q not found for cluster %q", connectorName, clusterID)
	}
	
	return connector, nil
}

// Fetch retrieves a work item by ID, trying all connectors.
func (m *Manager) Fetch(ctx context.Context, clusterID, workItemID string) (*contracts.WorkItem, error) {
	connector, err := m.GetConnectorForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	
	return connector.Fetch(ctx, clusterID, workItemID)
}

// FetchBySourceKey retrieves a work item by its source system key.
func (m *Manager) FetchBySourceKey(ctx context.Context, clusterID, sourceKey string) (*contracts.WorkItem, error) {
	connector, err := m.GetConnectorForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	
	return connector.FetchBySourceKey(ctx, clusterID, sourceKey)
}

// UpdateStatus updates the status of a work item.
func (m *Manager) UpdateStatus(ctx context.Context, clusterID, workItemID string, status contracts.WorkStatus) error {
	connector, err := m.GetConnectorForCluster(clusterID)
	if err != nil {
		return err
	}
	
	return connector.UpdateStatus(ctx, clusterID, workItemID, status)
}

// AddComment adds a comment to a work item.
func (m *Manager) AddComment(ctx context.Context, clusterID, workItemID string, comment *contracts.Comment) error {
	connector, err := m.GetConnectorForCluster(clusterID)
	if err != nil {
		return err
	}
	
	return connector.AddComment(ctx, clusterID, workItemID, comment)
}

// AddAttachment attaches evidence to a work item.
func (m *Manager) AddAttachment(ctx context.Context, clusterID, workItemID string, attachment *contracts.Attachment, content []byte) error {
	connector, err := m.GetConnectorForCluster(clusterID)
	if err != nil {
		return err
	}
	
	return connector.AddAttachment(ctx, clusterID, workItemID, attachment, content)
}

// Search searches for work items matching criteria across all connectors.
func (m *Manager) Search(ctx context.Context, clusterID, query string) ([]contracts.WorkItem, error) {
	connector, err := m.GetConnectorForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	
	return connector.Search(ctx, clusterID, query)
}

// Watch returns a combined channel for receiving work item events from all connectors.
func (m *Manager) Watch(ctx context.Context, clusterID string) (<-chan pkgoffice.WorkItemEvent, error) {
	connector, err := m.GetConnectorForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	
	return connector.Watch(ctx, clusterID)
}

// ListConnectors returns the names of all registered connectors.
func (m *Manager) ListConnectors() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	names := make([]string, 0, len(m.connectors))
	for name := range m.connectors {
		names = append(names, name)
	}
	
	return names
}

// RemoveConnector removes a connector from the manager.
func (m *Manager) RemoveConnector(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.connectors[name]; !exists {
		return fmt.Errorf("connector %q not found", name)
	}
	
	delete(m.connectors, name)
	
	// Remove from cluster mappings
	for clusterID, connectorName := range m.byCluster {
		if connectorName == name {
			delete(m.byCluster, clusterID)
		}
	}
	
	return nil
}