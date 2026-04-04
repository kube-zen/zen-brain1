# Credential Rails - Canonical Runbook

**Version:** 3.0 (Credential Rails Enforcement)
**Date:** 2026-04-02
**Status:** Active - Enforced by CI Gates

---

## Executive Summary

**Single Source of Truth:** All Jira/Git credentials flow through canonical resolvers.

**Cluster Runtime:** `/zen-lock/secrets/*` (ZenLock mount-only, no env vars)

**Local Runtime:** `internal/secrets/jira.go:ResolveJira()` with `DirPath`

**Bootstrap:** `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh`

**CI Enforcement:** 5 gates block violations (see "CI Guardrails" below)

---

## Single Source of Truth

### Code Resolver

| Credential | Resolver | Location |
|------------|----------|----------|
| Jira | `secrets.ResolveJira()` | `internal/secrets/jira.go` |
| Git | `secrets.ResolveGit()` | `internal/secrets/git.go` |

### Credential Storage

| Item | Path | Purpose |
|------|------|---------|
| Age keypair | `~/zen/keys/zen-brain/credentials.key` | Encrypts all local credentials |
| Age public key | `~/zen/keys/zen-brain/credentials.pub` | Recipient for encryption |
| Encrypted bundle | `~/zen/keys/zen-brain/secrets.d/jira.enc` | **Canonical credential store** (age-encrypted JSON) |
| Token input (ephemeral) | `~/zen/DONOTASKMOREFORTHISSHIT.txt` | Bootstrap-only, deleted after rotation |
| Bootstrap script | `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh` | Single command to rotate all credentials |

### Cluster Runtime (Production)

| Field | Mount Path | Source |
|-------|------------|--------|
| JIRA_URL | `/zen-lock/secrets/JIRA_URL` | ZenLock injection |
| JIRA_EMAIL | `/zen-lock/secrets/JIRA_EMAIL` | ZenLock injection |
| JIRA_API_TOKEN | `/zen-lock/secrets/JIRA_API_TOKEN` | ZenLock injection |
| JIRA_PROJECT_KEY | `/zen-lock/secrets/JIRA_PROJECT_KEY` | ZenLock injection |

**Canonical Values:**
- `JIRA_URL`: `https://zen-mesh.atlassian.net`
- `JIRA_EMAIL`: `zen@zen-mesh.io`
- `JIRA_PROJECT_KEY`: `ZB`

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Bootstrap Phase (Local)                       │
│  ~/zen/DONOTASKMOREFORTHISSHIT.txt (token, ephemeral)           │
│  ~/zen/keys/zen-brain/credentials.key (AGE private key)         │
│                              │                                   │
│                              ▼                                   │
│  deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh           │
│    - Validates token against Jira                               │
│    - Encrypts to ~/zen/keys/zen-brain/secrets.d/jira.enc        │
│    - Creates ZenLock manifest                                   │
│    - Updates K8s secret                                         │
│    - Deletes plaintext token                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Runtime Phase (Cluster)                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Pod: foreman, factory-fill, scheduler, etc.            │   │
│  │                                                         │   │
│  │  Annotation: zen-lock/inject: jira-credentials          │   │
│  │                                                         │   │
│  │  Volume Mount:                                          │   │
│  │    /zen-lock/secrets/                                   │   │
│  │      ├─ JIRA_URL                                        │   │
│  │      ├─ JIRA_EMAIL                                      │   │
│  │      ├─ JIRA_API_TOKEN                                  │   │
│  │      └─ JIRA_PROJECT_KEY                                │   │
│  │                                                         │   │
│  │  Code Access:                                           │   │
│  │    secrets.ResolveJira(JiraResolveOptions{              │   │
│  │      ClusterMode: true,                                 │   │
│  │      DirPath: "/zen-lock/secrets",                      │   │
│  │    })                                                   │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Rotation Procedure

