# Queue Steward Role

**Status:** Production
**Binary:** `cmd/queue-steward/`
**Template:** `config/task-templates/queue-steward-l1.yaml`
**Schedules:** `config/schedules/queue-steward-fast.yaml`, `queue-steward-summary.yaml`

## What It Is

The Queue Steward is an L1 factory-floor supervisor. It keeps the factory filled with ready work, detects stuck tickets, and produces structured queue-health artifacts. It does NOT make strategic decisions, change policy, or bypass validation gates.

## What It Does

1. **Gather queue snapshot** — Query Jira for all non-Done ticket states
2. **Call L1 for recommendations** — Feed snapshot to 0.8b model for structured JSON output
3. **Execute safe actions** — Dispatch, requeue, pause, or escalate based on recommendations
4. **Write artifacts** — `queue-health.json`, `queue-actions.json`, `queue-health.md`

## What It Does NOT Do

- Move tickets to Done (only validation/closure rules do that)
- Change worker counts or concurrency targets
- Override approval gates or quality thresholds
- Invent policy or change strategic priorities
- Bypass validation or evidence rules

## Allowed Transitions

| From | To | Condition |
|------|----|-----------|
| Backlog | Selected for Development | Ready ticket, factory underfilled |
| Selected for Development | In Progress | Ready ticket, factory underfilled |
| RETRYING | In Progress | Retry policy allows |
| In Progress | PAUSED | Stuck too long |
| In Progress | TO_ESCALATE | Exceeded retry limit |

## Schedules

| Schedule | Cadence | Purpose |
|----------|---------|---------|
| queue-steward-fast | 5 min | Queue hygiene, fill check, stuck detection |
| queue-steward-summary | 30 min | Queue-health summary artifact |

## Fill Rule

```
target_in_progress = min(safe_l1_concurrency, ready_backlog + retrying)
```

Underfilled factory with ready backlog is a bug. The steward exists to prevent that.

## Discovery Throttle

If ready backlog > `DISCOVERY_MAX` (default: 10), the steward flags discovery as throttled. The scheduler's `countBacklogTickets()` enforces this.

## Work Split

- 70% remediation / backlog drain
- 20% roadmap / office execution
- 10% discovery refresh / dedup

## Fallback

When L1 is unavailable, the steward uses deterministic heuristic recommendations:
- Recommend dispatch for ready tickets when factory is underfilled
- Recommend retry for RETRYING tickets
- No hallucinated actions — purely rule-based

## Artifacts

| File | Format | Purpose |
|------|--------|---------|
| `queue-health.json` | JSON | Machine-readable current state |
| `queue-actions.json` | JSON | Actions taken + L1 recommendations |
| `queue-health.md` | Markdown | Human-readable report |
| `queue-health-{run-id}.md` | Markdown | Per-run history |

All artifacts written to `/var/lib/zen-brain1/evidence/queue-steward/`.

## GLM-5 Role

GLM-5 stays in supervisor/policy role:
- Define and adjust policy (thresholds, work split, throttle rules)
- Inspect exceptions and edge cases
- Review metrics and trends
- Intervene on failures or repeated issues

GLM-5 does NOT do routine queue care. The Queue Steward does.
