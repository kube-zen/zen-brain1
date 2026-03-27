# Design Documents

This directory contains detailed design specifications for Zen‑Brain components.

## Core Components

| Document | Purpose | Status |
|----------|---------|--------|
| [Block 2 Office](BLOCK2_OFFICE.md) | Design of Office layer (Jira connector, intent analysis, planning) | **Complete** |
| [ZenContext](ZEN_CONTEXT.md) | Tiered memory system (Hot/Warm/Cold) | **Draft** |
| [ZenJournal](ZEN_JOURNAL.md) | Immutable event ledger with cryptographic chain hashes | **Draft** |
| [ZenLedger](ZEN_LEDGER.md) | Token and cost accounting with yield metrics | **Draft** |
| [ZenGate & ZenPolicy](ZEN_GATE_POLICY.md) | Admission control and declarative policy engine | **Draft** |
| [LLM Gateway](LLM_GATEWAY.md) | Provider‑agnostic LLM interface with intelligent routing | **Draft** |
| [Bounded Orchestrator Loop](BOUNDED_ORCHESTRATOR_LOOP.md) | State machine for task execution with bounded retries and resume/recovery | **Draft** |
| [Proof of Work](PROOF_OF_WORK.md) | Bundle format for AI work evidence (session IDs, intent, files, tests, logs) | **Draft** |
| [Skills and Subagents](SKILLS_AND_SUBAGENTS.md) | Design for bounded execution helpers under RoleProfile and policies | **Draft** |
| [Small Model Strategy](SMALL_MODEL_STRATEGY.md) | CPU-first local model lane (calibration, routing, benchmarking) | **Draft** |
| [Ops Department](OPS_DEPARTMENT.md) | Jira-centric ops model (incidents, changes, deploys, approvals) | **Draft** |
| [Agent Sandbox and Evaluation](SANDBOX_AND_EVALUATION.md) | Non-destructive evaluation lane for 1.1 (testing without production risk) | **Draft** |
| [Model-Facing Files and Skills Policy](MODEL_FACING_FILES_AND_SKILLS_POLICY.md) | Policy for advisory-only AGENTS.md/WORKFLOW.md and bounded skills/subagents | **Draft** |
| [Operating Model](OPERATING_MODEL.md) | 5-layer architecture (Leonardo → Board → Portfolio Office → Office → Factory) | **Active** |
| [Portfolio Office Layer](PORTFOLIO_OFFICE_LAYER.md) | Missing layer definition — strategy decomposition and release slicing | **Design** |
| [Roadmap Governance](ROADMAP_GOVERNANCE.md) | Version model (1.0/1.1/1.2/2.0) and roadmap decision process | **Active** |

## Upcoming Designs

- **ZenGuardian** – proactive monitoring and safety boundaries
- **Factory Execution** – warm worker pools, session affinity, worktree isolation
- **Knowledge Base Ingestion** – QMD integration, Confluence sync (optional)
- **Funding Evidence Aggregator** – SR&ED/IRAP report generation
- **Multi‑cluster Topology** – control plane / data plane communication

## Design Principles

All components adhere to the architectural principles outlined in the [Construction Plan](../01-ARCHITECTURE/CONSTRUCTION_PLAN.md):

1. **Jira is the human front door** – work originates in Jira, but the internal execution model uses canonical `WorkItem` types.
2. **ZenOffice is the abstraction boundary** – external system connectors live here; no Jira‑specific types leak into Factory or Planner.
3. **Git‑based knowledge base** – `zen‑docs` repository is the source of truth; qmd indexes it for search; Confluence is a one‑way published mirror (optional).
4. **SR&ED evidence collection default ON** – every action is recorded for funding‑ready audit trails.
5. **Multi‑cluster aware** – control plane, data plane agents, and workload placement across heterogeneous Kubernetes clusters.

## How to Use These Documents

- **Implementors** – follow the interface definitions and implementation notes.
- **Reviewers** – check for consistency with the overall architecture and other components.
- **Contributors** – when extending a component, update the corresponding design document.

## Contributing

When creating a new design document:

1. Start from the [ADR template](../01-ARCHITECTURE/ADR/TEMPLATE.md) for ADRs, or copy an existing design doc structure.
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

*These documents are living specifications; update as implementation progresses.*