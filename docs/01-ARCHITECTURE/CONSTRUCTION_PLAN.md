> **NOTE:** This document references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

> **DEPRECATED:** This document references `zen.kube-zen.com` API groups. These are migrating to `brain.zen-mesh.io` / `platform.zen-mesh.io`. See [ADR-0010](ADR/0010_API_GROUP_MIGRATION.md).

# Zen-Brain 1.0 Construction Plan

> **Canonical Plan Location:** This is the authoritative construction plan for zen-brain 1.0.
> All future updates to the plan should be made directly to this file in the repo.
> Do not replace this file with symlinks to external downloads.

---

EOF# Zen-Brain 1.0 Construction Plan

**Version:** 6.1
**Date:** 2026-03-07
**Status:** Ready for Execution (Patched)
**Philosophy:** Build clean

## 0. What's New in V6

This version adds critical capabilities for IRAP/SR&ED alignment, multi-cluster architecture, and cost optimization.

**Key Changes from V5.2:**

1. **SR&ED Evidence Collection: Default ON** - Every session produces SR&ED-eligible records automatically. Explicit `sred_disabled: true` opt-out flag. Not a mode—the default behavior.
2. **AI Attribution in Jira** - All AI-generated content includes structured attribution headers: `[zen-brain | agent: {role} | model: {model} | session: {id} | task: {id} | {timestamp}]`
3. **Multi-Project, Multi-Cluster Architecture** - Control plane / data plane separation from day one. ZenProject and ZenCluster CRDs. Session Affinity Dispatcher is cluster-aware.
4. **ZenLedger: Token and Cost Accounting** - Track input/output tokens, cost (real for API, estimated for local), latency, and task outcome per model per task. Model efficiency ranking, project cost breakdown, SR&ED-eligible cost export. **ZenLedgerClient interface defined in Block 1.7.1 for clean dependency ordering.**
5. **Database Provisioning** - CockroachDB provisioning and migration added to Block 3.1 with `make db-up` target.
6. **Funding Evidence Aggregator** - Block 5.4 generates SR&ED T661 narratives and IRAP technical reports from accumulated evidence.
7. **Experiment-Class Events** - ZenJournal now includes HypothesisFormulated, ApproachAttempted, ResultObserved, ApproachAbandoned, ExperimentConcluded event types.
8. **Canonical Taxonomy Extended** - Added SR&ED tags (u1_dynamic_provisioning, u2_security_gates, u3_deterministic_delivery, u4_backpressure, experimental_general) and AIAttribution struct.

## 0.1 What's New in V6.1

This patch aligns V6 with the actual execution direction:

1. **zen-sdk Reuse Tightened** — generic runtime concerns are no longer just “recommended imports”; they are explicit implementation requirements.
2. **KB/QMD Direction Corrected** — Git remains the KB source of truth; qmd is the default indexing/search path; CockroachDB is not the default backend for KB/QMD.
3. **Testing Strategy Corrected** — local integration uses k3d, and a first-class vertical-slice acceptance lane is added.
4. **Control Model Extended** — future system vocabulary is made explicit: RoleProfile, ExecutionPolicy, HandoffPolicy, Tool, ToolBinding, ComplianceProfile, WorkspaceClass.
5. **1.1 Radar Captured** — agent sandbox, small-model/MLQ strategy, Ops Department, and compliance overlays are recorded now to avoid drift.
6. **Funding Evidence Aggregator Demoted** — still important, but no longer allowed to block the first trustworthy internal-use vertical slice.

**New Architecture Principles:**
- Architect first, implement second: All major schemas are designed in Block 1 before any implementation begins
- Clean abstractions: ZenOffice interface exists before any specific connector (Jira)
- Configurable from day 0: ZEN_BRAIN_HOME environment variable for all runtime paths
- Memory as first-class: ZenContext is not optional—it is a core architectural component
- Deliver when possible: No dates, no estimates, no velocity tracking—quality takes what it takes
- Strict decoupling: Factory depends on abstractions, not implementations
- Reuse first: All cross-cutting concerns come from zen-sdk
- **SR&ED by default: Every session produces funding-eligible evidence unless explicitly disabled**
- **Cost awareness: Every LLM call is tracked with yield metrics for optimization**
- **Multi-cluster native: All interfaces accept cluster context from day one**
---

## 1. Executive Summary

This document outlines the construction plan for Zen-Brain 1.0, a complete rebuild of the existing zen-brain 0.1 system. The new architecture follows the proven Office + Factory model, where Jira serves as the "Office" for intent recognition and planning, while Kubernetes serves as the "Factory" for executing AI agent work units.

---

## 2. Architecture Overview

### 2.1 The Office + Factory Model

Zen-Brain 1.0 adopts a two-domain separation that mirrors how human organizations function:

**The Office (Planning):** This is where intent originates. Human operators create work items in Jira (or any compatible system), describe what they need, and receive completed work with evidence attached. The Office handles intent analysis, task decomposition, planning, and human-in-the-loop approvals. It is the system of record for what should be done. Critically, the Office is abstracted behind the ZenOffice interface, allowing any system (Jira, Linear, Trello, Slack, email) to serve as the interface.

**The Factory (Execution):** This is where work gets executed. The Factory spawns AI agents as Kubernetes Pods, manages their lifecycle, coordinates multi-agent collaboration, and stores evidence of completed work. It is the system of record for how work is done and what actually happened. The Factory operates on canonical work-item contracts, independent of which system originated the request.

**The Bridge:** The bidirectional synchronization layer connects Office and Factory through ZenJournal. All events flow through the immutable ledger, creating a causal chain that any agent can reconstruct to understand the full history of any task.

### 2.2 Core Components

The following components form the foundation of Zen-Brain 1.0:

**ZenJournal (Immutable Event Ledger):** An append-only log of all significant actions in the system. Each entry is cryptographically linked to the previous entry via Merkle tree hashing, enabling tamper detection and efficient state verification. **Implementation: Uses zen-sdk/pkg/receiptlog.**

**ZenContext (Tiered Memory):** A three-tier memory system for agents. Tier 1 (Hot) uses Redis and tmpfs for sub-millisecond access to session context. Tier 2 (Warm) uses a vector database (QMD - Question-Answer Memory Database) for fast knowledge and procedure lookups. Tier 3 (Cold) uses object storage for archival logs. ZenContext enables agents to pick up where they left off and retrieve relevant historical information.

**ZenOffice (Abstract Interface):** A generic interface defining how Zen-Brain interacts with external planning systems. Specific implementations (Jira Connector, Linear Connector, Slack Connector) satisfy this interface. The Office depends on the interface, never on a specific implementation. This abstraction is critical: the Factory should never know or care which system originated a work request.

**ZenGate (Admission Controller):** The gatekeeper validating and authorizing requests before they enter the Factory. Implements input validation, authorization checks, and policy enforcement. ZenGate is a system-wide concern, not just a Factory concern.

**ZenPolicy (Declarative Rules):** Declarative YAML-based rules that define what actions are allowed, required, or forbidden. Policies are defined once and enforced everywhere (Office and Factory).

**ZenGuardian (Proactive Monitor):** The active monitoring system that watches running agents and intervenes when necessary. Implements circuit breaking, anomaly detection, PII filtering, and safety boundaries.

**LLM Gateway:** A unified interface for LLM interactions, abstracting provider differences. Supports OpenAI, Anthropic, local models, and future providers through a standardized adapter interface.
### 2.3 Multi-Project, Multi-Cluster Architecture (NEW in V6)

Zen-Brain operates in a control plane / data plane topology from day one. This enables:
- Managing multiple projects across multiple Kubernetes clusters
- Scaling from 2 laptops with k3d to cloud deployments without architecture changes
- Project isolation with shared governance

**Topology:**

```
+------------------------------------------------------------------------------+
|                           CONTROL PLANE (zen-brain-core)                      |
|                          Single instance on primary machine                    |
|                                                                               |
|  +-------------------+  +-------------------+  +---------------------------+  |
|  | Project Registry  |  | Cluster Registry  |  | Global ZenJournal         |  |
|  | (ZenProject CRDs) |  | (ZenCluster CRDs) |  | Aggregation               |  |
|  +-------------------+  +-------------------+  +---------------------------+  |
|                                                                               |
|  +-------------------+  +-------------------+  +---------------------------+  |
|  | Cross-Project     |  | Board of          |  | Funding Report            |  |
|  | Scheduling        |  | Directors         |  | Generator                 |  |
|  +-------------------+  +-------------------+  +---------------------------+  |
|                                                                               |
+----------------------------------|-------------------------------------------+
                                   |
                    +--------------+--------------+
                    |         gRPC/HTTP           |
                    +--------------+--------------+
                                   |
          +------------------------+------------------------+
          |                        |                        |
          v                        v                        v
+------------------+    +------------------+    +------------------+
| DATA PLANE       |    | DATA PLANE       |    | DATA PLANE       |
| (zen-brain-agent)|    | (zen-brain-agent)|    | (zen-brain-agent)|
|                  |    |                  |    |                  |
| Machine 1        |    | Machine 2        |    | Cloud Provider   |
| k3d cluster      |    | k3d cluster      |    | Managed K8s      |
|                  |    |                  |    |                  |
| Project:         |    | Project:         |    | Project:         |
| - zen-brain      |    | - zen-mesh       |    | - client-work    |
|                  |    |                  |    |                  |
| Local ZenJournal |    | Local ZenJournal |    | Local ZenJournal |
| shard            |    | shard            |    | shard            |
+------------------+    +------------------+    +------------------+
```

**CRDs:**

```yaml
# ZenProject CRD
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenProject
metadata:
  name: zen-brain
spec:
  display_name: "Zen-Brain 1.0 Development"
  cluster_ref: "local-machine-1"
  repo_urls:
    - "git@github.com:kube-zen/zen-brain-1.0.git"
  kb_scopes:
    - "zen-brain"
    - "general"
    - "company"
  sred_tags:
    - "u1_dynamic_provisioning"
    - "u2_security_gates"
    - "u3_deterministic_delivery"
    - "u4_backpressure"
  funding_programs:
    - "sred"
    - "irap"
  sred_disabled: false  # Default: SR&ED evidence collection ON
```

```yaml
# ZenCluster CRD
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenCluster
metadata:
  name: local-machine-1
spec:
  endpoint: "https://k3d-local:6443"
  auth_ref: "kubeconfig-local-1"  # Reference to K8s Secret
  capacity:
    cpu_cores: 8
    memory_gb: 32
  status: "active"
  location: "local"  # local | cloud
```

**Current State (2 machines, 2 k3d clusters):**
- Machine 1: Control plane + Data plane for zen-brain project
- Machine 2: Data plane for zen-mesh project

**Future State (cloud):**
- Control plane stays on trusted machine
- Data plane agents deploy to cloud clusters as needed
- Same topology, same interfaces

**Interface Changes (All Accept Cluster Context):**

```go
// All major interfaces accept cluster context
type ZenOffice interface {
    FetchTicket(ctx context.Context, clusterID string, ticketID string) (*WorkItem, error)
    UpdateStatus(ctx context.Context, clusterID string, ticketID string, status WorkStatus) error
    // ...
}

type ZenContext interface {
    GetSessionContext(ctx context.Context, clusterID string, sessionID string) (*SessionContext, error)
    // ...
}
```

