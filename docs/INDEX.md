# Documentation Index

## Taxonomy

Zen‑Brain documentation is organized into numbered directories:

| Directory | Purpose |
|-----------|---------|
| [`01‑ARCHITECTURE/`](01-ARCHITECTURE/) | High‑level architecture, ADRs, glossary, project structure |
| [`02‑CONTRACTS/`](02-CONTRACTS/) | Core data models, type definitions, interfaces |
| [`03‑DESIGN/`](03-DESIGN/) | Component design, detailed specifications |
| [`04‑DEVELOPMENT/`](04-DEVELOPMENT/) | Setup, configuration, development workflow |
| [`05‑OPERATIONS/`](05-OPERATIONS/) | Deployment, monitoring, cutover procedures |
| [`06‑EXAMPLES/`](06-EXAMPLES/) | Example workflows, sample data, tutorials |
| [`99‑ARCHIVE/`](99-ARCHIVE/) | Deprecated or historical documents |

## Key Documents

### Architecture
- [SOURCE_OF_TRUTH.md](01-ARCHITECTURE/SOURCE_OF_TRUTH.md) – Where canonical truth resides (CRDs, Git, Jira, DB, model-facing docs)
- [CONSTRUCTION_PLAN.md](01-ARCHITECTURE/CONSTRUCTION_PLAN.md) – Overall construction plan
- [PROJECT_STRUCTURE.md](01-ARCHITECTURE/PROJECT_STRUCTURE.md) – Repository layout rules
- [GLOSSARY.md](01-ARCHITECTURE/GLOSSARY.md) – Terminology
- [KB_QMD_STRATEGY.md](01-ARCHITECTURE/KB_QMD_STRATEGY.md) – Knowledge‑base strategy
- [ADR/](01-ARCHITECTURE/ADR/) – Architecture Decision Records

### Contracts
- [DATA_MODEL.md](02-CONTRACTS/DATA_MODEL.md) – Core data types and relationships

### Design
- [BLOCK2_OFFICE.md](03-DESIGN/BLOCK2_OFFICE.md) – Office layer design (Blocks 2.1‑2.6)
- [LLM_GATEWAY.md](03-DESIGN/LLM_GATEWAY.md) – LLM provider gateway design
- [ZEN_CONTEXT.md](03-DESIGN/ZEN_CONTEXT.md) – Context management design
- [ZEN_GATE_POLICY.md](03-DESIGN/ZEN_GATE_POLICY.md) – Admission‑control design
- [ZEN_JOURNAL.md](03-DESIGN/ZEN_JOURNAL.md) – Journal/evidence design
- [ZEN_LEDGER.md](03-DESIGN/ZEN_LEDGER.md) – Cost‑ledger design

### User Guides
- [VERTICAL_SLICE.md](04-DEVELOPMENT/VERTICAL_SLICE.md) – Complete vertical slice guide with examples

### Development
- [VERTICAL_SLICE.md](04-DEVELOPMENT/VERTICAL_SLICE.md) – Complete vertical slice guide with examples
- [TEST_COVERAGE.md](04-DEVELOPMENT/TEST_COVERAGE.md) – Test coverage summary and test categories
- [TRUSTWORTHY_VERTICAL_SLICE_COMPLETE.md](04-DEVELOPMENT/TRUSTWORTHY_VERTICAL_SLICE_COMPLETE.md) – Complete vertical slice implementation report
- [SETUP.md](04-DEVELOPMENT/SETUP.md) – Development environment setup

### Testing

- [CUTOVER.md](05-OPERATIONS/CUTOVER.md) – Cutover/migration procedures

### Examples
- [WORKFLOW_EXAMPLES.md](06-EXAMPLES/WORKFLOW_EXAMPLES.md) – Example workflows
- [jira‑workitem‑example.json](06-EXAMPLES/jira-workitem-example.json) – Sample Jira work item
- [zen‑docs‑tree.txt](06-EXAMPLES/zen-docs-tree.txt) – Example documentation tree

## Naming Convention

All markdown files under `docs/` (except `README.md` and `INDEX.md`) must use **UPPER_SNAKE_CASE.md** names.

Examples:
- `PROJECT_STRUCTURE.md` ✅
- `DATA_MODEL.md` ✅
- `project‑structure.md` ❌ (lowercase)
- `ProjectStructure.md` ❌ (camel case)

## Updating Documentation

1. Place new documents in the appropriate numbered directory.
2. Name the file using UPPER_SNAKE_CASE.md.
3. Update this index if adding a high‑level document.
4. Run the docs‑link gate (`python3 scripts/ci/docs_link_gate.py`) to verify internal links.

## Model‑Facing Files

Root‑level `AGENTS.md` and `WORKFLOW.md` are advisory only. The canonical source of truth is the code and the structured documentation under `docs/`.

---

> This index is maintained manually. Please keep it up‑to‑date when adding or removing documents.