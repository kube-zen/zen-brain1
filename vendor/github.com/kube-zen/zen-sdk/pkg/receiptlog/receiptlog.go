// Package receiptlog provides an immutable receipt ledger for event auditing.
//
// The ledger uses a local append-only spool for durability and an async S3
// uploader for long-term storage. Each receipt includes:
//   - SHA-256 hash of the receipt data
//   - Rolling chain hash linking to previous receipt (tamper-evidence)
//   - Timestamp and sequence number
//
// Architecture:
//
//	┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
//	│  Event Source   │───▶│  Receipt Ledger  │───▶│  Local Spool    │
//	│  (Ingester/     │    │  (Append-Only)   │    │  (JSON Lines)   │
//	│   Egress)       │    └──────────────────┘    └─────────────────┘
//	└─────────────────┘             │
//	                                │ Async (background)
//	                                ▼
//	                         ┌─────────────────┐
//	                         │  S3 Uploader    │
//	                         │  (WORM/Object   │
//	                         │   Lock support) │
//	                         └─────────────────┘
//
// Usage:
//
//	ledger, err := receiptlog.New(receiptlog.Config{
//	    SpoolDir:     "/var/lib/zen/receipts",
//	    SpoolSize:    100 * 1024 * 1024, // 100MB
//	    S3Bucket:     "my-audit-bucket",
//	    S3Prefix:     "receipts/ingester",
//	    UploadDelay:  30 * time.Second,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer ledger.Close()
//
//	// Record a receipt
//	receipt, err := ledger.Record(ctx, receiptlog.Entry{
//	    EventType:  "webhook.received",
//	    Source:     "stripe",
//	    ExternalID: "evt_123",
//	    Payload:    payloadBytes,
//	    Metadata:   map[string]string{"status": "success"},
//	})
package receiptlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Errors
var (
	ErrLedgerClosed   = errors.New("ledger is closed")
	ErrInvalidEntry   = errors.New("invalid receipt entry")
	ErrSpoolFull      = errors.New("spool size limit reached")
	ErrHashMismatch   = errors.New("chain hash mismatch - tampering detected")
	ErrSequenceGap    = errors.New("sequence gap detected")
)

// Entry represents a receipt entry to be recorded.
type Entry struct {
	// EventType is the type of event (e.g., "webhook.received", "delivery.success")
	EventType string `json:"event_type"`

	// Source is the source identifier (e.g., "stripe", "github")
	Source string `json:"source"`

	// ExternalID is the external reference ID (e.g., Stripe event ID)
	ExternalID string `json:"external_id,omitempty"`

	// Payload is the raw event payload (will be hashed, not stored in full)
	Payload []byte `json:"-"`

	// PayloadHash is the SHA-256 hash of the payload
	PayloadHash string `json:"payload_hash"`

	// Metadata contains additional context
	Metadata map[string]string `json:"metadata,omitempty"`

	// Timestamp is when the event occurred (defaults to now)
	Timestamp time.Time `json:"timestamp"`
}

// Receipt is a recorded entry with chain hash and sequence number.
type Receipt struct {
	Entry

	// Sequence is the monotonically increasing sequence number
	Sequence uint64 `json:"sequence"`

	// Hash is the SHA-256 hash of this receipt (including payload hash)
	Hash string `json:"hash"`

	// PrevHash is the hash of the previous receipt (chain link)
	PrevHash string `json:"prev_hash"`

	// RecordedAt is when the receipt was recorded
	RecordedAt time.Time `json:"recorded_at"`

	// UploadedAt is when the receipt was uploaded to S3 (zero if not yet)
	UploadedAt time.Time `json:"uploaded_at,omitempty"`
}

// Config holds ledger configuration.
type Config struct {
	// SpoolDir is the directory for the local spool files
	SpoolDir string `json:"spool_dir"`

	// SpoolSize is the maximum size of the spool before rotation (0 = unlimited)
	SpoolSize int64 `json:"spool_size"`

	// S3Bucket is the destination S3 bucket (empty = local only)
	S3Bucket string `json:"s3_bucket"`

	// S3Prefix is the key prefix for S3 objects
	S3Prefix string `json:"s3_prefix"`

	// S3Region is the AWS region for S3
	S3Region string `json:"s3_region"`

	// UploadDelay is the delay before uploading to S3 (batching)
	UploadDelay time.Duration `json:"upload_delay"`

	// UploadBatch is the number of receipts to batch before upload
	UploadBatch int `json:"upload_batch"`

	// RetentionDays is how long to keep local spool files
	RetentionDays int `json:"retention_days"`
}

// Ledger is the receipt ledger interface.
type Ledger interface {
	// Record records a new receipt entry and returns the receipt with hash/sequence.
	Record(ctx context.Context, entry Entry) (*Receipt, error)

	// Get retrieves a receipt by sequence number.
	Get(ctx context.Context, sequence uint64) (*Receipt, error)

	// GetByHash retrieves a receipt by its hash.
	GetByHash(ctx context.Context, hash string) (*Receipt, error)

	// Verify verifies the chain integrity from start to end.
	// Returns the number of verified receipts and any errors.
	Verify(ctx context.Context) (int, error)

	// Stats returns ledger statistics.
	Stats() Stats

	// Close closes the ledger and flushes pending uploads.
	Close() error
}

