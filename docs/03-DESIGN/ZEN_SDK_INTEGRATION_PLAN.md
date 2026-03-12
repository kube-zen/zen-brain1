# zen-sdk Integration Plan for zen-brain1

## Current State

**Already Using (✅ 95% reuse):**
- ✅ retry (pkg/retry) - LLM gateway and adapters
- ✅ scheduler (pkg/scheduler) - QMD orchestrator
- ✅ receiptlog (pkg/receiptlog) - Journal/receiptlog
- ✅ health (pkg/health) - API server health checks
- ✅ dedup (pkg/dedup) - Redis message bus deduplication
- ✅ store (pkg/store) - Session store

**Missing Components (❌ 0% reuse):**
- ❌ dlq (pkg/dlq) - Dead Letter Queue for failed events
- ❌ observability (pkg/observability) - OpenTelemetry tracing
- ❌ leader (pkg/leader) - Kubernetes leader election
- ❌ logging (pkg/logging) - Unified structured logging
- ❌ events (pkg/events) - Kubernetes event recording
- ❌ crypto (pkg/crypto) - Age-based encryption for secrets

## Integration Plan

### 1. Observability (OpenTelemetry Tracing)
**Priority: HIGH** - Critical for production debugging

**Components to modify:**
- `cmd/controller/main.go` - Initialize OTEL for controller
- `cmd/apiserver/main.go` - Initialize OTEL for API server
- `internal/apiserver/server.go` - Add HTTP tracing middleware

**Implementation:**
```go
// In main.go files
import "github.com/kube-zen/zen-sdk/pkg/observability"

func main() {
    ctx := context.Background()

    // Initialize OTEL with defaults
    shutdown, err := observability.InitWithDefaults(ctx, "zen-brain-controller")
    if err != nil {
        log.Fatalf("Failed to initialize observability: %v", err)
    }
    defer shutdown(ctx)

    // ... rest of application
}
```

**Environment Variables:**
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTLP collector endpoint
- `OTEL_SERVICE_NAME` - Service name (default: zen-brain-{component})
- `OTEL_SAMPLING_RATE` - Sampling rate (default: 1.0 for dev, 0.1 for prod)

---

### 2. Leader Election
**Priority: HIGH** - Critical for multi-replica deployments

**Components to modify:**
- `cmd/controller/main.go` - Add leader election to controller manager

**Implementation:**
```go
import (
    "github.com/kube-zen/zen-sdk/pkg/leader"
    "github.com/kube-zen/zen-brain1/internal/config"
)

func main() {
    ctx := context.Background()

    // Get namespace from config or environment
    namespace := os.Getenv("POD_NAMESPACE")
    if namespace == "" {
        namespace = "zen-system"
    }

    // Prepare leader election options
    opts, err := leader.PrepareManagerOptions(ctx, "zen-brain-controller", namespace)
    if err != nil {
        log.Fatalf("Failed to prepare leader election: %v", err)
    }

    // Create manager with leader election
    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
    // ... rest of setup
}
```

**Environment Variables:**
- `LEADER_ELECTION` - Enable/disable (default: true)
- `LEADER_ELECTION_RESOURCE_NAME` - Lease resource name (default: zen-brain-controller-leader)
- `LEADER_ELECTION_NAMESPACE` - Lease namespace

---

### 3. Unified Logging
**Priority: HIGH** - Improves observability and debugging

**Components to modify:**
- Replace ad-hoc logging with zen-sdk/pkg/logging
- Add context-aware logging throughout
- Integrate with observability for trace/span IDs

**Implementation:**
```go
import (
    "github.com/kube-zen/zen-sdk/pkg/logging"
)

// Create logger at package level
var logger = logging.NewLogger("zen-brain.controller")

func (r *ZenProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Log with context (automatically extracts trace_id, request_id, etc.)
    logger.WithContext(ctx).Info("Reconciling ZenProject",
        logging.Operation("reconcile"),
        logging.String("name", req.Name),
        logging.String("namespace", req.Namespace),
    )

    // ... reconcile logic
}
```

**Key Features:**
- Automatic trace/span ID extraction from context
- Structured JSON logs in production
- Pretty console logs in development
- Context helpers: WithRequestID, WithTenantID, WithUserID, etc.

---

### 4. Kubernetes Events
**Priority: MEDIUM** - Improves observability in Kubernetes

**Components to modify:**
- `internal/zencontroller/` - Add event recording to reconcilers
- `cmd/controller/main.go` - Initialize event recorder

**Implementation:**
```go
import (
    "github.com/kube-zen/zen-sdk/pkg/events"
)

// In reconciler setup
recorder := events.NewRecorder(mgr.GetClient(), "zen-brain-controller")

// In reconcile logic
recorder.Eventf(
    project,
    corev1.EventTypeNormal,
    "ReconciliationSucceeded",
    "Successfully reconciled ZenProject %s",
    req.Name,
)
```

