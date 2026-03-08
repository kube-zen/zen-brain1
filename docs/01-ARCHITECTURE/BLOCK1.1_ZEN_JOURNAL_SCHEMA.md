# Block 1.1: ZenJournal Schema Definition

**Status:** 🚧 IN PROGRESS
**Date:** 2026-03-07
**Prerequisites:** Block 0.5 (SDK Preparation) ✅ COMPLETE

---

## Overview

This document defines the immutable event ledger schema for ZenJournal, building on zen-sdk/pkg/receiptlog. The schema provides cryptographic integrity verification, efficient querying, and archival for SR&ED evidence collection.

---

## 1. EventBlock Schema (Protobuf Definition)

### 1.1 Core Structure

```protobuf
syntax = "proto3";

package zen.brain.v1;

import "google/protobuf/timestamp.proto";

// EventBlock is the atomic unit of the journal ledger.
// Each block contains multiple events and cryptographic chain links.
message EventBlock {
  // BlockHeader contains cryptographic metadata for tamper detection
  BlockHeader header = 1;

  // Events are the journal entries in this block
  repeated EventEntry events = 2;

  // MerkleRoot is the root hash of all events in this block
  bytes merkle_root = 3;

  // BlockHash is SHA256(header | merkle_root | prev_block_hash)
  bytes block_hash = 4;

  // Timestamp when block was finalized
  google.protobuf.Timestamp finalized_at = 5;
}

// BlockHeader contains metadata for the event block
message BlockHeader {
  // PreviousBlockHash links to previous block (chain integrity)
  bytes prev_block_hash = 1;

  // BlockNumber is monotonically increasing block sequence
  uint64 block_number = 2;

  // BlockSize is the total size of this block in bytes
  uint64 block_size = 3;

  // EventCount is the number of events in this block
  uint32 event_count = 4;

  // ClusterID for multi-cluster context
  string cluster_id = 5;

  // ProjectID for project context
  string project_id = 6;

  // Version is the schema version for compatibility
  string version = 7;
}

// EventEntry represents a single journal event
message EventEntry {
  // EventType is the type of event (see EventTypes below)
  string event_type = 1;

  // Actor is who/what caused the event
  string actor = 2;

  // CorrelationID links related events (session/task ID)
  string correlation_id = 3;

  // TaskID for task-specific events
  string task_id = 4;

  // SessionID for session-specific events
  string session_id = 5;

  // Timestamp when the event occurred
  google.protobuf.Timestamp timestamp = 6;

  // Payload is event-specific data (JSON marshaled)
  bytes payload = 7;

  // SREDTags for SR&ED categorization
  repeated string sred_tags = 8;

  // EntryHash is SHA256 of this event entry
  bytes entry_hash = 9;
}
```

---

## 2. Event Types

### 2.1 Intent Events

| EventType | Description | Required Fields | Typical Actor |
|-----------|-------------|-----------------|---------------|
| `intent_created` | New work intent received from Office | `correlation_id`, `payload: WorkItem` | `jira:connector` |
| `intent_analyzed` | Intent analyzed by Analyzer | `task_id`, `payload: AnalysisResult` | `analyzer` |

### 2.2 Planning Events

| EventType | Description | Required Fields | Typical Actor |
|-----------|-------------|-----------------|---------------|
| `plan_generated` | Planner generated execution plan | `task_id`, `payload: Plan` | `planner` |
| `plan_approved` | Plan approved by human gatekeeper | `task_id`, `payload: Approval` | `human:alice` |
| `plan_rejected` | Plan rejected by human gatekeeper | `task_id`, `payload: Rejection` | `human:alice` |

### 2.3 Execution Events

| EventType | Description | Required Fields | Typical Actor |
|-----------|-------------|-----------------|---------------|
| `task_queued` | Task queued for execution | `task_id` | `dispatcher` |
| `task_started` | Task execution started | `task_id`, `session_id` | `worker:pod-123` |
| `action_executed` | Specific action performed by agent | `task_id`, `payload: Action` | `agent:code-gen` |
| `task_completed` | Task execution completed | `task_id`, `payload: Result` | `worker:pod-123` |
| `task_failed` | Task execution failed | `task_id`, `payload: Error` | `worker:pod-123` |

