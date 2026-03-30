# Dedup Package

Event deduplication package for preventing duplicate observations within configurable time windows.

## Features

- **Content-Based Fingerprinting**: SHA-256 hashing of normalized event content
- **Time-Based Buckets**: Efficient cleanup with configurable bucket sizes
- **Rate Limiting**: Per-source token bucket algorithm to prevent overwhelming
- **Event Aggregation**: Rolling window aggregation for high-volume events
- **Per-Source Windows**: Configurable deduplication windows per source
- **LRU Eviction**: Automatic cache eviction when capacity is reached

## Usage

```go
import "github.com/kube-zen/zen-sdk/pkg/dedup"

// Create a deduper with 60 second window and 10k max entries
deduper := dedup.NewDeduper(60, 10000)

// Check if observation should be created
key := dedup.DedupKey{
    Source:      "falco",
    Namespace:   "default",
    Kind:        "Pod",
    Name:        "test-pod",
    Reason:      "FileAccess",
    MessageHash: dedup.HashMessage("test message"),
}

content := map[string]interface{}{
    "spec": map[string]interface{}{
        "source":   "falco",
        "severity": "HIGH",
        // ... other fields
    },
}

if deduper.ShouldCreateWithContent(key, content) {
    // Create observation
}

// Cleanup on shutdown
defer deduper.Stop()
```

## Configuration

Configuration via environment variables:

- `DEDUP_WINDOW_BY_SOURCE`: JSON map of source -> window seconds (e.g., `{"falco": 300, "default": 60}`)
- `DEDUP_BUCKET_SIZE_SECONDS`: Size of each time bucket (default: 10% of window, min 10s)
- `DEDUP_MAX_RATE_PER_SOURCE`: Maximum events per second per source (default: 100)
- `DEDUP_RATE_BURST`: Burst capacity (default: 2x rate limit)
- `DEDUP_ENABLE_AGGREGATION`: Enable event aggregation (default: true)

## Strategies

The package supports multiple deduplication strategies via `dedup.GetStrategy()`:

- **fingerprint** (default): Content-based fingerprinting
- **event-stream**: Shorter windows for high-volume sources
- **key**: Field-based deduplication

## Thread Safety

All methods are thread-safe and can be called concurrently. The implementation uses fine-grained locking (RLock for reads, Lock for writes) to optimize concurrent performance.

## Recent Improvements

- **Deadlock Fix**: Fixed double-locking issue in `ShouldCreateWithContent` method
- **Idempotent Stop**: `Stop()` method is now idempotent using `sync.Once`
- **Fingerprint Expiration**: Expired fingerprints are automatically removed during duplicate checks
- **Cache Key Selection**: Uses fingerprint as cache key for fingerprint-based dedup, key for key-based dedup
- **Test Coverage**: All 12 tests passing, including LRU eviction, fingerprint strategy, and event stream strategy

