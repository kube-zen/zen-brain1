// Package reme implements the ReMe (Recursive Memory) protocol for zen-brain1.
// Agents use ReMe to reconstruct their state from ZenJournal history after
// restarts, failures, or scheduled work.
//
// This is an adaptation of the zen-brain 0.1 reme package, rewritten to use
// zen-brain1's canonical types (pkg/context.SessionContext, pkg/journal).
package reme

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// Journal is the interface ReMe uses to query ZenJournal for event history.
// Implemented by the real ZenJournal (Block 3.3) or by adapters for testing.
type Journal interface {
	// Query retrieves journal entries matching the given filter options.
	Query(ctx context.Context, opts QueryOptions) ([]JournalEntry, error)
}

// QueryOptions specifies filters for journal queries.
type QueryOptions struct {
	SessionID string
	ClusterID string
	TaskID    string
	EventType string
	Limit     int
	// UpToTime filters entries before this timestamp (inclusive).
	UpToTime time.Time
}

// JournalEntry represents a single entry from ZenJournal.
type JournalEntry struct {
	Sequence      int64                  `json:"sequence"`
	EventType     string                 `json:"event_type"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	TaskID        string                 `json:"task_id,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	ClusterID     string                 `json:"cluster_id,omitempty"`
	Payload       map[string]interface{} `json:"payload,omitempty"`
}

// ReMeState contains the fully reconstructed agent state.
type ReMeState struct {
	Session        *zenctx.SessionContext `json:"session"`
	CausalChain    []JournalEntry         `json:"causal_chain"`
	VerifiedAt     time.Time              `json:"verified_at"`
	StateHash      string                 `json:"state_hash"`
	RecoveryNeeded bool                   `json:"recovery_needed"`
	Stats          ReMeStats              `json:"stats"`
}

// ReMeStats holds reconstruction statistics.
type ReMeStats struct {
	EventsReplayed   int   `json:"events_replayed"`
	JournalEntries   int   `json:"journal_entries"`
	ThoughtsRecorded int   `json:"thoughts_recorded"`
	DecisionsMade    int   `json:"decisions_made"`
	LLMCalls         int   `json:"llm_calls"`
	ToolCalls        int   `json:"tool_calls"`
	FilesModified    int   `json:"files_modified"`
	TokensUsed       int64 `json:"tokens_used"`
}

// ProtocolConfig configures the ReMe protocol.
type ProtocolConfig struct {
	// MaxEvents is the maximum events to replay per reconstruction (default: 10000).
	MaxEvents int
	// ReplayTimeout is the timeout for replay operations (default: 30s).
	ReplayTimeout time.Duration
	// Verbose enables verbose logging.
	Verbose bool
}

// DefaultProtocolConfig returns sensible defaults.
func DefaultProtocolConfig() ProtocolConfig {
	return ProtocolConfig{
		MaxEvents:     10000,
		ReplayTimeout: 30 * time.Second,
		Verbose:       false,
	}
}

// Protocol implements the ReMe protocol for state reconstruction.
type Protocol struct {
	journal Journal
	config  ProtocolConfig
	verbose bool
	mu      sync.RWMutex
}

// NewProtocol creates a new ReMe protocol handler.
func NewProtocol(journal Journal, config ProtocolConfig) *Protocol {
	if config.MaxEvents == 0 {
		config.MaxEvents = 10000
	}
	if config.ReplayTimeout == 0 {
		config.ReplayTimeout = 30 * time.Second
	}

	return &Protocol{
		journal: journal,
		config:  config,
		verbose: config.Verbose,
	}
}

// Reconstruct reconstructs agent state from journal history.
//
// Steps:
//  1. Query ZenJournal for causal chain of events
//  2. Replay events to rebuild session state
//  3. Verify state consistency
//  4. Return reconstructed state with recovery flag
func (p *Protocol) Reconstruct(ctx context.Context, sessionID, clusterID, taskID string) (*ReMeState, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.ReplayTimeout)
	defer cancel()

	if p.verbose {
		log.Printf("[ReMe] Starting reconstruction for session=%s cluster=%s task=%s",
			sessionID, clusterID, taskID)
	}

	// Step 1: Query journal for causal chain
	opts := QueryOptions{
		SessionID: sessionID,
		ClusterID: clusterID,
		TaskID:    taskID,
		Limit:     p.config.MaxEvents,
	}

	entries, err := p.journal.Query(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("query journal: %w", err)
	}

	if p.verbose {
		log.Printf("[ReMe] Retrieved %d journal entries", len(entries))
	}

	// Step 2: Build session and replay events
	session := &zenctx.SessionContext{
		SessionID:      sessionID,
		ClusterID:      clusterID,
		TaskID:         taskID,
		CreatedAt:      time.Now(),
		LastAccessedAt: time.Now(),
	}

	stats := ReMeStats{
		JournalEntries: len(entries),
	}

	for _, entry := range entries {
		p.applyEvent(session, &stats, entry)
	}

	// Step 3: Compute state hash
	stateHash := p.computeStateHash(session, &stats)

	// Step 4: Determine if recovery is needed.
	// Recovery is needed when there are journal entries but no completion event
	// (session was mid-task when interrupted).
	recoveryNeeded := p.needsRecovery(entries)

	// Attach causal chain to session for agent consumption
	session.JournalEntries = make([]interface{}, len(entries))
	for i, e := range entries {
		session.JournalEntries[i] = e
	}

	state := &ReMeState{
		Session:        session,
		CausalChain:    entries,
		VerifiedAt:     time.Now(),
		StateHash:      stateHash,
		RecoveryNeeded: recoveryNeeded,
		Stats:          stats,
	}

	if p.verbose {
		log.Printf("[ReMe] Reconstructed: %d events, hash=%s, recovery=%v",
			stats.EventsReplayed, stateHash, recoveryNeeded)
	}

	return state, nil
}

