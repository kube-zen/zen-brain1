# ADR 0002: Define SR&ED uncertainty categories as a typed enum

## Status

**Accepted** (2026‑03‑07)

## Context

SR&ED (Scientific Research and Experimental Development) tax credits require documenting **scientific or technological uncertainty**. To qualify, work must address a specific uncertainty that cannot be resolved by standard engineering practice.

In Zen‑Brain, we want to:
1. **Collect SR&ED evidence by default** for all eligible work.
2. **Categorize uncertainties** to streamline evidence collection and reporting.
3. **Ensure type safety** so that only valid SR&ED categories can be assigned.

A flat list of strings (e.g., `["dynamic‑provisioning", "security‑gates"]`) would be error‑prone and hard to validate.

## Decision

Define a typed enum `SREDTag` in the `pkg/contracts` package:

```go
type SREDTag string

const (
    SREDU1DynamicProvisioning  SREDTag = "u1_dynamic_provisioning"
    SREDU2SecurityGates        SREDTag = "u2_security_gates"
    SREDU3DeterministicDelivery SREDTag = "u3_deterministic_delivery"
    SREDU4Backpressure         SREDTag = "u4_backpressure"
    SREDExperimentalGeneral    SREDTag = "experimental_general"
)
```

The enum is used in:
- `WorkTags.SRED []SREDTag` – typed SR&ED categorization of work items.
- `ZenProject.SREDTags []SREDTag` – project‑level SR&ED eligibility.
- `ZenJournal` experiment‑class events – linking events to specific uncertainty areas.
- `ZenFunding` reports – filtering evidence by SR&ED category.

Each constant has a clear description (see `pkg/taxonomy/tags.go` for mapping).

## Consequences

### Positive

- **Type safety** – Only valid SREDTag values can be assigned; invalid strings cause compilation errors.
- **Documentation** – Each constant is self‑documenting; IDE autocomplete shows available options.
- **Validation simplicity** – Checking if a tag is a valid SR&ED category is a simple map lookup.
- **Consistency** – All components (Office, Factory, Journal, Funding) use the same typed values.
- **Future‑proof** – New categories can be added as new constants; existing code handles them automatically (if using switch statements with defaults).

### Negative

- **Less flexible** – Cannot add new SR&ED categories at runtime without recompiling.
- **Migration required** – If categories change in the future, database values may need updating (though the enum values are stable strings).

### Neutral

- The enum values are strings, so they serialize to JSON as readable values (e.g., `"u1_dynamic_provisioning"`).
- External systems (Jira, Confluence) see the string values; they don’t need to understand the enum.

## Alternatives Considered

### 1. Use plain strings with a validation function

- **Pros**: Runtime flexibility, no recompilation needed to add new categories.
- **Cons**: No compile‑time checking, easy to make typos, validation overhead.

### 2. Use integer constants

- **Pros**: More efficient storage and comparison.
- **Cons**: Not human‑readable in JSON, requires mapping to/from strings, harder to debug.

### 3. Use a protobuf enum

- **Pros**: Language‑neutral, efficient binary serialization.
- **Cons**: Overkill for a single‑language codebase, adds protobuf dependency.

The typed string enum provides a good balance of readability, type safety, and simplicity.

## Related Decisions

- [ADR‑0001](0001_STRUCTURED_TAGS.md) – Structured tags model (SRED is one category).
- [ADR‑0003](0003_CONTRACTS_PACKAGE.md) – Centralized canonical types.

## References

- Construction Plan, Section “SR&ED/IRAP Alignment Design”
- `pkg/contracts/contracts.go` – `SREDTag` definition
- `pkg/taxonomy/tags.go` – `SREDTagDescription` mapping