# Zen-Brain 1.0

A production AI agent orchestration system for the Zen ecosystem.

## Overview

Zen-Brain provides intelligent task planning, execution, and evidence collection for AI-assisted software development. It integrates with Jira for human workflows and Kubernetes for scalable execution.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        ZenOffice                             │
│  (Jira Connector, Intent Analyzer, Planner, Gatekeeper)      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      ZenContext                              │
│  (Session State, Work Memory, Task Tracking)                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Factory                                │
│  (Kubernetes Execution, Worker Pools, Task Dispatch)         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      ZenJournal                              │
│  (Immutable Event Log, SR&ED Evidence)                       │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Build
make build

# Run tests
make test

# Run locally
make run
```

## Configuration

Zen-Brain uses a configurable home directory:

- Default: `~/.zen-brain/`
- Override: Set `ZEN_BRAIN_HOME` environment variable

## Development Status

**Current Phase:** Block 0 - Clean Foundation

See `/docs/architecture/CONSTRUCTION-PLAN.md` for the full build roadmap.

## License

Copyright 2026 Kube-Zen