**Global ZenJournal Aggregation:**
- Each cluster maintains a local ZenJournal shard
- Control plane aggregates shards for:
  - Cross-project queries
  - SR&ED reporting
  - Board of Directors sessions
- Aggregation happens every 5 minutes (configurable)

### 2.4 zen-sdk Ecosystem Dependencies

Zen-Brain 1.0 leverages the existing zen-sdk ecosystem for cross-cutting concerns:

| Package | Version | Used By | Description |
|---------|---------|---------|-------------|
| zen-sdk/pkg/receiptlog | v0.3.0+ | ZenJournal (Block 3.3) | Append-only ledger with chain hashes |
| zen-sdk/pkg/scheduler | v0.3.0+ | KB Ingestion (Block 3.5) | Cron + one-time job scheduling |
| zen-sdk/pkg/dedup | v0.3.0+ | Message Bus (Block 3.1) | Event deduplication with windows |
| zen-sdk/pkg/dlq | v0.3.0+ | Task Failures (Block 4) | Dead letter queue with retry |
| zen-sdk/pkg/observability | v0.3.0+ | All components | OpenTelemetry tracing |
| zen-sdk/pkg/retry | v0.3.0+ | LLM Gateway, KB Queries | Exponential backoff with jitter |
| zen-sdk/pkg/events | v0.3.0+ | Worker Lifecycle | Kubernetes event recording |
| zen-sdk/pkg/leader | v0.3.0+ | HA Components | Leader election |
| zen-sdk/pkg/logging | v0.3.0+ | All components | Structured logging |
| zen-sdk/pkg/health | v0.3.0+ | All components | Health/readiness probes |
| zen-sdk/pkg/crypto | v0.3.0+ | Secrets (Block 4.0) | Age encryption (migrated from zen-lock) |

**Rule: If zen-sdk has it, use it. If zen-brain needs a new cross-cutting concern, build it in zen-sdk first.**
---

## 3. Research Integration

This section documents the research findings for specific features requested during planning.

### 3.1 Merkle Tree for ZenJournal

**Purpose:** Provide cryptographic integrity verification for the event ledger, enabling efficient state verification without replaying the entire chain.

**Implementation:** Uses zen-sdk/pkg/receiptlog which provides:
- Append-only ledger with SHA-256 chain hashes
- Rolling hash linking (tamper-evidence)
- Sequence numbers for ordering
- S3/MinIO backup support
- Verify() method for integrity checking

**zen-brain adds:**
- ZenJournal-specific event types (IntentCreated, PlanGenerated, ActionExecuted, etc.)
- Event type filtering and querying
- Compaction strategy for archival

### 3.2 Knowledge Base with QMD

**Purpose:** Enable fast retrieval of relevant knowledge and procedures by AI agents.

**Architecture:**
- **Source of Truth:** Git repositories (zen-docs, zen-sdk, project repos)
- **Indexing/Search:** qmd CLI tool (Question-Answer Memory Database)
- **Storage:** qmd manages its own vector index (not CockroachDB)

**Implementation:** zen-brain wraps qmd CLI as a subprocess adapter (internal/qmd/adapter.go) that:
- Runs `qmd refresh` to update index from git repositories
- Runs `qmd search` to query the vector index
- Parses JSON output into structured results
- Provides clean QMD interface abstraction for internal consumption

**Key Points:**
- Git remains the authoritative source of truth for all KB content
- qmd handles vector indexing and search semantics
- zen-brain does not manage KB storage directly
- Adapter pattern allows future qmd backend changes without zen-brain code changes
- CockroachDB is NOT used for KB/QMD (see V6.1 correction)

**Embedding Model:** qmd tool uses configured embedding model (typically nomic-embed-text via Ollama for local inference)

**Scope Isolation:** qmd supports repository-level and path-based scoping for multi-repo knowledge bases

**Decision Criteria:**
- If CPU-only inference: nomic-embed-text (768d)
- If budget allows API: text-embedding-3-small (1536d)
- Can change later via QMD interface abstraction

**Use Cases:**

- "How do I deploy to staging?" → Returns specific steps
- "What was the fix for the memory leak in service X?" → Returns relevant ticket context
- "What policies apply to this action?" → Returns relevant policy rules

**Performance Expectations:**

| Scale | Expected Latency | Notes |
|-------|------------------|-------|
| < 100K vectors | 1-5ms | Single partition, often cached |
| 100K - 1M vectors | 5-15ms | 2-3 levels of tree |
| 1M - 1B vectors | 15-50ms | 3-4 levels, some network hops |

#### 3.2.2 Chunking Strategy

**Rules:**
- Target chunk size: 512 tokens (~2KB text)
- Overlap: 50 tokens (10%) for context continuity
- Respect document structure (do not split mid-section)

**By File Type:**

| Type | Strategy |
|------|----------|
| Markdown | Split by ## headings, then by paragraph |
| Code | Split by function/class, include imports |
| YAML/JSON | Keep as single chunk if <4KB, else split by top-level keys |
| ADR (Architecture Decision) | One chunk per ADR file |

**Chunk Metadata:**

```go
type ChunkMetadata struct {
    SourcePath  string   `json:"source_path"`   // Original file path
    HeadingPath []string `json:"heading_path"`  // ["Section", "Subsection"]
    ChunkIndex  int      `json:"chunk_index"`   // Position in file
    TokenCount  int      `json:"token_count"`   // Actual size
    FileType    string   `json:"file_type"`     // markdown, go, yaml, etc.
    Language    string   `json:"language"`      // Programming language (if code)
}
```
### 3.3 Tmpfs for Agent Speed

**Purpose:** Provide near-zero latency scratchpad operations for distributed agents in Kubernetes.

**Implementation:** Configure agent pods with emptyDir volumes backed by tmpfs (Medium: Memory). This provides sub-millisecond I/O for agent working memory, intermediate computations, and inter-process communication within the pod.

**Configuration:**

```yaml
spec:
  containers:
  - name: agent
    volumeMounts:
    - name: agent-scratch
      mountPath: /dev/shm
  volumes:
  - name: agent-scratch
    emptyDir:
      medium: Memory
      sizeLimit: 512Mi
```

**Use Cases:**

- Agent scratchpad for intermediate reasoning
- Temporary file storage for code generation
- IPC between agent sub-processes

### 3.4 ReMe (Recursive Memory)

**Purpose:** Enable agents to reconstruct their state when waking up, verifying previous context before acting.

**Implementation:** When an agent picks up a task (after restarts, failures, or scheduled work), it executes a ReMe protocol: read the ZenJournal entries for this task, reconstruct the causal chain of events, pull relevant context from ZenContext, verify the current state matches expectations, and then continue execution.

**Benefits:**

- Agents recover gracefully from failures
- Full state reconstruction for debugging
- Consistent behavior across restarts

### 3.5 Warm Worker Pool with Session Affinity (A+C Hybrid)

**Purpose:** Eliminate cold-start overhead (10-40s per task) by keeping workers warm with models loaded, using git worktrees for isolation and session affinity for context carry.

**Problem Statement:**
Current destroy/recreate pattern has significant overhead:

```
Task arrives -> Schedule pod -> Pull image -> Start container -> Load model -> Execute -> Die
                    |           |            |              |
                 ~2-5s       ~1-3s        ~1-2s         ~5-30s (CPU)
Total cold start: 10-40 seconds per task
```

**Architecture:**

```
+---------------------------------------------------------------------+
|                      FACTORY FLOOR                                   |
|                                                                      |
|  Shared Volume: /factory/                                            |
|  |-- repos/                                                          |
|  |   |-- zen-brain-1.0/          (bare repo)                        |
|  |   +-- worktrees/                                                  |
|  |       |-- wt-task-001/        (worktree for task 001)            |
|  |       |-- wt-task-002/        (worktree for task 002)            |
|  |       +-- wt-session-abc/     (worktree for session abc)         |
|  |-- artifacts/                  (shared output)                     |
|  +-- cache/                      (model cache, shared)               |
|                                                                      |
+---------------------------------------------------------------------+
|                       WORKER POOL                                    |
|                                                                      |
|  +-------------------------------------------------------------+    |
|  | Worker Pool (Deployment, replicas=N)                         |    |
|  |                                                              |    |
|  |  Pod-1              Pod-2              Pod-3              Pod-4|    |
|  |  +----------+      +----------+      +----------+      +-----+|    |
|  |  | Session  |      | Session  |      |  Idle    |      |Task ||    |
|  |  |   ABC    |      |   DEF    |      |          |      | GHI ||    |
|  |  |          |      |          |      |          |      |     ||    |
|  |  | wt-abc/  |      | wt-def/  |      |          |      |wt-gh||    |
|  |  | /dev/shm |      | /dev/shm |      |          |      |/dev/||    |
|  |  | (ctx)    |      | (ctx)    |      |          |      |shm  ||    |
|  |  |          |      |          |      |          |      |     ||    |
|  |  | Model OK |      | Model OK |      | Model OK |      |Model||    |
|  |  +----------+      +----------+      +----------+      +-----+|    |
|  |                                                              |    |
|  |  All pods:                                                   |    |
|  |  - Mount /factory (read-write)                              |    |
|  |  - tmpfs /dev/shm 512Mi (per-pod private scratch)           |    |
|  |  - Model pre-loaded on startup                              |    |
|  |  - Long-running (do not die after task)                     |    |
|  +-------------------------------------------------------------+    |
|                                                                      |
+---------------------------------------------------------------------+
|                      DISPATCHER                                      |
|                                                                      |
|  Task arrives:                                                       |
|  1. Check if session exists -> route to same worker                 |
|  2. If new task -> pick idle worker or queue                        |
|  3. Worker creates worktree: git worktree add /factory/wt-task-N    |
|  4. Worker executes in worktree                                     |
|  5. Worker cleans up worktree: git worktree remove                  |
|  6. Worker marks itself available                                   |
+---------------------------------------------------------------------+
```

**Key Design Elements:**

1. **Shared Workspace (hostPath or PVC):**
   ```yaml
   volumes:
   - name: factory-floor
     hostPath:
       path: /factory
       type: DirectoryOrCreate
   volumeMounts:
   - name: factory-floor
     mountPath: /factory
   ```
   Shared: bare git repo, worktrees, artifacts, model cache

2. **Per-Pod Tmpfs (private scratch):**
   ```yaml
   volumes:
   - name: scratch
     emptyDir:
       medium: Memory
       sizeLimit: 512Mi
   volumeMounts:
   - name: scratch
     mountPath: /dev/shm
   ```
   Private: agent reasoning state, temporary files, context for current task

3. **Worktree Strategy:**
   ```bash
   # Task starts
   git worktree add /factory/worktrees/wt-task-123 main
   # Agent works there
   cd /factory/worktrees/wt-task-123
   # Commit results
   git add . && git commit -m "Task 123 complete"
   # Task ends
   git worktree remove /factory/worktrees/wt-task-123
   ```
   Benefits: Full git isolation, no merge conflicts, cheap to create (~100ms), easy cleanup

4. **Worker Lifecycle:**
   ```bash
   # Worker does not exit after task - it loops:
   while true; do
     task=$(wait_for_task_from_queue)
     worktree=$(create_worktree $task)
     execute_task $task $worktree
     cleanup_worktree $worktree
     mark_task_complete $task
   done
   ```

