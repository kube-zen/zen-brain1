# ZenJournal Design

## Overview

ZenJournal is the immutable event ledger that records all significant actions in Zen‑Brain. Each entry is cryptographically linked to the previous entry via chain hashes, enabling tamper detection and efficient state verification.

**Key properties:**

- **Immutable** – once recorded, entries cannot be modified or deleted.
- **Append‑only** – new entries are appended to the end of the chain.
- **Cryptographically linked** – each entry includes the hash of the previous entry (Merkle‑style chain).
- **Efficient verification** – the entire chain can be verified by checking hash links.
- **Queryable** – entries can be filtered by type, correlation ID, task, session, cluster, project, SR&ED tag, and time range.

ZenJournal serves as the **single source of truth** for what happened in the system, critical for audit trails, debugging, and SR&ED evidence collection.

## Interface

The `ZenJournal` interface (`pkg/journal/interface.go`) defines the operations:

```go
type ZenJournal interface {
    // Record records a new journal entry and returns the receipt.
    Record(ctx context.Context, entry Entry) (*Receipt, error)

    // Retrieval
    Get(ctx context.Context, sequence uint64) (*Receipt, error)
    GetByHash(ctx context.Context, hash string) (*Receipt, error)

    // Querying
    Query(ctx context.Context, opts QueryOptions) ([]Receipt, error)
    QueryByCorrelation(ctx context.Context, correlationID string) ([]Receipt, error)
    QueryByTask(ctx context.Context, taskID string) ([]Receipt, error)
    QueryBySREDTag(ctx context.Context, tag contracts.SREDTag, start, end time.Time) ([]Receipt, error)

    // Integrity
    Verify(ctx context.Context) (int, error)  // returns number of verified receipts

    // Monitoring
    Stats() Stats

    // Cleanup
    Close() error
}
```

## Data Structures

### Entry

An event to be recorded:

- `EventType` – type of event (see standard event types below).
- `Actor` – who/what caused the event (e.g., `"planner‑v1"`, `"worker‑123"`, `"human:alice"`).
- `CorrelationID` – links related events (e.g., a session or task ID).
- `TaskID`, `SessionID`, `ClusterID`, `ProjectID` – context identifiers.
- `Payload` – event‑specific data (structured, serializable).
- `SREDTags` – SR&ED uncertainty categories (`u1_dynamic_provisioning`, etc.).
- `Timestamp` – when the event occurred.

### Receipt

A recorded entry with chain metadata:

- Contains all `Entry` fields.
- `Sequence` – monotonically increasing sequence number.
- `Hash` – SHA‑256 hash of this receipt (includes previous hash).
- `PrevHash` – hash of the previous receipt (chain link).
- `RecordedAt` – when the receipt was recorded.

### QueryOptions

Parameters for searching receipts:

- `EventType`, `CorrelationID`, `TaskID`, `SessionID`, `ClusterID`, `ProjectID`
- `SREDTag` – filter by SR&ED uncertainty category.
- `Start`, `End` – time range.
- `Limit`, `OrderBy`

## Event Types

ZenJournal defines a comprehensive set of event types covering the entire workflow:

### Intent Events
- `intent_created` – work item created in external system (Jira, Linear, etc.).
- `intent_analyzed` – Office has analyzed the intent and generated a canonical WorkItem.

### Planning Events
- `plan_generated` – Planner has generated an execution plan.
- `plan_approved` – Plan approved by human or automated gate.
- `plan_rejected` – Plan rejected.

### Execution Events
- `task_queued` – Task queued for execution.
- `task_started` – Task started by a worker.
- `action_executed` – Worker performed a specific action (e.g., code change, test run).
- `task_completed` – Task completed successfully.
- `task_failed` – Task failed.

### Approval Events
- `approval_requested` – Human approval requested.
- `approval_granted` – Approval granted.
- `approval_denied` – Approval denied.

### Agent Events
- `agent_heartbeat` – Periodic heartbeat from an agent.
- `session_started` – Agent session started.
- `session_ended` – Agent session ended.

### Policy Events
- `policy_violation` – ZenPolicy violation detected.
- `gate_enforced` – ZenGate enforced a rule.

### SR&ED Experiment Events (V6)
- `hypothesis_formulated` – Agent formulated a hypothesis about approach.
- `approach_attempted` – Agent attempted a specific approach.
- `result_observed` – Agent observed results from an approach.
- `approach_abandoned` – Agent abandoned an approach after testing.
- `experiment_concluded` – Agent concluded experimental work with findings.

These experiment‑class events are **automatically generated** when SR&ED mode is enabled (default), providing direct evidence for funding claims.

## Implementation

ZenJournal is built on **`zen‑sdk/pkg/receiptlog`**, which provides the core append‑only ledger with chain hashes.

### Storage Backend

**Primary:** Local SSD with periodic backup to S3‑compatible object storage.

