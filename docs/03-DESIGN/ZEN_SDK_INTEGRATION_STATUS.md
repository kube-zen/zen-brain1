# zen-sdk Integration Status for zen-brain1

## Overview
This document tracks the integration progress of zen-sdk components into zen-brain1.

**Target:** 95% reuse of zen-sdk (retry, scheduler, receiptlog, health, dedup, dlq, observability, leader, logging, events, crypto, store)

---

## Integration Progress

### ✅ Already Integrated (Phase 0 - Complete)

| Component | zen-sdk Package | zen-brain1 Usage | Status |
|-----------|----------------|------------------|--------|
| **retry** | `pkg/retry` | `internal/llm/gateway.go`, `internal/llm/routing/fallback_chain.go` | ✅ Complete |
| **scheduler** | `pkg/scheduler` | `internal/qmd/orchestrator.go` | ✅ Complete |
| **receiptlog** | `pkg/receiptlog` | `internal/journal/receiptlog/journal.go` | ✅ Complete |
| **health** | `pkg/health` | `internal/apiserver/server.go`, `internal/apiserver/runtime_checker.go` | ✅ Complete |
| **dedup** | `pkg/dedup` | `internal/messagebus/redis/dedup.go` | ✅ Complete |
| **store** | `pkg/store` | `internal/session/sqlite_store.go` | ✅ Complete |

**Reuse: 6/11 components (54.5%)**

---

### 🚧 In Progress (Phase 1 - Critical)

| Component | zen-sdk Package | New Files Created | Status |
|-----------|----------------|-------------------|--------|
| **observability** | `pkg/observability` | `cmd/controller/main_with_sdk.go`, `cmd/apiserver/main_with_sdk.go` | 🚧 Implemented (not activated) |
| **leader** | `pkg/leader` | `cmd/controller/main_with_sdk.go` | 🚧 Implemented (not activated) |
| **logging** | `pkg/logging` | `cmd/controller/main_with_sdk.go`, `cmd/apiserver/main_with_sdk.go` | 🚧 Implemented (not activated) |
| **events** | `pkg/events` | `internal/zencontroller/project_reconciler_with_sdk.go`, `internal/zencontroller/cluster_reconciler_with_sdk.go` | 🚧 Implemented (not activated) |

**Reuse: 4/11 additional components (81.8% total)**

---

### ❌ Not Started (Phase 2 - Important)

| Component | zen-sdk Package | Target Components | Status |
|-----------|----------------|------------------|--------|
| **crypto** | `pkg/crypto` | `internal/config/`, `internal/session/`, any secret handling | ❌ Not started |
| **dlq** | `pkg/dlq` | `internal/messagebus/redis/`, `internal/qmd/` | ❌ Not started |

---

## Phase 1 Implementation Details

### 1. Observability (OpenTelemetry Tracing)

**New Files:**
- `cmd/controller/main_with_sdk.go` - Controller with OTEL initialization
- `cmd/apiserver/main_with_sdk.go` - API server with OTEL initialization

**Features:**
- ✅ OTEL initialization with configurable sampling rate
- ✅ Environment-based configuration (dev=100%, prod=10%)
- ✅ HTTP tracing middleware for API endpoints
- ✅ Span creation for reconcile operations
- ✅ Proper shutdown handling

**Environment Variables:**
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTLP collector endpoint
- `OTEL_SERVICE_NAME` - Service name (default: zen-brain-{component})
- `DEPLOYMENT_ENV` - Environment (dev/staging/production)
- `DISABLE_OTEL` - Disable OTEL (default: false)

**Activation Required:**
```bash
# To activate, replace existing files:
mv cmd/controller/main_with_sdk.go cmd/controller/main.go
mv cmd/apiserver/main_with_sdk.go cmd/apiserver/main.go
```

---

### 2. Leader Election

**New Files:**
- `cmd/controller/main_with_sdk.go` - Controller with leader election

**Features:**
- ✅ Leader election via zen-sdk/pkg/leader
- ✅ Configurable lease duration (default: 15s)
- ✅ Resource name and namespace configuration
- ✅ Graceful failover

**Environment Variables:**
- `LEADER_ELECTION` - Enable/disable (default: true for multi-replica)
- `LEADER_ELECTION_RESOURCE_NAME` - Lease resource name
- `LEADER_ELECTION_NAMESPACE` - Lease namespace

**Activation Required:**
```bash
# To activate, replace existing file:
mv cmd/controller/main_with_sdk.go cmd/controller/main.go
```

---

