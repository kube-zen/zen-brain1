# Queue Steward Pilot Report

**Date:** 2026-03-28
**Status:** Pilot complete — recommend production deployment

## Operating Statements
- Queue Steward is an L1 factory-floor supervisor role
- GLM-5 supervises policy and exceptions, not routine queue care
- Underfilled factory with ready backlog is treated as a bug
- Queue Steward manages flow, not strategy
- Queue Steward does not bypass validation or approval rules

## 1. Scope

Test the queue-steward binary against live Jira project ZB with real queue state. Verify:
- Queue snapshot gathering works
- L1 recommendation call works (with heuristic fallback)
- Safe Jira transitions execute correctly
- Structured artifacts are written
- No unsafe Done transitions occur

## 2. Schedule / Cadence Used

Three runs in fast succession (manual invocation):
- Run 1: DRY_RUN=true, mode=fast → heuristic fallback (L1 timeout)
- Run 2: DRY_RUN=true, mode=fast → heuristic fallback (L1 timeout)
- Run 3: Live execution, mode=summary → L1 call succeeded

All runs used `SAFE_L1_CONCURRENCY=5`.

## 3. Queue Metrics Before

| Metric | Value |
|--------|-------|
| Ready backlog | 0 |
| Retry | 3 (ZB-883, ZB-1045, ZB-1047) |
| In Progress | 0 → 3 (after retry) |
| Paused | 8 |
| Blocked | 0 |
| Safe target | 5 |
| Fill ratio | 0% → 60% |

## 4. Queue Metrics After

| Metric | Value |
|--------|-------|
| Ready backlog | 0 |
| Retry | 0 (all moved to In Progress) |
| In Progress | 3 |
| Paused | 8 |
| Fill ratio | 60% (3/5 target) |
| Discovery throttled | false (0 ready backlog) |

## 5. Dispatch Actions Taken

Run 1 (DRY_RUN):
- ZB-883: DRY_RUN: would retry
- ZB-1045: DRY_RUN: would retry
- ZB-1047: DRY_RUN: would retry

Run 3 (live):
- ZB-883: moved from RETRYING → In Progress
- ZB-1045: moved from RETRYING → In Progress
- ZB-1047: moved from RETRYING → In Progress

All transitions were safe: RETRYING → In Progress (standard retry path).

## 6. Stale Tickets Corrected

3 RETRYING tickets were identified and actioned. No tickets were stuck in Selected for Development or stale In Progress during the pilot.

## 7. Discovery Throttle Behavior

Discovery was NOT throttled during the pilot:
- Ready backlog = 0 (below threshold of 10)
- `discovery_throttled: false` in all runs
- Correct behavior: no backlog pressure → no throttle needed

## 8. Backlog Drain Effect

- 3 RETRYING tickets were re-dispatched to In Progress
- 8 PAUSED tickets correctly left alone (steward does not force-retry PAUSED)
- 0 unsafe Done transitions occurred
- Factory fill improved from 0% → 60%

## 9. Blocker Summary

- L1 endpoint was unavailable for runs 1-2 (context deadline exceeded at 60s)
- Heuristic fallback worked correctly — recommended retry for all RETRYING tickets
- Run 3: L1 came back, provided structured recommendations
- No data loss or incorrect state transitions

## 10. Recommendation

**Deploy queue-steward to production with:**
- `queue-steward-fast` schedule: every 5 minutes
- `queue-steward-summary` schedule: every 30 minutes
- `SAFE_L1_CONCURRENCY=5` (aligned with factory-fill)
- Heuristic fallback is production-safe — deterministic, no hallucinated actions
- No unsafe Done transitions possible — steward only dispatches, requeues, pauses, escalates

**Artifact paths:**
- JSON: `/var/lib/zen-brain1/evidence/queue-steward/queue-health.json`
- Actions: `/var/lib/zen-brain1/evidence/queue-steward/queue-actions.json`
- Markdown: `/var/lib/zen-brain1/evidence/queue-steward/queue-health.md`
- Per-run: `/var/lib/zen-brain1/evidence/queue-steward/queue-health-{run-id}.md`
