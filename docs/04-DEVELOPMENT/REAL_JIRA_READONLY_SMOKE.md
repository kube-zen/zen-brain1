# Real Jira Read-Only Smoke Test

This document describes how to validate Jira integration with real credentials using read-only API access.

## Prerequisites

1. Jira credentials (URL, email, API token, project key)
2. zen-brain built: `make build`
3. zen-lock CLI installed: for encrypted manifests (optional)

## Credential Sources

### Local Host Path (Default)

**File:** `~/.zen-brain/secrets/jira.yaml`

**Format:**
```yaml
stringData:
  JIRA_URL: "https://your-domain.atlassian.net"
  JIRA_EMAIL: "your-email@example.com"
  JIRA_API_TOKEN: "your-api-token"
  JIRA_PROJECT_KEY: "YOURPROJ"
```

**Resolution:** Automatically resolved via canonical credential resolver in `internal/config`.
Priority: `credentials_file` (medium priority).

### In-Cluster Path (Optional)

**Mount path:** `/zen-lock/secrets`

**Secret manifest:** `deploy/zen-lock/jira-credentials.zenlock.yaml`

**Resolution:** Automatically resolved when ZenLock controller decrypts and mounts.
Priority: `credentials_dir` (highest priority).

**Consumer:** foreman (opt-in via `foreman.jiraZenLock.enabled=true` in Helm values).

## Installation

### Install Credentials (Local)

```bash
# Create input file with your credentials
cat > /path/to/jira-input.yaml << 'EOF'
stringData:
  JIRA_URL: "https://your-domain.atlassian.net"
  JIRA_EMAIL: "your-email@example.com"
  JIRA_API_TOKEN: "your-api-token"
  JIRA_PROJECT_KEY: "YOURPROJ"
EOF

# Install credentials
make jira-install FILE=/path/to/jira-input.yaml
```

This creates:
- `~/.zen-brain/secrets/jira.yaml` (local host file)
- `deploy/zen-lock/jira-credentials.zenlock.yaml` (encrypted manifest)

## Validation

### 1. Office Doctor

Check configuration and credential resolution:

```bash
./bin/zen-brain office doctor
```

Expected output:
```
=== Office Doctor ===
Config: loaded from file/env
Connectors: jira

=== Office Pipeline Components ===
  knowledge_base: ✗ mode=disabled
  ledger:         ✗ mode=disabled
  message_bus:    ✗ mode=disabled
Cluster mapping: default -> jira
Jira base URL: https://your-domain.atlassian.net
Project key: YOURPROJ
Webhook: enabled=true, path=/webhook, port=8080
Credentials: present=true
Credentials source: host-file:/home/user/.zen-brain/secrets/jira.yaml
Connector: real (https://your-domain.atlassian.net)
API reachability: ok
```

**Key indicators:**
- `Credentials present: true` - Credentials found and loaded
- `Credentials source: host-file:...` or `zenlock-dir:...` - Canonical source used
- `Connector: real` - Not mock connector
- `API reachability: ok` - Jira API accessible

### 2. Smoke Real Test

Validate read-only Jira API access:

```bash
./bin/zen-brain office smoke-real
```

Expected output:
```
=== Office Smoke Real (Jira API Reachability) ===

Config: loaded from file/env
Connectors: jira
Cluster mapping: default -> jira

=== Credential Check ===
Credentials present: true
Credentials source: host-file:/home/user/.zen-brain/secrets/jira.yaml
Connector: real

=== API Reachability ===
API reachability: PASS

=== Read-Only Project Search ===
Project: YOURPROJ
Search query: project = YOURPROJ ORDER BY created DESC
Executing search (read-only)...
Search: PASS

=== Smoke Real Summary ===
✓ API reachability validated
✓ Read-only query executed
✓ Jira integration functional

Jira is ready for use with canonical credential source
```

**Key indicators:**
- `API reachability: PASS` - Authentication successful
- `Search: PASS` - Read-only API access working
- `Jira integration functional` - Ready for use

## In-Cluster Usage (Optional)

### Enable Foreman Zen-Lock Injection

**Helm Values:**
```yaml
foreman:
  jiraZenLock:
    enabled: true
    secretName: "jira-credentials"
    mountPath: "/zen-lock/secrets"
```

**K3d Manifest:**
Uncomment annotations in `deployments/k3d/foreman.yaml`:
```yaml
annotations:
  zen-lock/inject: "jira-credentials"
  zen-lock/mount-path: "/zen-lock/secrets"
```

### Deploy ZenLock Resource

```bash
# Deploy encrypted manifest
kubectl apply -f deploy/zen-lock/jira-credentials.zenlock.yaml

# Verify ZenLock status
kubectl get zenlock jira-credentials -n zen-brain
```

### Deploy Foreman with Injection

```bash
# Using Helm
helm upgrade --install zen-brain ./charts/zen-brain --set foreman.jiraZenLock.enabled=true

# Or using k3d manifest
kubectl apply -f deployments/k3d/foreman.yaml
```

### Validate In-Cluster

```bash
# Check foreman pod has credentials mounted
kubectl exec -n zen-brain deployment/foreman -- ls /zen-lock/secrets

# Run office doctor in-cluster
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office doctor
```

## Configuration Resolution Priority

The canonical credential resolver uses this priority:

1. **credentials_dir** (`/zen-lock/secrets` default) - Highest priority
   - In-cluster ZenLock mount path
   - Used when ZenLock controller decrypts and mounts

2. **credentials_file** (`~/.zen-brain/secrets/jira.yaml` default) - Medium priority
   - Local host file
   - Used for local development

3. **Environment variables** - Lowest priority (disabled by default)
   - JIRA_URL, JIRA_EMAIL, JIRA_API_TOKEN, JIRA_PROJECT_KEY
   - Only used when `jira.allow_env_fallback: true` in config
   - Or when `ZEN_BRAIN_ALLOW_ENV_SECRETS=1` env var is set

## Security Notes

- **No secrets printed:** All commands redact API tokens
- **Read-only only:** Smoke tests use GET requests only
- **Git-safe:** Encrypted manifests (`*.zenlock.yaml`) safe to commit
- **Local files ignored:** `~/.zen-brain/secrets/` is gitignored
- **RBAC-restricted:** In-cluster access limited to specific ServiceAccounts

## Troubleshooting

### Credentials Not Found

**Symptom:** `Credentials present: false`

**Check:**
1. Verify credential file exists: `ls -la ~/.zen-brain/secrets/jira.yaml`
2. Check file permissions: Should be 0600
3. Verify file format: Should have `stringData` section

**Fix:**
```bash
# Reinstall credentials
make jira-install FILE=/path/to/jira-input.yaml
```

### API Reachability Failed

**Symptom:** `API reachability: FAILED`

**Check:**
1. Verify Jira URL is correct
2. Verify credentials are valid
3. Check network connectivity

**Fix:**
```bash
# Test with curl
curl -u email:token https://your-domain.atlassian.net/rest/api/3/myself
```

### Search Failed

**Symptom:** `Search: FAILED`

**Check:**
1. Verify project key exists in Jira
2. Verify user has read access to project

**Fix:**
```bash
# Test search query manually
# Replace YOURPROJ and your credentials
curl -u email:token "https://your-domain.atlassian.net/rest/api/3/search?jql=project=YOURPROJ"
```

## Next Steps

After Jira read-only smoke passes:
1. Enable Jira status updates (write access) for production
2. Configure webhooks for real-time updates
3. Test with actual work items and comments

See `docs/04-DEVELOPMENT/JIRA_TESTING_FINDINGS.md` for historical testing details.