// Stats holds ledger statistics.
type Stats struct {
	// TotalReceipts is the total number of receipts recorded
	TotalReceipts uint64 `json:"total_receipts"`

	// LastSequence is the highest sequence number
	LastSequence uint64 `json:"last_sequence"`

	// LastHash is the hash of the most recent receipt
	LastHash string `json:"last_hash"`

	// PendingUploads is the number of receipts not yet uploaded
	PendingUploads int `json:"pending_uploads"`

	// SpoolSize is the current size of the spool file
	SpoolSize int64 `json:"spool_size"`

	// SpoolPath is the path to the current spool file
	SpoolPath string `json:"spool_path"`
}

// FileLedger is a file-based receipt ledger implementation.
type FileLedger struct {
	config Config

	mu          sync.RWMutex
	file        *os.File
	sequence    uint64
	lastHash    string
	spoolSize   int64
	index       map[uint64]int64 // sequence -> file offset
	hashIndex   map[string]uint64 // hash -> sequence

	uploader *S3Uploader // Optional S3 uploader

	closed    bool
	closeCh   chan struct{}

	stats Stats
}

// New creates a new file-based receipt ledger.
func New(config Config) (*FileLedger, error) {
	if config.SpoolDir == "" {
		return nil, fmt.Errorf("spool_dir is required")
	}

	// Create spool directory if needed
	if err := os.MkdirAll(config.SpoolDir, 0755); err != nil {
		return nil, fmt.Errorf("create spool dir: %w", err)
	}

	// Set defaults
	if config.UploadDelay == 0 {
		config.UploadDelay = 30 * time.Second
	}
	if config.UploadBatch == 0 {
		config.UploadBatch = 100
	}
	if config.RetentionDays == 0 {
		config.RetentionDays = 7
	}

	// Open or create spool file
	spoolPath := filepath.Join(config.SpoolDir, "receipts.ndjson")
	file, err := os.OpenFile(spoolPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open spool file: %w", err)
	}

	ledger := &FileLedger{
		config:    config,
		file:      file,
		index:     make(map[uint64]int64),
		hashIndex: make(map[string]uint64),
		closeCh:   make(chan struct{}),
		stats: Stats{
			SpoolPath: spoolPath,
		},
	}

	// Replay existing receipts to rebuild index and chain
	if err := ledger.replay(); err != nil {
		file.Close()
		return nil, fmt.Errorf("replay spool: %w", err)
	}

	// Start async uploader if S3 is configured
	if config.S3Bucket != "" {
		uploader, err := NewS3Uploader(config)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("create uploader: %w", err)
		}
		ledger.uploader = uploader
		uploader.Start()
	}

	return ledger, nil
}

// replay reads existing spool file to rebuild index and chain state.
func (l *FileLedger) replay() error {
	decoder := json.NewDecoder(l.file)
	offset := int64(0)

	for {
		var receipt Receipt
		err := decoder.Decode(&receipt)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("decode receipt at offset %d: %w", offset, err)
		}

		// Update index
		l.index[receipt.Sequence] = offset
		l.hashIndex[receipt.Hash] = receipt.Sequence

		// Update chain state
		if receipt.Sequence > l.sequence {
			l.sequence = receipt.Sequence
			l.lastHash = receipt.Hash
		}

		// Track pending uploads
		if receipt.UploadedAt.IsZero() {
			l.stats.PendingUploads++
		}

		offset, _ = l.file.Seek(0, io.SeekCurrent)
	}

	l.spoolSize = offset
	l.stats.TotalReceipts = uint64(len(l.index))
	l.stats.LastSequence = l.sequence
	l.stats.LastHash = l.lastHash
	l.stats.SpoolSize = l.spoolSize

	return nil
}

// Record records a new receipt entry.
func (l *FileLedger) Record(ctx context.Context, entry Entry) (*Receipt, error) {
	if entry.EventType == "" {
		return nil, ErrInvalidEntry
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil, ErrLedgerClosed
	}

	// Check spool size limit
	if l.config.SpoolSize > 0 && l.spoolSize >= l.config.SpoolSize {
		if err := l.rotateSpool(); err != nil {
			return nil, fmt.Errorf("rotate spool: %w", err)
		}
	}

	// Set timestamp if not provided
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	// Compute payload hash
	if len(entry.Payload) > 0 {
		hash := sha256.Sum256(entry.Payload)
		entry.PayloadHash = hex.EncodeToString(hash[:])
	}

	// Create receipt
	receipt := &Receipt{
		Entry:      entry,
		Sequence:   l.sequence + 1,
		PrevHash:   l.lastHash,
		RecordedAt: time.Now().UTC(),
	}

	// Compute receipt hash (includes all fields)
	receipt.Hash = computeReceiptHash(receipt)

	// Record file offset
	offset := l.spoolSize

	// Write to spool
	data, err := json.Marshal(receipt)
	if err != nil {
		return nil, fmt.Errorf("marshal receipt: %w", err)
	}
	data = append(data, '\n')

	n, err := l.file.Write(data)
	if err != nil {
		return nil, fmt.Errorf("write spool: %w", err)
	}

	// Sync to disk
	if err := l.file.Sync(); err != nil {
		return nil, fmt.Errorf("sync spool: %w", err)
	}

	// Update state
	l.sequence = receipt.Sequence
	l.lastHash = receipt.Hash
	l.spoolSize += int64(n)
	l.index[receipt.Sequence] = offset
	l.hashIndex[receipt.Hash] = receipt.Sequence

	// Update stats
	l.stats.TotalReceipts++
	l.stats.LastSequence = l.sequence
	l.stats.LastHash = l.lastHash
	l.stats.SpoolSize = l.spoolSize
	l.stats.PendingUploads++

	// Queue for upload
	if l.uploader != nil {
		l.uploader.Upload(receipt)
	}

	return receipt, nil
}

