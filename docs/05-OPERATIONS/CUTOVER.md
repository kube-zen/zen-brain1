# Cutover Documentation

This document tracks the transition from `zen-brain` (prototype) to `zen-brain1` (production).

## Repository Information

| Property | Value |
|----------|-------|
| Repository | `github.com/kube-zen/zen-brain1` |
| Local Path | `~/zen/zen-brain1` |
| Git Remote | `git@github.com:kube-zen/zen-brain1.git` |
| Go Module | `github.com/kube-zen/zen-brain1` |

## Construction Plan

The master construction plan is located at:
```
/home/neves/Downloads/ZEN-BRAIN-1.0-CONSTRUCTION-PLAN-V6.md
```

Key sections:
- Block 0: Clean Foundation (current)
- Block 0.5: Pre-requisite SDK Packages
- Block 1: Neuro-Anatomy (schemas, interfaces)
- Block 2: Office (ZenOffice, Jira connector)
- Block 3: Nervous System (Message bus, ZenJournal, KB, ZenLedger)
- Block 4: Factory (K8s execution)
- Block 5: Intelligence (QMD, ReMe, Funding reports)
- Block 6: Developer Experience (k3d cluster)

## Directory Structure

```
zen-brain1/
├── api/v1alpha1/          # CRD definitions
├── cmd/zen-brain/         # Main entrypoint
├── pkg/                   # Public packages
│   ├── office/            # ZenOffice interface
│   ├── context/           # ZenContext interface
│   ├── journal/           # ZenJournal interface
│   └── llm/               # LLM Gateway interface
├── internal/              # Private implementation
│   ├── factory/           # Factory implementation
│   ├── connector/         # Office connectors (Jira, etc.)
│   └── config/            # Configuration, home paths
├── docs/                  # Documentation
├── deployments/           # K8s manifests
├── go.mod
├── Makefile
└── README.md
```

## Configuration

Home directory: `~/.zen-brain/` (override with `ZEN_BRAIN_HOME`)

Standard paths:
- `~/.zen-brain/journal/` - ZenJournal event logs
- `~/.zen-brain/context/` - Session state
- `~/.zen-brain/cache/` - Ephemeral data
- `~/.zen-brain/config/` - Configuration files
- `~/.zen-brain/logs/` - Application logs

## Related Projects

| Project | Location | Purpose |
|---------|----------|---------|
| zen-sdk | `~/zen/zen-sdk` | Shared Go library with reusable packages |
| zen-docs | `~/zen/zen-docs` | Documentation system |
| zen-lock | `~/zen/zen-lock` | Lock management service |
| zen-flow | `~/zen/zen-flow` | Workflow orchestration |
| zen-watcher | `~/zen/zen-watcher` | Watch service |

## Milestones

**Block 0 (Foundation) — complete.** No open tasks; all sub-blocks done.

- [x] Block 0.1: GitHub repo created
- [x] Block 0.2: Directory scaffold
- [x] Block 0.3: Configurable home directory (`ZEN_BRAIN_HOME`, `internal/config/home.go`, `paths.go`)
- [x] Block 0.4: Cutover documentation (this doc)
- [x] Block 0.5: SDK package audit (see [COMPLETENESS_MATRIX.md](../01-ARCHITECTURE/COMPLETENESS_MATRIX.md) and Block 0.5 in [CONSTRUCTION_PLAN.md](../01-ARCHITECTURE/CONSTRUCTION_PLAN.md); reuse ~95%, contract in good shape; DLQ/observability/leader/logging/events/crypto deferred — low risk, backlog not done-done)
- [x] Block 1: Neuro-Anatomy (see [ROADMAP.md](../01-ARCHITECTURE/ROADMAP.md): 1.1 ZenJournal, 1.2 ZenContext tiers, 1.3 SessionManager, 1.4 Agent/Planner, 1.5 Redis/S3, 1.6 Config, 1.7 Integration tests)
- [x] Block 2: Office (ZenOffice, Jira connector, Intent Analyzer, Session Manager, Planner, Gatekeeper; see [COMPLETENESS_MATRIX.md](../01-ARCHITECTURE/COMPLETENESS_MATRIX.md))
- [x] Block 3: Nervous System (Message bus, ZenJournal, API server, KB/QMD, ZenLedger, CockroachDB; see [COMPLETENESS_MATRIX.md](../01-ARCHITECTURE/COMPLETENESS_MATRIX.md))
- [x] Block 4: Factory (CRDs, Foreman, Worker, FactoryTaskRunner, real git worktree when configured, proof-of-work, review:real, ZenGate policy default, ZenGuardian log/circuit-breaker, metrics; optional next steps in [COMPLETENESS_MATRIX.md](../01-ARCHITECTURE/COMPLETENESS_MATRIX.md) and [PROGRESS.md](../01-ARCHITECTURE/PROGRESS.md))
- [x] Block 6: Developer Experience (k3d cluster, make dev-up/dev-image, in-cluster Foreman + API server via [deployments/k3d/](../../deployments/k3d/README.md))
