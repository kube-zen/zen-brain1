# Zen-Brain Jira Integration - Canonical Setup

**ZB-025A ENFORCED:** This is the single authoritative runbook for Jira credential management. No other docs needed.

## Executive Summary

Jira credentials are managed through **ZenLock** as the ONLY source of truth in cluster mode.

**FORBIDDEN PATHS (DO NOT USE):**
- `~/.zen-brain/secrets/jira.yaml` - Legacy path, will fail in cluster mode
- Environment variables as primary source - Disabled by default
- Chat-pasted tokens - Never store or use

**CANONICAL PATHS (USE THESE):**

**Local bootstrap (operator setup):**
- `~/zen/DONOTASKMOREFORTHISSHIT.txt` - Contains JIRA_API_TOKEN (ephemeral)
- `~/zen/keys/zen-brain/credentials.key` - AGE private key (canonical)
- `~/zen/keys/zen-brain/credentials.pub` - AGE public key (canonical)

**Runtime (cluster):**
- `/zen-lock/secrets` - Mounted by ZenLock, ONLY allowed source

**Legacy keys (deprecated but still exist):**
- `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` → Use `~/zen/keys/zen-brain/credentials.key`
- `~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age` → Use `~/zen/keys/zen-brain/credentials.pub`

**Setup command:**
```bash
deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh
```

**Sanity check:**
```bash
scripts/check_jira_canonical_path.sh
```

## Overview

Jira credentials are managed through **ZenLock** as the source of truth. Access is restricted to specific service accounts via **AllowedSubjects**.

## Prerequisites

- zen-lock CLI installed
- Access to Kubernetes cluster (or host runtime)
- zen-brain repository checked out

## Canonical Credential Source

**ZenLock CRD:** `security.kube-zen.io/v1alpha1/ZenLock`
**Resource Name:** `jira-credentials`
**Namespace:** `zen-brain`

### Credential Fields

| Field | Description |
|--------|-------------|
| JIRA_URL | Base URL (e.g., `https://zen-mesh.atlassian.net`) |
| JIRA_EMAIL | User email (e.g., `zen@kube-zen.io`) |
| JIRA_API_TOKEN | API token (user-level `ATATT3...`) |
| JIRA_PROJECT_KEY | Project key (e.g., `ZB`) |

## ONE-WAY SETUP FLOW (ZB-025A)

### Step 1: Prepare Local Files

Create these files in `~/zen/`:

```bash
# Jira API token (paste from Atlassian)
cat > ~/zen/DONOTASKMOREFORTHISSHIT.txt << 'EOF'
ATATT3...your-full-token-here...
EOF

# Generate AGE keypair (if not exists) - CANONICAL PATH
mkdir -p ~/zen/keys/zen-brain
age-keygen -o ~/zen/keys/zen-brain/credentials.key
age-keygen -y ~/zen/keys/zen-brain/credentials.key > ~/zen/keys/zen-brain/credentials.pub

# Legacy path (deprecated, but still works for backward compatibility)
# age-keygen -o ~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age
```

**NOTE:**
- Get API token from: https://id.atlassian.com/manage-profile/security/api-tokens
- Token format MUST be `ATATT3...` (user-level)
- Workspace tokens (`ATCTT3...`) do NOT work

### Step 2: Run Bootstrap Script

```bash
cd ~/zen/zen-brain1
./deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh
```

**This script:**
- Reads credentials from `~/zen/DONOTASKMOREFORTHISSHIT.txt`
- Encrypts with AGE keys
- Creates ZenLock manifest
- Updates Kubernetes secret
- Restarts zen-lock-webhook and zen-lock-controller
- Applies ZenLock to cluster
- Updates foreman config
- Validates installation

### Step 3: Validate Setup

```bash
# Run sanity check
scripts/check_jira_canonical_path.sh

# Or validate with office doctor
./bin/zen-brain office doctor
./bin/zen-brain office smoke-real
```

