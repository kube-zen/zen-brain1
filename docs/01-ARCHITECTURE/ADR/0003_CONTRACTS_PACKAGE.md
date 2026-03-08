# ADR 0003: Create a neutral contracts package for canonical types

## Status

**Accepted** (2026‑03‑07)

## Context

Zen‑Brain follows the **Office + Factory** architectural pattern:
- **Office** (planning) receives work from external systems (Jira, Linear, etc.).
- **Factory** (execution) processes work using canonical work items.

To keep these domains decoupled, we need a **shared language** – data types that both sides agree on, but that belong to neither side. Placing these types in either the Office or Factory package would create an undesirable dependency:

- If types are defined in `pkg/office`, the Factory would depend on Office (violating separation).
- If types are defined in `pkg/factory`, the Office would depend on Factory (also wrong).

Additionally, other components (ZenJournal, ZenLedger, ZenContext, ZenPolicy, ZenGate, ZenFunding) need to reference the same canonical types.

## Decision

Create a neutral `pkg/contracts` package that contains **all canonical data types** used across Zen‑Brain:

```go
// pkg/contracts/contracts.go
package contracts

// Enums: WorkType, WorkDomain, Priority, ExecutionMode, WorkStatus, EvidenceRequirement, SREDTag, ApprovalState
// Structs: AIAttribution, SourceMetadata, ExecutionConstraints, WorkTags, WorkItem, Comment, Attachment
```

The package has **no dependencies** on other zen‑brain packages (except standard library). It defines only data structures, no behavior.

All other packages import `pkg/contracts`:

- `pkg/office` – uses `contracts.WorkItem` as the output of `ZenOffice.FetchTicket()`.
- `pkg/journal` – uses `contracts.SREDTag` for event tagging.
- `pkg/ledger` – uses `contracts.WorkItem` fields for cost attribution.
- `pkg/context` – stores `contracts.WorkItem` in session state.
- `pkg/policy`, `pkg/gate`, `pkg/funding` – all reference `contracts` types.
- `api/v1alpha1` – CRDs embed `contracts.SREDTag` and other enums.

**Rule:** No component‑specific types live in `pkg/contracts`. If a type is only used by one component, it belongs in that component’s package.

## Consequences

### Positive

- **Clean decoupling** – Office and Factory both depend on `contracts`, not on each other.
- **Single source of truth** – All components use the same definitions; no translation or mapping between similar types.
- **Compile‑time safety** – Changes to contracts are immediately visible to all dependents; mismatches are caught early.
- **Simplified serialization** – JSON/YAML serialization uses the same struct tags everywhere.
- **Easier evolution** – When the data model evolves, we update one package and fix compilation errors across the codebase.

### Negative

- **Centralized change impact** – A breaking change in `contracts` forces updates in many packages.
- **Potential for bloating** – The package could accumulate types that should be component‑specific.
- **Circular dependency risk** – If `contracts` imports any other zen‑brain package, the decoupling breaks.

### Neutral

- The package is **internal to Zen‑Brain**; external systems never see these types directly.
- Office connectors map external types (Jira issues, Linear tickets) to `contracts` types.

## Alternatives Considered

### 1. Define types in each package and convert between them

- **Pros**: Each package owns its types; no central dependency.
- **Cons**: Conversion boilerplate, risk of mismatched fields, harder to maintain consistency.

### 2. Use protobuf/gRPC for interface definitions

- **Pros**: Language‑neutral, versioning support, efficient serialization.
- **Cons**: Overhead for a single‑language project, extra tooling, less readable Go code.

### 3. Put types in `pkg/office` and have Factory depend on Office

- **Pros**: Simple, fewer packages.
- **Cons**: Violates architectural separation; Factory shouldn’t depend on Office.

### 4. Use separate `pkg/types` package (same as `contracts`)

- **Pros**: Same benefits as `contracts`.
- **Cons**: The name “types” is generic; “contracts” better conveys the purpose (agreement between components).

The `contracts` package is the clear winner for enforcing architectural boundaries while maintaining type consistency.

## Related Decisions

- [ADR‑0001](0001‑structured‑tags.md) – Structured tags are defined in `contracts.WorkTags`.
- [ADR‑0002](0002‑sred‑taxonomy.md) – SREDTag enum is defined in `contracts`.

## References

- Construction Plan V6.0, Section “Block 1: The Neuro‑Anatomy”
- `pkg/contracts/contracts.go` – canonical type definitions
- `go.mod` – no internal dependencies