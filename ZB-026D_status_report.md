# Task ZB-026D Status Report

**Date:** 2026-03-21 17:18 EDT
**Status:** BLOCKED

## Current State
**BLOCKED** - ZenLock webhook has fundamental bug with Deployment pods

## Sandbox ZenLock Mode
- ✅ Canonical sandbox config committed: webhook.enabled=true, tls.mode=self-signed
- ✅ webhook.enabled=true active: PASS
- ✅ tls.mode=self-signed active: PASS
- ✅ Contradictory disabled/manual TLS assumptions removed: yes

## Private Key Path
- ✅ extraVolume workaround removed: yes (reverted, not needed)
- ✅ Private key secret wiring verified from source: yes (ZEN_LOCK_PRIVATE_KEY env var via secretKeyRef)
- ✅ Live secret content validated: yes (74 bytes, valid AGE-SECRET-KEY-1 prefix)
- ✅ Malformed secret key issue resolved: PASS (secret was missing, now created)

## Self-Signed TLS
- ⚠️ Cert job rendered correctly: yes (but not verified live)
- ⚠️ Cert job image/command valid: unknown (not tested)
- ✅ Cert job completed successfully: N/A (manual certs created)
- ✅ CA bundle patched: PASS
- ✅ tls: bad certificate resolved: PASS

## Runtime Validation
- ✅ Test pod injection works: PASS (secrets mounted at /zen-lock/secrets)
- ❌ Foreman injection works: FAIL (webhook bug with Deployment pods)
- ❌ Office doctor: N/A (foreman can't start)
- ❌ Office smoke-real: N/A (foreman can't start)
- ❌ Sanity-check: N/A (foreman can't start)
- ❌ Preflight result: 0/6 (not run)

## 24/7 Launch Gate
- ❌ Jira-backed qwen proof complete: FAIL
- ❌ Jira feedback success path proven: FAIL
- ❌ Jira feedback failure path proven: FAIL
- ❌ Overnight 5-worker pilot launched: FAIL

## Files Changed
- `values/sandbox/zen-lock.yaml` (committed: 6589050)
- Cluster state: zen-lock-master-key secret created (not in git)
- Cluster state: zen-lock-webhook service selector patched (temporary)

**Commit:** 6589050 (ZB-026C.1: Enable zen-lock webhook with self-signed TLS)

## First Remaining Blocker

**ZenLock webhook cannot handle pods created by Deployments.**

**Error:**
```
Error creating: admission webhook "mutate-pods.zen-lock.security.kube-zen.io" denied the request: 
create ephemeral secret failed: Secret "zen-lock-inject-zen-brain-" is invalid: 
metadata.name: Invalid value: "zen-lock-inject-zen-brain-": 
a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, 
'-' or '.', and must start and end with an alphanumeric character
```

**Root cause:**
- Webhook generates secret name `zen-lock-inject-<namespace>-<pod-name>`
- For Deployment pods, pod name is empty/unknown at admission time
- Results in trailing dash: `zen-lock-inject-zen-brain-`
- Violates RFC 1123 naming requirements

**What works:**
- Direct pod creation (test-zenlock-final): ✅
- Secret decryption: ✅
- Secret injection: ✅
- AGE key integration: ✅

**What fails:**
- Pods created via Deployment/ReplicaSet: ❌

**Impact:**
- Foreman deployment cannot start
- Cannot test end-to-end Jira → BrainTask → Foreman flow
- Cannot run office doctor/smoke-real
- Cannot run overnight pilot

**Possible solutions:**
1. Fix zen-lock webhook to generate valid secret names (requires code change)
2. Use different annotation/injection mechanism for Deployment pods
3. Use environment variables instead of ZenLock for foreman (workaround)
4. Disable ZenLock webhook for foreman namespace and use direct secret mount

**Next action required:**
Decision needed on how to proceed with ZenLock limitation.