**Expected output:**
```
✓ ALL CHECKS PASS
Jira credentials are using canonical source of truth
```

## Alternative Setup Methods (NOT RECOMMENDED)

### Method A: generate_jira_secret.py (Manual)

This script exists but requires manual Jira token entry.
Prefer `bootstrap-jira-zenlock-from-local.sh` instead.

```bash
# Only use if you can't use bootstrap script
python3 scripts/generate_jira_secret.py
kubectl apply -f deploy/zen-lock/jira-zenlock.yaml
```

### Method B: Host Runtime (Local Dev Only)

For local development outside Kubernetes, you may use:
- `~/.zen-brain/jira-credentials.env` (NOT `secrets/jira.yaml`)

**WARNING:** This is for local dev ONLY. Never use in cluster mode.

```bash
cat > ~/.zen-brain/jira-credentials.env << 'EOF'
JIRA_URL="https://zen-mesh.atlassian.net"
JIRA_EMAIL="zen@kube-zen.io"
JIRA_TOKEN="ATATT3..."
JIRA_PROJECT_KEY="ZB"
EOF

python3 scripts/load_jira_credentials.py
```

## Service Account Access

Only these service accounts may use Jira credentials:

- `zb-nightshift-sa` - Night-shift worker
- `zb-reporter-sa` - Reporter worker
- `zb-planner-sa` - Planner worker

To add access, edit `allowedSubjects` in the ZenLock resource.

## Setup: Kubernetes Runtime

### Step 1: Generate ZenLock Resource

```bash
python3 scripts/generate_jira_secret.py
```

This will:
- Generate zen-lock keypair (if not exists)
- Prompt for Jira credentials
- Encrypt credentials as ciphertext
- Create `jira-zenlock.yaml` manifest

### Step 2: Deploy to Cluster

```bash
kubectl apply -f deploy/zen-lock/jira-zenlock.yaml
```

ZenLock controller will:
- Decrypt the secret
- Create ephemeral Kubernetes Secret
- Inject env vars into Pod

### Step 3: Validate

```bash
# Check ZenLock status
kubectl get zenlock jira-credentials -n zen-brain

# Verify phase is Ready
kubectl get zenlock jira-credentials -n zen-brain -o jsonpath='{.status.phase}'
# Expected: Ready

# Validate Jira access
./bin/zen-brain office doctor
./bin/zen-brain office fetch ZB-XXX
```

## Setup: Host Runtime

### Step 1: Create Credential File

Create `~/.zen-brain/jira-credentials.env`:

```bash
JIRA_URL="https://zen-mesh.atlassian.net"
JIRA_EMAIL="zen@kube-zen.io"
JIRA_TOKEN="ATATT3..."
JIRA_PROJECT_KEY="ZB"
```

**IMPORTANT:**
- Never commit this file to Git
- Add `~/.zen-brain/jira-credentials.env` to `.gitignore`

### Step 2: Load Credentials

```bash
python3 scripts/load_jira_credentials.py
```

This will:
- Check for credential file
- Load credentials into environment
- Validate all fields are present
- Display confirmation

### Step 3: Validate

```bash
./bin/zen-brain office doctor
./bin/zen-brain office fetch ZB-XXX
```

## Rotation and Updates

### Kubernetes Runtime

```bash
# 1. Regenerate encrypted secret
python3 scripts/generate_jira_secret.py

# 2. Apply updated ZenLock
kubectl apply -f deploy/zen-lock/jira-zenlock.yaml

# 3. Validate
./bin/zen-brain office doctor
```

### Host Runtime

```bash
# 1. Edit credential file
vi ~/.zen-brain/jira-credentials.env

# 2. Reload
python3 scripts/load_jira_credentials.py

# 3. Validate
./bin/zen-brain office doctor
```

## Validation Commands

