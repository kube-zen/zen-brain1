// Package office provides the ZenOffice interface for work ingress.
// ZenOffice is the abstract interface that defines how zen-brain
// interacts with external planning systems (Jira, Linear, Slack, etc.)
//
// The Factory operates on canonical WorkItem types only.
// All Office-specific concepts are translated at the ZenOffice boundary.
// No Factory type, API, CRD, or event schema may import Jira-specific models.
package office

import (
	"context"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// ZenOffice is the interface for work ingress from external systems.
// Implementations: Jira connector, Linear connector, Slack connector, etc.
type ZenOffice interface {
	// Fetch retrieves a work item by ID.
	Fetch(ctx context.Context, clusterID, workItemID string) (*contracts.WorkItem, error)

	// FetchBySourceKey retrieves a work item by its source system key (e.g., "PROJ-123").
	FetchBySourceKey(ctx context.Context, clusterID, sourceKey string) (*contracts.WorkItem, error)

	// UpdateStatus updates the status of a work item.
	UpdateStatus(ctx context.Context, clusterID, workItemID string, status contracts.WorkStatus) error

	// AddComment adds a comment to a work item.
	// AI-generated comments must include attribution.
	AddComment(ctx context.Context, clusterID, workItemID string, comment *contracts.Comment) error

	// AddAttachment attaches evidence to a work item.
	AddAttachment(ctx context.Context, clusterID, workItemID string, attachment *contracts.Attachment, content []byte) error

	// Search searches for work items matching criteria.
	Search(ctx context.Context, clusterID string, query string) ([]contracts.WorkItem, error)

	// Watch returns a channel for receiving work item events.
	Watch(ctx context.Context, clusterID string) (<-chan WorkItemEvent, error)
}

// WorkEventType represents the type of work item event.
type WorkEventType string

const (
	WorkItemCreated   WorkEventType = "created"
	WorkItemUpdated   WorkEventType = "updated"
	WorkItemCommented WorkEventType = "commented"
	WorkItemDeleted   WorkEventType = "deleted"
)

// WorkItemEvent represents an event from the Office system.
type WorkItemEvent struct {
	Type      WorkEventType     `json:"type"`
	WorkItem  *contracts.WorkItem `json:"work_item"`
	Timestamp time.Time         `json:"timestamp"`
}