5. **Session Affinity:**
   Multi-step tasks (plan -> code -> test -> review) stay on same worker:
   - Context in /dev/shm persists for session
   - Worktree created once per session, not per step
   - Worker affinity: session X always routes to worker Y

**Benefits:**

| Aspect | Destroy/Recreate | Warm Pool (A+C) |
|--------|------------------|-----------------|
| Startup overhead | 10-40s | 0s (model loaded) |
| Isolation | Full container | Git worktree |
| Memory idle cost | 0 | Medium (workers stay alive) |
| Context carry | No | Yes (session affinity) |
| Worktree overhead | N/A | ~100ms create/remove |
### 3.6 KB Ingestion Service Architecture

**Purpose:** Aggregate knowledge from multiple repositories into fast QMD for AI workers, with proper scope isolation and Confluence synchronization.

**Physical Layout: The /factory/repos Pattern**

```
/factory/
|-- repos/                          # BARE REPOSITORIES (shared, read-only for workers)
|   |-- zen-docs.git/               # Primary KB source
|   |-- zen-sdk.git/                # SDK docs + code
|   |-- zen-lock.git/               # Lock docs + ADRs
|   |-- zen-brain-1.0.git/          # Brain code + docs
|   +-- ... (other projects)
|
|-- worktrees/                      # PER-TASK WORKTREES (isolated, writable)
|   |-- wt-task-001/
|   |   |-- code/                   # -> zen-brain-1.0 worktree (for code changes)
|   |   +-- docs/                   # -> zen-docs worktree (if task needs doc updates)
|   +-- wt-task-002/
|       +-- code/                   # -> different repo worktree
|
|   `-- kb/                             # qmd vector index (managed by qmd CLI, not CockroachDB)
|
+-- cache/                          # Shared model cache
```

**Scope Model: Knowledge Segmentation**

```yaml
# ~/.zen/zen-brain/kb-scopes.yaml
scopes:
  # COMPANY - policies, standards, onboarding (all workers see this)
  company:
    sources:
      - repo: zen-docs
        paths: ["/company/*", "/policies/*", "/standards/*"]
    visibility: all
    
  # GENERAL - useful across projects (SDK patterns, best practices)
  general:
    sources:
      - repo: zen-docs
        paths: ["/guides/*", "/reference/*"]
      - repo: zen-sdk
        paths: ["/docs/*", "/README.md", "/examples/*"]
    visibility: all
    
  # PROJECT-SPECIFIC - only relevant workers
  zen-brain:
    sources:
      - repo: zen-brain-1.0
        paths: ["/docs/*", "/adr/*", "/runbooks/*"]
      - repo: zen-docs
        paths: ["/projects/zen-brain/*"]
    visibility: [zen-brain-planner, zen-brain-worker]
    
  zen-lock:
    sources:
      - repo: zen-lock
        paths: ["/docs/*", "/adr/*"]
      - repo: zen-docs
        paths: ["/projects/zen-lock/*"]
    visibility: [zen-lock-workers]
```

**Ingestion Loop:**

```
KB Ingestion Service:
  every 5 minutes:
    for repo in configured_repos:
      current_head = git rev-parse HEAD
      if current_head != last_synced_head[repo]:
        changed_files = git diff --name-only last_synced_head HEAD
        for file in changed_files:
          if file matches scope_patterns:
            delete_old_chunks(file)  # Remove stale data
            chunks = chunk_document(file)
            embeddings = generate_embeddings(chunks)
            store_in_qmd(chunks, embeddings, scope)
        last_synced_head[repo] = current_head
        
  every 1 hour:
    sync_to_confluence()  # Export for human access
```

**Confluence Sync Strategy:**

Git is source of truth, Confluence is a **view**:

```
zen-docs (markdown)                 Confluence (human-readable)
       |                                    ^
       |                                    |
       v                                    |
