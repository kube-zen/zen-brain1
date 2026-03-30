package reme

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

func TestNewProtocol_DefaultConfig(t *testing.T) {
	journal := NewMemoryJournalAdapter()
	p := NewProtocol(journal, ProtocolConfig{})

	assert.NotNil(t, p)
	assert.Equal(t, 10000, p.config.MaxEvents)
	assert.Equal(t, 30*time.Second, p.config.ReplayTimeout)
}

func TestProtocol_Reconstruct_EmptyJournal(t *testing.T) {
	journal := NewMemoryJournalAdapter()
	p := NewProtocol(journal, DefaultProtocolConfig())

	state, err := p.Reconstruct(context.Background(), "sess-1", "cluster-1", "task-1")

	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, "sess-1", state.Session.SessionID)
	assert.Equal(t, "cluster-1", state.Session.ClusterID)
	assert.Equal(t, "task-1", state.Session.TaskID)
	assert.Empty(t, state.CausalChain)
	assert.False(t, state.RecoveryNeeded)
	assert.Equal(t, 0, state.Stats.EventsReplayed)
}

func TestProtocol_Reconstruct_WithEvents(t *testing.T) {
	journal := NewMemoryJournalAdapter()
	now := time.Now()

	journal.Append(JournalEntry{
		EventType: "session_created", Timestamp: now,
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
	})
	journal.Append(JournalEntry{
		EventType: "task_started", Timestamp: now.Add(1 * time.Second),
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
	})
	journal.Append(JournalEntry{
		EventType: "thought_recorded", Timestamp: now.Add(5 * time.Second),
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
		Payload: map[string]interface{}{"content": "Need to refactor the gateway"},
	})
	journal.Append(JournalEntry{
		EventType: "decision_made", Timestamp: now.Add(10 * time.Second),
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
		Payload: map[string]interface{}{"description": "Use adapter pattern for providers"},
	})
	journal.Append(JournalEntry{
		EventType: "llm_call", Timestamp: now.Add(15 * time.Second),
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
	})
	journal.Append(JournalEntry{
		EventType: "tool_call", Timestamp: now.Add(20 * time.Second),
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
	})
	journal.Append(JournalEntry{
		EventType: "file_modified", Timestamp: now.Add(25 * time.Second),
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
	})
	journal.Append(JournalEntry{
		EventType: "tokens_used", Timestamp: now.Add(30 * time.Second),
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
		Payload: map[string]interface{}{"tokens": float64(1500)},
	})

	p := NewProtocol(journal, DefaultProtocolConfig())
	state, err := p.Reconstruct(context.Background(), "sess-1", "cluster-1", "task-1")

	require.NoError(t, err)
	assert.Equal(t, 8, state.Stats.EventsReplayed)
	assert.Equal(t, 8, state.Stats.JournalEntries)
	assert.Equal(t, 1, state.Stats.ThoughtsRecorded)
	assert.Equal(t, 1, state.Stats.DecisionsMade)
	assert.Equal(t, 1, state.Stats.LLMCalls)
	assert.Equal(t, 1, state.Stats.ToolCalls)
	assert.Equal(t, 1, state.Stats.FilesModified)
	assert.Equal(t, int64(1500), state.Stats.TokensUsed)
	assert.Len(t, state.Session.JournalEntries, 8)
	assert.NotEmpty(t, state.Session.Scratchpad)
	assert.NotEmpty(t, state.StateHash)
}

func TestProtocol_Reconstruct_RecoveryNeeded(t *testing.T) {
	journal := NewMemoryJournalAdapter()
	now := time.Now()

	// Scenario: session was created and started, then failed — recovery needed
	journal.Append(JournalEntry{
		EventType: "session_created", Timestamp: now,
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
	})
	journal.Append(JournalEntry{
		EventType: "task_started", Timestamp: now.Add(time.Second),
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
		Payload: map[string]interface{}{"phase": "execution", "step": 3},
	})
	journal.Append(JournalEntry{
		EventType: "session_failed", Timestamp: now.Add(2 * time.Second),
		SessionID: "sess-1", ClusterID: "cluster-1", TaskID: "task-1",
	})

	p := NewProtocol(journal, DefaultProtocolConfig())
	state, err := p.Reconstruct(context.Background(), "sess-1", "cluster-1", "task-1")

	require.NoError(t, err)
	// Recovery is needed when the session has events but was never completed
	assert.True(t, state.RecoveryNeeded)
	assert.Equal(t, 3, state.Stats.EventsReplayed)
}

