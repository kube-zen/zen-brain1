// Package context tests the journal adapter for the ReMe protocol.
package context

import (
	stdctx "context"
	"errors"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/journal"
)

// mockJournalForAdapter is a minimal ZenJournal implementation for adapter testing.
type mockJournalForAdapter struct {
	queryFn func(stdctx.Context, journal.QueryOptions) ([]journal.Receipt, error)
}

func (m *mockJournalForAdapter) Query(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, opts)
	}
	return []journal.Receipt{
		{
			Entry: journal.Entry{
				EventType:    journal.EventSessionCreated,
				CorrelationID: "corr-1",
				TaskID:       "task-1",
			},
			Sequence: 1,
		},
	}, nil
}

func (m *mockJournalForAdapter) Record(ctx stdctx.Context, entry journal.Entry) (*journal.Receipt, error) {
	return nil, nil
}

func (m *mockJournalForAdapter) Get(ctx stdctx.Context, sequence uint64) (*journal.Receipt, error) {
	return nil, nil
}

func (m *mockJournalForAdapter) GetByHash(ctx stdctx.Context, hash string) (*journal.Receipt, error) {
	return nil, nil
}

func (m *mockJournalForAdapter) QueryByCorrelation(ctx stdctx.Context, correlationID string) ([]journal.Receipt, error) {
	return nil, nil
}

func (m *mockJournalForAdapter) QueryByTask(ctx stdctx.Context, taskID string) ([]journal.Receipt, error) {
	return nil, nil
}

func (m *mockJournalForAdapter) QueryBySREDTag(ctx stdctx.Context, tag contracts.SREDTag, start, end time.Time) ([]journal.Receipt, error) {
	return nil, nil
}

func (m *mockJournalForAdapter) Verify(ctx stdctx.Context) (int, error) {
	return 0, nil
}

func (m *mockJournalForAdapter) Stats() journal.Stats {
	return journal.Stats{}
}

func (m *mockJournalForAdapter) Close() error {
	return nil
}

func TestNewJournalAdapter(t *testing.T) {
	mockJ := &mockJournalForAdapter{}
	adapter := NewJournalAdapter(mockJ, true)

	if adapter == nil {
		t.Error("NewJournalAdapter returned nil")
	}
	if !adapter.verbose {
		t.Error("NewJournalAdapter did not set verbose flag")
	}
}

func TestJournalAdapter_QueryWithQueryOptions(t *testing.T) {
	mockJ := &mockJournalForAdapter{}
	adapter := NewJournalAdapter(mockJ, false)

	opts := journal.QueryOptions{
		EventType:    journal.EventSessionCreated,
		TaskID:       "task-1",
		CorrelationID: "corr-1",
		Limit:        10,
	}

	results, err := adapter.Query(stdctx.Background(), opts)
	if err != nil {
		t.Errorf("Query(QueryOptions) = %v", err)
	}
	if results == nil {
		t.Error("Query(QueryOptions) returned nil results")
	}
}

func TestJournalAdapter_QueryWithMap(t *testing.T) {
	mockJ := &mockJournalForAdapter{}
	adapter := NewJournalAdapter(mockJ, false)

	mapOpts := map[string]interface{}{
		"event_type":    journal.EventSessionTransitioned,
		"task_id":       "task-2",
		"correlation_id": "corr-2",
		"limit":         5,
		"session_id":    "session-1",
		"cluster_id":    "cluster-1",
		"project_id":    "project-1",
		"sred_tag":      contracts.SREDU1DynamicProvisioning,
	}

	results, err := adapter.Query(stdctx.Background(), mapOpts)
	if err != nil {
		t.Errorf("Query(map) = %v", err)
	}
	if results == nil {
		t.Error("Query(map) returned nil results")
	}
}

func TestJournalAdapter_QueryConvertsEventType(t *testing.T) {
	mockJ := &mockJournalForAdapter{
		queryFn: func(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
			if opts.EventType != journal.EventSessionCreated {
				t.Errorf("EventType = %v, want %v", opts.EventType, journal.EventSessionCreated)
			}
			return []journal.Receipt{}, nil
		},
	}
	adapter := NewJournalAdapter(mockJ, false)

	mapOpts := map[string]interface{}{
		"event_type": journal.EventSessionCreated,
	}

	_, err := adapter.Query(stdctx.Background(), mapOpts)
	if err != nil {
		t.Errorf("Query(event_type string) = %v", err)
	}
}

func TestJournalAdapter_QueryWithStringEventType(t *testing.T) {
	mockJ := &mockJournalForAdapter{
		queryFn: func(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
			if opts.EventType != journal.EventSessionTransitioned {
				t.Errorf("EventType = %v, want %v", opts.EventType, journal.EventSessionTransitioned)
			}
			return []journal.Receipt{}, nil
		},
	}
	adapter := NewJournalAdapter(mockJ, false)

	mapOpts := map[string]interface{}{
		"event_type": string(journal.EventSessionTransitioned),
	}

	_, err := adapter.Query(stdctx.Background(), mapOpts)
	if err != nil {
		t.Errorf("Query(event_type string) = %v", err)
	}
}

