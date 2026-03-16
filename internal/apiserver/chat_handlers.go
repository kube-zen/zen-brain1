// Package apiserver provides REST handlers for TUI and other clients.
// All business logic (help, commands, status, reports) lives in handlers/server, not here.
package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/internal/session"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// ChatRequest represents a request to the zen-brain chat API
type ChatRequest struct {
	Input      string                 `json:"input"`                // Raw user input or command
	SessionID  string                 `json:"session_id,omitempty"`  // Optional session ID
	WorkingDir string                 `json:"working_dir,omitempty"`  // Optional working directory
	ClientID   string                 `json:"client_id,omitempty"`   // Client identity for session attachment
	ClientType string                 `json:"client_type,omitempty"` // tui, slack, http, websocket
	Options    map[string]interface{}  `json:"options,omitempty"`    // Additional options
}

// ChatResponse represents a response from the chat API
type ChatResponse struct {
	SessionID   string                 `json:"session_id"`
	Response    string                 `json:"response"`      // Plain text response (for simple clients)
	Output      string                 `json:"output"`        // Structured output (for advanced clients)
	Data        interface{}            `json:"data"`          // Arbitrary data for rendering
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   string                 `json:"timestamp"`
}

// HelpRequest represents a help request
type HelpRequest struct {
	Topic   string `json:"topic,omitempty"`   // Specific help topic
	Command string `json:"command,omitempty"` // Specific command help
}

// HelpResponse represents a help response (server-generated)
type HelpResponse struct {
	Text     string                 `json:"text"`               // Plain text help
	Commands []CommandHelpEntry    `json:"commands,omitempty"` // Command list
	Topics   []string             `json:"topics,omitempty"`   // Available topics
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CommandHelpEntry describes a command (server-defined)
type CommandHelpEntry struct {
	Name        string `json:"name"`
	Synopsis    string `json:"synopsis"`
	Description string `json:"description"`
	Usage       string `json:"usage"`
	Examples    []string `json:"examples,omitempty"`
}

// StatusRequest represents a status request
type StatusRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Detail    string `json:"detail,omitempty"` // brief, sessions, workers, full
}

// StatusResponse represents a status response (server-generated)
type StatusResponse struct {
	Status     string                 `json:"status"`       // ok, degraded, error
	Components map[string]interface{} `json:"components"` // Individual component status
	Sessions  []SessionSummary     `json:"sessions,omitempty"` // Active sessions
	Workers    []WorkerSummary     `json:"workers,omitempty"` // Worker status
	Queue      []TaskSummary      `json:"queue,omitempty"` // Queue status
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Timestamp  string                 `json:"timestamp"`
}

// SessionSummary is a minimal session view
type SessionSummary struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	WorkItemID string `json:"work_item_id,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// WorkerSummary describes a worker
type WorkerSummary struct {
	ID     string `json:"id"`
	Status string `json:"status"` // idle, busy, error
	LastActivity string `json:"last_activity,omitempty"`
}

// TaskSummary describes a task in queue
type TaskSummary struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"` // pending, running, completed, failed
	Created string `json:"created,omitempty"`
}

// ChatHandler returns an http.Handler that processes chat/commands (server owns all logic).
// TUI sends raw input, server parses and executes.
func ChatHandler(sessionMgr session.Manager, plannerAgent *planner.Planner) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// TODO: Route to server-side command processor
		// For now, return a simple response

		resp := ChatResponse{
			SessionID: req.SessionID,
			Response:  "Server received your request: " + req.Input,
			Output:    "Server received your request: " + req.Input,
			Metadata: map[string]interface{}{
				"client_type": req.ClientType,
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
}

// HelpHandler returns an http.Handler that provides help (server-generated).
func HelpHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Server-generated help text
		help := HelpResponse{
			Text: "zen-brain - Intelligent Planning and Execution System\n\n" +
				"Available commands:\n" +
				"  /help         - Show this help\n" +
				"  /status       - Show system status\n" +
				"  /sessions     - List active sessions\n" +
				"  /chat <text> - Send a message or task\n" +
				"  /exit         - Exit the interface\n\n" +
				"For detailed command help: /help <command>",
			Commands: []CommandHelpEntry{
				{
					Name:        "/help",
					Synopsis:    "Show help information",
					Description: "Display available commands or detailed help for a specific command.",
					Usage:       "/help [command]",
					Examples:    []string{"/help", "/help status"},
				},
				{
					Name:        "/status",
					Synopsis:    "Show system status",
					Description: "Display overall system health and component status.",
					Usage:       "/status [brief|sessions|workers|full]",
				},
				{
					Name:        "/sessions",
					Synopsis:    "List active sessions",
					Description: "List all active sessions with their current state.",
					Usage:       "/sessions [--state=<state>] [--limit=<n>]",
				},
				{
					Name:        "/chat",
					Synopsis:    "Send message or task",
					Description: "Send a message to the assistant or submit a task.",
					Usage:       "/chat <text>",
					Examples:    []string{"/chat analyze this ticket", "/chat what is the status?"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(help)
	})
}

// StatusHandler returns an http.Handler that provides status (server-generated).
func StatusHandler(sessionMgr session.Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Server-generated status
		status := StatusResponse{
			Status: "ok",
			Components: map[string]interface{}{
				"session_manager": "available",
				"planner":        "available",
				"factory":        "available",
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// Add sessions if available
		if sessionMgr != nil {
			if list, err := sessionMgr.ListSessions(r.Context(), session.SessionFilter{Limit: 5}); err == nil {
				summaries := make([]SessionSummary, 0, len(list))
				for _, s := range list {
					summaries = append(summaries, SessionSummary{
						ID:         s.ID,
						State:      string(s.State),
						WorkItemID: s.WorkItemID,
						CreatedAt:  s.CreatedAt.Format(time.RFC3339),
					})
				}
				status.Sessions = summaries
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})
}
