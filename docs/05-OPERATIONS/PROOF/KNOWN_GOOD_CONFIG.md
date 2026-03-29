# Known-Good Runtime Profile - Zen-Brain 1.0

> **NOTE:** This documents a proven configuration including the Ollama L0 fallback lane. The **primary** runtime is **llama.cpp** (L1/L2).

**Date:** 2026-03-13
**Commit:** be3d531
**Status:** ✅ FROZEN - DO NOT MODIFY WITHOUT EVIDENCE

## Overview

This profile represents the proven, working operating mode for Zen-Brain 1.0 on current snapshot. All values have been tested and confirmed working end-to-end.

**Change policy:** Do not modify this profile without:
1. Explicit reason documented in git commit
2. Full regression testing (build + local run + deploy)
3. Update to this document with new proven values

---

## Toolchain

### Go
```bash
Version: go1.25.0
Platform: linux/amd64
```

### Prerequisites
```bash
docker: Running
k3d: Installed and working
helm: Installed
helmfile: Installed
kubectl: Installed
```

---

## Local Development Mode

### Environment Variables
```bash
export ZEN_RUNTIME_PROFILE=dev
export OLLAMA_BASE_URL=http://127.0.0.1:11434
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
```

### Host Docker Ollama
```bash
# Start Ollama container
docker run -d --name ollama \
  --network host \
  -p 11434:11434 \
  ollama/ollama

# Verify running
docker ps | grep ollama
curl http://127.0.0.1:11434/api/version

# Expected output
{"version":"0.17.6"}
```

### Build Commands
```bash
cd /home/neves/zen/zen-brain1
make deps
make build-all
```

### Runtime Verification Commands
```bash
# All should succeed
./bin/zen-brain runtime doctor
./bin/zen-brain runtime report
./bin/zen-brain runtime ping
./bin/zen-brain office doctor
```

### End-to-End Test Commands
```bash
# Should complete successfully
timeout 120 ./bin/zen-brain vertical-slice --mock
./bin/zen-brain test
```

### Expected Runtime Doctor Output
```
Profile:          dev
Strict mode:      false
ZenContext:     ✗ mode=degraded   healthy=false
                    └─ failed to create Tier 1 store: Redis URL or Addr not set
Tier1 (Hot):    ✗ mode=degraded   healthy=false
Tier2 (Warm):   ✗ mode=disabled   healthy=false
Tier3 (Cold):   ✗ mode=disabled   healthy=false
Journal:        ✗ mode=disabled   healthy=false
Ledger:         ✗ mode=disabled   healthy=false
MessageBus:     ✗ mode=disabled   healthy=false

STATUS: HEALTHY
```

### Expected Office Doctor Output
```
knowledge_base: ✓ mode=stub enabled=true
                  └─ explicit stub opt-in via ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
ledger:         ✓ mode=stub enabled=true
                  └─ explicit stub opt-in via ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
message_bus:    ✗ mode=disabled enabled=false
                    └─ Message Bus disabled
Cluster mapping: default -> jira
Jira base URL: https://zen-mesh.atlassian.net
Project key: ZB
Webhook: enabled=true, path=/webhook, port=8080
Credentials: present=true
Connector: real (https://zen-mesh.atlassian.net)
API reachability: ok
```

### Jira Configuration (Stored Outside Repo)
```bash
# Location: ~/zen/.zen-brain1-config/jira.yaml
# NOT committed to repository (security)
email: zen@zen-mesh.io
token: ATATT3... (user-level API token)
project_key: ZB

# Token type: MUST be ATATT3... (user-level)
# NOT ATCTT3... (workspace-level - does not work with Basic Auth)
```

---

## Cluster Deployment Mode

### Config Values (config/clusters.yaml)
```yaml
# Ollama deployment model
deploy:
  use_ollama: false              # Host Docker Ollama is default
  ollama:
    models: []                   # Empty = disabled in-cluster
  host_ollama_base_url: "http://host.k3d.internal:11434"
```

### Deploy Commands
```bash
cd /home/neves/zen/zen-brain1
make dev-up
# or
python3 scripts/zen.py env redeploy --env sandbox
```

