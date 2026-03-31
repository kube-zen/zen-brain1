// Package feedback provides BrainTask -> Jira result reporting service (ZB-025).
// This service watches BrainTask status changes and reports results back to Jira.
package feedback

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/labels"
	"github.com/kube-zen/zen-brain1/internal/office/jira"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// BrainTaskToJiraConfig configures the feedback service.
type BrainTaskToJiraConfig struct {
	// JiraTransitionMapping maps BrainTask phases to Jira status transitions
	JiraTransitionMapping map[v1alpha1.BrainTaskPhase]string
	// IncludeProofOfWork determines if proof-of-work is attached to Jira comments
	IncludeProofOfWork bool
	// IncludeCommitLinks determines if commit/PR links are included
	IncludeCommitLinks bool
}

// DefaultBrainTaskToJiraConfig returns sensible defaults.
func DefaultBrainTaskToJiraConfig() *BrainTaskToJiraConfig {
	return &BrainTaskToJiraConfig{
		JiraTransitionMapping: map[v1alpha1.BrainTaskPhase]string{
			v1alpha1.BrainTaskPhaseRunning:   "In Progress",
			v1alpha1.BrainTaskPhaseCompleted: "Done",
			v1alpha1.BrainTaskPhaseFailed:    "Blocked",
			v1alpha1.BrainTaskPhaseCanceled:  "Cancelled",
		},
		IncludeProofOfWork: true,
		IncludeCommitLinks: true,
	}
}

// BrainTaskToJiraService reports BrainTask results back to Jira.
type BrainTaskToJiraService struct {
	k8sClient client.Client
	jiraConn  *jira.JiraOffice
	config    *BrainTaskToJiraConfig
}

// NewBrainTaskToJiraService creates a new feedback service.
func NewBrainTaskToJiraService(
	k8sClient client.Client,
	jiraConn *jira.JiraOffice,
	config *BrainTaskToJiraConfig,
) *BrainTaskToJiraService {
	if config == nil {
		config = DefaultBrainTaskToJiraConfig()
	}
	return &BrainTaskToJiraService{
		k8sClient: k8sClient,
		jiraConn:  jiraConn,
		config:    config,
	}
}

// ReportResult reports a BrainTask result to Jira.
func (s *BrainTaskToJiraService) ReportResult(ctx context.Context, task *v1alpha1.BrainTask) error {
	// Only report if task has SourceKey (Jira key)
	if task.Spec.SourceKey == "" {
		log.Printf("[ReportResult] Task %s has no SourceKey, skipping Jira update", task.Name)
		return nil
	}

	// Only report terminal states
	if task.Status.Phase != v1alpha1.BrainTaskPhaseCompleted &&
		task.Status.Phase != v1alpha1.BrainTaskPhaseFailed &&
		task.Status.Phase != v1alpha1.BrainTaskPhaseCanceled {
		log.Printf("[ReportResult] Task %s phase %s is not terminal, skipping", task.Name, task.Status.Phase)
		return nil
	}

	log.Printf("[ReportResult] Reporting task %s result to Jira (SourceKey: %s, Phase: %s)",
		task.Name, task.Spec.SourceKey, task.Status.Phase)

	// Build result comment
	comment := s.buildResultComment(task)

	// Update Jira issue
	if err := s.updateJiraIssue(ctx, task); err != nil {
		return fmt.Errorf("failed to update Jira issue: %w", err)
	}

	// Add comment with result
	if err := s.addJiraComment(ctx, task.Spec.SourceKey, comment); err != nil {
		return fmt.Errorf("failed to add Jira comment: %w", err)
	}

	log.Printf("[ReportResult] Successfully reported task %s result to Jira %s",
		task.Name, task.Spec.SourceKey)

	return nil
}

// buildResultComment builds a Jira comment from BrainTask result.
func (s *BrainTaskToJiraService) buildResultComment(task *v1alpha1.BrainTask) string {
	var sb strings.Builder

	// Header
	sb.WriteString("h2. Zen-Brain Execution Result\n\n")

	// Status
	emoji := "✅"
	if task.Status.Phase == v1alpha1.BrainTaskPhaseFailed {
		emoji = "❌"
	} else if task.Status.Phase == v1alpha1.BrainTaskPhaseCanceled {
		emoji = "⏹"
	}

	sb.WriteString(fmt.Sprintf("*Status:* %s %s\n\n", emoji, task.Status.Phase))

	// Task ID
	sb.WriteString(fmt.Sprintf("*Task ID:* %s\n", task.Name))
	sb.WriteString(fmt.Sprintf("*Work Type:* %s\n", task.Spec.WorkType))
	sb.WriteString(fmt.Sprintf("*Work Domain:* %s\n\n", task.Spec.WorkDomain))

	// Message
	if task.Status.Message != "" {
		sb.WriteString(fmt.Sprintf("*Message:*\n{code}\n%s\n{code}\n\n", task.Status.Message))
	}

	// Result summary
	sb.WriteString("h3. Execution Summary\n\n")
	sb.WriteString(fmt.Sprintf("*Objective:*\n%s\n\n", task.Spec.Objective))

	// Acceptance criteria
	if len(task.Spec.AcceptanceCriteria) > 0 {
		sb.WriteString("*Acceptance Criteria:*\n")
		for _, criteria := range task.Spec.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("# %s\n", criteria))
		}
		sb.WriteString("\n")
	}

	// Constraints
	if len(task.Spec.Constraints) > 0 {
		sb.WriteString("*Constraints:*\n")
		for _, constraint := range task.Spec.Constraints {
			sb.WriteString(fmt.Sprintf("* %s\n", constraint))
		}
		sb.WriteString("\n")
	}

	// TODO: Add proof-of-work link if IncludeProofOfWork is true
	// TODO: Add commit/PR links if IncludeCommitLinks is true

	// Footer
	sb.WriteString("\n---\n")
	sb.WriteString(fmt.Sprintf("*Reported by:* zen-brain1\n"))
	sb.WriteString(fmt.Sprintf("*Timestamp:* %s\n", time.Now().Format(time.RFC3339)))

	return sb.String()
}

