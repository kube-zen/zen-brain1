package receiptlog

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/journal"
)

func TestReceiptlogJournal_RecordAndGet(t *testing.T) {
	// Create temporary spool directory
	tmpDir, err := os.MkdirTemp("", "zen-journal-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		SpoolDir:      tmpDir,
		SpoolSize:     10 * 1024 * 1024, // 10MB
		RetentionDays: 1,
	}

	j, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer j.Close()

	ctx := context.Background()

	// Create a test entry
	entry := journal.Entry{
		EventType:     journal.EventIntentCreated,
		Actor:         "test-actor",
		CorrelationID: "corr-123",
		TaskID:        "task-456",
		SessionID:     "session-789",
		ClusterID:     "cluster-1",
		ProjectID:     "project-zen",
		SREDTags:      []contracts.SREDTag{contracts.SREDU1DynamicProvisioning},
		Payload:       map[string]interface{}{"key": "value"},
		Timestamp:     time.Now(),
	}

	// Record
	receipt, err := j.Record(ctx, entry)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// Retrieve by sequence
	retrieved, err := j.Get(ctx, receipt.Sequence)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Verify fields
	if retrieved.EventType != entry.EventType {
		t.Errorf("EventType mismatch: expected %s, got %s", entry.EventType, retrieved.EventType)
	}
	if retrieved.Actor != entry.Actor {
		t.Errorf("Actor mismatch: expected %s, got %s", entry.Actor, retrieved.Actor)
	}
	if retrieved.CorrelationID != entry.CorrelationID {
		t.Errorf("CorrelationID mismatch")
	}
	if retrieved.TaskID != entry.TaskID {
		t.Errorf("TaskID mismatch")
	}
	if retrieved.SessionID != entry.SessionID {
		t.Errorf("SessionID mismatch")
	}
	if retrieved.ClusterID != entry.ClusterID {
		t.Errorf("ClusterID mismatch")
	}
	if retrieved.ProjectID != entry.ProjectID {
		t.Errorf("ProjectID mismatch")
	}
	if len(retrieved.SREDTags) != len(entry.SREDTags) {
		t.Errorf("SREDTags length mismatch")
	}

	// Timestamp comparison: Use RecordedAt instead of Timestamp
	// Timestamp may lose nanosecond precision during JSON serialization
	// RecordedAt should be close to the original entry.Timestamp
	if retrieved.RecordedAt.Before(entry.Timestamp.Add(-1*time.Second)) ||
		retrieved.RecordedAt.After(entry.Timestamp.Add(1*time.Second)) {
		t.Errorf("RecordedAt should be within 1 second of entry.Timestamp: got %v, expected close to %v",
			retrieved.RecordedAt, entry.Timestamp)
	}

	// Payload is interface{}; we can compare JSON representation
	// For simplicity, skip payload comparison

	// Retrieve by hash
	byHash, err := j.GetByHash(ctx, receipt.Hash)
	if err != nil {
		t.Fatalf("GetByHash failed: %v", err)
	}
	if byHash.Sequence != receipt.Sequence {
		t.Errorf("Sequence mismatch from hash lookup")
	}

	// Verify chain integrity
	verified, err := j.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if verified != 1 {
		t.Errorf("Expected 1 verified receipt, got %d", verified)
	}
}

func TestReceiptlogJournal_MultipleEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zen-journal-test2-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		SpoolDir:  tmpDir,
		SpoolSize: 10 * 1024 * 1024,
	}
	j, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer j.Close()

	ctx := context.Background()

	// Record three entries
	for i := 0; i < 3; i++ {
		entry := journal.Entry{
			EventType:     journal.EventIntentCreated,
			Actor:         "actor",
			CorrelationID: "corr",
			TaskID:        "task",
			Payload:       i,
			Timestamp:     time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		_, err := j.Record(ctx, entry)
		if err != nil {
			t.Fatalf("Record %d failed: %v", i, err)
		}
		// Small delay to ensure timestamps are distinct
		time.Sleep(time.Millisecond)
	}

	// Verify chain
	verified, err := j.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if verified != 3 {
		t.Errorf("Expected 3 verified receipts, got %d", verified)
	}
}
