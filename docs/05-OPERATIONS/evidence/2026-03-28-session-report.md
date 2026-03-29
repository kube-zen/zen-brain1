# zen-brain1 Operations Report — 2026-03-28

**Prepared by:** GLM-5 (supervisor/policy role)
**Date:** 2026-03-28 20:51 EDT
**Session scope:** Phase A through Queue Steward deployment

---

## Executive Summary

Today's session delivered four major infrastructure improvements to zen-brain1's remediation factory:

1. **Fixed the critical quality-gate terminal-state bug** — tickets no longer get stuck In Progress when rejected
2. **Validated and cleaned per-schedule workers override** — each schedule now has explicit W settings
3. **Ran the first bounded concurrency sweep** — W=5 baseline captured, W=7 inconclusive but no regression
4. **Deployed the Queue Steward** — a new L1 factory-floor supervisor that keeps the factory filled and queue healthy

All changes are committed, pushed, and live-proven against real Jira tickets.

---

## Jira State of the Union

| State | Count |
|-------|-------|
| Backlog | 53 |
| In Progress | 3 |
| Done | 677 |
| PAUSED | 8 |
| RETRYING | 0 |
| TO_ESCALATE | 0 |
| **Total** | **741** |

**Done rate:** 677/741 = **91.4%** of all tickets have reached Done.
**Stuck In Progress:** 0 (was the primary bug before today — fixed).

---

## Phase A: Quality-Gate Terminal-State Fix

### Problem
When the quality gate rejected a ticket (score < 15/25), the worker logged `REJECTED` to stdout but the factory-fill couldn't reliably detect it. Tickets remained stuck In Progress indefinitely.

### Solution
Replaced fragile stdout string matching with explicit JSON terminal result files:
- Worker writes `RESULT_DIR/{KEY}.json` after every ticket
- Factory-fill reads terminal result file for state decisions
- Reconciliation remains as safety net (label heuristic fallback)

### Terminal Classifications

| Class | Quality Gate | Jira State |
|-------|-------------|------------|
| done | passed | Done |
| needs_review | passed | Done |
| paused | rejected | PAUSED |
| blocked_invalid_payload | rejected | PAUSED |
| retrying | L1 failure | RETRYING |
| to_escalate | L1 says human needed | TO_ESCALATE |

### Live Proof (3 paths)

| Ticket | Score | Class | Jira State | Result |
|--------|-------|-------|------------|--------|
| ZB-1032 | 17/25 | done | Done | ✅ Pass |
| ZB-1037 | 15/25 | needs_review | Done | ✅ Pass |
| ZB-1045 | 5/25 | blocked_invalid_payload | PAUSED | ✅ Reject → PAUSED correctly |

**Key guarantee:** Quality-gate-rejected tickets NEVER remain In Progress.

### Commit
`9d0a2bf` — PHASE A: fix quality-gate terminal-state bug

---

## Phase B: Per-Schedule Workers Override

### What Changed
- Scheduler already supported per-schedule WORKERS in code (lines 224–236)
- Cleaned YAML config to remove duplicate `workers:` entries
- Set explicit values per schedule

### Schedule Workers

| Schedule | Workers | Tasks | Rationale |
|----------|---------|-------|-----------|
| hourly-scan | 5 | 3 | Conservative for small batches |
| quad-hourly-summary | 5 | 6 | Same as hourly until evidence supports higher |
| daily-sweep | 7 | 10 | First experiment — larger batch benefits from concurrency |

### Commit
`bee1a89` — feat(scheduler): per-schedule workers override for bounded concurrency control

---

## Phase C: Concurrency Sweep (W=5 vs W=7)

### W=5 Baseline

**Batch:** 13 tickets, 4 dispatch cycles, ~7 min wall time

| Metric | Value |
|--------|-------|
| Avg latency | 77.8s |
| P50 latency | 27.0s |
| P95 latency | 248.0s |
| Done | 6 (46%) |
| Paused | 3 (23%) |
| Blocked (quality gate) | 4 (31%) |
| Stuck In Progress | 0 |
| L1 timeout rate (cycle 1) | 60% |
| L1 timeout rate (later) | 12.5% |
| Quality gate pass rate | 69% |

### W=7 Comparison

**Inconclusive** — only 3 tickets available to fill 7 slots. The factory correctly filtered stale/blocked tickets.

| Metric | W=5 | W=7 |
|--------|-----|-----|
| Batch size | 13 | 3 |
| Done | 6 | 3 |
| Stuck In Progress | 0 | 0 |
| Quality regression | — | None |
| L1 timeouts | 23% | 0% |