func TestProtocol_VerifyState(t *testing.T) {
	journal := NewMemoryJournalAdapter()
	p := NewProtocol(journal, DefaultProtocolConfig())

	// Nil session
	assert.False(t, p.VerifyState(&ReMeState{Session: nil}))

	// Valid session with LastAccessedAt set (Reconstruct sets it to time.Now())
	assert.True(t, p.VerifyState(&ReMeState{
		Session: &zenctx.SessionContext{
			SessionID:      "sess-1",
			LastAccessedAt: time.Now(),
		},
		CausalChain: []JournalEntry{},
	}))

	// Session with events should have non-zero LastAccessedAt
	journal.Append(JournalEntry{
		EventType: "session_created", Timestamp: time.Now(), SessionID: "sess-1",
	})
	state, _ := p.Reconstruct(context.Background(), "sess-1", "", "")
	assert.True(t, p.VerifyState(state))
}

func TestMemoryJournalAdapter_Filters(t *testing.T) {
	journal := NewMemoryJournalAdapter()
	now := time.Now()

	journal.Append(JournalEntry{EventType: "session_created", SessionID: "sess-1", ClusterID: "c1", Timestamp: now})
	journal.Append(JournalEntry{EventType: "task_started", SessionID: "sess-2", ClusterID: "c1", Timestamp: now.Add(time.Second)})
	journal.Append(JournalEntry{EventType: "thought_recorded", SessionID: "sess-1", ClusterID: "c2", Timestamp: now.Add(2 * time.Second)})
	journal.Append(JournalEntry{EventType: "llm_call", SessionID: "sess-1", ClusterID: "c1", Timestamp: now.Add(3 * time.Second)})

	t.Run("by session", func(t *testing.T) {
		entries, err := journal.Query(context.Background(), QueryOptions{SessionID: "sess-1"})
		require.NoError(t, err)
		assert.Len(t, entries, 3)
	})

	t.Run("by cluster", func(t *testing.T) {
		entries, err := journal.Query(context.Background(), QueryOptions{ClusterID: "c2"})
		require.NoError(t, err)
		assert.Len(t, entries, 1)
	})

	t.Run("by event type", func(t *testing.T) {
		entries, err := journal.Query(context.Background(), QueryOptions{EventType: "llm_call"})
		require.NoError(t, err)
		assert.Len(t, entries, 1)
	})

	t.Run("with limit", func(t *testing.T) {
		entries, err := journal.Query(context.Background(), QueryOptions{Limit: 2})
		require.NoError(t, err)
		assert.Len(t, entries, 2)
	})

	t.Run("no matches", func(t *testing.T) {
		entries, err := journal.Query(context.Background(), QueryOptions{SessionID: "nonexistent"})
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("clear and re-query", func(t *testing.T) {
		journal.Clear()
		entries, err := journal.Query(context.Background(), QueryOptions{})
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}

func TestMemoryJournalAdapter_AutoSequence(t *testing.T) {
	journal := NewMemoryJournalAdapter()
	journal.Append(JournalEntry{EventType: "a"})
	journal.Append(JournalEntry{EventType: "b"})
	journal.Append(JournalEntry{EventType: "c"})

	entries, _ := journal.Query(context.Background(), QueryOptions{})
	require.Len(t, entries, 3)
	assert.Equal(t, int64(1), entries[0].Sequence)
	assert.Equal(t, int64(2), entries[1].Sequence)
	assert.Equal(t, int64(3), entries[2].Sequence)
}
