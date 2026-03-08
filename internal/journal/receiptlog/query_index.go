package receiptlog

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/journal"
)

// QueryIndex provides in-memory indexing for efficient queries.
// Supports O(1) lookups by EventType, CorrelationID, TaskID, etc.
type QueryIndex struct {
	// ByEventType maps event type to sequences
	ByEventType map[journal.EventType][]uint64

	// ByCorrelationID maps correlation ID to sequences
	ByCorrelationID map[string][]uint64

	// ByTaskID maps task ID to sequences
	ByTaskID map[string][]uint64

	// BySessionID maps session ID to sequences
	BySessionID map[string][]uint64

	// BySREDTag maps SR&ED tags to sequences
	BySREDTag map[contracts.SREDTag][]uint64

	// ByTimestamp is a sorted slice for time range queries
	ByTimestamp []TimeRangeEntry

	// ByClusterID maps cluster ID to sequences
	ByClusterID map[string][]uint64

	// ByProjectID maps project ID to sequences
	ByProjectID map[string][]uint64

	mu sync.RWMutex
}

// TimeRangeEntry stores sequence and timestamp for time range queries.
type TimeRangeEntry struct {
	Sequence  uint64
	Timestamp time.Time
	EventType journal.EventType
}

// NewQueryIndex creates a new empty query index.
func NewQueryIndex() *QueryIndex {
	return &QueryIndex{
		ByEventType:      make(map[journal.EventType][]uint64),
		ByCorrelationID:  make(map[string][]uint64),
		ByTaskID:        make(map[string][]uint64),
		BySessionID:      make(map[string][]uint64),
		BySREDTag:        make(map[contracts.SREDTag][]uint64),
		ByTimestamp:      make([]TimeRangeEntry, 0),
		ByClusterID:      make(map[string][]uint64),
		ByProjectID:      make(map[string][]uint64),
	}
}

// Add indexes a receipt for querying.
func (idx *QueryIndex) Add(receipt *journal.Receipt) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Add to event type index
	idx.ByEventType[receipt.EventType] = append(idx.ByEventType[receipt.EventType], receipt.Sequence)

	// Add to correlation ID index
	if receipt.CorrelationID != "" {
		idx.ByCorrelationID[receipt.CorrelationID] = append(idx.ByCorrelationID[receipt.CorrelationID], receipt.Sequence)
	}

	// Add to task ID index
	if receipt.TaskID != "" {
		idx.ByTaskID[receipt.TaskID] = append(idx.ByTaskID[receipt.TaskID], receipt.Sequence)
	}

	// Add to session ID index
	if receipt.SessionID != "" {
		idx.BySessionID[receipt.SessionID] = append(idx.BySessionID[receipt.SessionID], receipt.Sequence)
	}

	// Add to cluster ID index
	if receipt.ClusterID != "" {
		idx.ByClusterID[receipt.ClusterID] = append(idx.ByClusterID[receipt.ClusterID], receipt.Sequence)
	}

	// Add to project ID index
	if receipt.ProjectID != "" {
		idx.ByProjectID[receipt.ProjectID] = append(idx.ByProjectID[receipt.ProjectID], receipt.Sequence)
	}

	// Add to SR&ED tag index
	for _, tag := range receipt.SREDTags {
		idx.BySREDTag[tag] = append(idx.BySREDTag[tag], receipt.Sequence)
	}

	// Add to timestamp index (insert sorted)
	idx.insertSorted(TimeRangeEntry{
		Sequence:  receipt.Sequence,
		Timestamp: receipt.Timestamp,
		EventType: receipt.EventType,
	})
}

// Remove removes a receipt from the index (used for compaction).
func (idx *QueryIndex) Remove(sequence uint64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove from timestamp index
	for i, entry := range idx.ByTimestamp {
		if entry.Sequence == sequence {
			idx.ByTimestamp = append(idx.ByTimestamp[:i], idx.ByTimestamp[i+1:]...)
			break
		}
	}

	// For simplicity, we don't remove from other indexes in this initial implementation.
	// In production, we'd maintain reverse mappings or rebuild indexes periodically.
}