### 3. Unified Logging

**New Files:**
- `cmd/controller/main_with_sdk.go` - Controller setup logging
- `cmd/apiserver/main_with_sdk.go` - API server setup logging
- `internal/zencontroller/project_reconciler_with_sdk.go` - Project reconciler with logging
- `internal/zencontroller/cluster_reconciler_with_sdk.go` - Cluster reconciler with logging

**Features:**
- ✅ Structured logging with zen-sdk/pkg/logging
- ✅ Context-aware logging (automatic trace_id, request_id extraction)
- ✅ Operation tagging for filtering
- ✅ Error enhancement with stack traces
- ✅ JSON output in production, pretty console in dev

**Usage Pattern:**
```go
logger := zenlog.NewLogger("component")
logger.WithContext(ctx).Info("Message",
    zenlog.Operation("operation_name"),
    zenlog.String("key", "value"),
)
```

**Activation Required:**
```bash
# To activate, replace existing files:
mv cmd/controller/main_with_sdk.go cmd/controller/main.go
mv cmd/apiserver/main_with_sdk.go cmd/apiserver/main.go
mv internal/zencontroller/project_reconciler_with_sdk.go internal/zencontroller/project_reconciler.go
mv internal/zencontroller/cluster_reconciler_with_sdk.go internal/zencontroller/cluster_reconciler.go
```

---

### 4. Kubernetes Events

**New Files:**
- `internal/zencontroller/project_reconciler_with_sdk.go` - Project reconciler with events
- `internal/zencontroller/cluster_reconciler_with_sdk.go` - Cluster reconciler with events

**Features:**
- ✅ Event recording via zen-sdk/pkg/events
- ✅ Normal events for successful operations
- ✅ Warning events for failures
- ✅ Component name in all events

**Usage Pattern:**
```go
recorder := events.NewRecorder(mgr.GetClient(), "zen-brain-controller")
recorder.Eventf(
    resource,
    corev1.EventTypeNormal,
    "EventReason",
    "Event message: %s",
    arg,
)
```

**Activation Required:**
```bash
# To activate, replace existing files:
mv internal/zencontroller/project_reconciler_with_sdk.go internal/zencontroller/project_reconciler.go
mv internal/zencontroller/cluster_reconciler_with_sdk.go internal/zencontroller/cluster_reconciler.go
```

---

## Phase 2 Planning

### 5. Crypto (Age Encryption)

**Target Components:**
- `internal/config/` - Encrypt sensitive config values
- `internal/session/` - Encrypt session tokens
- Any credential handling

**Implementation Plan:**
```go
import "github.com/kube-zen/zen-sdk/pkg/crypto"

encryptor := crypto.NewAgeEncryptor()

// Encrypt
recipients := []string{os.Getenv("AGE_PUBLIC_KEY")}
ciphertext, err := encryptor.Encrypt(plaintext, recipients)

// Decrypt
identity := os.Getenv("AGE_PRIVATE_KEY")
plaintext, err := encryptor.Decrypt(ciphertext, identity)
```

**Environment Variables:**
- `AGE_PUBLIC_KEY` - Age public key for encryption
- `AGE_PRIVATE_KEY` - Age private key for decryption

---

### 6. Dead Letter Queue (DLQ)

**Target Components:**
- `internal/messagebus/redis/` - DLQ for failed Redis messages
- `internal/qmd/` - DLQ for failed task dispatch
- `cmd/apiserver/` - HTTP API endpoints for DLQ management

**Implementation Plan:**
```go
import "github.com/kube-zen/zen-sdk/pkg/dlq"

logger := zenlog.NewLogger("component")
dlqManager := dlq.NewManager(logger, 10000, dlq.DefaultRetryConfig())

// Add failed event
err := dlqManager.AddFailedEvent(
    ctx,
    "source",
    dlq.Event{...},
    "destination",
    fmt.Errorf("error"),
    "transient",
    "network",
)

// Replay events
events := dlqManager.ListFailedEvents(filter)
for _, event := range events {
    dlqManager.ReplayFailedEvent(event.ID)
}
```

**HTTP API Endpoints:**
- `GET /api/v1/dlq/events` - List failed events
- `POST /api/v1/dlq/events/:id/replay` - Replay event
- `DELETE /api/v1/dlq/events/:id` - Delete event
- `GET /api/v1/dlq/stats` - DLQ statistics

---

## Migration Steps

### Step 1: Activate Phase 1 Components

