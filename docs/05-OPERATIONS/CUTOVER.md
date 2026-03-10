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
- [x] Block 0.5: SDK package audit (see [BLOCK0_5_ZEN_SDK.md](../01-ARCHITECTURE/BLOCK0_5_ZEN_SDK.md); mandatory reuse satisfied; dlq/observability/leader/logging/events/crypto documented as deferred)
- [ ] Block 1: Neuro-Anatomy
