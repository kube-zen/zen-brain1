// Package gatekeeper provides the Human Gatekeeper for zen-brain.
package gatekeeper

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// DefaultGatekeeper is the default implementation of Gatekeeper.
type DefaultGatekeeper struct {
	config  *Config
	planner planner.Planner

	// Internal state
	approvalRequests map[string]*ApprovalRequest
	approvalEvents   map[string][]*ApprovalEvent
	notifiers        map[string]Notifier

	// Mutex for thread safety
	mu sync.RWMutex

	// Background tasks
	reminderTicker   *time.Ticker
	escalationTicker *time.Ticker
	shutdownChan     chan struct{}
	shutdownWg       sync.WaitGroup

	// Audit logging
	auditLogFile *os.File
}

// New creates a new DefaultGatekeeper.
func New(config *Config) (*DefaultGatekeeper, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if config.Planner == nil {
		return nil, fmt.Errorf("Planner is required")
	}

	gatekeeper := &DefaultGatekeeper{
		config:           config,
		planner:          config.Planner,
		approvalRequests: make(map[string]*ApprovalRequest),
		approvalEvents:   make(map[string][]*ApprovalEvent),
		notifiers:        make(map[string]Notifier),
		shutdownChan:     make(chan struct{}),
	}

	// Setup audit logging
	if config.AuditLogEnabled {
		if err := gatekeeper.setupAuditLog(); err != nil {
			log.Printf("Failed to setup audit log: %v", err)
		}
	}

	// Start background tasks
	gatekeeper.startBackgroundTasks()

	return gatekeeper, nil
}

// GetPendingApprovals returns sessions waiting for human approval.
func (g *DefaultGatekeeper) GetPendingApprovals(ctx context.Context, filter ApprovalFilter) ([]*ApprovalRequest, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*ApprovalRequest

	// First, get pending sessions from planner
	sessions, err := g.planner.GetPendingApprovals(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending sessions: %w", err)
	}

	// Convert sessions to approval requests
	for _, session := range sessions {
		approvalID := fmt.Sprintf("approval-%s", session.ID)

		// Check if we already have an approval request for this session
		req, exists := g.approvalRequests[approvalID]
		if !exists {
			// Create new approval request
			req = g.createApprovalRequest(session)
			g.approvalRequests[approvalID] = req

			// Record creation event
			g.recordApprovalEvent(ctx, req.ID, session.ID, "requested", "system", nil)

			// Send notifications
			g.sendApprovalNotifications(ctx, req)
		}

		// Apply filter
		if g.matchesFilter(req, filter) {
			result = append(result, req)
		}

		// Apply limit if specified
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}

	return result, nil
}

// GetApproval returns a specific approval request.
func (g *DefaultGatekeeper) GetApproval(ctx context.Context, approvalID string) (*ApprovalRequest, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	req, exists := g.approvalRequests[approvalID]
	if !exists {
		return nil, fmt.Errorf("approval request %s not found", approvalID)
	}

	return req, nil
}

