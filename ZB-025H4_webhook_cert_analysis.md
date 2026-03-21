# Webhook Certificate Solution Analysis (ZB-025H4)

**Date:** 2026-03-21
**Focus:** How zen-platform solved zen-lock webhook certificate issues

## Executive Summary

**zen-platform** has fully automated webhook certificate management for zen-lock using Helm hooks with a self-signed certificate generation Job. **zen-brain1** currently has webhooks disabled to avoid certificate failures, requiring manual certificate generation.

## zen-platform Solution (Automated)

### Architecture

zen-platform uses a **3-mode TLS approach** with automatic certificate management:

```
┌─────────────────────────────────────────────────────────┐
│ TLS Mode Selection Logic                                │
│                                                          │
│ 1. cert-manager (if certManager.enabled=true)           │
│    └─> Uses cert-manager Certificate CRD                │
│        └─> Auto-injects caBundle via annotation         │
│                                                          │
│ 2. self-signed (default fallback)                       │
│    └─> Helm hook Job generates CA + server cert         │
│        └─> Patches MutatingWebhookConfiguration         │
│                                                          │
│ 3. provided (advanced use)                              │
│    └─> User provides TLS secret + caBundle              │
└─────────────────────────────────────────────────────────┘
```

### Implementation Details

#### 1. Self-Signed Mode (Default)

**File:** `zen-lock/templates/webhook-cert-job.yaml`

**How it works:**
1. **Helm Hook:** Runs as `post-install,post-upgrade` with weight "1"
2. **Hook Deletion:** `before-hook-creation,hook-succeeded` (clean up after success)
3. **Certificate Generation:**
   - Generates 4096-bit RSA CA (10-year validity)
   - Generates ECDSA P-256 server key (modern security)
   - Creates server certificate with proper SANs:
     - `zen-lock-webhook.{namespace}.svc`
     - `zen-lock-webhook.{namespace}.svc.cluster.local`
   - Signs server cert with CA (1-year validity)
4. **Kubernetes Integration:**
   - Creates TLS secret: `zen-lock-webhook-cert`
   - Patches `MutatingWebhookConfiguration` with CA bundle
   - Uses `replace` then `add` strategy for idempotency

**Job Container:**
```yaml
image: alpine/k8s:1.28.0  # Has kubectl + openssl
command: /bin/bash
script: |
  # Generate CA
  openssl genrsa -out ca.key 4096
  openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
    -subj "/CN=zen-lock-webhook-ca"
  
  # Generate server cert (ECDSA P-256)
  openssl ecparam -genkey -name prime256v1 -out server.key
  openssl req -new -key server.key -out server.csr \
    -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc" \
    -addext "subjectAltName=..."
  
  # Sign cert
  openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key \
    -out server.crt -days 365
  
  # Create secret + patch webhook
  kubectl create secret tls "${SECRET_NAME}" ...
  kubectl patch mutatingwebhookconfiguration ...
```

#### 2. cert-manager Mode (Production)

**File:** `zen-lock/templates/webhook.yaml`

**How it works:**
1. Creates `Certificate` CRD with ECDSA P-256
2. Uses `cert-manager.io/inject-ca-from` annotation
3. cert-manager automatically:
   - Issues certificate from specified Issuer/ClusterIssuer
   - Injects CA bundle into MutatingWebhookConfiguration
   - Handles renewal (default: renew 10 days before expiry)

**Configuration:**
```yaml
webhook:
  certManager:
    enabled: true
    issuer:
      name: letsencrypt-prod
      kind: ClusterIssuer
    duration: "2160h"  # 90 days
    renewBefore: "240h"  # 10 days
    rotationPolicy: Always
```

#### 3. Webhook Configuration

**File:** `zen-lock/templates/webhook.yaml`

**Key features:**
- Conditional `caBundle` based on TLS mode
- cert-manager mode: annotation handles injection
- self-signed mode: Job patches it post-install
- provided mode: user supplies base64-encoded caBundle

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: zen-lock-mutating-webhook
  {{- if eq $tlsMode "cert-manager" }}
  annotations:
    cert-manager.io/inject-ca-from: {{ namespace }}/{{ fullname }}-webhook-cert
  {{- end }}
webhooks:
  - name: mutate-pods.zen-lock.security.kube-zen.io
    clientConfig:
      service:
        name: zen-lock-webhook
        namespace: zen-lock-system
      {{- if eq $tlsMode "provided" }}
      caBundle: {{ .Values.webhook.tls.caBundle }}
      {{- end }}
```

### Configuration Options

```yaml
webhook:
  enabled: true
  certSecret: zen-lock-webhook-cert
  
  # Auto-detect: cert-manager if enabled, else self-signed
  tls:
    mode: ""  # cert-manager | self-signed | provided
  
  certManager:
    enabled: false  # Set true for production
    issuer:
      name: ""  # Required if enabled
      kind: ClusterIssuer
    duration: ""
    renewBefore: "240h"
    rotationPolicy: Always
  
  selfSigned:
    duration: "365d"
    caDuration: "87600h"  # 10 years
```

## zen-brain1 Current State (Manual)

### Current Configuration

**File:** `values/sandbox/zen-lock.yaml`

```yaml
webhook:
  enabled: false  # Disable webhook for sandbox to avoid cert job failures
  certManager:
    enabled: false
