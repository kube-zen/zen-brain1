# Jira Zen-Lock Runbook (ZB-014)

**No more asking for credentials. Ever.**

## Quick Start

```bash
# One command to set up everything:
~/zen/zen-brain1/deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh
```

## Source of Truth (Local Files Only)

| File | Purpose | Never Commit |
|------|---------|--------------|
| `~/zen/keys/zen-brain/credentials.key` | AGE private key (canonical) | ✓ |
| `~/zen/keys/zen-brain/credentials.pub` | AGE public key (canonical) | ✓ |
| `~/zen/DONOTASKMOREFORTHISSHIT.txt` | Jira API token (ephemeral) | ✓ |

**Legacy keys (still exist but deprecated):**
- `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` → Use `~/zen/keys/zen-brain/credentials.key`
- `~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age` → Use `~/zen/keys/zen-brain/credentials.pub`

## Non-Negotiable Rules

1. **NEVER** ask the operator for Jira token if the file exists
2. **NEVER** print the token to stdout/stderr
3. **NEVER** commit plaintext credentials
4. **NEVER** use placeholder keys in live zen-lock
5. **ALWAYS** validate with Deployment-managed foreman pod

## DO / DON'T

### ✓ DO
- **DO** use canonical bootstrap script: `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh`
- **DO** store AGE keys in: `~/zen/keys/zen-brain/credentials.key` (canonical)
- **DO** use plaintext token file for bootstrap only: `~/zen/DONOTASKMOREFORTHISSHIT.txt`
- **DO** delete plaintext token after successful bootstrap verification
- **DO** use ZenLock injection for cluster runtime: pod annotation `zen-lock/inject: jira-credentials`
- **DO** read credentials from `/zen-lock/secrets` in cluster pods
- **DO** verify with `office doctor` and `office smoke-real` after bootstrap

### ✗ DON'T
- **DON'T** use `~/.zen-lock/private-key.age` (legacy path, not supported)
- **DON'T** use `~/.zen-brain/secrets/jira.yaml` (legacy path, not supported)
- **DON'T** use `.env.jira.local` files for secrets (only for non-secret config)
- **DON'T** use quarantined scripts: `install_jira_credentials.py`, `load_jira_credentials.py`, `zen-lock-source.sh`
- **DON'T** use `--from-literal` for AGE key secret creation (stores path string, not contents)
- **DON'T** keep plaintext token file after successful bootstrap
- **DON'T** fallback to environment variables for credentials in cluster mode
- **DON'T** use ZenLock for non-secret config (JIRA_URL, JIRA_EMAIL, JIRA_PROJECT_KEY)

### Bootstrap-Only vs Runtime-Only

| Item | Bootstrap | Runtime |
|------|-----------|---------|
| `~/zen/DONOTASKMOREFORTHISSHIT.txt` | ✓ (input) | ✗ (must be deleted) |
| `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` | ✓ (input) | ✗ (stays local) |
| ZenLock secret in cluster | ✗ (created by bootstrap) | ✓ (read-only) |
| `/zen-lock/secrets/*` | ✗ (injected by webhook) | ✓ (ONLY source) |
| Environment variables | ✗ (not for secrets) | ✗ (not for secrets) |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Local Files                               │
│  ~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age                │
│  ~/zen/DONOTASKMOREFORTHISSHIT.txt (Jira token)                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Bootstrap Script                               │
│  - Reads local files only                                        │
│  - Encrypts Jira creds with AGE                                  │
│  - Generates ZenLock manifest                                    │
│  - Updates k8s secrets                                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster                             │
│  ┌───────────────────────┐    ┌───────────────────────────────┐ │
│  │ zen-lock-system       │    │ zen-brain                     │ │
│  │ ┌───────────────────┐ │    │ ┌───────────────────────────┐ │ │
│  │ │ zen-lock-master-  │ │    │ │ foreman pod               │ │ │
│  │ │ key secret        │ │    │ │ /zen-lock/secrets/        │ │ │
│  │ │ (AGE private key) │ │    │ │  - JIRA_URL              │ │ │
│  │ └───────────────────┘ │    │ │  - JIRA_EMAIL            │ │ │
│  │         │             │    │ │  - JIRA_API_TOKEN        │ │ │
│  │         ▼             │    │ │  - JIRA_PROJECT_KEY      │ │ │
│  │ ┌───────────────────┐ │    │ └───────────────────────────┘ │ │
│  │ │ zen-lock-         │──────▶│ (decrypted at pod startup)  │ │
│  │ │ webhook           │ │    │ └───────────────────────────┘ │ │
│  │ │ (decrypts creds)  │ │    └───────────────────────────────┘ │
│  │ └───────────────────┘ │                                      │
│  └───────────────────────┘                                      │
└─────────────────────────────────────────────────────────────────┘
```

## Validation Commands

```bash
# Check ZenLock status
kubectl get zenlock jira-credentials -n zen-brain