// updateJiraIssue updates the Jira issue status based on BrainTask phase.
func (s *BrainTaskToJiraService) updateJiraIssue(ctx context.Context, task *v1alpha1.BrainTask) error {
	// Get transition from mapping
	transition, ok := s.config.JiraTransitionMapping[task.Status.Phase]
	if !ok {
		log.Printf("[updateJiraIssue] No transition mapping for phase %s", task.Status.Phase)
		return nil
	}

	// Map transition string to WorkStatus
	status := contracts.WorkStatus(transition)

	// Update issue status
	if err := s.jiraConn.UpdateStatus(ctx, "default", task.Spec.SourceKey, status); err != nil {
		return fmt.Errorf("failed to update Jira issue status: %w", err)
	}

	log.Printf("[updateJiraIssue] Updated Jira %s status to %s", task.Spec.SourceKey, transition)

	return nil
}

// addJiraComment adds a comment to a Jira issue.
func (s *BrainTaskToJiraService) addJiraComment(ctx context.Context, issueKey, comment string) error {
	// Add comment via Jira connector
	jiraComment := &contracts.Comment{
		Body: comment,
	}

	if err := s.jiraConn.AddComment(ctx, "default", issueKey, jiraComment); err != nil {
		return fmt.Errorf("failed to add Jira comment: %w", err)
	}

	log.Printf("[addJiraComment] Added comment to Jira %s", issueKey)

	return nil
}

// WatchAndReport watches BrainTask status changes and reports to Jira.
func (s *BrainTaskToJiraService) WatchAndReport(ctx context.Context, taskName string) error {
	// Get initial task
	task := &v1alpha1.BrainTask{}
	if err := s.k8sClient.Get(ctx, types.NamespacedName{
		Namespace: s.getNamespace(),
		Name:      taskName,
	}, task); err != nil {
		return fmt.Errorf("failed to get BrainTask: %w", err)
	}

	// Check if already reported
	if s.isReported(task) {
		log.Printf("[WatchAndReport] Task %s already reported", taskName)
		return nil
	}

	// Report result
	if err := s.ReportResult(ctx, task); err != nil {
		return err
	}

	// Mark as reported
	if err := s.markReported(ctx, task); err != nil {
		log.Printf("[WatchAndReport] Failed to mark task as reported: %v", err)
	}

	return nil
}

// isReported checks if a BrainTask has already been reported to Jira.
// Uses labels.GetReportedToJira which reads the new brain.zen-mesh.io key first
// and falls back to the legacy zen.kube-zen.com key.
func (s *BrainTaskToJiraService) isReported(task *v1alpha1.BrainTask) bool {
	return labels.GetReportedToJira(task.Labels)
}

// markReported marks a BrainTask as reported to Jira.
func (s *BrainTaskToJiraService) markReported(ctx context.Context, task *v1alpha1.BrainTask) error {
	// Add label
	task.Labels = labels.EnsureLabels(task.Labels)
	labels.SetReportedToJira(task.Labels)

	// Update task
	if err := s.k8sClient.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update BrainTask labels: %w", err)
	}

	return nil
}

// getNamespace returns the namespace to use for BrainTask lookups.
func (s *BrainTaskToJiraService) getNamespace() string {
	// TODO: Make this configurable
	return "zen-brain"
}

// StartFeedbackLoop starts a loop that watches all BrainTasks and reports results.
func (s *BrainTaskToJiraService) StartFeedbackLoop(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[FeedbackLoop] Starting feedback loop (interval: %v)", interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[FeedbackLoop] Context cancelled, stopping")
			return ctx.Err()
		case <-ticker.C:
			// List all BrainTasks
			taskList := &v1alpha1.BrainTaskList{}
			if err := s.k8sClient.List(ctx, taskList, client.InNamespace(s.getNamespace())); err != nil {
				log.Printf("[FeedbackLoop] Failed to list BrainTasks: %v", err)
				continue
			}

			// Check each task
			reported := 0
			for _, task := range taskList.Items {
				// Skip if already reported
				if s.isReported(&task) {
					continue
				}

				// Report result
				if err := s.ReportResult(ctx, &task); err != nil {
					log.Printf("[FeedbackLoop] Failed to report task %s: %v", task.Name, err)
					continue
				}

				// Mark as reported
				if err := s.markReported(ctx, &task); err != nil {
					log.Printf("[FeedbackLoop] Failed to mark task %s as reported: %v", task.Name, err)
				}

				reported++
			}

			if reported > 0 {
				log.Printf("[FeedbackLoop] Reported %d BrainTask results to Jira", reported)
			}
		}
	}
}
