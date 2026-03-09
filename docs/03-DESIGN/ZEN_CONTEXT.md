# ZenContext Design

## Overview

ZenContext is the tiered memory system for Zen‑Brain agents. It provides three tiers of memory with different latency/durability tradeoffs, enabling agents to retrieve relevant historical information and pick up where they left off.

**Three‑tier architecture:**

| Tier | Storage | Latency | Use Case |
|------|---------|---------|----------|
| **Tier 1 (Hot)** | Redis + tmpfs | Sub‑millisecond | Session context, scratchpad, intermediate reasoning |
| **Tier 2 (Warm)** | Vector database (QMD) | 1‑50ms | Knowledge and procedure lookups, semantic search |
| **Tier 3 (Cold)** | Object storage (S3/MinIO) | 100ms‑2s | Archival logs, long‑term evidence storage |

## Interface

The `ZenContext` interface (`pkg/context/interface.go`) defines the operations available to agents and other components:

```go
type ZenContext interface {
    // Session management (Tier 1)
    GetSessionContext(ctx context.Context, clusterID, sessionID string) (*SessionContext, error)
    StoreSessionContext(ctx context.Context, clusterID string, session *SessionContext) error
    DeleteSessionContext(ctx context.Context, clusterID, sessionID string) error

    // Knowledge retrieval (Tier 2)
    QueryKnowledge(ctx context.Context, opts QueryOptions) ([]KnowledgeChunk, error)
    StoreKnowledge(ctx context.Context, chunks []KnowledgeChunk) error  // Used by KB ingestion

    // Archival (Tier 3)
    ArchiveSession(ctx context.Context, clusterID, sessionID string) error

    // Recursive Memory (ReMe) protocol
    ReconstructSession(ctx context.Context, req ReMeRequest) (*ReMeResponse, error)

    // Monitoring
    Stats(ctx context.Context) (map[Tier]interface{}, error)

    // Cleanup
    Close() error
}
```

## Data Structures

### SessionContext

Contains the complete context for an agent session:

- `SessionID`, `TaskID`, `ClusterID`, `ProjectID`
- Timestamps (`CreatedAt`, `LastAccessedAt`)
- `State []byte` – serialized agent state (Go‑gob or JSON)
- `RelevantKnowledge []KnowledgeChunk` – retrieved KB chunks for this session
- `Scratchpad []byte` – intermediate reasoning (Tier 1 only)

### KnowledgeChunk

Represents a retrieved knowledge piece from QMD:

- `ID`, `Scope`, `Content`, `SourcePath`, `HeadingPath`
- `SimilarityScore` – relevance score (0‑1)
- `RetrievedAt` – timestamp

### QueryOptions

Parameters for knowledge retrieval:

- `Query` – natural language query
- `Scopes` – filter by scope (`company`, `general`, `project`)
- `Limit`, `MinSimilarity`
- `ClusterID`, `ProjectID` – multi‑cluster context

### ReMeRequest / ReMeResponse

Used by the **Recursive Memory** protocol to reconstruct agent state after a restart or failure.

## Tier Implementations

### Tier 1: Hot Memory (Redis + tmpfs)

**Purpose:** Provide sub‑millisecond access to session context and scratchpad.

**Implementation:**

- **Redis** (cluster‑scoped) – stores `SessionContext` objects serialized as JSON.
- **tmpfs** (pod‑local) – each agent pod mounts a `emptyDir` volume with `medium: Memory` for scratchpad.
  - Size limit: 512 MiB per pod (configurable).
  - Path: `/dev/shm/zen‑context/{sessionID}/scratchpad`

**Key‑value pattern:**

```
zen‑context:{clusterID}:{sessionID} → SessionContext (JSON)
```

**TTL:** Session contexts expire after 30 minutes of inactivity (configurable). Expired sessions are automatically archived to Tier 3.

**Scratchpad synchronization:** The agent writes scratchpad state to tmpfs; a background goroutine periodically syncs it to Redis (debounced) to survive pod restarts.

### Tier 2: Warm Memory (QMD Search)

**Purpose:** Fast semantic search over the knowledge base (`zen‑docs` repository).

**Implementation:**

- **QMD (Question‑Answer Memory Database)** – indexes the `zen‑docs` Git repository and provides semantic search via CLI.
- **Embedding model:** `nomic‑embed‑text` (768 dimensions) for local inference, or `text‑embedding‑3‑small` (1536d) for API‑based.
- **Indexing:** Performed by the **KB Ingestion Service** (Block 3.5) which runs `qmd index` on repository changes.

**Query flow:**

1. Agent calls `QueryKnowledge` with `QueryOptions`.
2. ZenContext service invokes `qmd search` with the query and scope filters.
3. QMD returns JSON results with chunk content and similarity scores.
4. Results are mapped to `KnowledgeChunk` objects and returned to the agent.

