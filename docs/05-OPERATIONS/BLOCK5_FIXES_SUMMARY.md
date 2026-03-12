# Block 5 Office Bootstrap - Fixes Applied

## Issues Fixed

### 1. Real-Path Discipline for Shared Entry Paths ✅
**File:** `internal/context/factory.go`

Fixed `DefaultZenContextConfig()` to use real-path discipline:
- Added `filepath.Join(os.Getenv("HOME"), ".zen", "zen-brain1")` for home dir
- Default paths: `~/.zen/zen-brain1/zen-docs`, `~/.zen/zen-brain1/journal`
- All paths resolved via `filepath.Abs()` in `NewZenContext()`

### 2. Removed Dev-Mode Mock Fallback ✅
**File:** `cmd/zen-brain/main.go`

Fixed `getZenContext()` function:
- Removed dev-mode fallback that returned `newMockZenContext()`
- Now FAILS CLOSED: returns `nil` when strict mode init fails
- Caller must explicitly handle nil ZenContext
- Use `--mock` flag for testing

### 3. Removed Hardcoded Local Redis/S3 ✅
**File:** `cmd/zen-brain/main.go`

Fixed `createRealZenContext()` function:
- Reads config from environment variables:
  - `REDIS_URL`, `REDIS_PASSWORD` for Redis
  - `S3_ENDPOINT`, `S3_BUCKET`, `S3_REGION` for S3
  - `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_SESSION_TOKEN` for S3 auth
- No more hardcoded `localhost:6379` or `minioadmin` credentials
- Uses real-path discipline: `~/.zen/zen-brain1` for zen-docs/journal paths

### 4. Office Bootstrap Strict Mode Enforcement ✅ FIXED
**File:** `internal/integration/office.go`

**Applied fixes:**

#### KB Section (lines 80-136):
- Added explicit `kb.enabled` flag check before using configured KB
- Simplified error handling: always FAILS CLOSED (no complex strict mode checks)
- Added `kb.required` check when KB is not configured

```go
if cfg != nil && cfg.KB.DocsRepo != "" && cfg.QMD.BinaryPath != "" {
    // FAIL CLOSED: KB requires explicit enabled flag
    if !cfg.KB.Enabled {
        return nil, fmt.Errorf("KB configured but not enabled (set kb.enabled=true)")
    }
    // Use real qmd-backed KB
    // ... creates KB ...
} else {
    // KB not configured
    if cfg != nil && cfg.KB.Required {
        // FAIL CLOSED: KB required but not configured
        return nil, fmt.Errorf("KB required but not configured (set kb.docs_repo and qmd.binary_path)")
    }
    // Use stub KB (only when not required)
    kbStore = kbinternal.NewStubStore()
}
```

#### Ledger Section (lines 165-195):
- Simplified error handling: always FAILS CLOSED
- Removed complex `strictMode` conditional logic
- Added `ledger.required` check when ledger is not enabled

#### Message Bus Section (lines 200-241):
- Removed `redis://localhost:6379` default URL → FAILS CLOSED if redis_url is empty
- Simplified error handling: always FAILS CLOSED
- Added `message_bus.required` check when message bus is not enabled

```go
if cfg != nil && cfg.MessageBus.Enabled {
    redisURL := cfg.MessageBus.RedisURL
    if redisURL == "" {
        // FAIL CLOSED: no default Redis URL
        return nil, fmt.Errorf("Message Bus enabled but redis_url not configured (set message_bus.redis_url)")
    }
    // ... creates Redis client ...
} else {
    // Message bus not enabled
    if cfg != nil && cfg.MessageBus.Required {
        // FAIL CLOSED: message bus required but not enabled
        return nil, fmt.Errorf("Message Bus required but not enabled (set message_bus.enabled=true)")
    }
    log.Println("(Message bus disabled)")
}
```

**Key Behavior Changes:**

| Scenario | Before | After |
|----------|--------|-------|
| KB configured but not enabled | Uses real KB | FAILS CLOSED |
| KB init fails (not strict) | Logs error, continues | FAILS CLOSED |
| KB required but not configured | Logs error, uses stub | FAILS CLOSED |
| Message Bus enabled but no redis_url | Uses localhost:6379 | FAILS CLOSED |
| Ledger init fails (not strict, not required) | Logs error, uses stub | FAILS CLOSED |

### 5. Block 4 Template TODOs ⚠️ REMAINING
**File:** `internal/factory/repo_aware_templates.go`

TODO placeholders that need implementation:
1. `registerRepoAwareDocsTemplate()` - docs template has TODOs in Getting Started, Usage, Configuration sections
2. `registerRepoAwareTestTemplate()` - test template has TODO in Production-Quality Tests section
3. `registerRepoAwareCICDTemplate()` - CI/CD template has TODO in Deployment Strategy, Environment Variables sections
4. `registerRepoAwareMonitoringTemplate()` - monitoring template has TODO throughout
5. These make Block 4 "much more real" but not yet "high-quality autonomous execution" across all template lanes

## Summary

- ✅ Real-path discipline applied to all entry paths
- ✅ Removed dev-mode mock fallback (getZenContext)
- ✅ Removed hardcoded local Redis/S3 (createRealZenContext)
- ✅ Office bootstrap strict mode now FAILS CLOSED for KB/Ledger/MessageBus
- ⚠️ Block 4 templates have 5 TODO placeholders (need manual implementation)

## Validation

Created `/tmp/office_bootstrap_validate.go` with 9 test cases covering:
- KB configured + enabled ✓
- KB configured + NOT enabled → FAILS CLOSED ✓
- KB not configured + NOT required → uses stub ✓
- KB required but not configured → FAILS CLOSED ✓

All validation tests pass.
