# Jira Project Key Rails - ZB-CREDENTIAL-RAILS

## Overview

This document defines the canonical rules for Jira project key configuration and verification in zen-brain1.

## Canonical Project Key Source

**Rule**: The Jira project key comes from source-controlled metadata and must be validated by preflight.

### Source Location

- **File**: `deploy/zen-lock/jira-metadata.yaml`
- **Field**: `jira.project_key`

### Current Canonical Key

```yaml
jira:
  url: "https://zen-mesh.atlassian.net"
  email: "zen@kube-zen.io"
  project_key: "ZB"
```

**Canonical Project Key: ZB**

## Project Key Rules

### 1. Single Source of Truth

- The project key in `jira-metadata.yaml` is the ONLY source of truth
- Environment variable `JIRA_PROJECT_KEY` is secondary
- Legacy references to other project keys (e.g., SCRUM) are stale and MUST NOT be used

### 2. Runtime Verification

Before any Jira operation (create, ingest, feedback):

1. **Preflight Check**: Verify project key is accessible
   - Call `GET /rest/api/3/project/search` to verify account has access
   - Call `GET /rest/api/3/project/{key}` to verify direct access
   - Fail closed if project key not accessible

2. **Startup Logging**: Print configured project key at startup
   ```
   [Startup] Jira project key: ZB
   [Startup] Jira Credentials Source: zenlock-dir
   ```

### 3. Failure Mode

If the configured project key is not accessible to the runtime account:

**STOP with exact message**:
```
Configured Jira project key <X> is not accessible to runtime account <email>
```

Do NOT:
- Fall back to other project keys
- Guess or infer project keys
- Continue with degraded functionality

## Implementation

### Preflight Check

File: `internal/runtime/preflight_jira_project.go`

Function: `PreflightJiraProjectKey(ctx, cfg)`

Verifies:
- Project key is configured
- Credentials are available
- Project is visible in `/project/search`
- Project is directly accessible via `/project/{key}`

### Startup Logging

File: `cmd/foreman/main.go`

Logs at startup:
- Jira enabled/disabled
- Configured project key (or warning if missing)
- Credentials source
- Base URL
- Email

### Enhanced Preflight Integration

File: `internal/runtime/preflight_enhanced.go`

The Jira project key check is integrated into the enhanced preflight system and runs automatically during:
- Foreman startup
- Strict runtime bootstrap
- Readiness checks

## CI Guardrails

### Drift Detection

CI MUST fail if:
- Multiple project keys appear in active code/config/docs
- Legacy project keys (e.g., SCRUM) appear in non-example/non-legacy paths
- Source metadata project key differs from hardcoded defaults

### Example: CI Check

```bash
# Check for project key drift
python3 scripts/ci/check_jira_project_key_drift.py
```

## Legacy Reference Handling

### SCRUM Project Key

The SCRUM project key appears in:
- Old repository history
- Example files
- Local development artifacts

**Rule**: SCRUM references MUST be marked as legacy/example or removed from active paths.

Active paths:
- `cmd/` (except test files)
- `internal/` (except test files)
- `configs/`
- `deploy/`
- Active documentation in `docs/`

Non-active paths (examples/legacy allowed):
- `examples/`
- `*_test.go` files
- Docs explicitly marked as historical/archived

## Verification Checklist

### For Operators

Before starting Foreman:

- [ ] Verify `deploy/zen-lock/jira-metadata.yaml` exists
- [ ] Verify `project_key: "ZB"` is set
- [ ] Verify credentials in ZenLock (`/zen-lock/secrets/`)
- [ ] Check startup logs show correct project key
- [ ] Run `zen-brain office doctor` to verify connectivity

### For Developers

When modifying Jira-related code:

- [ ] Never hardcode project keys
- [ ] Always read from `config.Jira.ProjectKey`
- [ ] Run preflight checks in tests
- [ ] Update this document if source location changes

## Related Documentation

- `docs/06-OPERATIONS/ZB_025_JIRA_INTAKE_CONTRACT.md` - Jira intake contract
- `deploy/zen-lock/jira-metadata.yaml` - Canonical metadata
- `internal/runtime/preflight_jira_project.go` - Preflight implementation

## Changelog

- **2026-03-22**: Initial creation - canonicalized ZB as project key, added preflight verification
