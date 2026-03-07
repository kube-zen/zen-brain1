// Package office provides the base implementation of ZenOffice.
// This package contains the base struct that can be embedded by specific
// Office connectors (Jira, Linear, Slack, etc.).
package office

import (
	"context"
	"fmt"
	"time"

	pkgoffice "github.com/kube-zen/zen-brain1/pkg/office"
)

// BaseOffice provides a base implementation of ZenOffice.
// Embed this struct in your connector and override methods as needed.
type BaseOffice struct {
	// Name is the connector name (e.g., "jira", "linear")
	Name string

	// ClusterID is the cluster this connector operates in
	ClusterID string

	// Config contains connector-specific configuration
	Config map[string]interface{}
}

// NewBaseOffice creates a new BaseOffice.
func NewBaseOffice(name, clusterID string, config map[string]interface{}) *BaseOffice {
	return &BaseOffice{
		Name:      name,
		ClusterID: clusterID,
		Config:    config,
	}
}

// Fetch implements ZenOffice.Fetch.
func (b *BaseOffice) Fetch(ctx context.Context, clusterID, workItemID string) (*pkgoffice.WorkItem, error) {
	return nil, fmt.Errorf("not implemented")
}

// FetchBySourceKey implements ZenOffice.FetchBySourceKey.
func (b *BaseOffice) FetchBySourceKey(ctx context.Context, clusterID, sourceKey string) (*pkgoffice.WorkItem, error) {
	return nil, fmt.Errorf("not implemented")
}

// UpdateStatus implements ZenOffice.UpdateStatus.
func (b *BaseOffice) UpdateStatus(ctx context.Context, clusterID, workItemID string, status pkgoffice.WorkStatus) error {
	return fmt.Errorf("not implemented")
}

// AddComment implements ZenOffice.AddComment.
func (b *BaseOffice) AddComment(ctx context.Context, clusterID, workItemID string, comment *pkgoffice.Comment) error {
	return fmt.Errorf("not implemented")
}

// AddAttachment implements ZenOffice.AddAttachment.
func (b *BaseOffice) AddAttachment(ctx context.Context, clusterID, workItemID string, attachment *pkgoffice.Attachment, content []byte) error {
	return fmt.Errorf("not implemented")
}

// Search implements ZenOffice.Search.
func (b *BaseOffice) Search(ctx context.Context, clusterID string, query string) ([]pkgoffice.WorkItem, error) {
	return nil, fmt.Errorf("not implemented")
}

// Watch implements ZenOffice.Watch.
func (b *BaseOffice) Watch(ctx context.Context, clusterID string) (<-chan pkgoffice.WorkItemEvent, error) {
	return nil, fmt.Errorf("not implemented")
}

// Helper functions for common transformations

// MapPriority maps external priority strings to canonical Priority.
func (b *BaseOffice) MapPriority(externalPriority string) pkgoffice.Priority {
	switch externalPriority {
	case "highest", "critical", "1":
		return pkgoffice.PriorityCritical
	case "high", "2":
		return pkgoffice.PriorityHigh
	case "medium", "3":
		return pkgoffice.PriorityMedium
	case "low", "4":
		return pkgoffice.PriorityLow
	case "lowest", "5":
		return pkgoffice.PriorityBackground
	default:
		return pkgoffice.PriorityMedium
	}
}

// MapWorkType maps external issue types to canonical WorkType.
func (b *BaseOffice) MapWorkType(externalType string) pkgoffice.WorkType {
	switch externalType {
	case "bug", "defect":
		return pkgoffice.WorkTypeDebug
	case "task", "chore":
		return pkgoffice.WorkTypeImplementation
	case "story", "feature":
		return pkgoffice.WorkTypeDesign
	case "epic", "initiative":
		return pkgoffice.WorkTypeResearch
	case "spike", "investigation":
		return pkgoffice.WorkTypeAnalysis
	case "documentation":
		return pkgoffice.WorkTypeDocumentation
	case "refactor":
		return pkgoffice.WorkTypeRefactor
	case "security":
		return pkgoffice.WorkTypeSecurity
	case "test":
		return pkgoffice.WorkTypeTesting
	case "operation", "ops":
		return pkgoffice.WorkTypeOperations
	default:
		return pkgoffice.WorkTypeImplementation
	}
}

// MapWorkDomain maps external components to canonical WorkDomain.
func (b *BaseOffice) MapWorkDomain(externalComponent string) pkgoffice.WorkDomain {
	switch externalComponent {
	case "office", "ui", "frontend":
		return pkgoffice.DomainOffice
	case "factory", "worker", "agent":
		return pkgoffice.DomainFactory
	case "sdk", "library", "lib":
		return pkgoffice.DomainSDK
	case "policy", "gate", "guardian":
		return pkgoffice.DomainPolicy
	case "memory", "context", "kb":
		return pkgoffice.DomainMemory
	case "observability", "monitoring", "logs":
		return pkgoffice.DomainObservability
	case "infrastructure", "cluster", "k8s":
		return pkgoffice.DomainInfrastructure
	case "integration", "api", "gateway":
		return pkgoffice.DomainIntegration
	default:
		return pkgoffice.DomainCore
	}
}

// CreateAIAttribution creates an AIAttribution struct for AI-generated content.
func (b *BaseOffice) CreateAIAttribution(agentRole, modelUsed, sessionID, taskID string) *pkgoffice.AIAttribution {
	return &pkgoffice.AIAttribution{
		AgentRole: agentRole,
		ModelUsed: modelUsed,
		SessionID: sessionID,
		TaskID:    taskID,
		Timestamp: time.Now().UTC(),
	}
}

// FormatAIAttributionHeader formats AI attribution as a Jira comment header.
func (b *BaseOffice) FormatAIAttributionHeader(attr *pkgoffice.AIAttribution) string {
	return fmt.Sprintf(
		"[zen-brain | agent: %s | model: %s | session: %s | task: %s | %s]",
		attr.AgentRole,
		attr.ModelUsed,
		attr.SessionID,
		attr.TaskID,
		attr.Timestamp.Format(time.RFC3339),
	)
}