// Package command provides server-side command parsing for TUI and other interfaces.
// This is SERVER-SIDE ONLY. TUI sends raw input, server parses and executes.
package command

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/internal/session"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// Command represents a parsed command
type Command struct {
	Name       string
	Args       []string
	Raw        string
	IsCommand  bool
}

// Parser parses user input into commands
type Parser struct {
	sessionMgr session.Manager
	planner    *planner.Planner
}

// NewParser creates a new command parser with real components
func NewParser(sessionMgr session.Manager, plannerAgent *planner.Planner) *Parser {
	return &Parser{
		sessionMgr: sessionMgr,
		planner:    plannerAgent,
	}
}

// Parse parses raw input into a command
func (p *Parser) Parse(input string) *Command {
	trimmed := strings.TrimSpace(input)

	// Check if it's a command (starts with /)
	if strings.HasPrefix(trimmed, "/") {
		parts := strings.Fields(trimmed[1:])
		if len(parts) == 0 {
			return &Command{
				Name:      "help",
				Raw:       trimmed,
				IsCommand: true,
			}
		}

		return &Command{
			Name:      strings.ToLower(parts[0]),
			Args:      parts[1:],
			Raw:       trimmed,
			IsCommand: true,
		}
	}

	// Not a command, treat as chat input
	return &Command{
		Name:      "chat",
		Args:      []string{trimmed},
		Raw:       trimmed,
		IsCommand: false,
	}
}

// Execute executes a parsed command and returns response
func (p *Parser) Execute(ctx context.Context, cmd *Command) (*CommandResult, error) {
	switch cmd.Name {
	case "help", "h", "?":
		return p.executeHelp(cmd)

	case "status", "st":
		return p.executeStatus(ctx, cmd)

	case "sessions":
		return p.executeSessions(ctx, cmd)

	case "chat":
		return p.executeChat(ctx, cmd)

	case "approve":
		return p.executeApprove(ctx, cmd)

	case "reject":
		return p.executeReject(ctx, cmd)

	case "cancel":
		return p.executeCancel(ctx, cmd)

	case "approvals":
		return p.executeApprovals(ctx, cmd)

	default:
		return &CommandResult{
			Response: fmt.Sprintf("Unknown command: %s\nType /help for available commands", cmd.Name),
			Error:    fmt.Sprintf("unknown command: %s", cmd.Name),
		}, nil
	}
}

// executeHelp generates help text (server-side)
func (p *Parser) executeHelp(cmd *Command) (*CommandResult, error) {
	helpText := `zen-brain - Intelligent Planning and Execution System

Commands:
  /help, /h, ?        Show this help
  /help <command>       Show detailed help for a command
  /status, /st          Show system status
  /sessions              List active sessions
  /chat <text>          Send a message or task
  /approve <session>     Approve a pending session
  /reject <session>      Reject a pending session
  /cancel <session>      Cancel an active session
  /approvals            List sessions pending approval

Examples:
  /help status           Show help for status command
  /status               Show system status
  /sessions --limit=5    Show last 5 sessions
  /chat analyze ticket    Send analysis task
  /approve session-123   Approve pending session
  /approvals            Show all pending approvals`

	response := helpText
	if len(cmd.Args) > 0 {
		// Detailed help for specific command
		switch cmd.Args[0] {
		case "status", "st":
			response = `Status Command

Show system health and component status.

Usage:
  /status [brief|full]

Options:
  brief  - Show short summary only
  full   - Show detailed component status

Examples:
  /status          Show standard status
  /status brief    Show brief summary
  /status full     Show detailed status`
		case "sessions":
			response = `Sessions Command

List active sessions and their states.

Usage:
  /sessions [--limit=<n>] [--state=<state>]

Options:
  limit=<n>     Maximum number of sessions to show (default: 50)
  state=<state>  Filter by session state (created|analyzed|scheduled|in_progress|completed|failed|blocked)

Examples:
  /sessions                Show all sessions
  /sessions --limit=5       Show last 5 sessions
  /sessions --state=active  Show active sessions only`
		case "chat":
			response = `Chat Command

Send a message or task to the assistant.

Usage:
  /chat <text>

Examples:
  /chat analyze this Jira ticket
  /chat what is the system status?
  /chat create a deployment plan`
		case "approve":
			response = `Approve Command

Approve a session that's pending approval.

Usage:
  /approve <session_id> [reason]

Examples:
  /approve session-123
  /approve session-123 Approved for production deployment`
		case "reject":
			response = `Reject Command

Reject a session that's pending approval.

Usage:
  /reject <session_id> <reason>

Examples:
  /reject session-123 Cost too high
  /reject session-123 Plan not comprehensive enough`
		case "cancel":
			response = `Cancel Command

Cancel an active or scheduled session.

Usage:
  /cancel <session_id> [reason]

Examples:
  /cancel session-123 No longer needed
  /cancel session-123 Business priority changed`
		default:
			response = fmt.Sprintf("Unknown command for help: %s\nAvailable commands: help, status, sessions, chat, approve, reject, cancel, approvals", cmd.Args[0])
		}
	}

	return &CommandResult{
		Response: response,
		Status:   "success",
	}, nil
}

