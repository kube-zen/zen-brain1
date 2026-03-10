package receiptlog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/objectstore"
)

// SignalEmitter is an interface for emitting operational signals.
// This is used to avoid circular dependencies between zen-sdk and zen-platform.
type SignalEmitter interface {
	// Emit emits a signal with metadata.
	// Implementations should be non-blocking and best-effort.
	Emit(ctx context.Context, signalType string, severity string, metadata map[string]string) error
}

// S3Uploader handles async upload of receipts to S3.
type S3Uploader struct {
	store         objectstore.ObjectStore
	config        Config
	batch         []*Receipt
	batchMu       sync.Mutex
	closeCh       chan struct{}

	// Optional signal emitter for ops signals (best-effort)
	signalEmitter SignalEmitter
}

// NewS3Uploader creates a new S3 uploader.
func NewS3Uploader(config Config) (*S3Uploader, error) {
	if config.S3Bucket == "" {
		return nil, fmt.Errorf("s3_bucket is required")
	}
	if config.S3Region == "" {
		config.S3Region = "us-east-1"
	}

	store, err := objectstore.NewObjectStore(&objectstore.Config{
		Type: "s3",
		S3: &objectstore.S3Config{
			BucketName: config.S3Bucket,
			Region:     config.S3Region,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create object store: %w", err)
	}

	uploader := &S3Uploader{
		store:   store,
		config:  config,
		batch:   make([]*Receipt, 0, config.UploadBatch),
		closeCh: make(chan struct{}),
	}

	return uploader, nil
}

// SetSignalEmitter sets the signal emitter for ops signals.
// This is optional - if not set, no signals will be emitted.
func (u *S3Uploader) SetSignalEmitter(emitter SignalEmitter) {
	u.signalEmitter = emitter
}

// Upload adds a receipt to the upload batch.
func (u *S3Uploader) Upload(receipt *Receipt) {
	u.batchMu.Lock()
	u.batch = append(u.batch, receipt)
	shouldFlush := len(u.batch) >= u.config.UploadBatch
	u.batchMu.Unlock()

	if shouldFlush {
		go u.flushBatch()
	}
}

// Start begins the periodic upload worker.
func (u *S3Uploader) Start() {
	go u.worker()
}

// Close stops the uploader and flushes pending uploads.
func (u *S3Uploader) Close() error {
	close(u.closeCh)
	u.flushBatch() // Final flush
	return u.store.Close()
}

// worker handles periodic uploads.
func (u *S3Uploader) worker() {
	ticker := time.NewTicker(u.config.UploadDelay)
	defer ticker.Stop()

	for {
		select {
		case <-u.closeCh:
			return
		case <-ticker.C:
			u.flushBatch()
		}
	}
}

// flushBatch uploads the current batch to S3.
func (u *S3Uploader) flushBatch() {
	u.batchMu.Lock()
	if len(u.batch) == 0 {
		u.batchMu.Unlock()
		return
	}

	batch := u.batch
	u.batch = make([]*Receipt, 0, u.config.UploadBatch)
	u.batchMu.Unlock()

	// Group receipts by date for S3 key organization
	dateGroups := make(map[string][]*Receipt)
	for _, r := range batch {
		dateKey := r.RecordedAt.Format("2006-01-02")
		dateGroups[dateKey] = append(dateGroups[dateKey], r)
	}

	// Upload each date group as a single file
	for dateKey, receipts := range dateGroups {
		if err := u.uploadDateGroup(dateKey, receipts); err != nil {
			// Log error but continue - receipts still in local spool
			fmt.Printf("[S3Uploader] Upload failed for %s: %v\n", dateKey, err)
		}
	}
}

// uploadDateGroup uploads a group of receipts to S3.
func (u *S3Uploader) uploadDateGroup(dateKey string, receipts []*Receipt) error {
	// Serialize receipts as JSON lines
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	for _, r := range receipts {
		if err := encoder.Encode(r); err != nil {
			u.emitSignal("archive.upload.failed", "warning", map[string]string{
				"error":       fmt.Sprintf("encode receipt %d: %v", r.Sequence, err),
				"error_class": "serialization",
				"count":       fmt.Sprint(len(receipts)),
			})
			return fmt.Errorf("encode receipt %d: %w", r.Sequence, err)
		}
	}

	// Generate S3 key
	timestamp := time.Now().UTC().Format("150405")
	key := filepath.Join(
		u.config.S3Prefix,
		dateKey,
		fmt.Sprintf("receipts-%s.ndjson", timestamp),
	)

	// Upload to S3
	ctx := context.Background()
	if err := u.store.PutObject(ctx, u.config.S3Bucket, key, buf.Bytes()); err != nil {
		u.emitSignal("archive.upload.failed", "warning", map[string]string{
			"error":       err.Error(),
			"error_class": "transient",
			"count":       fmt.Sprint(len(receipts)),
			"date":        dateKey,
		})
		return fmt.Errorf("put object: %w", err)
	}

	// Success - emit signal
	u.emitSignal("archive.upload.success", "info", map[string]string{
		"count": fmt.Sprint(len(receipts)),
		"date":  dateKey,
	})

	return nil
}

// emitSignal emits a signal if the signal emitter is set.
// Best-effort - errors are logged but don't affect upload logic.
func (u *S3Uploader) emitSignal(signalType, severity string, metadata map[string]string) {
	if u.signalEmitter == nil {
		return
	}

	ctx := context.Background()
	_ = u.signalEmitter.Emit(ctx, signalType, severity, metadata)
}

// Stats returns upload statistics.
func (u *S3Uploader) Stats() map[string]interface{} {
	u.batchMu.Lock()
	defer u.batchMu.Unlock()

	return map[string]interface{}{
		"pending_batch": len(u.batch),
		"batch_size":    u.config.UploadBatch,
		"upload_delay":  u.config.UploadDelay.String(),
	}
}

// CheckBacklog checks if the pending batch exceeds threshold.
// Emits a signal if backlog is high.
func (u *S3Uploader) CheckBacklog(threshold int) {
	u.batchMu.Lock()
	currentSize := len(u.batch)
	u.batchMu.Unlock()

	// Check if backlog is high
	if currentSize >= threshold {
		u.emitSignal("archive.backlog.high", "warning", map[string]string{
			"backlog_records": fmt.Sprint(currentSize),
			"threshold":      fmt.Sprint(threshold),
			"batch_size":     fmt.Sprint(u.config.UploadBatch),
		})
	}
}
