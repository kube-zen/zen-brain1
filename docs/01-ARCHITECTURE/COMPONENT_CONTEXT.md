# Block 1.2: ZenContext Architecture Design

**Status:** 🚧 IN PROGRESS
**Date:** 2026-03-08
**Prerequisites:** Block 0.5 (SDK Preparation) ✅ COMPLETE
**Prerequisites:** Block 1.1 (ZenJournal Schema) 🚧 IN PROGRESS

---

## Overview

ZenContext is the tiered memory system that enables AI agents to retrieve relevant historical information, pick up where they left off (ReMe), and maintain context across long-running tasks. It provides three tiers of storage optimized for different access patterns:

- **Tier 1 (Hot):** Sub-millisecond access for active session state
- **Tier 2 (Warm):** Fast knowledge and procedure lookups via vector similarity
- **Tier 3 (Cold):** Archival storage for long-term retention

---

## 1. Tier 1: Hot Memory (Active Sessions)

### 1.1 Purpose

Provide sub-millisecond access to session context for:
- Agent state and intermediate reasoning
- Scratchpad for temporary computations
- Active task tracking
- Heartbeat and liveness monitoring

### 1.2 Storage Options

| Option | Access Latency | Capacity | Use Case |
|---------|----------------|----------|-----------|
| **Redis** (Primary) | 1-5 ms | 1-10 GB per session | Production with persistence |
| **tmpfs** (Secondary) | <1 ms | 512 MB per pod | Distributed agent pods |

**Architecture:**

```
┌─────────────────────────────────────────────────┐
│           Agent Pod (Worker)              │
│                                             │
│  ┌────────────┐    ┌──────────────┐   │
│  │  Redis     │    │   tmpfs       │   │
│  │  (Hot)    │    │  (Scratchpad) │   │
│  └────────────┘    └──────────────┘   │
│         │                  │              │
│         │                  │              │
│         └─────────┬────────┘              │
│                   │                         │
│                   ▼                         │
│           ZenContext API                  │
└─────────────────────────────────────────────────┘
```

### 1.3 Redis Key Patterns

**Key Namespace:** `zen:ctx:{cluster_id}:{session_id}`

| Data Type | Key Pattern | Value Type | TTL |
|-----------|-------------|-------------|-------|
| Session State | `ctx:{session_id}:state` | JSON (SessionContext) | Session timeout (default: 30 min) |
| Scratchpad | `ctx:{session_id}:scratchpad` | Binary (intermediate reasoning) | Session timeout |
| Task List | `ctx:{session_id}:tasks` | JSON (array of task IDs) | Session timeout |
| Metadata | `ctx:{session_id}:meta` | JSON (session metadata) | Session timeout |
| Heartbeat | `ctx:{session_id}:heartbeat` | String (ISO timestamp) | 1 minute |
| Lock | `ctx:{session_id}:lock` | String (owner: timestamp) | Session timeout |

**Example Keys:**
```
zen:ctx:cluster-1:session-abc123:state
zen:ctx:cluster-1:session-abc123:scratchpad
zen:ctx:cluster-1:session-abc123:tasks
zen:ctx:cluster-1:session-abc123:meta
zen:ctx:cluster-1:session-abc123:heartbeat
zen:ctx:cluster-1:session-abc123:lock
```

### 1.4 SessionContext Schema

```go
type SessionContext struct {
    // SessionID uniquely identifies the session
    SessionID string `json:"session_id"`

    // TaskID identifies the current task
    TaskID string `json:"task_id,omitempty"`

    // ClusterID for multi-cluster context
    ClusterID string `json:"cluster_id,omitempty"`

    // ProjectID for project context
    ProjectID string `json:"project_id,omitempty"`

    // CreatedAt is when the session was created
    CreatedAt time.Time `json:"created_at"`

    // LastAccessedAt is when the session was last accessed
    LastAccessedAt time.Time `json:"last_accessed_at"`

    // State is the current agent state (serialized)
    State []byte `json:"state,omitempty"`

    // RelevantKnowledge contains retrieved KB chunks for this session
    RelevantKnowledge []KnowledgeChunk `json:"relevant_knowledge,omitempty"`

    // Scratchpad contains intermediate reasoning (Tier 1 only)
    Scratchpad []byte `json:"scratchpad,omitempty"`

    // AgentType is the type of agent (planner, code-gen, test-runner, etc.)
    AgentType string `json:"agent_type,omitempty"`

    // AgentPod is the Kubernetes pod name
    AgentPod string `json:"agent_pod,omitempty"`

    // StepNumber is the current step in the workflow
    StepNumber int `json:"step_number,omitempty"`

    // TotalSteps is the total number of steps
    TotalSteps int `json:"total_steps,omitempty"`
}
```