### 2.4 Approval Events

| EventType | Description | Required Fields | Typical Actor |
|-----------|-------------|-----------------|---------------|
| `approval_requested` | Human approval requested | `task_id`, `payload: ApprovalRequest` | `agent:planner` |
| `approval_granted` | Human approval granted | `task_id` | `human:alice` |
| `approval_denied` | Human approval denied | `task_id`, `payload: DenialReason` | `human:alice` |

### 2.5 Agent Events

| EventType | Description | Required Fields | Typical Actor |
|-----------|-------------|-----------------|---------------|
| `agent_heartbeat` | Agent heartbeat (liveness check) | `session_id`, `payload: Heartbeat` | `worker:pod-123` |
| `session_started` | Session created | `session_id`, `correlation_id` | `factory` |
| `session_ended` | Session terminated | `session_id`, `payload: SessionSummary` | `factory` |

### 2.6 Policy Events

| EventType | Description | Required Fields | Typical Actor |
|-----------|-------------|-----------------|---------------|
| `policy_violation` | Policy violation detected | `correlation_id`, `payload: Violation` | `zen-gate` |
| `gate_enforced` | Gate decision made (allow/deny) | `correlation_id`, `payload: GateDecision` | `zen-gate` |

### 2.7 SR&ED Experiment Events (V6)

| EventType | Description | Required Fields | Typical Actor |
|-----------|-------------|-----------------|---------------|
| `hypothesis_formulated` | Agent formulated hypothesis | `task_id`, `sred_tags: ["u1"]`, `payload: Hypothesis` | `planner` |
| `approach_attempted` | Agent attempted specific approach | `task_id`, `sred_tags: ["u2"]`, `payload: Approach` | `agent:code-gen` |
| `result_observed` | Agent observed results | `task_id`, `payload: Observation` | `agent:test-runner` |
| `approach_abandoned` | Agent abandoned an approach | `task_id`, `payload: Abandonment` | `agent:debugger` |
| `experiment_concluded` | Experiment concluded with findings | `task_id`, `payload: ExperimentSummary` | `planner` |

---

## 3. Query API Design

### 3.1 QueryOptions (Existing in pkg/journal/interface.go)

```go
type QueryOptions struct {
    // EventType filters by event type
    EventType EventType `json:"event_type,omitempty"`

    // CorrelationID filters by correlation ID
    CorrelationID string `json:"correlation_id,omitempty"`

    // TaskID filters by task ID
    TaskID string `json:"task_id,omitempty"`

    // SessionID filters by session ID
    SessionID string `json:"session_id,omitempty"`

    // ClusterID filters by cluster ID
    ClusterID string `json:"cluster_id,omitempty"`

    // ProjectID filters by project ID
    ProjectID string `json:"project_id,omitempty"`

    // SREDTag filters by SR&ED tag
    SREDTag contracts.SREDTag `json:"sred_tag,omitempty"`

    // Start filters events after this time
    Start time.Time `json:"start,omitempty"`

    // End filters events before this time
    End time.Time `json:"end,omitempty"`

    // Limit limits the number of results
    Limit int `json:"limit,omitempty"`

    // OrderBy specifies sort order ("asc" or "desc", default "desc")
    OrderBy string `json:"order_by,omitempty"`
}
```

### 3.2 Query Strategy Implementation

#### Indexing Strategy

For efficient querying, we need in-memory indexes for commonly queried fields:

