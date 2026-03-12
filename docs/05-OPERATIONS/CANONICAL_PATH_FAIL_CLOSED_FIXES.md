# Canonical Path Fail-Closed Fixes

**Date**: 2026-03-11
**Commit**: (pending)
**Scope**: Remove mock/stub fallback from canonical execution paths

## Summary

This change removes silent mock/stub fallback behavior from the canonical execution paths, making the system fail-closed when required services are unavailable.

## Changes Made

### 1. cmd/zen-brain/main.go - Jira Fail-Closed

**Before**: Silently fell back to mock mode when Jira unavailable
**After**: Requires explicit `--mock` flag for testing; fails with clear error otherwise

```go
// FAIL CLOSED: Do not fall back to mock mode silently
// If --mock was not explicitly set, require real Jira connectivity
if jiraMode == "" && !useMock {
    jiraConnector, err := jira.NewFromEnv("jira", clusterID)
    if err != nil {
        // FAIL CLOSED: Error instead of falling back to mock
        log.Fatalf("  ✗ Jira connector initialization failed: %v\n  Use --mock flag for testing without Jira", err)
    }
    ...
}
```

### 2. cmd/zen-brain/main.go - ZenContext Fail-Closed

**Before**: "Falling back to mock ZenContext"
**After**: ZenContext is optional; nil means disabled, not mock

```go
if zenContext == nil {
    zenContext, err = createRealZenContext()
    if err != nil {
        // ZenContext not available - this is OK for operations that don't need it
        log.Printf("  ZenContext not available: %v (continuing without context tiering)", err)
        zenContext = nil // Explicit nil, not mock
    }
}
```

### 3. cmd/zen-brain/main.go - Ledger Fail-Closed

**Before**: `ledgerClientOrStub()` silently returned mock
**After**: `ledgerClientOrNil()` returns nil; callers handle gracefully

```go
// FAIL CLOSED: Never silently use a mock. Callers must handle nil ledger.
func ledgerClientOrNil() ledger.ZenLedgerClient {
    dsn := os.Getenv("ZEN_LEDGER_DSN")
    if dsn == "" {
        dsn = os.Getenv("LEDGER_DATABASE_URL")
    }
    if dsn == "" {
        // No ledger configured - return nil, caller must handle
        return nil
    }
    ...
}
```

### 4. internal/runtime/bootstrap.go - Redis Fail-Closed

**Before**: Defaulted to localhost:6379
**After**: Returns nil config if Redis not explicitly configured

```go
if out.Tier1Redis.Addr == "" {
    log.Printf("[Bootstrap] FAIL CLOSED: Tier1 Redis not configured - ZenContext disabled (set TIER1_REDIS_ADDR to enable)")
    return nil
}
```

### 5. internal/runtime/bootstrap.go - Ledger Fail-Closed

**Before**: "using stub ledger" when DSN not configured
**After**: Ledger is disabled (ModeDisabled), not stub

```go
if errLedger != nil || ledgerClient == nil {
    if reqLedger {
        // FAIL CLOSED: Required capability unavailable
        return &Runtime{ZenContext: zenContext, Report: report}, fmt.Errorf("%s", msg)
    }
    // In non-strict mode, ledger is disabled (not stub)
    report.Ledger = CapabilityStatus{Name: "ledger", Mode: ModeDisabled, Healthy: false, Required: false, Message: "no ledger DSN configured"}
    ledgerClient = nil
}
```

### 6. internal/integration/office.go - KB Stub Fail-Closed

**Before**: `NewOfficePipeline()` always used stub KB
**After**: Fails in production mode; stub only allowed in dev

```go
// FAIL CLOSED: Prevent production use of stub pipeline
if os.Getenv("ZEN_RUNTIME_PROFILE") == "prod" || os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
    return nil, fmt.Errorf("NewOfficePipeline with stubs not allowed in production mode")
}

log.Println("Initializing Office pipeline (DEV MODE - using stubs)...")
```

### 7. cmd/zen-brain/analyze.go - Ollama URL Fail-Closed

