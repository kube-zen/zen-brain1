package reme

import (
	"context"
	"fmt"
	"sync"
)

// MemoryJournalAdapter is an in-memory journal for testing and development.
// It implements the Journal interface without requiring real infrastructure.
type MemoryJournalAdapter struct {
	entries []JournalEntry
	mu      sync.RWMutex
}

// NewMemoryJournalAdapter creates a new in-memory journal.
func NewMemoryJournalAdapter() *MemoryJournalAdapter {
	return &MemoryJournalAdapter{
		entries: make([]JournalEntry, 0),
	}
}

// Append adds an entry to the in-memory journal.
func (a *MemoryJournalAdapter) Append(entry JournalEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry.Sequence = int64(len(a.entries) + 1)
	a.entries = append(a.entries, entry)
}

// Query retrieves journal entries matching the given filters.
func (a *MemoryJournalAdapter) Query(_ context.Context, opts QueryOptions) ([]JournalEntry, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	limit := opts.Limit
	if limit <= 0 {
		limit = 1000
	}

	results := make([]JournalEntry, 0, limit)
	for _, entry := range a.entries {
		if opts.SessionID != "" && entry.SessionID != opts.SessionID {
			continue
		}
		if opts.ClusterID != "" && entry.ClusterID != opts.ClusterID {
			continue
		}
		if opts.TaskID != "" && entry.TaskID != opts.TaskID {
			continue
		}
		if opts.EventType != "" && entry.EventType != opts.EventType {
			continue
		}
		if !opts.UpToTime.IsZero() && entry.Timestamp.After(opts.UpToTime) {
			continue
		}

		results = append(results, entry)
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// Clear removes all entries.
func (a *MemoryJournalAdapter) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = a.entries[:0]
}

// Len returns the number of entries.
func (a *MemoryJournalAdapter) Len() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.entries)
}

// ReceiptLogAdapter adapts zen-sdk receiptlog to the ReMe Journal interface.
// Reads NDJSON spool files from a directory.
type ReceiptLogAdapter struct {
	spoolDir string
	verbose  bool
}

// NewReceiptLogAdapter creates a new receipt log adapter.
func NewReceiptLogAdapter(spoolDir string, verbose bool) *ReceiptLogAdapter {
	return &ReceiptLogAdapter{
		spoolDir: spoolDir,
		verbose:  verbose,
	}
}

// Query reads receipt log files and filters entries.
// NOTE: This is a minimal adapter. The real ZenJournal (Block 3.3) should
// be used in production. This adapter is for development and testing.
func (a *ReceiptLogAdapter) Query(_ context.Context, opts QueryOptions) ([]JournalEntry, error) {
	if a.spoolDir == "" {
		return nil, fmt.Errorf("spool directory not configured")
	}

	// For now, return empty — the real implementation will read NDJSON spool files.
	// This adapter will be completed when Block 3.3 (ZenJournal) is wired.
	return nil, nil
}