| Command | Purpose | Success Criteria |
|----------|-----------|-----------------|
| `kubectl get zenlock jira-credentials -n zen-brain` | Check ZenLock exists | Resource exists |
| `kubectl get zenlock jira-credentials -n zen-brain -o jsonpath='{.status.phase}'` | Check ZenLock status | Phase: `Ready` |
| `./bin/zen-brain office doctor` | Validate Jira integration | Credentials present, API reachable |
| `./bin/zen-brain office fetch ZB-XXX` | Test Jira fetch | Returns work item details |

## Troubleshooting

### AI Asks for Jira Token Again

**This should NEVER happen after ZB-025A enforcement.**

If an AI assistant asks for Jira token, run:

```bash
# 1. Run sanity check
scripts/check_jira_canonical_path.sh

# 2. If check fails, follow output:
#    deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh

# 3. Verify canonical path is being used:
#    - Local: ~/zen/DONOTASKMOREFORTHISSHIT.txt exists
#    - Runtime: /zen-lock/secrets is being used
```

**Why this happens:**
- Old documentation still mentions `~/.zen-brain/secrets/jira.yaml`
- Code/docs drift if CI gates are bypassed
- AI reading outdated runbooks

**Permanent fix:**
ZB-025A added hard-fail in cluster mode. The code now FORBIDS:
- `credentials_file` usage in cluster mode
- Silent fallback to wrong paths
- AI continuing if canonical path missing

### ZenLock Phase = Error

**Check:**
```bash
kubectl describe zenlock jira-credentials -n zen-brain
```

**Common causes:**
- Invalid public/private key pair
- Malformed encrypted data
- AllowedSubjects references non-existent ServiceAccount

**Fix:**
1. Regenerate keypair: `zen-lock keygen --output ~/.zen-lock/private-key.age`
2. Re-run: `python3 scripts/generate_jira_secret.py`
3. Re-apply: `kubectl apply -f deploy/zen-lock/jira-zenlock.yaml`

### office doctor: Credentials not present

**Kubernetes runtime:**
- Check Pod has `zenbrain-sa` or allowed SA
- Check ZenLock `status.phase` is `Ready`

**Host runtime:**
- Check `~/.zen-brain/jira-credentials.env` exists
- Run: `python3 scripts/load_jira_credentials.py`

### office doctor: API reachability failed

**Check:**
1. JIRA_URL is correct
2. JIRA_EMAIL and JIRA_TOKEN are valid
3. Network can reach Jira API

**Test:**
```bash
curl -u "$JIRA_EMAIL:$JIRA_TOKEN" "$JIRA_URL/rest/api/3/myself"
```

### office fetch: 401 Unauthorized

**Cause:** Invalid API token

**Fix:**
1. Generate new API token in Jira: Settings → API tokens
2. Update credentials
3. Re-apply: `kubectl apply -f jira-zenlock.yaml` (or edit env file)

## Security Notes

### ZenLock Properties

- **Zero-Knowledge:** API server/etcd stores only ciphertext
- **Ephemeral:** Decrypted secrets exist only in Pod memory
- **RBAC-Bound:** Only specified ServiceAccounts can access
- **GitOps-Safe:** Ciphertext can be committed to Git

### Credential Type Requirements

Use **user-level API tokens** (`ATATT3...`) with Basic Auth.
**Do NOT use** workspace-level Connect tokens (`ATCTT3...`) - they do not work with Basic Auth.

### Service Account Isolation

Each service account has limited access:
- `zb-nightshift-sa` - Safe non-critical tasks only
- `zb-reporter-sa` - Jira comments/artifacts only
- `zb-planner-sa` - Read/write Jira, no cloud creds

Never grant broad access to all service accounts.

## Quick Reference

| Task | Command |
|-------|----------|
| Generate encrypted secret | `python3 scripts/generate_jira_secret.py` |
| Deploy to cluster | `kubectl apply -f deploy/zen-lock/jira-zenlock.yaml` |
| Load host credentials | `python3 scripts/load_jira_credentials.py` |
| Validate integration | `./bin/zen-brain office doctor` |
| Check ZenLock status | `kubectl get zenlock jira-credentials -n zen-brain` |

