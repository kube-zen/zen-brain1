# Staged Night Rollout - AI Handoff

## Tonight's Mission

Run Zen-Brain self-improvement loop with staged ramp: 5 → 10 → 15 tickets.

## Success Ladder

1. ✅ 15-20 min canary (PARTIAL) - Infrastructure proven
2. ✅ One real ticket canary (FULL PASS) - End-to-end proven
3. ⏭️ **Phase 1: 5 tickets (45-60 min)** - Low-volume test
4. ⏭️ **Phase 2: 10 tickets (if Phase 1 clean)** - Medium-volume test
5. ⏭️ **Phase 3: 15 tickets overnight (if Phase 2 clean)** - Full production

## Gates to Advance

**Only move to next phase if ALL gates are clean:**

### Safety Gates
- ✅ No crashes
- ✅ No duplicate claims
- ✅ No duplicate comments
- ✅ Safe Class A/B behavior only (no Class C)

### Quality Gates
- ✅ Useful output quality (not spammy/junky)
- ✅ Morning report stays concise
- ✅ No "junk work" dominating the queue

## Phase 1: Start Now (5 Tickets)

### Create 5 Tickets

Open Jira: https://zen-mesh.atlassian.net/projects/ZB

Create 5 tickets with these specs:

**All tickets:**
- Label: `zen-brain-nightshift`
- Priority: `Low`
- Type: `Improvement` or `Task`

---

**Ticket 1:** Improve runtime doctor clarity
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

**Ticket 2:** Improve Jira proof/comment formatting
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

**Ticket 3:** Improve nightly summary formatting
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

**Ticket 4:** Add regression test for Jira ADF parsing
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

**Ticket 5:** Hunt low-risk hardcoded/default-path issue
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

### Run Phase 1 (45-60 minutes)

```bash
cd /home/neves/zen/zen-brain1
export ZEN_RUNTIME_PROFILE=dev
export OLLAMA_BASE_URL=http://127.0.0.1:11434
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
export JIRA_URL=https://zen-mesh.atlassian.net
export JIRA_EMAIL=zen@kube-zen.io
export JIRA_TOKEN=$(grep "^token:" ~/.zen-brain1-config/jira.yaml | awk '{print $2}' | tr -d '"')

./bin/zen-brain self-improvement 2>&1 | tee /tmp/nightshift-phase1-$(date +%Y%m%d).log
```

### Check Results (Phase 1)

```bash
# Check morning report
cat /tmp/nightshift-phase1-$(date +%Y%m%d).log | grep -A 20 "=== Morning Report ==="

# Check for errors
grep -i error /tmp/nightshift-phase1-$(date +%Y%m%d).log

# Check for Class C processing
grep -i "class C\|approval required\|escalated" /tmp/nightshift-phase1-$(date +%Y%m%d).log
```

### Decision (Phase 1 Complete)

**If ALL GATES CLEAN → Proceed to Phase 2:**
- Add 5 more nightshift tickets (10 total)
- Run controlled run
- Verify gates again

**If ANY GATE NOISY → Fix before advancing:**
- Fix discovery query
- Fix task selection logic
- Fix report quality
- Fix claim/lease behavior
- Restart Phase 1

---

## Phase 2: Expand to 10 Tickets (if Phase 1 clean)

**Action:** Add 5 more nightshift tickets (10 total)

**Run Command:** Same as Phase 1

**Check Results:** Same as Phase 1

**Decision:** Same gates apply

---

## Phase 3: 15-Ticket Overnight Run (if Phase 2 clean)

**Action:** Add 5 more nightshift tickets (15 total), let run overnight

**Run Command:** Same as Phase 1, but run unattended overnight

**Check Tomorrow:** Review morning report, Jira comments/artifacts

**Decision:**
- Clean → Production-ready
- Noisy → Fix before next overnight run

---

## Quick Reference

```bash
# Verify Jira before starting
./bin/zen-brain office doctor

# Check nightshift tickets
./bin/zen-brain office search 'project = ZB AND labels = "zen-brain-nightshift"'

# Run self-improvement
./bin/zen-brain self-improvement

# View morning report
grep -A 20 "=== Morning Report ===" /tmp/nightshift-phase1-$(date +%Y%m%d).log

# Check for errors
grep -i error /tmp/nightshift-phase1-$(date +%Y%m%d).log

# Check for Class C processing
grep -i "class C\|approval required\|escalated" /tmp/nightshift-phase1-$(date +%Y%m%d).log
```

---

## Summary

**Start:** Phase 1 (5 tickets, 45-60 min)
**Advance:** Only if all gates are clean (safety + quality)
**Ramp:** 5 → 10 → 15 tickets
**Goal:** Staged rollout with low blast radius, enough volume to reveal noise

**Practical Rule:** Start at 5. If first hour is clean, raise to 15.
