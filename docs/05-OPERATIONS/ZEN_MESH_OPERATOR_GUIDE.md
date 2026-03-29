> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.



**Version:** Zen-Brain 1.0
**Date:** 2026-03-13
**Mode:** Observer/Shadow Mode (Safe Read-Only)
**Status:** Operational

---

## Overview

Zen-Brain 1.0 runs in **observer/shadow mode** for Zen-Mesh operations, providing intelligent analysis and recommendations without autonomous code/deploys.

**Key Principles:**
- **Read-Only Observer:** Analyze, classify, recommend - do NOT autonomously change code/infrastructure
- **Safe Write-Back:** Jira comments, artifact attachments, status updates (only where proven safe)
- **Gated Actions:** Repo writes, merges, deploys, config changes require approval
- **Trusted Foundation:** Host Docker Ollama + real Jira on proven lane

---

## Known-Good Baseline (FROZEN)

### Environment Configuration
```bash
# Runtime Profile
export ZEN_RUNTIME_PROFILE=dev

# LLM Configuration
export OLLAMA_BASE_URL=http://127.0.0.1:11434

# Office Configuration
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1    # Stub KB for now
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1  # Stub ledger for now

# Jira Configuration (from ~/zen/.zen-brain1-config/jira.yaml)
export JIRA_URL=https://zen-mesh.atlassian.net
export JIRA_EMAIL=zen@zen-mesh.io
export JIRA_PROJECT_KEY=ZB

# Token Type: ATATT3... (user-level API token - works with Basic Auth)
# NOT ATCTT3... (workspace-level - does NOT work)
```

### Jira Credentials
```yaml
# Location: ~/zen/.zen-brain1-config/jira.yaml (NOT in repo)
enabled: true
base_url: "https://zen-mesh.atlassian.net"
email: "zen@zen-mesh.io"
token: "ATATT3..."  # user-level API token
default_project_key: "ZB"
```

### Proven Commands
```bash
# Health Checks
./bin/zen-brain runtime doctor
./bin/zen-brain office doctor
./bin/zen-brain runtime report

# Jira Operations
./bin/zen-brain office fetch ZB-XXX        # Fetch ticket details
./bin/zen-brain office search "status=Done"   # Search tickets

# Analysis
./bin/zen-brain analyze work-item ZB-XXX     # Analyze ticket
./bin/zen-brain vertical-slice ZB-XXX         # Execute task (shadow mode)
```

---

## Action Classes

### Class A: Always Allowed (Read & Recommend)
**Risk Level:** None - Safe to run autonomously

| Action | Command | Description |
|--------|----------|-------------|
| Fetch Jira Issue | `office fetch ZB-XXX` | Retrieve ticket details |
| Analyze Work Item | `analyze work-item ZB-XXX` | Analyze with Ollama |
| Summarize | Built into analysis | Generate summary |
| Classify | Built into analysis | Categorize issue type |
| Recommend | Built into analysis | Suggest actions |
| Generate Artifacts | `vertical-slice` (proof/work) | Create analysis output |

**Policy:** Execute freely, no approval needed.

---

### Class B: Safe Write-Back (Restricted Writes)
**Risk Level:** Low - Safe writes with proven paths only

| Action | Command | Restrictions |
|--------|----------|--------------|
| Jira Comments | `vertical-slice` (proof/work) | Only analysis/recommendations, no code/infra changes |
| Artifact Attachments | `vertical-slice` (proof/work) | Generated artifacts, no executable binaries |
| Status Updates | `vertical-slice` (proof/work) | Only safe transitions (e.g., "In Progress" → "In Review") |
| Limited Transitions | `vertical-slice` (proof/work) | No "Closed", "Resolved", "Blocked" (no operational impact) |

**Policy:** Execute with human review of output before Jira write-back.

---

### Class C: Approval Required (High-Impact Actions)
**Risk Level:** Medium-High - Require explicit approval

| Action | Command | Approval Required | Business Impact |
|--------|----------|------------------|------------------|
| Repo Writes | Manual git push | ✅ YES | Code changes |
| Merges | Manual git merge | ✅ YES | Code integration |
| Deploys | `make dev-up` | ✅ YES | Infrastructure change |
| Secret/Config Changes | Manual | ✅ YES | Credential exposure |
| Meaningful Status Changes | `vertical-slice` | ✅ YES | Operational impact |
| Workflow Transitions | `vertical-slice` | ✅ YES | Business process change |

