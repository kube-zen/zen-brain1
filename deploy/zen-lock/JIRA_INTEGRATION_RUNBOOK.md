# Zen-Brain Jira Integration - Canonical Setup

This is the single authoritative runbook for Jira credential management. No other docs needed.

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
| JIRA_EMAIL | User email (e.g., `zen@zen-mesh.io`) |
| JIRA_API_TOKEN | API token (user-level `ATATT3...`) |
| JIRA_PROJECT_KEY | Project key (e.g., `ZB`) |

## Service Account Access

Only these service accounts may use Jira credentials:

- `zb-nightshift-sa` - Night-shift worker
- `zb-reporter-sa` - Reporter worker
- `zb-planner-sa` - Planner worker

To add access, edit `allowedSubjects` in the ZenLock resource.

## Setup: Kubernetes Runtime

### Step 1: Generate ZenLock Resource

```bash
cd deploy/zen-lock
./generate-jira-secret.sh
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
JIRA_EMAIL="zen@zen-mesh.io"
JIRA_TOKEN="ATATT3..."
JIRA_PROJECT_KEY="ZB"
```

**IMPORTANT:**
- Never commit this file to Git
- Add `~/.zen-brain/jira-credentials.env` to `.gitignore`

### Step 2: Load Credentials

```bash
source deploy/zen-lock/load-jira-credentials.sh
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
cd deploy/zen-lock
./generate-jira-secret.sh

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
source deploy/zen-lock/load-jira-credentials.sh

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
2. Re-run: `./generate-jira-secret.sh`
3. Re-apply: `kubectl apply -f jira-zenlock.yaml`

### office doctor: Credentials not present

**Kubernetes runtime:**
- Check Pod has `zenbrain-sa` or allowed SA
- Check ZenLock `status.phase` is `Ready`

**Host runtime:**
- Check `~/.zen-brain/jira-credentials.env` exists
- Run: `source deploy/zen-lock/load-jira-credentials.sh`

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
| Generate encrypted secret | `./deploy/zen-lock/generate-jira-secret.sh` |
| Deploy to cluster | `kubectl apply -f deploy/zen-lock/jira-zenlock.yaml` |
| Load host credentials | `source deploy/zen-lock/load-jira-credentials.sh` |
| Validate integration | `./bin/zen-brain office doctor` |
| Check ZenLock status | `kubectl get zenlock jira-credentials -n zen-brain` |

## Next Credentials (Future)

After Jira is canonical, apply same pattern in order:
1. Redis
2. Cockroach
3. Git provider
4. AWS/GCP

Each gets: ZenLock resource → ServiceAccount restrictions → Validation commands → Single runbook.