func TestJournalAdapter_QueryInvalidOptsType(t *testing.T) {
	mockJ := &mockJournalForAdapter{}
	adapter := NewJournalAdapter(mockJ, false)

	_, err := adapter.Query(stdctx.Background(), "invalid-type")
	if err == nil {
		t.Error("Query(invalid type) expected error")
	}
}

func TestJournalAdapter_QueryReturnsReceiptsAsInterface(t *testing.T) {
	mockJ := &mockJournalForAdapter{
		queryFn: func(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
			return []journal.Receipt{
				{
					Entry: journal.Entry{
						EventType: journal.EventSessionCreated,
						CorrelationID: "corr-1",
						TaskID: "task-1",
					},
					Sequence: 1,
				},
				{
					Entry: journal.Entry{
						EventType: journal.EventSessionTransitioned,
						CorrelationID: "corr-1",
						TaskID: "task-2",
					},
					Sequence: 2,
				},
			}, nil
		},
	}
	adapter := NewJournalAdapter(mockJ, false)

	results, err := adapter.Query(stdctx.Background(), journal.QueryOptions{})
	if err != nil {
		t.Errorf("Query = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query returned %d results, want 2", len(results))
	}

	// Verify results are journal.Receipt values
	if r, ok := results[0].(journal.Receipt); ok {
		if r.Sequence != 1 {
			t.Errorf("Result[0].Sequence = %v, want 1", r.Sequence)
		}
	} else {
		t.Error("Result[0] is not a journal.Receipt")
	}
}

func TestJournalAdapter_QueryWithTimeRange(t *testing.T) {
	mockJ := &mockJournalForAdapter{
		queryFn: func(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
			if !opts.Start.Equal(start) {
				t.Errorf("Start = %v, want %v", opts.Start, start)
			}
			if !opts.End.Equal(end) {
				t.Errorf("End = %v, want %v", opts.End, end)
			}
			return []journal.Receipt{}, nil
		},
	}
	adapter := NewJournalAdapter(mockJ, false)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)

	mapOpts := map[string]interface{}{
		"start": start,
		"end":   end,
	}

	_, err := adapter.Query(stdctx.Background(), mapOpts)
	if err != nil {
		t.Errorf("Query(time range) = %v", err)
	}
}

func TestJournalAdapter_QueryJournalError(t *testing.T) {
	expectedErr := errors.New("journal query failed")
	mockJ := &mockJournalForAdapter{
		queryFn: func(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
			return nil, expectedErr
		},
	}
	adapter := NewJournalAdapter(mockJ, false)

	_, err := adapter.Query(stdctx.Background(), journal.QueryOptions{})
	if err == nil {
		t.Error("Query(journal error) expected error")
	}
}

func TestJournalAdapter_QueryDefaultLimit(t *testing.T) {
	mockJ := &mockJournalForAdapter{
		queryFn: func(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
			if opts.Limit != 100 {
				t.Errorf("Limit = %d, want 100 (default)", opts.Limit)
			}
			return []journal.Receipt{}, nil
		},
	}
	adapter := NewJournalAdapter(mockJ, false)

	_, err := adapter.Query(stdctx.Background(), map[string]interface{}{})
	if err != nil {
		t.Errorf("Query(empty map) = %v", err)
	}
}

func TestJournalAdapter_QueryWithNegativeLimit(t *testing.T) {
	mockJ := &mockJournalForAdapter{
		queryFn: func(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
			// Negative limit should be ignored (default 100 used)
			if opts.Limit != 100 {
				t.Errorf("Limit = %d, want 100 (negative ignored)", opts.Limit)
			}
			return []journal.Receipt{}, nil
		},
	}
	adapter := NewJournalAdapter(mockJ, false)

	mapOpts := map[string]interface{}{
		"limit": -1,
	}

	_, err := adapter.Query(stdctx.Background(), mapOpts)
	if err != nil {
		t.Errorf("Query(negative limit) = %v", err)
	}
}

func TestJournalAdapter_QueryWithOrderBy(t *testing.T) {
	mockJ := &mockJournalForAdapter{
		queryFn: func(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
			if opts.OrderBy != "timestamp" {
				t.Errorf("OrderBy = %v, want timestamp", opts.OrderBy)
			}
			return []journal.Receipt{}, nil
		},
	}
	adapter := NewJournalAdapter(mockJ, false)

	mapOpts := map[string]interface{}{
		"order_by": "timestamp",
	}

	_, err := adapter.Query(stdctx.Background(), mapOpts)
	if err != nil {
		t.Errorf("Query(order_by) = %v", err)
	}
}

func TestJournalAdapter_QueryWithSREDTagString(t *testing.T) {
	mockJ := &mockJournalForAdapter{
		queryFn: func(ctx stdctx.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
			if opts.SREDTag != contracts.SREDU2SecurityGates {
				t.Errorf("SREDTag = %v, want %v", opts.SREDTag, contracts.SREDU2SecurityGates)
			}
			return []journal.Receipt{}, nil
		},
	}
	adapter := NewJournalAdapter(mockJ, false)

	mapOpts := map[string]interface{}{
		"sred_tag": string(contracts.SREDU2SecurityGates),
	}

	_, err := adapter.Query(stdctx.Background(), mapOpts)
	if err != nil {
		t.Errorf("Query(sred_tag string) = %v", err)
	}
}