```go
type QueryIndex struct {
    // ByEventType: map[eventType][]Sequence
    ByEventType map[EventType][]uint64

    // ByCorrelationID: map[correlationID][]Sequence
    ByCorrelationID map[string][]uint64

    // ByTaskID: map[taskID][]Sequence
    ByTaskID map[string][]uint64

    // BySessionID: map[sessionID][]Sequence
    BySessionID map[string][]uint64

    // BySREDTag: map[sredTag][]Sequence
    BySREDTag map[contracts.SREDTag][]uint64

    // ByTimestamp: sorted slice for time range queries
    ByTimestamp []TimeRangeEntry

    mu sync.RWMutex
}

type TimeRangeEntry struct {
    Sequence   uint64
    Timestamp  time.Time
    EventType  EventType
}
```

#### Query Algorithms

**By Event Type:**
```go
func (idx *QueryIndex) QueryByEventType(eventType EventType) []uint64 {
    idx.mu.RLock()
    defer idx.mu.RUnlock()
    sequences, exists := idx.ByEventType[eventType]
    if !exists {
        return nil
    }
    return append([]uint64{}, sequences...) // Return copy
}
```

**By Correlation ID (O(log n) with binary search on timestamp):**
```go
func (idx *QueryIndex) QueryByCorrelation(correlationID string, start, end time.Time) []uint64 {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    sequences, exists := idx.ByCorrelationID[correlationID]
    if !exists {
        return nil
    }

    // Filter by time range if specified
    if !start.IsZero() || !end.IsZero() {
        filtered := []uint64{}
        for _, seq := range sequences {
            // Get receipt to check timestamp
            // This is O(n) for time-filtered queries
            // Optimization: Add timestamp to index entry
            filtered = append(filtered, seq)
        }
        return filtered
    }

    return append([]uint64{}, sequences...)
}
```

**By SR&ED Tag:**
```go
func (idx *QueryIndex) QueryBySREDTag(tag contracts.SREDTag, start, end time.Time) []uint64 {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    sequences, exists := idx.BySREDTag[tag]
    if !exists {
        return nil
    }

    // Time filtering similar to correlation ID queries
    return idx.filterByTime(sequences, start, end)
}
```

---

## 4. Compaction and Archival Strategy

### 4.1 Tiered Storage Model

