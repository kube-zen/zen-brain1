// Package receiptlog implements ZenJournal using zen-sdk/pkg/receiptlog.
// It provides an immutable event ledger with chain hashing for tamper evidence.
package receiptlog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/journal"
	"github.com/kube-zen/zen-sdk/pkg/receiptlog"
)

// Config holds configuration for the receiptlog journal.
type Config struct {
	// SpoolDir is the directory for local spool files.
	SpoolDir string `json:"spool_dir"`

	// SpoolSize is the maximum size of each spool file in bytes.
	SpoolSize int64 `json:"spool_size"`

	// RetentionDays is how long to keep spool files (0 = forever).
	RetentionDays int `json:"retention_days"`

	// S3Bucket for archival upload (optional).
	S3Bucket string `json:"s3_bucket,omitempty"`

	// S3Prefix for object prefix (optional).
	S3Prefix string `json:"s3_prefix,omitempty"`

	// S3Region for bucket (optional).
	S3Region string `json:"s3_region,omitempty"`

	// UploadDelay is the delay between upload batches.
	UploadDelay time.Duration `json:"upload_delay,omitempty"`

	// UploadBatch is the batch size for uploads.
	UploadBatch int `json:"upload_batch,omitempty"`
}

// receiptlogJournal implements journal.ZenJournal.
type receiptlogJournal struct {
	ledger receiptlog.Ledger
	index *QueryIndex
}

// New creates a new ZenJournal backed by receiptlog.
func New(cfg *Config) (journal.ZenJournal, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if cfg.SpoolDir == "" {
		return nil, fmt.Errorf("SpoolDir is required")
	}
	if cfg.SpoolSize == 0 {
		cfg.SpoolSize = 100 * 1024 * 1024 // 100MB default
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 7 // keep for a week
	}

	rcfg := receiptlog.Config{
		SpoolDir:      cfg.SpoolDir,
		SpoolSize:     cfg.SpoolSize,
		RetentionDays: cfg.RetentionDays,
		S3Bucket:      cfg.S3Bucket,
		S3Prefix:      cfg.S3Prefix,
		S3Region:      cfg.S3Region,
		UploadDelay:   cfg.UploadDelay,
		UploadBatch:   cfg.UploadBatch,
	}

	ledger, err := receiptlog.New(rcfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create receiptlog ledger: %w", err)
	}

	return &receiptlogJournal{
		ledger: ledger,
		index:  NewQueryIndex(),
	}, nil
}

// Record records a new journal entry and returns the receipt.
func (j *receiptlogJournal) Record(ctx context.Context, entry journal.Entry) (*journal.Receipt, error) {
	// Convert journal.Entry to receiptlog.Entry
	payloadJSON, err := json.Marshal(entry.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	rlogEntry := receiptlog.Entry{
		EventType:  string(entry.EventType),
		Source:     entry.Actor,
		ExternalID: entry.CorrelationID,
		Payload:    payloadJSON, // used for hash, not stored
		Metadata:   make(map[string]string),
		Timestamp:  entry.Timestamp,
	}

	// Add additional fields as metadata
	if entry.TaskID != "" {
		rlogEntry.Metadata["task_id"] = entry.TaskID
	}
	if entry.SessionID != "" {
		rlogEntry.Metadata["session_id"] = entry.SessionID
	}
	if entry.ClusterID != "" {
		rlogEntry.Metadata["cluster_id"] = entry.ClusterID
	}
	if entry.ProjectID != "" {
		rlogEntry.Metadata["project_id"] = entry.ProjectID
	}
	// SREDTags as comma-separated list
	if len(entry.SREDTags) > 0 {
		var tags []string
		for _, tag := range entry.SREDTags {
			tags = append(tags, string(tag))
		}
		rlogEntry.Metadata["sred_tags"] = strings.Join(tags, ",")
	}
	// Store payload as JSON string in metadata (since receiptlog discards Payload)
	rlogEntry.Metadata["payload"] = string(payloadJSON)

	// Record in receiptlog
	rlogReceipt, err := j.ledger.Record(ctx, rlogEntry)
	if err != nil {
		return nil, fmt.Errorf("receiptlog.Record failed: %w", err)
	}

	// Convert receiptlog.Receipt to journal.Receipt
	journalReceipt := toJournalReceipt(rlogReceipt, entry)

	// Add to query index
	j.index.Add(journalReceipt)

	return journalReceipt, nil
}

// Get retrieves a receipt by sequence number.
func (j *receiptlogJournal) Get(ctx context.Context, sequence uint64) (*journal.Receipt, error) {
	rlogReceipt, err := j.ledger.Get(ctx, sequence)
	if err != nil {
		return nil, fmt.Errorf("receiptlog.Get failed: %w", err)
	}
	return fromReceiptlogReceipt(rlogReceipt), nil
}

// GetByHash retrieves a receipt by its hash.
func (j *receiptlogJournal) GetByHash(ctx context.Context, hash string) (*journal.Receipt, error) {
	rlogReceipt, err := j.ledger.GetByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("receiptlog.GetByHash failed: %w", err)
	}
	return fromReceiptlogReceipt(rlogReceipt), nil
}