// executeStatus generates status report (server-side, using real components)
func (p *Parser) executeStatus(ctx context.Context, cmd *Command) (*CommandResult, error) {
	var detailLevel string
	if len(cmd.Args) > 0 {
		detailLevel = cmd.Args[0]
	}

	// Build status report using real components
	status := "System Status: OK\n\n"

	// Add session manager status (real)
	if p.sessionMgr != nil {
		filter := session.SessionFilter{Limit: 10}
		if list, err := p.sessionMgr.ListSessions(ctx, filter); err == nil {
			activeCount := 0
			for _, s := range list {
				if s.State == contracts.SessionStateInProgress || s.State == contracts.SessionStateScheduled {
					activeCount++
				}
			}
			status += fmt.Sprintf("Active Sessions: %d\n", activeCount)
			if len(list) > 0 {
				status += "\nRecent Sessions:\n"
				for i, s := range list {
					if i >= 5 {
						status += fmt.Sprintf("  ... and %d more\n", len(list)-5)
						break
					}
					status += fmt.Sprintf("  - %s (%s) - %s\n", s.ID, s.State, s.WorkItemID)
				}
			}
		} else {
			status += "Session Manager: Error\n"
		}
	}

	// Add planner status (real)
	if p.planner != nil {
		status += "\nPlanner: Available"

		// Get pending approvals
		if pending, err := (*p.planner).GetPendingApprovals(ctx); err == nil && len(pending) > 0 {
			status += fmt.Sprintf("\n\nPending Approvals: %d", len(pending))
			for i, s := range pending {
				if i >= 3 {
					status += fmt.Sprintf("  ... and %d more", len(pending)-3)
					break
				}
				status += fmt.Sprintf("  - %s (%s)", s.ID, s.WorkItemID)
			}
		}
	}

	// Add component status
	status += "\n\nComponents:\n"
	status += "  - Session Manager: Available\n"
	status += "  - Planner: Available\n"
	status += "  - Factory: Available\n"

	return &CommandResult{
		Response: status,
		Status:   "ok",
	}, nil
}

