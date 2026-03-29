# Tonight: Zen-Brain Self-Improvement Proof Test

## Goal

Prove real Jira-driven self-improvement loop works on a tiny queue.

## Rules

- **One worker only** (zb-self-improvement-1)
- **One task at a time** (no parallelism)
- **Only 3-5 real Zen-Brain tickets**
- **Class A or safe B only** (no Class C)
- **No deploys**
- **No repo-write autonomy**
- **No secrets/cloud changes**
- **No compliance worker yet** (save for tomorrow)

---

## Tickets to Create (3-5)

Use these exact ticket specs in Jira (project: ZB):

### ZB-001: Improve runtime doctor clarity for failures/warnings
**Title:** Improve runtime doctor clarity for failures/warnings
**Labels:** zen-brain-nightshift
**Priority:** Low
**Type:** Improvement
**Description:**
```
The runtime doctor output should be clearer when failures or warnings occur.

Current state:
- Error messages are generic
- No clear remediation steps
- Output is hard to scan

Desired state:
- Clear error categories
- Actionable remediation steps
- Table-formatted output for readability
- Summary section at top

Focus areas:
- internal/runtime/doctor.go
- internal/runtime/bootstrap.go
- Add clearer section headers
- Include actionable next steps
- Format output as table for easier scanning

Action Class: A (always allowed - read/analyze/recommend)
```

---

### ZB-002: Improve Jira proof/comment formatting
**Title:** Improve Jira proof/comment formatting
**Labels:** zen-brain-nightshift
**Priority:** Low
**Type:** Improvement
**Description:**
```
Jira proof and comment formatting should be more consistent and readable.

Current state:
- Inconsistent proof format across workers
- Comments lack clear structure
- Hard to distinguish between different output types

Desired state:
- Standard proof-of-work.json schema
- Clear comment structure with sections
- Consistent formatting across all workers
- Easy to parse for automation

Focus areas:
- internal/factory/proof_formatter.go
- Define standard proof schema
- Add comment formatting helpers
- Ensure consistent usage across workers

Action Class: B (safe write-back - Jira comments/artifacts only)
```

---

### ZB-003: Improve nightly summary report structure
**Title:** Improve nightly summary report structure
**Labels:** zen-brain-nightshift
**Priority:** Low
**Type:** Improvement
**Description:**
```
Nightly summary reports should be more structured and actionable.

Current state:
- Reports vary in format
- No clear priority order
- Hard to see what matters most

Desired state:
- Standard report structure
- Prioritized sections (Critical > Important > Nice-to-have)
- Concise summaries
- Clear next steps

Focus areas:
- internal/runtime/report.go
- cmd/zen-brain/self_improvement.go
- cmd/zen-brain/compliance.go
- Standardize report format
- Add priority ordering
- Include actionable next steps

Action Class: A (always allowed - read/analyze/recommend)
```

---

### ZB-004: Add regression test for Jira ADF parsing
**Title:** Add regression test for Jira ADF parsing
**Labels:** zen-brain-nightshift
**Priority:** Low
**Type:** Task
**Description:**
```
Add regression test coverage for Jira ADF parsing after the v3 API fix.

Context:
- Fixed ADF parsing issue in commit 0812d0e
- Need to ensure fix doesn't regress
- Should test various ADF formats from real Jira data

Desired test:
- Test parsing of simple ADF text
- Test parsing of complex ADF objects
- Test malformed ADF handling
- Test nil/empty ADF handling
- Use real Jira ADF examples where possible

Location:
- internal/office/jira/connector_test.go or new test file

Action Class: B (safe write-back - add test file only)
```

---

### ZB-005: Hunt low-risk hardcoded/default-path issue
**Title:** Hunt low-risk hardcoded/default-path issue
**Labels:** zen-brain-nightshift
**Priority:** Low
**Type:** Task
**Description:**
```
Find and document one low-risk hardcoded value or default path issue.

Scope:
- Search for hardcoded paths, ports, URLs
- Look for TODO/FIXME comments related to hardcoding
- Focus on non-critical, low-risk areas (not security-related)

Desired outcome:
- Identify one specific hardcoded/default-path issue
- Document why it should be configurable
- Suggest configuration approach (env var, config file, CLI flag)
- If low-risk, create a small patch plan

Action Class: A (always allowed - read/analyze/recommend)
```