## Next Credentials (Future)

After Jira is canonical, apply same pattern in order:
1. Redis
2. Cockroach
3. Git provider
4. AWS/GCP

Each gets: ZenLock resource → ServiceAccount restrictions → Validation commands → Single runbook.

---

## ZB-025A: A/B/C Confusion Resolution

### The Recurring Question

AI assistants kept asking this question:

> Should I copy a Jira token from repo? (A)
> Should I use ~/.zen-brain/secrets/jira.yaml? (B)
> Should I generate a token and run bootstrap script? (C)

### The Permanent Answer

**A) NO.** Do not copy a token from repo.
- Repo should NEVER contain secrets
- Even if found, do not use it
- Use canonical local source instead

**B) NO.** Do NOT use `~/.zen-brain/secrets/jira.yaml`.
- This is a legacy path that caused confusion
- It is FORBIDDEN in cluster mode
- Code will hard-fail if you try

**C) YES.** This is the ONLY correct path:
1. Generate/revoke token in Atlassian when needed
2. Save to `~/zen/DONOTASKMOREFORTHISSHIT.txt`
3. Run `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh`
4. Use `zenlock-dir:/zen-lock/secrets` at runtime

### Canonical Chain

```
~/zen/DONOTASKMOREFORTHISSHIT.txt (local source)
    ↓
deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh (bootstrap)
    ↓
zen-lock security.kube-zen.io/v1alpha1 (K8s resource)
    ↓
/zen-lock/secrets (runtime mount)
    ↓
zen-brain office doctor (validation)
```

### If AI Asks Anyway

Run the sanity check:
```bash
scripts/check_jira_canonical_path.sh
```

If it passes, the AI is confused. Show it:
- Output of sanity check
- Link to this runbook section
- Ask it to re-read this section

If it fails, fix the issues it reports.

### Enforcement in Code

ZB-025A added these hard-fails:

1. **Cluster mode detection** - If running in K8s, only allow `/zen-lock/secrets`
2. **Forbidden path check** - Non-default `credentials_file` in cluster mode = ERROR
3. **Missing ZenLock = FAIL** - If Jira enabled but ZenLock secrets missing, hard-fail with:
   ```
   ERROR: Jira credentials not loaded from ZenLock
   Resolution: Run deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh
   ```
4. **No default credentials_file** - Code no longer sets default `~/.zen-brain/secrets/jira.yaml`

### Why This Matters

Before ZB-025A:
- Multiple paths in config/docs
- "Medium priority" fallback language
- No hard-fail for wrong path
- AI kept asking A/B/C every 2 hours

After ZB-025A:
- ONE canonical path enforced in code
- Clear error message if wrong path used
- Sanity check script to validate
- This runbook as single source of truth

### Quick Reference

| Task | Command |
|-------|----------|
| Validate canonical path | `scripts/check_jira_canonical_path.sh` |
| Bootstrap from local files | `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh` |
| Validate with office | `./bin/zen-brain office doctor` |
| Check ZenLock status | `kubectl get zenlock jira-credentials -n zen-brain` |
| Get API token | https://id.atlassian.com/manage-profile/security/api-tokens |

### DO NOT

❌ Copy token from repo
❌ Use `~/.zen-brain/secrets/jira.yaml`
❌ Paste token from chat
❌ Use env vars as primary source
❌ Edit config to use legacy paths

### DO

✅ Save to `~/zen/DONOTASKMOREFORTHISSHIT.txt`
✅ Run `bootstrap-jira-zenlock-from-local.sh`
✅ Use `zenlock-dir:/zen-lock/secrets` at runtime
✅ Run `scripts/check_jira_canonical_path.sh` to validate
✅ Refer AI to this runbook section if confused
