# Block 0.5: SDK Audit and Reusable Assets Plan

**Status:** Planning Complete
**Date:** 2026-03-07
**Purpose:** Document zen-sdk packages and zen-brain 0.1 assets for reuse in zen-brain 1.0

---

## 1. zen-sdk Package Audit

### 1.1 Available Packages (Verified 2026-03-07)

| Package | Location | Status | Notes |
|---------|----------|--------|-------|
| **receiptlog** | `pkg/receiptlog/` | ✅ 2 files | Core for ZenJournal - chain hashes, S3 upload |
| **scheduler** | `pkg/scheduler/` | ✅ 2 files | Cron + one-time job scheduling |
| **dedup** | `pkg/dedup/` | ✅ 2 files | Message bus event deduplication |
| **dlq** | `pkg/dlq/` | ✅ 3 files | Dead letter queue with retry |
| **observability** | `pkg/observability/` | ✅ 1 file | OpenTelemetry tracing |
| **retry** | `pkg/retry/` | ✅ 3 files | Exponential backoff |
| **events** | `pkg/events/` | ✅ 1 file | K8s event recording |
| **leader** | `pkg/leader/` | ✅ 1 file | Leader election |
| **logging** | `pkg/logging/` | ✅ 11 files | Structured logging |
| **health** | `pkg/health/` | ✅ 1 file | Health/readiness probes |
| **crypto** | `pkg/crypto/` | ✅ 1 file | Age encryption (migrated from zen-lock) |
| **store** | `pkg/store/` | ✅ 1 file | SQLite helper with proper pragmas |
| **http** | `pkg/http/` | ✅ 1 file | Shared HTTP client with retry/timeout |
| **lifecycle** | `pkg/lifecycle/` | ✅ 2 files | Shutdown handling, finalizers |
| **webhook** | `pkg/webhook/` | ✅ 1 file | Webhook handling |
| **problemdetails** | `pkg/problemdetails/` | ✅ 1 file | RFC 7807 problem details |
| **filter** | `pkg/filter/` | ✅ 6 files | Filter expression parsing |
| **objectstore** | `pkg/objectstore/` | ✅ 3 files | S3-compatible object storage |
| **zenlead** | `pkg/zenlead/` | ✅ 2 files | ZenLedger client config |
| **config** | `pkg/config/` | ✅ Available | Configuration management |
| **controller** | `pkg/controller/` | ✅ Available | Controller patterns |
| **metrics** | `pkg/metrics/` | ✅ Available | Prometheus metrics |
| **k8s** | `pkg/k8s/` | ✅ Available | Kubernetes client utilities |
| **errors** | `pkg/errors/` | ✅ Available | Error handling |
| **endpoints** | `pkg/endpoints/` | ✅ Available | Endpoint registry |
| **secrets** | `pkg/secrets/` | ✅ Available | Secrets utilities |
| **gc** | `pkg/gc/` | ✅ Available | Garbage collection patterns |

### 1.2 Package Categories

**Infrastructure (Use from zen-sdk):**
- `store` - SQLite with WAL, busy_timeout
- `http` - Shared HTTP client
- `retry` - Exponential backoff
- `crypto` - Age encryption for secrets
- `lifecycle` - Graceful shutdown
- `health` - Health endpoints

**Event Processing (Use from zen-sdk):**
- `receiptlog` - **CORE for ZenJournal** - immutable ledger with chain hashes
- `dedup` - Event deduplication
- `dlq` - Dead letter queue

**Observability (Use from zen-sdk):**
- `events` - K8s event recording
- `leader` - Leader election for HA
- `logging` - Structured logging (where wired)

### 1.3 Actions Required

1. **Verify scheduler package** - Check if `pkg/scheduler/` exists
2. **Verify observability package** - Check if `pkg/observability/` exists
3. **Verify retry package** - Check if `pkg/retry/` exists
4. **Update zen-sdk version** - Current zen-brain uses `v0.2.12-alpha`

---

## 2. zen-brain 0.1 Reusable Assets

### 2.1 AI Interface (HIGH VALUE - Adapt, don't copy)

**Location:** `~/zen/zen-brain/internal/ai/interface.go`

**Key Types:**
```go
// These are good designs - adapt for zen-brain1
type Message struct {
    Role             string
    Content          string
    ReasoningContent string  // For chain-of-thought
    ToolCalls        []ToolCall
    ToolCallID       string
    Metadata         map[string]interface{}
}

type ToolCall struct {
    ID   string
    Name string
    Args map[string]interface{}
}

type Tool struct {
    Name        string
    Description string
    Parameters  map[string]interface{}
}

type ChatRequest struct {
    Messages      []Message
    Tools         []Tool
    Model         string
    Temperature   float64
    MaxTokens     int
    ThinkingLevel ThinkingLevel  // off/low/medium/high
    Stream        bool
    SkipWarden    bool
}

type ChatResponse struct {
    Content          string
    ReasoningContent string
    FinishReason     string
    ToolCalls        []ToolCall
}

type Provider interface {
    Name() string
    SupportsTools() bool
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) (*ChatResponse, error)
}
```

**Action:** Copy the interface design, but place in `pkg/llm/` per construction plan. This becomes the **LLM Gateway Interface (Block 1.7)**.

### 2.2 Agent Core (HIGH VALUE - Adapt patterns)

**Location:** `~/zen/zen-brain/internal/agent/agent.go`

**Key Patterns:**
- `AICaller` interface - abstraction for AI calls
- `Tool` interface - standard tool definition
- `ProgressCallback` - progress event streaming
- Run limits (budget exceeded, cycle detection) - H101/H102
- Tool result caching for read-only tools
- Parallel execution of read-only tools