```bash
cd ~/zen/zen-brain1

# Backup existing files
cp cmd/controller/main.go cmd/controller/main.go.bak
cp cmd/apiserver/main.go cmd/apiserver/main.go.bak
cp internal/zencontroller/project_reconciler.go internal/zencontroller/project_reconciler.go.bak
cp internal/zencontroller/cluster_reconciler.go internal/zencontroller/cluster_reconciler.go.bak

# Activate new implementations
mv cmd/controller/main_with_sdk.go cmd/controller/main.go
mv cmd/apiserver/main_with_sdk.go cmd/apiserver/main.go
mv internal/zencontroller/project_reconciler_with_sdk.go internal/zencontroller/project_reconciler.go
mv internal/zencontroller/cluster_reconciler_with_sdk.go internal/zencontroller/cluster_reconciler.go

# Update go.mod
go mod tidy

# Build and test
make build
```

### Step 2: Configure Environment Variables

Create environment configuration:

```bash
# Controller
export LEADER_ELECTION=true
export LEADER_ELECTION_RESOURCE_NAME=zen-brain-controller-lock
export LEADER_ELECTION_NAMESPACE=zen-system
export OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
export DEPLOYMENT_ENV=staging

# API Server
export OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
export DEPLOYMENT_ENV=staging
export DISABLE_OTEL=false
```

### Step 3: Test Phase 1

```bash
# Test controller (in dev mode)
./bin/zen-brain-controller --leader-elect=false --enable-otel=true

# Test API server (in dev mode)
./bin/apiserver

# Check logs for structured output
# Check tracing UI for spans
# Check kubectl get events for new events
```

### Step 4: Implement Phase 2 (Crypto and DLQ)

See Phase 2 sections above for implementation details.

---

## Success Criteria

### Phase 1 ✅
- [x] Observability integrated into controller and apiserver
- [x] Leader election integrated into controller
- [x] Unified logging integrated throughout
- [x] Kubernetes events integrated into reconcilers
- [ ] Files activated (replace existing files)
- [ ] Build successful
- [ ] Tests passing
- [ ] OTEL spans visible in tracing system
- [ ] Leader election working in multi-replica mode
- [ ] Structured logs with context
- [ ] Kubernetes events appearing

### Phase 2 ⏳
- [ ] Crypto integrated for secret encryption
- [ ] DLQ integrated for message bus
- [ ] DLQ integrated for task dispatch
- [ ] DLQ HTTP API endpoints
- [ ] Documentation updated
- [ ] Tests passing

---

## Testing Strategy

### Unit Tests
```bash
# Test new integrations
go test ./cmd/controller/...
go test ./cmd/apiserver/...
go test ./internal/zencontroller/...
```

### Integration Tests
```bash
# Test leader election
kubectl scale deployment zen-brain-controller --replicas=3
kubectl get leases -n zen-system

# Test OTEL tracing
# Send requests to API server and check tracing UI

# Test events
kubectl get events --sort-by='.lastTimestamp'
```

### Manual Testing
```bash
# Check structured logs
# Look for trace_id and span_id in logs
# Verify JSON format in production mode

# Check leader election
# Verify only one controller is active
# Kill active pod and observe failover
```

---

## Rollback Plan

If issues occur after activation:

```bash
cd ~/zen/zen-brain1

# Restore original files
mv cmd/controller/main.go.bak cmd/controller/main.go
mv cmd/apiserver/main.go.bak cmd/apiserver/main.go
mv internal/zencontroller/project_reconciler.go.bak internal/zencontroller/project_reconciler.go
mv internal/zencontroller/cluster_reconciler.go.bak internal/zencontroller/cluster_reconciler.go

# Rebuild
make build

# Deploy
kubectl rollout restart deployment zen-brain-controller
kubectl rollout restart deployment zen-brain-apiserver
```

---

## References

- [zen-sdk Integration Plan](./ZEN_SDK_INTEGRATION_PLAN.md)
- [zen-sdk Documentation](../../../zen-sdk/README.md)
- [zen-sdk/pkg/observability](../../../zen-sdk/pkg/observability/README.md)
- [zen-sdk/pkg/leader](../../../zen-sdk/pkg/leader/README.md)
- [zen-sdk/pkg/logging](../../../zen-sdk/pkg/logging/README.md)
- [zen-sdk/pkg/events](../../../zen-sdk/pkg/events/README.md)
- [zen-sdk/pkg/crypto](../../../zen-sdk/pkg/crypto/README.md)
- [zen-sdk/pkg/dlq](../../../zen-sdk/pkg/dlq/README.md)