### Expected Deploy Output
```
UPDATED RELEASES:
  zen-brain-dependencies   zen-context   deployed   0s
  zen-brain-ollama         zen-brain     deployed   0s
  zen-brain-crds           zen-brain     deployed   0s
  zen-brain-core           zen-brain     deployed   1s

Health endpoints verified: /healthz ✓ /readyz ✓

Zen-brain environment ready.
  Cluster: zen-brain-sandbox
  Context: k3d-zen-brain-sandbox
  Apiserver: http://127.0.1.6:8080/healthz
  Readyz:    http://127.0.1.6:8080/readyz
```

### Workload Health Check
```bash
kubectl get pods -n zen-brain --context k3d-zen-brain-sandbox

# Expected output
NAME                         READY   STATUS    RESTARTS   AGE
apiserver-xxx               1/1     Running   0
foreman-xxx                 1/1     Running   0
# NO ollama pods (in-cluster Ollama disabled)
```

### Apiserver Ollama Path Check
```bash
kubectl exec -n zen-brain apiserver-xxx -- env | grep OLLAMA_BASE_URL

# Expected output
OLLAMA_BASE_URL=http://host.k3d.internal:11434
```

### Health Endpoints
```bash
# Both should return 200 OK
curl http://127.0.1.6:8080/healthz
curl http://127.0.1.6:8080/readyz
```

---

## Dependency Status

### Enabled
- **Host Docker Ollama:** v0.17.6 (Running outside Kubernetes)
- **Stub Knowledge Base:** Explicit opt-in via ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
- **Stub Ledger:** Explicit opt-in via ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
- **Jira:** Configured with ATATT3... token (authentication works, JSON parsing issue)

### Known Issues
- **Jira API v3 ADF:** Go struct expects `description` as string, but API returns ADF object
  - Blocks: office fetch, office search, office create, office update
  - Fix: Update Go struct to handle `map[string]interface{}` or proper ADF struct
  - Workaround: Use Jira API v2 (might return description as string)

### Disabled (by design)
- **In-Cluster Ollama:** use_ollama: false
- **Redis:** No TIER1_REDIS_ADDR set
- **S3/MinIO:** No S3 endpoint configured
- **CockroachDB:** No ZEN_LEDGER_DSN set
- **Message Bus:** Disabled in config

---

## Regression Test Checklist

When making changes, run this checklist:

### Build Sanity
- [ ] make deps succeeds
- [ ] make build-all succeeds (all 4 binaries)
- [ ] go test ./internal/factory/... passes
- [ ] go test ./cmd/zen-brain/... passes
- [ ] go test ./pkg/contracts/... passes

### Local Mode
- [ ] runtime doctor shows STATUS: HEALTHY
- [ ] runtime report returns valid JSON
- [ ] runtime ping returns "ok"
- [ ] office doctor shows stub mode enabled
- [ ] vertical-slice --mock completes successfully
- [ ] Ollama logs show http://127.0.0.1:11434 (local)

### Cluster Deploy
- [ ] make dev-up completes without errors
- [ ] apiserver pod is 1/1 Running
- [ ] foreman pod is 1/1 Running
- [ ] /healthz returns 200 OK
- [ ] /readyz returns 200 OK
- [ ] OLLAMA_BASE_URL=http://host.k3d.internal:11434
- [ ] No ollama pods in zen-brain namespace

---

## Change History

### 2026-03-13
- **Commit 137be79:** Fixed CockroachLedger nil pointer crashes
- **Commit be3d531:** Locked down Ollama deployment model (host Docker = default)

### Before (known issues)
- Vertical-slice --mock: panic (nil pointer dereference)
- Ambiguous Ollama deployment path (in-cluster vs host)

---

## Quick Reference

### Full Local Run Sequence
```bash
# 1. Set environment
export ZEN_RUNTIME_PROFILE=dev
export OLLAMA_BASE_URL=http://127.0.0.1:11434
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1

# 2. Verify Ollama
curl http://127.0.0.1:11434/api/version

# 3. Run vertical-slice
timeout 120 ./bin/zen-brain vertical-slice --mock

# 4. Deploy to cluster
make dev-up

# 5. Verify cluster
kubectl get pods -n zen-brain
curl http://127.0.1.6:8080/healthz
```

### Rollback Procedure
If something breaks:
1. Check this document for drift
2. Revert to commits 137be79 or be3d531
3. Run regression test checklist
4. If still broken, the issue is environmental

---

**Status:** ✅ FROZEN - Use as baseline for all changes
