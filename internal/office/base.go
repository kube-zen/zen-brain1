// Package office provides the base implementation of ZenOffice.
// This package contains the base struct that can be embedded by specific
// Office connectors (Jira, Linear, Slack, etc.).
package office

import (
	"context"
	"fmt"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
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
func (b *BaseOffice) Fetch(ctx context.Context, clusterID, workItemID string) (*contracts.WorkItem, error) {
	return nil, fmt.Errorf("not implemented")
}

// FetchBySourceKey implements ZenOffice.FetchBySourceKey.
func (b *BaseOffice) FetchBySourceKey(ctx context.Context, clusterID, sourceKey string) (*contracts.WorkItem, error) {
	return nil, fmt.Errorf("not implemented")
}

// UpdateStatus implements ZenOffice.UpdateStatus.
func (b *BaseOffice) UpdateStatus(ctx context.Context, clusterID, workItemID string, status contracts.WorkStatus) error {
	return fmt.Errorf("not implemented")
}

// AddComment implements ZenOffice.AddComment.
func (b *BaseOffice) AddComment(ctx context.Context, clusterID, workItemID string, comment *contracts.Comment) error {
	return fmt.Errorf("not implemented")
}

// AddAttachment implements ZenOffice.AddAttachment.
func (b *BaseOffice) AddAttachment(ctx context.Context, clusterID, workItemID string, attachment *contracts.Attachment, content []byte) error {
	return fmt.Errorf("not implemented")
}

// Search implements ZenOffice.Search.
func (b *BaseOffice) Search(ctx context.Context, clusterID string, query string) ([]contracts.WorkItem, error) {
	return nil, fmt.Errorf("not implemented")
}

// Watch implements ZenOffice.Watch.
func (b *BaseOffice) Watch(ctx context.Context, clusterID string) (<-chan pkgoffice.WorkItemEvent, error) {
	return nil, fmt.Errorf("not implemented")
}

// Helper functions for common transformations

// MapPriority maps external priority strings to canonical Priority.
func (b *BaseOffice) MapPriority(externalPriority string) contracts.Priority {
	switch externalPriority {
	case "highest", "critical", "1":
		return contracts.PriorityCritical
	case "high", "2":
		return contracts.PriorityHigh
	case "medium", "3":
		return contracts.PriorityMedium
	case "low", "4":
		return contracts.PriorityLow
	case "lowest", "5":
		return contracts.PriorityBackground
	default:
		return contracts.PriorityMedium
	}
}

// MapWorkType maps external issue types to canonical WorkType.
func (b *BaseOffice) MapWorkType(externalType string) contracts.WorkType {
	switch externalType {
	case "bug", "defect":
		return contracts.WorkTypeDebug
	case "task", "chore":
		return contracts.WorkTypeImplementation
	case "story", "feature":
		return contracts.WorkTypeDesign
	case "epic", "initiative":
		return contracts.WorkTypeResearch
	case "spike", "investigation":
		return contracts.WorkTypeAnalysis
	case "documentation":
		return contracts.WorkTypeDocumentation
	case "refactor":
		return contracts.WorkTypeRefactor
	case "security":
		return contracts.WorkTypeSecurity
	case "test":
		return contracts.WorkTypeTesting
	case "operation", "ops":
		return contracts.WorkTypeOperations
	default:
		return contracts.WorkTypeImplementation
	}
}

// MapWorkDomain maps external components to canonical WorkDomain.
func (b *BaseOffice) MapWorkDomain(externalComponent string) contracts.WorkDomain {
	switch externalComponent {
	case "office", "ui", "frontend":
		return contracts.DomainOffice
	case "factory", "worker", "agent":
		return contracts.DomainFactory
	case "sdk", "library", "lib":
		return contracts.DomainSDK
	case "policy", "gate", "guardian":
		return contracts.DomainPolicy
	case "memory", "context", "kb":
		return contracts.DomainMemory
	case "observability", "monitoring", "logs":
		return contracts.DomainObservability
	case "infrastructure", "cluster", "k8s":
		return contracts.DomainInfrastructure
	case "integration", "api", "gateway":
		return contracts.DomainIntegration
	default:
		return contracts.DomainCore
	}
}

// CreateAIAttribution creates an AIAttribution struct for AI-generated content.
func (b *BaseOffice) CreateAIAttribution(agentRole, modelUsed, sessionID, taskID string) *contracts.AIAttribution {
	return &contracts.AIAttribution{
		AgentRole: agentRole,
		ModelUsed: modelUsed,
		SessionID: sessionID,
		TaskID:    taskID,
		Timestamp: time.Now().UTC(),
	}
}

// FormatAIAttributionHeader formats AI attribution as a Jira comment header.
func (b *BaseOffice) FormatAIAttributionHeader(attr *contracts.AIAttribution) string {
	return fmt.Sprintf(
		"[zen-brain | agent: %s | model: %s | session: %s | task: %s | %s]",
		attr.AgentRole,
		attr.ModelUsed,
		attr.SessionID,
		attr.TaskID,
		attr.Timestamp.Format(time.RFC3339),
	)
}