- **Active journal** – SQLite database with `receiptlog` table (managed by `zen‑sdk/pkg/receiptlog`).
- **Backup** – every 5 minutes, new receipts are uploaded to S3/MinIO as compressed delta files.
- **Archival** – after 30 days, receipts are moved to cold storage (Tier 3) and removed from the active database.

### Hash Chain Details

Each receipt’s hash is computed as:

```
hash = SHA‑256(
    prev_hash +
    sequence +
    event_type +
    actor +
    correlation_id +
    task_id +
    session_id +
    cluster_id +
    project_id +
    payload_json +
    sred_tags_json +
    timestamp +
    recorded_at
)
```

The `prev_hash` field creates the immutable chain. Tampering with any receipt breaks the chain, which is detected by `Verify()`.

### Multi‑cluster Considerations

- Each cluster maintains its own **local ZenJournal shard**.
- The control plane periodically **aggregates shards** for cross‑project queries, SR&ED reporting, and Board of Directors sessions.
- Aggregation interval: every 5 minutes (configurable).
- Global queries are served from the aggregated view; local queries use the shard.

### SR&ED Integration

When `SREDDisabled` is `false` (the default), the Planner agent automatically generates experiment‑class events:

1. `hypothesis_formulated` – when a task addresses an uncertainty area.
2. `approach_attempted` – for each attempted approach.
3. `result_observed` – for each result.
4. `approach_abandoned` – if an approach is abandoned.
5. `experiment_concluded` – when the experimental work concludes.

These events include `SREDTags` linking them to specific uncertainty categories (`u1`‑`u4`, `experimental_general`).

## Query Patterns

### By Task

```go
receipts, err := journal.QueryByTask(ctx, "task‑abc123")
```

Used by the ReMe protocol to reconstruct task state.

### By SR&ED Tag

```go
receipts, err := journal.QueryBySREDTag(ctx,
    contracts.SREDU1DynamicProvisioning,
    startTime,
    endTime,
)
```

Used by the Funding Evidence Aggregator (Block 5.4) to generate SR&ED reports.

### By Correlation ID

```go
receipts, err := journal.QueryByCorrelation(ctx, "session‑xyz789")
```

Used to trace all events belonging to a session.

### Complex Queries

```go
opts := QueryOptions{
    EventType: EventActionExecuted,
    ClusterID: "local‑machine‑1",
    ProjectID: "zen‑brain",
    Start:     startTime,
    End:       endTime,
    Limit:     100,
    OrderBy:   "desc",
}
receipts, err := journal.Query(ctx, opts)
```

## Configuration

Example `config.yaml` snippet:

```yaml
journal:
  # Active storage
  database:
    path: "~/.zen‑brain/journal.db"  # Overridden by ZEN_BRAIN_HOME
    max_size_mb: 1024

  # Backup
  backup:
    enabled: true
    interval_seconds: 300
    s3:
      bucket: "zen‑brain‑journals"
      prefix: "{clusterID}/"
      endpoint: "minio‑service:9000"
      access_key: "${MINIO_ACCESS_KEY}"
      secret_key: "${MINIO_SECRET_KEY}"

  # Archival
  archival:
    enabled: true
    after_days: 30
    s3:
      bucket: "zen‑brain‑archives"
      prefix: "journals/{year}/{month}/"

  # Multi‑cluster aggregation
  aggregation:
    enabled: true
    interval_seconds: 300
    control_plane_endpoint: "https://control‑plane:8080"
```

## Monitoring

**Metrics (Prometheus):**

- `zen_journal_receipts_total` – total receipts recorded.
- `zen_journal_record_latency_seconds` – histogram for `Record()` latency.
- `zen_journal_query_latency_seconds` – histogram for query latency.
- `zen_journal_chain_verification_total` – number of chain verifications.
- `zen_journal_backup_operations_total` – backup operations.

**Dashboards (Grafana):**

- Receipts per second (by event type).
- Chain verification status.
- Backup lag (time since last successful backup).
- Storage usage.

## Integration Points

- **All components** – record events for significant actions.
- **ZenContext ReMe protocol** – queries journal to reconstruct session state.
- **Funding Evidence Aggregator** – queries by SR&ED tag to generate reports.
- **Observability stack** – journal events can be exported to tracing systems.

## Open Questions

1. **Should we support event schema evolution?** – Probably; use protobuf or Avro for `Payload` with versioning.
2. **How to handle very high write throughput?** – `receiptlog` supports batching; we can batch writes every 100ms.
3. **Should journal events be published to a message bus?** – Yes, `Record()` can also publish to NATS/Redis Streams for real‑time consumers.

## Next Steps

1. Implement `internal/journal/receiptlog.go` – wraps `zen‑sdk/pkg/receiptlog`.
2. Add SQLite persistence for active journal.
3. Implement S3 backup and archival.
4. Write unit and integration tests.
5. Integrate with ZenContext ReMe protocol.

---

*This document is a living design spec; update as implementation progresses.*