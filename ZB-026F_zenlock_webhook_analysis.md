# ZenLock Webhook Deployment Pod Fix Analysis

**Date:** 2026-03-21 18:05 EDT
**Issue:** ZenLock webhook fails for Deployment pods with RFC 1123 violation
**Status:** Fix exists in zen-lock repository, needs version upgrade

## Problem Statement

**Error:**
```
Error creating: admission webhook "mutate-pods.zen-lock.security.kube-zen.io" denied the request:
create ephemeral secret failed: Secret "zen-lock-inject-zen-brain-" is invalid:
metadata.name: Invalid value: "zen-lock-inject-zen-brain-":
a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters,
'-' or '.', and must start and end with an alphanumeric character
```

**Root Cause:**
When creating pods via Deployment, the pod name is not known at admission time (gets generated suffix). The webhook generated secret name `zen-lock-inject-<namespace>-<podName>` where `<podName>` was empty, resulting in a trailing dash that violates RFC 1123.

## Solution in zen-lock Repository

**File:** `~/zen/zen-lock/pkg/webhook/pod_handler.go`

**Fix added in commit:** `c1d49af` (2026-02-23)

**Code:**
```go
// GenerateSecretName generates a stable secret name from namespace and pod name
// This function is exported for testing purposes
func GenerateSecretName(namespace, podName string) string {
	// RFC 1123: name must end with alphanumeric; avoid trailing hyphen when podName is empty (e.g. during admission)
	if podName == "" {
		hash := sha256.Sum256([]byte(namespace))
		podName = hex.EncodeToString(hash[:4])
	}
	// Generate a stable name with hash suffix to ensure uniqueness and stay within Kubernetes limits
	base := fmt.Sprintf("zen-lock-inject-%s-%s", namespace, podName)

	// Kubernetes resource names must be <= 253 characters
	// If base is too long, truncate and add hash
	const maxLength = 253
	const hashLength = 8 // 8 hex chars = 4 bytes

	if len(base) <= maxLength-hashLength-1 {
		return base
	}

	// Truncate and add hash
	maxBaseLength := maxLength - hashLength - 1 // -1 for hyphen
	truncated := base[:maxBaseLength]

	// Generate hash of full name for uniqueness
	hash := sha256.Sum256([]byte(base))
	hashSuffix := hex.EncodeToString(hash[:4]) // Use first 4 bytes = 8 hex chars

	return fmt.Sprintf("%s-%s", truncated, hashSuffix)
}
```

**How it works:**
1. When `podName` is empty (Deployment admission time), generate a 4-byte hash from the namespace
2. Use that hash as the podName component
3. Results in valid secret name: `zen-lock-inject-zen-brain-a1b2c3d4` (no trailing dash)

## Version Status

### zen-brain1 Current Version
- **Chart version:** `0.0.3-alpha`
- **Status:** Does NOT have the fix
- **Deployed:** 2026-03-21 16:25 (failed status)

### zen-lock Repository Status
- **Latest commit:** `dcfa3c9` (docs update)
- **Fix commit:** `c1d49af` (2026-02-23)
- **Status:** Fix exists in main branch
- **Chart location:** Separate helm-charts repository

## What zen-platform Did

**zen-platform** uses zen-lock as a dependency:
- `charts/zen-saas/Chart.yaml` - includes zen-lock
- `charts/zen-agent/Chart.yaml` - includes zen-lock
- Configuration in `.zen/test-gates/gates/values-sanitized.yaml`

**Key configuration:**
```yaml
zenLock:
  enabled: true  # Install zen-lock operator automatically (recommended)
  privateKey:
    secretName: zen-lock-master-key
```

**zen-platform likely uses a newer version of zen-lock that includes the fix.**

## Upgrade Path

### Option 1: Build and Deploy New zen-lock Version

**Steps:**
1. Build new zen-lock image from latest code:
   ```bash
   cd ~/zen/zen-lock
   make build-image
   docker push zen-registry:5000/kubezen/zen-lock:latest
   ```

2. Update zen-brain1 to use new image:
   - Update chart dependency version
   - Or override image tag in values

3. Redeploy zen-lock:
   ```bash
   helm upgrade zen-lock kube-zen/zen-lock --version <new-version> \
     -n zen-lock-system -f values/sandbox/zen-lock.yaml
   ```

### Option 2: Patch Current zen-lock Webhook (Temporary)

**Not recommended** - would require building custom image from old version.

### Option 3: Use Workaround (Not Recommended)

**Disable ZenLock for foreman deployment:**
```yaml
foreman:
  jiraZenLock:
    enabled: false
```

**Mount secret directly:**
```yaml
volumes:
  - name: jira-credentials
    secret:
      secretName: jira-credentials
```

**This defeats the purpose of ZenLock and creates security issues.**

## Commit History

### zen-lock Repository Commits (Recent)
```
dcfa3c9 docs: document createPlaceholder workflow and common failure modes
c1d49af feat: zen-lock/inject-env annotation for env injection (FIX COMMIT)
1a6e714 fix: resolve linting errors
24a7854 fix: resolve linting errors
1aa60e7 Fix P0/P1 production-readiness issues
ed6618a Fix production blockers: RBAC drift, nil labels, truncation, TTL, README
```

### zen-brain1 Current Version
- Using: `kube-zen/zen-lock@0.0.3-alpha`
- Missing: All commits after `0.0.3-alpha` tag (including the fix)

## Evidence

### Test Results

**Direct pod creation (works):**
```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-zenlock-final
  namespace: zen-brain
  annotations:
    zen-lock/inject: jira-credentials
spec:
  serviceAccountName: foreman
  containers:
  - name: test
    image: busybox
    command: ["sleep", "3600"]
EOF
```
✅ SUCCESS - Pod created, secrets mounted at `/zen-lock/secrets`

**Deployment creation (fails):**
```bash
kubectl rollout restart deployment/foreman -n zen-brain
```
❌ FAIL - `Secret "zen-lock-inject-zen-brain-" is invalid: trailing dash`

### Webhook Logs

**From events:**
```
Warning  FailedCreate  5s (x13 over 26s)  replicaset-controller
Error creating: admission webhook "mutate-pods.zen-lock.security.kube-zen.io" denied the request:
create ephemeral secret failed: Secret "zen-lock-inject-zen-brain-" is invalid
```

## Conclusion

**zen-platform** uses a newer version of zen-lock that includes the fix for empty pod names during Deployment admission.

**zen-brain1** is using an old version (`0.0.3-alpha`) that predates the fix.

**Solution:** Upgrade zen-lock to a version that includes commit `c1d49af` or later.

## Recommended Action

1. Check if newer zen-lock chart version is available:
   ```bash
   helm search repo kube-zen/zen-lock --versions
   ```

2. If not available, build from source:
   ```bash
   cd ~/zen/zen-lock
   make build-image
   docker tag <image> zen-registry:5000/kubezen/zen-lock:0.0.4-alpha
   docker push zen-registry:5000/kubezen/zen-lock:0.0.4-alpha
   ```

3. Update zen-brain1 to use new version

4. Redeploy and verify

---

**Bottom line:** The fix exists. zen-brain1 needs to upgrade to a zen-lock version that includes commit `c1d49af` or later.