**Policy:** Queue for human approval, document risk/benefit, await sign-off.

---

## Continuous Operational Loop

### Discovery Phase
```bash
# 1. Find open/candidate Zen-Mesh tickets
./bin/zen-brain office search "status IN (To Do, In Progress) AND assignee IS EMPTY"

# 2. Select eligible items
# Filter by:
#   - Project: ZB
#   - Type: Task/Story/Bug (not Epic)
#   - Priority: High/Medium
#   - No assignee (unclaimed)
```

### Analysis Phase
```bash
# 3. Fetch ticket details
./bin/zen-brain office fetch ZB-XXX

# 4. Analyze with real Ollama
./bin/zen-brain analyze work-item ZB-XXX

# 5. Generate recommendation/action summary
# Output includes:
#   - Summary of issue
#   - Estimated cost/effort
#   - Recommended actions (Class A/B/C)
#   - Required approval (if Class C)
```

### Action Phase
```bash
# 6a. Class A/B actions - execute immediately
./bin/zen-brain vertical-slice ZB-XXX

# 6b. Class C actions - queue for approval
# Document:
#   - Action type
#   - Risk level
#   - Business impact
#   - Approval required
#   - Implementation steps

# 7. Safe Jira write-back
# Only via:
#   - Comment: "Analysis complete - see attached proof"
#   - Attachment: proof-of-work.json, analysis.md
#   - Status: "In Review" (NOT "Closed"/"Resolved")
```

### Loop Frequency
- **Discovery:** Every 1-2 hours (check for new tickets)
- **Analysis:** On-demand (when tickets selected)
- **Action:** On-demand (after human review/approval)

---

## Proven Lane Guardrails

### Host Docker Ollama (Canonical)
```bash
# Verify Ollama is running on host
curl http://127.0.0.1:11434/api/version
# Expected: {"version":"0.17.6"}

# Verify zen-brain1 uses host path
grep OLLAMA_BASE_URL ~/.zen-brain/config.yaml
# Expected: http://host.k3d.internal:11434 (cluster) or http://127.0.0.1:11434 (local)
# NOT: http://ollama:11434 (in-cluster - EXPLICITLY DISABLED)
```

**Policy:** Do NOT switch to in-cluster Ollama without explicit approval + documented rollback plan.

---

### Real Jira Lane (Proven)
```bash
# Verify Jira connectivity
./bin/zen-brain office doctor
# Expected: "API reachability: ok"

# Verify ADF parsing works
./bin/zen-brain office fetch ZB-216
# Expected: Returns ticket details (no JSON decode errors)

# Verify end-to-end
./bin/zen-brain vertical-slice ZB-216
# Expected: Completes successfully, updates Jira
```

**Policy:** Do NOT modify Jira connector/types without regression testing.

---

### Office Mode Visibility (Explicit)
```bash
# Verify stub modes are explicit
./bin/zen-brain office doctor
# Expected:
#   knowledge_base: ✓ mode=stub enabled=true
#   ledger:         ✓ mode=stub enabled=true
#   (explicit opt-in flags set)
```

**Policy:** Do NOT enable real KB/qmd or real ledger without Phase 6 upgrade.

---

## Operator Workflows

### Workflow 1: Issue Analysis (Class A)
```bash
# Input: Jira ticket key
# Steps:
# 1. Fetch ticket details
# 2. Analyze with Ollama
# 3. Generate recommendations
# 4. Create proof-of-work artifacts
# 5. Update Jira with comment + attachments
# Approval: Not required (safe write-back)
```

### Workflow 2: Code Review Recommendation (Class B)
```bash
# Input: Jira pull request ticket
# Steps:
# 1. Fetch PR details
# 2. Analyze code changes
# 3. Generate review summary
# 4. Update Jira with findings
# Approval: Review comment before posting
```

### Workflow 3: Deployment Preparation (Class C)
```bash
# Input: Jira deployment ticket
# Steps:
# 1. Fetch ticket details
# 2. Analyze deployment plan
# 3. Generate deployment checklist
# 4. Update Jira with "Ready for Approval" status
# 5. Queue for human approval
# Approval: Human sign-off required before execution
```

