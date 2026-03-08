# ADR 0004: Design multi‑cluster topology with ZenProject and ZenCluster CRDs

## Status

**Accepted** (2026‑03‑07)

## Context

Zen‑Brain 1.0 must operate across multiple Kubernetes clusters from day one, supporting:
- **Control plane / data plane separation** – A single control plane managing multiple data plane clusters.
- **Project isolation** – Different projects (zen‑brain, zen‑mesh, client‑work) may run on different clusters.
- **Heterogeneous infrastructure** – Mix of local k3d clusters, cloud‑managed Kubernetes, and edge clusters.
- **Cluster‑aware routing** – Tasks must be dispatched to the appropriate cluster based on project configuration.

Kubernetes Custom Resource Definitions (CRDs) provide a natural way to model this topology, leveraging the same API machinery used by the rest of the system.

## Decision

Define two CRDs in the `zen.kube‑zen.com/v1alpha1` API group:

### 1. ZenProject

Represents a logical project (e.g., zen‑brain, zen‑mesh) with its configuration and runtime state.

```go
type ZenProjectSpec struct {
    DisplayName                 string               `json:"display_name"`
    ClusterRef                  string               `json:"cluster_ref"`           // Target cluster
    RepoURLs                    []string             `json:"repo_urls,omitempty"`
    KBScopes                    []string             `json:"kb_scopes,omitempty"`
    SREDTags                    []contracts.SREDTag  `json:"sred_tags,omitempty"`   // Typed SR&ED categories
    FundingPrograms             []string             `json:"funding_programs,omitempty"`
    SREDDisabled                bool                 `json:"sred_disabled,omitempty"`
    AutoGenerateFundingReports  bool                 `json:"auto_generate_funding_reports,omitempty"`
    CostBudgetUSD               float64              `json:"cost_budget_usd,omitempty"`
    Metadata                    map[string]string    `json:"metadata,omitempty"`
}
```

### 2. ZenCluster

Represents a physical or virtual Kubernetes cluster that can execute work.

```go
type ZenClusterSpec struct {
    Endpoint                    string               `json:"endpoint"`
    AuthRef                     string               `json:"auth_ref"`               // Secret reference
    Capacity                    ClusterCapacity      `json:"capacity,omitempty"`
    Status                      string               `json:"status,omitempty"`       // active, inactive, draining
    Location                    string               `json:"location,omitempty"`     // local, cloud, edge
    Labels                      map[string]string    `json:"labels,omitempty"`
    Metadata                    map[string]string    `json:"metadata,omitempty"`
}
```

Both CRDs include a **status subresource** for observed state (phase, conditions, capacity usage, cost tracking).

All core interfaces (`ZenOffice`, `ZenContext`, `ZenJournal`, `ZenLedger`) are extended to accept a `clusterID` parameter, enabling cluster‑aware implementations.

## Consequences

### Positive

- **Native Kubernetes integration** – Use `kubectl get zenprojects`, watch for changes, leverage controller patterns.
- **Declarative configuration** – Projects and clusters are defined as YAML, can be GitOps‑managed.
- **Cluster‑aware routing** – The dispatcher reads `ZenProject.Spec.ClusterRef` to route tasks.
- **Capacity tracking** – `ZenCluster.Status.AvailableCapacity` helps with load‑aware scheduling.
- **Cost tracking per project** – `ZenProject.Status.CostSpentUSD` accumulates via ZenLedger.
- **SR&ED project configuration** – SR&ED categories are defined at the project level (`SREDTags`).
- **Extensible** – New fields can be added without breaking existing clients (CRD versioning).

### Negative

- **Increased complexity** – Need to manage CRD lifecycle (install, upgrade, version migration).
- **Additional API surface** – More objects to secure, audit, and monitor.
- **Controller overhead** – Need controllers to reconcile ZenProject/ZenCluster state.

### Neutral

- The control plane runs on a primary machine; data plane agents run on each cluster.
- Global ZenJournal aggregation must reconcile events from multiple clusters.

## Alternatives Considered

### 1. Configuration files only (no CRDs)

- **Pros**: Simpler, no Kubernetes API dependencies.
- **Cons**: No dynamic updates, no status tracking, harder to integrate with Kubernetes tooling.

### 2. Single ZenCluster CRD with embedded project list

- **Pros**: Fewer CRDs, tighter coupling of cluster‑project relationship.
- **Cons**: Less flexible (projects can’t move between clusters without redefinition), harder to scale.

### 3. Use Kubernetes Namespaces as projects

- **Pros**: Leverages built‑in namespace isolation, no custom CRDs.
- **Cons**: Cannot attach custom metadata (SREDTags, funding programs), limited to single‑cluster scope.

The dual‑CRD approach provides the right balance of flexibility, Kubernetes‑native operation, and project‑level configuration.

## Related Decisions

- [ADR‑0002](0002_SRED_TAXONOMY.md) – SREDTag enum used in `ZenProject.SREDTags`.
- [ADR‑0003](0003_CONTRACTS_PACKAGE.md) – `contracts` package provides `SREDTag` type.
- Construction Plan V6.0, Section “Multi‑Project, Multi‑Cluster Architecture”

## References

- `api/v1alpha1/zenproject_types.go` – ZenProject CRD definition
- `api/v1alpha1/zencluster_types.go` – ZenCluster CRD definition
- `pkg/contracts/contracts.go` – `SREDTag` type