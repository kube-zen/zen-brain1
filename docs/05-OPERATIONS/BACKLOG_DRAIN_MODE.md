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

The remediation-worker subprocess exits 0 even when the quality gate rejects a ticket's payload.
This previously left tickets stuck In Progress with no correct terminal state.

**Fix (commit 9d0a2bf):** Explicit terminal classification via JSON result file.

### Worker Terminal Result Contract

Every remediation-worker run writes `RESULT_DIR/{JIRA_KEY}.json`:

```json
{
  "jira_key": "ZB-931",
  "terminal_class": "blocked_invalid_payload",
  "quality_score": 10,
  "quality_passed": false,
  "l1_status": "success",
  "jira_state": "PAUSED",
  "evidence_path": "/var/lib/zen-brain1/evidence/...",
  "blocker_reason": "Quality gate score 10/25 < 15. Missing: evidence, validation",
  "issues": ["evidence", "validation"],
  "gate_log_path": "/var/lib/zen-brain1/quality-gate-logs/ZB-931-rejected.json",
  "timestamp": "2026-03-28T17:20:00Z"
}
```

### Terminal Classifications

| Class | Meaning | Jira State |
|-------|---------|------------|
| `done` | Quality gate passed, L1 success | Done |
| `needs_review` | Quality gate passed, L1 says review needed | Done |
| `paused` | Quality gate rejected (score < 15) | PAUSED |
| `blocked_invalid_payload` | Quality gate rejected, payload not usable | PAUSED |
| `retrying` | L1 call failed entirely | RETRYING |
| `to_escalate` | L1 says human must handle | TO_ESCALATE |
| `failed` | Unrecoverable error | RETRYING |

### Factory-Fill Dispatcher Flow

1. `dispatchTicket` moves ticket to In Progress
2. Runs remediation-worker subprocess with `RESULT_DIR` env
3. Reads `RESULT_DIR/{JIRA_KEY}.json` for explicit classification
4. `handleTerminalResult` processes each class:
   - `done`/`needs_review` → success
   - `paused`/`blocked_invalid_payload` → verify PAUSED state
   - `retrying`/`failed` → verify RETRYING state
   - `to_escalate` → verify TO_ESCALATE state
5. If no terminal result file: fall back to stdout heuristic (legacy)
6. Post-dispatch: `reconcileDispatchedStates` double-checks all dispatched tickets

### Reconciliation Safety Net

Two reconciliation passes catch any stragglers:

1. **`reconcileStuckInProgress`** — runs at factory-fill startup and every cycle start
   - Checks ALL In Progress tickets
   - Terminal result file present → use authoritative classification
   - No terminal result → fall back to label heuristic

2. **`reconcileDispatchedStates`** — runs after each dispatch batch
   - Checks only tickets dispatched this cycle
   - Same dual-source logic as above

### Quality Gate Rejection Flow

When quality gate rejects (score < 15/25):

1. Worker moves ticket to In Progress
2. Worker adds `quality:blocked-invalid-payload` label
3. Worker posts explicit Jira comment with score, reasons, evidence path
4. Worker moves ticket to PAUSED
5. Worker writes terminal result file (`blocked_invalid_payload`)
6. Factory-fill reads terminal result and confirms PAUSED state
7. If anything fails, reconciliation catches it on next cycle

**Invariant: quality-gate-rejected tickets must not remain In Progress.**

## Allocation

| Work Type | % | Notes |
|-----------|---|-------|
| Remediation / backlog drain | 70% | Primary — Done movement is the metric |
| Roadmap / office execution | 20% | Office support, bounded tickets |
| Discovery refresh / dedup | 10% | Keep from flooding |

## Per-Schedule WORKERS Override

Currently all schedules use hardcoded `SAFE_L1_CONCURRENCY=5`.
Evidence supports:
- W=5: 3-task hourly-scan completes in 55-144s
- W=7: no throughput gain for ≤3 task batches (CPU contention)
- W=7 may benefit daily-sweep (10 tasks) — test separately

Per-schedule override: `WORKERS` env per schedule config.

## Decision Rule

After throughput experiment:
- Keep highest worker level that improves done/hour without degrading quality/state correctness
- Do not choose level based on raw CPU usage alone
- **Throughput measurements are invalid until terminal states are correct** (fixed in Phase A)

## Jira State Machine

States: Backlog(11), Selected for Development(21), In Progress(31), Done(41), PAUSED(51), RETRYING(61), TO_ESCALATE(71)

Required: Jira In Progress reflects actual active work.

## Operating Statements

- Quality-gate-rejected tickets must not remain In Progress
- Throughput measurements are invalid until terminal states are correct
- Discovery must be throttled when backlog drain is the priority
- Worker-count changes are evidence-based, not guesswork