### 1.5 Tmpfs Configuration

**Mount Point:** `/dev/shm/agent`

**Size Limit:** 512 MB per pod

**Directory Structure:**
```
/dev/shm/agent/
├── scratchpad.bin          # Intermediate reasoning (up to 256 MB)
├── state.json             # Current agent state
├── tasks.json             # Active task list
└── heartbeat.txt           # Last heartbeat timestamp
```

**Volume Mount (Kubernetes):**
```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: agent
    volumeMounts:
    - name: scratch
      mountPath: /dev/shm/agent
  volumes:
    - name: scratch
      emptyDir:
        medium: Memory  # Uses tmpfs
        sizeLimit: 512Mi
```

---

## 2. Tier 2: Warm Memory (Knowledge Base)

### 2.1 Purpose

Provide fast knowledge and procedure lookups via vector similarity:
- Retrieve relevant documentation for tasks
- Find similar problems and solutions
- Access procedural knowledge (runbooks, playbooks)

### 2.2 Storage: Vector Database (QMD)

**Database:** CockroachDB with C-SPANN vector index

**Embedding Model:** nomic-embed-text (768 dimensions)

**Collection:** `kb_chunks`

### 2.3 Schema

**Table: kb_chunks**

```sql
CREATE TABLE kb_chunks (
    -- Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- PREFIX COLUMNS (C-SPANN indexing)
    scope TEXT NOT NULL,           -- 'company', 'general', 'zen-brain', etc.
    repo TEXT NOT NULL,            -- 'zen-docs', 'zen-sdk', etc.

    -- CONTENT
    path TEXT NOT NULL,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,

    -- EMBEDDING
    embedding VECTOR(768),

    -- METADATA
    heading_path TEXT[],             -- ['Section', 'Subsection']
    token_count INT,
    file_type TEXT,                 -- 'markdown', 'go', 'yaml', etc.
    language TEXT,                  -- Programming language (if code)

    -- INDEXES
    VECTOR INDEX (scope, repo, embedding),

    -- TIMESTAMPS
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),

    -- CONSTRAINTS
    UNIQUE (scope, repo, path, chunk_index)
);
```

### 2.4 KnowledgeChunk Schema

```go
type KnowledgeChunk struct {
    // ID is the unique identifier
    ID string `json:"id"`

    // Scope is the KB scope (company, general, project)
    Scope string `json:"scope"`

    // Repo is the source repository
    Repo string `json:"repo"`

    // Path is the original file path
    Path string `json:"path"`

    // Content is the text content
    Content string `json:"content"`

    // HeadingPath is the heading hierarchy
    HeadingPath []string `json:"heading_path"`

    // TokenCount is the size in tokens
    TokenCount int `json:"token_count"`

    // FileType is the type (markdown, go, yaml, etc.)
    FileType string `json:"file_type"`

    // Language is the programming language (if code)
    Language string `json:"language,omitempty"`

    // SimilarityScore is the relevance score (0-1)
    SimilarityScore float64 `json:"similarity_score"`

    // RetrievedAt is when this chunk was retrieved
    RetrievedAt time.Time `json:"retrieved_at"`
}
```

### 2.5 Query Interface

```go
type QueryOptions struct {
    // Query is the natural language query
    Query string `json:"query"`

    // Scopes filters by scope (company, general, project)
    Scopes []string `json:"scopes,omitempty"`

    // Limit limits the number of results
    Limit int `json:"limit,omitempty"`

    // MinSimilarity filters by minimum similarity score
    MinSimilarity float64 `json:"min_similarity,omitempty"`

    // ClusterID for multi-cluster context
    ClusterID string `json:"cluster_id,omitempty"`

    // ProjectID for project context
    ProjectID string `json:"project_id,omitempty"`
}
```

### 2.6 QMD Adapter Integration

**Flow:**

