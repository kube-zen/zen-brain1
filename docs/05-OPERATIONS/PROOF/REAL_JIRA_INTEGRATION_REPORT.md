> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.



> ⚠️ **HISTORICAL NOTE**: This document contains commands using `zen@zen-mesh.io`, which is the
> **WRONG/FORBIDDEN** Jira email. The canonical email is `zen@kube-zen.io` (see AGENTS.md).
> These commands were run before the email was corrected. Do not reuse them as-is.

**Date:** 2026-03-13
**Status:** ✅ PARTIAL - Jira Authentication WORKS, JSON Parsing Issue Remains

---

## Executive Summary

Successfully identified working Jira authentication method for Zen-Brain 1.0. Jira connectivity is now **functional** with proper token type. Remaining blocker: JSON parsing error for Jira API v3 ADF (Atlassian Document Format) description field.

**Status:**
- Host Docker Ollama path: ✅ Still working
- Stub KB: ✅ Still working
- Stub Ledger: ✅ Still working
- Real Jira: ✅ **AUTHENTICATION WORKS** - JSON parsing issue (ADF format)

---

## Phase 1: Validate Jira Connectivity

### Attempt 1 - Workspace Token (ATCTT3...) - FAILED
```bash
export JIRA_URL=https://zen-mesh.atlassian.net
export JIRA_EMAIL=zen@zen-mesh.io
export JIRA_TOKEN=ATCTT3...

./bin/zen-brain office doctor
```

### Result 1
```
API reachability: failed (jira resource not found (404) at /rest/api/3/project/ZB)
```

**Analysis:** Workspace token (ATCTT3...) does NOT work with Jira REST API v3 Basic Auth.

---

### Attempt 2 - User Token with Typo - FAILED
```bash
export JIRA_TOKEN=REDACTED_JIRA_TOKENyF8F4YJ... (with extra "e" in token)
```

### Result 2
```
API reachability: failed (404)
```

**Analysis:** Typo in token caused failure.

---

### Attempt 3 - User Token (ATATT3...) - SUCCESS ✅
```bash
export JIRA_EMAIL=zen@zen-mesh.io
export JIRA_TOKEN=ATATT3...
export JIRA_PROJECT_KEY=ZB

./bin/zen-brain office doctor
```

### Result 3
```
API reachability: ok
```

**Analysis:** User-level API token (ATATT3...) works with Basic Auth! Authentication successful.

---

### Attempt 4 - Fetch Ticket - FAILED (JSON Parsing)
```bash
./bin/zen-brain office fetch ZB-216
```

### Result 4
```
Fetch failed: failed to decode Jira issue: json: cannot unmarshal object into Go struct field .fields.description of type string
```

**Analysis:** Jira API v3 returns description as ADF object, but Go code expects string.

---

## Token Type Analysis

### Critical Discovery: Token Prefix Matters

| Token Prefix | Type | Basic Auth Support | Used By |
|-------------|------|------------------|----------|
| **ATATT3...** | **API Token** (user-level) | ✅ YES | ✅ zen-brain, zen-brain1 (NOW WORKING) |
| **ATCTT3...** | **Connect Token** (workspace-level) | ❌ NO | ❌ zen-brain1 (NOT WORKING) |

### Working Token (ATATT3...)
```
Token: ATATT3...
Email: zen@zen-mesh.io
Project key: ZB
```

### Non-Working Token (ATCTT3...)
```
Token: ATCTT3...
```

### Why zen-brain Works
zen-brain uses ATATT3... token (not ATCTT3...):
```
~/.zen/zen-brain/data/config/jira.yaml:
token: <zen-brain_token>
```

---

## Root Cause Analysis

### Issue 1: Workspace Token Incompatibility (RESOLVED)
**Cause:** ATCTT3... tokens are Connect tokens designed for OAuth, not Jira REST API v3 Basic Auth.

**Solution:** Use ATATT3... user-level API tokens.

**Status:** ✅ RESOLVED

---

### Issue 2: JSON Parsing Error (OPEN)
**Cause:** Jira API v3 returns description as ADF object:
```json
"description": {
  "type": "doc",
  "version": 1,
  "content": [...]
}
```

**But Go code expects:**
```go
Description string `json:"description"`
```

**Solution Required:** Update Go struct to handle ADF format:
```go
Description map[string]interface{} `json:"description"`
// OR use proper ADF struct
```

**Status:** ❌ OPEN - Blocks `office fetch` command

---

## What We Did NOT Change