// Approve approves a session.
func (g *DefaultGatekeeper) Approve(ctx context.Context, approvalID string, decision ApprovalDecision) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	req, exists := g.approvalRequests[approvalID]
	if !exists {
		return fmt.Errorf("approval request %s not found", approvalID)
	}

	if req.Status != ApprovalStatusPending {
		return fmt.Errorf("approval request %s is not pending (status: %s)", approvalID, req.Status)
	}

	// Validate decision
	if decision.Decision != "approved" && decision.Decision != "rejected" {
		return fmt.Errorf("invalid decision: %s", decision.Decision)
	}

	if decision.DecidedBy == "" {
		return fmt.Errorf("decided_by is required")
	}

	if decision.DecidedAt.IsZero() {
		decision.DecidedAt = time.Now()
	}

	// Call planner to approve/reject the session
	if decision.Decision == "approved" {
		if err := g.planner.ApproveSession(ctx, req.SessionID, decision.DecidedBy, decision.Reason); err != nil {
			return fmt.Errorf("failed to approve session: %w", err)
		}
	} else {
		if err := g.planner.RejectSession(ctx, req.SessionID, decision.DecidedBy, decision.Reason); err != nil {
			return fmt.Errorf("failed to reject session: %w", err)
		}
	}

	// Update approval request
	req.Status = ApprovalStatusApproved
	if decision.Decision == "rejected" {
		req.Status = ApprovalStatusRejected
	}

	// Record decision event
	eventDetails := map[string]string{
		"decision": decision.Decision,
		"reason":   decision.Reason,
	}
	g.recordApprovalEvent(ctx, req.ID, req.SessionID, decision.Decision, decision.DecidedBy, eventDetails)

	// Send decision notifications
	g.sendDecisionNotifications(ctx, req, decision)

	// Audit log
	g.auditLogDecision(ctx, req, decision)

	log.Printf("Approval %s %s by %s: %s", approvalID, decision.Decision, decision.DecidedBy, decision.Reason)

	return nil
}

// Reject rejects a session.
func (g *DefaultGatekeeper) Reject(ctx context.Context, approvalID string, decision ApprovalDecision) error {
	// Reject is just a specific case of Approve with decision="rejected"
	decision.Decision = "rejected"
	return g.Approve(ctx, approvalID, decision)
}

// DelegateApproval delegates an approval to another user.
func (g *DefaultGatekeeper) DelegateApproval(ctx context.Context, approvalID string, delegateTo string, reason string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	req, exists := g.approvalRequests[approvalID]
	if !exists {
		return fmt.Errorf("approval request %s not found", approvalID)
	}

	if req.Status != ApprovalStatusPending {
		return fmt.Errorf("approval request %s is not pending (status: %s)", approvalID, req.Status)
	}

	// Update assigned users
	req.AssignedTo = []string{delegateTo}
	req.Status = ApprovalStatusDelegated

	// Record delegation event
	eventDetails := map[string]string{
		"delegate_to": delegateTo,
		"reason":      reason,
	}
	g.recordApprovalEvent(ctx, req.ID, req.SessionID, "delegated", "system", eventDetails)

	// Send delegation notifications
	g.sendDelegationNotifications(ctx, req, delegateTo, reason)

	log.Printf("Approval %s delegated to %s: %s", approvalID, delegateTo, reason)

	return nil
}

// EscalateApproval escalates an approval to a higher authority.
func (g *DefaultGatekeeper) EscalateApproval(ctx context.Context, approvalID string, reason string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	req, exists := g.approvalRequests[approvalID]
	if !exists {
		return fmt.Errorf("approval request %s not found", approvalID)
	}

	if req.Status != ApprovalStatusPending {
		return fmt.Errorf("approval request %s is not pending (status: %s)", approvalID, req.Status)
	}

	// Determine next approval level
	nextLevel := g.getNextApprovalLevel(req.ApprovalLevel)
	if nextLevel == "" {
		return fmt.Errorf("cannot escalate beyond highest approval level")
	}

	// Update approval level
	req.ApprovalLevel = nextLevel
	req.Status = ApprovalStatusEscalated

	// Record escalation event
	eventDetails := map[string]string{
		"from_level": req.ApprovalLevel,
		"to_level":   nextLevel,
		"reason":     reason,
	}
	g.recordApprovalEvent(ctx, req.ID, req.SessionID, "escalated", "system", eventDetails)

	// Send escalation notifications
	g.sendEscalationNotifications(ctx, req, nextLevel, reason)

	log.Printf("Approval %s escalated to %s: %s", approvalID, nextLevel, reason)

	return nil
}