# Check foreman pod logs
kubectl logs -n zen-brain -l app.kubernetes.io/name=foreman

# Run office doctor
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office doctor

# Run smoke test
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office smoke-real

# Search Jira issues
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office search "project = ZB"
```

## Troubleshooting

### ZenLock in Error state

```bash
# Check controller logs
kubectl -n zen-lock-system logs -l app.kubernetes.io/name=zen-lock

# Common issue: malformed secret key
# The AGE key must be just the key, no comments, no trailing newline
# Fix:
KEY=$(grep '^AGE-SECRET-KEY-1' ~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age)
printf '%s' "$KEY" > ~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age

# Restart zen-lock components
kubectl -n zen-lock-system rollout restart deployment/zen-lock-webhook
kubectl -n zen-lock-system rollout restart deployment/zen-lock-controller
```

### Foreman pod not starting

```bash
# Check events
kubectl get events -n zen-brain --sort-by='.lastTimestamp' | tail -10

# Check foreman deployment
kubectl describe deployment foreman -n zen-brain

# Common issue: webhook denied request
# Check zen-lock-webhook logs
kubectl -n zen-lock-system logs -l app.kubernetes.io/component=webhook
```

### Credentials not loading

```bash
# Check mounted secrets
kubectl exec -n zen-brain deployment/foreman -- ls -la /zen-lock/secrets/

# Check config file
kubectl exec -n zen-brain deployment/foreman -- cat /home/zenuser/.zen-brain/config.yaml
```

## Files Reference

| File | Location | Purpose |
|------|----------|---------|
| Bootstrap script | `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh` | Complete setup script |
| ZenLock manifest | `deploy/zen-lock/jira-credentials.zenlock.yaml` | Encrypted Jira credentials |
| Foreman config | `foreman-config` ConfigMap | App configuration |
| Runbook | `deploy/zen-lock/RUNBOOK.md` | This file |

## Rotation Procedure

1. Generate new Jira API token at https://id.atlassian.com/manage-profile/security/api-tokens
2. Save to `~/zen/DONOTASKMOREFORTHISSHIT.txt`
3. Run bootstrap script: `~/zen/zen-brain1/deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh`
4. Validate: `kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office doctor`

## History

- **2026-03-19**: ZB-014 completed - Jira zen-lock integration with local-only credential source

---

## Operational Observability

### How to Verify Live zen-lock Image/Digest

```bash
# Check controller image
kubectl get deploy zen-lock-controller -n zen-lock-system -o jsonpath='{.spec.template.spec.containers[0].image}'

# Check webhook image
kubectl get deploy zen-lock-webhook -n zen-lock-system -o jsonpath='{.spec.template.spec.containers[0].image}'

# Check running pod image digest
kubectl get po -n zen-lock-system -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.containerStatuses[0].imageID}{"\n"}{end}'
```

### How to Verify Live Private Key Secret is Real

```bash
# Check secret exists
kubectl get secret zen-lock-master-key -n zen-lock-system

# Verify secret size (should be ~74 bytes for key-only format)
kubectl get secret zen-lock-master-key -n zen-lock-system -o jsonpath='{.data.keyTxt}' | base64 -d | wc -c

# Verify starts with AGE-SECRET-KEY-1
kubectl get secret zen-lock-master-key -n zen-lock-system -o jsonpath='{.data.keyTxt}' | base64 -d | head -c 20
```

### How to Verify ZenLock Ready

```bash
# Check ZenLock phase
kubectl get zenlock -n zen-brain

