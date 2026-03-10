// Package context provides the composite ZenContext implementation.
// This file contains the journal adapter for the ReMe protocol.

package context

import (
	stdctx "context"
	"fmt"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/journal"
)

// JournalAdapter adapts journal.ZenJournal to the composite.Journal interface.
type JournalAdapter struct {
	journal journal.ZenJournal
	verbose bool
}

// NewJournalAdapter creates a new adapter for journal.ZenJournal.
func NewJournalAdapter(j journal.ZenJournal, verbose bool) *JournalAdapter {
	return &JournalAdapter{
		journal: j,
		verbose: verbose,
	}
}

// Query implements the composite.Journal interface.
// Accepts opts as map[string]interface{} or journal.QueryOptions.
func (a *JournalAdapter) Query(ctx stdctx.Context, opts interface{}) ([]interface{}, error) {
	// Convert opts to journal.QueryOptions
	queryOpts, err := a.convertQueryOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("invalid query options: %w", err)
	}

	if a.verbose {
		fmt.Printf("[JournalAdapter] Query: eventType=%s, taskID=%s, correlationID=%s, limit=%d\n",
			queryOpts.EventType, queryOpts.TaskID, queryOpts.CorrelationID, queryOpts.Limit)
	}

	// Call the journal
	receipts, err := a.journal.Query(ctx, queryOpts)
	if err != nil {
		return nil, fmt.Errorf("journal query failed: %w", err)
	}

	// Convert []journal.Receipt to []interface{}
	result := make([]interface{}, len(receipts))
	for i, receipt := range receipts {
		result[i] = receipt
	}

	if a.verbose {
		fmt.Printf("[JournalAdapter] Query: returned %d receipts\n", len(result))
	}

	return result, nil
}

// convertQueryOptions converts opts to journal.QueryOptions.
// Supports map[string]interface{} or journal.QueryOptions.
func (a *JournalAdapter) convertQueryOptions(opts interface{}) (journal.QueryOptions, error) {
	switch v := opts.(type) {
	case journal.QueryOptions:
		return v, nil
	case map[string]interface{}:
		return a.convertFromMap(v), nil
	default:
		return journal.QueryOptions{}, fmt.Errorf("unsupported options type: %T", opts)
	}
}

// convertFromMap converts a map to journal.QueryOptions.
func (a *JournalAdapter) convertFromMap(m map[string]interface{}) journal.QueryOptions {
	opts := journal.QueryOptions{
		Limit: 100, // default limit
	}

	// Extract values from map
	if v, ok := m["event_type"].(string); ok && v != "" {
		opts.EventType = journal.EventType(v)
	}
	if v, ok := m["event_type"].(journal.EventType); ok {
		opts.EventType = v
	}
	if v, ok := m["correlation_id"].(string); ok && v != "" {
		opts.CorrelationID = v
	}
	if v, ok := m["task_id"].(string); ok && v != "" {
		opts.TaskID = v
	}
	if v, ok := m["session_id"].(string); ok && v != "" {
		opts.SessionID = v
	}
	if v, ok := m["cluster_id"].(string); ok && v != "" {
		opts.ClusterID = v
	}
	if v, ok := m["project_id"].(string); ok && v != "" {
		opts.ProjectID = v
	}
	if v, ok := m["sred_tag"].(string); ok && v != "" {
		opts.SREDTag = contracts.SREDTag(v)
	}
	if v, ok := m["sred_tag"].(contracts.SREDTag); ok {
		opts.SREDTag = v
	}
	if v, ok := m["start"].(time.Time); ok && !v.IsZero() {
		opts.Start = v
	}
	if v, ok := m["end"].(time.Time); ok && !v.IsZero() {
		opts.End = v
	}
	if v, ok := m["limit"].(int); ok && v > 0 {
		opts.Limit = v
	}
	if v, ok := m["order_by"].(string); ok && v != "" {
		opts.OrderBy = v
	}

	return opts
}