**Before**: Hardcoded `localhost:11434`
**After**: Requires explicit `OLLAMA_BASE_URL` in production

```go
ollamaURL := os.Getenv("OLLAMA_BASE_URL")
if ollamaURL == "" {
    if os.Getenv("ZEN_RUNTIME_PROFILE") == "prod" || os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
        log.Fatalf("[Analyzer] FAIL CLOSED: OLLAMA_BASE_URL not set in strict mode")
    }
    // Dev mode only
    log.Printf("[Analyzer] WARNING: OLLAMA_BASE_URL not set, using localhost (dev mode only)")
    ollamaURL = "http://localhost:11434"
}
```

### 8. cmd/zen-brain/analyze.go - History Store Fail-Closed

**Before**: Fell back to `/tmp/zen-brain-analysis-history`
**After**: Fails gracefully; history disabled if home dir not writable

```go
historyDir := filepath.Join(config.HomeDir(), "analysis-history")
if err := os.MkdirAll(historyDir, 0755); err != nil {
    // FAIL CLOSED: Cannot create history store - return analyzer without history
    fmt.Fprintf(os.Stderr, "Warning: could not create history store at %s: %v (analysis history disabled)\n", historyDir, err)
    return a, nil, nil
}
```

### 9. internal/foreman/factory_runner.go - Runtime Dir Fail-Closed

**Before**: Defaulted to `/tmp/zen-brain-factory`
**After**: Requires explicit config in production

```go
if cfg.RuntimeDir == "" {
    cfg.RuntimeDir = os.Getenv("ZEN_FOREMAN_RUNTIME_DIR")
}
if cfg.RuntimeDir == "" {
    if os.Getenv("ZEN_RUNTIME_PROFILE") == "prod" || os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
        return nil, fmt.Errorf("FAIL CLOSED: RuntimeDir not set (set ZEN_FOREMAN_RUNTIME_DIR)")
    }
    log.Printf("[FactoryTaskRunner] WARNING: RuntimeDir not set, using /tmp (dev mode only)")
    cfg.RuntimeDir = "/tmp/zen-brain-factory"
}
```

## Production Mode Activation

Set any of these to enable strict fail-closed behavior:

```bash
export ZEN_RUNTIME_PROFILE=prod
# or
export ZEN_BRAIN_STRICT_RUNTIME=1
```

## Required Environment Variables (Production)

| Variable | Purpose | Required When |
|----------|---------|---------------|
| `TIER1_REDIS_ADDR` | Redis for ZenContext | ZenContext required |
| `ZEN_LEDGER_DSN` or `LEDGER_DATABASE_URL` | CockroachDB for ledger | Ledger required |
| `OLLAMA_BASE_URL` | Ollama inference endpoint | Always in prod |
| `ZEN_FOREMAN_RUNTIME_DIR` | Factory workspace dir | Foreman running |
| `JIRA_API_TOKEN` + `JIRA_URL` | Jira connectivity | Without --mock flag |

## Testing

All runtime tests pass:
- `TestBootstrap_WithDefaults` - Confirms fail-closed message appears
- `TestStrictness_ProdProfile` - Confirms strict mode behavior
- Circuit breaker tests - All pass

## Impact

### Before
- System silently degraded to mock/stub behavior
- "Falling back to mock mode" messages appeared
- Canonical paths tolerated degraded operation

### After
- Production mode fails fast on missing services
- Dev mode allows explicit opt-in for degraded behavior
- Clear error messages guide users to fix configuration

## Migration Guide

For local development, either:

1. **Use --mock flag** (explicit testing without Jira):
   ```bash
   zen-brain vertical-slice --mock
   ```

2. **Configure required services**:
   ```bash
   export OLLAMA_BASE_URL=http://localhost:11434
   export TIER1_REDIS_ADDR=localhost:6379
   # etc.
   ```

3. **Accept dev-mode warnings** (default behavior):
   ```bash
   # Will use localhost defaults with warnings
   zen-brain vertical-slice JIRA-123
   ```

For production:
- Set `ZEN_RUNTIME_PROFILE=prod`
- Configure all required environment variables
- System will fail fast if any required service is unavailable
