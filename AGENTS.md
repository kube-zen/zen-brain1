# AGENTS.md - Zen-Brain Canonical Rules

**This file is the authoritative source of truth for all agents (human and AI) working on zen-brain.**

**Last Updated**: 2026-03-22
**Status**: LOCKED - Changes require explicit operator approval

---

## Jira Credential Rails

### Canonical Email Identity

**Canonical Jira Email**: `zen@kube-zen.io`

This email MUST be used for all Jira operations. Do NOT use:
- zen@zen-mesh.io (WRONG - causes 401 auth failures)
- Any other email variant

### Credential Source Hierarchy

1. **Cluster Runtime (Production)**:
   - Source: `/zen-lock/secrets/` (ZenLock injection)
   - Files: `JIRA_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`, `JIRA_PROJECT_KEY`
   - Email in secret MUST be `zen@kube-zen.io`

2. **Bootstrap/Rotation Only**:
   - `~/zen/DONOTASKMOREFORTHISSHIT.txt` (token file)
   - `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` (AGE private key)
   - `~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age` (AGE public key)
   - Script: `~/zen/zen-brain1/deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh`

3. **Local Dev Only** (when cluster not available):
   - Environment variables: `JIRA_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`
   - NEVER use in cluster mode

### Jira Validation Flow

**STEP 1: Authentication Check**
```
GET $JIRA_URL/rest/api/3/myself
Headers: Authorization: Basic base64($JIRA_EMAIL:$JIRA_API_TOKEN)

Interpretation:
- 200 => auth OK, email + token match
- 401 => auth FAIL (wrong email OR bad token)
- Other => investigate endpoint/tenant/network
```

**STEP 2: Project Access Check** (only after auth = 200)
```
GET $JIRA_URL/rest/api/3/project/$PROJECT_KEY
GET $JIRA_URL/rest/api/3/project/search

Interpretation:
- /myself 200 + /project/ZB 200 => project accessible
- /myself 200 + /project/ZB 404 => project missing or account lacks access
- Do NOT call this auth failure if /myself = 200
```

### Canonical Project Key

**Project Key**: `ZB` (defined in `deploy/zen-lock/jira-metadata.yaml`)

Legacy project keys (e.g., `SCRUM`) are DEPRECATED. Do NOT use.

---

## Forbidden Actions

### DO NOT

1. **Ask for credentials during normal work**
   - Credentials are in ZenLock or bootstrap files
   - Only ask during explicit rotation/bootstrap

2. **Use legacy secret paths**
   - `~/.zen-brain/secrets/jira.yaml` - FORBIDDEN
   - `~/.zen-lock/private-key.age` - FORBIDDEN
   - Random `.env.jira.local` files - FORBIDDEN

3. **Use wrong Jira email**
   - zen@zen-mesh.io - FORBIDDEN (causes 401)
   - Always verify email from live Foreman context

4. **Speculate about credentials without evidence**
   - Test from live Foreman pod first
   - Use exact decision tree defined above

5. **Mix auth and project validation**
   - Auth check = /myself endpoint
   - Project check = /project/{key} endpoint
   - They are separate concerns

6. **Use Jira UI for issue creation**
   - Use zen-brain Jira connector
   - Use Office abstraction layer
   - Do NOT use curl as product path

---

## Runtime Validation

### From Live Foreman Pod

```bash
# 1. Check credentials mounted
kubectl exec -n zen-brain deployment/foreman -- cat /zen-lock/secrets/JIRA_EMAIL
# Should output: zen@kube-zen.io

# 2. Test auth
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office doctor
# Should show:
#   Auth check: PASS
#   Project check: PASS (project ZB accessible)

# 3. Test API directly (debugging only)
kubectl exec -n zen-brain deployment/foreman -- /tmp/test-auth
# Tests both emails, shows which works
```

### Preflight Checks

**MUST fail if**:
- Jira auth fails from runtime context
- Project key ZB is inaccessible
- Runtime source is not ZenLock (in cluster mode)

**MUST log at startup**:
- Config source
- Jira credential source
- Jira URL/email present
- Jira project key
- Auth check result
- Project check result
- Tier1 Redis source
- Local model
- Timeout
- Keep-alive
- Stale threshold

---

## CI Guardrails

CI MUST fail if:

1. **AGENTS.md missing**
2. **Legacy secret paths in active docs/code**
   - `~/.zen-brain/secrets/jira.yaml`
   - `~/.zen-lock/private-key.age`
3. **Active docs/code claim Jira is optional in normal mode**
4. **Project key drift**
   - Multiple project keys in active paths
   - Legacy SCRUM references not marked as examples
5. **Local LLM timeout too short**
   - 0.8b path uses timeout < 2700s
6. **Wrong Jira email in metadata**
   - `deploy/zen-lock/jira-metadata.yaml` must have `email: zen@kube-zen.io`

---

## Documentation Requirements

### CURRENT_STATE.md

Must contain:
- Current exact blocker (or "none")
- Current proven state
- Next exact action

### Memory Files

- `CLAUDE.md` → points to `AGENTS.md`
- `MEMORY.md` → points to `AGENTS.md`
- All agents read `AGENTS.md` first

---

## Troubleshooting

### "Jira auth fails with 401"

1. **Check email from live Foreman pod**:
   ```bash
   kubectl exec -n zen-brain deployment/foreman -- cat /zen-lock/secrets/JIRA_EMAIL
   ```
   Must be `zen@kube-zen.io`

2. **If wrong email**:
   - Update `deploy/zen-lock/jira-metadata.yaml`
   - Run bootstrap: `~/zen/zen-brain1/deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh`
   - Restart Foreman

3. **If email correct but still 401**:
   - Token may be expired
   - Follow explicit rotation flow in credential rails

### "Project ZB not found"

1. **Verify auth passes first**:
   - Auth check MUST show PASS
   - Only then check project access

2. **If auth passes but project fails**:
   - Account lacks permissions
   - Project doesn't exist
   - Do NOT rotate token (token is valid)

---

## Project Key Rules

1. **Single Source of Truth**:
   - `deploy/zen-lock/jira-metadata.yaml`
   - Field: `jira.project_key: "ZB"`

2. **No Legacy Keys**:
   - SCRUM is DEPRECATED
   - Must not appear in active code/docs
   - Only allowed in examples marked as legacy

3. **Preflight Verification**:
   - Verifies project key accessible
   - Fails closed if not accessible

---

## Applicability

These rules apply to **all agents, human or AI**.

Violations are **bugs**.

Runtime, CI, and preflight are the **source of truth**, not memory.

---

## Change Process

To modify these rules:

1. Propose change to operator
2. Get explicit approval
3. Update this file
4. Update dependent systems (CI, preflight, docs)
5. Verify from live runtime context
6. Commit with message explaining change

**No other process is valid.**
