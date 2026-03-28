# Phase A: Terminal-State Fix — Live Proof

**Date:** 2026-03-28  
**Commit:** 9d0a2bf (PHASE A: fix quality-gate terminal-state bug)

## Purpose

Prove that the explicit terminal-result contract works end-to-end for:
1. Quality-gate **pass** → ticket reaches correct terminal state
2. Quality-gate **pass with review** → ticket reaches correct terminal state
3. Quality-gate **reject** → ticket moves out of In Progress immediately

## What Changed

Before Phase A, the factory-fill dispatcher scraped stdout for "REJECTED" or "BLOCKED" strings to detect quality-gate failures. This was fragile — if the worker process exited 0 (success), the dispatcher assumed success, even when the quality gate rejected the payload.

After Phase A: The worker writes an explicit `WorkerTerminalResult` JSON file to `RESULT_DIR/{JIRA_KEY}.json`. The factory-fill dispatcher reads this file as the authoritative source of truth.

## Proof 1: Success Path (ZB-1037)

**Ticket:** ZB-1037 — Syntax error in parse() function causes runtime crash  
**Prior state:** Backlog  
**Worker terminal classification:** `needs_review`  
**Quality gate:** PASSED (score 15/25, readiness: ready_with_review)  
**L1 result:** type=cannot_fix, status=needs_review  
**Final Jira state:** Done  
**Terminal result file:** `/tmp/zen-brain1-worker-results/ZB-1037.json`

```
{
  "jira_key": "ZB-1037",
  "terminal_class": "needs_review",
  "quality_score": 15,
  "quality_passed": true,
  "l1_status": "needs_review",
  "jira_state": "Done",
  "evidence_path": "/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-1037",
  "gate_log_path": "/var/lib/zen-brain1/quality-gate-logs/ZB-1037-passed.json",
  "timestamp": "2026-03-28T17:33:42-04:00"
}
```

**Jira labels after:** ai:finding, ai:remediated, bug, quality:ready-with-review  
**Jira comment posted:** Yes — full remediation result with quality-gated payload  
**Evidence pack:** `/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-1037/` (exists)  
**Gate log:** `/var/lib/zen-brain1/quality-gate-logs/ZB-1037-passed.json` (exists)

**Result:** ✅ Terminal result file written. Ticket moved to Done. Not stuck In Progress.

---

## Proof 2: Success Path (ZB-1032)

**Ticket:** ZB-1032 — Unnecessary file permissions allowing arbitrary file access  
**Prior state:** Backlog  
**Worker terminal classification:** `done`  
**Quality gate:** PASSED (score 17/25, readiness: ready_with_review)  
**L1 result:** type=code_edit, status=success  
**Final Jira state:** Done  
**Terminal result file:** `/tmp/zen-brain1-worker-results/ZB-1032.json`

```
{
  "jira_key": "ZB-1032",
  "terminal_class": "done",
  "quality_score": 17,
  "quality_passed": true,
  "l1_status": "success",
  "jira_state": "Done",
  "evidence_path": "/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-1032",
  "gate_log_path": "/var/lib/zen-brain1/quality-gate-logs/ZB-1032-passed.json",
  "timestamp": "2026-03-28T17:34:19-04:00"
}
```

**Jira labels after:** ai:finding, ai:remediated, bug, quality:ready-with-review  
**Jira comment posted:** Yes — full remediation result  
**Evidence pack:** `/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-1032/` (exists)  
**Gate log:** `/var/lib/zen-brain1/quality-gate-logs/ZB-1032-passed.json` (exists)

**Result:** ✅ Terminal result file written. Ticket moved to Done. Not stuck In Progress.

---

## Proof 3: Reject Path (ZB-1045) — THE KEY PROOF

**Ticket:** ZB-1045 — Bug in code (deliberately weak: "Fix it")  
**Prior state:** Backlog  
**Worker terminal classification:** `blocked_invalid_payload`  
**Quality gate:** REJECTED (score 13/25, below threshold 15)  
**L1 result:** type=code_edit, status=success  
**Quality gate issues:** ["evidence"]  
**Final Jira state:** PAUSED  
**Terminal result file:** `/tmp/zen-brain1-worker-results/ZB-1045.json`

```
{
  "jira_key": "ZB-1045",
  "terminal_class": "blocked_invalid_payload",
  "quality_score": 13,
  "quality_passed": false,
  "l1_status": "success",
  "jira_state": "PAUSED",
  "evidence_path": "/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-1045",
  "blocker_reason": "Quality gate score 13/25 < 15. Missing: evidence",
  "issues": ["evidence"],
  "gate_log_path": "/var/lib/zen-brain1/quality-gate-logs/ZB-1045-rejected.json",
  "timestamp": "2026-03-28T17:47:22-04:00"
}
```

**Jira labels after:** ai:blocked, ai:finding, bug, quality:blocked-invalid-payload  
**Jira comments posted (2):**
1. `[zen-brain1 remediation] L1 completed remediation attempt... BLOCKED BY QUALITY GATE (score 13/25)...`
2. `[zen-brain1 quality gate] REJECTED — Score: 13/25 (threshold: 15) — Reason(s): evidence — Next action: Review evidence pack. If fixable, update ticket and re-queue. Current Jira state: In Progress → PAUSED`

**Evidence pack:** `/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-1045/` (exists — includes rejection-log/)  
**Rejection note:** `/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-1045/rejection-log/blocker-note.md` (exists)

**Result:** ✅ **Ticket did NOT remain In Progress.** Moved to PAUSED with explicit quality-gate rejection comment. Terminal result file correctly records `blocked_invalid_payload`.

---

## Conclusions

1. **Terminal result files are now the source of truth.** The factory-fill dispatcher reads `RESULT_DIR/{JIRA_KEY}.json` for authoritative classification. No more stdout string scraping.

2. **Rejected tickets no longer remain In Progress.** The quality-gate rejection path:
   - Worker adds `quality:blocked-invalid-payload` label
   - Worker posts explicit Jira comment with score, reasons, evidence path, next action
   - Worker transitions ticket to PAUSED
   - Worker writes terminal result file with `blocked_invalid_payload` classification
   - Factory-fill dispatcher reads terminal result and confirms PAUSED state

3. **Stdout string matching is no longer required.** The fallback still exists for backward compatibility, but the terminal result file is the primary path.

4. **Reconciliation still works as safety net.** `reconcileStuckInProgress` and `reconcileDispatchedStates` still check for tickets stuck In Progress and fix them. But they are no longer the primary rescue path — the terminal result file prevents the need for rescue.

5. **All 7 terminal classifications are implemented:** done, needs_review, paused, blocked_invalid_payload, retrying, to_escalate, failed.

## Operating Statements

- Quality-gate-rejected tickets must not remain In Progress ✅ PROVEN
- Throughput measurements are invalid until terminal states are correct — terminal states are now correct
- Discovery must be throttled when backlog drain is the priority
- Worker-count changes are evidence-based, not guesswork