// GetApprovalHistory returns approval history for a session.
func (g *DefaultGatekeeper) GetApprovalHistory(ctx context.Context, sessionID string) ([]*ApprovalEvent, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Find approval ID for this session
	var approvalID string
	for id, req := range g.approvalRequests {
		if req.SessionID == sessionID {
			approvalID = id
			break
		}
	}

	if approvalID == "" {
		return nil, fmt.Errorf("no approval request found for session %s", sessionID)
	}

	events, exists := g.approvalEvents[approvalID]
	if !exists {
		return []*ApprovalEvent{}, nil
	}

	return events, nil
}

// RegisterNotifier registers a notification channel.
func (g *DefaultGatekeeper) RegisterNotifier(ctx context.Context, notifier Notifier) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.notifiers[notifier.Name()]; exists {
		return fmt.Errorf("notifier %s already registered", notifier.Name())
	}

	g.notifiers[notifier.Name()] = notifier
	log.Printf("Registered notifier: %s", notifier.Name())

	return nil
}

// Close closes the gatekeeper.
func (g *DefaultGatekeeper) Close() error {
	close(g.shutdownChan)
	g.shutdownWg.Wait()

	if g.reminderTicker != nil {
		g.reminderTicker.Stop()
	}

	if g.escalationTicker != nil {
		g.escalationTicker.Stop()
	}

	if g.auditLogFile != nil {
		g.auditLogFile.Close()
	}

	return nil
}

// Helper methods

// createApprovalRequest creates an approval request from a session.
func (g *DefaultGatekeeper) createApprovalRequest(session *contracts.Session) *ApprovalRequest {
	now := time.Now()
	deadline := now.Add(time.Duration(g.config.DefaultDeadlineHours) * time.Hour)

	// Determine priority based on estimated cost
	priority := "medium"
	if session.AnalysisResult != nil {
		cost := session.AnalysisResult.EstimatedTotalCostUSD
		if cost < 1.0 {
			priority = "low"
		} else if cost > 5.0 {
			priority = "high"
		} else if cost > 10.0 {
			priority = "critical"
		}
	}

	return &ApprovalRequest{
		ID:               fmt.Sprintf("approval-%s", session.ID),
		SessionID:        session.ID,
		WorkItem:         session.WorkItem,
		Analysis:         session.AnalysisResult,
		EstimatedCostUSD: g.getEstimatedCost(session),
		Requester:        "planner",
		RequestedAt:      now,
		Deadline:         &deadline,
		Priority:         priority,
		ApprovalLevel:    g.getDefaultApprovalLevel(),
		Notes:            g.getApprovalNotes(session),
		AssignedTo:       []string{"team-lead"}, // Default assignee
		Status:           ApprovalStatusPending,
	}
}

// getEstimatedCost gets the estimated cost from a session.
func (g *DefaultGatekeeper) getEstimatedCost(session *contracts.Session) float64 {
	if session.AnalysisResult != nil {
		return session.AnalysisResult.EstimatedTotalCostUSD
	}
	return 0.0
}

// getApprovalNotes generates approval notes from a session.
func (g *DefaultGatekeeper) getApprovalNotes(session *contracts.Session) string {
	if session.AnalysisResult != nil {
		return session.AnalysisResult.AnalysisNotes
	}

	if session.WorkItem != nil {
		return fmt.Sprintf("Work item: %s\n%s", session.WorkItem.Title, session.WorkItem.Body)
	}

	return "No analysis available"
}

