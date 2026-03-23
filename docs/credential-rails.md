# Credential Rails - Canonical Runbook

## Normal Credential Model

**Long-lived local bootstrap material:**
- `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age`
- `~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age`

**Encrypted artifact:**
- `deploy/zen-lock/jira-credentials.zenlock.yaml`

**Runtime source ONLY:**
- `/zen-lock/secrets` (ZenLock injection)

## One-Time Bootstrap or Token Rotation

**Plaintext token file (ONE-TIME USE ONLY):**
- `~/zen/DONOTASKMOREFORTHISSHIT.txt`
- Used ONLY during bootstrap or explicit token rotation
- MUST be deleted after successful bootstrap
- NOT part of normal recurring workflow

### Bootstrap Steps

```bash
# 1. Ensure AGE keypair exists
ls -la ~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age
ls -la ~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age

# 2. Place new token in bootstrap file (rotation only)
# This file should already exist from initial setup
cat ~/zen/DONOTASKMOREFORTHISSHIT.txt

# 3. Run bootstrap script
cd ~/zen/zen-brain1
./deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh

# 4. Verify Jira auth works
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office doctor
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office smoke-real

# 5. Verify plaintext token was deleted
ls -la ~/zen/DONOTASKMOREFORTHISSHIT.txt
# Should NOT exist after successful bootstrap
```

### Verification Steps

```bash
# Check ZenLock health
kubectl -n zen-lock-system get pods

# Check Foreman has ZenLock mounted
kubectl exec -n zen-brain deployment/foreman -- ls -la /zen-lock/secrets/

# Check Jira credentials present
kubectl exec -n zen-brain deployment/foreman -- cat /zen-lock/secrets/JIRA_URL
kubectl exec -n zen-brain deployment/foreman -- cat /zen-lock/secrets/JIRA_EMAIL
kubectl exec -n zen-brain deployment/foreman -- sh -c 'wc -c /zen-lock/secrets/JIRA_API_TOKEN'

# Test Jira auth
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office doctor
```

## DO / DO NOT

### DO

- Use ZenLock for all cluster credentials
- Run bootstrap script when setting up Jira or rotating tokens
- Delete plaintext token after successful bootstrap
- Verify Jira auth after bootstrap
- Check Foreman startup logs for credential source
- Monitor preflight checks for ZenLock health

### DO NOT

- Use `~/.zen-brain/secrets/jira.yaml` as active path in cluster
- Use `~/.zen-lock/private-key.age` as active path in cluster
- Create random `.env.jira.local` files as secret source
- Paste tokens in chat
- Accept runtime plaintext/env fallback in cluster mode
- Create alternate bootstrap stories
- Keep plaintext token file after successful bootstrap
- Ask for plaintext token during normal operations

## Runtime Guardrails

### Cluster Mode (Strict)

- ZenLock MUST be healthy
- Credentials MUST come from `/zen-lock/secrets`
- Plaintext token file MUST NOT exist after bootstrap
- Non-ZenLock credential sources are FORBIDDEN
- Violations result in hard failure with clear error messages

### Local Dev Mode (Relaxed)

- May use `~/.zen-brain/secrets/jira.yaml` for debugging
- ZenLock not required
- Env fallback allowed with explicit opt-in

## Enforcement

### Runtime

- Config loader fails closed in cluster mode
- Error messages point to bootstrap script and required files
- No silent fallback to alternate credential sources

### Preflight

- ZenLock health check
- Jira credential availability check
- Jira project key accessibility check
- Plaintext token file absence check
- Local model configuration check

### CI (Future)

- Fail if legacy Jira paths in active code/docs
- Fail if secret creation uses `--from-literal` for key.txt
- Fail if token-like strings in repo
- Fail if cluster runtime accepts non-ZenLock Jira creds
- Fail if 0.8b local lane timeout < 2700s
- Fail if stale threshold <= 45m
- Fail if project key drift detected (multiple keys in active paths)

## Troubleshooting

### "Jira credentials not loaded from ZenLock"

**Symptom:** Foreman logs show credentials not found

**Solution:**
1. Check ZenLock is healthy: `kubectl -n zen-lock-system get pods`
2. Check ZenLock secret exists: `kubectl -n zen-lock-system get secrets`
3. Check Foreman has ZenLock annotation: `kubectl get deployment foreman -n zen-brain -o yaml | grep zen-lock`
4. Run bootstrap script if needed

### "Plaintext bootstrap file still exists after bootstrap"

**Symptom:** Preflight check fails

**Solution:**
1. Verify bootstrap completed successfully
2. Delete plaintext file manually: `rm ~/zen/DONOTASKMOREFORTHISSHIT.txt`
3. Re-run preflight checks

### "credentials_file is forbidden in cluster mode"

**Symptom:** Config loader rejects credentials_file

**Solution:**
1. Remove credentials_file from config
2. Ensure ZenLock is configured correctly
3. Run bootstrap script if ZenLock not set up

## Security Model

**Normal Operation:**
1. Credentials encrypted with AGE keypair
2. ZenLock stores encrypted credentials
3. ZenLock webhook injects credentials into pods
4. Foreman reads from `/zen-lock/secrets`
5. NO plaintext credentials in cluster

**Bootstrap/Rotation:**
1. Operator places new token in plaintext file
2. Bootstrap script encrypts and updates ZenLock
3. Script verifies Jira auth works
4. Script deletes plaintext file
5. Normal operation resumes

**Audit Trail:**
- Bootstrap script logs all actions
- Foreman logs credential source at startup
- Preflight checks verify ZenLock health
- CI gates prevent regression

## Applicability

These rules apply to **all developers, human or AI**.

Violating them is a **bug**.

Runtime, CI, and preflight are the **source of truth**, not memory.
h works
4. Script deletes plaintext file
5. Normal operation resumes

**Audit Trail:**
- Bootstrap script logs all actions
- Foreman logs credential source at startup
- Preflight checks verify ZenLock health
- CI gates prevent regression

## Applicability

These rules apply to **all developers, human or AI**.

Violating them is a **bug**.

Runtime, CI, and preflight are the **source of truth**, not memory.