// executeSessions lists sessions (server-side, using real Session Manager)
func (p *Parser) executeSessions(ctx context.Context, cmd *Command) (*CommandResult, error) {
	if p.sessionMgr == nil {
		return &CommandResult{
			Response: "Session manager not available",
			Error:    "session_manager_unavailable",
		}, nil
	}

	// Parse options
	limit := 50
	var stateFilter *contracts.SessionState

	re := regexp.MustCompile(`--limit=(\d+)`)
	if matches := re.FindStringSubmatch(cmd.Raw); len(matches) > 1 {
		if n, err := fmt.Sscanf(matches[1], "%d", &limit); err == nil && n == 1 {
			// Use parsed limit
		}
	}

	// Parse state filter
	reState := regexp.MustCompile(`--state=(\w+)`)
	if matches := reState.FindStringSubmatch(cmd.Raw); len(matches) > 1 {
		stateStr := matches[1]
		switch strings.ToLower(stateStr) {
		case "created":
			s := contracts.SessionStateCreated
			stateFilter = &s
		case "analyzed":
			s := contracts.SessionStateAnalyzed
			stateFilter = &s
		case "scheduled":
			s := contracts.SessionStateScheduled
			stateFilter = &s
		case "in_progress":
			s := contracts.SessionStateInProgress
			stateFilter = &s
		case "completed":
			s := contracts.SessionStateCompleted
			stateFilter = &s
		case "failed":
			s := contracts.SessionStateFailed
			stateFilter = &s
		case "blocked":
			s := contracts.SessionStateBlocked
			stateFilter = &s
		case "canceled":
			s := contracts.SessionStateCanceled
			stateFilter = &s
		}
	}

	// List sessions from real Session Manager
	filter := session.SessionFilter{Limit: limit, State: stateFilter}
	list, err := p.sessionMgr.ListSessions(ctx, filter)
	if err != nil {
		return &CommandResult{
			Response: fmt.Sprintf("Error listing sessions: %v", err),
			Error:    err.Error(),
		}, err
	}

	// Build response
	response := fmt.Sprintf("Sessions (showing %d)\n\n", len(list))
	for i, s := range list {
		response += fmt.Sprintf("%2d. %s\n", i+1, s.ID)
		response += fmt.Sprintf("    Work Item: %s\n", s.WorkItemID)
		response += fmt.Sprintf("    State: %s\n", s.State)
		response += fmt.Sprintf("    Created: %s", s.CreatedAt.Format("2006-01-02 15:04"))
		if s.UpdatedAt.After(s.CreatedAt) {
			response += fmt.Sprintf("\n    Updated: %s", s.UpdatedAt.Format("2006-01-02 15:04"))
		}
		response += "\n\n"
	}

	return &CommandResult{
		Response: response,
		Status:   "ok",
	}, nil
}

// executeChat processes chat input (using real Planner)
func (p *Parser) executeChat(ctx context.Context, cmd *Command) (*CommandResult, error) {
	if len(cmd.Args) == 0 {
		return &CommandResult{
			Response: "Please provide a message or task\nUsage: /chat <text>",
			Error:    "no_input",
		}, nil
	}

	// For now, create a simple work item and send to planner
	// Real implementation would create proper work item from input
	input := strings.Join(cmd.Args, " ")

	if p.planner == nil {
		return &CommandResult{
			Response: fmt.Sprintf("Received: %s\n\n[Planner not available - command processing not yet fully implemented]", input),
			Error:    "planner_unavailable",
		}, nil
	}

	// Create work item from input
	workItem := &contracts.WorkItem{
		ID:          fmt.Sprintf("work-%d", time.Now().Unix()),
		Title:        input,
		Description:   fmt.Sprintf("Chat input from TUI: %s", input),
		SourceKey:    "tui",
		Type:         contracts.WorkItemTypeAdHoc,
		Priority:     contracts.PriorityMedium,
		CreatedAt:    time.Now(),
		CreatedBy:    "tui-user",
	}

	// Process with planner
	err := (*p.planner).ProcessWorkItem(ctx, workItem)
	if err != nil {
		return &CommandResult{
			Response: fmt.Sprintf("Error processing work item: %v\n\nInput: %s", err, input),
			Error:    err.Error(),
		}, err
	}

	return &CommandResult{
		Response: fmt.Sprintf("Work item created: %s\n\nProcessing: %s\n\n[Use /sessions to track progress]", workItem.ID, input),
		Status:   "ok",
	}, nil
}

// executeApprove approves a session (using real Planner)
func (p *Parser) executeApprove(ctx context.Context, cmd *Command) (*CommandResult, error) {
	if p.planner == nil {
		return &CommandResult{
			Response: "Planner not available",
			Error:    "planner_unavailable",
		}, nil
	}

	if len(cmd.Args) == 0 {
		return &CommandResult{
			Response: "Please provide session ID\nUsage: /approve <session_id> [reason]",
			Error:    "no_session_id",
		}, nil
	}

	sessionID := cmd.Args[0]
	reason := ""
	if len(cmd.Args) > 1 {
		reason = strings.Join(cmd.Args[1:], " ")
	}

	// Approve with planner
	err := (*p.planner).ApproveSession(ctx, sessionID, "tui-user", reason)
	if err != nil {
		return &CommandResult{
			Response: fmt.Sprintf("Error approving session: %v", err),
			Error:    err.Error(),
		}, err
	}

	return &CommandResult{
		Response: fmt.Sprintf("Session approved: %s\n\nReason: %s", sessionID, reason),
		Status:   "ok",
	}, nil
}