```

### Manual Certificate Process

**File:** `deploy/CLUSTER_RECOVERY_RUNBOOK.md`

**Steps:**
1. Generate certificate manually with openssl
2. Create proper SANs configuration file
3. Generate CA + server certificate
4. Create Kubernetes secret manually
5. No automatic renewal or rotation

**Problems:**
- Manual intervention required
- Prone to human error
- No automatic renewal
- Certificates can expire unexpectedly
- Webhook disabled to avoid failures

## Comparison Matrix

| Feature | zen-platform | zen-brain1 |
|---------|--------------|------------|
| **Webhook Status** | ✅ Enabled (default) | ❌ Disabled |
| **Cert Generation** | ✅ Automatic (Helm hook) | ⚠️ Manual |
| **Cert Mode** | ✅ 3 modes (self-signed, cert-manager, provided) | ❌ N/A (disabled) |
| **CA Management** | ✅ Auto-generated + patched | ⚠️ Manual |
| **Renewal** | ✅ Automatic (cert-manager mode) | ❌ Manual |
| **SANs** | ✅ Auto-generated | ⚠️ Manual config |
| **Idempotency** | ✅ Built-in (replace + add) | ❌ N/A |
| **Security** | ✅ ECDSA P-256 | ⚠️ RSA (if manually generated) |
| **Recovery** | ✅ Self-healing (re-run Helm) | ⚠️ Runbook steps |

## Recommendations for zen-brain1

### Option 1: Adopt zen-platform's Automated Solution (Recommended)

**Steps:**
1. Update zen-lock dependency to version with webhook-cert-job.yaml
2. Enable webhook in `values/sandbox/zen-lock.yaml`:
   ```yaml
   webhook:
     enabled: true
     certManager:
       enabled: false  # Use self-signed mode
   ```
3. Remove manual certificate steps from runbooks
4. Test: `helm upgrade --install zen-lock ./charts/zen-lock`

**Benefits:**
- Zero manual intervention
- Automatic certificate generation
- Self-healing on redeployment
- Modern ECDSA security

### Option 2: Use cert-manager (Production)

**Steps:**
1. Install cert-manager in cluster
2. Create Issuer/ClusterIssuer
3. Configure zen-lock:
   ```yaml
   webhook:
     enabled: true
     certManager:
       enabled: true
       issuer:
         name: letsencrypt-prod
         kind: ClusterIssuer
   ```

**Benefits:**
- Production-grade PKI
- Automatic renewal
- Audit trail
- Multi-namespace support

### Option 3: Keep Manual (Not Recommended)

**Current approach:**
- Webhook remains disabled
- Manual certificate generation when needed
- Higher operational burden
- Risk of expired certificates

## Implementation Checklist

If adopting zen-platform's solution:

- [ ] Verify zen-lock chart version >= 0.0.3-alpha
- [ ] Check for `webhook-cert-job.yaml` template
- [ ] Update `values/sandbox/zen-lock.yaml` to enable webhook
- [ ] Remove manual certificate steps from runbooks
- [ ] Test deployment: `make deploy-sandbox`
- [ ] Verify webhook is running: `kubectl get pods -n zen-lock-system`
- [ ] Check certificate secret: `kubectl get secret zen-lock-webhook-cert -n zen-lock-system`
- [ ] Validate webhook config: `kubectl get mutatingwebhookconfiguration`

## Technical Details

### Helm Hook Ordering

```
Helm Install/Upgrade
    ↓
1. Templates without hooks (deployments, services, etc.)
    ↓
2. webhook-cert-job.yaml (hook-weight: "1")
    ├─ Generate CA + server cert
    ├─ Create TLS secret
    └─ Patch MutatingWebhookConfiguration
    ↓
3. Webhook starts with valid certs
    ↓
4. Job deleted (hook-delete-policy)
```

### Security Considerations

**Self-signed mode:**
- CA valid for 10 years (reduced rotation overhead)
- Server cert valid for 1 year (balance security + operations)
- ECDSA P-256 (smaller keys, better performance than RSA)
- Private keys never leave the cluster

**cert-manager mode:**
- Leverages external PKI (Let's Encrypt, Venafi, etc.)
- Short-lived certificates (90 days typical)
- Automatic renewal before expiry
- Audit trail via cert-manager

## Files Referenced

### zen-platform
- `charts/zen-saas/charts/zen-lock-0.0.3-alpha.tgz`
  - `zen-lock/templates/webhook-cert-job.yaml`
  - `zen-lock/templates/webhook.yaml`
  - `zen-lock/values.yaml`

### zen-brain1
- `values/sandbox/zen-lock.yaml`
- `deploy/CLUSTER_RECOVERY_RUNBOOK.md`
- `deploy/zen-lock/RUNBOOK.md`

## Conclusion

zen-platform has solved the webhook certificate problem elegantly with a **zero-touch automated solution** using Helm hooks. zen-brain1 should adopt this approach to eliminate manual certificate management and enable webhooks by default.

The self-signed mode provides a good balance of security and operational simplicity for sandbox/development environments, while the cert-manager mode offers production-grade PKI for production deployments.

---

**Next Action:** Update zen-brain1 to use zen-platform's webhook certificate automation.
