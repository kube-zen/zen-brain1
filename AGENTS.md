# AGENTS.md - Zen-Brain Canonical Rules

**This file is the authoritative source of truth for all agents (human and AI) working on zen-brain.**

**Last Updated**: 2026-04-02
**Status**: LOCKED - Changes require explicit operator approval
**Credential Rails**: Version 3.0 Enforced (Layers 1-2 Complete)

---

## For qwen3.5:0.8b: Generic Role Prompts Are Insufficient

**CRITICAL:** Small models like qwen3.5:0.8b need structured task packets, not generic "You are a planner/worker" prompts.

**Every execution MUST use a structured task packet that includes:**

### Required Components
1. **Task Identity**
   - Jira key
   - Summary
   - Work type
   - Timeout

2. **Scope**
   - Allowed paths (what CAN be modified)
   - Forbidden paths (what MUST NOT be touched)
   - Context files (what to read FIRST)
   - Target files (what will be modified)

3. **Architecture Constraints**
   - Existing types/interfaces to use
   - Existing packages to import
   - Wiring points (where integration belongs)
   - Do not modify list

4. **Phased Execution**
   - Phase 1, 2, 3, etc.
   - Each with:
     - Requirements (WHAT)
     - Expected Behavior (HOW)
     - Verification (TEST)

5. **Verification Commands**
   - Compile: `go build ./...`
   - Tests: `go test ./...`
   - Static checks: `grep`, assertions

6. **Output Contract**
   - Exact files changed
   - Verification output
   - Result: SUCCESS | FAILURE
   - Blockers reported honestly

### Forbidden Actions
- Do NOT invent new packages/imports not in existing list
- Do NOT create fake artifacts or placeholder code
- Do NOT modify files outside allowed paths
- Do NOT claim success if compile/test fails
- If a required type is missing, report blocker instead of hallucinating

### For Rescue Tasks (0.1 → 1.0)
Always provide:
- 0.1 source file(s) to read
- 1.0 target file(s) to read
- Ask for bounded adaptation, not freeform rewrite
- Use template: `config/task-templates/rescue_from_01.yaml`

### Prompt Builder Location
- Code: `internal/promptbuilder/packet.go`
- Template: `config/task-templates/rescue_from_01.yaml`
- Function: `BuildPrompt(packet TaskPacket) (string, error)`

**DO NOT** use generic planner/worker prompts for rescue tasks without template/context injection.

See `docs/PROMPT_ENGINEERING_MIGRATION.md` for full details.

---

## CANONICAL JIRA IDENTITY — DO NOT ASK AGAIN

**Canonical Jira Email**: `zen@zen-mesh.io`
**Canonical Jira URL**: `https://zen-mesh.atlassian.net`
**Canonical Project Key**: `ZB`

These are final. No future operator or AI should ever ask which Jira email to use.
If the live secret and this doc are present, follow them. Do not ask the user.

### Forbidden
- `zen@zen-mesh.io` — **WRONG, causes 401 auth failures** — FORBIDDEN
- Any other email variant — FORBIDDEN

### Runtime Credential Source (ONLY path)
All runtime Jira credentials come from ZenLock-injected secrets:
```
/zen-lock/secrets/JIRA_URL          → https://zen-mesh.atlassian.net
/zen-lock/secrets/JIRA_EMAIL        → zen@zen-mesh.io
/zen-lock/secrets/JIRA_API_TOKEN    → (API token, not logged)
/zen-lock/secrets/JIRA_PROJECT_KEY  → ZB
```

### Bootstrap / Rotation (ONLY for token changes)
1. Obtain new API token from Atlassian
2. Place in `~/zen/DONOTASKMOREFORTHISSHIT.txt` (ONE-TIME USE ONLY)
3. Run `deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh`
4. Verify: `zen-brain office doctor` or `zen-brain office smoke-real`
5. **Delete** `~/zen/DONOTASKMOREFORTHISSHIT.txt`
6. Runtime uses only ZenLock sources after this

### On 401 / Expired Token
1. Auth preflight blocks immediately (admission gate)
2. Mark run as `blocked:jira-auth`
3. Do NOT dispatch Jira-backed AI work
4. Follow bootstrap/rotation procedure above to get fresh token

### Proof Command (verify from cluster)
```bash
# From live pod - check file exists (safe, no secret value printed)
kubectl exec -n zen-brain deployment/foreman -- test -f /zen-lock/secrets/JIRA_EMAIL && echo "JIRA_EMAIL present"

# Check capability matrix (safe - shows source, not value)
kubectl logs -n zen-brain deployment/foreman | grep -i "jira.*loaded from"
# Expected: "[JIRA] ✅ Credentials loaded from zenlock-dir:/zen-lock/secrets"

kubectl exec -n zen-brain deployment/foreman -- zen-brain office doctor
# Expected: Jira auth OK
```

