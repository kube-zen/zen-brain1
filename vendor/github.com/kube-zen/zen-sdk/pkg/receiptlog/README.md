# Receipt Ledger

An immutable receipt ledger for event auditing with local spool and async S3 upload.

## Features

- **Append-only local spool**: Receipts written to newline-delimited JSON files
- **Chain hashing**: Each receipt links to previous via SHA-256 hash (tamper-evidence)
- **Async S3 upload**: Background upload to S3 with WORM/Object Lock support
- **Fast indexing**: In-memory index for O(1) lookup by sequence or hash
- **Crash recovery**: Replays spool on restart to rebuild state
- **Size-based rotation**: Rotates spool files when size limit reached

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Event Source   │───▶│  Receipt Ledger  │───▶│  Local Spool    │
│  (Ingester/     │    │  (Append-Only)   │    │  (JSON Lines)   │
│   Egress)       │    └──────────────────┘    └─────────────────┘
└─────────────────┘             │
                                │ Async (background)
                                ▼
                         ┌─────────────────┐
                         │  S3 Uploader    │
                         │  (WORM/Object   │
                         │   Lock support) │
                         └─────────────────┘
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/kube-zen/zen-sdk/pkg/receiptlog"
)

func main() {
    // Create ledger with local spool only
    ledger, err := receiptlog.New(receiptlog.Config{
        SpoolDir:     "/var/lib/zen/receipts",
        SpoolSize:    100 * 1024 * 1024, // 100MB
        RetentionDays: 7,
    })
    if err != nil {
        panic(err)
    }
    defer ledger.Close()

    // Record a receipt
    receipt, err := ledger.Record(context.Background(), receiptlog.Entry{
        EventType:  "webhook.received",
        Source:     "stripe",
        ExternalID: "evt_123",
        Payload:    payloadBytes,
        Metadata: map[string]string{
            "status":     "success",
            "source_ip":  "192.168.1.1",
        },
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("Recorded receipt %d with hash %s\n", receipt.Sequence, receipt.Hash)
}
```

### With S3 Upload

```go
ledger, err := receiptlog.New(receiptlog.Config{
    SpoolDir:    "/var/lib/zen/receipts",
    SpoolSize:   100 * 1024 * 1024, // 100MB
    S3Bucket:    "my-audit-bucket",
    S3Prefix:    "receipts/ingester",
    S3Region:    "us-east-1",
    UploadDelay: 30 * time.Second,  // Batch uploads
    UploadBatch: 100,               // Or 100 receipts
})
```

### Retrieving Receipts

```go
// By sequence number
receipt, err := ledger.Get(ctx, 123)

// By hash
receipt, err := ledger.GetByHash(ctx, "sha256:abc123...")
```

### Verifying Chain Integrity

```go
verified, err := ledger.Verify(ctx)
if err != nil {
    // Tampering detected or chain broken
    log.Printf("Chain verification failed: %v", err)
}
fmt.Printf("Verified %d receipts\n", verified)
```

## Receipt Structure

```json
{
  "sequence": 123,
  "hash": "a1b2c3...",
  "prev_hash": "x9y8z7...",
  "event_type": "webhook.received",
  "source": "stripe",
  "external_id": "evt_123",
  "payload_hash": "sha256:def456...",
  "metadata": {
    "status": "success"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "recorded_at": "2024-01-15T10:30:00.123Z",
  "uploaded_at": "2024-01-15T10:30:30Z"
}
```

## S3 Object Layout

```
s3://my-audit-bucket/
└── receipts/
    └── ingester/
        ├── 2024-01-15/
        │   ├── receipts-103000.ndjson
        │   ├── receipts-103030.ndjson
        │   └── receipts-103100.ndjson
        └── 2024-01-16/
            └── receipts-090000.ndjson
```

## Tamper Evidence

The chain hash provides tamper evidence:

1. Each receipt includes `prev_hash` linking to previous receipt
2. Changing any receipt breaks the chain
3. Compute `hash = SHA256(sequence | fields | prev_hash | ...)`
4. Any modification changes the hash, breaking the chain

### Verification

```go
// Verify entire chain
verified, err := ledger.Verify(ctx)
if err != nil {
    // One of:
    // - ErrHashMismatch: receipt hash doesn't match computed hash
    // - ErrSequenceGap: missing sequence number
    log.Fatalf("Chain corrupted: %v", err)
}
```

## Performance

- **Record**: ~100µs (fsync to disk)
- **Get by sequence**: ~10µs (in-memory index + file read)
- **Get by hash**: ~10µs (hash -> sequence lookup + file read)
- **Verify**: ~100µs per receipt (hash computation)

## Best Practices

1. **Use IAM roles for S3**: Don't hardcode credentials
2. **Enable Object Lock**: Prevent deletion in S3
3. **Monitor spool size**: Set alerts on disk usage
4. **Regular verification**: Run `Verify()` periodically
5. **Backup spool files**: Copy rotated files to backup storage

## Integration Points

### zen-ingester (Ingress Receipts)

Record receipts in `webhook_handler.go`:

```go
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    
    receipt, err := h.ledger.Record(r.Context(), receiptlog.Entry{
        EventType:  "webhook.received",
        Source:     source,
        ExternalID: extractEventID(body),
        Payload:    body,
        Metadata: map[string]string{
            "source_ip": r.RemoteAddr,
            "path":      r.URL.Path,
        },
    })
    // Continue processing...
}
```

### zen-egress (Delivery Receipts)

Record receipts in `simple_event_processor.go`:

```go
func (p *SimpleEventProcessor) deliver(event Event) error {
    resp, err := p.httpClient.Do(req)
    
    receipt, err := p.ledger.Record(ctx, receiptlog.Entry{
        EventType:  "delivery.success",
        Source:     event.Source,
        ExternalID: event.ID,
        Metadata: map[string]string{
            "target":     p.config.Target,
            "status":     strconv.Itoa(resp.StatusCode),
            "duration_ms": fmt.Sprint(duration.Milliseconds()),
        },
    })
    // Continue...
}
```

## License

Apache 2.0
