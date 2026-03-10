# Canonical Data Model

This document describes the shared data types used across Zen‑Brain components. All canonical definitions live in `pkg/contracts`.

## Core Enums

### WorkType
- `research`, `design`, `implementation`, `debug`, `refactor`, `documentation`, `analysis`, `operations`, `security`, `testing`

### WorkDomain
- `office`, `factory`, `sdk`, `policy`, `memory`, `observability`, `infrastructure`, `integration`, `core`

### Priority
- `critical`, `high`, `medium`, `low`, `background`

### ExecutionMode
- `autonomous`, `approval_required`, `read_only`, `simulation_only`, `supervised`

### WorkStatus
- `requested`, `analyzing`, `analyzed`, `planning`, `planned`, `pending_approval`, `approved`, `queued`, `running`, `blocked`, `completed`, `failed`, `canceled`

### EvidenceRequirement
- `none`, `summary`, `logs`, `diff`, `test_results`, `full_artifact`

### SREDTag (SR&ED uncertainty categories)
- `u1_dynamic_provisioning`, `u2_security_gates`, `u3_deterministic_delivery`, `u4_backpressure`, `experimental_general`

### ApprovalState
- `pending`, `approved`, `rejected`, `not_required`

## Structured Tags

Flat labels are replaced by a structured `WorkTags` model:

```go
type WorkTags struct {
    HumanOrg  []string   `json:"human_org,omitempty"`   // Epics, teams, quarters
    Routing   []string   `json:"routing,omitempty"`     // System routing decisions
    Policy    []string   `json:"policy,omitempty"`      // ZenGate policy evaluation
    Analytics []string   `json:"analytics,omitempty"`   // Dashboards and reporting
    SRED      []SREDTag  `json:"sred,omitempty"`        // Typed SR&ED categories
}
```

This taxonomy enables Jira‑as‑front‑door mapping:

- HumanOrg → Jira components, epics, teams
- Routing → planner/worker assignment
- Policy → ZenGate policy class
- Analytics → dashboards, reporting
- SRED → SR&ED evidence tagging

## WorkItem

The canonical work representation that all Office connectors map to, and the Factory operates on exclusively.

### Identity & Classification
- `ID`, `Title`, `Summary`, `Body`
- `WorkType`, `WorkDomain`, `Priority`, `ExecutionMode`

### Lifecycle
- `Status`, `CreatedAt`, `UpdatedAt`

### Context
- `ClusterID`, `ProjectID`, `WorkingDir`

### Tags & Scopes
- `Tags WorkTags`
- `KBScopes []string` – knowledge base scopes for retrieval

### Requirements & Evidence
- `EvidenceRequirement`
- `EvidenceRefs []string`
- `SREDDisabled bool`

### Source & Attribution
- `Source SourceMetadata` – origin information (Jira, Linear, GitHub, Slack)
- `Attribution *AIAttribution` – AI‑generated content attribution

### Relationships
- `ParentID`, `DependsOn`

### Request & Approval
- `RequestedBy`, `ApprovalState`, `PolicyClass`

### Execution Constraints
- `ExecutionConstraints` – max cost, timeout, allowed clusters, required approval

## SourceMetadata

Preserves origin system details (not execution‑critical):

- `System`, `IssueKey`, `Project`, `IssueType`, `ParentKey`, `EpicKey`
- `Reporter`, `Assignee`, `Sprint`
- `CreatedAt`, `UpdatedAt`

## AIAttribution

Structured attribution for AI‑generated content (injected by ZenOffice adapters):

- `AgentRole`, `ModelUsed`, `SessionID`, `TaskID`, `Timestamp`

## Comment & Attachment

Standard comment and attachment types with optional AI attribution.

## Multi‑cluster CRDs

- `ZenProject` – project‑level configuration, includes typed `SREDTags`
- `ZenCluster` – cluster registration and topology

## Validation and Normalization

Block 1 (Neuro-Anatomy) adds an explicit validation and normalization layer in `pkg/contracts`:

- **`validate.go`**: `IsValidWorkType`, `IsValidWorkDomain`, `IsValidPriority`, and similar helpers for all canonical enums; `Parse*` functions for strict string→enum parsing (no silent guessing); `ValidateWorkTags` (duplicates, valid SRED); `ValidateWorkItem`, `ValidateBrainTaskSpec`, `ValidateAnalysisResult`, `ValidateExecutionConstraints`.
- **`normalize.go`**: `NormalizeWorkItem`, `NormalizeBrainTaskSpec` — trim strings, sort and dedupe slices (DependsOn, KBScopes, AllowedClusters, tag categories). No invention of enum values.

Use these before persisting or passing canonical structs to ensure consistency and to catch invalid data early.

## Versioning & Compatibility

Canonical types are considered **stable** once Block 1 is complete. Changes require a migration plan and update of all dependent packages (Office, Factory, Journal, Ledger, Policy, Gate, Funding, Taxonomy).