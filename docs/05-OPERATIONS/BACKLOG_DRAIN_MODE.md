# BACKLOG_DRAIN_MODE.md — Operating Policy

**Created:** 2026-03-28
**Status:** ACTIVE

## Core Principle

**Success is tickets reaching Done, not tickets created.**

Jira must be a workflow engine, not an issue bucket. Every ticket that enters
Backlog must eventually leave it. If drain can't keep up, throttle creation.

## State Machine

```
Backlog → Selected for Development → In Progress → Done
                                                  ↗ Needs_review → Done
                                                 ↗ PAUSED → (dependency wait) → In Progress
                                                ↗ RETRYING → (retry) → In Progress
                                               ↗ TO_ESCALATE → (L2/human)
```

Transition IDs (global, same from any state):
- Backlog: 11
- Selected for Development: 21
- In Progress: 31
- Done: 41
- PAUSED: 51
- RETRYING: 61
- TO_ESCALATE: 71

## Done Criteria

A ticket may move to Done ONLY if ALL of these are true:

1. Remediation/output exists
2. Validation passed
3. Jira comment/log added
4. Evidence pack updated (if applicable)
5. No unresolved blocker remains
6. Governance/compliance fields preserved

No "looks okay" closures. No silent transitions without evidence.

## Batch Report Policy

All batch reports (daily-sweep, hourly-scan, quad-hourly-summary) are
informational telemetry artifacts. They SHALL be:

1. Created with `ai:completed` label
2. Immediately transitioned to Done after creation
3. Never left in Backlog

## L1 Execution Policy

L1 (0.8b model) handles:
- First-pass remediation for bounded findings
- Evidence-pack drafting
- Ticket update drafting
- Validation execution

L1 does NOT handle:
- Multi-file refactoring
- Ambiguous requirements
- Architecture decisions

## GLM-5 Supervisor Policy

GLM-5 handles:
- Queue management and ticket selection
- Jira state policy enforcement
- Blocker analysis and escalation decisions
- Cadence tuning
- Compliance/evidence oversight
- Throttle decisions

## Throttle Rule

If Backlog grows faster than Done for 3 consecutive drain cycles:
1. Reduce ticket creation to top-N only
2. Keep discovery running but limit ticketization
3. Do NOT create more backlog than drain capacity

## Drain Modes

### MODE=drain-backlog
Closes informational batch reports in bulk. Usage:
```
MODE=drain-backlog DRAIN_MAX_PER_LABEL=500 go run ./cmd/remediation-worker/
```

### MODE=pilot
Processes specific tickets through L1 with quality gating. Usage:
```
MODE=pilot PILOT_KEYS="ZB-xxx,ZB-yyy" go run ./cmd/remediation-worker/
```

### MODE=remediate
Standard remediation cycle — fetches open ai:finding tickets and processes
them through L1 with full quality gate pipeline. Usage:
```
MODE=remediate MAX_TICKETS=5 go run ./cmd/remediation-worker/
```

## Evidence Path

All evidence packs are written to:
```
docs/05-OPERATIONS/evidence/evidence-{YYYY-MM}/rem-{JIRA_KEY}/
```

Quality gate logs at:
```
docs/05-OPERATIONS/quality-gate-logs/{JIRA_KEY}-{decision}.json
```

Drain reports at:
```
docs/05-OPERATIONS/evidence/drain-reports/drain-{timestamp}.md
```