+-----------------------------------------------------------------+
|  Confluence Exporter                                             |
|                                                                  |
|  1. Parse markdown files                                         |
|  2. Convert to Atlassian Document Format                         |
|  3. Map scopes -> Confluence spaces:                              |
|     - company/*    -> "Company Policies" space                    |
|     - general/*    -> "Engineering Wiki" space                    |
|     - zen-brain/*  -> "Zen Brain" space                           |
|  4. Preserve page hierarchy (headings -> child pages)             |
|  5. Add footer: "Source: git@repo:/path/file.md @ abc1234"       |
|  6. Update pages (do not delete - humans may add comments)        |
+-----------------------------------------------------------------+
```

#### 3.6.1 Confluence Sync Policy

**Policy: Strict Mirror (Git is Source of Truth)**

Confluence is a read-only view of documentation. Git is the single source of truth.

| Scenario | Behavior |
|----------|----------|
| Human edits in Confluence | **Overwritten on next sync** - Git wins |
| File deleted in Git | Page archived to "Archive" space |
| File added in Git | New page created in appropriate space |
| File renamed in Git | Page renamed (history preserved) |

**Warning Banner (auto-added to all synced pages):**

```html
<ac:structured-macro ac:name="note">
  <ac:parameter ac:name="title">Auto-Synced from Git</ac:parameter>
  <ac:rich-text-body>
    <p>This page is automatically synced from Git. 
    <strong>Any edits here will be overwritten on the next sync.</strong></p>
    <p>Source: <code>git@repo:/path/file.md @ abc1234</code></p>
  </ac:rich-text-body>
</ac:structured-macro>
```

**Sync Behavior:**
- Every sync overwrites Confluence page body with Git content
- Page comments and attachments are preserved (not deleted)
- Previous versions available via Confluence page history
- If human made edits, those edits are lost but previous version is preserved in history

**Worker Access Pattern:**

Workers query QMD directly via API. No git worktree needed for reading KB:

```go
// Workers query QMD directly
results := qmdClient.Search(ctx, &qmd.SearchRequest{
    Query:  "How do I configure TLS for zen-lock?",
    Scopes: []string{"general", "zen-lock"},
    Limit:  5,
})
```

**QMD Interface (Abstracted for Future Migration):**

```go
type QMD interface {
    Search(ctx context.Context, query string, scopes []string, limit int) ([]SearchResult, error)
    SearchWithEmbedding(ctx context.Context, embedding []float32, scopes []string, limit int) ([]SearchResult, error)
    Insert(ctx context.Context, chunks []Chunk) error
    DeleteBySource(ctx context.Context, scope, repo, path string) error
    GetStats(ctx context.Context) (*IndexStats, error)
}
```

**CockroachDB Cluster Resources (3-node):**

| Resource | Per Node | Total |
|----------|----------|-------|
| CPU | 2 cores | 6 cores |
| RAM | 4GB | 12GB |
| Disk | 20GB SSD | 60GB |
### 3.7 CockroachDB Backup Strategy

**Purpose:** Ensure KB data can be recovered from failures.

**Backup:**
- Full backup: daily at 2 AM UTC
- Incremental: every 4 hours
- Retention: 7 days
- Storage: S3-compatible (MinIO for self-hosted)

**Recovery:**
- Point-in-time recovery via CockroachDB built-in backup
- RTO: ~15 minutes
- RPO: 4 hours (incremental backup interval)

**Implementation:**

```sql
-- Schedule backup
CREATE SCHEDULE FOR BACKUP INTO 's3://backups/zen-brain-kb?AWS_ACCESS_KEY_ID=...'
  RECURRING '1d' FULL BACKUP ALWAYS
  WITH SCHEDULE OPTIONS first_run = 'now';
```

**Note:** KB can also be fully regenerated from git repos (re-run ingestion).

### 3.8 Local Development Mode

**Purpose:** Enable developers to run zen-brain locally using k3d clusters (consistent with zen-lock, zen-flow, zen-watcher patterns).

**k3d Cluster (Dev):**

```bash
# Create development cluster
k3d cluster create zen-brain-dev \
  -p "8080:80@loadbalancer" \
  -p "26257:26257@loadbalancer" \
  --registry-create zen-registry:5000

# Deploy dependencies (CockroachDB single-node, Redis)
kubectl apply -f deployments/k3d/dependencies.yaml

# Build and load image
make dev-build

# Deploy zen-brain
kubectl apply -f deployments/k3d/zen-brain-dev.yaml
```

**Benefits of k3d approach:**
- Same topology as production (Kubernetes-native)
- Helper services from other zen projects can run in same cluster
- No docker-compose.yaml maintenance
- Consistent with existing zen project patterns

**Environment Detection:**

```go
func GetQMDConfig() QMDConfig {
    if os.Getenv("ZEN_BRAIN_DEV") == "true" {
        return QMDConfig{
            SingleNode: true,
            Insecure:   true,
            URI:        "postgresql://root@localhost:26257/zen_brain",
        }
    }
    return QMDConfig{
        SingleNode: false,
        URI:        os.Getenv("COCKROACHDB_URI"),
    }
}
```

### 3.9 Observability Stack

**Purpose:** Monitor zen-brain health and performance.

**Metrics (Prometheus):**
- Worker pool utilization
- Task execution latency
- KB query latency (p50, p95, p99)
- LLM API call rates and latency
- ZenJournal write rate

**Tracing (OpenTelemetry via zen-sdk/pkg/observability):**
- End-to-end task flow (Office -> Factory -> Completion)
- LLM Gateway calls
- KB queries

**Dashboards (Grafana):**
- Factory overview (active workers, queue depth)
- KB health (query latency, index stats)
- LLM usage (tokens, cost, latency)

**Alerts:**
- CockroachDB node down
- Worker pool exhausted
- KB query latency > 100ms
- Task failure rate > 5%
- ZenJournal write failures

### 3.10 Schema Migration Strategy

**Purpose:** Evolve CockroachDB schema safely over time.

**Tool:** golang-migrate

**Migration Files:**

```
migrations/
|-- 001_initial_schema.up.sql
|-- 001_initial_schema.down.sql
|-- 002_add_metadata_index.up.sql
+-- 002_add_metadata_index.down.sql
```

**Deployment:**
- Migrations run as init container before main app starts
- CockroachDB supports online schema changes (no downtime)
- Backward-compatible migrations preferred

**Version Tracking:**

```sql
CREATE TABLE schema_migrations (
    version BIGINT PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT now()
);
```

### 3.11 Session Affinity Edge Cases

**Worker Death:**
- Session -> worker mapping stored in CockroachDB (survives restart)
- If worker pod dies:
  1. Kubernetes restarts pod (same pod name if StatefulSet)
  2. Pod reads session mapping from DB
  3. /dev/shm context is lost -> trigger ReMe protocol
  4. Agent reconstructs state from ZenJournal

**Session Timeout:**
- Default: 30 minutes of inactivity
- On timeout: session -> worker mapping deleted
- Next task for that session starts fresh

**Load Rebalancing:**
- Sessions can be migrated if worker overloaded
- Trigger: worker CPU > 80% for 5 minutes
- Action: migrate sessions to less loaded workers

### 3.12 KB Quality Validation

**Purpose:** Ensure KB search returns relevant results.

**Test Queries (Golden Set):**

```yaml
# test-queries.yaml
- query: "How do I deploy to staging?"
  expected_scopes: ["general", "zen-brain"]
  must_contain: ["staging", "deploy"]
  
- query: "What is the ADR for TLS configuration?"
  expected_scopes: ["zen-lock", "general"]
  must_contain: ["TLS", "zen-lock"]
```

**Metrics:**
- Precision: % of results that are relevant
- Recall: % of relevant docs that are found
- Target: >80% precision, >70% recall

**Feedback Loop:**
- Workers log KB queries and whether results were useful
- Weekly review of low-quality queries
- Adjust chunking/embedding based on feedback
### 3.13 SR&ED/IRAP Evidence Collection (NEW in V6)

**Purpose:** Generate funding-eligible evidence automatically as a byproduct of every session. This is not an opt-in mode—it is the default behavior.

**Key Principle:** SR&ED eligibility is determined by what you can prove was experimental, not what actually was. The system produces that proof automatically.

**Default Behavior:** Every session produces SR&ED-eligible records unless explicitly disabled with `sred_disabled: true` in ZenPolicy or session configuration.

**Experiment-Class Events (NEW for ZenJournal):**

```yaml
# Added to ZenJournal event types
HypothesisFormulated:
  description: "Agent formulated a hypothesis about approach"
  payload:
    task_id: string
    hypothesis: string
    uncertainty_area: "u1" | "u2" | "u3" | "u4" | "general"
    timestamp: timestamp

ApproachAttempted:
  description: "Agent attempted a specific approach"
  payload:
    task_id: string
    approach_description: string
    approach_type: "implementation" | "debugging" | "optimization" | "research"
    timestamp: timestamp

ResultObserved:
  description: "Agent observed results from an approach"
  payload:
    task_id: string
    result: "success" | "partial" | "failure" | "inconclusive"
    evidence: string
    metrics: map<string, number>
    timestamp: timestamp

ApproachAbandoned:
  description: "Agent abandoned an approach after testing"
  payload:
    task_id: string
    reason: string
    learnings: string
    time_invested_minutes: number
    timestamp: timestamp

ExperimentConcluded:
  description: "Agent concluded experimental work with findings"
  payload:
    task_id: string
    hypothesis_validated: boolean
    final_approach: string
    key_learnings: string[]
    total_iterations: number
    timestamp: timestamp
```

**SR&ED Tag Categories:**

```yaml
Tags.sred:  # Added to Tag Categories in Block 1.5
  u1_dynamic_provisioning:
    description: "Dynamic resource provisioning uncertainty"
    examples: [k8s-autoscaling, worker-pool-sizing, session-affinity]
  u2_security_gates:
    description: "Security and access control uncertainty"
    examples: [zen-gate, policy-enforcement, credential-rotation]
  u3_deterministic_delivery:
    description: "Deterministic output delivery uncertainty"
    examples: [llm-consistency, evidence-verification, plan-reproducibility]
  u4_backpressure:
    description: "Backpressure and flow control uncertainty"
    examples: [queue-management, rate-limiting, load-shedding]
  experimental_general:
    description: "General experimental work not tied to specific uncertainty"
    examples: [prototype, poc, investigation]
```

**Evidence Classes for SR&ED:**

```yaml
EvidenceClass:
  benchmark_run:
    description: "Performance benchmark results"
    required_fields: [metric_name, baseline, result, improvement_pct]
  failure_case:
    description: "Documented failure and root cause"
    required_fields: [approach_tried, failure_mode, root_cause, resolution]
  iteration_record:
    description: "Iteration through experimental loop"
    required_fields: [iteration_number, hypothesis, result, next_step]
  experiment_card:
    description: "Summary card for entire experiment"
    required_fields: [hypothesis, approaches_tried, final_result, learnings]
```

**Planner Agent Behavior (Default):**

```yaml
# Planner automatically generates SR&ED framing
planner_session:
  sred_mode: default  # Always ON unless sred_disabled: true
  
  # For every task, Planner generates:
  hypothesis_framing:
    - "What is the uncertainty being addressed?"
    - "What approach will be tried?"
    - "What does success look like?"
    - "What are the failure modes?"
  
  # After task completion, Planner generates:
  experiment_summary:
    - "What was tried?"
    - "What worked? What didn't?"
    - "What was learned?"
    - "What would be tried next?"
```

**ZenFunding Configuration Layer:**

```yaml
# ZenFunding maps projects to funding programs
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenFunding
metadata:
  name: zen-brain-sred-2026
spec:
  project_ref: "zen-brain"
  program: "sred"  # sred | irap | mitacs | other
  tax_year: 2026
  evidence_requirements:
    - type: "hypothesis_documentation"
      frequency: "per_task"
    - type: "approach_attempts"
      frequency: "per_task"
    - type: "time_tracking"
      frequency: "continuous"
    - type: "outcome_documentation"
      frequency: "per_task"
  reporting:
    - type: "t661_technical_narrative"
      schedule: "yearly"
    - type: "quarterly_progress"
      schedule: "quarterly"
```

### 3.14 ZenLedger: Token and Cost Accounting (NEW in V6)

**Purpose:** Track the yield (value produced) per token spent. The core metric is not cost—it's value-per-token.

**Key Insight:** A task that costs $0.40 in API tokens but produces a merged PR is cheaper than a task that costs $0.02 but produces a comment that needs three human corrections.

**TokenRecord Schema:**

```go
type TokenRecord struct {
    SessionID      string    `json:"session_id"`
    TaskID         string    `json:"task_id"`
    AgentRole      string    `json:"agent_role"`
    ModelID        string    `json:"model_id"`         // glm-4.7, claude-sonnet-4-6, nomic-embed-text, etc.
    InferenceType  string    `json:"inference_type"`   // chat, embedding, rerank
    Source         string    `json:"source"`           // local | api
    
    // Cost side
    TokensInput    int64     `json:"tokens_input"`
    TokensOutput   int64     `json:"tokens_output"`
    TokensCached   int64     `json:"tokens_cached"`    // if provider supports prompt caching
    CostUSD        float64   `json:"cost_usd"`         // 0.00 for local, calculated for API
    LatencyMs      int64     `json:"latency_ms"`
    
    // Yield side
    Outcome        string    `json:"outcome"`          // completed | failed | human_corrected | abandoned
    EvidenceClass  string    `json:"evidence_class"`   // pr_merged, test_passed, doc_updated, plan_approved, etc.
    HumanCorrections int     `json:"human_corrections"` // count of times human had to fix the output
    SREDEligible   bool      `json:"sred_eligible"`
    
    Timestamp      time.Time `json:"timestamp"`
    ClusterID      string    `json:"cluster_id"`
    ProjectID      string    `json:"project_id"`
}
```

**Local Inference Cost Model:**

Local inference is not free—it has a cost in time and hardware:

```yaml
local_cost_model:
  cpu_inference_rate: 0.001    # $/min of CPU time
  gpu_inference_rate: 0.02     # $/min of GPU time (if available)
  memory_overhead_rate: 0.0001 # $/GB/min
  
# Example calculation:
# - 50ms latency on CPU
# - 4GB memory allocated
# - Equivalent cost: (50/1000/60 * 0.001) + (4 * 0.0001 * 50/1000/60) ≈ $0.000001
```

**Query Views (Required from Day One):**

```sql
-- Model efficiency report
SELECT 
    model_id,
    COUNT(*) as total_tasks,
    AVG(tokens_input + tokens_output) as avg_tokens_per_task,
    AVG(cost_usd) as avg_cost_per_task,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) as success_rate,
    AVG(human_corrections) as avg_corrections
FROM token_records
WHERE project_id = $1
GROUP BY model_id
ORDER BY avg_cost_per_task ASC;

-- Task type cost profile
SELECT 
    evidence_class,
    COUNT(*) as total_tasks,
    AVG(cost_usd) as avg_cost,
    AVG(latency_ms) as avg_latency,
    AVG(human_corrections) as avg_corrections
FROM token_records
WHERE project_id = $1
GROUP BY evidence_class
ORDER BY avg_cost ASC;

-- Local vs API comparison
SELECT 
    source,
    evidence_class,
    AVG(cost_usd) as avg_cost,
    AVG(latency_ms) as avg_latency,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) as success_rate
FROM token_records
WHERE project_id = $1 AND model_id LIKE '%glm%'
GROUP BY source, evidence_class
ORDER BY evidence_class, source;

-- Project cost breakdown
SELECT 
    date_trunc('week', timestamp) as week,
    project_id,
    SUM(cost_usd) as total_cost,
    SUM(tokens_input + tokens_output) as total_tokens,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END) as completed_tasks
FROM token_records
GROUP BY week, project_id
ORDER BY week DESC, project_id;

-- SR&ED cost export (for T661 schedule)
SELECT 
    project_id,
    sred_tag,
    SUM(cost_usd) as eligible_cost,
    SUM(tokens_input + tokens_output) as total_tokens,
    COUNT(*) as experimental_tasks
FROM token_records tr
JOIN task_sred_tags tst ON tr.task_id = tst.task_id
WHERE tr.sred_eligible = true
  AND tr.timestamp >= '2026-01-01'
  AND tr.timestamp < '2027-01-01'
GROUP BY project_id, sred_tag
ORDER BY project_id, sred_tag;
```

**Planner Agent Cost-Awareness:**

```yaml
# ZenPolicy extension for cost optimization
model_selection_policy:
  strategy: cost_optimized  # cost_optimized | quality_optimized | balanced
  max_cost_per_task_usd: 0.50
  prefer_local: true         # use local models when latency acceptable
  fallback_to_api: true      # fall back to API if local fails or too slow
  
  # Per-task-type overrides
  task_overrides:
    - task_type: "debug"
      preferred_models: ["glm-4.7-local", "claude-sonnet-4-6-api"]
      max_cost_usd: 0.30
    - task_type: "documentation"
      preferred_models: ["glm-4.7-local"]
      max_cost_usd: 0.10
```

**ZenLedger Dashboard (Grafana):**

- Model efficiency ranking (cost per completed task, success rate)
- Project cost breakdown (weekly/monthly)
- Local vs API comparison
- SR&ED-eligible cost accumulator (running total for tax year)

---

The construction is organized into **seven major blocks** (0 through 6). Block 0.5 is a sub-block within Block 0, not a separate block. Each block contains multiple phases that must be completed in order. Blocks can run in parallel only where explicitly noted in the dependencies section.

---

### Block 0: The Clean Foundation (Prerequisites)

**Purpose:** Establish a clean development environment before any design or implementation begins.

This block ensures 1.0 starts with zero technical debt. No code from 0.1 is copied. No configuration is inherited. The only connection to 0.1 is that 0.1 will be used to execute the construction plan.

#### Block 0.1: Create New GitHub Repository

**Goal:** Fresh remote repository with no git history pollution.

**Steps:**
1. Create new private repository: kube-zen/zen-brain-1.0
2. Initialize with README only (no .gitignore templates, no license)
3. Add description: "Zen-Brain 1.0 - Clean Architecture Implementation"
4. Document that 0.1 repository (kube-zen/zen-brain) remains active

**Acceptance Criteria:**
- [ ] gh repo view kube-zen/zen-brain-1.0 returns repo info
- [ ] Repo is empty except README.md
- [ ] No branches inherited from 0.1

**Owner:** Human (one-time setup)

#### Block 0.2: Create Local Repository

**Goal:** Fresh local workspace with clean directory structure.

**Steps:**
1. Create directory: mkdir -p ~/zen/zen-brain-1.0
2. Initialize git: cd ~/zen/zen-brain-1.0 && git init
3. Add remote: git remote add origin git@github.com:kube-zen/zen-brain-1.0.git
4. Create initial structure:
   ```
   ~/zen/zen-brain-1.0/
   |-- api/
   |   +-- v1alpha1/          # CRD definitions
   |-- cmd/
   |   +-- zen-brain/         # Main executable
   |-- pkg/
   |   |-- office/            # ZenOffice interface
   |   |-- context/           # ZenContext interface
   |   |-- journal/           # ZenJournal interface
   |   +-- llm/               # LLM Gateway interface
   |-- internal/
   |   |-- factory/           # Factory implementation
   |   |-- connector/         # Office connectors (Jira, etc.)
   |   +-- config/            # Configuration (home dir logic here)
   |-- docs/
   |   +-- architecture/      # Design documents
   |-- deployments/
   |   +-- kubernetes/        # K8s manifests
   |-- go.mod
   |-- go.sum
   |-- Makefile
   +-- README.md
   ```
5. Push initial commit: git add . && git commit -m "Initial scaffold"

**Acceptance Criteria:**
- [ ] ~/zen/zen-brain-1.0 exists
- [ ] Git remote points to new repo
- [ ] Directory structure matches above
- [ ] Initial commit pushed to main

**Owner:** Human or AI (can be executed by 0.1)

#### Block 0.3: Define Configurable Home Directory

**Goal:** All runtime paths derive from a single configurable variable.

**Design Requirements:**

```go
// Environment variable: ZEN_BRAIN_HOME
// Flag: --home (overrides env)
// Default: ~/.zen/zen-brain-1.0 (during development)

package config

const (
    DefaultHomeDir = ".zen/zen-brain-1.0"  // Development default
    EnvHomeDir     = "ZEN_BRAIN_HOME"
)

func GetHomeDir() string {
    // 1. Check --home flag (if using cobra/pflag)
    // 2. Check ZEN_BRAIN_HOME env var
    // 3. Fall back to default
    if home := os.Getenv(EnvHomeDir); home != "" {
        return home
    }
    userHome, _ := os.UserHomeDir()
    return filepath.Join(userHome, DefaultHomeDir)
}

type Paths struct {
    Home       string
    Config     string  // $HOME/config.yaml
    Data       string  // $HOME/data/
    Logs       string  // $HOME/logs/
    Cache      string  // $HOME/cache/
    Journal    string  // $HOME/data/journal.db
    Context    string  // $HOME/data/context/
    Artifacts  string  // $HOME/data/artifacts/
}

func NewPaths() *Paths {
    home := GetHomeDir()
    return &Paths{
        Home:      home,
        Config:    filepath.Join(home, "config.yaml"),
        Data:      filepath.Join(home, "data"),
        Logs:      filepath.Join(home, "logs"),
        Cache:     filepath.Join(home, "cache"),
        Journal:   filepath.Join(home, "data", "journal.db"),
        Context:   filepath.Join(home, "data", "context"),
        Artifacts: filepath.Join(home, "data", "artifacts"),
    }
}
```

**Critical Rule:** No code in 1.0 ever hardcodes ~/.zen/zen-brain or any specific path. All paths derive from GetHomeDir().

**Acceptance Criteria:**
- [ ] internal/config/home.go implements GetHomeDir()
- [ ] internal/config/paths.go implements NewPaths()
- [ ] Unit tests verify:
  - Env var override works
  - Flag override works (if implemented)
  - Default is used when neither specified
  - All path types are derived from home

**Owner:** AI (can be executed by 0.1)

#### Block 0.4: Document Cutover Plan

**Goal:** Clear documentation for switching from 0.1 to 1.0.

**Document:** Create docs/CUTOVER.md

```markdown
# Zen-Brain Cutover Plan

## Current State (During Development)

| Component | 0.1 | 1.0 |
|-----------|-----|-----|
| Source | ~/zen/zen-brain | ~/zen/zen-brain-1.0 |
| Home Dir | ~/.zen/zen-brain | ~/.zen/zen-brain-1.0 |
| Systemd | zen-brain.service | (not yet) |
| Status | ACTIVE | DEVELOPMENT |

## Cutover Steps (When 1.0 is Ready)

### Option A: Replace (Recommended)

```bash
# 1. Stop 0.1
systemctl --user stop zen-brain

# 2. Archive 0.1 home
mv ~/.zen/zen-brain ~/.zen/zen-brain-0.1-archived

# 3. Promote 1.0 home (if migrating data)
# OR keep separate and just switch env var
export ZEN_BRAIN_HOME=~/.zen/zen-brain

# 4. Update systemd to point to new binary
# 5. Start 1.0
```

### Option B: Parallel (For Gradual Migration)

```bash
# Keep both running, route traffic gradually
# 0.1: port 8080
# 1.0: port 8090
# Use reverse proxy to shift traffic
```

## Rollback Plan

```bash
# If 1.0 has issues, rollback to 0.1
systemctl --user stop zen-brain-1.0
mv ~/.zen/zen-brain-0.1-archived ~/.zen/zen-brain
systemctl --user start zen-brain
```

## Archive 0.1 Source (After Successful Cutover)

```bash
mv ~/zen/zen-brain ~/zen/zen-brain-0.1-archived
gh repo rename kube-zen/zen-brain --new-name zen-brain-0.1-archived
```
```

**Acceptance Criteria:**
- [ ] docs/CUTOVER.md exists
- [ ] Cutover steps are documented
- [ ] Rollback steps are documented
- [ ] Archive steps are documented

**Owner:** AI (can be executed by 0.1)

**Checkpoint 0 (Clean Foundation):**
- GitHub repo created
- Local repo scaffolded
- Configurable home directory implemented
- Cutover plan documented
- Zero code from 0.1 exists in 1.0
- Ready to begin Block 0.5
---

### Block 0.5: Pre-requisite SDK (MANDATORY REUSE CONTRACT)

**Purpose:** Ensure zen-brain 1.0 consumes zen-sdk for all generic cross-cutting runtime capabilities before implementing higher-level behavior.

**Rule:** The following capabilities MUST be imported from zen-sdk and MUST NOT be reimplemented inside zen-brain unless a documented exception is approved:

- `zen-sdk/pkg/receiptlog` → foundation for ZenJournal
- `zen-sdk/pkg/dedup` → message bus deduplication
- `zen-sdk/pkg/dlq` → failed task/message handling
- `zen-sdk/pkg/retry` → LLM provider retries, KB/qmd retries, transient external errors
- `zen-sdk/pkg/observability` → tracing/metrics wiring
- `zen-sdk/pkg/health` → readiness/liveness endpoints
- `zen-sdk/pkg/leader` → leader election for HA control-plane components
- `zen-sdk/pkg/logging` → structured logging
- `zen-sdk/pkg/events` → Kubernetes event recording
- `zen-sdk/pkg/crypto` → encryption and secret-protection helpers

**Implementation Rule:** If a new capability is generic and reusable across Zen projects, it must be added to zen-sdk first, then imported into zen-brain.

**Acceptance Criteria:**
- [ ] ZenJournal implementation is explicitly built on `zen-sdk/pkg/receiptlog`
- [ ] Message bus implementation explicitly uses `zen-sdk/pkg/dedup`
- [ ] Failed task/message handling explicitly uses `zen-sdk/pkg/dlq`
- [ ] LLM/provider layer explicitly uses `zen-sdk/pkg/retry`
- [ ] API/runtime health endpoints explicitly use `zen-sdk/pkg/health`
- [ ] Runtime tracing/metrics explicitly use `zen-sdk/pkg/observability`
- [ ] HA control-plane path explicitly uses `zen-sdk/pkg/leader`
- [ ] No local replacement package for these concerns exists in zen-brain without an approved ADR

---

### Block 1: The Neuro-Anatomy (Schemas and Design)

**Purpose:** Define all data structures, protocols, and interfaces before writing any execution code.

**Prerequisite:** Block 0 and Block 0.5 complete

**Components:**

**Block 1.1: ZenJournal Schema Definition.** Design the immutable event ledger architecture. Define the Protobuf or Avro schemas for EventBlock, including block header (previous block hash, Merkle root, timestamp), event entries (event type, actor, payload, correlation ID), and Merkle tree structure. Design the event types: IntentCreated, PlanGenerated, ActionExecuted, ResultVerified, ApprovalRequested, ApprovalGranted, AgentHeartbeat, PolicyViolation. Define the query API for retrieving events by time range, correlation ID, event type. Design the compaction strategy for archival. **Note: Core ledger functionality from zen-sdk/pkg/receiptlog.**

**Block 1.2: ZenContext Architecture Design.** Design the tiered memory system. Define Tier 1 (Hot) schema: Redis key patterns for session context, tmpfs mount points and size limits. Define Tier 2 (Warm) schema: Vector database collections for QMD, embedding models, similarity thresholds. Define Tier 3 (Cold) schema: S3/MinIO object paths for archival, retention policies. Design the ReMe protocol: how an agent reconstructs state from ZenJournal plus ZenContext. Design the context injection API: how agents receive relevant context before executing tasks.

**Block 1.3: ZenOffice Interface Definition.** Design the abstract interface for work ingress. Define the IZenOffice interface with methods: FetchTicket() returns a work item, UpdateStatus() updates work item status, LogWork() records work progress, RequestApproval() pauses for human input. This interface must have zero dependencies on any specific system (Jira, Linear, etc.). Design the WorkItem generic structure that all specific ticket types map to. Once this interface exists, any system can become an Office by implementing it.

**Block 1.4: ZenPolicy and ZenGate Schema Design.** Design the policy and gate architecture as system-wide concerns. Define ZenPolicy YAML structure: policy name, version, conditions, rules, metadata. Define ZenGate admission request/response payloads. Define the policy evaluation engine interface.

**Block 1.5: Canonical Work Taxonomy and Jira Mapping.** Define the stable internal model that all Office connectors map to. This taxonomy is the contract between human intent and machine execution — getting it right early prevents expensive refactoring later.

**Canonical Enums:**

```yaml
WorkType:
  values: [research, design, implementation, debug, refactor, documentation, analysis, operations, security, testing]
  description: "What kind of work is this?"

WorkDomain:
  values: [office, factory, sdk, policy, memory, observability, infrastructure, integration, core]
  description: "Which part of the system does this affect?"

Priority:
  values: [critical, high, medium, low, background]
  description: "How urgent? (normalized, not Jira-native)"

ExecutionMode:
  values: [autonomous, approval-required, read-only, simulation-only, supervised]
  description: "What level of human oversight?"

EvidenceRequirement:
  values: [none, summary, logs, diff, test-results, full-artifact]
  description: "What proof of work is needed?"

WorkStatus:
  values: [requested, analyzing, analyzed, planning, planned, pending_approval, approved, queued, running, blocked, completed, failed, canceled]
  description: "Canonical lifecycle state"
```

**Tag Categories:**

```yaml
Tags:
  human_org:     # For human organization (epics, teams, quarters)
    examples: [q1-2026, team-platform, epic-auth]
  routing:       # For system routing decisions
    examples: [llm-required, kb-query, long-running]
  policy:        # For ZenGate policy evaluation
    examples: [prod-affecting, requires-approval, audit-trail]
  analytics:     # For dashboards and reporting
    examples: [tech-debt, incident, feature, maintenance]
  sred:          # For SR&ED/IRAP evidence categorization (NEW in V6)
    values: [u1_dynamic_provisioning, u2_security_gates, u3_deterministic_delivery, u4_backpressure, experimental_general]
    description: "Which experimental uncertainty area does this address?"
```

**AI Attribution Struct (NEW in V6):**

All AI-generated content in Jira includes structured attribution headers for traceability and SR&ED compliance.

```yaml
AIAttribution:
  agent_role: string      # "planner-v1", "worker-debug", "worker-impl", etc.
  model_used: string      # "glm-4.7", "claude-sonnet-4-6", etc.
  session_id: string      # Session UUID for correlation
  task_id: string         # Task UUID for correlation
  timestamp: ISO8601      # When the content was generated
  
# Jira Comment/Description Header Format:
# [zen-brain | agent: planner-v1 | model: glm-4.7 | session: abc123 | task: def456 | 2026-03-07T14:32:00Z]
# 
# <actual content follows>
```

**Attribution Injection Rule:**
> The ZenOffice Jira adapter is responsible for injecting the AIAttribution header automatically. No agent writes to Jira without this header. This serves as SR&ED evidence—documenting which AI performed which action at what time.

**Source Metadata (preserved but not execution-critical):**

```yaml
SourceMetadata:
  system: "jira" | "linear" | "github" | "slack"
  issue_key: "PROJ-123"
  project: "PROJECT"
  issue_type: "Task" | "Bug" | "Story" | "Epic"
  parent_key: "PROJ-100"  # For subtasks
  epic_key: "PROJ-50"
  reporter: "alice"
  assignee: "bob"
  sprint: "Sprint 23"
  created_at: "2026-03-07T10:00:00Z"
  updated_at: "2026-03-07T12:00:00Z"
```

**Jira Field Mapping Table:**

| Jira Concept | Canonical Field | Transformation | Notes |
|--------------|-----------------|----------------|-------|
| Issue Type | WorkType | Task→implementation, Bug→debug, Story→design | Never leak Jira names into core |
| Priority | Priority | Highest→critical, High→high, etc. | Normalize strings to enum |
| Status | WorkStatus | Map workflow states to canonical | Configurable per project |
| Labels | Tags.human_org | Pass through with validation | Normalize case, strip spaces |
| Component | WorkDomain | Direct map if valid, else "core" | Optional field |
| Custom Field: Risk | Tags.policy | high→requires-approval | Used by ZenGate |
| Custom Field: KB Scope | (metadata) | Passed to QMD query | For context retrieval |
| Epic Link | SourceMetadata.epic_key | Preserve for hierarchy | Not execution-critical |
| Parent Issue | SourceMetadata.parent_key | Preserve for subtasks | Not execution-critical |
| Sprint | SourceMetadata.sprint | Preserve for analytics | Not execution-critical |

**Design Principles:**

1. **Jira is the human console** — humans request work, check status, review plans, approve/reject, inspect evidence
2. **Canonical model is the execution contract** — Factory never sees Jira fields, only WorkType/WorkDomain/Status
3. **Mapping is configurable** — different Jira projects can have different workflows, all map to same canonical states
4. **Source metadata preserved** — for auditing, analytics, and human context, but not for routing/execution
5. **Tags have purpose** — human_org, routing, policy, analytics categories prevent tag sprawl

**Checkpoint 1.5 (Taxonomy Frozen):**
- All canonical enums defined and reviewed
- Jira field mapping table complete
- Tag categories established
- Test cases: sample Jira issues map correctly
- Documented in `docs/data-model.md`

### Block 1.4.1: Future Control-Model Vocabulary (NEW in V6.1)

To support dynamic role creation with hard policy boundaries, the following concepts are defined now even if full implementation lands incrementally:

- **ZenRoleProfile** — dynamic mission/behavior definition for an agent role
- **ZenExecutionPolicy** — hard walls: allowed scopes, tools, budgets, approvals, forbidden actions
- **ZenHandoffPolicy** — controls whether and how one area/role can trigger another
- **ZenTool** — declarative tool definition
- **ZenToolBinding** — controlled mapping of tools to roles/policies
- **ZenComplianceProfile** — optional policy overlay for SR&ED, IRAP, SOC2, ISO 27k, FedRAMP-style futures
- **WorkspaceClass / TrustLevel** — classifies how sensitive a task/run/workspace is and what protections apply

**Rule:** Dynamic behavior is allowed to evolve quickly. Authority is enforced strictly through policy, scope, tool-binding, and infrastructure boundaries.

**Block 1.6: Repository and Build Infrastructure.** Set up Makefile targets, Dockerfiles, and CI/CD. Port essential utilities from 0.1 (logging, error handling) - **reference only, do not copy**.

**Block 1.7: LLM Gateway Interface.** Define the provider-agnostic LLM interface. Define methods: ChatCompletion(), Embedding(), ModelList(). Define the provider adapter interface. Actual provider implementations come later. **Embedding model decision: nomic-embed-text (768d) for local, text-embedding-3-small (1536d) for API.**

**Block 1.7.1: ZenLedger Query Interface (NEW in V6).** Define the provider-agnostic interface for cost/yield lookup used by the Planner Agent for model selection and policy-aware routing. The Planner depends on this interface only; the concrete implementation is provided later by Block 3.6.

```go
// ZenLedgerClient interface - Planner depends on this, Block 3.6 implements it
type ZenLedgerClient interface {
    // GetModelEfficiency returns historical efficiency data for models on a task type
    GetModelEfficiency(ctx context.Context, projectID string, taskType WorkType) ([]ModelEfficiency, error)
    
    // GetCostBudgetStatus returns current spending against budget limits
    GetCostBudgetStatus(ctx context.Context, projectID string) (*BudgetStatus, error)
    
    // RecordPlannedModelSelection logs the Planner's model choice for later analysis
    RecordPlannedModelSelection(ctx context.Context, sessionID string, taskID string, modelID string, reason string) error
}

type ModelEfficiency struct {
    ModelID           string  `json:"model_id"`
    AvgCostPerTask    float64 `json:"avg_cost_per_task"`
    AvgTokensPerTask  int64   `json:"avg_tokens_per_task"`
    SuccessRate       float64 `json:"success_rate"`
    AvgCorrections    float64 `json:"avg_corrections"`
    AvgLatencyMs      int64   `json:"avg_latency_ms"`
    SampleSize        int     `json:"sample_size"`
}

type BudgetStatus struct {
    ProjectID         string    `json:"project_id"`
    PeriodStart       time.Time `json:"period_start"`
    PeriodEnd         time.Time `json:"period_end"`
    SpentUSD          float64   `json:"spent_usd"`
    BudgetLimitUSD    float64   `json:"budget_limit_usd"`
    RemainingUSD      float64   `json:"remaining_usd"`
    PercentUsed       float64   `json:"percent_used"`
}
```

**Dependency Contract:**
- Block 2.5 (Planner Agent) imports this interface from Block 1.7.1
- Block 3.6 (ZenLedger Implementation) provides the concrete implementation
- This preserves build order: Block 1 → Block 2 → Block 3 without circular dependencies

**Block 1.8: SR&ED/IRAP Alignment Design (NEW in V6).** Design the evidence collection and reporting infrastructure for funding compliance. This serves two purposes: (1) generate SR&ED evidence automatically while building zen-brain 1.0, and (2) make zen-brain capable of producing SR&ED/IRAP documentation as a deliverable for any project it manages.

**Components to Define:**

```yaml
# ZenFunding Interface
ZenFunding:
  methods:
    - GetEvidenceRequirements(program: string) -> EvidenceRequirement[]
    - RecordEvidence(task_id: string, evidence: Evidence) -> void
    - GenerateReport(project_id: string, program: string, period: DateRange) -> FundingReport
  
# FundingReportRole (ZenRole template)
FundingReportRole:
  description: "Generates funding reports from accumulated evidence"
  capabilities:
    - Query ZenJournal filtered by SR&ED tags
    - Query ZenLedger for cost data
    - Query Evidence Vault for experiment artifacts
    - Generate T661 technical narrative
    - Generate IRAP technical report
    - Generate quarterly progress reports
```

**SR&ED Evidence Flow:**

```
Task Execution (default SR&ED mode ON)
    |
    v
HypothesisFormulated event -> ZenJournal
    |
    v
ApproachAttempted events -> ZenJournal (may be multiple)
    |
    v
ResultObserved events -> ZenJournal
    |
    v
Evidence collected -> Evidence Vault (with evidence_class)
    |
    v
TokenRecord -> ZenLedger (with sred_eligible: true)
    |
    v
Task completed -> ExperimentConcluded event -> ZenJournal
    |
    v
(Periodically) FundingReportRole generates T661/IRAP reports
```

**Checkpoint 1.8 (Funding Design Complete):**
- ZenFunding interface defined
- FundingReportRole template designed
- Evidence flow documented
- SR&ED event types added to ZenJournal schema
- Evidence classes defined in Evidence Vault schema

**Block 1.9: Multi-Cluster CRD Design (NEW in V6).** Design the CRDs for multi-project, multi-cluster topology. Define ZenProject CRD with fields: project_name, cluster_ref, repo_urls, kb_scopes, sred_tags, funding_programs, sred_disabled. Define ZenCluster CRD with fields: cluster_name, endpoint, auth_ref, capacity, status, location. Ensure all core interfaces (ZenOffice, ZenContext, ZenJournal) accept cluster context parameter.

**Checkpoint 1.9 (Multi-Cluster Design Complete):**
- ZenProject CRD defined
- ZenCluster CRD defined
- Interface signatures updated with cluster context
- Control plane / data plane communication protocol defined
- Global ZenJournal aggregation strategy documented

**Checkpoint 1 (Design Complete):** All schemas are designed, reviewed, and documented. The ZenOffice interface exists on paper before any Jira code is written. The Merkle tree structure is defined. ZenContext tiers are specified. SR&ED evidence collection is designed as default behavior. Multi-cluster topology is defined.

---

### Block 2: The Office (Abstraction Layer)

**Purpose:** Build the interface layer where human intent is captured, with clean separation between abstraction and implementation.

**Prerequisite:** Block 1 complete

**Components:**

**Block 2.1: ZenOffice Interface Implementation.** Implement the IZenOffice interface defined in Block 1.3. Create the generic work item types that map from any source system to internal representations. Ensure zero Jira-specific types leak into the core Office logic.

**Block 2.2: Jira Connector.** Implement the Jira provider that satisfies IZenOffice. Implement Jira REST API client with authentication. Create webhook listener for issue events. Implement bidirectional sync. **NEW in V6: AI Attribution** — all AI-generated comments and description updates include structured attribution header: `[zen-brain | agent: {role} | model: {model} | session: {id} | task: {id} | {timestamp}]`. The connector automatically injects this header on all writes.

**Block 2.3: Intent Analyzer.** Build the intelligence that understands what humans want. Implement multi-stage analysis using LLM Gateway. Output structured BrainTask specifications.

**Block 2.4: Session Manager.** Implement work session tracking. State machine: Created -> Analyzed -> Scheduled -> InProgress -> Completed/Failed.

**Block 2.5: Planner Agent.** Create the agent that generates execution strategies. **Depends on ZenLedgerClient interface (Block 1.7.1) for cost-aware model selection.** Queries historical efficiency data to choose optimal model for each task type. Enforces budget limits via GetCostBudgetStatus(). Generates SR&ED hypothesis framing by default.

**Block 2.6: Human Gatekeeper.** Implement approval and feedback mechanisms.

**Checkpoint 2 (Office Operational):** A work item from any source is received via ZenOffice interface. Jira tickets are analyzed. Planner generates task breakdowns. All Office actions write to ZenJournal.

---

### Block 3: The Nervous System (Connectivity)

**Purpose:** Establish the connectivity layer that connects Office and Factory.

**Prerequisite:** Block 1 complete

**Components:**

**Block 3.1: Message Bus.** Implement pub/sub for event distribution. Use zen-sdk/pkg/dedup for event deduplication. Choose NATS, Redis Streams, or Kafka.

**Block 3.2: State Synchronization.** Implement mechanisms to keep state consistent. Implement caching with invalidation.

**Block 3.3: ZenJournal Implementation.** Build the event store service using zen-sdk/pkg/receiptlog as foundation. Add zen-brain-specific event types. Implement query APIs.

**Block 3.4: API Server.** Implement the REST/GraphQL API surface. Health and readiness endpoints using zen-sdk/pkg/health. Authentication and authorization. Add OpenTelemetry tracing via zen-sdk/pkg/observability.

**Block 3.5: KB / QMD Adapter and Index Orchestration.**
Build the default KB retrieval path around Git + qmd, not a custom database-backed ingestion service.

**Source of Truth:**
- Git repositories (for example `zen-docs`) are the canonical KB source of truth.
- Confluence, if used later, is a view/publishing surface and not the canonical write path.

**Default Runtime Path:**
- qmd is the default indexing/search engine for 1.0
- zen-brain implements a QMD adapter around qmd CLI/process execution and result parsing
- index refresh is triggered by repo updates or explicit refresh commands
- background/scheduled refresh may use `zen-sdk/pkg/scheduler`

**Non-Goals for 1.0:**
- No custom CockroachDB-backed KB/QMD implementation by default
- No graph/relationship layer in 1.0
- No requirement to sync KB into Confluence before internal usefulness

**Acceptance Criteria:**
- [ ] Git repo KB source-of-truth path documented
- [ ] qmd adapter implemented behind a small interface
- [ ] refresh/index orchestration implemented
- [ ] analyzer/planner can query KB through the adapter
- [ ] qmd failure path uses `zen-sdk/pkg/retry`
- [ ] KB quality tested with golden queries

**Block 3.6: ZenLedger Implementation (NEW in V6).** Build the token and cost accounting service. Implement TokenRecord schema in CockroachDB with indexes for all query patterns (model efficiency, project breakdown, SR&ED export). Implement TokenRecorder interface that worker agents call after every LLM call. Implement LocalInferenceCostCalculator for estimating local model costs. Create SQL views for required reports: model efficiency, task type cost profile, local vs API comparison, project cost breakdown, SR&ED cost export. Add ZenLedger dashboard to Grafana (see Section 3.14).

**Block 3.7: CockroachDB Provisioning and Migrations (NEW in V6).** Implement database provisioning before KB Ingestion Service needs it. Create `make db-up` target for local development (spins up single-node CockroachDB in k3d). Create `make db-migrate` target to run golang-migrate migrations. Create `make db-reset` target for clean slate during development. Ensure migrations run as init containers in production deployments. **This block must complete before Block 3.5 (KB Ingestion) and Block 3.6 (ZenLedger) can write to the database.**

**Checkpoint 3 (Connected):** Events flow Office -> Factory via Message Bus. State syncs within seconds. ZenJournal records all actions. KB is searchable with scope isolation. ZenLedger tracks all token usage with yield metrics. Database is provisioned and migrated.

---

### Block 4: The Factory (Execution)

**Purpose:** Build the Kubernetes-based execution environment where AI agents perform work.

**Prerequisite:** Block 1, Block 2.1, Block 3 complete

**CRITICAL DECOUPLING RULE:**
> **No Factory type, API, CRD, event schema, or controller may import or reference Jira-specific models, field names, statuses, or webhook payloads.** The Factory operates on canonical WorkItem types only. All Office-specific concepts are translated at the ZenOffice boundary.

**Components:**

**Block 4.0: Secrets and Credentials.** Set up secure credential management. Git SSH keys, CockroachDB credentials, Confluence/Jira tokens stored in Kubernetes Secrets. Encryption via zen-sdk/pkg/crypto.

**Block 4.1: Core CRDs.** Define and implement Kubernetes Custom Resource Definitions. BrainTask, BrainAgent, BrainQueue, BrainPolicy.

**Block 4.2: Foreman Controller.** Implement the Kubernetes operator for BrainTask reconciliation.

**Block 4.3: Worker Agents.** Implement the AI agents that do work. Each agent receives BrainTask, uses LLM Gateway to reason, produces Evidence.

**Block 4.4: ZenContext Implementation.** Implement the tiered memory system. Deploy Redis for Tier 1. Deploy vector database for Tier 2 QMD.

**Block 4.5: Evidence Vault.** Implement evidence collection and storage.

**Block 4.6: ZenGate Implementation.** Implement the admission controller. Validate BrainTask specs. Enforce ZenPolicy rules.

**Block 4.7: ZenGuardian Implementation.** Implement active monitoring. Circuit breaking, anomaly detection, safety boundaries.

**Block 4.8: Tmpfs Integration.** Configure agent pods with tmpfs scratch volumes.

**Block 4.9: Worktree Manager.** Implement git worktree lifecycle management. Create isolated worktrees per task/session. Implement cleanup on task completion or timeout.

**Block 4.10: Worker Pool.** Implement long-running worker deployment. Workers stay alive after task completion. Pre-load models on startup.

**Block 4.11: Session Affinity Dispatcher.** Route multi-step tasks to same worker. Track session -> worker mapping in CockroachDB. Handle worker death -> trigger ReMe. **NEW in V6: Cluster-aware routing** — dispatcher routes tasks to the correct cluster based on ZenProject config, not just the right worker within a cluster. Queries ZenCluster CRD to determine target cluster, then delegates to cluster-local dispatcher.

**Block 4.12: Shared Factory Floor.** Implement shared volume (PVC or hostPath). Configure bare git repo for worktrees.

**Block 4.13: Observability Integration.** Deploy observability stack per Section 3.9. Configure Prometheus scraping, Grafana dashboards, alerts. **NEW in V6: Add ZenLedger dashboard** (model efficiency, cost per project, local vs API breakdown, SR&ED cost accumulator).

**Block 4.14: Multi-Cluster Agent Deployment (NEW in V6).** Implement zen-brain-agent deployment pattern for data plane clusters. Agent is lightweight, maintains local ZenJournal shard, reports to control plane. Supports both local k3d clusters and cloud managed K8s. Configuration via ZenCluster and ZenProject CRDs.

**Checkpoint 4 (Factory Running):** End-to-end flow executes. Work item -> Office analysis -> BrainTask creation -> Agent execution -> Evidence collection -> Status update. Workers remain warm between tasks. Observability dashboards show system health.

---

### Block 5: Intelligence and Memory (Contextual Awareness)

**Purpose:** Give agents memory, rapid knowledge retrieval, and state reconstruction capabilities.

**Prerequisite:** Block 4.4, Block 3.3, Block 3.5 complete
> **Note:** Block 3.3 (ZenJournal) is explicitly required because ReMe depends on replaying journal history for state reconstruction.

**Components:**

**Block 5.1: QMD Population.** Populate the Question-Answer Memory Database with initial content. Review and curate source documents, validate scope assignments, test semantic search quality with golden set (per Section 3.12). **Depends on Block 3.5 (KB Ingestion Service).**

**Block 5.2: ReMe Protocol Implementation.** Implement the full Recursive Memory protocol. Agent startup reads ZenJournal (from Block 3.3), reconstructs causal chain, verifies state.

**Block 5.3: Agent-Context Binding.** Agents write intermediate thoughts to ZenContext. Implement context retrieval for continuation.

**Block 5.4: Funding Evidence Aggregator (OPTIONAL FOR 1.0 CUT).**
Generate SR&ED/IRAP report material from accumulated evidence in ZenJournal and ZenLedger.

**Important:** This block is valuable, but it must not block the first trustworthy internal-use vertical slice. If sequencing pressure exists, complete Blocks 2–4 and the first useful parts of Block 5 before implementing narrative/report-generation automation.

**Checkpoint 5 (Fully Aware):** Agents have memory. QMD returns relevant knowledge in <100ms. Agents recover from failures via ReMe protocol. SR&ED reports can be generated from accumulated evidence with cost breakdowns.

---

### Block 6: Developer Experience

**Purpose:** Ensure developers can run and debug zen-brain locally using k3d clusters (consistent with other zen projects).

**Prerequisite:** Block 3 complete

**Components:**

**Block 6.1: k3d Cluster Setup.** Create local development cluster using k3d (consistent with zen-lock, zen-flow, zen-watcher patterns). Cluster runs locally with all dependencies: CockroachDB (single-node insecure), Redis, helper services. Helper services from other zen projects (zen-lock, zen-flow, zen-watcher/zen-ingester) can run in the same cluster for integration testing without any code changes.

```bash
# Create dev cluster
k3d cluster create zen-brain-dev \
  -p "8080:80@loadbalancer" \
  -p "26257:26257@loadbalancer" \
  --registry-create zen-registry:5000

# Deploy dependencies
kubectl apply -f deployments/k3d/dependencies.yaml
```

**Block 6.2: Development Scripts.** Create scripts for common dev tasks:
- make dev-up - Start k3d cluster and deploy dependencies
- make dev-down - Stop k3d cluster
- make dev-clean - Reset databases (drop and recreate)
- make dev-logs - Tail all logs
- make dev-build - Build and load image into k3d registry

**Block 6.3: Local Configuration.** Create config.dev.yaml with sensible defaults for local development. ZEN_BRAIN_DEV=true enables development mode (single-node CockroachDB, insecure connections).

**Block 6.4: Debugging Guide.** Document how to debug workers, KB queries, LLM calls. Include k3d-specific debugging patterns.

**Checkpoint 6 (Developer Ready):** Developers can run zen-brain locally with `make dev-up`. k3d cluster mirrors production topology. Documentation explains debugging workflows.

---

## 5. Dependencies

```
Block 0: The Clean Foundation (No dependencies)
|-- 0.1 Create GitHub Repository
|-- 0.2 Create Local Repository
|-- 0.3 Define Configurable Home Directory
+-- 0.4 Document Cutover Plan

Block 0.5: Pre-requisite SDK Packages (depends on Block 0)
|-- 0.5.1 Audit Existing zen-sdk Packages
|-- 0.5.2 Migrate zen-lock/pkg/crypto to zen-sdk
+-- 0.5.3 Document zen-sdk Dependencies in zen-brain

Block 1: The Neuro-Anatomy (depends on Block 0, Block 0.5)
|-- 1.1 ZenJournal Schema Definition
|-- 1.2 ZenContext Architecture Design
|-- 1.3 ZenOffice Interface Definition
|-- 1.4 ZenPolicy and ZenGate Schema Design
|-- 1.5 Canonical Work Taxonomy and Jira Mapping (includes SR&ED tags, AIAttribution)
|-- 1.6 Repository and Build Infrastructure
|-- 1.7 LLM Gateway Interface
|-- 1.7.1 ZenLedger Query Interface (NEW in V6 - Planner depends on this)
|-- 1.8 SR&ED/IRAP Alignment Design (NEW in V6)
+-- 1.9 Multi-Cluster CRD Design (NEW in V6)

Block 2: The Office (depends on Block 1)
|-- 2.1 ZenOffice Interface Implementation
|-- 2.2 Jira Connector (depends on 2.1, 1.5) [includes AI Attribution]
|-- 2.3 Intent Analyzer (depends on 1.7)
|-- 2.4 Session Manager
|-- 2.5 Planner Agent (depends on 2.3, 2.4, 1.7.1) [SR&ED framing default ON, cost-aware via ZenLedgerClient interface]
+-- 2.6 Human Gatekeeper

Block 3: The Nervous System (depends on Block 1)
|-- 3.1 Message Bus
|-- 3.2 State Synchronization
|-- 3.3 ZenJournal Implementation (depends on 1.1) [includes experiment-class events]
|-- 3.4 API Server
|-- 3.5 KB / QMD Adapter and Index Orchestration (depends on 1.7, 3.3)
|-- 3.6 ZenLedger Implementation (depends on 3.7) (NEW in V6)
+-- 3.7 CockroachDB Provisioning and Migrations (NEW in V6)

Block 4: The Factory (depends on Block 1, Block 2.1, Block 3)
|-- 4.0 Secrets and Credentials
|-- 4.1 Core CRDs
|-- 4.2 Foreman Controller
|-- 4.3 Worker Agents [emit TokenRecords to ZenLedger]
|-- 4.4 ZenContext Implementation (depends on 1.2)
|-- 4.5 Evidence Vault [includes SR&ED evidence classes]
|-- 4.6 ZenGate Implementation (depends on 1.4)
|-- 4.7 ZenGuardian Implementation
|-- 4.8 Tmpfs Integration
|-- 4.9 Worktree Manager
|-- 4.10 Worker Pool (depends on 4.8, 4.9)
|-- 4.11 Session Affinity Dispatcher (depends on 4.10) [cluster-aware]
|-- 4.12 Shared Factory Floor (depends on 4.10)
|-- 4.13 Observability Integration [includes ZenLedger dashboard]
+-- 4.14 Multi-Cluster Agent Deployment (NEW in V6)

Block 5: Intelligence and Memory (depends on Block 4.4, Block 3.3, Block 3.5, Block 3.6)
|-- 5.1 QMD Population (depends on 3.5)
|-- 5.2 ReMe Protocol Implementation (depends on 3.3 for journal replay)
|-- 5.3 Agent-Context Binding
+-- 5.4 Funding Evidence Aggregator (depends on 3.3, 3.6, 4.5) (NEW in V6)

Block 6: Developer Experience (depends on Block 3)
|-- 6.1 k3d Cluster Setup
|-- 6.2 Development Scripts (includes make db-up, make db-migrate)
|-- 6.3 Local Configuration
+-- 6.4 Debugging Guide
```

**Critical Path:** Block 0 -> Block 0.5 -> Block 1 -> Block 2.1 -> Block 3 -> Block 4 -> Block 5

---

## 6. Reuse Strategy

**Import from zen-sdk (Do Not Reimplement):**
- Logging (zen-sdk/pkg/logging)
- Retry logic (zen-sdk/pkg/retry)
- Deduplication (zen-sdk/pkg/dedup)
- Dead letter queue (zen-sdk/pkg/dlq)
- Scheduling (zen-sdk/pkg/scheduler)
- Tracing (zen-sdk/pkg/observability)
- Health checks (zen-sdk/pkg/health)
- Leader election (zen-sdk/pkg/leader)
- Event recording (zen-sdk/pkg/events)
- Receipt ledger (zen-sdk/pkg/receiptlog)
- Encryption (zen-sdk/pkg/crypto)

**Reference Only (Read, Do Not Copy):**
- Logging utilities from 0.1 (use zen-sdk instead)
- Configuration patterns from 0.1 (improve in 1.0)
- Jira integration from 0.1 (reference for understanding)
- LLM provider adapters from 0.1 (extend for new providers)

**Deprecated (Do Not Reference):**
- Gateway server (replaced by new API Server)
- Legacy queue system (replaced by Message Bus)
- Old consensus voting (replaced by new design)

**Build in zen-sdk First:**
When zen-brain needs a new cross-cutting capability:
1. Evaluate if it belongs in zen-sdk (reusable across projects)
2. If yes, build in zen-sdk first with tests and documentation
3. Then import in zen-brain

---

## 7. Testing Strategy

### 7.1 Unit Tests
Test individual functions in isolation. Target 80% coverage for business logic.

### 7.2 Integration Tests
Test component interactions against a real Kubernetes API using local k3d as the default development/integration environment.

### 7.3 Vertical Slice Acceptance Tests (NEW in V6.1)
The first trusted goal of zen-brain 1.0 is one complete internal-useful vertical slice:

Office intake -> analyze -> plan -> session -> factory execution -> proof-of-work -> status update

These tests are first-class and must pass before widening scope.

### 7.4 End-to-End Tests
Test complete workflows across multiple components. Include a test Jira instance or controlled Jira workspace. Run the full pipeline.

### 7.5 Chaos and Reliability Tests
Test adverse conditions: pod failures, worker crashes, transient provider errors, and network partitions. Verify recovery through ZenJournal / ReMe-compatible reconstruction.

### 7.6 KB Quality Tests
Test KB search quality with golden set queries. Validate qmd-backed retrieval against known expected results.

### 7.7 Flow/Policy Regression Tests (NEW in V6.1)
Validate canonical policy and routing behavior:
- tag/category interpretation
- approval-required vs autonomous behavior
- handoff policy
- tool-binding enforcement
- provider escalation paths
- hidden-vs-visible runtime behavior where relevant## 8. Risk Assessment

### 8.1 Technical Risks
- **LLM Provider Reliability:** Implement retry logic via zen-sdk/pkg/retry, caching, multiple providers
- **Kubernetes Complexity:** Use kubebuilder/Operator SDK
- **Intent Analyzer Difficulty:** Start simple, iterate based on real data
- **Embedding Model Performance:** Start with nomic-embed-text, can switch later via interface

### 8.2 Architectural Risks
- **Schema Design Flaws:** Thorough design review before implementation
- **Abstraction Leakage:** Enforce ZenOffice interface discipline

### 8.3 Operational Risks
- **Observability Gaps:** Implement observability from Block 1 via zen-sdk/pkg/observability
- **Secrets Management:** Use Block 4.0 patterns with zen-sdk/pkg/crypto
- **Data Loss:** CockroachDB backups for structured runtime data (for example ZenLedger) + KB regeneration from git/qmd refresh path

---

## 9. Quick Reference

### Block Summary

| Block | Name | Purpose | Depends On |
|-------|------|---------|------------|
| 0 | Clean Foundation | Fresh repo, configurable paths | Nothing |
| 0.5 | Pre-requisite SDK | zen-sdk packages ready and wired as mandatory dependencies | Block 0 |
| 1 | Neuro-Anatomy | All schemas, interfaces, taxonomy, policy/gate design, multi-cluster CRDs | Block 0, 0.5 |
| 2 | Office | Intent capture via ZenOffice, AI attribution in Jira, planning entrypoint | Block 1 |
| 3 | Nervous System | Message bus, ZenJournal, API, KB/QMD adapter, ZenLedger, runtime plumbing | Block 1 |
| 4 | Factory | Kubernetes execution, warm workers, cluster-aware dispatch, proof-of-work | Block 1, 2.1, 3 |
| 5 | Intelligence | ReMe, agent memory, model routing, optional evidence/report generation | Block 4.4, 3.3, 3.5, 3.6 |
| 6 | Developer Experience | k3d cluster setup, debugging, db scripts, local dev paths | Block 3 |

### zen-sdk Packages Used

| Package | Purpose |
|---------|---------|
| receiptlog | ZenJournal (immutable event ledger) |
| scheduler | KB ingestion, Confluence sync scheduling |
| dedup | Message bus event deduplication |
| dlq | Failed task handling |
| observability | OpenTelemetry tracing |
| retry | LLM API, KB query retries |
| events | Kubernetes event recording |
| leader | Leader election for HA |
| logging | Structured logging |
| health | Health/readiness probes |
| crypto | Secrets encryption (age) |

### V6 New Components Summary

| Component | Block | Purpose |
|-----------|-------|---------|
| ZenLedger | 3.6 | Token/cost accounting with yield metrics |
| ZenLedgerClient | 1.7.1 | Interface for Planner to query cost data (implemented by 3.6) |
| ZenProject/ZenCluster CRDs | 1.9 | Multi-project, multi-cluster topology |
| AIAttribution | 1.5, 2.2 | Structured AI attribution in Jira |
| Experiment-Class Events | 1.1, 3.3 | SR&ED evidence in ZenJournal |
| FundingReportRole | 5.4 | SR&ED/IRAP report generation |
| SR&ED Tags | 1.5 | U1-U4 uncertainty categorization |
| CockroachDB Provisioning | 3.7 | Database setup before KB/ZenLedger |
| Model Selection Policy | 2.5 | Cost-aware model routing |
| Cluster-Aware Dispatcher | 4.11 | Multi-cluster task routing |

### Cutover Quick Reference

```bash
# Stop 0.1
systemctl --user stop zen-brain

# Archive 0.1 home
mv ~/.zen/zen-brain ~/.zen/zen-brain-0.1-archived

# Switch 1.0 to production home
export ZEN_BRAIN_HOME=~/.zen/zen-brain

# Or: promote 1.0 home
mv ~/.zen/zen-brain-1.0 ~/.zen/zen-brain
```

### Local Development Quick Reference

```bash
# Start k3d cluster with dependencies
make dev-up

# Set development mode
export ZEN_BRAIN_DEV=true

# Build and load image into k3d registry
make dev-build

# Deploy zen-brain to dev cluster
kubectl apply -f deployments/k3d/zen-brain-dev.yaml

# View logs
make dev-logs

# Stop when done
make dev-down
```

## 10. 1.1 Radar (Do Not Lose)

These are intentionally not required to make 1.0 internally useful, but they are strategically important and must stay visible:

1. **Agent Sandbox**
   - Non-destructive evaluation lane
   - Agents can plan, simulate, and score behavior without making real external changes
   - Used for dogfooding, policy validation, and trust-building before granting higher authority

2. **Small-Model / MLQ Strategy**
   - CPU-first high-throughput worker lane
   - Qwen 3.5 0.8B-style worker optimization
   - model routing / escalation to stronger models
   - warmup, calibration, memory shaping, ReMe optimization
   - future fine-tuning/distillation research

3. **Ops Department**
   - Dedicated operational domain for incidents, problems, changes, deploy coordination, and launch-readiness work
   - likely centered in one Jira space initially
   - intended to reduce toil for Zen-Mesh and adjacent operations

4. **Compliance Overlays**
   - SOC 2-oriented controls
   - ISO 27k-oriented controls
   - FedRAMP-style future posture
   - profile-driven, not bespoke project hacks

---

**Document Status:** Ready for Execution (Version 6.1)
**Last Updated:** 2026-03-07
**Changes in V6.1:**
- Tightened mandatory zen-sdk reuse rules
- Corrected KB/QMD direction to Git + qmd default path
- Corrected testing strategy to k3d + vertical-slice acceptance
- Added future control-model vocabulary
- Added 1.1 radar items: agent sandbox, small-model strategy, Ops Department, compliance overlays
- Demoted Funding Evidence Aggregator from blocking the first trustworthy vertical slice
