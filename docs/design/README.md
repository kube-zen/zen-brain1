# Design Documents

This directory contains detailed design specifications for Zen‑Brain components.

## Core Components

| Document | Purpose | Status |
|----------|---------|--------|
| [Block 2 Office](block2‑office.md) | Design of the Office layer (Jira connector, intent analysis, planning) | **Complete** |
| [ZenContext](zen‑context.md) | Tiered memory system (Hot/Warm/Cold) | **Draft** |
| [ZenJournal](zen‑journal.md) | Immutable event ledger with cryptographic chain hashes | **Draft** |
| [ZenLedger](zen‑ledger.md) | Token and cost accounting with yield metrics | **Draft** |
| [ZenGate & ZenPolicy](zen‑gate‑policy.md) | Admission control and declarative policy engine | **Draft** |
| [LLM Gateway](llm‑gateway.md) | Provider‑agnostic LLM interface with intelligent routing | **Draft** |

## Upcoming Designs

- **ZenGuardian** – proactive monitoring and safety boundaries
- **Factory Execution** – warm worker pools, session affinity, worktree isolation
- **Knowledge Base Ingestion** – QMD integration, Confluence sync
- **Funding Evidence Aggregator** – SR&ED/IRAP report generation
- **Multi‑cluster Topology** – control plane / data plane communication

## Design Principles

All components adhere to the architectural principles outlined in the [Construction Plan](../architecture/CONSTRUCTION‑PLAN.md):

1. **Jira is the human front door** – work originates in Jira, but the internal execution model uses canonical `WorkItem` types.
2. **ZenOffice is the abstraction boundary** – external system connectors live here; no Jira‑specific types leak into Factory or Planner.
3. **Git‑based knowledge base** – `zen‑docs` repository is the source of truth; qmd indexes it for search; Confluence is a one‑way published mirror.
4. **SR&ED evidence collection default ON** – every action is recorded for funding‑ready audit trails.
5. **Multi‑cluster aware** – control plane, data plane agents, and workload placement across heterogeneous Kubernetes clusters.

## How to Use These Documents

- **Implementors** – follow the interface definitions and implementation notes.
- **Reviewers** – check for consistency with overall architecture and other components.
- **Contributors** – when extending a component, update the corresponding design document.

## Contributing

When creating a new design document:

1. Start from the [template](../architecture/adr/template.md) for ADRs, or copy an existing design doc structure.
2. Include:
   - Overview and purpose
   - Interface definitions
   - Data structures
   - Implementation details (storage, configuration, monitoring)
   - Integration points
   - Open questions
3. Mark the status as **Draft** until reviewed.
4. Submit a pull request linking to the relevant construction plan block.

---

*These documents are living specs; update as implementation progresses.*