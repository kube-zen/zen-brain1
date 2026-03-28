# Backlog Baseline — 2026-03-28

**Captured:** 2026-03-28T08:37:00-04:00
**Source:** Jira REST API v3 /search/jql against project ZB

## 1. State Counts

| Status | Count |
|--------|-------|
| Backlog | 517 |
| Selected for Development | 0 |
| In Progress | 0 |
| PAUSED | 0 |
| RETRYING | 0 |
| TO_ESCALATE | 0 |
| Done | 0 |
| **Total** | **517** |

**Key range:** ZB-285 to ZB-801

**All 517 issues are Task type. All are in Backlog. Zero have ever been transitioned.**

## 2. Ticket Composition

### By Source Label

| Source | Count | Description |
|--------|-------|-------------|
| hourly-scan | 167 | Hourly automated batch reports |
| daily-sweep | 145 | Daily automated batch reports |
| quad-hourly-summary | 122 | 4-hour summary batches |
| ai:finding (other) | 67 | Security/code findings from discovery |
| other | 16 | Miscellaneous |

### By Label

| Label | Count | Meaning |
|-------|-------|---------|
| zen-brain | 439 | Created by zen-brain1 |
| finding | 347 | Discovery finding |
| ai:completed | 214 | Batch completed (but never transitioned to Done!) |
| hourly-scan | 167 | Hourly scan batch |
| daily-sweep | 145 | Daily sweep batch |
| quad-hourly-summary | 122 | Quad-hourly summary |
| scheduled-batch | 76 | Part of a scheduled batch run |
| bug | 70 | Bug finding |
| ai:finding | 67 | AI-generated finding ticket |
| security | 28 | Security-related finding |
| ai:blocked | 18 | Previously blocked |
| defect | 5 | Defect finding |
| high-priority | 5 | High priority |
| ai:remediated | 3 | Already remediated (Phase 39 pilot) |
| pilot:phase39 | 3 | Phase 39 remediation pilot |
| quality:ready-with-review | 3 | Quality gate passed, needs review |
| irap:* | 3 | IRAP work package tagged |
| sred:* | 3 | SR&ED uncertainty tagged |
| quality:blocked-invalid-payload | 2 | Quality gate blocked |

### By Issue Type

| Type | Count |
|------|-------|
| Task | 517 |

All 517 are Task type. No epics, no sub-tasks, no stories.

## 3. Bounded vs Unbounded Estimate

### Batch Reports (434 tickets)
- **daily-sweep:** 145 — These are batch run parent/child reports. Most are informational ("here's what ran").
- **hourly-scan:** 167 — Same pattern. Each batch creates 3 child findings + 1 parent.
- **quad-hourly-summary:** 122 — Summary roll-ups of hourly scans.
- **Assessment:** These are **not remediation candidates**. They are telemetry artifacts that should have been closed (moved to Done) after creation. Many are labeled `ai:completed` already — they just never got transitioned.

### AI Findings (67 tickets)
- 47 are `bug` labeled
- 19 are `security` labeled
- 3 are already `ai:remediated` (Phase 39 pilot)
- 2 are `ai:blocked`
- **Assessment:** These are the **real work items**. ~62 are unremediated findings that could potentially be drained.

### Other (16 tickets)
- Mixed: audit, discovery, misc
- **Assessment:** Small set, needs individual triage.

### Summary

| Category | Count | Bounded? | Actionable? |
|----------|-------|----------|-------------|
| Batch reports (daily-scan/hourly/quad) | 434 | Yes (informational) | Close to Done immediately |
| AI findings (bug/security) | 62 | Mostly bounded | Drain through L1 remediation |
| Already remediated (Phase 39) | 3 | Bounded | Close or move to review |
| Other/misc | 16 | Unknown | Triage individually |

## 4. Execution-Ready Estimate

### Immediate Close Candidates (434 batch reports)
- Already labeled `ai:completed`
- Informational only — no code changes needed
- Should have been Done from the start
- **Estimated effort:** API transition only, no L1 work needed

### L1 Execution-Ready (40-50 findings)
- Bounded single-target bug/security findings
- Have problem descriptions
- Some have evidence paths in descriptions
- Need: target file extraction, validation command definition
- **Estimated effort:** L1 remediation + validation per ticket

### Needs Triage (12-16)
- Missing context or ambiguous
- May need human review before L1

### Blocked (2)
- ZB-614, ZB-618 already labeled ai:blocked
- Quality gate failures from Phase 39

## 5. Immediate Drain Candidates

### Tier 1: Batch Report Closure (434 tickets)
Move all `ai:completed` + `daily-sweep`/`hourly-scan`/`quad-hourly-summary` labeled tickets directly to Done.

**Rationale:** These are telemetry artifacts. They served their purpose at creation time. The `ai:completed` label proves they ran successfully. Keeping them in Backlog inflates the count and obscures real work.

### Tier 2: Already-Remediated Closure (3 tickets)
- ZB-614 (ai:remediated, quality:ready-with-review)
- ZB-616 (ai:remediated, quality:ready-with-review)
- ZB-618 (ai:remediated, quality:ready-with-review)

Move to Done or Selected for Development → In Progress → Done.

### Tier 3: Fresh Finding Drain (10 bounded tickets for pilot)
Select 10 fresh ai:finding tickets that are:
- Single-target (bug or security)
- Have descriptive summaries
- Can be dispatched to L1

**Priority candidates:**
1. ZB-586: Debugging tools not integrated with main CLI
2. ZB-615: Test_gaps report stability
3. ZB-570: Sensitive data access control logic missing
4. ZB-572: Input validation for user inputs absent
5. ZB-575: Session Management Incomplete
6. ZB-576: Comprehensive logging for error events
7. ZB-580: Incomplete Session Management in cmd/
8. ZB-587: Resource allocation not dynamically optimized
9. ZB-599: Untrusted Input Bypass via Exec/Env
10. ZB-606: Weak input validation leading to data corruption

## 6. Jira Transition IDs

All transitions are globally available from any state:

| Transition | ID | Status Category |
|-----------|-----|----------------|
| Backlog | 11 | To Do |
| Selected for Development | 21 | To Do |
| In Progress | 31 | In Progress |
| Done | 41 | Done |
| PAUSED | 51 | In Progress |
| RETRYING | 61 | In Progress |
| TO_ESCALATE | 71 | In Progress |

## 7. Root Cause

**Why 517 tickets are stuck in Backlog:**

1. **No transition code in remediation-worker:** The `updateJiraOutcome()` function only adds comments and labels. It never calls the Jira transitions API.
2. **Batch reports auto-created but never closed:** The scheduler/ledger creates tickets with `ai:completed` labels but never transitions them to Done.
3. **No drain loop exists:** There is no scheduled process that picks tickets from Backlog and moves them through the workflow.

**Fix required:**
1. Add `transitionJiraStatus()` to the remediation worker
2. Add batch-closure for informational tickets
3. Wire state machine transitions into the remediation pipeline
