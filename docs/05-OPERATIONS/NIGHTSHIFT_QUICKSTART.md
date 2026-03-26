# Tonight: Self-Improvement Proof Test - Quick Start

## 5-Minute Setup

### 1. Create Tickets in Jira (5 minutes)

Open: https://zen-mesh.atlassian.net/projects/ZB

Create 3-5 tickets using these exact specs:

**All tickets should have:**
- Label: `zen-brain-nightshift`
- Priority: `Low`
- Type: `Improvement` or `Task`

---

### Ticket 1: Improve runtime doctor clarity

**Title:** Improve runtime doctor clarity for failures/warnings

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

### Ticket 2: Improve Jira proof/comment formatting

**Title:** Improve Jira proof/comment formatting

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

### Ticket 3: Improve nightly summary report structure

**Title:** Improve nightly summary report structure

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

### Ticket 4: Add regression test for Jira ADF parsing (optional)

**Title:** Add regression test for Jira ADF parsing

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

### Ticket 5: Hunt low-risk hardcoded/default-path issue (optional)

**Title:** Hunt low-risk hardcoded/default-path issue

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

## Run Tonight (2 commands)

### Option 1: Quick Test (manual steps)

```bash
cd /home/neves/zen/zen-brain1

# 1. Verify tickets exist
export JIRA_URL=https://zen-mesh.atlassian.net
export JIRA_EMAIL=zen@kube-zen.io
export JIRA_TOKEN=$(grep "^token:" ~/.zen-brain1-config/jira.yaml | awk '{print $2}' | tr -d '"')
./bin/zen-brain office search 'project = ZB AND labels = "zen-brain-nightshift"'

# 2. Run self-improvement loop
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
./bin/zen-brain self-improvement 2>&1 | tee /tmp/nightshift-$(date +%Y%m%d).log
```

### Option 2: Use helper script

```bash
cd /home/neves/zen/zen-brain1
./scripts/run-nightshift-proof.sh
```

---

## Check Tomorrow (5 minutes)

### 1. Review Morning Report

```bash
cat /tmp/nightshift-$(date +%Y%m%d).log | grep -A 20 "=== Morning Report ==="
```

Look for:
- Tasks discovered (should be 3-5)
- Tasks claimed (should be 1-3, depending on run length)
- Tasks processed (should have useful comments)
- Tasks escalated (should be 0)

---

### 2. Check Jira for Each Ticket

Open each ticket in Jira and verify:
- Worker comment exists (identity: zb-self-improvement-1)
- Action class is A or B only (no Class C)
- Recommendations are actionable
- Proof artifacts attached (proof-of-work.json)
- No noisy junk

---

### 3. Verify Safety

```bash
# Check for any errors
grep -i error /tmp/nightshift-$(date +%Y%m%d).log

# Check for Class C processing (should be 0)
grep -i "class C\|approval required\|escalated" /tmp/nightshift-$(date +%Y%m%d).log
```

---

## Success Criteria

✅ **Real Jira discovery works** - Found 3-5 nightshift tickets
✅ **Claim/lease prevents duplicates** - One ticket at a time, no re-processing
✅ **Useful comments/artifacts** - Worker identity, action class, actionable recommendations
✅ **Morning report is concise** - Tasks discovered/claimed/processed/escalated
✅ **No noisy/unsafe behavior** - No Class C, no deploys, no repo writes

---

## Tomorrow's Decision

### If Successful (all 5 ✅)

Add compliance lane:
```bash
# Create compliance tickets (label: compliance-reporting)
./bin/zen-brain compliance reporter
./bin/zen-brain compliance gap-hunter
```

### If Noisy/Brittle (any ❌)

Fix before expanding:
- Discovery query
- Task selection logic
- Report quality
- Claim/lease behavior

---

## Quick Reference Commands

```bash
# Verify Jira
cd /home/neves/zen/zen-brain1
./bin/zen-brain office doctor

# Check nightshift tickets
./bin/zen-brain office search 'project = ZB AND labels = "zen-brain-nightshift"'

# Run loop
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
./bin/zen-brain self-improvement

# View morning report
cat /tmp/nightshift-$(date +%Y%m%d).log | grep -A 20 "Morning Report"

# Check errors
grep -i error /tmp/nightshift-$(date +%Y%m%d).log
```

---

## Notes

- **One worker only** - zb-self-improvement-1
- **One task at a time** - No parallelism
- **Class A/B only** - No Class C (approval required)
- **No deploys** - Read-only or safe writes only
- **No repo-write autonomy** - Manual review required for code changes
- **No secrets/cloud changes** - Read-only access only

---

## Documentation

Full details: `docs/05-OPERATIONS/NIGHTSHIFT_TONIGHT.md`

---

**Good luck tonight! 🚀**