```
┌─────────────────────────────────────────┐
│            ZenContext API            │
│                                         │
│         QueryKnowledge()              │
│                                         │
│                 ▼                        │
│         ┌───────────────┐               │
│         │   QMD Adapter │               │
│         └───────────────┘               │
│                 │                        │
│                 ▼                        │
│         qmd CLI (subprocess)            │
│                 │                        │
│                 ▼                        │
│         CockroachDB (QMD)              │
│                 │                        │
│                 ▼                        │
│         Parse JSON Output                │
│                                         │
│         KnowledgeChunk[]                  │
└─────────────────────────────────────────┘
```

**Adapter Pattern:** Wraps QMD CLI as subprocess (already implemented in `internal/qmd/adapter.go`)

---

## 3. Tier 3: Cold Memory (Archival)

### 3.1 Purpose

Long-term archival storage for:
- Completed sessions
- Historical context reconstruction
- Compliance and audit trails
- Cost optimization (move cold data from expensive storage)

### 3.2 Storage: S3/MinIO

**Bucket:** `zen-brain-context`

**Layout:**
```
s3://zen-brain-context/
├── sessions/
│   ├── 2026/
│   │   ├── 03/
│   │   │   ├── session-{id}-{date}.json.gz
│   │   │   └── session-{id}-{date}.json.gz
│   │   └── 04/
│   │       └── ...
│   └── 2025/
│       └── ...
├── scratchpads/
│   ├── 2026/
│   │   ├── 03/
│   │   └── 04/
│   └── 2025/
└── metadata/
    └── index.json  # Global session index
```

### 3.3 Retention Policies

| Data Type | Retention | Compressed | Notes |
|-----------|-----------|-----------|--------|
| Session Context | 90 days | Yes (gzip) | After archival, moved to S3 |
| Scratchpads | 30 days | Yes (gzip) | Larger than Redis tmpfs |
| KB Query Logs | 365 days | No | For audit and debugging |

### 3.4 Lifecycle

**State Transitions:**
```
┌─────────────┐    ┌──────────────┐    ┌──────────────┐
│  Tier 1    │───▶│   Tier 2     │───▶│   Tier 3     │
│  (Redis/   │    │  (QMD)        │    │  (S3)         │
│   tmpfs)    │    │               │    │               │
└─────────────┘    └──────────────┘    └──────────────┘
       │                   │               │               │
       │ Session ends       │ KB queries      │ Session ends    │
       ▼                   ▼               ▼               │
   Archive to Tier 3    (No change)    Upload to S3
```

---

## 4. ReMe Protocol (Recursive Memory)

### 4.1 Purpose

Enable agents to reconstruct their state when waking up after:
- Pod restarts
- Failures and crash recovery
- Scheduled re-execution
- Session resumption

### 4.2 ReMe Workflow