```
┌─────────────────────────────────────────────────────────┐
│                   ZenJournal                      │
│                                                  │
│  ┌───────────────────────────────────────────┐    │
│  │ Tier 1: Hot (Last 7 days)           │    │
│  │ - Full query index                    │    │
│  │ - All fields indexed                  │    │
│  │ - Fast queries (<10ms)               │    │
│  │ - Spool: /data/journal/hot/          │    │
│  └───────────────────────────────────────────┘    │
│                       │                           │
│                       │ After 7 days              │
│                       ▼                           │
│  ┌───────────────────────────────────────────┐    │
│  │ Tier 2: Warm (7-30 days)          │    │
│  │ - Reduced index (key fields only)     │    │
│  │ - Compressed spool                  │    │
│  │ - Slower queries (~50ms)            │    │
│  │ - Spool: /data/journal/warm/        │    │
│  └───────────────────────────────────────────┘    │
│                       │                           │
│                       │ After 30 days             │
│                       ▼                           │
│  ┌───────────────────────────────────────────┐    │
│  │ Tier 3: Cold (>30 days)             │    │
│  │ - No index                         │    │
│  │ - Archived in S3/MinIO             │    │
│  │ - Query by hash only (~100ms)         │    │
│  │ - S3: s3://zen-brain-journal/      │    │
│  └───────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

### 4.2 Compaction Triggers

```go
type CompactionConfig struct {
    // HotRetentionDays: Keep full index for N days
    HotRetentionDays int `json:"hot_retention_days"` // Default: 7

    // WarmRetentionDays: Keep reduced index for N days
    WarmRetentionDays int `json:"warm_retention_days"` // Default: 30

    // CompactionInterval: Run compaction every N hours
    CompactionInterval time.Duration `json:"compaction_interval"` // Default: 6h

    // MinBlocksToCompact: Minimum blocks before compaction
    MinBlocksToCompact int `json:"min_blocks_to_compact"` // Default: 10
}
```

### 4.3 Compaction Process

**Step 1: Identify Tiers to Compact**

```go
func (j *receiptlogJournal) compact(ctx context.Context) error {
    now := time.Now()

    // Hot tier: Last 7 days
    hotCutoff := now.AddDate(0, 0, -j.config.HotRetentionDays)

    // Warm tier: 7-30 days
    warmCutoff := now.AddDate(0, 0, -j.config.WarmRetentionDays)

    // Compact warm to cold
    if shouldCompact(j.lastCompaction, warmCutoff) {
        if err := j.compactWarmToCold(ctx); err != nil {
            return fmt.Errorf("failed to compact warm to cold: %w", err)
        }
        j.lastCompaction = now
    }

    // Compact hot to warm
    if shouldCompact(j.lastWarmCompaction, hotCutoff) {
        if err := j.compactHotToWarm(ctx); err != nil {
            return fmt.Errorf("failed to compact hot to warm: %w", err)
        }
        j.lastWarmCompaction = now
    }

    return nil
}
```

**Step 2: Hot to Warm Compaction**

```go
func (j *receiptlogJournal) compactHotToWarm(ctx context.Context) error {
    // 1. Find receipts older than hot retention
    cutoff := time.Now().AddDate(0, 0, -j.config.HotRetentionDays)
    receipts, err := j.findReceiptsOlderThan(cutoff)
    if err != nil {
        return err
    }

    if len(receipts) == 0 {
        return nil // Nothing to compact
    }

    // 2. Move to warm spool (compressed)
    warmFile := filepath.Join(j.warmDir, fmt.Sprintf("warm-%s.ndjson.gz", time.Now().Format("20060102")))
    if err := j.writeCompressed(warmFile, receipts); err != nil {
        return err
    }

    // 3. Create reduced index (key fields only)
    j.createReducedIndex(receipts)

    // 4. Remove from hot spool
    for _, receipt := range receipts {
        j.hotIndex.Remove(receipt.Sequence)
        j.removeFromHotSpool(receipt.Sequence)
    }

    log.Printf("Compacted %d receipts from hot to warm", len(receipts))
    return nil
}
```

**Step 3: Warm to Cold Compaction**

```go
func (j *receiptlogJournal) compactWarmToCold(ctx context.Context) error {
    // 1. Find receipts older than warm retention
    cutoff := time.Now().AddDate(0, 0, -j.config.WarmRetentionDays)
    receipts, err := j.findReceiptsOlderThan(cutoff)
    if err != nil {
        return err
    }

    if len(receipts) == 0 {
        return nil
    }

    // 2. Upload to S3/MinIO
    s3Key := fmt.Sprintf("journal/%s/receipts-%s.ndjson.gz",
        time.Now().Format("2006/01"),
        time.Now().Format("20060102"))
    if err := j.uploadToS3(ctx, s3Key, receipts); err != nil {
        return err
    }

    // 3. Delete from warm spool and index
    for _, receipt := range receipts {
        j.warmIndex.Remove(receipt.Sequence)
        j.removeFromWarmSpool(receipt.Sequence)
    }

    log.Printf("Compacted %d receipts from warm to cold (S3: %s)", len(receipts), s3Key)
    return nil
}
```

### 4.4 S3 Object Layout

```
s3://zen-brain-journal/
├── hot/
│   ├── receipts-2026-03-07.ndjson       # Today (kept locally)
│   └── receipts-2026-03-08.ndjson
├── warm/
│   ├── warm-2026-02-28.ndjson.gz      # Compressed, indexed
│   └── warm-2026-03-01.ndjson.gz
└── cold/
    ├── 2026/01/receipts-2026-01-15.ndjson.gz  # Archived
    ├── 2026/01/receipts-2026-01-31.ndjson.gz
    └── 2026/02/receipts-2026-02-28.ndjson.gz
