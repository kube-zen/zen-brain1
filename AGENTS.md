# Zen‑Brain – Model‑Facing Instructions

> **Advisory Only** – This file is a convenience for AI agents and developers. The canonical source of truth is the code (`internal/`, `pkg/`) and structured configuration (`configs/`). Do not place unique policy or business logic here.

## Project Overview

Zen‑Brain is an AI‑native orchestration platform that:

- **Coordinates work execution** across heterogeneous LLM providers (Ollama, OpenAI, Anthropic, etc.)
- **Implements SR&ED‑ready evidence collection** with attribution headers (V6 spec)
- **Provides human‑approval workflows** via the Gatekeeper component
- **Tracks costs and efficiency** via the ZenLedger
- **Isolates external systems** via Office connectors (Jira, GitHub, etc.)

## Repository Structure Summary

```
zen‑brain/
├── cmd/                         # Entrypoints (CLI, server)
├── internal/                    # Private implementation
│   ├── analyzer/               # Intent analyzer (Block 2.3)
│   ├── gatekeeper/             # Human gatekeeper (Block 2.6)
│   ├── office/                 # Office connectors (Block 2.1–2.2)
│   ├── planner/                # Planner agent (Block 2.5)
│   └── session/                # Session manager (Block 2.4)
├── pkg/                        # Public packages
│   ├── contracts/              # Core data types (WorkItem, Session, etc.)
│   ├── gate/                   # Admission control (ZenGate)
│   ├── ledger/                 # Cost‑tracking ledger (ZenLedger)
│   ├── llm/                    # LLM provider abstractions
│   ├── office/                 # Office connector interface (ZenOffice)
│   └── policy/                 # Policy evaluation (ZenPolicy)
├── configs/                    # Configuration examples
├── deployments/                # Kubernetes manifests
├── docs/                       # Documentation (numbered taxonomy)
├── scripts/                    # Python‑only scripts (no .sh)
└── api/                        # External API definitions
```

## Canonical Source‑of‑Truth Rule

- **Code** in `internal/` and `pkg/` is authoritative.
- **Configuration** in `configs/` defines the canonical config schema.
- **Contracts** in `pkg/contracts/` define the core data model.
- **Documentation** in `docs/` is the authoritative reference.
- **Source‑of‑Truth mapping**: See `docs/01‑ARCHITECTURE/SOURCE_OF_TRUTH.md` for complete ownership mapping.

Model‑facing files (`AGENTS.md`, `WORKFLOW.md`) are **advisory only** and must not contain unique policy or business logic.

## Running Tests & Gates

### Unit Tests
```bash
go test ./...                     # Run all Go unit tests
```

### Repository Hygiene Gates
Zen‑Brain enforces strict repo hygiene via Python gates:

```bash
# No shell scripts anywhere
python3 scripts/ci/no_shell_scripts_gate.py

# Python scripts must be under scripts/
python3 scripts/ci/python_script_placement_gate.py

# Docs must follow numbered taxonomy + UPPER_SNAKE_CASE
python3 scripts/ci/repo_layout_gate.py

# No executable sprawl outside scripts/
python3 scripts/ci/no_executable_sprawl_gate.py

# No large/binary files
python3 scripts/ci/no_binaries_gate.py

# Validate internal markdown links
python3 scripts/ci/docs_link_gate.py
```

Run all gates before committing. Gates are also enforced via the pre‑commit hook (see `.githooks/pre‑commit`).

## Updating Documentation When Contracts Change

When you modify core contracts (`pkg/contracts/`) or architecture:

1. **Update the source code** first.
2. **Update the corresponding documentation** in `docs/` (never the reverse).
3. **Follow naming conventions**: markdown files under `docs/` must use `UPPER_SNAKE_CASE.md`.
4. **Run the docs‑link gate** to verify internal links:
   ```bash
   python3 scripts/ci/docs_link_gate.py
   ```
5. **Update `docs/INDEX.md`** if you add a high‑level document.

## Jira‑Specific Details

Jira‑specific logic belongs exclusively in `internal/office/jira/`. Keep it out of:

- Factory layer (future Block 3)
- Core contracts (`pkg/contracts/`)
- LLM provider abstractions
- Admission‑control policies

The Office connector pattern isolates external‑system details. Use the `ZenOffice` interface and the Jira connector’s AI‑attribution headers (V6 spec).

## Skills & Subagents (Future)

Skills and subagents are planned but not yet implemented. When added, they will be:

- **Narrow and composable**
- **Backed by canonical config or code**
- **Never the only place where system‑critical behavior is defined**

## KB/QMD Direction (1.0)

- **Source of truth**: `zen‑docs` Git repository.
- **qmd**: Search/index only (not storage).
- **Confluence sync**: Optional / deferred unless explicitly enabled.
- **CockroachDB**: Not used as KB/QMD storage in 1.0.

---

> This file is advisory. Always prefer the canonical sources listed above.