```
┌─────────────────────────────────────────────────────────────┐
│                   ReMe Protocol                      │
│                                                        │
│  1. ┌─────────────────┐                                 │
│     │  Agent Wakes Up │                                 │
│     └─────────────────┘                                 │
│              │                                            │
│              ▼                                            │
│  2. ┌─────────────────┐                                 │
│     │  ReMe Request   │                                 │
│     │  (session_id,    │                                 │
│     │   task_id)       │                                 │
│     └─────────────────┘                                 │
│              │                                            │
│              ▼                                            │
│  3. ┌─────────────────────────────────────────────┐     │
│     │  Query ZenJournal                          │     │
│     │  (QueryByTask or                         │     │
│     │   QueryByCorrelation)                    │     │
│     └─────────────────────────────────────────────┘     │
│              │                                            │
│              ▼                                            │
│  4. ┌─────────────────────────────────────────────┐     │
│     │  Get Journal Entries                    │     │
│     │  (all events for task/session)            │     │
│     └─────────────────────────────────────────────┘     │
│              │                                            │
│              ▼                                            │
│  5. ┌─────────────────────────────────────────────┐     │
│     │  Reconstruct Causal Chain               │     │
│     │  - Parse events in order                 │     │
│     │  - Build state transition timeline        │     │
│     │  - Identify last state                  │     │
│     └─────────────────────────────────────────────┘     │
│              │                                            │
│              ▼                                            │
│  6. ┌─────────────────────────────────────────────┐     │
│     │  Query Tier 2 (QMD)                     │     │
│     │  - Get relevant knowledge               │     │
│     │  - Retrieve procedural docs              │     │
│     │  - Find similar problems/solutions       │     │
│     └─────────────────────────────────────────────┘     │
│              │                                            │
│              ▼                                            │
│  7. ┌─────────────────────────────────────────────┐     │
│     │  Reconstruct SessionContext              │     │
│     │  - Restore state from journal           │     │
│     │  - Add knowledge from Tier 2           │     │
│     │  - Rehydrate scratchpad if available    │     │
│     └─────────────────────────────────────────────┘     │
│              │                                            │
│              ▼                                            │
│  8. ┌─────────────────────────────────────────────┐     │
│     │  Store in Tier 1 (Redis/tmpfs)        │     │
│     │  - Hot storage for fast access          │     │
│     │  - Set LastAccessedAt = now          │     │
│     │  - Start heartbeat timer              │     │
│     └─────────────────────────────────────────────┘     │
│              │                                            │
│              ▼                                            │
│  9. ┌─────────────────────────────────────────────┐     │
│     │  Verify State                           │     │
│     │  - Check state consistency               │     │
│     │  - Compare with last journal entry      │     │
│     │  - Validate against expectations        │     │
│     └─────────────────────────────────────────────┘     │
│              │                                            │
│              ▼                                            │
│  10. ┌─────────────────┐                              │
│      │  Agent Ready     │                              │
│      └─────────────────┘                              │
│                                                        │
│              ▼                                            │
│         Continue Execution                                     │
└─────────────────────────────────────────────────────────────┘
```

### 4.3 ReMeRequest Schema

```go
type ReMeRequest struct {
    // SessionID to reconstruct
    SessionID string `json:"session_id"`

    // TaskID to reconstruct
    TaskID string `json:"task_id"`

    // ClusterID for multi-cluster context
    ClusterID string `json:"cluster_id,omitempty"`

    // ProjectID for project context
    ProjectID string `json:"project_id,omitempty"`

    // UpToTime reconstructs up to this time (default: now)
    UpToTime time.Time `json:"up_to_time,omitempty"`
}
```

### 4.4 ReMeResponse Schema

```go
type ReMeResponse struct {
    // SessionContext is the reconstructed session context
    SessionContext *SessionContext `json:"session_context"`

    // JournalEntries are the relevant journal entries
    JournalEntries []interface{} `json:"journal_entries"`

    // ReconstructionMetadata contains stats about reconstruction
    ReconstructionMetadata *ReconstructionMetadata `json:"reconstruction_metadata"`
}

type ReconstructionMetadata struct {
    // EventsProcessed is the number of journal entries processed
    EventsProcessed int `json:"events_processed"`

    // KBChunksRetrieved is the number of KB chunks fetched
    KBChunksRetrieved int `json:"kb_chunks_retrieved"`

    // StateRestored is whether agent state was successfully restored
    StateRestored bool `json:"state_restored"`

    // KnowledgeInjected is whether knowledge was injected
    KnowledgeInjected bool `json:"knowledge_injected"`

    // ReconstructionDuration is how long reconstruction took
    ReconstructionDuration time.Duration `json:"reconstruction_duration_ms"`

    // ConfidenceScore is how confident the reconstruction is (0-1)
    ConfidenceScore float64 `json:"confidence_score"`
}
```

### 4.5 Confidence Scoring

**High Confidence (>0.8):**
- All events in order
- State restored successfully
- KB matches found
- Journal chain verified

**Medium Confidence (0.5-0.8):**
- Most events in order
- State partially restored
- Some KB matches found

**Low Confidence (<0.5):**
- Events missing or out of order
- State not restored
- No KB matches found
- Journal chain broken

---

## 5. Context Injection API

### 5.1 Purpose

Inject relevant context into agents before task execution:
- Retrieve procedural knowledge
- Add historical context
- Provide task-specific information

### 5.2 Injection Workflow