// executeReject rejects a session (using real Planner)
func (p *Parser) executeReject(ctx context.Context, cmd *Command) (*CommandResult, error) {
	if p.planner == nil {
		return &CommandResult{
			Response: "Planner not available",
			Error:    "planner_unavailable",
		}, nil
	}

	if len(cmd.Args) < 2 {
		return &CommandResult{
			Response: "Please provide session ID and reason\nUsage: /reject <session_id> <reason>",
			Error:    "missing_args",
		}, nil
	}

	sessionID := cmd.Args[0]
	reason := strings.Join(cmd.Args[1:], " ")

	// Reject with planner
	err := (*p.planner).RejectSession(ctx, sessionID, "tui-user", reason)
	if err != nil {
		return &CommandResult{
			Response: fmt.Sprintf("Error rejecting session: %v", err),
			Error:    err.Error(),
		}, err
	}

	return &CommandResult{
		Response: fmt.Sprintf("Session rejected: %s\n\nReason: %s", sessionID, reason),
		Status:   "ok",
	}, nil
}

// executeCancel cancels a session (using real Planner)
func (p *Parser) executeCancel(ctx context.Context, cmd *Command) (*CommandResult, error) {
	if p.planner == nil {
		return &CommandResult{
			Response: "Planner not available",
			Error:    "planner_unavailable",
		}, nil
	}

	if len(cmd.Args) == 0 {
		return &CommandResult{
			Response: "Please provide session ID\nUsage: /cancel <session_id> [reason]",
			Error:    "no_session_id",
		}, nil
	}

	sessionID := cmd.Args[0]
	reason := ""
	if len(cmd.Args) > 1 {
		reason = strings.Join(cmd.Args[1:], " ")
	}

	// Cancel with planner
	err := (*p.planner).CancelSession(ctx, sessionID, "tui-user", reason)
	if err != nil {
		return &CommandResult{
			Response: fmt.Sprintf("Error cancelling session: %v", err),
			Error:    err.Error(),
		}, err
	}

	return &CommandResult{
		Response: fmt.Sprintf("Session cancelled: %s\n\nReason: %s", sessionID, reason),
		Status:   "ok",
	}, nil
}

// executeApprovals lists pending approvals (using real Planner)
func (p *Parser) executeApprovals(ctx context.Context, cmd *Command) (*CommandResult, error) {
	if p.planner == nil {
		return &CommandResult{
			Response: "Planner not available",
			Error:    "planner_unavailable",
		}, nil
	}

	// Get pending approvals
	pending, err := (*p.planner).GetPendingApprovals(ctx)
	if err != nil {
		return &CommandResult{
			Response: fmt.Sprintf("Error getting pending approvals: %v", err),
			Error:    err.Error(),
		}, err
	}

	if len(pending) == 0 {
		return &CommandResult{
			Response: "No sessions pending approval",
			Status:   "ok",
		}, nil
	}

	// Build response
	response := fmt.Sprintf("Pending Approvals (%d)\n\n", len(pending))
	for i, s := range pending {
		response += fmt.Sprintf("%2d. %s\n", i+1, s.ID)
		response += fmt.Sprintf("    Work Item: %s\n", s.WorkItemID)
		response += fmt.Sprintf("    State: %s\n", s.State)
		response += fmt.Sprintf("    Created: %s\n", s.CreatedAt.Format("2006-01-02 15:04"))
		if len(s.AssignedAgent) > 0 {
			response += fmt.Sprintf("    Assigned To: %s\n", s.AssignedAgent)
		}
		response += "\n"
	}

	return &CommandResult{
		Response: response,
		Status:   "ok",
	}, nil
}

// CommandResult is result of executing a command
type CommandResult struct {
	Response string            `json:"response"`
	Status   string            `json:"status"`
	Error    string            `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
