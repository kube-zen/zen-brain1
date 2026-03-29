# Phase B — Existing Capabilities Evidence

**Date:** 2026-03-28
**Status:** Validated, no new code needed

## What Already Existed

### 1. Per-Schedule WORKERS Override

**Code path:** `cmd/scheduler/main.go` lines 224–236

```go
// Per-schedule WORKERS override (Phase B).
defaultWorkers := 5
if envW := os.Getenv("WORKERS_OVERRIDE"); envW != "" {
    if n, err := strconv.Atoi(envW); err == nil && n > 0 {
        defaultWorkers = n
    }
}
effectiveWorkers := defaultWorkers
if s.Workers > 0 {
    effectiveWorkers = s.Workers
}
log.Printf("[SCHED] %s: effective WORKERS=%d (schedule=%d, default=%d)",
    s.Name, effectiveWorkers, s.Workers, defaultWorkers)
```

**Schedule struct** (line 46):
```go
type Schedule struct {
    Name        string   `yaml:"name"`
    Tasks       []string `yaml:"tasks"`
    Cadence     string   `yaml:"cadence"`
    Description string   `yaml:"description"`
    Workers     int      `yaml:"workers"`
}
```

**Resolution chain:** `s.Workers` from YAML → env `WORKERS_OVERRIDE` → hardcoded default 5

### 2. Backlog / Discovery Throttle

**Code path:** `cmd/scheduler/main.go` lines 295–315

```go
ticketizableSchedules := map[string]bool{
    "hourly-scan": true,
    "daily-sweep": true,
}
if ticketizableSchedules[s.Name] && jiraCfg.enabled {
    backlogReady, _ := countBacklogTickets(jiraCfg)
    if backlogReady > 10 {
        log.Printf("[SCHED] %s: DISCOVERY THROTTLED — backlog has %d ready tickets (> 10 threshold). Skipping ticketizer.",
            s.Name, backlogReady)
    } else {
        log.Printf("[SCHED] %s: discovery allowed — backlog has %d ready tickets (<= 10 threshold). Running ticketizer.",
            s.Name, backlogReady)
        runFindingTicketizer(runDir, s.Name, jiraCfg)
    }
}
```

**Helper function:** `countBacklogTickets(jiraCfg)` (line 415)
- JQL: `project="{key}" AND status=Backlog AND labels=bug AND labels=ai:finding`
- Returns `(ready, total)` count
- Threshold: >10 ready = skip discovery

## What Was Changed This Phase

| Item | Change | Reason |
|------|--------|--------|
| `config/schedules/hourly-scan.yaml` | Cleaned to single `workers: 5` | Was duplicated from prior session |
| `config/schedules/daily-sweep.yaml` | Cleaned to single `workers: 7` | Was duplicated from prior session |
| `config/schedules/quad-hourly-summary.yaml` | Cleaned to single `workers: 5` | Was duplicated from prior session |
| Docs updates | OBSERVABILITY_AND_THROUGHPUT.md, BACKLOG_DRAIN_MODE.md | Aligned with validated behavior |

## What Was NOT Changed (Already Existed)

- `cmd/scheduler/main.go` — per-schedule WORKERS override logic (lines 224–236)
- `cmd/scheduler/main.go` — discovery throttle logic (lines 295–315)
- `cmd/scheduler/main.go` — `countBacklogTickets()` function (lines 415–445)
- `cmd/scheduler/main.go` — `Schedule.Workers` struct field (line 53)

## Config Source of Truth

| Schedule | File | Workers |
|----------|------|---------|
| hourly-scan | `config/schedules/hourly-scan.yaml` | 5 |
| quad-hourly-summary | `config/schedules/quad-hourly-summary.yaml` | 5 |
| daily-sweep | `config/schedules/daily-sweep.yaml` | 7 |

## Operating Statements

- Per-schedule WORKERS override already existed in code
- Backlog throttle already existed in code
- This phase is about validation, config cleanup, and measured behavior
- W=7 is the only approved concurrency step in this phase
- W=10 remains unapproved
