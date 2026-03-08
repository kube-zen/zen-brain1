# Zen‑Brain – Index

## Overview
Zen‑Brain is an AI‑native orchestration platform that coordinates work execution across heterogeneous LLM providers, with built‑in SR&ED evidence collection and human‑approval workflows.

## Repository Layout

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
│   ├── contracts/              # Core data types
│   ├── gate/                   # Admission control
│   ├── ledger/                 # Cost‑tracking ledger
│   ├── llm/                    # LLM provider abstractions
│   ├── office/                 # Office connector interface
│   └── policy/                 # Policy evaluation
├── configs/                    # Configuration examples
├── deployments/                # Kubernetes manifests
├── docs/                       # Documentation (numbered taxonomy)
├── scripts/                    # Python‑only scripts (no .sh)
└── api/                        # External API definitions
```

## Quick Links

- [README.md](README.md) – Project introduction
- [AGENTS.md](AGENTS.md) – Model‑facing instructions (advisory)
- [CONTRIBUTING.md](CONTRIBUTING.md) – Contribution guidelines
- [docs/](docs/) – Full documentation

## Canonical Source of Truth

- **Code** in `internal/` and `pkg/` is the authoritative implementation.
- **Configuration** in `configs/` is the canonical config schema.
- **Contracts** in `pkg/contracts/` define the core data model.
- **Documentation** in `docs/` is the authoritative reference.

Model‑facing files (`AGENTS.md`, `WORKFLOW.md`) are advisory only and must not contain unique policy or business logic.

## Running Tests & Gates

```bash
# Run all unit tests
go test ./...

# Run repository hygiene gates
python3 scripts/ci/no_shell_scripts_gate.py
python3 scripts/ci/python_script_placement_gate.py
python3 scripts/ci/repo_layout_gate.py
python3 scripts/ci/no_executable_sprawl_gate.py
python3 scripts/ci/no_binaries_gate.py
python3 scripts/ci/docs_link_gate.py
```

See `scripts/ci/` for the full suite of repo‑hygiene gates.

## Updating Documentation

When contracts or architecture change:

1. Update the relevant `.go` source files in `pkg/contracts/` or `internal/`.
2. Update the corresponding documentation in `docs/` (never the reverse).
3. Ensure markdown files follow the UPPER_SNAKE_CASE naming convention.
4. Run the docs‑link gate to verify internal links.

## Jira‑Specific Details

Jira‑specific logic belongs in `internal/office/jira/` and must not leak into the Factory layer or core contracts. The Office connector pattern isolates external‑system details.

---

> This file is advisory. The canonical source of truth is the code and structured config.