# Zen‑Brain 1.0

A production AI agent orchestration system for the Zen ecosystem.

## Overview

Zen‑Brain provides intelligent task planning, execution, and evidence collection for AI‑assisted software development. It integrates with Jira for human workflows, Kubernetes for scalable execution, and a Git‑based knowledge base for contextual retrieval.

## Architecture

### Core Principles

- **Jira is the human front door** – work originates in Jira, but the internal execution model uses canonical `WorkItem` types.
- **ZenOffice is the abstraction boundary** – external system connectors live here; no Jira‑specific types leak into Factory or Planner.
- **Git‑based knowledge base** – `zen‑docs` repository is the source of truth; qmd indexes it for search; Confluence is a one‑way published mirror (optional).
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

## Quick Start

```bash
# Clone the repository
git clone git@github.com:kube-zen/zen-brain1.git
cd zen-brain1

# Build the binary
make build

# Run locally
make run
```

## Configuration

Zen‑Brain uses a configurable home directory:

- Default: `~/.zen‑brain/`
- Override: Set `ZEN_BRAIN_HOME` environment variable

## Dependencies

### zen-sdk (Required)

Zen-Brain 1.0 depends on [zen-sdk](https://github.com/kube-zen/zen-sdk) v0.3.0 or later for cross-cutting concerns:

| Package | Version | Purpose |
|---------|---------|---------|
| receiptlog | v0.3.0+ | Core for ZenJournal (immutable event ledger) |
| scheduler | v0.3.0+ | KB ingestion scheduling |
| dedup | v0.3.0+ | Message bus deduplication |
| dlq | v0.3.0+ | Dead letter queue |
| observability | v0.3.0+ | OpenTelemetry tracing |
| retry | v0.3.0+ | Exponential backoff with jitter |
| events | v0.3.0+ | Kubernetes event recording |
| leader | v0.3.0+ | Leader election for HA components |
| logging | v0.3.0+ | Structured logging |
| health | v0.3.0+ | Health and readiness probes |
| crypto | v0.3.0+ | Age encryption for secrets and credentials |

**Installation:**

```bash
go get github.com/kube-zen/zen-sdk@v0.3.0
```

**Usage:**

```go
import (
    "github.com/kube-zen/zen-sdk/pkg/receiptlog"
    "github.com/kube-zen/zen-sdk/pkg/scheduler"
    "github.com/kube-zen/zen-sdk/pkg/crypto"
)
```

### External Dependencies

- **Kubernetes client-go** (v0.35.0): For cluster operations and custom resources
- **Redis** (v9.18.0): For Tier 1 hot memory (session context)
- **Controller-runtime** (v0.19.0): For custom resource controllers

## Documentation

Comprehensive documentation is available in the `docs/` directory:

- **[Construction Plan](docs/01-ARCHITECTURE/CONSTRUCTION_PLAN.md)** – Master build roadmap (V6.1)
- **[Architecture Decision Records (ADRs)](docs/01-ARCHITECTURE/ADR/)** – Key design decisions with context and consequences
- **[Data Model](docs/02-CONTRACTS/DATA_MODEL.md)** – Canonical types and structured tags
- **[Knowledge Base & QMD Strategy](docs/01-ARCHITECTURE/KB_QMD_STRATEGY.md)** – How documentation is stored, searched, and published
- **[Project Structure](docs/01-ARCHITECTURE/PROJECT_STRUCTURE.md)** – Directory layout and package organization
- **[Glossary](docs/01-ARCHITECTURE/GLOSSARY.md)** – Definitions of terms, components, and processes
- **[Block 2 Office Design](docs/03-DESIGN/BLOCK2_OFFICE.md)** – Detailed design for the Jira connector and AI attribution
- **[Component Design Documents](docs/03-DESIGN/)** – Detailed specifications for core components:
  - [ZenContext](docs/03-DESIGN/ZEN_CONTEXT.md) – Tiered memory system (Hot/Warm/Cold)
  - [ZenJournal](docs/03-DESIGN/ZEN_JOURNAL.md) – Immutable event ledger with cryptographic chain hashes
  - [ZenLedger](docs/03-DESIGN/ZEN_LEDGER.md) – Token and cost accounting with yield metrics
  - [ZenGate & ZenPolicy](docs/03-DESIGN/ZEN_GATE_POLICY.md) – Admission control and declarative policy engine
  - [LLM Gateway](docs/03-DESIGN/LLM_GATEWAY.md) – Provider‑agnostic LLM interface with intelligent routing
- **[Configuration Reference](docs/04-DEVELOPMENT/CONFIGURATION.md)** – All configurable options across components
- **[Development Setup](docs/04-DEVELOPMENT/SETUP.md)** – Step‑by‑step guide to set up a local development environment
- **[Workflow Examples](docs/06-EXAMPLES/WORKFLOW_EXAMPLES.md)** – Illustrated end‑to‑end workflows (Jira → PR, incident response, documentation)
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

See `docs/01-ARCHITECTURE/CONSTRUCTION_PLAN.md` for the full build roadmap.

## License

Copyright 2026 Kube‑Zen
