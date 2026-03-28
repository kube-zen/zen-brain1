# Backlog Baseline — 2026-03-28

**Captured:** 2026-03-28 08:40 EDT
**Source:** Jira API `/rest/api/3/search/jql`

## 1. State Counts

| Status | Count |
|--------|-------|
| Backlog | 524 |
| Selected for Development | 0 |
| In Progress | 0 |
| PAUSED | 0 |
| RETRYING | 0 |
| TO_ESCALATE | 0 |
| Done | 0 |
| **Total** | **524** |

**Critical finding:** 524/524 in Backlog. Zero drain. Zero Done. The factory creates tickets but has never closed one.

## 2. Ticket Composition

### By Source/Label

| Category | Count | Notes |
|----------|-------|-------|
| `scheduled-batch` (batch parents) | ~77 | Container tickets for scan runs |
| `daily-sweep` findings | ~57 | Repeated daily scan outputs |
| `hourly-scan` findings | ~19 | Repeated hourly scan outputs |
| `quad-hourly-summary` findings | ~19 | Repeated quad-hourly outputs |
| `finding` (all) | ~400+ | Auto-generated reports |
| `defect` | 5 | Genuine defect tickets from bug-hunting |
| `discovery` | 1 | Single discovery ticket |

### By Finding Report Type (duplicates across runs)

| Report Type | Count | Most Recent |
|-------------|-------|-------------|
| bug_hunting | 20 | Many duplicates, same stubs |
| defects | 19 | Mostly same findings repeated |
| stub_hunting | 19 | Same stubs reported repeatedly |
| roadmap | 13 | Near-identical roadmaps |
| config_drift | 12 | Similar drift each time |
| tech_debt | 11 | Same debts re-reported |
| package_hotspots | 9 | Duplicate hotspots |
| test_gaps | 9 | Same gaps, different timestamps |
| dead_code | 8 | Often "zero dead code found" |
| executive_summary | 7 | Variations on same summary |
| bug-hunting.md (defect) | 5 | Specific defect extractions |

### Issue Types

- **Task:** 524/524 (100%)
- No epics, no stories, no subtasks

## 3. Bounded vs Unbounded Estimate

| Category | Bounded? | Count | Rationale |
|----------|----------|-------|-----------|
| Defect tickets (ZB-286..290) | **Yes** | 5 | Specific files, specific bugs |
| Latest batch of each report type | **Partially** | ~10-15 | Have artifact paths, could be triaged |
| Older duplicate reports | **No — stale** | ~400+ | Superseded by newer runs |
| Batch parent containers | **No — metadata** | ~77 | Just containers, not actionable work |

**Bounded estimate:** ~20 tickets max (5 defects + 15 latest findings)
**Unbounded/stale:** ~504 (bulk close candidates)

## 4. Execution-Ready Estimate

| Criterion | Count | Notes |
|-----------|-------|-------|
| Has target file(s) in description | 5 | Defect tickets only |
| Has artifact path | ~400 | But most are stale duplicates |
| Has validation command | 0 | None have validation commands |
| Has governance/compliance labels | 0 | Zero `sred:*`, `irap:*`, `governance:*` |
| Has `ai:*` labels | 0 | Zero `ai:finding`, `ai:remediated`, etc. |
| Has evidence links | 0 | None |

**Execution-ready estimate: 5 tickets** (the 5 defect tickets)
**Near-ready after dedup: ~10-15** (latest unique findings)

## 5. Immediate Drain Candidates

### Tier 1 — Directly Actionable (5 tickets)

| Key | Summary | Target | Bounded |
|-----|---------|--------|---------|
| ZB-286 | defect: cmd/main.go location | `cmd/main.go` | Yes |
| ZB-287 | defect: sync primitives in main.go | `cmd/main.go` | Yes |
| ZB-288 | defect: unlocked sync.WaitGroup | `cmd/main.go` | Yes |
| ZB-289 | defect: unlocked sync.Mutex | `cmd/main.go` | Yes |
| ZB-290 | defect: unlocked sync.RWMutex | `cmd/main.go` | Yes |

### Tier 2 — Bulk Action (close as stale/duplicate)

- ~77 batch parent tickets → close to Done (these are metadata containers)
- ~400+ duplicate finding reports → keep only latest per type, close rest to Done

### Tier 3 — Needs Triaging (latest unique findings)

- Latest test_gaps, config_drift, stub_hunting reports — need human triage to determine if actionable
- Most other reports (roadmap, executive_summary, tech_debt) are informational, not executable work

## 6. Key Operational Problems

1. **No drain at all** — 524/524 in Backlog, 0 in any other state
2. **Scanner spam** — scheduled scans create tickets but nothing consumes them
3. **No deduplication** — same findings reported dozens of times across runs
4. **No governance labels** — zero SR&ED/IRAP/governance metadata
5. **No `ai:*` workflow labels** — findings never entered the remediation pipeline
6. **Success metric inversion** — ticket creation rate >> ticket closure rate (infinite)

## 7. Recommended Immediate Actions

1. **Bulk-close stale batch parents and duplicate findings** — move ~480+ tickets to Done with comment "bulk close: stale/duplicate scanner output"
2. **Keep latest of each report type** for reference (1 each of ~10 types = ~10 tickets)
3. **Drain the 5 defect tickets** through L1 → validate → Done
4. **Add deduplication to the ticket creation pipeline** — don't create tickets for identical findings
5. **Throttle scanner ticket creation** — only create when new findings differ from previous run
