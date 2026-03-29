# Factory Fill Dashboard

Updated: 2026-03-29T08:40:54-04:00

| Metric | Value |
|--------|-------|
| Backlog (ready) | 0 |
| Backlog (total) | 0 |
| Retrying | 0 |
| In Progress | 0 |
| Safe Target | 5 |
| Actual Active | 0 |
| Done this run | 18 |
| Failed this run | 21 |

✅ Factory utilization OK

## Operating Policy
- Underfilled factory with backlog present = BUG
- target_in_progress = min(safe_target, ready_backlog + retrying)
- Jira In Progress reflects actual active work
- Success = done-rate + honest attribution
