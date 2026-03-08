# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) for Zen‑Brain 1.0. ADRs document significant architectural decisions, the context that led to them, and their consequences.

## What is an ADR?

An ADR is a lightweight document that captures:
- **Context** – What problem are we solving?
- **Decision** – What did we decide?
- **Consequences** – What are the benefits, drawbacks, and implications?
- **Alternatives** – What other options were considered?

ADRs are **living documents**. They can be superseded or deprecated as the system evolves.

## ADR Format

We use a variant of the [MADR](https://adr.github.io/madr/) template. Each ADR is a Markdown file with the following sections:

- **Title** – Short present‑tense imperative phrase
- **Status** – Proposed | Accepted | Superseded | Deprecated
- **Context** – The problem statement
- **Decision** – The chosen solution
- **Consequences** – Positive, negative, and neutral outcomes
- **Alternatives Considered** – Other options that were evaluated
- **Related Decisions** – Links to other ADRs
- **References** – Links to code, documentation, or external resources

See [template.md](template.md) for the full template.

## ADR Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [0001](0001‑structured‑tags.md) | Use structured tags instead of flat labels | Accepted | 2026‑03‑07 |
| [0002](0002‑sred‑taxonomy.md) | Define SR&ED uncertainty categories as a typed enum | Accepted | 2026‑03‑07 |
| [0003](0003‑contracts‑package.md) | Create a neutral contracts package for canonical types | Accepted | 2026‑03‑07 |
| [0004](0004‑multi‑cluster‑crds.md) | Design multi‑cluster topology with ZenProject and ZenCluster CRDs | Accepted | 2026‑03‑07 |
| [0005](0005‑ai‑attribution‑jira.md) | Inject AI attribution headers in all Jira content | Accepted | 2026‑03‑07 |
| [0006](0006‑warm‑worker‑pools.md) | Use warm worker pools with session affinity and git worktrees | Accepted | 2026‑03‑07 |
| [0007](0007‑qmd‑for‑knowledge‑base.md) | Use qmd for knowledge base search with git as source of truth | Accepted | 2026‑03‑07 |

## Creating a New ADR

1. Copy `template.md` to a new file `XXXX‑short‑title.md` (where `XXXX` is the next sequential number).
2. Fill in the sections with clear, concise prose.
3. Set the status to **Proposed**.
4. Open a pull request for discussion.
5. Once consensus is reached, update the status to **Accepted** and merge.

## Updating an ADR

- To **supersede** an ADR, create a new ADR that references the old one and change the old ADR’s status to **Superseded**.
- To **deprecate** an ADR, change its status to **Deprecated** and add a note explaining why.

## Why ADRs?

- **Document institutional knowledge** – Why did we choose this approach?
- **Avoid repeating discussions** – Point to the ADR when the same question arises.
- **Onboard new contributors** – ADRs explain the architectural rationale.
- **Track evolution** – See how decisions change over time.

## References

- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) (Michael Nygard)
- [MADR – Markdown Architectural Decision Records](https://adr.github.io/madr/)
- [Architecture Decision Records – GitHub Blog](https://github.blog/2020-08-13-why-write-adrs/)