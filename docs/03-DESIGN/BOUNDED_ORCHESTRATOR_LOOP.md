# Bounded Orchestrator Loop

## Purpose

The bounded orchestrator ensures that Zen‑Brain’s execution loops are predictable, resumable, and free of infinite retries. It provides a deterministic state‑machine wrapper around potentially non‑deterministic LLM calls and external system interactions.

## Core Requirements

- **Explicit polling cadence** – No arbitrary sleeps; cadence is configurable and observable.
- **Eligibility rules** – Clear pre‑conditions for moving to the next state.
- **Bounded retries** – Limited retry counts with exponential backoff.
- **Deterministic state transitions** – Every transition is logged and replayable.
- **Resume/recovery behavior** – Survives restarts without losing progress.
- **Timeout/escalation behavior** – Stuck tasks time out and escalate (e.g., to human review).
- **No infinite loops** – Guaranteed termination via step limits and timeouts.

## Design

### State Machine

```
created → eligible → scheduled → in_progress → (completed | failed | blocked | canceled)
```

Each state has:
- **Entry guard** – conditions that must be true before entering.
- **Action** – the work performed while in the state.
- **Exit guard** – conditions that must be true before transitioning out.
- **Timeout** – maximum duration allowed in this state.
- **Retry policy** – how many retries, backoff pattern, escalation after exhaustion.

### Polling Cadence

The orchestrator polls for work at a fixed interval (e.g., 30 seconds). The interval is configurable per queue/priority.

Polling is implemented as a **pull‑based tick**, not a busy loop. Each tick:
1. Checks for eligible sessions (those whose guards are satisfied).
2. Moves a bounded number of sessions to the next state (avoiding stampedes).
3. Updates metrics and logs.

### Eligibility Rules

Eligibility is expressed as a **guard function** that evaluates:
- Previous state completed successfully.
- Dependencies (other sessions, external systems) are satisfied.
- Resource constraints (budget, quota) are not exceeded.
- Temporal constraints (start‑after, deadline) are met.

Guards are pure functions of the session’s current evidence and system state.

### Bounded Retries

Each action has a **max‑retry count** (default: 3). Retries use exponential backoff with jitter.

After retries are exhausted, the session moves to a **blocked** state and an escalation rule is triggered (e.g., notify human, create a follow‑up ticket).

### Deterministic State Transitions

Transition logic is deterministic given:
- Session state (including all evidence items)
- System state (ledger balances, provider health, etc.)
- Current time (for timeouts and deadlines)

Non‑determinism (e.g., LLM output) is captured as **evidence** and becomes part of the session state, making the overall transition deterministic.

### Resume/Recovery

The orchestrator persists:
- Session state (including evidence)
- Retry counts
- Timeouts and deadlines
- Transition history

On restart, the orchestrator loads all **in‑progress** sessions and continues from their last recorded state. No work is lost.

### Timeout & Escalation

Each state has a **timeout** (wall‑clock duration). If the timeout expires before the exit guard is satisfied, the session moves to **blocked** and an escalation rule fires.

Escalation rules can:
- Notify a human (via Gatekeeper)
- Create a follow‑up ticket (in Jira or similar)
- Route to a different queue/provider
- Log an alert for operator intervention

### No Infinite Loops

Guarantees:
- **Step limit** – maximum number of state transitions per session (e.g., 100).
- **Wall‑clock limit** – maximum total duration per session (e.g., 7 days).
- **Termination detection** – sessions that cannot progress are moved to **blocked** and escalated.

## Implementation Notes

- The bounded orchestrator is part of the **Planner Agent** (Block 2.5).
- State transitions are logged as **evidence items** (type: `transition`).
- Timeouts are managed by a dedicated **timeout supervisor** that scans for stale sessions.
- Escalation rules are defined in **policy configuration** and evaluated by the ZenPolicy component.

## Configuration Example

```yaml
orchestrator:
  polling_interval_seconds: 30
  max_sessions_per_tick: 10
  default_timeout_minutes: 120
  retry:
    max_attempts: 3
    backoff_base_seconds: 2
    jitter_percent: 20
  escalation:
    - state: blocked
      action: create_followup_ticket
      params:
        project: "ZB"
        issue_type: "Task"
        assignee: "team-lead"
```

## Related Documents

- [Session Manager](../03-DESIGN/ZEN_JOURNAL.md) – session state and evidence storage.
- [Planner Agent](../03-DESIGN/BLOCK2_OFFICE.md) – overall orchestration.
- [Proof‑of‑Work](PROOF_OF_WORK.md) – evidence bundle format.