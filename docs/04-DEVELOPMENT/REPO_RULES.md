# Repository Governance Rules

This document describes the repository governance rules enforced by CI gates. All contributors must follow these rules to maintain code quality, architectural consistency, and direction.

## Summary

Zen‑Brain uses a set of automated CI gates that run on every commit and pull request. These gates ensure the repository stays aligned with the architectural direction and does not accumulate technical debt. The gates are defined in `scripts/ci/` and can be run locally with `make repo‑check`.

## Gate Categories

### 1. Documentation Structure

**Rule:** All documentation must be placed under `docs/` using the numbered taxonomy:

- `01‑ARCHITECTURE/` – High‑level architecture, ADRs, glossary
- `02‑CONTRACTS/` – Canonical data models and interfaces
- `03‑DESIGN/` – Component design specifications
- `04‑DEVELOPMENT/` – Development guides, configuration, setup
- `05‑OPERATIONS/` – Operations and deployment guides
- `06‑EXAMPLES/` – Example workflows and use cases
- `99‑ARCHIVE/` – Deprecated or archived documentation

**Enforcement:** `repo_layout_gate.py`

- Markdown files under `docs/` must use `UPPER_SNAKE_CASE.md` filenames (except `README.md`, `INDEX.md`).
- Root‑level markdown files are limited to:
  - `README.md`
  - `INDEX.md`
  - `AGENTS.md`
  - `WORKFLOW.md`
  - `CONTRIBUTING.md`
- No ad‑hoc status or completion notes at root—move them to `docs/99‑ARCHIVE/`.

### 2. Canonical Construction Plan

**Rule:** There is exactly one canonical construction plan file: `docs/01‑ARCHITECTURE/CONSTRUCTION_PLAN.md`. All references in documentation must point to this file.

**Enforcement:** `canonical_plan_gate.py`

- No other construction‑plan‑like files may exist.
- No broken symlinks pointing to construction plans.
- README, CONTRIBUTING, and docs index files must link to the canonical plan.

### 3. SDK Ownership

**Rule:** Cross‑cutting concerns must come from `zen‑sdk`. Local reimplementation of SDK‑owned packages is forbidden unless explicitly allowed by an ADR.

**SDK‑owned concerns include:**
- `receiptlog` / event ledger foundation
- `dedup`
- `dlq`
- `retry`
- `observability`
- `health`
- `leader` election
- generic `logging`
- generic `crypto` helpers
- `scheduler`
- `events`

**Enforcement:** `zen_sdk_ownership_gate.py`

- Directories under `internal/` or `pkg/` must not match SDK package names.
- Files that implement SDK‑like functionality are flagged unless allowlisted in `scripts/ci/zen_sdk_allowlist.txt`.

### 4. KB/QMD Direction

**Rule:** The knowledge base (KB) uses Git as the source of truth, with QMD as a vector search adapter. CockroachDB is a vector store implementation detail, not the default QMD path.

**Allowed default language:**
- Git source of truth
- qmd adapter
- qmd refresh/orchestration
- optional Confluence mirror later

**Disallowed default language:**
- CockroachDB‑backed QMD as 1.0 default
- custom graph‑backed KB as 1.0 default
- Any phrasing that makes a database the primary source of truth.

**Enforcement:** `kb_qmd_direction_gate.py`

- Scans documentation and source code for disallowed phrasing.
- Allowlisted files (e.g., the construction plan) may mention CockroachDB as an implementation detail.

### 5. Model‑Facing File Policy

**Rule:** `AGENTS.md` and `WORKFLOW.md` are advisory model‑facing convenience documents. They are derived summaries, not canonical source of truth.

**Canonical truth lives in:**
- Code (`internal/`, `pkg/`)
- Structured configuration (`configs/`)
- Architecture and design documentation (`docs/01‑ARCHITECTURE/`, `docs/03‑DESIGN/`)

**Enforcement:** `model_facing_policy_gate.py`

- Flags language that frames `AGENTS.md` or `WORKFLOW.md` as canonical, definitive, or authoritative.
- Allows language that explicitly states they are advisory.

### 6. Script Language and Placement

**Rule:** All scripts must be Python‑only and reside under `scripts/`. No shell scripts (`.sh`) are allowed anywhere in the repository.

**Enforcement:** `no_shell_scripts_gate.py`, `python_script_placement_gate.py`

- Executable Python scripts must be under `scripts/` (exceptions: test files, Go entrypoints under `cmd/`).
- No `.sh` files may be committed.
- Executable files outside `scripts/` are prohibited (see `EXEC_OUTSIDE_SCRIPTS_ALLOWLIST.txt` for exceptions).

### 7. Binary and Executable Sprawl

**Rule:** Avoid committing large binary files (>5 MiB) and executable binaries (ELF/Mach‑O/PE) unless they are explicitly allowed test fixtures.

**Enforcement:** `no_executable_sprawl_gate.py`, `no_binaries_gate.py`

- Executable files may only reside under `scripts/` (with allowlist).
- Large files trigger warnings.
- Binary executables are forbidden.

### 8. Internal Documentation Links

**Rule:** All internal markdown links must be valid (point to existing files).

**Enforcement:** `docs_link_gate.py`

- Broken links cause CI failure.
- Helps keep documentation navigable.

## Running Gates Locally

You can run the gates locally before committing:

```bash
# Run all repo‑hygiene gates
make repo-check

# Or run the CI runner directly
python3 scripts/ci/run.py default
```

Gate suites:
- `default` – all repo‑hygiene gates
- `governance` – repo layout, shell scripts, Python placement, executable sprawl
- `docs` – docs layout and internal links
- `binaries` – large/binary file checks
- `all` – every gate

## Adding New Gates

When the architecture direction evolves, new gates may be needed. To add a gate:

1. Create a Python script in `scripts/ci/` following the existing pattern.
2. Add the gate to `scripts/ci/run.py` in the `GATES` dictionary.
3. Decide which suites should include it and update `SUITES`.
4. Document the rule in this file.
5. Test the gate locally and ensure it passes on the current codebase (use allowlists if necessary).

## Allowlists

Some gates support allowlists to grandfather existing violations or approve necessary exceptions:

- `scripts/ci/zen_sdk_allowlist.txt` – allowed local SDK‑like implementations
- `scripts/ci/kb_qmd_allowlist.txt` – files allowed to mention CockroachDB
- `scripts/EXEC_OUTSIDE_SCRIPTS_ALLOWLIST.txt` – executable files allowed outside `scripts/`

Allowlist entries must be justified with a comment. Prefer fixing the violation over adding to the allowlist.

## Failure Messages

When a gate fails, it prints a clear error message pointing to this document. Fix the violation or—if the violation is intentional—update the appropriate allowlist and ensure an ADR exists to justify the exception.

## Philosophy

These gates exist to preserve architectural integrity, not to burden developers. They encode decisions that are expensive to change later, ensuring the repository stays aligned with the Zen‑Brain 1.0 vision.

---

*See also:* `docs/01‑ARCHITECTURE/SOURCE_OF_TRUTH.md` – where canonical truth lives.