```bash
# 1. Generate new API token at:
#    https://id.atlassian.com/manage-profile/security/api-tokens

# 2. Place token in bootstrap file (single line, no newline)
echo -n 'ATATT3x...' > ~/zen/DONOTASKMOREFORTHISSHIT.txt

# 3. Run bootstrap (validates, encrypts, updates all consumers, deletes plaintext)
~/zen/zen-brain1/deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh

# 4. Verify
zen-brain office doctor
# Expected: Jira auth OK, credentials source: zenlock-dir:/zen-lock/secrets

# 5. Confirm plaintext deleted
test -f ~/zen/DONOTASKMOREFORTHISSHIT.txt && echo "FAIL" || echo "OK"
```

**The bootstrap script handles:**
- Token validation against Jira API (HTTP 200 check)
- Encryption into `jira.enc` with age keypair
- ZenLock manifest update
- K8s secret rotation
- Service restarts (if applicable)
- **Secure deletion of plaintext token**

---

## Code Access Patterns

### Cluster Mode (Production)

```go
import "github.com/kube-zen/zen-brain1/internal/secrets"

// Cluster mode - hard-fail if ZenLock not available
jiraCreds, err := secrets.ResolveJira(secrets.JiraResolveOptions{
    ClusterMode: true,
    DirPath:     "/zen-lock/secrets",
})
if err != nil {
    // Hard-fail: no env fallback in cluster mode
    log.Fatalf("cluster mode: %v", err)
}

// Use credentials
client := jira.NewClient(jiraCreds)
```

---

## CI Guardrails

Direct credential access is blocked:
- ❌ Environment variable reads for credentials
- ❌ Deprecated constructor methods

Use canonical resolver instead:
- ✅ `secrets.ResolveJira(opts)` with ClusterMode=true for cluster
- ✅ `secrets.ResolveJira(opts)` with ClusterMode=false for local
- ✅ `config.LoadJiraConfig()` which uses canonical resolver

```

---

## K8s Manifest Pattern

### ✓ DO: ZenLock Mount-Only

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: foreman
  annotations:
    zen-lock/inject: jira-credentials
spec:
  template:
    spec:
      containers:
      - name: foreman
        volumeMounts:
        - name: zen-lock-secrets
          mountPath: /zen-lock/secrets
          readOnly: true
      volumes:
      - name: zen-lock-secrets
        csi:
          driver: zen-lock.csi.k8s.io
          readOnly: true
```

---

## CI Guardrails

### Gate Suite: credentials

Run all credential gates:
```bash
python3 scripts/ci/run.py --suite credentials
```

### Individual Gates

| Gate | Purpose | Files Scanned |
|------|---------|---------------|
| `canonical_credential_access_gate.py` | Block raw credential access outside allowlist | 307+ files |
| `no_secret_echo_gate.py` | Block secret exposure patterns | 825+ files |
| `no_alt_credential_rails_gate.py` | Block alternate credential files/paths | 826+ files |
| `zenlock_mount_only_gate.py` | Enforce ZenLock mount-only in K8s manifests | 34 manifests |
| `docs_drift_credential_rails_gate.py` | Ensure docs consistency | 4 docs |

### Gate Behavior

- **Block NEW violations** in any non-allowlisted file
- **Fail CI** if violations introduced
- **Allowlisted files** are ONLY those being actively migrated (temporary)
- **Self-exemption**: Gate scripts exempted from their own checks

---

## Quarantined Scripts (DO NOT USE)

The following scripts are **HARD-FAIL DEPRECATED** and will block execution:

| Script | Status | Replacement |
|--------|--------|-------------|
| `scripts/install_jira_credentials.py` | ❌ QUARANTINED | `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh` |
| `scripts/load_jira_credentials.py` | ❌ QUARANTINED | `/zen-lock/secrets/*` (cluster) or `secrets.ResolveJira()` (local) |
| `scripts/zen-lock-source.sh` | ❌ QUARANTINED | `secrets.ResolveJira()` with `DirPath` |

**Why quarantined:**
- Use forbidden credential paths (`~/.zen-brain/secrets/*`, `~/.zen-lock/*`)
- Export credentials as environment variables (forbidden in cluster mode)
- Violate canonical credential model enforced by CI gates

---

## Validation Commands

### Cluster Runtime

```bash
# Check ZenLock status
kubectl get zenlock jira-credentials -n zen-brain

# Validate Jira connectivity
kubectl exec -n zen-brain deployment/foreman -- zen-brain office doctor

# Expected output:
#   Credentials source: zenlock-dir:/zen-lock/secrets
#   Jira auth: OK
```

