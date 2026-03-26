# CURRENT_STATE.md

**Last Updated**: 2026-03-26 19:30 EDT

## CANONICAL JIRA IDENTITY — DO NOT ASK AGAIN

| Field | Value |
|-------|-------|
| **Jira Email** | `zen@kube-zen.io` |
| **Jira URL** | `https://zen-mesh.atlassian.net` |
| **Project Key** | `ZB` |
| **Forbidden Email** | `zen@zen-mesh.io` (causes 401) |
| **Runtime Source** | `/zen-lock/secrets/` (ZenLock injection) |
| **Bootstrap Script** | `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh` |
| **Preflight Command** | `JIRA_TOKEN=<t> MODE=preflight STRICT=true ./cmd/admission-gate/admission-gate` |

See AGENTS.md for full canonical section with proof commands.

## Current Proven State

### ✅ Auth & Configuration
- Canonical Jira email: `zen@kube-zen.io` (verified from live Foreman pod)
- Canonical Jira project key: `ZB` (verified from live Foreman pod)
- Jira token: Valid (verified from live Foreman pod)
- Auth check: PASS (`GET /rest/api/3/myself` = 200)
- Project check: PASS (`GET /rest/api/3/project/ZB` = 200)

### ✅ Live Runtime
- Image: `zen-registry:5000/zen-brain:dev-17743d4`
- ImageID: `sha256:e2596cad0319305707d559ab98ae24804dbd7726d9e4e18b4c8c5a4b81bf9f12`
- Office doctor: Shows split auth/project checks
- Config source: ConfigMap `foreman-jira-config` (email: zen@kube-zen.io)
- Secret source: Secret `zen-lock-jira-credentials` (token MD5: 801cbc1f...)
- Credentials source: zenlock-dir:/zen-lock/secrets

### ✅ Real Jira Issues Created
- **ZB-256**: ZB-027H/I Pilot Success 1 - Update README (labels: zen-brain-dogfood, zen-brain-nightshift)
- **ZB-257**: ZB-027H/I Pilot Success 2 - Add inline comment (labels: zen-brain-dogfood, zen-brain-nightshift)
- **ZB-258**: ZB-027H/I Pilot Failure Test - Intentional timeout (labels: zen-brain-dogfood, zen-brain-nightshift)

### ⏳ In Progress
- Ingest Jira issues into BrainTasks
- Prove success path (ZB-256, ZB-257)
- Prove controlled failure path (ZB-258)
- Write feedback back to Jira issues
- Launch overnight 5-worker pilot

## Next Exact Action

**Ingest ZB-256, ZB-257, ZB-258 into BrainTasks**

```bash
# Query Jira for dogfood issues
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office search 'project = ZB AND labels in ("zen-brain-dogfood") ORDER BY created DESC'

# Ingest into BrainTasks (need to implement/verify ingestion path)
# Expected: Issues ZB-256, ZB-257, ZB-258 become BrainTasks
# Expected: Deduplication on re-run (no duplicate active BrainTasks)
```

## Configuration Alignment

All source-controlled metadata aligned:

| Component | File/Resource | Value | Status |
|-----------|---------------|-------|--------|
| Jira Email | `deploy/zen-lock/jira-metadata.yaml` | `zen@kube-zen.io` | ✅ |
| Jira Email | ConfigMap `foreman-jira-config` | `zen@kube-zen.io` | ✅ |
| Jira Email | Secret `zen-lock-jira-credentials` | `zen@kube-zen.io` | ✅ |
| Jira Project | `deploy/zen-lock/jira-metadata.yaml` | `ZB` | ✅ |
| Jira Project | ConfigMap `foreman-jira-config` | `ZB` | ✅ |
| Jira Project | Secret `zen-lock-jira-credentials` | `ZB` | ✅ |
| Jira Token | Secret `zen-lock-jira-credentials` | (192 chars, MD5: 801cbc1f...) | ✅ |

## Live Loop Requirements

### Success Path (ZB-256, ZB-257)
- [ ] Issues ingested into BrainTasks
- [ ] BrainTask claimed by Foreman
- [ ] source=llm
- [ ] model=qwen3.5:0.8b
- [ ] normal lane uses 45m profile
- [ ] terminal Completed state
- [ ] feedback written back to Jira issue

### Failure Path (ZB-258)
- [ ] Issue ingested into BrainTask
- [ ] Controlled failure (intentional timeout)
- [ ] terminal Failed or Blocked state
- [ ] retry/escalation bounded
- [ ] feedback written back to Jira issue

### Pilot Requirements
- [ ] 5-worker pilot launched
- [ ] Unattended operation
- [ ] Safe bounded tasks only
- [ ] No manual Jira UI in critical path

## Related Documentation

- `AGENTS.md` - Canonical rules
- `docs/06-OPERATIONS/JIRA_PROJECT_KEY_RAILS.md` - Project key rails
- `docs/credential-rails.md` - Credential rails

## Commit History

- **a73bee3**: ZB-027J: Fix Jira email/config mismatch, create real pilot issues
- **17743d4**: ZB-CREDENTIAL-RAILS: Canonicalize Jira project key with preflight verification

## Evidence

All claims verified from live Foreman pod context:
```bash
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office doctor
# Output:
#   Auth check: PASS
#   Project check: PASS (project ZB accessible)
```

Token validity verified:
```bash
# Test from live pod with correct email
kubectl exec -n zen-brain deployment/foreman -- /tmp/test-auth
# Output:
#   zen@kube-zen.io: PASS (HTTP 200)
#   zen@zen-mesh.io: FAIL (HTTP 401)
```
