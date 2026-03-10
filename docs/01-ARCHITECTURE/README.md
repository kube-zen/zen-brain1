# Zen-Brain Architecture

This directory contains architecture documentation.

## Key Documents

- **[Source of Truth](SOURCE_OF_TRUTH.md)** – Where canonical truth resides (CRDs, Git, Jira, DB, model-facing docs)
- **[Construction Plan](CONSTRUCTION_PLAN.md)** – Master build roadmap
- **[Roadmap](ROADMAP.md)** – Prioritized roadmap with 1.0 must-have, 1.1 radar, and explicitly deferred items
- **[Recommended Next Steps](RECOMMENDED_NEXT_STEPS.md)** – Wave-based execution order (cleanup, governance, deployment, Block 5)
- **[Control Plane Vocabulary](CONTROL_PLANE_VOCABULARY.md)** – First-class control-plane objects: `ZenRoleProfile`, `ZenExecutionPolicy`, `ZenHandoffPolicy`, `ZenTool`, `ZenToolBinding`, `ZenComplianceProfile`, workspace classes, trust levels
- **[Glossary](GLOSSARY.md)** – Definitions of terms, components, and processes
- **[KB/QMD Strategy](KB_QMD_STRATEGY.md)** – How documentation is stored, searched, and published (git + qmd, Confluence optional)
- **[Project Structure](PROJECT_STRUCTURE.md)** – Directory layout and package organization

## Architecture Decision Records (ADRs)

See the [ADR index](ADR/README.md) for all architecture decisions.

- **[ADR Index](ADR/README.md)** – All ADRs with status and descriptions
- **[ADR Template](ADR/TEMPLATE.md)** – Template for creating new ADRs

## Design Documentation

See the [Design directory](../03-DESIGN/) for detailed component design specifications.

## Development Documentation

See the [Development directory](../04-DEVELOPMENT/) for practical guides for developers.

## Operating Philosophy

Zen-Brain follows the **Office + Factory** architectural pattern:
- **Jira is the human front door** – work originates in Jira, but the internal execution model uses canonical `WorkItem` types.
- **ZenOffice is the abstraction boundary** – external system connectors live here; no Jira‑specific types leak into Factory or Planner.
- **Git‑based knowledge base** – `zen‑docs` repository is the source of truth; qmd indexes it for search; Confluence is a one‑way published mirror (optional).
- **SR&ED evidence collection default ON** – every action is recorded for funding‑ready audit trails.
- **Multi‑cluster aware** – control plane, data plane agents, and workload placement across heterogeneous Kubernetes clusters.

## Related Documentation

- [Design Documents](../03-DESIGN/)
- [Contracts](../02-CONTRACTS/)
- [Development Guide](../04-DEVELOPMENT/)
- [Examples](../06-EXAMPLES/)
- [Root README](../../README.md)