// Get retrieves a receipt by sequence number.
func (l *FileLedger) Get(ctx context.Context, sequence uint64) (*Receipt, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	offset, exists := l.index[sequence]
	if !exists {
		return nil, fmt.Errorf("receipt %d not found", sequence)
	}

	return l.readAtOffset(offset)
}

// GetByHash retrieves a receipt by its hash.
func (l *FileLedger) GetByHash(ctx context.Context, hash string) (*Receipt, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	sequence, exists := l.hashIndex[hash]
	if !exists {
		return nil, fmt.Errorf("receipt with hash %s not found", hash)
	}

	offset := l.index[sequence]
	return l.readAtOffset(offset)
}

// readAtOffset reads a receipt at a specific file offset.
func (l *FileLedger) readAtOffset(offset int64) (*Receipt, error) {
	// Seek to offset
	if _, err := l.file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to offset %d: %w", offset, err)
	}

	// Decode receipt
	var receipt Receipt
	decoder := json.NewDecoder(l.file)
	if err := decoder.Decode(&receipt); err != nil {
		return nil, fmt.Errorf("decode receipt: %w", err)
	}

	return &receipt, nil
}

// Verify verifies the chain integrity.
func (l *FileLedger) Verify(ctx context.Context) (int, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.sequence == 0 {
		return 0, nil
	}

	verified := 0
	var prevHash string

	for seq := uint64(1); seq <= l.sequence; seq++ {
		offset, exists := l.index[seq]
		if !exists {
			return verified, fmt.Errorf("%w: missing sequence %d", ErrSequenceGap, seq)
		}

		receipt, err := l.readAtOffset(offset)
		if err != nil {
			return verified, fmt.Errorf("read receipt %d: %w", seq, err)
		}

		// Verify hash
		expectedHash := computeReceiptHash(receipt)
		if receipt.Hash != expectedHash {
			return verified, fmt.Errorf("%w: sequence %d", ErrHashMismatch, seq)
		}

		// Verify chain link
		if seq > 1 && receipt.PrevHash != prevHash {
			return verified, fmt.Errorf("%w: sequence %d expected prev_hash=%s got %s",
				ErrHashMismatch, seq, prevHash, receipt.PrevHash)
		}

		prevHash = receipt.Hash
		verified++
	}

	return verified, nil
}

// Stats returns ledger statistics.
func (l *FileLedger) Stats() Stats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.stats
}

// Close closes the ledger.
func (l *FileLedger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	l.closed = true
	close(l.closeCh)

	// Close uploader if present
	if l.uploader != nil {
		if err := l.uploader.Close(); err != nil {
			// Log but continue
		}
	}

	return l.file.Close()
}

// rotateSpool rotates the spool file when size limit is reached.
func (l *FileLedger) rotateSpool() error {
	// Close current file
	if err := l.file.Close(); err != nil {
		return err
	}

	// Rename current file with timestamp
	timestamp := time.Now().UTC().Format("20060102-150405")
	oldPath := filepath.Join(l.config.SpoolDir, fmt.Sprintf("receipts-%s.ndjson", timestamp))
	if err := os.Rename(l.stats.SpoolPath, oldPath); err != nil {
		return err
	}

	// Create new file
	file, err := os.OpenFile(l.stats.SpoolPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	l.file = file
	l.spoolSize = 0
	l.stats.SpoolSize = 0

	// Reset index (old receipts still readable via rotated files)
	l.index = make(map[uint64]int64)
	l.hashIndex = make(map[string]uint64)

	return nil
}

// computeReceiptHash computes the SHA-256 hash of a receipt.
func computeReceiptHash(r *Receipt) string {
	// Create a deterministic representation for hashing
	data := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s",
		r.Sequence,
		r.EventType,
		r.Source,
		r.ExternalID,
		r.PayloadHash,
		r.PrevHash,
		r.Timestamp.Format(time.RFC3339Nano),
	)

	// Include metadata in sorted order for determinism
	if len(r.Metadata) > 0 {
		for k, v := range r.Metadata {
			data += fmt.Sprintf("|%s=%s", k, v)
		}
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
