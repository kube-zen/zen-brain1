// Package ingestion provides Jira -> BrainTask ingestion service (ZB-023).
// This service converts Jira issues into BrainTask CRs for Foreman execution.
package ingestion

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/office/jira"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// JiraToBrainTaskConfig configures the ingestion service.
type JiraToBrainTaskConfig struct {
	// JiraLabel is the label that marks issues for zen-brain1 self-work
	JiraLabel string
	// DefaultNamespace is the namespace to create BrainTask CRs in
	DefaultNamespace string
	// DefaultQueueName is the default queue to assign (optional)
	DefaultQueueName string
	// WorkTypeMapping maps Jira issue types to WorkType
	WorkTypeMapping map[string]contracts.WorkType
	// WorkDomainMapping maps Jira components to WorkDomain
	WorkDomainMapping map[string]contracts.WorkDomain
	// PriorityMapping maps Jira priorities to Priority
	PriorityMapping map[string]contracts.Priority
}

// DefaultJiraToBrainTaskConfig returns sensible defaults.
func DefaultJiraToBrainTaskConfig() *JiraToBrainTaskConfig {
	return &JiraToBrainTaskConfig{
		JiraLabel:        "zen-brain-self-work",
		DefaultNamespace: "zen-brain",
		DefaultQueueName: "dogfood",
		WorkTypeMapping: map[string]contracts.WorkType{
			"Task":          contracts.WorkTypeImplementation,
			"Story":         contracts.WorkTypeImplementation,
			"Bug":           contracts.WorkTypeDebug,
			"Improvement":   contracts.WorkTypeRefactor,
			"Sub-task":      contracts.WorkTypeImplementation,
			"Documentation": contracts.WorkTypeDocumentation,
			"Test":          contracts.WorkTypeTesting,
		},
		WorkDomainMapping: map[string]contracts.WorkDomain{
			"core":           contracts.DomainCore,
			"api":            contracts.DomainIntegration,
			"factory":        contracts.DomainFactory,
			"office":         contracts.DomainOffice,
			"foreman":        contracts.DomainFactory,
			"policy":         contracts.DomainPolicy,
			"observability":  contracts.DomainObservability,
			"docs":           contracts.DomainOffice,
			"infrastructure": contracts.DomainInfrastructure,
		},
		PriorityMapping: map[string]contracts.Priority{
			"Highest": contracts.PriorityCritical,
			"High":    contracts.PriorityHigh,
			"Medium":  contracts.PriorityMedium,
			"Low":     contracts.PriorityLow,
			"Lowest":  contracts.PriorityBackground,
		},
	}
}

// JiraToBrainTaskService converts Jira issues to BrainTask CRs.
type JiraToBrainTaskService struct {
	k8sClient client.Client
	jiraConn  *jira.JiraOffice
	config    *JiraToBrainTaskConfig
}

// NewJiraToBrainTaskService creates a new ingestion service.
func NewJiraToBrainTaskService(
	k8sClient client.Client,
	jiraConn *jira.JiraOffice,
	config *JiraToBrainTaskConfig,
) *JiraToBrainTaskService {
	if config == nil {
		config = DefaultJiraToBrainTaskConfig()
	}
	return &JiraToBrainTaskService{
		k8sClient: k8sClient,
		jiraConn:  jiraConn,
		config:    config,
	}
}

// IngestIssues fetches Jira issues and creates BrainTask CRs (idempotent).
func (s *JiraToBrainTaskService) IngestIssues(ctx context.Context) (int, error) {
	log.Printf("[IngestIssues] Fetching Jira issues with label: %s", s.config.JiraLabel)

	// Fetch issues from Jira using Search
	query := fmt.Sprintf("labels = %s AND status != Done", s.config.JiraLabel)
	issues, err := s.jiraConn.Search(ctx, s.jiraConn.ClusterID, query)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch Jira issues: %w", err)
	}

	log.Printf("[IngestIssues] Found %d issues with label %s", len(issues), s.config.JiraLabel)

	created := 0
	for i := range issues {
		issue := &issues[i]
		// Convert to BrainTask
		task := s.issueToBrainTask(issue)

		// Check if BrainTask already exists (idempotency by SourceKey)
		existing := &v1alpha1.BrainTask{}
		err := s.k8sClient.Get(ctx, types.NamespacedName{
			Namespace: task.Namespace,
			Name:      task.Name,
		}, existing)

		if err == nil {
			// Already exists, skip
			log.Printf("[IngestIssues] BrainTask %s already exists (SourceKey: %s)", task.Name, task.Spec.SourceKey)
			continue
		}

		if !errors.IsNotFound(err) {
			// Unexpected error
			return created, fmt.Errorf("failed to check existing BrainTask: %w", err)
		}

		// Create new BrainTask
		if err := s.k8sClient.Create(ctx, task); err != nil {
			log.Printf("[IngestIssues] Failed to create BrainTask %s: %v", task.Name, err)
			continue
		}

		log.Printf("[IngestIssues] Created BrainTask %s (SourceKey: %s, WorkType: %s, WorkDomain: %s)",
			task.Name, task.Spec.SourceKey, task.Spec.WorkType, task.Spec.WorkDomain)
		created++
	}

	return created, nil
}

