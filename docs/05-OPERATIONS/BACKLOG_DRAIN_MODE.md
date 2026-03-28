# Backlog Drain Mode

**Version:** 1.0
**Created:** 2026-03-28
**Status:** Active

## Core Principle

**Success is tickets reaching Done, not tickets created.**
**Discovery must not outpace execution.**
**L1 does the heavy lift for bounded remediation.**

## Operating Rules

### Rule 1: Done is the Metric
- The system's health is measured by tickets reaching Done
- Tickets created but never drained = system failure
- Backlog growing while Done stays flat = throttle creation

### Rule 2: Throttle Creation if Drain is Losing
```
if Backlog_count > 2 * Drain_capacity_per_cycle:
    throttle ticket creation to top-N only
    stop generating duplicate findings
    focus on draining what exists
```

### Rule 3: Deduplicate Before Creating
- Before creating a new finding ticket, check if the same finding exists
- Same finding from a new scan run = comment on existing ticket, not new ticket
- Batch parent tickets should aggregate, not multiply

### Rule 4: State Machine is Operational
States are not decoration. They mean things:
- **Backlog** — waiting to be triaged
- **Selected for Development** — selected for execution
- **In Progress** — being worked by L1/supervisor
- **PAUSED** — waiting on dependency/review
- **RETRYING** — L1 failed, retrying
- **TO_ESCALATE** — needs human/L2
- **Done** — validated and closed

### Rule 5: Done Criteria
A ticket may move to Done ONLY if:
1. Remediation/output exists
2. Validation passed
3. Jira comment/log added
4. Evidence pack updated (if applicable)
5. No unresolved blocker remains
6. Governance/compliance fields preserved

### Rule 6: Separate Ops from Factory
| Category | Measured By |
|----------|------------|
| Ops cleanup | Backlog reduction, stale closure |
| L1 production | Attributable artifacts per L1 attribution policy |
| Supervisor work | Intervention rate, escalation handling |

## Current State (2026-03-28)

### Backlog Drain Results
- **Baseline:** 524 Backlog, 0 Done
- **After drain:** 2 Backlog, 539 Done (including bulk close + pilot)
- **Method:** Script-driven bulk close of stale scanner tickets (ops cleanup, NOT L1 production)

### L1 Attribution Results
- **Pilot:** 10 bounded tasks dispatched to 0.8b
- **L1-produced:** 3/10 (30%)
- **L1-needs-review:** 7/10 (70% — timeouts, parse failures)
- **Assessment:** L1 not ready for autonomous expansion. See L1_ATTRIBUTION_POLICY.md.

### Throttle Status
Scanner ticket creation should be throttled:
- Only create new tickets for NEW findings (diff against previous run)
- Stop creating batch parent tickets for runs that produce no new findings
- Maximum: 10 new tickets per cycle until drain capacity catches up

## Tools

- `scripts/jira-drain.py` — State machine + drain operations
- `scripts/l1-attribution-pilot.py` — L1 attribution testing
- `scripts/backlog-baseline.py` — Capture backlog state
- `cmd/remediation-worker/` — Go-based remediation worker (production path)

## Jira Transition IDs

| Target State | Transition ID |
|-------------|--------------|
| Backlog | 11 |
| Selected for Development | 21 |
| In Progress | 31 |
| Done | 41 |
| PAUSED | 51 |
| RETRYING | 61 |
| TO_ESCALATE | 71 |

All transitions are global (available from any state).

## Mandatory Statements

- Success is tickets reaching Done, not tickets created
- L1 does the heavy lift (when it can)
- Jira states are now operational, not decorative
- Evidence packs are part of closure
- Discovery is throttled if backlog drain falls behind
- Ops cleanup ≠ factory production
