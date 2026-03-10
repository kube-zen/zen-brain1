package session

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	journalpkg "github.com/kube-zen/zen-brain1/pkg/journal"
	"github.com/kube-zen/zen-brain1/pkg/messagebus"
)

// mockJournalRecorder records entries for tests.
type mockJournalRecorder struct {
	mu      sync.Mutex
	entries []journalpkg.Entry
}

func (m *mockJournalRecorder) Record(ctx context.Context, entry journalpkg.Entry) (*journalpkg.Receipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
	return &journalpkg.Receipt{Entry: entry, Sequence: uint64(len(m.entries)), RecordedAt: time.Now()}, nil
}

func (m *mockJournalRecorder) Entries() []journalpkg.Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]journalpkg.Entry, len(m.entries))
	copy(out, m.entries)
	return out
}

// mockEventPublisher records published events for tests.
type mockEventPublisher struct {
	mu     sync.Mutex
	events []struct{ stream string; ev *messagebus.Event }
}

func (m *mockEventPublisher) Publish(ctx context.Context, stream string, event *messagebus.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, struct{ stream string; ev *messagebus.Event }{stream, event})
	return nil
}

func (m *mockEventPublisher) Events() []struct{ Stream string; Type string } {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]struct{ Stream string; Type string }, len(m.events))
	for i, e := range m.events {
		out[i] = struct{ Stream string; Type string }{e.stream, e.ev.Type}
	}
	return out
}

func TestEmitSessionCreated(t *testing.T) {
	ctx := context.Background()
	journal := &mockJournalRecorder{}
	bus := &mockEventPublisher{}
	cfg := &Config{Journal: journal, EventBus: bus, EventStream: "test.events"}
	session := &contracts.Session{ID: "s1", WorkItemID: "w1", State: contracts.SessionStateCreated}
	EmitSessionCreated(ctx, cfg, session, "w1")
	if len(journal.Entries()) != 1 {
		t.Fatalf("expected 1 journal entry, got %d", len(journal.Entries()))
	}
	if journal.Entries()[0].EventType != journalpkg.EventSessionCreated {
		t.Errorf("event_type = %s", journal.Entries()[0].EventType)
	}
	if len(bus.Events()) != 1 {
		t.Fatalf("expected 1 bus event, got %d", len(bus.Events()))
	}
	if bus.Events()[0].Type != "session.created" {
		t.Errorf("bus event type = %s", bus.Events()[0].Type)
	}
}

func TestEmitSessionTransitioned(t *testing.T) {
	ctx := context.Background()
	journal := &mockJournalRecorder{}
	bus := &mockEventPublisher{}
	cfg := &Config{Journal: journal, EventBus: bus, EventStream: "test.events"}
	EmitSessionTransitioned(ctx, cfg, "s1", "w1", "created", "analyzed", "test", "agent")
	if len(journal.Entries()) != 1 {
		t.Fatalf("expected 1 journal entry, got %d", len(journal.Entries()))
	}
	payload, _ := json.Marshal(journal.Entries()[0].Payload)
	var m map[string]interface{}
	json.Unmarshal(payload, &m)
	if m["to_state"] != "analyzed" {
		t.Errorf("payload to_state = %v", m["to_state"])
	}
	if len(bus.Events()) != 1 || bus.Events()[0].Type != "session.transitioned" {
		t.Errorf("bus events = %+v", bus.Events())
	}
}

func TestEmitSessionEvidenceAdded(t *testing.T) {
	ctx := context.Background()
	bus := &mockEventPublisher{}
	cfg := &Config{EventBus: bus, EventStream: "test.events"}
	ev := contracts.EvidenceItem{ID: "e1", Type: contracts.EvidenceTypeProofOfWork, CollectedBy: "factory"}
	EmitSessionEvidenceAdded(ctx, cfg, "s1", "w1", ev)
	if len(bus.Events()) != 1 || bus.Events()[0].Type != "session.evidence_added" {
		t.Errorf("bus events = %+v", bus.Events())
	}
}

func TestEmitSessionCheckpointUpdated(t *testing.T) {
	ctx := context.Background()
	journal := &mockJournalRecorder{}
	cfg := &Config{Journal: journal, EventStream: "test.events"}
	EmitSessionCheckpointUpdated(ctx, cfg, "s1", "w1", "proof_attached")
	if len(journal.Entries()) != 1 {
		t.Fatalf("expected 1 journal entry, got %d", len(journal.Entries()))
	}
	if journal.Entries()[0].EventType != journalpkg.EventSessionCheckpointUpdated {
		t.Errorf("event_type = %s", journal.Entries()[0].EventType)
	}
}
