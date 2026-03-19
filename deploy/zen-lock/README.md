# Zen-Lock Integration for Zen-Brain

> ⚠️ **SECURITY WARNING**: Never commit real Jira API tokens to git.
> All credentials must be managed through ZenLock CRDs with encrypted data.
> See task ZB-012 for security hardening details.
> Pre-commit hooks are installed to prevent accidental token exposure.

This directory contains Zen-Lock resources and scripts for canonical Jira credential management.

## Files

| File | Purpose |
|------|---------|
| `jira-credentials.zenlock.yaml` | Encrypted ZenLock resource for Jira credentials (generated) |
| `install_jira_credentials.py` | Non-interactive installer for Jira credentials |
| `JIRA_INTEGRATION_RUNBOOK.md` | Single authoritative runbook for Jira setup |

## Quick Start

### Kubernetes Runtime (In-Cluster)

```bash
# 1. Install Jira credentials
make jira-install FILE=/absolute/path/to/jira-input.yaml

# 2. Deploy ZenLock resource to cluster
kubectl apply -f deploy/zen-lock/jira-credentials.zenlock.yaml

# 3. Enable foreman Zen-Lock injection (optional)
# Set foreman.jiraZenLock.enabled=true in Helm values
# OR uncomment annotations in deployments/k3d/foreman.yaml

# 4. Validate
./bin/zen-brain office doctor
```

### Host Runtime (Local)

```bash
# 1. Create credential input file
# (See JIRA_INTEGRATION_RUNBOOK.md for format)

# 2. Install credentials
make jira-install FILE=/absolute/path/to/jira-input.yaml

# 3. Validate
./bin/zen-brain office doctor
./bin/zen-brain office smoke-real
```

## Architecture

### Canonical Source of Truth

**ZenLock CRD** (`security.kube-zen.io/v1alpha1/ZenLock`)
- Stores encrypted Jira credentials as ciphertext
- Safe to commit to Git
- Zero-knowledge: API server/etcd cannot read plaintext

### Runtime Delivery

**Kubernetes (In-Cluster)**
- ZenLock controller decrypts at Pod admission
- Creates ephemeral Kubernetes Secret
- Mounts to `/zen-lock/secrets` in Pod
- Restricted to specific ServiceAccounts via AllowedSubjects
- **Consumer:** foreman (optional, opt-in via values/manifests)

**Host Runtime (Local)**
- Credential file at `~/.zen-brain/secrets/jira.yaml`
- Loaded via canonical credential resolver in `internal/config`
- File is gitignored (not in repo)
- Mirrors ZenLock structure for consistency

### Access Control

Allowed ServiceAccounts:
- `foreman` - Foreman controller (zen-brain namespace)

Only these SAs can access Jira credentials. See `jira-credentials.zenlock.yaml` for current allowedSubjects.

## Credential Paths

### Local Host Path
```
~/.zen-brain/secrets/jira.yaml
```
- Format: stringData with JIRA_URL, JIRA_EMAIL, JIRA_API_TOKEN, JIRA_PROJECT_KEY
- Mode: 0600
- Source: host-file

### In-Cluster Path
```
Mount path: /zen-lock/secrets
Consumer: foreman pod (opt-in via foreman.jiraZenLock.enabled=true)
Secret manifest: deploy/zen-lock/jira-credentials.zenlock.yaml
```
- Format: encrypted YAML (ZenLock CRD)
- Source: zenlock-dir

## Validation

Always validate after setup:

```bash
# Local validation
./bin/zen-brain office doctor
./bin/zen-brain office smoke-real

# In-cluster validation (once deployed)
kubectl get zenlock jira-credentials -n zen-brain
./bin/zen-brain office doctor
```

## Rotation

To rotate Jira credentials:

```bash
# 1. Regenerate encrypted secret
make jira-install FILE=/absolute/path/to/jira-input.yaml

# 2. Apply updated ZenLock
kubectl apply -f deploy/zen-lock/jira-credentials.zenlock.yaml

# 3. Validate
./bin/zen-brain office doctor
```

## Security Properties

- **Zero-Knowledge:** Only ciphertext in Git/etcd
- **Ephemeral:** Secrets exist only in Pod memory
- **RBAC-Bound:** Access restricted to specific ServiceAccounts
- **GitOps-Safe:** Encrypted manifests can be committed
- **Optional In-Cluster:** Foreman injection is opt-in, disabled by default

## Troubleshooting

See `JIRA_INTEGRATION_RUNBOOK.md` for detailed troubleshooting.

## Next Credentials

After Jira is canonical, apply same pattern in order:
1. Redis
2. Cockroach
3. Git provider
4. AWS/GCP

Each gets: ZenLock resource → ServiceAccount restrictions → Validation commands → Single runbook.