---

## Safety Checks

### Pre-Action Checklist (Class A/B)
- [ ] Jira ticket is valid (exists, accessible)
- [ ] Ollama is responding (`curl http://127.0.0.1:11434/api/version`)
- [ ] Office doctor passes (`./bin/zen-brain office doctor`)
- [ ] Action class is A or B (not C)

### Pre-Action Checklist (Class C)
- [ ] All Class A/B checks pass
- [ ] Action type documented (risk/benefit)
- [ ] Business impact assessed
- [ ] Human approval obtained (written sign-off)
- [ ] Rollback plan documented (if deployment)

### Post-Action Verification
- [ ] Jira updated (comment/status)
- [ ] Artifacts attached (proof-of-work)
- [ ] No errors in execution log
- [ ] Next action identified (if needed)

---

## Incident Response

### If Real Jira Lane Fails
```bash
# 1. Check Jira connectivity
./bin/zen-brain office doctor

# 2. Check Ollama
curl http://127.0.0.1:11434/api/version

# 3. Check token validity
curl -u "zen@zen-mesh.io:ATATT3..." \
  -H "Accept: application/json" \
  "https://zen-mesh.atlassian.net/rest/api/3/myself"

# 4. Fallback to stub mode
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1

# 5. Document incident in Jira
./bin/zen-brain office fetch <incident-ticket>
./bin/zen-brain analyze work-item <incident-ticket>
# Update with incident report
```

### If Ollama Goes Down
```bash
# 1. Verify host Docker Ollama
docker ps | grep ollama

# 2. Restart if needed
docker restart ollama

# 3. Verify
curl http://127.0.0.1:11434/api/version

# 4. Notify in Jira
# Update running tickets with Ollama status
```

---

## Upgrade Path (Zen-Brain 1.1)

### Planned Upgrades (24/7+)

**Upgrade 1: Real KB/qmd Integration**
- Add real knowledge base access
- Enable QMD document retrieval
- Provide Zen-Mesh context from KB

**Upgrade 2: Approval/RBAC Strengthening**
- Implement approval workflow in zen-brain1
- Add role-based access control
- Require sign-off for Class C actions

**Upgrade 3: Real Ledger/Auditing**
- Enable real CockroachDB ledger
- Track all zen-brain1 operations
- Add audit trail for Class C actions

**Policy:** One upgrade at a time, validate between upgrades.

---

## Success Criteria

### Minimum Success (24/7 Operations)
- ✅ Zen-Brain 1.0 continuously processes real Zen-Mesh Jira items
- ✅ Analysis/comments/artifacts are useful and repeatable
- ✅ No regressions on proven lane (Ollama, Jira, vertical-slice)
- ✅ Risky actions (Class C) remain gated

### Strong Success (24/7 Operations)
- ✅ Stable shadow mode for Zen-Mesh (high confidence safe write actions)
- ✅ High-confidence Jira write-back (comments, attachments, status updates)
- ✅ Clear backlog for Zen-Brain 1.1 upgrades (based on real usage)
- ✅ Minimal human intervention needed for Class A/B actions

---

## Quick Reference

### Operator Commands
```bash
# Daily operations
./bin/zen-brain office doctor
./bin/zen-brain office search "status IN (To Do, In Progress)"
./bin/zen-brain analyze work-item ZB-XXX
./bin/zen-brain vertical-slice ZB-XXX

# Health checks
curl http://127.0.0.1:11434/api/version
docker ps | grep ollama
```

### Key Files
```
Config:          ~/zen/.zen-brain1-config/jira.yaml (NOT in repo)
Runtime:         ~/.zen-brain/
Artifacts:       ~/.zen-brain/runtime/proof-of-work/
Analysis:        ~/.zen-brain/analysis/
```

### Known-Good Commit
```
Commit: 0812d0e - fix: Handle Jira API v3 ADF description format
Date:   2026-03-13
Status: ✅ Real Jira lane working
```

---

**Mode:** 24/7 Operations (Observer/Shadow Mode)
**Status:** Operational
**Next Upgrade:** Zen-Brain 1.1 (real KB/qmd, RBAC, ledger/audit)