---

### 5. Crypto (Age Encryption)
**Priority: MEDIUM** - Security enhancement for secrets

**Components to modify:**
- `internal/config/` - Encrypt sensitive config values
- `internal/session/` - Encrypt session tokens
- Any component handling secrets/credentials

**Implementation:**
```go
import (
    "github.com/kube-zen/zen-sdk/pkg/crypto"
)

// Create encryptor
encryptor := crypto.NewAgeEncryptor()

// Encrypt secrets
recipients := []string{os.Getenv("AGE_PUBLIC_KEY")}
ciphertext, err := encryptor.Encrypt(plaintext, recipients)

// Decrypt secrets
identity := os.Getenv("AGE_PRIVATE_KEY")
plaintext, err := encryptor.Decrypt(ciphertext, identity)
```

**Key Management:**
- Generate age keys: `age-keygen -o age-key.txt`
- Store private key securely (environment variable, secret management)
- Rotate keys periodically (supports multiple recipients)

---

### 6. Dead Letter Queue (DLQ)
**Priority: LOW** - Error handling enhancement

**Components to modify:**
- `internal/messagebus/redis/` - Add DLQ for failed Redis messages
- `internal/qmd/` - Add DLQ for failed task dispatch

**Implementation:**
```go
import (
    "github.com/kube-zen/zen-sdk/pkg/dlq"
    "github.com/kube-zen/zen-sdk/pkg/logging"
)

// Create DLQ manager
logger := logging.NewLogger("zen-brain.messagebus")
dlqManager := dlq.NewManager(logger, 10000, dlq.DefaultRetryConfig())

// When message fails to process
err := dlqManager.AddFailedEvent(
    ctx,
    "messagebus-redis",
    dlq.Event{
        Source:    "redis",
        Timestamp: time.Now(),
        RawData:   messageData,
    },
    "task-queue",
    fmt.Errorf("failed to dispatch task"),
    "transient",
    "network",
)

// Periodic replay of DLQ events
events := dlqManager.ListFailedEvents(&dlq.Filter{
    ErrorType: "transient",
    MinRetryCount: 1,
})
for _, event := range events {
    _, exists := dlqManager.ReplayFailedEvent(event.ID)
}
```

**HTTP API Endpoints:**
- `GET /api/v1/dlq/events` - List failed events
- `POST /api/v1/dlq/events/:id/replay` - Replay event
- `DELETE /api/v1/dlq/events/:id` - Delete event
- `GET /api/v1/dlq/stats` - DLQ statistics

---

## Implementation Order

1. **Phase 1 (Critical):**
   - Observability (tracing)
   - Leader election
   - Unified logging

2. **Phase 2 (Important):**
   - Kubernetes events
   - Crypto (secrets)

3. **Phase 3 (Enhancement):**
   - Dead Letter Queue (DLQ)

---

## Testing Strategy

### Unit Tests
- Test OTEL initialization
- Test leader election logic
- Test logging with context
- Test event recording
- Test encryption/decryption
- Test DLQ operations

### Integration Tests
- Test controller with leader election enabled
- Test API server with tracing middleware
- Test end-to-end tracing spans
- Test DLQ replay flow

---

## Migration Notes

### Breaking Changes
- None expected (all additive)

### Configuration Changes
- Add environment variables for OTEL, leader election, and age keys
- Update deployment manifests

### Monitoring Changes
- New OTEL spans will appear in tracing system
- New Kubernetes events will appear in `kubectl get events`
- DLQ metrics will be available via HTTP API

---

## Success Criteria

- ✅ All zen-sdk components (dlq, observability, leader, logging, events, crypto) integrated
- ✅ Controller runs with leader election in multi-replica mode
- ✅ API server has OTEL tracing on all HTTP endpoints
- ✅ All logs use structured logging with context
- ✅ Kubernetes events recorded for controller operations
- ✅ Secrets encrypted with age where appropriate
- ✅ DLQ captures and retries failed messages
- ✅ No regressions in existing functionality
- ✅ Code coverage >80% for new integrations

---

## References

- [zen-sdk README](../../../zen-sdk/README.md)
- [zen-sdk/pkg/observability](../../../zen-sdk/pkg/observability/README.md)
- [zen-sdk/pkg/leader](../../../zen-sdk/pkg/leader/README.md)
- [zen-sdk/pkg/logging](../../../zen-sdk/pkg/logging/README.md)
- [zen-sdk/pkg/events](../../../zen-sdk/pkg/events/README.md)
- [zen-sdk/pkg/crypto](../../../zen-sdk/pkg/crypto/README.md)
- [zen-sdk/pkg/dlq](../../../zen-sdk/pkg/dlq/README.md)
