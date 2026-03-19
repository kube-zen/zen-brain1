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
| `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` | AGE private key | ✓ |
| `~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age` | AGE public key | ✓ |
| `~/zen/DONOTASKMOREFORTHISSHIT.txt` | Jira API token | ✓ |

## Non-Negotiable Rules

1. **NEVER** ask the operator for Jira token if the file exists
2. **NEVER** print the token to stdout/stderr
3. **NEVER** commit plaintext credentials
4. **NEVER** use placeholder keys in live zen-lock
5. **ALWAYS** validate with Deployment-managed foreman pod

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