### Verdict
- W=7 shows **no regression** — correct terminal states, no stuck tickets
- Throughput benefit **unproven** — need ≥10 ready tickets for proper comparison
- **Keep W=7 for daily-sweep**, revert to W=5 if regression appears
- W=10 remains **not approved**

### Commit
`f8086a2` — test(runtime): bounded W=5 vs W=7 sweep + evidence docs

---

## Phase D: Backlog Priority / Discovery Throttle

### What Changed
- Throttle logic already existed in code (lines 295–315)
- Validated behavior: ready backlog > 10 → discovery throttled
- Policy enforced: 70% remediation, 20% roadmap, 10% discovery

### Commit
`91769e0` — feat(queue): throttle discovery and prioritize backlog drain

---

## Queue Steward

### What It Is
A new L1 factory-floor supervisor that keeps the factory filled with ready work and produces structured queue-health artifacts.

### What It Does
1. Gathers Jira queue snapshot (all non-Done states)
2. Calls L1 for structured JSON recommendations
3. Executes safe actions (dispatch, requeue, pause, escalate)
4. Writes artifacts (queue-health.json, queue-actions.json, queue-health.md)

### What It Does NOT Do
- Move tickets to Done (no closure bypass)
- Change policy or worker counts
- Override approval gates
- Invent strategic decisions

### Pilot Results

| Run | Mode | L1 Status | Actions |
|-----|------|-----------|---------|
| 1 | fast (dry-run) | Timeout → heuristic | 3 retry recommendations |
| 2 | fast (dry-run) | Timeout → heuristic | 3 retry recommendations |
| 3 | summary (live) | Timeout → heuristic | 3 tickets re-dispatched |

- 0 unsafe transitions
- 0 data loss
- Factory fill: 0% → 60%
- Artifacts written to `/var/lib/zen-brain1/evidence/queue-steward/`

### Schedules
- `queue-steward-fast` — every 5 min (queue hygiene)
- `queue-steward-summary` — every 30 min (health artifact)

### Commit
`ef0f009` — feat(tasks): add queue-steward-l1 template, binary, schedules, and pilot evidence

---

## All Commits This Session

```
ef0f009 feat(tasks): add queue-steward-l1 template, binary, schedules, and pilot evidence
f8086a2 test(runtime): bounded W=5 vs W=7 sweep + evidence docs
2555301 docs(ops): document terminal-state handling, workers override, and backlog-priority policy
bee1a89 feat(scheduler): per-schedule workers override for bounded concurrency control
91769e0 feat(queue): throttle discovery and prioritize backlog drain
9d0a2bf PHASE A: fix quality-gate terminal-state bug
```

## Files Changed (21 files, +1889 / -296 lines)

| Category | Files |
|----------|-------|
| Core fix | `cmd/remediation-worker/main.go`, `cmd/factory-fill/main.go` |
| Scheduler | `cmd/scheduler/main.go` |
| Queue Steward | `cmd/queue-steward/main.go` (new) |
| Config | `config/schedules/*.yaml`, `config/task-templates/queue-steward-l1.yaml` |
| Evidence | `docs/05-OPERATIONS/evidence/phase-a-*.md`, `queue-steward-pilot.md`, `l1-workers-*.md` |
| Ops docs | `OBSERVABILITY_AND_THROUGHPUT.md`, `QUEUE_STEWARD_ROLE.md` |
| Binaries | `factory-fill`, `remediation-worker`, `queue-steward` |

---

## Recommended Next Actions

1. **Activate queue-steward-fast** schedule in production (every 5 min)
2. **Run proper W=5 vs W=7 comparison** when natural backlog reaches ≥10 ready tickets
3. **Review queue-health artifacts** after 24h of steward operation
4. **Consider queue-steward-daily** schedule for backlog composition review
5. **Monitor fill ratio** — underfilled factory with ready backlog is now a tracked bug

---

## Operating Statements

- Terminal result files are the authoritative source for state transitions
- Quality-gate-rejected tickets must not remain In Progress
- Worker-count changes are evidence-based, not guesswork
- Backlog drain remains the priority over new discovery
- W=7 is the first approved concurrency step; W=10 remains unapproved
- Queue Steward is an L1 factory-floor supervisor role
- GLM-5 supervises policy and exceptions, not routine queue care
- Underfilled factory with ready backlog is a bug
- Queue Steward manages flow, not strategy
- Queue Steward does not bypass validation or approval rules
