# Zen-Lock Integration for Zen-Brain

This directory contains Zen-Lock resources and scripts for canonical Jira credential management.

## Files

| File | Purpose |
|-------|---------|
| `generate-jira-secret.sh` | Generate encrypted ZenLock resource for Jira credentials |
| `load-jira-credentials.sh` | Canonical credential loader for host runtime |
| `JIRA_INTEGRATION_RUNBOOK.md` | Single authoritative runbook for Jira setup |

## Quick Start

### Kubernetes Runtime

```bash
# 1. Generate encrypted ZenLock resource
./generate-jira-secret.sh

# 2. Deploy to cluster
kubectl apply -f jira-zenlock.yaml

# 3. Validate
./bin/zen-brain office doctor
```

### Host Runtime

```bash
# 1. Create credential file at ~/.zen-brain/jira-credentials.env
#    (See JIRA_INTEGRATION_RUNBOOK.md for format)

# 2. Load credentials
source load-jira-credentials.sh

# 3. Validate
./bin/zen-brain office doctor
```

## Architecture

### Canonical Source of Truth

**ZenLock CRD** (`security.kube-zen.io/v1alpha1/ZenLock`)
- Stores encrypted Jira credentials as ciphertext
- Safe to commit to Git
- Zero-knowledge: API server/etcd cannot read plaintext

### Runtime Delivery

**Kubernetes:**
- ZenLock controller decrypts at Pod admission
- Creates ephemeral Kubernetes Secret
- Env vars injected into Pod
- Restricted to specific ServiceAccounts via AllowedSubjects

**Host Runtime:**
- Credential file at `~/.zen-brain/jira-credentials.env`
- Loaded via `load-jira-credentials.sh`
- File is gitignored (not in repo)
- Mirrors ZenLock structure for consistency

### Access Control

Allowed ServiceAccounts:
- `zb-nightshift-sa` - Night-shift worker
- `zb-reporter-sa` - Reporter worker
- `zb-planner-sa` - Planner worker

Only these SAs can access Jira credentials. See `JIRA_INTEGRATION_RUNBOOK.md` for how to add more.

## Validation

Always validate after setup:

```bash
# Check ZenLock status (Kubernetes only)
kubectl get zenlock jira-credentials -n zen-brain

# Validate Jira integration
./bin/zen-brain office doctor
./bin/zen-brain office fetch ZB-XXX
```

## Rotation

To rotate Jira credentials:

### Kubernetes Runtime

```bash
# 1. Regenerate encrypted secret
./generate-jira-secret.sh

# 2. Apply updated ZenLock
kubectl apply -f jira-zenlock.yaml

# 3. Validate
./bin/zen-brain office doctor
```

### Host Runtime

```bash
# 1. Edit credential file
vi ~/.zen-brain/jira-credentials.env

# 2. Reload
source load-jira-credentials.sh

# 3. Validate
./bin/zen-brain office doctor
```

## Security Properties

- **Zero-Knowledge:** Only ciphertext in Git/etcd
- **Ephemeral:** Secrets exist only in Pod memory
- **RBAC-Bound:** Access restricted to specific ServiceAccounts
- **GitOps-Safe:** Encrypted manifests can be committed

## Troubleshooting

See `JIRA_INTEGRATION_RUNBOOK.md` for detailed troubleshooting.

## Next Credentials

After Jira is canonical, apply same pattern in order:
1. Redis
2. Cockroach
3. Git provider
4. AWS/GCP

Each gets: ZenLock resource → ServiceAccount restrictions → Validation commands → Single runbook.