```
┌─────────────────────────────────────────────────┐
│         Task Scheduler / Dispatcher          │
│                                           │
│  1. ┌──────────────┐                   │
│     │  Task Arrives │                   │
│     └──────────────┘                   │
│              │                              │
│              ▼                              │
│  2. ┌─────────────────────────────┐     │
│     │  Analyze Task Requirements  │     │
│     │  - Extract keywords         │     │
│     │  - Identify scope          │     │
│     │  - Determine agent type   │     │
│     └─────────────────────────────┘     │
│              │                              │
│              ▼                              │
│  3. ┌─────────────────────────────┐     │
│     │  Query Tier 2 (QMD)      │     │
│     │  - Vector similarity       │     │
│     │  - Scope filtering        │     │
│     │  - Top N results          │     │
│     └─────────────────────────────┘     │
│              │                              │
│              ▼                              │
│  4. ┌─────────────────────────────┐     │
│     │  Select Knowledge Chunks │     │
│     │  - Relevance score        │     │
│     │  - Diversity filter       │     │
│     └─────────────────────────────┘     │
│              │                              │
│              ▼                              │
│  5. ┌─────────────────────────────┐     │
│     │  Query Tier 1 (Redis)   │     │
│     │  - Get or create session  │     │
│     │  - Check existing context │     │
│     └─────────────────────────────┘     │
│              │                              │
│              ▼                              │
│  6. ┌─────────────────────────────┐     │
│     │  Build SessionContext     │     │
│     │  - Add knowledge chunks   │     │
│     │  - Set task metadata     │     │
│     └─────────────────────────────┘     │
│              │                              │
│              ▼                              │
│  7. ┌─────────────────────────────┐     │
│     │  Store in Tier 1        │     │
│     │  - Redis set            │     │
│     │  - tmpfs write         │     │
│     └─────────────────────────────┘     │
│              │                              │
│              ▼                              │
│  8. ┌─────────────────────────────┐     │
│     │  Inject into Agent       │     │
│     │  - Environment variables   │     │
│     │  - Stdin / Sidecar      │     │
│     └─────────────────────────────┘     │
│                                           │
│              ▼                              │
│  9. ┌──────────────┐                   │
│     │  Agent Starts │                   │
│     └──────────────┘                   │
│                                           │
│              ▼                              │
│         Task Execution                       │
└─────────────────────────────────────────────────┘
```

### 5.3 Injection Methods

**Method A: Environment Variables**

```bash
# Set before agent starts
export ZEN_CONTEXT_SESSION_ID="session-abc123"
export ZEN_CONTEXT_KB_CHUNKS='[{"id":"1","content":"..."}]'
export ZEN_CONTEXT_TASK_ID="task-456"
```

**Method B: Sidecar Injection**

```yaml
# Agent pod with sidecar
apiVersion: v1
kind: Pod
spec:
  containers:
    # Agent container
    - name: agent
      image: zen-brain-agent:latest
      env:
        - name: ZEN_CONTEXT_SESSION_ID
          valueFrom:
            configMapKeyRef:
              name: zen-context
              key: session-id
    # Sidecar for context injection
    - name: context-injector
      image: zen-context-injector:latest
      command: ["/bin/inject-context"]
      volumeMounts:
        - name: zen-context
  volumes:
    - name: zen-context
      configMap:
        name: zen-context
```

**Method C: HTTP API**

```go
// Agent requests context on startup
ctx := context.Background()
sessionCtx, err := zenContext.GetSessionContext(ctx, clusterID, sessionID)
if err != nil {
    // Fallback: Start with empty context
    sessionCtx = &SessionContext{SessionID: sessionID}
}

// Inject into agent process
agent := &Agent{
    SessionContext: sessionCtx,
}
```

---

## 6. Performance Characteristics

| Operation | Tier 1 (Redis) | Tier 2 (QMD) | Tier 3 (S3) |
|-----------|------------------|-----------------|---------------|
| Get Session | 1-5 ms | N/A | 50-200 ms |
| Store Session | 1-5 ms | N/A | N/A |
| Update Scratchpad | 1-5 ms | N/A | N/A |
| Query Knowledge | N/A | 5-15 ms | N/A |
| Reconstruct State | 50-200 ms | N/A | 200-500 ms |
| Archive Session | N/A | N/A | 100-300 ms |

**Total ReMe Latency:** ~300-700 ms (Typical)

---

## 7. Multi-Cluster Considerations

### 7.1 Cluster Context

All keys include `cluster_id` prefix:

