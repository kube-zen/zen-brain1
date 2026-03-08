# Zen-Brain 1.0

A production AI agent orchestration system for the Zen ecosystem.

## Overview

Zen-Brain provides intelligent task planning, execution, and evidence collection for AI‑assisted software development. It integrates with Jira for human workflows, Kubernetes for scalable execution, and a Git‑based knowledge base for contextual retrieval.

## Architecture

### Core Principles

- **Jira is the human front door** – work originates in Jira, but the internal execution model uses canonical `WorkItem` types.
- **ZenOffice is the abstraction boundary** – external system connectors live here; no Jira‑specific types leak into Factory or Planner.
- **Git‑based knowledge base** – `zen‑docs` repository is the source of truth; qmd indexes it for search; Confluence is a one‑way published mirror.
- **SR&ED evidence collection default ON** – every action is recorded for funding‑ready audit trails.
- **Multi‑cluster aware** – control plane, data plane agents, and workload placement across heterogeneous Kubernetes clusters.

### Component Map

```
┌─────────────────────────────────────────────────────────────────┐
│                         ZenOffice                                │
│   (Jira connector, intent analyzer, planner, gatekeeper)         │
│   Maps external issues → canonical WorkItem                      │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                         ZenContext                               │
│   (Session state, work memory, task tracking)                    │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Factory                                 │
│   (Kubernetes execution, worker pools, task dispatch)            │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                        ZenJournal                                │
│   (Immutable event log, SR&ED evidence)                          │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                        ZenLedger                                 │
│   (Token/cost accounting, value‑per‑token metrics)               │
└─────────────────────────────────────────────────────────────────┘
```

### Knowledge Base / QMD Strategy (1.0)

- **Source of truth**: `zen‑docs` Git repository
- **Search/index**: qmd over `zen‑docs` (CLI with JSON output, no MCP required)
- **Human publishing**: one‑way sync from `zen‑docs` → Confluence
- **Jira integration**: tickets link to KB scopes and docs; planner uses scopes to narrow retrieval
- **No CockroachDB for KB/QMD in 1.0** – CockroachDB holds structured runtime data (ZenLedger, session state, policies), not the document corpus.

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

Zen‑Brain uses a configurable home directory:

- Default: `~/.zen‑brain/`
- Override: Set `ZEN_BRAIN_HOME` environment variable

## Documentation

Comprehensive documentation is available in the `docs/` directory:

- **[Construction Plan](docs/architecture/CONSTRUCTION‑PLAN.md)** – Master build roadmap (V6.0)
- **[Architecture Decision Records (ADRs)](docs/01-ARCHITECTURE/ADR/)** – Key design decisions with context and consequences
- **[Data Model](docs/data‑model.md)** – Canonical types and structured tags
- **[Knowledge Base & QMD Strategy](docs/kb‑qmd.md)** – How documentation is stored, searched, and published
- **[Project Structure](docs/project‑structure.md)** – Directory layout and package organization
- **[Glossary](docs/glossary.md)** – Definitions of terms, components, and processes
- **[Block 2 Office Design](docs/design/block2‑office.md)** – Detailed design for the Jira connector and AI attribution
- **[Component Design Documents](docs/design/)** – Detailed specifications for core components:
  - [ZenContext](docs/design/zen‑context.md) – Tiered memory system (Hot/Warm/Cold)
  - [ZenJournal](docs/design/zen‑journal.md) – Immutable event ledger with cryptographic chain hashes
  - [ZenLedger](docs/design/zen‑ledger.md) – Token and cost accounting with yield metrics
  - [ZenGate & ZenPolicy](docs/design/zen‑gate‑policy.md) – Admission control and declarative policy engine
  - [LLM Gateway](docs/design/llm‑gateway.md) – Provider‑agnostic LLM interface with intelligent routing
- **[Configuration Reference](docs/04-DEVELOPMENT/CONFIGURATION.md)** – All configurable options across components
- **[Development Setup](docs/04-DEVELOPMENT/SETUP.md)** – Step‑by‑step guide to set up a local development environment
- **[Workflow Examples](docs/workflow‑examples.md)** – Illustrated end‑to‑end workflows (Jira → PR, incident response, documentation)
- **[Contributing Guide](CONTRIBUTING.md)** – Development workflow, coding standards, and testing

## Development Status

**Current Phase:** Block 1 – Schema Hardening Complete

- ✅ Block 0: Clean foundation (scaffold, go.mod, Makefile, README)
- ✅ Block 0.5: SDK package audit (26 reusable packages in `zen‑sdk`)
- ✅ Block 1: Neuro‑Anatomy – canonical contracts, structured tags, SR&ED taxonomy, multi‑cluster CRDs
- 🚧 Block 2: Office – Jira connector with AI attribution (in progress)
- 📋 Block 3: Nervous System – message bus, ZenJournal, KB, ZenLedger, DB provisioning
- 📋 Block 4: Factory – K8s execution, warm workers, multi‑cluster agent
- 📋 Block 5: Intelligence – QMD, ReMe, Funding Evidence Aggregator
- 📋 Block 6: Developer Experience – k3d cluster, local‑first workflow

See `docs/architecture/CONSTRUCTION‑PLAN.md` for the full build roadmap.

## License

Copyright 2026 Kube‑Zen