### Proof Command (local, with token)
```bash
# Use canonical resolver test (safe - no secret printed)
./bin/zen-brain office doctor
# Shows: credential source, email, project key (NOT token value)
```

### Mandatory Preflight Before Any Jira-Backed Run
```bash
JIRA_TOKEN=<token> MODE=preflight STRICT=true ./cmd/admission-gate/admission-gate
# If exit != 0, DO NOT proceed with Jira work
```

---

## Jira Credential Rails

### Canonical Email Identity

**Canonical Jira Email**: `zen@zen-mesh.io`

This email MUST be used for all Jira operations. Do NOT use:
- zen@zen-mesh.io (WRONG - causes 401 auth failures)
- Any other email variant

### Credential Source Hierarchy

1. **Cluster Runtime (Production)**:
   - Source: `/zen-lock/secrets/` (ZenLock injection)
   - Files: `JIRA_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`, `JIRA_PROJECT_KEY`
   - Email in secret MUST be `zen@zen-mesh.io`

2. **Bootstrap/Rotation Only**:
   - `~/zen/DONOTASKMOREFORTHISSHIT.txt` (token file, ephemeral)
   - `~/zen/keys/zen-brain/credentials.key` (AGE private key, canonical)
   - `~/zen/keys/zen-brain/credentials.pub` (AGE public key, canonical)
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
   - `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` - Legacy, use `~/zen/keys/zen-brain/credentials.key`

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
# 1. Check credentials mounted (safe - existence only, no value)
kubectl exec -n zen-brain deployment/foreman -- test -f /zen-lock/secrets/JIRA_EMAIL && echo "JIRA_EMAIL: present"
kubectl exec -n zen-brain deployment/foreman -- test -f /zen-lock/secrets/JIRA_API_TOKEN && echo "JIRA_API_TOKEN: present"

# 2. Test auth (safe - capability matrix, no secret values)
kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office doctor
# Should show:
#   Auth check: PASS
#   Project check: PASS (project ZB accessible)
#   Credential source: zenlock-dir:/zen-lock/secrets

# 3. Check startup logs (safe - shows source path, not values)
kubectl logs -n zen-brain deployment/foreman | grep -i "jira.*loaded"
# Expected: "[JIRA] ✅ Credentials loaded from zenlock-dir:/zen-lock/secrets"
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

### Default Suite (17 gates)

Run all gates:
```bash
python3 scripts/ci/run.py
```

### Credential Rails Suite (5 gates)

Run credential-specific gates:
```bash
python3 scripts/ci/run.py --suite credentials
```

| Gate | Purpose |
|------|---------|
| `canonical_credential_access_gate.py` | Block raw credential access outside allowlist |
| `no_secret_echo_gate.py` | Block secret exposure patterns |
| `no_alt_credential_rails_gate.py` | Block alternate credential files/paths |
| `zenlock_mount_only_gate.py` | Enforce ZenLock mount-only in K8s manifests |
| `docs_drift_credential_rails_gate.py` | Ensure docs consistency |

### All CI Gates

CI MUST fail if:

1. **AGENTS.md missing**
2. **Legacy secret paths in active docs/code**
   - `~/.zen-brain/secrets/jira.yaml`
   - `~/.zen-lock/private-key.age`
   - `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` (use canonical key)
3. **Active docs/code claim Jira is optional in normal mode**
4. **Project key drift**
   - Multiple project keys in active paths
   - Legacy SCRUM references not marked as examples
5. **Local LLM timeout too short**
   - 0.8b path uses timeout < 2700s
6. **Wrong Jira email in metadata**
   - `deploy/zen-lock/jira-metadata.yaml` must have `email: zen@zen-mesh.io`
7. **Direct env access outside resolver**
   - Raw env reads for credentials in non-allowlisted files
8. **K8s manifest violations**
   - `envFrom: secretRef` for Jira credentials
   - `zen-lock/inject-env: "true"` for Jira/Git

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

1. **Check email from capability matrix (safe - no secret printed)**:
   ```bash
   kubectl logs -n zen-brain deployment/foreman | grep -i "jira email"
   ```
   Must show `zen@zen-mesh.io`

   Or check mounted file exists (existence only):
   ```bash
   kubectl exec -n zen-brain deployment/foreman -- test -f /zen-lock/secrets/JIRA_EMAIL && echo "JIRA_EMAIL: present"
   ```

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