```
zen:ctx:cluster-1:session-abc123:state
zen:ctx:cluster-2:session-abc123:state
zen:ctx:local-machine:session-abc123:state
```

### 7.2 Session Affinity

- Sessions should stay on same cluster when possible
- Cluster failover triggers ReMe
- Global session index tracks session location

**Global Index (S3):**
```json
{
  "sessions": {
    "session-abc123": {
      "cluster_id": "cluster-1",
      "last_heartbeat": "2026-03-08T18:00:00Z",
      "location": "redis://cluster-1"
    }
  }
}
```

---

## 8. Error Handling and Fallbacks

### 8.1 Tier 1 Failures

**Redis Unavailable:**
- Fall back to tmpfs-only
- Log warning, continue with degraded performance

**tmpfs Full:**
- Compress scratchpad
- Spill to disk
- Log warning

### 8.2 Tier 2 Failures

**QMD Unavailable:**
- Fall back to keyword search in GitHub
- Log warning, continue with degraded results

**Vector Index Unavailable:**
- Fallback to full-text search
- Log warning, slower queries

### 8.3 Tier 3 Failures

**S3 Upload Failed:**
- Retry with exponential backoff
- Cache locally, retry later
- Log warning

**Reconstruction Failed:**
- Start with empty context
- Log reconstruction failure
- Continue with degraded state

---

## 9. Monitoring and Observability

### 9.1 Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `zen_context_session_retrieval_duration_ms` | Histogram | Time to retrieve session from Tier 1 |
| `zen_context_kb_query_duration_ms` | Histogram | Time to query Tier 2 |
| `zen_context_reme_duration_ms` | Histogram | Time for ReMe protocol |
| `zen_context_reme_confidence_score` | Gauge | Confidence score of reconstruction |
| `zen_context_tier1_hit_ratio` | Gauge | Cache hit ratio for Tier 1 |
| `zen_context_kb_similarity_score` | Histogram | Similarity scores from Tier 2 |
| `zen_context_scratchpad_size_bytes` | Gauge | Scratchpad size |

### 9.2 Logs

**Trace ID:** Each ReMe request should have a trace ID for distributed tracing

**Structured Logging:**
```json
{
  "timestamp": "2026-03-08T18:00:00Z",
  "level": "info",
  "component": "zen_context",
  "trace_id": "abc-123",
  "session_id": "session-abc123",
  "action": "reme_protocol",
  "duration_ms": 423,
  "confidence_score": 0.85,
  "kb_chunks_retrieved": 5,
  "events_processed": 12
}
```

---

## 10. Implementation Tasks

### Phase 1: Core Infrastructure (Immediate)
- [ ] Implement Tier 1 (Redis) storage backend
- [ ] Implement Tier 2 (QMD adapter) storage backend
- [ ] Implement Tier 3 (S3) archival backend
- [ ] Add context injection API methods

### Phase 2: ReMe Protocol (Short-term)
- [ ] Implement ReMe request handler
- [ ] Implement journal query integration
- [ ] Implement KB knowledge retrieval
- [ ] Implement state reconstruction logic
- [ ] Implement confidence scoring

### Phase 3: Multi-Cluster (Medium-term)
- [ ] Add cluster_id to all keys
- [ ] Implement global session index
- [ ] Implement session affinity tracking
- [ ] Add cluster failover logic

### Phase 4: Monitoring (Ongoing)
- [ ] Add Prometheus metrics
- [ ] Add distributed tracing (OpenTelemetry)
- [ ] Add structured logging
- [ ] Add alerting rules

---

## 11. Acceptance Criteria

- [ ] Tier 1 (Hot) schema defined and documented
- [ ] Tier 2 (Warm) schema defined and documented
- [ ] Tier 3 (Cold) schema defined and documented
- [ ] ReMe protocol designed with workflow
- [ ] Context injection API designed
- [ ] Multi-cluster considerations documented
- [ ] Performance characteristics documented
- [ ] Error handling and fallbacks documented
- [ ] Monitoring and observability documented

---

## References

- V6 Construction Plan: Section 2.2, Section 3.4
- pkg/context/interface.go: ZenContext interface
- internal/qmd/adapter.go: QMD adapter implementation
- Block 1.1: ZenJournal Schema (for ReMe journal queries)
- Block 5.1: QMD Population (for Tier 2 data)