// matchesFilter checks if an approval request matches the filter.
func (g *DefaultGatekeeper) matchesFilter(req *ApprovalRequest, filter ApprovalFilter) bool {
	if filter.Status != nil && *filter.Status != req.Status {
		return false
	}

	if filter.AssignedTo != nil {
		found := false
		for _, assigned := range req.AssignedTo {
			if assigned == *filter.AssignedTo {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if filter.Priority != nil && *filter.Priority != req.Priority {
		return false
	}

	if filter.ApprovalLevel != nil && *filter.ApprovalLevel != req.ApprovalLevel {
		return false
	}

	if filter.SessionID != nil && *filter.SessionID != req.SessionID {
		return false
	}

	if filter.WorkItemID != nil && req.WorkItem != nil && *filter.WorkItemID != req.WorkItem.ID {
		return false
	}

	if filter.RequestedAfter != nil && req.RequestedAt.Before(*filter.RequestedAfter) {
		return false
	}

	if filter.RequestedBefore != nil && req.RequestedAt.After(*filter.RequestedBefore) {
		return false
	}

	return true
}

// getDefaultApprovalLevel returns the default approval level.
func (g *DefaultGatekeeper) getDefaultApprovalLevel() string {
	if g.config.DefaultApprovalLevel != "" {
		return g.config.DefaultApprovalLevel
	}
	return "team_lead"
}

// getNextApprovalLevel returns the next approval level in the hierarchy.
func (g *DefaultGatekeeper) getNextApprovalLevel(currentLevel string) string {
	// If current level is empty, start with team_lead
	if currentLevel == "" {
		return "team_lead"
	}

	levels := map[string]string{
		"team_lead": "manager",
		"manager":   "director",
		"director":  "executive",
		"executive": "", // No higher level
	}

	nextLevel := levels[currentLevel]
	if nextLevel == "" {
		return "" // Already at highest level
	}

	return nextLevel
}

// recordApprovalEvent records an approval event.
func (g *DefaultGatekeeper) recordApprovalEvent(ctx context.Context, approvalID, sessionID, eventType, actor string, details map[string]string) {
	event := &ApprovalEvent{
		ID:         fmt.Sprintf("event-%d", time.Now().UnixNano()),
		ApprovalID: approvalID,
		SessionID:  sessionID,
		EventType:  eventType,
		Actor:      actor,
		Timestamp:  time.Now(),
		Details:    details,
	}

	g.approvalEvents[approvalID] = append(g.approvalEvents[approvalID], event)
}

// Notification methods

// sendApprovalNotifications sends approval request notifications.
func (g *DefaultGatekeeper) sendApprovalNotifications(ctx context.Context, req *ApprovalRequest) {
	for _, notifier := range g.notifiers {
		if err := notifier.SendApprovalRequest(ctx, req); err != nil {
			log.Printf("Failed to send approval notification via %s: %v", notifier.Name(), err)
		}
	}
}

// sendDecisionNotifications sends approval decision notifications.
func (g *DefaultGatekeeper) sendDecisionNotifications(ctx context.Context, req *ApprovalRequest, decision ApprovalDecision) {
	for _, notifier := range g.notifiers {
		if err := notifier.SendApprovalDecision(ctx, req, decision); err != nil {
			log.Printf("Failed to send decision notification via %s: %v", notifier.Name(), err)
		}
	}
}

// sendDelegationNotifications sends delegation notifications.
func (g *DefaultGatekeeper) sendDelegationNotifications(ctx context.Context, req *ApprovalRequest, delegateTo, reason string) {
	// For now, just log. In a real implementation, would send specific notifications.
	log.Printf("Delegation notification: approval %s delegated to %s", req.ID, delegateTo)
}

// sendEscalationNotifications sends escalation notifications.
func (g *DefaultGatekeeper) sendEscalationNotifications(ctx context.Context, req *ApprovalRequest, newLevel, reason string) {
	// For now, just log. In a real implementation, would send specific notifications.
	log.Printf("Escalation notification: approval %s escalated to %s", req.ID, newLevel)
}

// Background tasks

// startBackgroundTasks starts background monitoring tasks.
func (g *DefaultGatekeeper) startBackgroundTasks() {
	// Reminder ticker
	if g.config.ReminderInterval > 0 {
		g.reminderTicker = time.NewTicker(g.config.ReminderInterval)
		g.shutdownWg.Add(1)
		go func() {
			defer g.shutdownWg.Done()
			g.reminderRoutine()
		}()
	}

	// Escalation ticker
	if g.config.EscalationInterval > 0 {
		g.escalationTicker = time.NewTicker(g.config.EscalationInterval)
		g.shutdownWg.Add(1)
		go func() {
			defer g.shutdownWg.Done()
			g.escalationRoutine()
		}()
	}
}

// reminderRoutine sends reminders for pending approvals.
func (g *DefaultGatekeeper) reminderRoutine() {
	for {
		select {
		case <-g.reminderTicker.C:
			g.sendReminders()
		case <-g.shutdownChan:
			return
		}
	}
}

// escalationRoutine escalates overdue approvals.
func (g *DefaultGatekeeper) escalationRoutine() {
	for {
		select {
		case <-g.escalationTicker.C:
			g.checkEscalations()
		case <-g.shutdownChan:
			return
		}
	}
}

// sendReminders sends reminders for pending approvals.
func (g *DefaultGatekeeper) sendReminders() {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ctx := context.Background()

	for _, req := range g.approvalRequests {
		if req.Status == ApprovalStatusPending && req.Deadline != nil {
			// Check if reminder is due (halfway to deadline)
			halfway := req.RequestedAt.Add(time.Until(*req.Deadline) / 2)
			if time.Now().After(halfway) {
				for _, notifier := range g.notifiers {
					if err := notifier.SendReminder(ctx, req); err != nil {
						log.Printf("Failed to send reminder via %s: %v", notifier.Name(), err)
					}
				}

				// Record reminder event
				g.recordApprovalEvent(ctx, req.ID, req.SessionID, "reminded", "system", nil)
			}
		}
	}
}

// checkEscalations escalates overdue approvals.
func (g *DefaultGatekeeper) checkEscalations() {
	g.mu.Lock()
	defer g.mu.Unlock()

	ctx := context.Background()

	for _, req := range g.approvalRequests {
		if req.Status == ApprovalStatusPending && req.Deadline != nil {
			// Check if overdue
			if time.Now().After(*req.Deadline) {
				// Auto-escalate
				nextLevel := g.getNextApprovalLevel(req.ApprovalLevel)
				if nextLevel != "" {
					req.ApprovalLevel = nextLevel

					// Record escalation event
					eventDetails := map[string]string{
						"reason": "deadline passed",
					}
					g.recordApprovalEvent(ctx, req.ID, req.SessionID, "auto_escalated", "system", eventDetails)

					log.Printf("Auto-escalated approval %s to %s (deadline passed)", req.ID, nextLevel)
				}
			}
		}
	}
}

// Audit logging

// setupAuditLog sets up audit logging.
func (g *DefaultGatekeeper) setupAuditLog() error {
	// Create audit directory
	if err := os.MkdirAll(g.config.AuditLogDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create audit directory: %w", err)
	}

	// Open audit log file
	logPath := filepath.Join(g.config.AuditLogDirectory, fmt.Sprintf("audit-%s.log", time.Now().Format("2006-01-02")))
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}

	g.auditLogFile = file
	return nil
}

// auditLogDecision logs an approval decision to the audit log.
func (g *DefaultGatekeeper) auditLogDecision(ctx context.Context, req *ApprovalRequest, decision ApprovalDecision) {
	if g.auditLogFile == nil {
		return
	}

	entry := map[string]interface{}{
		"timestamp":      time.Now().Format(time.RFC3339),
		"action":         "approval_decision",
		"approval_id":    req.ID,
		"session_id":     req.SessionID,
		"work_item_id":   req.WorkItem.ID,
		"decision":       decision.Decision,
		"decided_by":     decision.DecidedBy,
		"reason":         decision.Reason,
		"estimated_cost": req.EstimatedCostUSD,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal audit log entry: %v", err)
		return
	}

	data = append(data, '\n')

	g.auditLogFile.Write(data)
	g.auditLogFile.Sync()
}