// Query searches for receipts matching the options.
func (j *receiptlogJournal) Query(ctx context.Context, opts journal.QueryOptions) ([]journal.Receipt, error) {
	// Get matching sequences from index
	sequences := j.index.Query(opts)

	if len(sequences) == 0 {
		return []journal.Receipt{}, nil
	}

	// Fetch receipts by sequences
	return FetchReceipts(ctx, sequences, j.Get)
}

// QueryByCorrelation retrieves all events for a correlation ID.
func (j *receiptlogJournal) QueryByCorrelation(ctx context.Context, correlationID string) ([]journal.Receipt, error) {
	sequences := j.index.QueryByCorrelationID(correlationID)
	if len(sequences) == 0 {
		return []journal.Receipt{}, nil
	}
	return FetchReceipts(ctx, sequences, j.Get)
}

// QueryByTask retrieves all events for a task.
func (j *receiptlogJournal) QueryByTask(ctx context.Context, taskID string) ([]journal.Receipt, error) {
	sequences := j.index.QueryByTaskID(taskID)
	if len(sequences) == 0 {
		return []journal.Receipt{}, nil
	}
	return FetchReceipts(ctx, sequences, j.Get)
}

// QueryBySREDTag retrieves all events with a specific SR&ED tag.
func (j *receiptlogJournal) QueryBySREDTag(ctx context.Context, tag contracts.SREDTag, start, end time.Time) ([]journal.Receipt, error) {
	sequences := j.index.QueryBySREDTag(tag)

	// Filter by time range if specified
	if !start.IsZero() || !end.IsZero() {
		timeFilteredSequences := j.index.filterByTime(sequences, start, end)
		if len(timeFilteredSequences) == 0 {
			return []journal.Receipt{}, nil
		}
		return FetchReceipts(ctx, timeFilteredSequences, j.Get)
	}

	if len(sequences) == 0 {
		return []journal.Receipt{}, nil
	}
	return FetchReceipts(ctx, sequences, j.Get)
}

// Verify verifies the chain integrity.
func (j *receiptlogJournal) Verify(ctx context.Context) (int, error) {
	verified, err := j.ledger.Verify(ctx)
	if err != nil {
		return 0, fmt.Errorf("receiptlog.Verify failed: %w", err)
	}
	return verified, nil
}

// Stats returns journal statistics.
func (j *receiptlogJournal) Stats() journal.Stats {
	// Get index statistics
	indexStats := j.index.Stats()

	// Get latest sequence and hash from receiptlog
	// For simplicity, return empty stats for now
	return journal.Stats{
		TotalReceipts:   uint64(indexStats.TotalEntries),
		LastSequence:    0, // TODO: get from ledger
		LastHash:        "", // TODO: get from ledger
		OldestTimestamp: time.Time{}, // TODO: compute from index
		NewestTimestamp: time.Time{}, // TODO: compute from index
	}
}

// Close closes the journal.
func (j *receiptlogJournal) Close() error {
	return j.ledger.Close()
}

// toJournalReceipt converts receiptlog.Receipt to journal.Receipt.
func toJournalReceipt(rlogReceipt *receiptlog.Receipt, entry journal.Entry) *journal.Receipt {
	return &journal.Receipt{
		Entry:      entry,
		Sequence:   rlogReceipt.Sequence,
		Hash:       rlogReceipt.Hash,
		PrevHash:   rlogReceipt.PrevHash,
		RecordedAt: rlogReceipt.RecordedAt,
	}
}

// fromReceiptlogReceipt converts receiptlog.Receipt to journal.Receipt.
func fromReceiptlogReceipt(rlogReceipt *receiptlog.Receipt) *journal.Receipt {
	entry := journal.Entry{
		EventType:     journal.EventType(rlogReceipt.EventType),
		Actor:         rlogReceipt.Source,
		CorrelationID: rlogReceipt.ExternalID,
		Timestamp:     rlogReceipt.Timestamp,
	}

	// Parse metadata
	if taskID, ok := rlogReceipt.Metadata["task_id"]; ok {
		entry.TaskID = taskID
	}
	if sessionID, ok := rlogReceipt.Metadata["session_id"]; ok {
		entry.SessionID = sessionID
	}
	if clusterID, ok := rlogReceipt.Metadata["cluster_id"]; ok {
		entry.ClusterID = clusterID
	}
	if projectID, ok := rlogReceipt.Metadata["project_id"]; ok {
		entry.ProjectID = projectID
	}
	if sredTagsStr, ok := rlogReceipt.Metadata["sred_tags"]; ok {
		tags := strings.Split(sredTagsStr, ",")
		for _, tag := range tags {
			entry.SREDTags = append(entry.SREDTags, contracts.SREDTag(tag))
		}
	}
	if payloadStr, ok := rlogReceipt.Metadata["payload"]; ok {
		// Unmarshal payload into interface{}
		var payload interface{}
		if err := json.Unmarshal([]byte(payloadStr), &payload); err == nil {
			entry.Payload = payload
		}
	}

	return &journal.Receipt{
		Entry:      entry,
		Sequence:   rlogReceipt.Sequence,
		Hash:       rlogReceipt.Hash,
		PrevHash:   rlogReceipt.PrevHash,
		RecordedAt: rlogReceipt.RecordedAt,
	}
}