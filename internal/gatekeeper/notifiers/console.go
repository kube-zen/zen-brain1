// Package notifiers provides notification implementations for the Human Gatekeeper.
package notifiers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kube-zen/zen-brain1/internal/gatekeeper"
)

// ConsoleNotifier sends notifications to the console (stdout).
type ConsoleNotifier struct {
	name string
}

// NewConsoleNotifier creates a new ConsoleNotifier.
func NewConsoleNotifier() *ConsoleNotifier {
	return &ConsoleNotifier{
		name: "console",
	}
}

// Name returns the notifier name.
func (c *ConsoleNotifier) Name() string {
	return c.name
}

// SendApprovalRequest sends an approval request notification.
func (c *ConsoleNotifier) SendApprovalRequest(ctx context.Context, req *gatekeeper.ApprovalRequest) error {
	message := fmt.Sprintf(`
═══════════════════════════════════════════════════════════════════════════════
📋 APPROVAL REQUIRED
───────────────────────────────────────────────────────────────────────────────
Session:    %s
Work Item:  %s
Cost:       $%.2f
Priority:   %s
Level:      %s
Deadline:   %s
───────────────────────────────────────────────────────────────────────────────
Notes: %s
───────────────────────────────────────────────────────────────────────────────
Approve:  /approve %s [reason]
Reject:   /reject %s [reason]
═══════════════════════════════════════════════════════════════════════════════
`,
		req.SessionID,
		req.WorkItem.Title,
		req.EstimatedCostUSD,
		req.Priority,
		req.ApprovalLevel,
		req.Deadline.Format("2006-01-02 15:04"),
		req.Notes,
		req.ID,
		req.ID)
	
	log.Print(message)
	return nil
}

// SendApprovalDecision sends an approval decision notification.
func (c *ConsoleNotifier) SendApprovalDecision(ctx context.Context, req *gatekeeper.ApprovalRequest, decision gatekeeper.ApprovalDecision) error {
	message := fmt.Sprintf(`
═══════════════════════════════════════════════════════════════════════════════
✅ APPROVAL DECISION
───────────────────────────────────────────────────────────────────────────────
Session:    %s
Work Item:  %s
Decision:   %s
By:         %s
At:         %s
───────────────────────────────────────────────────────────────────────────────
Reason: %s
═══════════════════════════════════════════════════════════════════════════════
`,
		req.SessionID,
		req.WorkItem.Title,
		decision.Decision,
		decision.DecidedBy,
		decision.DecidedAt.Format("2006-01-02 15:04"),
		decision.Reason)
	
	log.Print(message)
	return nil
}

// SendReminder sends a reminder for a pending approval.
func (c *ConsoleNotifier) SendReminder(ctx context.Context, req *gatekeeper.ApprovalRequest) error {
	message := fmt.Sprintf(`
═══════════════════════════════════════════════════════════════════════════════
⏰ APPROVAL REMINDER
───────────────────────────────────────────────────────────────────────────────
Session:    %s
Work Item:  %s
Cost:       $%.2f
Deadline:   %s (in %v)
───────────────────────────────────────────────────────────────────────────────
Reminder: This approval request is still pending. Please review.
═══════════════════════════════════════════════════════════════════════════════
`,
		req.SessionID,
		req.WorkItem.Title,
		req.EstimatedCostUSD,
		req.Deadline.Format("2006-01-02 15:04"),
		time.Until(*req.Deadline).Round(time.Minute))
	
	log.Print(message)
	return nil
}

// SupportsChannel returns true if the notifier supports the given channel.
func (c *ConsoleNotifier) SupportsChannel(channel string) bool {
	return channel == "console"
}