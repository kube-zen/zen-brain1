# Backlog Drain Mode

**Operating Policy:** Underfilled factory with backlog present = BUG.

## Current State

- **Factory-fill engine:** `cmd/factory-fill` — continuous dispatch loop
- **Safe concurrency target:** 5 (runtime env `SAFE_L1_CONCURRENCY`)
- **Proven throughput:** ~10 tickets/5min burst = ~120 tickets/hr (sustained lower)
- **L1-produced rate:** Varies by ticket complexity (patch-oriented contract)
- **Quality gate:** 15/25 threshold, rejects ~30% of complex tickets

## Fill Policy

```
target_in_progress = min(safe_l1_concurrency, ready_backlog + retrying)
```

If current in-progress < target: pull tickets immediately.

## State Reconciliation (PHASE A FIX)

The remediation-worker subprocess may exit 0 even when the quality gate rejects a ticket's payload. This leaves tickets stuck In Progress with incorrect terminal states.

**Fix:** Every factory-fill cycle starts with `reconcileStuckInProgress`:
- Checks all In Progress tickets
- If `ai:remediated` + `quality:blocked-invalid-payload` → PAUSED
- If `ai:remediated` + quality passed labels → Done
- If `ai:remediated` + no quality labels → Done (gate didn't fire)
- If NOT remediated → RETRYING (worker didn't process)

## Allocation
- 70% remediation / backlog drain
- 20% roadmap / office execution
- 10% discovery refresh / dedup

## Per-Schedule WORKERS Override
Currently all schedules use hardcoded `WORKERS=5`.
Evidence supports:
- W=5: 3-task hourly-scan completes in 55-144s
- W=7: no throughput gain for ≤3 task batches (CPU contention)
- W=7 may benefit daily-sweep (10 tasks) — test separately

## Decision Rule
After throughput experiment:
- Keep highest worker level that improves done/hour without degrading quality/state correctness
- Do not choose level based on raw CPU usage alone

## Jira State Machine
States: Backlog(11), Selected for Development(21), In Progress(31), Done(41), PAUSED(51), RETRYING(61), TO_ESCALATE(71)

Required: Jira In Progress reflects actual active work.