**Performance expectations:**

- < 100K vectors: 1‑5 ms
- 100K – 1M vectors: 5‑15 ms
- 1M – 1B vectors: 15‑50 ms

**Scope isolation:** QMD supports tagging chunks with scope (`company`, `general`, `project`). The KB Ingestion Service ensures proper tagging.

### Tier 3: Cold Memory (Object Storage)

**Purpose:** Archival storage for completed sessions and evidence.

**Implementation:**

- **S3‑compatible storage** (MinIO for self‑hosted, AWS S3 for cloud).
- **Path structure:** `{clusterID}/{projectID}/{year}/{month}/{sessionID}.tar.gz`
- **Contents:** Serialized `SessionContext`, attached evidence (logs, diffs, test results), ZenJournal entries for the session.

**Retention policy:** Archives are kept for 7 years (for SR&ED compliance) then automatically deleted.

**Access pattern:** Rarely read; used for auditing, compliance reporting, and the ReMe protocol when Tier 1/2 data is unavailable.

## Recursive Memory (ReMe) Protocol

**Purpose:** Enable agents to reconstruct their state when waking up (after restarts, failures, or scheduled work).

**Process:**

1. Agent calls `ReconstructSession` with `ReMeRequest`.
2. ZenContext:
   - Reads ZenJournal entries for the session (filtered by `UpToTime`).
   - Reconstructs the causal chain of events.
   - Retrieves relevant knowledge chunks from Tier 2 (if missing from Tier 1).
   - Fetches archived session data from Tier 3 (if Tier 1 is empty).
   - Returns `ReMeResponse` with reconstructed `SessionContext` and `JournalEntries`.
3. Agent verifies the reconstructed state matches expectations, then continues execution.

**Benefits:**

- Agents recover gracefully from failures.
- Full state reconstruction for debugging.
- Consistent behavior across restarts.

## Multi‑cluster Considerations

- Each cluster has its own ZenContext instance (data plane agent).
- Tier 1 (Redis) is cluster‑local.
- Tier 2 (CockroachDB) may be shared across clusters (global database) or per‑cluster (sharded).
- Tier 3 (object storage) is global; archives are tagged with `clusterID`.

All methods accept `clusterID` and `projectID` parameters for proper isolation.

## Configuration

Example `config.yaml` snippet:

```yaml
context:
  # Tier 1 (Hot)
  tier1:
    redis:
      address: "redis‑service:6379"
      password: "${REDIS_PASSWORD}"
      db: 0
    tmpfs:
      size_limit_mb: 512
      sync_interval_seconds: 5

  # Tier 2 (Warm)
  tier2_qmd:
    repo_path: "../zen-docs"
    qmd_binary_path: "qmd"
    verbose: false

  # Tier 3 (Cold)
  tier3:
    object_store:
      provider: "minio"
      endpoint: "minio‑service:9000"
      bucket: "zen‑brain‑archives"
      access_key: "${MINIO_ACCESS_KEY}"
      secret_key: "${MINIO_SECRET_KEY}"

  # Session TTL
  session_ttl_minutes: 30
  archive_after_days: 1
```

## Monitoring

**Metrics (via Prometheus):**

- `zen_context_tier1_operations_total` – counters for get/store/delete
- `zen_context_tier1_latency_seconds` – histogram for Redis operations
- `zen_context_tier2_query_latency_seconds` – vector query latency
- `zen_context_tier2_chunks_total` – total chunks indexed
- `zen_context_tier3_archive_operations_total` – archival operations
- `zen_context_sessions_active` – gauge of active sessions

**Dashboards (Grafana):**

- Tier 1 hit/miss rates
- Tier 2 query latency (p50, p95, p99)
- Tier 3 storage usage
- ReMe reconstruction success rate

## Integration Points

- **KB Ingestion Service** (Block 3.5) – calls `StoreKnowledge` to populate Tier 2.
- **ZenJournal** (Block 3.3) – ReMe protocol reads journal entries.
- **Worker Agents** (Block 4.3) – call `GetSessionContext`, `StoreSessionContext`, `QueryKnowledge`.
- **ZenLedger** (Block 3.6) – optional integration for cost attribution per session.

## Open Questions

1. **Should Tier 1 be replicated across clusters for failover?** – Probably not; sessions are cluster‑local.
2. **How to handle embedding model version upgrades?** – Need to re‑index Tier 2; plan for dual‑index during transition.
3. **Should we compress Tier 1 data?** – Redis compression may help for large state objects.

## Next Steps

1. Implement `internal/context/redis_tier1.go` – Redis + tmpfs integration.
2. Implement `internal/context/cockroach_tier2.go` – vector queries.
3. Implement `internal/context/s3_tier3.go` – archival.
4. Write unit and integration tests.
5. Integrate with KB Ingestion Service when ready.

---

*This document is a living design spec; update as implementation progresses.*