// issueToBrainTask converts a Jira issue to a BrainTask CR.
func (s *JiraToBrainTaskService) issueToBrainTask(issue *contracts.WorkItem) *v1alpha1.BrainTask {
	// Generate BrainTask name from Jira key
	name := fmt.Sprintf("jira-%s", strings.ToLower(issue.ID))

	// Use WorkType directly from issue (already a WorkType)
	workType := issue.WorkType
	if workType == "" {
		workType = contracts.WorkTypeImplementation
	}

	// Use WorkDomain directly from issue (already a WorkDomain)
	workDomain := issue.WorkDomain
	if workDomain == "" {
		workDomain = contracts.DomainOffice // Default to office for self-improvement
	}

	// Use Priority directly from issue (already a Priority)
	priority := issue.Priority
	if priority == "" {
		priority = contracts.PriorityMedium
	}

	// Build objective from title + body
	objective := issue.Title
	if issue.Body != "" {
		objective = fmt.Sprintf("%s\n\n%s", issue.Title, issue.Body)
	}

	// Default acceptance criteria
	acceptanceCriteria := []string{
		"Task completes without errors",
		"Result is captured in proof-of-work",
		"Jira issue is updated with result",
	}

	// Determine timeout: 2700s for normal lane, short timeout only for controlled failure test
	timeoutSeconds := int64(2700) // ZB-024: 45 minutes for qwen3.5:0.8b normal lane
	if strings.Contains(strings.ToLower(issue.Title), "timeout") ||
		strings.Contains(strings.ToLower(issue.Title), "failure test") {
		timeoutSeconds = 60 // Short timeout for controlled failure testing
	}

	// Create BrainTask
	task := &v1alpha1.BrainTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.config.DefaultNamespace,
			Labels: map[string]string{
				"zen.kube-zen.com/source":      "jira",
				"zen.kube-zen.com/jira-key":    issue.ID,
				"zen.kube-zen.com/work-type":   string(workType),
				"zen.kube-zen.com/work-domain": string(workDomain),
			},
		},
		Spec: v1alpha1.BrainTaskSpec{
			ID:                 name,
			WorkItemID:         issue.ID,
			SessionID:          fmt.Sprintf("jira-session-%s", issue.ID),
			SourceKey:          issue.ID, // Jira key (e.g., PROJ-123)
			Title:              issue.Title,
			Description:        issue.Body,
			WorkType:           workType,
			WorkDomain:         workDomain,
			Priority:           priority,
			Objective:          objective,
			AcceptanceCriteria: acceptanceCriteria,
			Constraints: []string{
				"Use only safe bounded operations",
				"Prefer real repo changes over synthetic defaults",
				"Local CPU must use qwen3.5:0.8b via llama.cpp L1 only",
			},
			EvidenceRequirement: contracts.EvidenceSummary,
			SREDTags: []contracts.SREDTag{
				contracts.SREDExperimentalGeneral,
			},
			TimeoutSeconds:   timeoutSeconds, // ZB-024: 2700s for normal lane, short for controlled failure
			MaxRetries:       1,
			EstimatedCostUSD: 0.05, // Rough estimate
			QueueName:        s.config.DefaultQueueName,
		},
		Status: v1alpha1.BrainTaskStatus{
			Phase: v1alpha1.BrainTaskPhasePending,
		},
	}

	return task
}

// mapWorkType maps Jira issue type to WorkType.
func (s *JiraToBrainTaskService) mapWorkType(issueType string) contracts.WorkType {
	if wt, ok := s.config.WorkTypeMapping[issueType]; ok {
		return wt
	}
	// Default to implementation
	return contracts.WorkTypeImplementation
}

// mapWorkDomain maps Jira component to WorkDomain.
func (s *JiraToBrainTaskService) mapWorkDomain(component string) contracts.WorkDomain {
	if component == "" {
		// Default to office for self-improvement
		return contracts.DomainOffice
	}
	if wd, ok := s.config.WorkDomainMapping[component]; ok {
		return wd
	}
	// Default to office
	return contracts.DomainOffice
}

// mapPriority maps Jira priority to Priority.
func (s *JiraToBrainTaskService) mapPriority(priority string) contracts.Priority {
	if p, ok := s.config.PriorityMapping[priority]; ok {
		return p
	}
	// Default to medium
	return contracts.PriorityMedium
}

// StartPeriodicIngestion starts a periodic ingestion loop.
func (s *JiraToBrainTaskService) StartPeriodicIngestion(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[PeriodicIngestion] Starting periodic ingestion (interval: %v)", interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[PeriodicIngestion] Context cancelled, stopping")
			return ctx.Err()
		case <-ticker.C:
			created, err := s.IngestIssues(ctx)
			if err != nil {
				log.Printf("[PeriodicIngestion] Ingestion failed: %v", err)
				continue
			}
			if created > 0 {
				log.Printf("[PeriodicIngestion] Created %d new BrainTasks", created)
			}
		}
	}
}
