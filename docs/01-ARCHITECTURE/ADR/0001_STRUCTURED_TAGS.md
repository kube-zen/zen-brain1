# ADR 0001: Use structured tags instead of flat labels

## Status

**Accepted** (2026‑03‑07)

## Context

In early versions of Zen‑Brain (and many other systems), work items were annotated with a flat list of labels (`[]string`). This led to several problems:

1. **Tag sprawl** – Without governance, the number of distinct labels grows uncontrollably.
2. **Ambiguous semantics** – The same label could be used for different purposes (e.g., `"auth"` could mean a team, a domain, a technology, or a security requirement).
3. **Difficulty in automation** – Systems that need to interpret tags (e.g., routing, policy evaluation, analytics) must guess the intent of each label.
4. **Jira‑driven taxonomy drift** – Because Jira’s label system is flat and untyped, the internal taxonomy tends to mirror Jira’s arbitrary labels, leaking Jira‑specific concepts into the core execution model.

The problem becomes acute when Zen‑Brain needs to make automated decisions based on tags (e.g., “route this task to the GPU worker pool”, “require approval for production‑affecting changes”, “include this work in SR&ED evidence collection”).

## Decision

Replace the flat `Labels []string` field with a structured `WorkTags` model that organizes tags into **categories**:

```go
type WorkTags struct {
    HumanOrg  []string   `json:"human_org,omitempty"`   // Epics, teams, quarters
    Routing   []string   `json:"routing,omitempty"`     // System routing decisions
    Policy    []string   `json:"policy,omitempty"`      // ZenGate policy evaluation
    Analytics []string   `json:"analytics,omitempty"`   // Dashboards and reporting
    SRED      []SREDTag  `json:"sred,omitempty"`        // Typed SR&ED categories
}
```

Each category has a defined purpose and a controlled vocabulary (see `pkg/taxonomy/tags.go` for examples). The `SRED` category uses a typed enum (`SREDTag`) to ensure only valid SR&ED uncertainty categories are used.

All canonical types (including `WorkItem`) now use `WorkTags` instead of `Labels`. The `pkg/contracts` package contains the canonical definitions, and the `pkg/taxonomy` package provides validation and helper functions.

## Consequences

### Positive

- **Clear semantics** – Every tag has a known purpose (human organization, routing, policy, analytics, or SR&ED).
- **Automation‑ready** – Systems can rely on the category to interpret tags without guessing.
- **Prevents Jira leakage** – Jira’s flat labels are mapped to structured categories at the ZenOffice boundary; the Factory never sees raw Jira labels.
- **Enables SR&ED typing** – SR&ED categories are typed (`SREDTag`), ensuring only valid uncertainty areas are used.
- **Better validation** – The `taxonomy` package can validate that a tag belongs to its claimed category.
- **Improved analytics** – Dashboards can slice by category (e.g., show all work with `Policy:prod‑affecting`).

### Negative

- **Mapping complexity** – Office connectors must map external labels to the appropriate categories (e.g., decide whether a Jira label is a `HumanOrg` or `Routing` tag).
- **Migration overhead** – Existing work items with flat labels need to be migrated (not a concern for Zen‑Brain 1.0, which starts fresh).
- **Slightly larger payload** – The structured model adds a few bytes per work item compared to a flat list.

### Neutral

- The change is internal; external systems (Jira, Linear, Slack) continue to use their native label/tag systems.
- Office connectors are responsible for the mapping; the mapping rules can be configured per project.

## Alternatives Considered

### 1. Keep flat labels but add a prefix convention (e.g., `"routing:gpu‑required"`)

- **Pros**: Simpler to implement, backward compatible.
- **Cons**: Still ambiguous (no validation), parsing overhead, easy to make mistakes, no type safety for SR&ED.

### 2. Separate fields for each category (e.g., `HumanOrgTags`, `RoutingTags`, etc.)

- **Pros**: Explicit, no need for a wrapper struct.
- **Cons**: Bloats the `WorkItem` struct, harder to extend with new categories.

### 3. Use a map of categories (e.g., `map[string][]string`)

- **Pros**: Flexible, easy to add new categories at runtime.
- **Cons**: No type safety, harder to validate, serialization/deserialization more complex.

The structured `WorkTags` struct strikes a balance between explicitness, type safety, and extensibility.

## Related Decisions

- [ADR‑0002](0002_SRED_TAXONOMY.md) – Define SR&ED uncertainty categories as a typed enum.
- [ADR‑0003](0003_CONTRACTS_PACKAGE.md) – Create a neutral `pkg/contracts` package for canonical types.

## References

- Construction Plan, Section “Canonical Work Taxonomy and Jira Mapping”
- `pkg/contracts/contracts.go` – `WorkTags` definition
- `pkg/taxonomy/tags.go` – Tag categories and validation