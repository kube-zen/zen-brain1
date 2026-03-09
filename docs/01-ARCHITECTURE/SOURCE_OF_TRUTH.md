# Source of Truth

This document defines where canonical truth resides in the Zen‑Brain project. It clarifies which artifacts are authoritative and which are derived or advisory.

## Core Principle

**Canonical truth is the artifact that must be changed to effect a permanent, correct change to the system.** Derived artifacts are generated from canonical sources and may be overwritten.

## Hierarchy of Truth

### 1. Code (`internal/`, `pkg/`)

The Go source code is the ultimate source of truth for runtime behavior.

| Path | Purpose | Canonical For |
|------|---------|---------------|
| `pkg/` | Public interfaces and contracts | API boundaries, data types, interfaces |
| `internal/` | Private implementations | Business logic, component behavior |
| `api/v1alpha1/` | CRD definitions | Kubernetes resource schemas |
| `cmd/` | Entry points | Binary behavior and flags |

**Rule:** If the code says one thing and a document says another, the code is correct. Update the document.

### 2. Structured Configuration (`configs/`)

YAML and JSON configuration templates define deploy‑time behavior.

| Path | Purpose | Canonical For |
|------|---------|---------------|
| `configs/config.dev.yaml` | Local development configuration | Local environment defaults |
| `configs/config.prod.yaml` | Production configuration | Production deployment defaults |
| `deployments/` | Kubernetes manifests | Deployment topology and resource limits |

**Rule:** Configuration is authoritative for deploy‑time choices; code should not hardcode values that belong in configuration.

### 3. Architecture and Design Documentation (`docs/01‑ARCHITECTURE/`, `docs/03‑DESIGN/`)

Documents that describe why the system is built a certain way.

| Path | Purpose | Canonical For |
|------|---------|---------------|
| `docs/01‑ARCHITECTURE/CONSTRUCTION_PLAN.md` | Master build roadmap | Overall construction sequence and principles |
| `docs/01‑ARCHITECTURE/ADR/` | Architecture Decision Records | Major design decisions with context and consequences |
| `docs/01‑ARCHITECTURE/GLOSSARY.md` | Terminology definitions | Terms used across the project |
| `docs/03‑DESIGN/` | Component design specifications | Detailed behavior of each component (ZenContext, ZenJournal, etc.) |

**Rule:** These documents are authoritative for intent and design. They explain why the code is shaped as it is. When the code diverges from the design, either update the code or revise the design (with an ADR).

### 4. Contracts and Data Models (`docs/02‑CONTRACTS/`)

Structured definitions of data types and interfaces that cross component boundaries.

| Path | Purpose | Canonical For |
|------|---------|---------------|
| `docs/02‑CONTRACTS/DATA_MODEL.md` | Canonical data types and tags | WorkItem, WorkType, SREDTag, AIAttribution, etc. |
| `pkg/contracts/` | Go types for contracts | Programmatic representation of the data model |

**Rule:** The Go types in `pkg/contracts/` are the executable source of truth; the markdown document is a human‑friendly rendering. Keep them synchronized.

### 5. Knowledge Base (`zen‑docs` repository)

The `zen‑docs` Git repository is the source of truth for all project documentation, policies, and procedures.

**Rule:** Documentation in `zen‑docs` is canonical for project knowledge. The QMD adapter indexes it for search; Confluence is a one‑way published mirror (optional). Never edit documentation directly in Confluence—edit `zen‑docs` and let the sync propagate.

### 6. Advisory Model‑Facing Documents (`AGENTS.md`, `WORKFLOW.md`)

These files are **not** source of truth. They are derived, advisory summaries for AI agents and developers.

| File | Purpose | Status |
|------|---------|--------|
| `AGENTS.md` | Instructions for AI agents working on the codebase | Advisory only – canonical truth is in code and config |
| `WORKFLOW.md` | High‑level workflow concepts and examples | Advisory only – canonical truth is in design docs and code |

**Rule:** These files may be updated automatically by AI agents or manually by developers, but they must never contain unique policy or business logic. They should always point to the canonical sources listed above.

### 7. Repository Governance Rules (`docs/04‑DEVELOPMENT/REPO_RULES.md`)

The rules enforced by CI gates are canonical for repository structure and hygiene.

**Rule:** The gates in `scripts/ci/` enforce these rules; the document describes them. If a gate fails, consult `REPO_RULES.md` for the rationale and fix guidance.

## Change Propagation

When a canonical source changes, derived artifacts must be updated:

| Change In | Update Required In |
|-----------|-------------------|
| `pkg/contracts/*.go` | `docs/02‑CONTRACTS/DATA_MODEL.md`, any generated code |
| `docs/01‑ARCHITECTURE/CONSTRUCTION_PLAN.md` | README, CONTRIBUTING, other docs that reference it |
| `zen‑docs` repository | QMD index, Confluence mirror (if enabled) |
| `configs/` | Deployment manifests, environment‑specific overrides |

## Verification

CI gates verify that:

1. Internal documentation links point to existing files (`docs_link_gate.py`)
2. Repository layout follows the taxonomy (`repo_layout_gate.py`)
3. Model‑facing documents remain advisory (`model_facing_policy_gate.py`)
4. No SDK‑owned concerns are reimplemented (`zen_sdk_ownership_gate.py`)
5. The canonical construction plan is unique and referenced correctly (`canonical_plan_gate.py`)
6. KB/QMD direction stays Git‑first (`kb_qmd_direction_gate.py`)

Run `make repo‑check` before submitting changes to ensure you haven’t inadvertently violated a source‑of‑truth rule.

## Rationale

Clear separation of canonical truth from derived artifacts prevents confusion, reduces drift, and ensures that changes are made in the right place. This discipline is essential for a project that uses AI agents extensively—agents need to know which sources are authoritative and which are summaries.

---

*See also:* `docs/04‑DEVELOPMENT/REPO_RULES.md` – repository governance enforced by CI gates.