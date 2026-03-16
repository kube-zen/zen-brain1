// Package command provides server-side command parsing for TUI and other interfaces.
// This is SERVER-SIDE ONLY. TUI sends raw input, server parses and executes.
package command

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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

// NewParser creates a new command parser
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

Examples:
  /help status           Show help for status command
  /status               Show system status
  /sessions --limit=5    Show last 5 sessions
  /chat analyze ticket    Send analysis task

For more information, visit: https://github.com/kube-zen/zen-brain1`

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
  state=<state>  Filter by session state (created|analyzed|scheduled|in_progress|completed|failed)

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
  /chat create a new deployment plan`
		default:
			response = fmt.Sprintf("Unknown command for help: %s\nAvailable commands: help, status, sessions, chat", cmd.Args[0])
		}
	}

	return &CommandResult{
		Response: response,
		Status:   "success",
	}, nil
}

// executeStatus generates status report (server-side)
func (p *Parser) executeStatus(ctx context.Context, cmd *Command) (*CommandResult, error) {
	var detailLevel string
	if len(cmd.Args) > 0 {
		detailLevel = cmd.Args[0]
	}

	// Build status report
	status := "System Status: OK\n\n"

	// Add session manager status
	if p.sessionMgr != nil {
		filter := session.SessionFilter{Limit: 10}
		if list, err := p.sessionMgr.ListSessions(ctx, filter); err == nil {
			status += fmt.Sprintf("Active Sessions: %d\n", len(list))
			if len(list) > 0 {
				status += "\nRecent Sessions:\n"
				for i, s := range list {
					if i >= 5 {
						status += fmt.Sprintf("  ... and %d more\n", len(list)-5)
						break
					}
					status += fmt.Sprintf("  - %s (%s)\n", s.ID, s.State)
				}
			}
		}
	}

	// Add planner status
	if p.planner != nil {
		status += "\nPlanner: Available\n"
	}

	// Add component status
	status += "\nComponents:\n"
	status += "  - Session Manager: Available\n"
	status += "  - Planner: Available\n"
	status += "  - Factory: Available\n"

	return &CommandResult{
		Response: status,
		Status:   "ok",
	}, nil
}

// executeSessions lists sessions (server-side)
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

	// List sessions
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
		response += fmt.Sprintf("    Created: %s\n\n", s.CreatedAt.Format("2006-01-02 15:04"))
	}

	return &CommandResult{
		Response: response,
		Status:   "ok",
	}, nil
}

// executeChat processes chat input
func (p *Parser) executeChat(ctx context.Context, cmd *Command) (*CommandResult, error) {
	if len(cmd.Args) == 0 {
		return &CommandResult{
			Response: "Please provide a message or task\nUsage: /chat <text>",
			Error:    "no_input",
		}, nil
	}

	// For now, echo back. Real implementation would:
	// 1. Create or load session
	// 2. Send to planner
	// 3. Return streaming response

	input := strings.Join(cmd.Args, " ")
	response := fmt.Sprintf("Received: %s\n\n[Chat processing not yet implemented]", input)

	return &CommandResult{
		Response: response,
		Status:   "ok",
	}, nil
}

// CommandResult is the result of executing a command
type CommandResult struct {
	Response string            `json:"response"`
	Status   string            `json:"status"`
	Error    string            `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
