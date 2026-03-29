# Break-Glass Recovery Runbook

> **Purpose:** Reduce MTTR during Zen-Lock / Jira credential incidents.

This runbook covers emergency recovery procedures for Jira integration failures.

---

## Quick Diagnosis

| Symptom | Likely Cause | Section |
|---------|--------------|---------|
| `ZenLock Phase = Error` | Private key mismatch | [1. Verify Private Key](#1-verify-private-key) |
| `office doctor: Credentials not present` | Secret not injected | [3. Force Fresh Rollout](#3-force-fresh-rollout) |
| `office smoke-real: 401 Unauthorized` | Token invalid/expired | [5. Regenerate Jira Token](#5-regenerate-jira-token) |
| `Pod admission denied` | Webhook failure | [6. Webhook Issues](#6-webhook-issues) |
| `Decrypt failed` | Wrong image version | [7. Image Issues](#7-image-issues) |

---

## 1. Verify Private Key

### Check live private key is real (not placeholder)

**Canonical path:** `~/zen/keys/zen-brain/credentials.key`

```bash
# Check private key exists at canonical path
ls -la ~/zen/keys/zen-brain/credentials.key

# Verify it's not a placeholder (should be 74 bytes, starts with AGE-SECRET-KEY-1)
wc -c ~/zen/keys/zen-brain/credentials.key
head -c 20 ~/zen/keys/zen-brain/credentials.key

# Get public key for comparison
age-keygen -y ~/zen/keys/zen-brain/credentials.key
```

**Expected:** Private key file is 74 bytes, starts with `AGE-SECRET-KEY-1`, public key output starts with `age1`.

**If missing or placeholder:** Run canonical bootstrap:
```bash
~/zen/zen-brain1/deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh
```

### Sync to cluster

```bash
# Recreate the master key secret (using canonical path and --from-file)
kubectl create secret generic zen-lock-master-key \
  --from-file=key.txt=~/zen/keys/zen-brain/credentials.key \
  -n zen-lock-system \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart controller to load new key
kubectl rollout restart deployment zen-lock-controller -n zen-lock-system
kubectl rollout status deployment zen-lock-controller -n zen-lock-system

# Restart webhook to load new key
kubectl rollout restart deployment zen-lock-webhook -n zen-lock-system
kubectl rollout status deployment zen-lock-webhook -n zen-lock-system
```

---

## 2. Verify Zen-Lock Image and Digest

### Check running image

```bash
# Controller image
kubectl get deployment zen-lock-controller -n zen-lock-system \
  -o jsonpath='{.spec.template.spec.containers[0].image}'

# Webhook image
kubectl get deployment zen-lock-webhook -n zen-lock-system \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
```

**Expected:** `zen-registry:5000/kubezen/zen-lock:0.0.3-alpha-zb1fix2` or later.

**If wrong image:** Update the helm values and redeploy.

### Verify digest (optional)

```bash
# Get digest from running pod
kubectl get pods -n zen-lock-system -l app.kubernetes.io/name=zen-lock \
  -o jsonpath='{.items[0].status.containerStatuses[0].imageID}'
```

---

## 3. Force Fresh Rollout

### For foreman deployment

```bash
# Delete pod to trigger fresh admission with current ZenLock
kubectl delete pods -n zen-brain -l app.kubernetes.io/name=foreman

# Wait for new pod
kubectl rollout status deployment/foreman -n zen-brain --timeout=60s

# Check new pod has secret volume
kubectl get pods -n zen-brain -l app.kubernetes.io/name=foreman \
  -o jsonpath='{.items[0].spec.volumes[?(@.name=="zen-secrets")]}' | jq .
```

### For test pod

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: zenlock-recovery-test
  namespace: zen-brain
  annotations:
    zen-lock/inject: jira-credentials
spec:
  serviceAccountName: foreman
  containers:
  - name: test
    image: busybox:1.36
    command: ["sh", "-c", "ls -la /zen-lock/secrets && cat /zen-lock/secrets/JIRA_PROJECT_KEY"]
    volumeMounts:
    - name: zen-secrets
      mountPath: /zen-lock/secrets
      readOnly: true
  restartPolicy: Never
EOF

# Check result
kubectl logs zenlock-recovery-test -n zen-brain

# Cleanup
kubectl delete pod zenlock-recovery-test -n zen-brain
```

---

## 4. Validate /zen-lock/secrets

### Check all four files present

```bash
kubectl exec -n zen-brain deployment/foreman -- ls -la /zen-lock/secrets
```

**Expected output:**
```
JIRA_API_TOKEN
JIRA_EMAIL
JIRA_PROJECT_KEY
JIRA_URL
```

### Check values are correct

```bash
kubectl exec -n zen-brain deployment/foreman -- cat /zen-lock/secrets/JIRA_PROJECT_KEY
# Expected: ZB
```

---

## 5. Regenerate Jira Token

### At Atlassian

1. Go to: https://id.atlassian.com/manage-profile/security/api-tokens
2. Revoke old token (if known)
3. Create new API token
4. Copy the new token

### Update ZenLock

```bash
# Create input file with new credentials
cat > /tmp/jira-input.yaml <<EOF
stringData:
  JIRA_URL: "https://zen-mesh.atlassian.net"
  JIRA_EMAIL: "zen@zen-mesh.io"
  JIRA_API_TOKEN: "NEW_TOKEN_HERE"
  JIRA_PROJECT_KEY: "ZB"
EOF

# Install credentials (regenerates ZenLock YAML)
cd ~/zen/zen-brain1
make jira-install FILE=/tmp/jira-input.yaml

# Apply updated ZenLock to cluster
kubectl apply -f deploy/zen-lock/jira-credentials.zenlock.yaml

# Verify ZenLock is Ready
kubectl get zenlock jira-credentials -n zen-brain
```

### Validate

```bash
./bin/zen-brain office doctor
./bin/zen-brain office smoke-real
```

---

## 6. Webhook Issues

### Check webhook is running

```bash
kubectl get pods -n zen-lock-system -l app.kubernetes.io/name=zen-lock

kubectl logs -n zen-lock-system deployment/zen-lock-webhook --tail=50
```

### Check webhook configuration

```bash
kubectl get mutatingwebhookconfiguration zen-lock-mutating-webhook -o yaml
```

### Restart webhook

```bash
kubectl rollout restart deployment zen-lock-webhook -n zen-lock-system
kubectl rollout status deployment zen-lock-webhook -n zen-lock-system
```

---

## 7. Image Issues

### Wrong image version

The old `0.0.3-alpha` image has a bug that causes decrypt failures.

**Fix:** Update to `0.0.3-alpha-zb1fix2` or later:

```bash
# Check helm values for zen-lock
grep -A5 "zen-lock" ~/zen-platform/helmfile.d/zen-lock.yaml

# Update image tag in helmfile or values
# Redeploy
helmfile -f ~/zen-platform/helmfile.d/zen-lock.yaml sync
```

---

## 8. Stale Deployment Template

If the deployment was created before ZenLock was Ready, it may have stale annotations.

### Check deployment annotations

```bash
kubectl get deployment foreman -n zen-brain -o yaml | grep -A5 "annotations:"
```

**Expected:**
```yaml
annotations:
  zen-lock/inject: jira-credentials
```

### Fix missing annotations

```bash
kubectl annotate deployment foreman -n zen-brain \
  zen-lock/inject=jira-credentials --overwrite

# Force rollout
kubectl rollout restart deployment foreman -n zen-brain
```

---

## Failure Mode Decision Tree

```
Pod fails to start
├── Check events: kubectl describe pod <pod> -n zen-brain
│   ├── "Error looking up service account" → RBAC issue
│   ├── "failed to call webhook" → Webhook down
│   └── "ZenLock not found" → ZenLock not applied
│
├── Check logs: kubectl logs <pod> -n zen-brain
│   ├── "credentials not present" → Secret not injected
│   └── "decrypt failed" → Private key mismatch
│
└── Check ZenLock: kubectl get zenlock -n zen-brain
    ├── Phase = Error → Key or encryption issue
    └── Phase = Ready → Check pod annotations
```

---

## Contact Points

| Issue | Owner |
|-------|-------|
| Jira token/permissions | Atlassian admin |
| Zen-Lock image bugs | Platform team |
| Private key rotation | Security team |
| Foreman CRD issues | Zen-Brain team |

---

## Revision History

| Date | Change |
|------|--------|
| 2026-03-19 | Initial version (ZB-013) |
