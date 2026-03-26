# Current State - zen-brain1

**Last Updated**: 2026-03-22
**Status**: Active Development

## Project Overview

Zen-Brain is an AI-native orchestration platform coordinating work execution across heterogeneous LLM providers with SR&ED-ready evidence collection.

## Current Block Status

### Block 3: Runtime Bootstrap ✅ COMPLETE

- **Status**: Production-ready
- **Components**: ZenContext, QMD, Ledger, MessageBus
- **Runtime Profile**: Strict mode in prod/staging
- **Preflight**: Enhanced preflight with dependency validation

### Block 4: Foreman & Workers ✅ OPERATIONAL

- **Status**: Running with 2-5 workers
- **Components**: BrainTask reconciler, Factory task runner
- **LLM Integration**: Local CPU inference (qwen3.5:0.8b)
- **Git Integration**: Git worktree support for isolated workspaces

### Block 5: Office Connectors 🔧 ACTIVE

- **Jira Integration**: ZB-025 24/7 operations
- **Canonical Project Key**: ZB (source: `deploy/zen-lock/jira-metadata.yaml`)
- **Credentials**: ZenLock-encrypted, preflight-verified
- **Operations**: start-dogfood, stop-dogfood, status, recover

## Active Initiatives

### ZB-025: Jira Intake & 24/7 Operations

**Goal**: Unattended 24/7 processing of Jira issues

**Status**:
- ✅ Jira connector operational
- ✅ Credential rails established
- ✅ Preflight project key verification
- ⏳ Dogfood ingestion loop (pending)
- ⏳ 5-worker pilot run (pending)

**Next Steps**:
1. Implement `start-dogfood` ingestion logic
2. Create BrainTask from Jira issues
3. Launch overnight pilot run

### ZB-CREDENTIAL-RAILS: Project Key Canonicalization ✅ COMPLETE

**Status**: Complete

**Achievements**:
- ✅ Canonical project key in `jira-metadata.yaml`
- ✅ Preflight verification of project accessibility
- ✅ Startup logging of project key
- ✅ Documentation in `docs/06-OPERATIONS/JIRA_PROJECT_KEY_RAILS.md`

**Canonical Project Key**: ZB

**Legacy Keys**: SCRUM (deprecated, do not use)

## Configuration

### Canonical Config Paths

| Config Type | Path | Source of Truth |
|------------|------|-----------------|
| Runtime Config | `$ZEN_BRAIN_HOME/config.yaml` | File (versioned) |
| Jira Metadata | `deploy/zen-lock/jira-metadata.yaml` | Git (source-controlled) |
| Jira Credentials | `/zen-lock/secrets/` | ZenLock (encrypted) |
| Local Overrides | `~/.zen-brain/secrets/` | Local only (dev mode) |

### Environment Variables

```bash
# Jira Configuration
JIRA_URL=https://zen-mesh.atlassian.net
JIRA_EMAIL=zen@kube-zen.io
JIRA_PROJECT_KEY=ZB  # Secondary to jira-metadata.yaml

# Runtime Profile
ZEN_RUNTIME_PROFILE=prod|staging|dev|test

# Preflight
ZEN_BRAIN_PREFLIGHT_STRICT=true
ZEN_BRAIN_PREFLIGHT_TIMEOUT=10s
```

## Known Issues & Mitigations

### None Currently

All known issues from previous sessions have been resolved.

## Architecture Decisions

### ADR-001: ZenLock for Secrets ✅

**Decision**: Use ZenLock (age-encrypted secrets) for all credentials in cluster mode.

**Rationale**:
- No plaintext secrets in cluster
- Git-ops friendly
- Preflight verification
- Fail-closed behavior

**Status**: Implemented and operational

### ADR-002: Canonical Project Key ✅

**Decision**: Single source-controlled project key in `jira-metadata.yaml`.

**Rationale**:
- Prevents key drift
- Enables preflight verification
- Clear audit trail

**Status**: Implemented, preflight enforced

### ADR-003: Local CPU Inference ✅

**Decision**: Use local LLM (qwen3.5:0.8b) for Factory execution.

**Rationale**:
- Cost-effective
- Low latency
- No external API dependencies

**Status**: Operational, tuned for 45m timeouts

## Operations Runbooks

### Starting Foreman

```bash
# 1. Verify config
cat deploy/zen-lock/jira-metadata.yaml

# 2. Verify credentials
ls -la /zen-lock/secrets/

# 3. Start Foreman
./bin/foreman --workers=5 --cluster-id=production

# 4. Check startup logs
# Look for:
#   [Startup] Jira project key: ZB
#   [Startup] Jira Credentials Source: zenlock-dir
```

### Checking Jira Connectivity

```bash
# Doctor check
./bin/zen-brain office doctor

# Smoke test
./bin/zen-brain office smoke-real
```

### Monitoring Health

```bash
# Readiness check
curl http://foreman:8081/readyz

# Health check
curl http://foreman:8081/healthz

# Metrics
curl http://foreman:8080/metrics
```

## Test Coverage

- Unit Tests: ✅ Comprehensive
- Integration Tests: ✅ Office connector integration
- E2E Tests: ⏳ Pending (ZB-025)
- Preflight Tests: ✅ Enhanced preflight coverage

## Deployment

### Current Deployment

- **Cluster**: k3s (local)
- **Namespace**: zen-brain
- **Replicas**: 1 (Foreman), 5 workers
- **Profile**: dev (local), staging (CI), prod (future)

### Deployment Checklist

- [ ] Verify `jira-metadata.yaml` project key
- [ ] Verify ZenLock secrets mounted
- [ ] Check preflight logs
- [ ] Run `zen-brain office doctor`
- [ ] Run `zen-brain office smoke-real`

## Recent Changes

### 2026-03-22: ZB-CREDENTIAL-RAILS Complete

- Added `preflight_jira_project.go` for project key verification
- Integrated check into enhanced preflight
- Added startup logging for project key
- Created `JIRA_PROJECT_KEY_RAILS.md` documentation
- Canonicalized ZB as project key

### 2026-03-21: ZB-026 Complete

- Foreman operational with 5 workers
- Local LLM integration complete
- Git worktree support added

## Next Priorities

1. **ZB-025**: Implement dogfood ingestion loop
2. **Testing**: Add E2E tests for Jira intake
3. **Documentation**: Update operations runbooks
4. **Monitoring**: Add project key accessibility metrics

## References

- Architecture: `docs/01-ARCHITECTURE/`
- Operations: `docs/06-OPERATIONS/`
- Credential Rails: `docs/06-OPERATIONS/JIRA_PROJECT_KEY_RAILS.md`
- Jira Contract: `docs/06-OPERATIONS/ZB_025_JIRA_INTAKE_CONTRACT.md`