// Query executes a query and returns matching sequences.
func (idx *QueryIndex) Query(opts journal.QueryOptions) []uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Start with all sequences, then filter
	var sequences []uint64

	if opts.EventType != "" {
		sequences = idx.filterSequences(sequences, idx.ByEventType[opts.EventType])
	}

	if opts.CorrelationID != "" {
		sequences = idx.filterSequences(sequences, idx.ByCorrelationID[opts.CorrelationID])
	}

	if opts.TaskID != "" {
		sequences = idx.filterSequences(sequences, idx.ByTaskID[opts.TaskID])
	}

	if opts.SessionID != "" {
		sequences = idx.filterSequences(sequences, idx.BySessionID[opts.SessionID])
	}

	if opts.ClusterID != "" {
		sequences = idx.filterSequences(sequences, idx.ByClusterID[opts.ClusterID])
	}

	if opts.ProjectID != "" {
		sequences = idx.filterSequences(sequences, idx.ByProjectID[opts.ProjectID])
	}

	if opts.SREDTag != "" {
		sequences = idx.filterSequences(sequences, idx.BySREDTag[opts.SREDTag])
	}

	// Time range filtering (O(log n) binary search)
	if !opts.Start.IsZero() || !opts.End.IsZero() {
		sequences = idx.filterByTime(sequences, opts.Start, opts.End)
	}

	// Apply limit
	if opts.Limit > 0 && len(sequences) > opts.Limit {
		sequences = sequences[:opts.Limit]
	}

	// Apply ordering (default: desc by sequence/timestamp)
	if opts.OrderBy == "asc" {
		// Sequences are already in ascending order by time
		// Keep as-is
	} else {
		// Default to descending
		sort.Slice(sequences, func(i, j int) bool {
			return sequences[i] > sequences[j]
		})
	}

	return sequences
}

// QueryByEventType returns sequences for a specific event type.
func (idx *QueryIndex) QueryByEventType(eventType journal.EventType) []uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	sequences, exists := idx.ByEventType[eventType]
	if !exists {
		return nil
	}
	return append([]uint64{}, sequences...) // Return copy
}

// QueryByCorrelationID returns sequences for a correlation ID.
func (idx *QueryIndex) QueryByCorrelationID(correlationID string) []uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	sequences, exists := idx.ByCorrelationID[correlationID]
	if !exists {
		return nil
	}
	return append([]uint64{}, sequences...) // Return copy
}

// QueryByTaskID returns sequences for a task ID.
func (idx *QueryIndex) QueryByTaskID(taskID string) []uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	sequences, exists := idx.ByTaskID[taskID]
	if !exists {
		return nil
	}
	return append([]uint64{}, sequences...) // Return copy
}

// QueryBySREDTag returns sequences for an SR&ED tag.
func (idx *QueryIndex) QueryBySREDTag(tag contracts.SREDTag) []uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	sequences, exists := idx.BySREDTag[tag]
	if !exists {
		return nil
	}
	return append([]uint64{}, sequences...) // Return copy
}

// filterByTime filters sequences by time range.
func (idx *QueryIndex) filterByTime(sequences []uint64, start, end time.Time) []uint64 {
	if len(sequences) == 0 {
		// Use full timestamp index
		sequences = idx.getTimeRangeSequences(start, end)
	}

	// Filter sequences by timestamp (need to get receipts for timestamps)
	// This is O(n) for now; optimization: cache timestamps per sequence
	var result []uint64
	for _, seq := range sequences {
		// Binary search in ByTimestamp
		entry := idx.findBySequence(seq)
		if entry == nil {
			continue
		}

		if start.IsZero() || entry.Timestamp.After(start) || entry.Timestamp.Equal(start) {
			if end.IsZero() || entry.Timestamp.Before(end) {
				result = append(result, seq)
			}
		}
	}

	return result
}

