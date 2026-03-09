# Workflow Overview (Advisory)

> **Advisory Only** – This file summarizes high‑level workflow concepts. The canonical source of truth is the code (`internal/`, `pkg/`) and structured configuration (`configs/`).

## Execution Lifecycle

1. **Ingress** – Work items arrive via Office connectors (Jira, GitHub, etc.).
2. **Analysis** – Intent analyzer breaks down work into BrainTaskSpecs.
3. **Planning** – Planner selects LLM models, routes tasks, enforces budget.
4. **Approval** – Human Gatekeeper approves/rejects sessions (cost/risk thresholds).
5. **Execution** – Factory (Block 3) schedules tasks across warm worker pools.
6. **Evidence** – Session manager collects SR&ED evidence (hypothesis, experiment, observation, etc.).
7. **Completion** – Work item status updated, results published back to source system.

## Proof‑of‑Work Expectations

Each completed session should produce a **proof‑of‑work bundle** containing:

- **Session identifier** and related work‑item keys
- **Summary of intent** and analysis results
- **Files changed** (diffs, new files, deletions)
- **Tests run** and their results
- **Lint/build results** (if applicable)
- **Artifacts/evidence references** (logs, metrics, traces)
- **Risks/open questions** identified during execution
- **Recommended next action** (if any)

Proof‑of‑work bundles are stored as part of the session’s evidence and can be used for audit, rollback, or continuation.

## Workspace / Worktree Expectations

- **Isolated workspaces** – Each concurrent task runs in its own Git worktree (branch `ai/{ticket‑key}`).
- **Clean‑state guarantee** – Worktrees start from a clean `origin/main` baseline.
- **No destructive resets** – `git reset --hard` is forbidden; it would destroy other AI’s work.
- **Branch‑based flow**:
  1. Create branch `ai/{ticket‑key}`
  2. Commit changes on that branch
  3. Push branch to origin
  4. Merge to main (fast‑forward or reviewed)
  5. Delete branch after successful merge

## Canonical References

- **Execution lifecycle**: `docs/03‑DESIGN/BLOCK2_OFFICE.md` (Office layer)
- **Proof‑of‑work**: `docs/03‑DESIGN/PROOF_OF_WORK.md` (when created)
- **Session management**: `docs/03‑DESIGN/ZEN_JOURNAL.md`
- **Cost tracking**: `docs/03‑DESIGN/ZEN_LEDGER.md`
- **Configuration**: `docs/04‑DEVELOPMENT/CONFIGURATION.md`
- **Source of truth mapping**: `docs/01‑ARCHITECTURE/SOURCE_OF_TRUTH.md`

## Repo Hygiene Rules

- **No shell scripts** – Python‑only scripts under `scripts/`.
- **Docs taxonomy** – All documentation under `docs/` in numbered directories.
- **UPPER_SNAKE_CASE** – Markdown filenames must follow this convention.
- **Pre‑commit gates** – Run repository hygiene gates before committing.

## Getting Started

1. Read the architecture overview: `docs/01‑ARCHITECTURE/CONSTRUCTION_PLAN.md`
2. Set up the development environment: `docs/04‑DEVELOPMENT/SETUP.md`
3. Explore the contracts: `docs/02‑CONTRACTS/DATA_MODEL.md`
4. Run the repo gates: `python3 scripts/ci/*.py`

---

> This file is advisory. Always refer to the canonical documentation and code.