// applyEvent applies a single journal event to the session state, updating stats.
func (p *Protocol) applyEvent(session *zenctx.SessionContext, stats *ReMeStats, entry JournalEntry) {
	stats.EventsReplayed++

	switch entry.EventType {
	case "session_created":
		if !entry.Timestamp.IsZero() {
			session.CreatedAt = entry.Timestamp
		}

	case "session_started", "task_started":
		session.LastAccessedAt = entry.Timestamp

	case "session_paused":
		session.LastAccessedAt = entry.Timestamp

	case "session_completed":
		session.LastAccessedAt = entry.Timestamp

	case "session_failed":
		session.LastAccessedAt = entry.Timestamp

	case "thought_recorded":
		stats.ThoughtsRecorded++
		// Store thought as scratchpad annotation
		if content, ok := entry.Payload["content"].(string); ok {
			annotation := fmt.Sprintf("[thought@%s] %s", entry.Timestamp.Format(time.RFC3339), content)
			session.Scratchpad = append(session.Scratchpad, []byte(annotation)...)
			if len(session.Scratchpad) > 0 && session.Scratchpad[len(session.Scratchpad)-1] != '\n' {
				session.Scratchpad = append(session.Scratchpad, '\n')
			}
		}

	case "decision_made":
		stats.DecisionsMade++
		if desc, ok := entry.Payload["description"].(string); ok {
			annotation := fmt.Sprintf("[decision@%s] %s", entry.Timestamp.Format(time.RFC3339), desc)
			session.Scratchpad = append(session.Scratchpad, []byte(annotation)...)
			if len(session.Scratchpad) > 0 && session.Scratchpad[len(session.Scratchpad)-1] != '\n' {
				session.Scratchpad = append(session.Scratchpad, '\n')
			}
		}

	case "llm_call":
		stats.LLMCalls++

	case "tool_call":
		stats.ToolCalls++

	case "file_modified":
		stats.FilesModified++

	case "tokens_used":
		if tokens, ok := entry.Payload["tokens"].(float64); ok {
			stats.TokensUsed += int64(tokens)
		}
	}

	session.LastAccessedAt = entry.Timestamp
}

// computeStateHash produces a deterministic hash of the reconstructed state.
func (p *Protocol) computeStateHash(session *zenctx.SessionContext, stats *ReMeStats) string {
	data := struct {
		SessionID      string
		TaskID         string
		HasState       bool
		EventsReplayed int
		TokensUsed     int64
		Thoughts       int
		Decisions      int
		LLMCalls       int
		ToolCalls      int
	}{
		SessionID:      session.SessionID,
		TaskID:         session.TaskID,
		HasState:       len(session.State) > 0,
		EventsReplayed: stats.EventsReplayed,
		TokensUsed:     stats.TokensUsed,
		Thoughts:       stats.ThoughtsRecorded,
		Decisions:      stats.DecisionsMade,
		LLMCalls:       stats.LLMCalls,
		ToolCalls:      stats.ToolCalls,
	}

	b, _ := json.Marshal(data)
	if len(b) > 16 {
		b = b[:16]
	}
	return fmt.Sprintf("%x", b)
}

// VerifyState performs basic consistency checks on a reconstructed session.
func (p *Protocol) VerifyState(state *ReMeState) bool {
	if state.Session == nil {
		return false
	}

	// If there are causal events, session should have been accessed
	if len(state.CausalChain) > 0 && state.Session.LastAccessedAt.IsZero() {
		return false
	}

	return true
}

// needsRecovery checks whether the journal entries indicate an interrupted session
// that needs recovery (i.e., events exist but no completion event).
func (p *Protocol) needsRecovery(entries []JournalEntry) bool {
	if len(entries) == 0 {
		return false
	}

	// Check if the last entry is a terminal event
	for _, e := range entries {
		switch e.EventType {
		case "session_completed", "session_completed_successfully":
			return false // Clean shutdown, no recovery needed
		}
	}

	// If there are events but no completion, recovery is needed
	return true
}