// filterSequences returns intersection of two sequences slices.
func (idx *QueryIndex) filterSequences(base, filter []uint64) []uint64 {
	if base == nil {
		// No base yet, use filter as starting point
		return append([]uint64{}, filter...)
	}

	// Compute intersection using a map for O(n+m) complexity
	filterSet := make(map[uint64]bool)
	for _, seq := range filter {
		filterSet[seq] = true
	}

	var result []uint64
	for _, seq := range base {
		if filterSet[seq] {
			result = append(result, seq)
		}
	}

	return result
}

// insertSorted inserts a TimeRangeEntry while maintaining sorted order.
func (idx *QueryIndex) insertSorted(entry TimeRangeEntry) {
	// Binary search for insertion point
	i := sort.Search(len(idx.ByTimestamp), func(i int) bool {
		return entry.Timestamp.Before(idx.ByTimestamp[i].Timestamp)
	})

	// Insert at position i
	idx.ByTimestamp = append(idx.ByTimestamp, TimeRangeEntry{})
	copy(idx.ByTimestamp[i+1:], idx.ByTimestamp[i:])
	idx.ByTimestamp[i] = entry
}

// findBySequence finds a TimeRangeEntry by sequence.
func (idx *QueryIndex) findBySequence(sequence uint64) *TimeRangeEntry {
	for i := range idx.ByTimestamp {
		if idx.ByTimestamp[i].Sequence == sequence {
			return &idx.ByTimestamp[i]
		}
	}
	return nil
}

// getTimeRangeSequences returns sequences in time range using binary search.
func (idx *QueryIndex) getTimeRangeSequences(start, end time.Time) []uint64 {
	var result []uint64

	// Find start index
	startIdx := sort.Search(len(idx.ByTimestamp), func(i int) bool {
		return idx.ByTimestamp[i].Timestamp.After(start) ||
			idx.ByTimestamp[i].Timestamp.Equal(start)
	})

	// Find end index
	endIdx := len(idx.ByTimestamp)
	if !end.IsZero() {
		endIdx = sort.Search(len(idx.ByTimestamp), func(i int) bool {
			return idx.ByTimestamp[i].Timestamp.After(end)
		})
	}

	// Extract sequences
	for i := startIdx; i < endIdx; i++ {
		result = append(result, idx.ByTimestamp[i].Sequence)
	}

	return result
}

// Stats returns index statistics.
func (idx *QueryIndex) Stats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return IndexStats{
		TotalEntries:    len(idx.ByTimestamp),
		TotalEventTypes:  len(idx.ByEventType),
		TotalCorrelations: len(idx.ByCorrelationID),
		TotalTasks:      len(idx.ByTaskID),
		TotalSessions:    len(idx.BySessionID),
		TotalClusters:    len(idx.ByClusterID),
		TotalProjects:    len(idx.ByProjectID),
		TotalSREDTags:    len(idx.BySREDTag),
	}
}

// IndexStats holds query index statistics.
type IndexStats struct {
	TotalEntries     int `json:"total_entries"`
	TotalEventTypes  int `json:"total_event_types"`
	TotalCorrelations int `json:"total_correlations"`
	TotalTasks       int `json:"total_tasks"`
	TotalSessions    int `json:"total_sessions"`
	TotalClusters    int `json:"total_clusters"`
	TotalProjects    int `json:"total_projects"`
	TotalSREDTags    int `json:"total_sred_tags"`
}

// FetchReceipts fetches receipts by sequences using a fetch function.
// This is used by QueryIndex to retrieve actual receipts after finding sequences.
func FetchReceipts(ctx context.Context, sequences []uint64, fetchFunc func(context.Context, uint64) (*journal.Receipt, error)) ([]journal.Receipt, error) {
	receipts := make([]journal.Receipt, 0, len(sequences))

	for _, seq := range sequences {
		receipt, err := fetchFunc(ctx, seq)
		if err != nil {
			// Log warning but continue
			continue
		}
		if receipt != nil {
			receipts = append(receipts, *receipt)
		}
	}

	if len(receipts) == 0 {
		return nil, fmt.Errorf("no receipts found for sequences")
	}

	return receipts, nil
}