```

---

## 5. SR&ED Evidence Query Interface

### 5.1 SR&ED Query API

```go
// SR&ED query options for funding report generation
type SREDQueryOptions struct {
    // Tags filters by SR&ED uncertainty tags (u1, u2, u3, u4, general)
    Tags []contracts.SREDTag `json:"tags"`

    // TaskID filters by specific task
    TaskID string `json:"task_id,omitempty"`

    // Start and End for time range queries
    Start time.Time `json:"start"`
    End   time.Time `json:"end"`

    // ProjectID filters by project
    ProjectID string `json:"project_id,omitempty"`

    // ClusterID filters by cluster
    ClusterID string `json:"cluster_id,omitempty"`

    // EvidenceClass filters by evidence type (experiment_card, benchmark_run, etc.)
    EvidenceClass string `json:"evidence_class,omitempty"`
}

// SREDEvidence represents aggregated SR&ED evidence
type SREDEvidence struct {
    // Tag is the SR&ED uncertainty category
    Tag contracts.SREDTag `json:"tag"`

    // Events are the journal entries with this tag
    Events []journal.Receipt `json:"events"`

    // TimeRange of this evidence
    TimeRange TimeRange `json:"time_range"`

    // TaskCount is number of experimental tasks
    TaskCount int `json:"task_count"`

    // TotalCost is the sum of eligible costs
    TotalCost float64 `json:"total_cost_usd"`
}

type TimeRange struct {
    Start time.Time `json:"start"`
    End   time.Time `json:"end"`
}
```

### 5.2 SR&ED Query Methods

```go
// ZenJournal interface additions
type ZenJournal interface {
    // ... existing methods ...

    // QuerySRED retrieves SR&ED evidence with filters
    QuerySRED(ctx context.Context, opts SREDQueryOptions) (*SREDEvidence, error)

    // GetExperimentTimeline retrieves full experiment chain for a task
    GetExperimentTimeline(ctx context.Context, taskID string) ([]journal.Receipt, error)
}
```

---

## 6. Implementation Tasks

### Phase 1: Protobuf Schema (Immediate)
- [ ] Create `api/v1alpha1/journal.proto`
- [ ] Generate Go code: `protoc --go_out=. api/v1alpha1/journal.proto`
- [ ] Add Protobuf dependency to go.mod

### Phase 2: Query Index (Immediate)
- [ ] Implement QueryIndex struct
- [ ] Add index updates to Record() method
- [ ] Implement query methods (ByEventType, ByCorrelationID, ByTaskID, etc.)
- [ ] Add unit tests for query methods

### Phase 3: Compaction (Short-term)
- [ ] Implement Hot/Warm/Cold tiered storage
- [ ] Implement compaction triggers
- [ ] Implement hot-to-warm compaction
- [ ] Implement warm-to-cold compaction (S3 upload)
- [ ] Add compaction tests

### Phase 4: SR&ED Queries (Medium-term)
- [ ] Implement SREDQueryOptions
- [ ] Implement SREDEvidence aggregation
- [ ] Add GetExperimentTimeline method
- [ ] Add SR&ED query tests

---

## 7. Acceptance Criteria

- [ ] Protobuf schema defined and generated
- [ ] Query API fully implemented with O(log n) lookups
- [ ] Compaction strategy implemented with 3-tier storage
- [ ] SR&ED query interface implemented
- [ ] All tests passing
- [ ] Documentation updated

---

## 8. Files to Create

- `api/v1alpha1/journal.proto` - Protobuf schema
- `api/v1alpha1/journal.pb.go` - Generated Go code
- `internal/journal/receiptlog/query_index.go` - Query index implementation
- `internal/journal/receiptlog/compaction.go` - Compaction logic
- `internal/journal/receiptlog/sred_query.go` - SR&ED queries

---

## References

- V6 Construction Plan: Section 2.1, Section 3.13
- zen-sdk/pkg/receiptlog: Core ledger implementation
- pkg/journal/interface.go: ZenJournal interface
- Block 5.4: Funding Evidence Aggregator (uses SR&ED queries)