### Preserved State
- ✅ Host Docker Ollama still working (http://127.0.0.1:11434)
- ✅ Stub KB still enabled (ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1)
- ✅ Stub Ledger still enabled (ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1)
- ✅ No Ollama deployment model changes (host Docker = canonical)

### NOT Attempted (Due to JSON Parsing Issue)
- ❌ Did NOT test `office search` (depends on fetch)
- ❌ Did NOT test `office create` (depends on fetch)
- ❌ Did NOT test `office update` (depends on fetch)
- ❌ Did NOT run `vertical-slice` with real Jira

---

## Proven Lane State

### What Still Works
```
Host Docker Ollama: ✅ http://127.0.0.1:11434
Stub KB: ✅ ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
Stub Ledger: ✅ ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
Jira Auth: ✅ ATATT3... token works with Basic Auth
Vertical-slice --mock: ✅ Working
make dev-up: ✅ Working
```

### Proven Lane Health Check
```bash
./scripts/check-proven-lane.sh
```
Expected: All checks PASS

---

## Requirements Status

| Step | Status | Notes |
|-------|--------|-------|
| Step 1: Validate Jira connectivity | ✅ PASS | ATATT3... token works |
| Step 2: Fetch real ticket | ❌ BLOCKED | JSON parsing error (ADF format) |
| Step 3: Analyze real ticket | ⏸️ SKIPPED | Depends on fetch |
| Step 4: Run real vertical-slice | ⏸️ SKIPPED | Depends on fetch/analyze |
| Step 5: Sandbox confirmation | ⏸️ SKIPPED | Cannot confirm until vertical-slice works |

---

## Honest Blockers

### Primary Blocker (OPEN)
**Jira API v3 ADF JSON Parsing Error** - Go struct expects `description` as `string`, but API returns ADF object.

**Impact:**
- ❌ Cannot fetch tickets (`office fetch`)
- ❌ Cannot search tickets (`office search`)
- ❌ Cannot create tickets (`office create`)
- ❌ Cannot update tickets (`office update`)
- ❌ Cannot run `vertical-slice` with real Jira

**What Works:**
- ✅ Authentication (Basic Auth with ATATT3... token)
- ✅ Office doctor (`API reachability: ok`)
- ✅ Basic API connectivity

---

## Next Steps (To Unblock)

### Required: Fix JSON Parsing
1. **Update Go struct** in Jira client:
   ```go
   // Current (broken):
   Description string `json:"description"`

   // Fixed:
   Description map[string]interface{} `json:"description"`
   // OR:
   Description struct {
       Type    string                   `json:"type"`
       Version int                      `json:"version"`
       Content []map[string]interface{} `json:"content"`
   } `json:"description"`
   ```

2. **Test `office fetch ZB-216`** - should work now

3. **Test `office search`** - validate JQL queries

4. **Test `vertical-slice`** - end-to-end with real Jira

### Alternative: Use Jira API v2
API v2 might return description as string (not ADF). Try:
```bash
curl -u "email:ATATT3..." \
  "https://zen-mesh.atlassian.net/rest/api/2/issue/ZB-216"
```

---

## Configuration

### Working Jira Config (Stored Outside Repo)
```yaml
# ~/zen/.zen-brain1-config/jira.yaml
enabled: true
base_url: "https://zen-mesh.atlassian.net"
email: "zen@zen-mesh.io"
token: "ATATT3..."
default_project_key: "ZB"
timeout_seconds: 30
```

### Environment Variables (For Testing)
```bash
export ZEN_RUNTIME_PROFILE=dev
export OLLAMA_BASE_URL=http://127.0.0.1:11434
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
export JIRA_URL=https://zen-mesh.atlassian.net
export JIRA_EMAIL=zen@zen-mesh.io
export JIRA_TOKEN=ATATT3...
export JIRA_PROJECT_KEY=ZB
```

---

## Commands Run Summary

### Validation Commands
```bash
# Office doctor (working)
./bin/zen-brain office doctor
# Result: API reachability: ok

# Fetch ticket (broken - JSON parsing)
./bin/zen-brain office fetch ZB-216
# Result: failed to decode Jira issue: json: cannot unmarshal object into Go struct field .fields.description of type string

# Manual curl validation (works)
curl -u "zen@zen-mesh.io:ATATT3..." \
  -H "Accept: application/json" \
  "https://zen-mesh.atlassian.net/rest/api/3/issue/ZB-216"
# Result: Returns full ticket with ADF description
```

---

## Summary

**Status:** ✅ PARTIAL - Authentication WORKS, JSON Parsing Issue Remains

**What Solved:**
- ✅ Jira authentication (ATATT3... user-level token)
- ✅ API connectivity (office doctor passes)
- ✅ Token type identification (ATATT3 vs ATCTT3)

**What Remains:**
- ❌ JSON parsing error (ADF format for description field)
- ❌ Cannot fetch/search/create/update tickets
- ❌ Cannot run vertical-slice with real Jira

**Next Step:** Fix Go struct to handle Jira ADF format.

---

**Note:** Token is stored in `~/zen/.zen-brain1-config/jira.yaml` (OUTSIDE repository for security).