# Should show:
# NAME               PHASE   AGE
# jira-credentials   Ready   Xs

# Detailed status
kubectl describe zenlock jira-credentials -n zen-brain
```

### How to Verify Foreman Mount Exists

```bash
# Check foreman pod has mount
FOREMAN_POD=$(kubectl get pod -n zen-brain -l app.kubernetes.io/name=foreman -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n zen-brain $FOREMAN_POD -- ls -la /zen-lock/secrets/

# Should show:
# JIRA_API_TOKEN -> ..data/JIRA_API_TOKEN
# JIRA_EMAIL -> ..data/JIRA_EMAIL
# JIRA_PROJECT_KEY -> ..data/JIRA_PROJECT_KEY
# JIRA_URL -> ..data/JIRA_URL
```

### How to Detect Decrypt/Admission Failures Quickly

```bash
# Check webhook logs for admission errors
kubectl logs -n zen-lock-system -l app=zen-lock-webhook --tail=50 | grep -i error

# Check controller logs for decrypt errors
kubectl logs -n zen-lock-system -l app=zen-lock-controller --tail=50 | grep -i error

# Check recent events in zen-brain namespace
kubectl get events -n zen-brain --sort-by='.lastTimestamp' | tail -10
```

### How to Distinguish Jira Access Failure from Injection Failure

| Symptom | Injection Failure | Jira Access Failure |
|---------|-------------------|---------------------|
| /zen-lock/secrets empty | ✓ | ✗ |
| office doctor shows "present=false" | ✓ | ✗ |
| office doctor shows "present=true" | ✗ | ✓ |
| 401/403 on API call | ✗ | ✓ |
| 404 on project lookup | ✗ | ✓ |

```bash
# Quick diagnostic
FOREMAN_POD=$(kubectl get pod -n zen-brain -l app.kubernetes.io/name=foreman -o jsonpath='{.items[0].metadata.name}')

# Check injection
kubectl exec -n zen-brain $FOREMAN_POD -- test -f /zen-lock/secrets/JIRA_API_TOKEN && echo "INJECTION: OK" || echo "INJECTION: FAILED"

# Check credentials
kubectl exec -n zen-brain $FOREMAN_POD -- /app/zen-brain office doctor 2>&1 | grep "Credentials:"
```

---

## Durable Guardrails

### Repo-Managed Secret Workflow

1. **NEVER commit local credential files:**
   - `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age`
   - `~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age`
   - `~/zen/DONOTASKMOREFORTHISSHIT.txt`
   - `~/.env.jira.local`

2. **ALWAYS use encrypted ZenLock manifests:**
   - Stored in repo as `jira-credentials.zenlock.yaml`
   - Contains age-encrypted credentials
   - Never plaintext

3. **Bootstrap from local files only:**
   - Run `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh`
   - Reads from local files, encrypts, updates k8s

### .gitignore Entries

Ensure these patterns are in `.gitignore`:

```gitignore
# Zen-Lock credentials (NEVER COMMIT)
.env.jira.local
DONOTASKMOREFORTHISSHIT.txt
ZENBRAIN*NEVERDELETETHISSHIT.age
zen-lock-private-key.age
zen-lock-public-key.age
*.zenlock.yaml.bak
```

### CI Secret Scanning

The repo has a GitHub Actions workflow for secret scanning:
- Location: `.github/workflows/secret-scan.yaml`
- Scans for: `ATATT3...` and `ATCTT3...` token patterns
- Runs on: push and pull_request

```bash
# Verify workflow exists
cat .github/workflows/secret-scan.yaml
```

### Pre-commit Hook (Supplemental)

Local pre-commit hooks are supplemental. The primary guardrails are:
1. CI secret scanning
2. .gitignore patterns
3. Encrypted manifests only

## ZB-017: Intended Path Migration (2026-03-19)

### Status: COMPLETE

The webhook bypass workaround has been removed. Zen-brain now runs on the intended Zen-Lock path.

### Key Changes

1. **Helm Chart RBAC Fixed**: Webhook ClusterRole now has `get, list, watch` verbs for zenlocks
2. **ZenLock Re-encrypted**: Jira credentials encrypted with matching public key
3. **Foreman Config**: ConfigMap mounted at `/home/zenuser/.zen-brain/config.yaml` enables Jira
4. **Intended Path Active**: Webhook creates secrets, pods receive mounts

### Credential Strategy (Verified)

- **Public Key**: `age18c8nfva9zhjjvhqkln60cxly5c0y5k08vyw9rh22hgt3rq6pn5aspu6nnv` (from `ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age`)
- **Private Key**: Stored in `zen-lock-master-key` secret in `zen-lock-system` namespace
- **ZenLock**: Encrypted with above public key, stored in `zen-brain` namespace

### Verification

\`\`\`bash
# ZenLock status
kubectl get zenlock -n zen-brain jira-credentials -o yaml | grep -A5 'status:'

# Expected output:
#   phase: Ready
#   Decryptable: True

# Foreman credentials source
kubectl exec -n zen-brain deploy/foreman -- /app/zen-brain office doctor | grep "Credentials source"

# Expected output:
#   Credentials source: zenlock-dir:/zen-lock/secrets

# Full smoke test
kubectl exec -n zen-brain deploy/foreman -- /app/zen-brain office smoke-real
\`\`\`

### Files Changed

| File | Change |
|------|--------|
| `~/zen/helm-charts/charts/zen-lock/templates/rbac.yaml` | Already had correct RBAC |
| `zen-lock-master-key` secret | Updated with correct private key |
| `jira-credentials` ZenLock | Re-encrypted with matching public key |
| `foreman-config` ConfigMap | Created to enable Jira |
| `foreman` Deployment | Added config volume mount |

### Helm Upgrade Path

\`\`\`bash
cd ~/zen/helm-charts
helm upgrade zen-lock charts/zen-lock -n zen-lock-system --reuse-values
\`\`\`

### Known Issues (Resolved)

- ~~RBAC drift~~: Fixed - Helm chart already had correct RBAC, deployed ClusterRole was manually patched then Helm upgrade synced it
- ~~AGE key mismatch~~: Fixed - ZenLock re-encrypted with matching public key
- ~~Placeholder master key~~: Fixed - Updated with actual private key

## DO / DO NOT (ZB-025B-SEC Security Model)

DO:
- Place plaintext token only temporarily for bootstrap at `~/zen/DONOTASKMOREFORTHISSHIT.txt`
- Bootstrap into ZenLock using AGE encryption
- Verify with `office doctor` / `office smoke-real`
- Delete plaintext file after successful verification
- Use only `/zen-lock/secrets` as runtime credential source

DO NOT:
- Run runtime from plaintext Jira files
- Keep plaintext token lying around after verification
- Commit tokens to repository
- Ask for creds again if bootstrap artifacts already exist locally
- Use `~/.zen-brain/secrets/jira.yaml` as active path
- Use host-file or env-var Jira sources in cluster mode

## Security Enforcement

### Bootstrap Phase (Temporary)
- Plaintext file: `~/zen/DONOTASKMOREFORTHISSHIT.txt`
- Purpose: Temporary input only for bootstrap
- Lifecycle: Created by operator → Encrypted → Deleted after verification
- Never used for runtime access

### Runtime Phase (ZenLock Only)
- Credentials source: `/zen-lock/secrets` (ZenLock-managed)
- Decryption: Happens at pod startup by zen-lock webhook
- Foreman config: `credentials_dir: "/zen-lock/secrets"`
- Fallback: `allow_env_fallback: false` (strict mode)

### Guardrails
1. Preflight check validates: credentials_dir=/zen-lock/secrets
2. Preflight check validates: plaintext bootstrap file removed
3. Preflight check validates: AGE keypair exists for bootstrap
4. Runtime fails closed if: plaintext Jira source detected
5. Runtime fails closed if: ZenLock not available

### Verification
```bash
# Check runtime credentials source
kubectl exec -n zen-brain deployment/foreman -- sh -c 'grep credentials_dir /home/zenuser/.zen-brain/config.yaml'

# Check plaintext file removed
ls ~/zen/DONOTASKMOREFORTHISSHIT.txt && echo "SECURITY FAIL: plaintext file exists" || echo "OK"

# Run security preflight checks
bash deploy/preflight-checks.sh | grep -E "Security:"
```