**Key Interfaces:**
```go
type AICaller interface {
    Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error)
    ChatStream(ctx context.Context, req ai.ChatRequest, callback ai.StreamCallback) (*ai.ChatResponse, error)
}

type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]interface{}
    Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}
```

**Action:** These patterns are solid. Adapt for Factory execution model (K8s-based).

### 2.3 Architecture Documentation (REFERENCE)

**Location:** `~/zen/zen-brain/docs/01-architecture/`

Key documents:
- `ARCHITECTURE_OVERVIEW.md` - Complete system architecture
- `ARCHITECTURE.md` - Detailed component breakdown
- `COMMANDS_ARCHITECTURE.md` - Command system design

**Action:** Use as reference for understanding patterns, but don't copy architecture. V6 plan supersedes.

### 2.4 SDK Adoption Lessons (REFERENCE)

**Location:** `~/zen/zen-brain/docs/SDK_ADOPTION.md`

**Completed Migrations in 0.1:**
- G067: SQLite → zen-sdk/pkg/store
- G068: Retry → zen-sdk/pkg/retry
- G069: HTTP client → zen-sdk/pkg/http

**Lessons Learned:**
- SQLite pragmas must be consistent (WAL, busy_timeout)
- Retry needs custom `RetryableErrors` for AI/HTTP
- HTTP client should be shared across providers

**Action:** Apply these lessons from day one in zen-brain1.

### 2.5 Config Patterns (ADAPT)

**Location:** `~/zen/zen-brain/internal/config/`

**Good Patterns:**
- YAML-based configuration
- Environment variable overrides
- Provider/model configuration
- Feature flags

**Avoid:**
- Hardcoded paths
- Global state

**Action:** Design clean config system in Block 1.6.

---

## 3. Construction Plan V6 - Interface Requirements

### 3.1 Block 1 Interface Dependencies

These interfaces MUST be defined in Block 1 before implementation:

| Interface | Block | Purpose | zen-brain 0.1 Reference |
|-----------|-------|---------|-------------------------|
| **Provider** | 1.7 | LLM abstraction | `internal/ai/interface.go` |
| **ZenLedgerClient** | 1.7.1 | Cost lookup for Planner | NEW in V6 |
| **ZenOffice** | 1.3 | Work ingress abstraction | Concept from 0.1 |
| **ZenContext** | 1.2 | Tiered memory | Concept from 0.1 |
| **ZenJournal** | 1.1 | Event ledger | Uses zen-sdk/receiptlog |

### 3.2 zen-sdk Package Dependencies by Block

| Block | zen-sdk Packages Required |
|-------|---------------------------|
| Block 1 | (none - schema design only) |
| Block 2 | `retry`, `http`, `crypto` |
| Block 3 | `receiptlog`, `dedup`, `dlq`, `store`, `health`, `observability` |
| Block 4 | `events`, `leader`, `lifecycle`, `crypto` |
| Block 5 | `receiptlog` (for ReMe), `store` |
| Block 6 | (dev tooling - no runtime deps) |

---

## 4. Recommended Execution Order

### Phase 1: Verify zen-sdk (30 min)
1. Check if `scheduler`, `observability`, `retry`, `logging`, `health` packages exist
2. Document any gaps
3. Update zen-sdk if needed

### Phase 2: Create Interface Scaffolds (Block 1 prep)
1. `pkg/llm/provider.go` - Copy/adapt Provider interface from 0.1
2. `pkg/llm/types.go` - Copy/adapt Message, ToolCall, Tool types
3. `pkg/journal/interface.go` - ZenJournal interface (wraps receiptlog)
4. `pkg/office/interface.go` - ZenOffice interface
5. `pkg/context/interface.go` - ZenContext interface

### Phase 3: Update Dependencies
1. Add zen-sdk to go.mod (or use local replace for development)
2. Document required packages in README

---

## 5. Decision Log

### 5.1 What to Reuse
- **AI types (Message, ToolCall, Tool, ChatRequest/Response)** - Good design, adapt
- **Provider interface** - Solid abstraction, adapt
- **Tool interface pattern** - Good design, adapt
- **Architecture patterns** - Reference only, don't copy

### 5.2 What NOT to Reuse
- **Gateway server** - Replaced by new API Server design
- **Session store** - New design in V6
- **Queue system** - New design in V6
- **Consensus engine** - Not in V6 scope
- **Any hardcoded paths** - V6 requires configurable home

### 5.3 What to Build in zen-sdk First
- Nothing identified yet - current packages appear sufficient
- If new cross-cutting concerns emerge, build in zen-sdk first

---

## 6. Next Steps

1. **Verify zen-sdk packages** - Run verification script
2. **Begin Block 1** - Neuro-Anatomy (schemas and interfaces)
3. **Start with Provider interface** - Most reusable from 0.1

---

## Appendix A: Package Verification Script

```bash
#!/bin/bash
# Verify zen-sdk packages exist and are usable

SDK_PATH="$HOME/zen/zen-sdk"

packages=(
    "receiptlog"
    "scheduler"
    "dedup"
    "dlq"
    "observability"
    "retry"
    "events"
    "leader"
    "logging"
    "health"
    "crypto"
    "store"
    "http"
    "lifecycle"
)

echo "Checking zen-sdk packages..."
for pkg in "${packages[@]}"; do
    if [ -d "$SDK_PATH/pkg/$pkg" ]; then
        go_files=$(find "$SDK_PATH/pkg/$pkg" -name "*.go" ! -name "*_test.go" | wc -l)
        echo "✅ $pkg ($go_files files)"
    else
        echo "❌ $pkg - NOT FOUND"
    fi
done
```
