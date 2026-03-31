# ADR-0010: API Group Migration to *.zen-mesh.io

## Status

**Approved** | PATCHSET A in progress | PATCHSET C held pending review

## Context

zen-brain1 CRDs currently use two legacy API groups:
- `zen.kube-zen.com` — the current primary group for all 6 CRDs
- `zenbrain.kube-zen.io` — a typo-era artifact used only by TaskSession

The company has migrated from `kube-zen.com` / `kube-zen.io` to `zen-mesh.io`.
The API groups should follow.

Additionally, a broader portfolio audit identified 21 CRDs across 7 repos
(zen-brain1, zen-platform, zen-watcher, zen-flow, zen-lead, zen-lock, zen-gc)
all using `*.kube-zen.*` groups. This ADR addresses zen-brain1;
the full portfolio plan is in `docs/01-ARCHITECTURE/ADR/0010_API_GROUP_MIGRATION_PORTFOLIO.md`.

**Key safety factor:** Zero live CRD objects exist in the analyzed cluster.
This eliminates the hardest migration risk (object re-creation / data loss).

## Decision

### Target Taxonomy

All CRDs migrate to `*.zen-mesh.io` subdomains:

| Group | Kinds | Ownership |
|-------|-------|-----------|
| `brain.zen-mesh.io` | BrainTask, BrainQueue, BrainAgent, BrainPolicy | Brain workload orchestration |
| `platform.zen-mesh.io` | ZenProject, ZenCluster | Platform inventory / cross-product metadata |
| `mesh.zen-mesh.io` | (future: routing, ingestion, workflow from other repos) | Data/edge plane |

### zen-brain1 Mapping

| Kind | Current Group | Target Group | Scope |
|------|---------------|-------------|-------|
| BrainTask | `zen.kube-zen.com` | `brain.zen-mesh.io` | Namespaced |
| BrainQueue | `zen.kube-zen.com` | `brain.zen-mesh.io` | Namespaced |
| BrainAgent | `zen.kube-zen.com` | `brain.zen-mesh.io` | Namespaced |
| BrainPolicy | `zen.kube-zen.com` | `brain.zen-mesh.io` | Cluster |
| ZenProject | `zen.kube-zen.com` | `platform.zen-mesh.io` | Namespaced |
| ZenCluster | `zen.kube-zen.com` | `platform.zen-mesh.io` | Namespaced |
| TaskSession | `zenbrain.kube-zen.io` | **Retire** | Namespaced |

### Why `platform.zen-mesh.io` for ZenCluster

ZenCluster tracks endpoint, auth ref, capacity, location, heartbeat.
The reconciler does simple lifecycle/status — no mesh networking, no tunnels,
no service mesh data plane. This is platform inventory, not mesh control-plane.

### Why bare `zen-mesh.io` is avoided

Without a subdomain prefix, `zen-mesh.io` becomes a junk-drawer group.
Using `brain.`, `mesh.`, `platform.` prefixes forces clear ownership boundaries
as the API surface grows.

### Why package split by domain

Current structure puts all 6 kinds in `api/v1alpha1/` with one shared `GroupName`.
After migration, BrainTask and ZenCluster would share a package but have different
API groups — confusing for codegen, scheme registration, and RBAC.

Target structure:
- `api/brain/v1alpha1/` — BrainTask, BrainQueue, BrainAgent, BrainPolicy
- `api/platform/v1alpha1/` — ZenProject, ZenCluster

### TaskSession Disposition

- No Go types (YAML-only CRD)
- No controller watching it
- No live objects
- RBAC rule exists but is dead code
- **Action:** Delete CRD, RBAC rule, and source file during PATCHSET C

### Deprecated Groups

The following groups must NOT appear in new active code:

| Group | Status |
|-------|--------|
| `zen.kube-zen.io` | **DEPRECATED** — historical typo-era group |
| `zen.kube-zen.com` | **DEPRECATED** — old company domain |
| `zenbrain.kube-zen.io` | **DEPRECATED** — TaskSession artifact |

CI guardrails (`scripts/ci/guardrail_api_groups.py`) enforce this.

## Migration Phases

### PATCHSET A — Governance / Guardrails / Safe Label Transition (current)

Safe, non-breaking, can commit:
- ADR and migration documentation
- CI guardrails to block new `*.kube-zen.*` usage
- Deprecation notices in source and docs
- Compatibility-safe migration of 2 operationally significant label keys:
  - `zen.kube-zen.com/reported-to-jira` → `brain.zen-mesh.io/reported-to-jira`
  - `zen.kube-zen.com/planned-model` → `brain.zen-mesh.io/planned-model`

### PATCHSET B — Non-breaking Prep (held)

Requires approval to commit:
- Package directory restructuring (`api/v1alpha1/` → `api/brain/v1alpha1/` + `api/platform/v1alpha1/`)
- Import path updates (code still uses old groups temporarily)
- Test infrastructure refactoring
- Factory-* annotation migration (11 keys, deferred unless blockers)

### PATCHSET C — Breaking API Cutover (held)

Requires explicit approval:
- Group name changes in Go types
- CRD YAML group updates
- RBAC apiGroups updates
- Controller/scheme/webhook changes
- Cluster CRD apply/delete
- Old group removal

## Consequences

### Positive
- API groups align with company domain (`zen-mesh.io`)
- Clear domain ownership via subdomain prefixes
- Package structure matches API groups
- CI enforcement prevents future drift
- Zero live objects = zero data migration risk

### Negative
- Package split requires import path changes across ~15 files
- Cross-repo migration (zen-platform, etc.) is a larger effort
- Label key migration requires compatibility period for existing objects

### Broader Portfolio Notes

- zen-platform is the highest-risk repo (4 legacy groups, 70+ hardcoded strings)
- Webhooks at risk: zen-flow, zen-gc, zen-lock
- zen-watcher shares CRDs with zen-platform (must migrate together)
- Full portfolio plan: `docs/01-ARCHITECTURE/ADR/0010_API_GROUP_MIGRATION_PORTFOLIO.md`

## Approval Boundary

- PATCHSET A: Approved (this document)
- PATCHSET B: Requires approval before commit
- PATCHSET C: Requires explicit approval before any breaking change
- Cross-repo work: Requires approval per repo