---

## How to Create These Tickets

1. Open Jira: https://zen-mesh.atlassian.net/projects/ZB
2. Create 3-5 new tickets using the specs above
3. Add label: `zen-brain-nightshift`
4. Set priority: Low
5. Set assignee: (leave unassigned - let worker claim)

---

## Tonight's Run Plan

### Step 1: Verify Jira Integration

```bash
cd /home/neves/zen/zen-brain1
./bin/zen-brain office doctor
```

### Step 2: Verify Discovery Query Works

```bash
cd /home/neves/zen/zen-brain1
export JIRA_URL=https://zen-mesh.atlassian.net
export JIRA_EMAIL=zen@zen-mesh.io
export JIRA_TOKEN=$(grep "^token:" ~/.zen-brain1-config/jira.yaml | awk '{print $2}' | tr -d '"')
./bin/zen-brain office search 'project = ZB AND labels = "zen-brain-nightshift"'
```

Expected output: 3-5 tickets found

### Step 3: Run Self-Improvement Loop

```bash
cd /home/neves/zen/zen-brain1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
./bin/zen-brain self-improvement > /tmp/zen-brain-nightshift-$(date +%Y%m%d).log 2>&1
```

### Step 4: Check Results in Morning

```bash
# Check morning report
cat /tmp/zen-brain-nightshift-$(date +%Y%m%d).log | grep -A 20 "=== Morning Report ==="

# Check for any errors
grep -i error /tmp/zen-brain-nightshift-$(date +%Y%m%d).log

# Check Jira for comments/artifacts
# Open each ticket and review:
# - Worker comments (should include worker identity)
# - Action class (should be A or B only)
# - Artifacts uploaded (proof-of-work.json, reports)
```

---

## Success Criteria

### 1. Real Jira Discovery Works
- ✅ Finds the 3-5 nightshift tickets
- ✅ Does not find other tickets (query is correct)
- ✅ Filters out already-claimed tickets

### 2. Claim/Lease Prevents Duplicates
- ✅ Worker claims one ticket at a time
- ✅ No duplicate processing of same ticket
- ✅ Worker identity visible in logs/comments

### 3. Useful Jira Comments/Artifacts Produced
- ✅ Comments include worker identity (zb-self-improvement-1)
- ✅ Comments include action class (A or B)
- ✅ Recommendations are actionable
- ✅ Proof artifacts are attached (proof-of-work.json)

### 4. Morning Report is Concise and Actionable
- ✅ Tasks discovered/claimed/processed/escalated
- ✅ Successfully processed list with titles
- ✅ Escalated tasks list (should be 0)
- ✅ Duration and worker identity
- ✅ No verbose junk

### 5. No Noisy or Unsafe Behavior
- ✅ No Class C tasks processed (all skipped/escalated)
- ✅ No deploys attempted
- ✅ No repo writes (read-only or safe test additions only)
- ✅ No secrets/cloud changes
- ✅ No noisy junk comments

---

## Tomorrow: Decide Next Steps

### If Overnight Run is Successful

Add compliance lane:
```bash
# Create compliance tickets (label: compliance-reporting)
./bin/zen-brain compliance reporter
./bin/zen-brain compliance gap-hunter
```

### If Overnight Run is Noisy/Brittle

Fix before expanding:
- Discovery query too broad
- Task selection logic
- Report quality
- Claim/lease behavior

---

## What NOT To Do Tonight

❌ Do NOT create 15 tickets (start with 3-5)
❌ Do NOT run compliance workers yet
❌ Do NOT run on Zen-Mesh tickets
❌ Do NOT use two workers
❌ Do NOT run on two PCs
❌ Do NOT enable repo-write autonomy
❌ Do NOT allow deploys
❌ Do NOT touch secrets/cloud/config

---

## Overnight Monitoring (Optional)

If you want to monitor progress:

```bash
# Tail the log file
tail -f /tmp/zen-brain-nightshift-$(date +%Y%m%d).log

# Check Jira for claimed tickets (look for worker-claim label)
```

---

## Summary

**Tonight:** Prove real Jira-driven self-improvement on 3-5 tickets

**If successful tomorrow:** Add compliance lane on separate queue

**If noisy tomorrow:** Fix discovery, task selection, report quality first

**Key rule:** Keep it small, safe, and focused. One worker, one queue, one task at a time.