### Local Runtime

```bash
# Validate with canonical resolver
go run cmd/zen-brain/main.go office doctor

# Expected output:
#   Credentials source: env (or dir, or file)
#   Jira auth: OK
```

### Preflight Check

```bash
# Before any Jira-backed work
MODE=preflight STRICT=true ./cmd/admission-gate/admission-gate

# Exit 0 = proceed
# Exit != 0 = DO NOT proceed with Jira work
```

---

## Security Model

### Principles

1. **Single Source of Truth**: One resolver, one path per mode
2. **No Secret Logging**: Capability reporting only (paths, booleans), never values
3. **No Env Fallback in Cluster**: Hard-fail if ZenLock unavailable
4. **Ephemeral Plaintext**: Token exists only during rotation (seconds)
5. **Encrypted at Rest**: `jira.enc` is age-encrypted
6. **CI Enforcement**: Gates block regressions

### Capability Reporting

Services emit at startup (no secret values):
```
[CAPABILITY] Jira Token Source: /zen-lock/secrets/JIRA_API_TOKEN
[CAPABILITY] Token Readable: true
[CAPABILITY] Read Allowed: true
[CAPABILITY] Update Allowed: true
[CAPABILITY] Create Allowed: true
[CAPABILITY] Git SSH Auth: WORKING
```

---

## Troubleshooting

### "Jira auth fails with 401"

1. **Check email from live pod (capability check, not secret value):**
   ```bash
   kubectl exec -n zen-brain deployment/foreman -- test -f /zen-lock/secrets/JIRA_EMAIL && echo "JIRA_EMAIL present" || echo "JIRA_EMAIL missing"
   ```
   Expected: `JIRA_EMAIL present`

   **DO NOT cat secret files** — only verify presence. For email validation:
   ```bash
   kubectl exec -n zen-brain deployment/foreman -- zen-brain office doctor | grep "Credentials source"
   ```
   Expected: `Credentials source: zenlock-dir:/zen-lock/secrets`

2. **If email correct, token may be expired:**
   - Follow rotation procedure above
   - Generate new token at Atlassian
   - Run bootstrap script

### "Credentials not found in cluster mode"

1. **Check ZenLock status:**
   ```bash
   kubectl get zenlock jira-credentials -n zen-brain -o yaml
   ```

2. **Check pod annotation:**
   ```bash
   kubectl get pod -n zen-brain -l app=foreman -o jsonpath='{.items[0].metadata.annotations}'
   ```
   Must have: `zen-lock/inject: jira-credentials`

3. **Check mount status:**
   ```bash
   kubectl exec -n zen-brain deployment/foreman -- zen-brain office doctor
   ```

### "CI gate failing on credential access"

1. **Check what triggered:**
   ```bash
   python3 scripts/ci/canonical_credential_access_gate.py --verbose
   ```

2. **If your file is flagged:**
   - Use `secrets.ResolveJira()` instead of `os.Getenv()`
   - Use `secrets.ResolveGit()` instead of direct SSH key access
   - Do NOT add to allowlist unless actively migrating

---

## DO / DON'T Summary

### ✓ DO
- Use `secrets.ResolveJira()` and `secrets.ResolveGit()` for all credential access
- Use `/zen-lock/secrets/*` in cluster mode (mount-only)
- Use `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh` for rotation
- Delete plaintext token after successful rotation
- Verify with `zen-brain office doctor`
- Report capability (paths, booleans), never secret values

### ✗ DON'T
- Use environment variable access for Jira credentials
- Use `zen-lock/inject-env: "true"` for Jira/Git
- Keep plaintext token files after rotation
- Ask for credentials when canonical sources exist

---

## Change Process

To modify these rules:

1. Propose change to operator
2. Get explicit approval
3. Update this file
4. Update CI gates if needed
5. Verify from live runtime context
6. Commit with message explaining change

**No other process is valid.**

---

## History

- **2026-04-02**: Version 3.0 - Credential Rails Enforcement (Layers 1-2 complete)
- **2026-03-29**: Version 2.0 - ZenLock integration
- **2026-03-19**: Version 1.0 - Initial credential model
