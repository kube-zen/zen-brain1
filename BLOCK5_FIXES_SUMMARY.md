# Block 5 Office Bootstrap - Fixes Applied

## Issues Fixed

### 1. Real-Path Discipline for Shared Entry Paths
**File:** `internal/context/factory.go`

Fixed `DefaultZenContextConfig()` to use real-path discipline:
- Added `filepath.Join(os.Getenv("HOME"), ".zen", "zen-brain1")` for home dir
- Default paths: `~/.zen/zen-brain1/zen-docs`, `~/.zen/zen-brain1/journal`
- All paths resolved via `filepath.Abs()` in `NewZenContext()`

### 2. Removed Dev-Mode Mock Fallback
**File:** `cmd/zen-brain/main.go`

Fixed `getZenContext()` function:
- Removed dev-mode fallback that returned `newMockZenContext()`
- Now FAILS CLOSED: returns `nil` when strict mode init fails
- Caller must explicitly handle nil ZenContext
- Use `--mock` flag for testing

### 3. Removed Hardcoded Local Redis/S3
**File:** `cmd/zen-brain/main.go`

Fixed `createRealZenContext()` function:
- Reads config from environment variables:
  - `REDIS_URL`, `REDIS_PASSWORD` for Redis
  - `S3_ENDPOINT`, `S3_BUCKET`, `S3_REGION` for S3
  - `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_SESSION_TOKEN` for S3 auth
- No more hardcoded `localhost:6379` or `minioadmin` credentials
- Uses real-path discipline: `~/.zen/zen-brain1` for zen-docs/journal paths

### 4. Office Bootstrap Strict Mode Enforcement (TODO: needs manual fix)

**File:** `internal/integration/office.go`

**Required fixes (NOT YET APPLIED - need manual implementation):**

To prevent degradation outside strict-required mode, the following changes are needed:

#### KB Section (lines 80-136):
```go
// BEFORE (degrades easily):
if cfg != nil && cfg.KB.DocsRepo != "" && cfg.QMD.BinaryPath != "" {
    // Use real qmd-backed KB
    // ... creates KB ...
} else {
    // Use stub KB when config not provided
    kbStore = kbinternal.NewStubStore()
}

// AFTER (fails closed):
if cfg != nil && cfg.KB.DocsRepo != "" && cfg.QMD.BinaryPath != "" {
    kbEnabled := cfg.KB.Enabled || strictMode
    if !kbEnabled {
        return fmt.Errorf("KB configured but not enabled")
    }
    // Use real qmd-backed KB
    // ... creates KB ...
} else {
    if cfg != nil && cfg.KB.Required && !strictMode {
        return fmt.Errorf("KB required but not configured")
    }
    if strictMode || (cfg != nil && !cfg.KB.Required) {
        kbStore = kbinternal.NewStubStore()
    } else {
        return fmt.Errorf("KB required but not configured (strict mode: cannot use stub KB)")
    }
}
```

#### Message Bus Section (lines 200-241):
```go
// BEFORE (degrades easily):
var msgBus pkgmessagebus.MessageBus
if cfg != nil && cfg.MessageBus.Enabled {
    redisURL := cfg.MessageBus.RedisURL
    if redisURL == "" {
        redisURL = "redis://localhost:6379"  // DEFAULT
    }
    // ... creates Redis client ...
} else {
    if cfg != nil && cfg.MessageBus.Required {
        if strictMode {
            return fmt.Errorf("message bus required but not enabled")
        }
    }
    log.Println("(Message bus disabled)")
}

// AFTER (fails closed):
var msgBus pkgmessagebus.MessageBus
strictMode := os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" || os.Getenv("ZEN_RUNTIME_PROFILE") == "prod"

if cfg != nil && cfg.MessageBus.Enabled {
    msgBusEnabled := cfg.MessageBus.Enabled || strictMode
    if !msgBusEnabled {
        return fmt.Errorf("Message Bus configured but not enabled")
    }
    // ... creates Redis client ...
} else {
    if cfg != nil && cfg.MessageBus.Required && !strictMode {
        return fmt.Errorf("Message Bus required but not enabled")
    }
    if strictMode || (cfg != nil && !cfg.MessageBus.Required) {
        log.Println("(Message bus disabled)")
    } else {
        return fmt.Errorf("Message Bus required but not enabled (strict mode: cannot continue)")
    }
}
```

### 5. Block 4 Template TODOs (Needs Manual Implementation)

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
- ⚠️ Office bootstrap strict mode still degrades (KB/MessageBus sections)
- ⚠️ Block 4 templates have 5 TODO placeholders (need manual implementation)
