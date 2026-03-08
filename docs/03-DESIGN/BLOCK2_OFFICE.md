# Block 2: Office Design

## Overview

Block 2 implements the **Office** layer of Zen‑Brain, where human intent is captured via external systems (Jira, Linear, Slack, etc.). The Office is responsible for:

- **Work ingress** – fetching work items from external systems
- **Intent analysis** – understanding what the human wants
- **Planning** – breaking down work into executable tasks
- **Human‑in‑the‑loop** – approvals, feedback, status updates
- **Evidence delivery** – attaching results back to the external system

This document focuses on the **Jira connector**, the first Office connector to be implemented.

## ZenOffice Interface Review

The `ZenOffice` interface (`pkg/office/interface.go`) defines the abstraction that all connectors must satisfy:

```go
type ZenOffice interface {
    Fetch(ctx context.Context, clusterID, workItemID string) (*contracts.WorkItem, error)
    FetchBySourceKey(ctx context.Context, clusterID, sourceKey string) (*contracts.WorkItem, error)
    UpdateStatus(ctx context.Context, clusterID, workItemID string, status contracts.WorkStatus) error
    AddComment(ctx context.Context, clusterID, workItemID string, comment *contracts.Comment) error
    AddAttachment(ctx context.Context, clusterID, workItemID string, attachment *contracts.Attachment, content []byte) error
    Search(ctx context.Context, clusterID string, query string) ([]contracts.WorkItem, error)
    Watch(ctx context.Context, clusterID string) (<-chan WorkItemEvent, error)
}
```

All methods accept a `clusterID` parameter, enabling multi‑cluster operation (see [ADR‑0004](../architecture/adr/0004‑multi‑cluster‑crds.md)).

## Jira Connector Responsibilities

The Jira connector must:

1. **Map Jira issues to canonical `WorkItem`s** – translate Jira‑specific fields (type, priority, status, labels) to the canonical taxonomy.
2. **Inject AI attribution** – prepend attribution headers to all AI‑generated content (comments, descriptions).
3. **Handle webhooks** – listen for Jira issue events (created, updated, commented) and forward them to the internal message bus.
4. **Maintain bidirectional sync** – ensure Zen‑Brain’s view of work status matches Jira’s (and vice‑versa).
5. **Respect Jira rate limits** – implement polite polling and exponential backoff.
6. **Support multiple authentication methods** – Personal Access Token (PAT), OAuth2, or basic auth.

## Field Mapping

| Jira Field | Canonical Field | Transformation Rules | Notes |
|------------|-----------------|----------------------|-------|
| `issue.key` | `Source.IssueKey` | Pass through | e.g., `ZEN‑42` |
| `issue.fields.summary` | `Title` | Pass through | |
| `issue.fields.description` | `Body` | Pass through (may contain wiki markup) | |
| `issue.fields.issuetype.name` | `WorkType` | See mapping table below | |
| `issue.fields.priority.name` | `Priority` | Highest→critical, High→high, Medium→medium, Low→low, Lowest→background | Normalized; custom priorities map to medium |
| `issue.fields.status.name` | `Status` | Configurable per project; default mapping in `config.yaml` | Jira statuses vary by workflow |
| `issue.fields.labels` | `Tags.HumanOrg` | All labels initially placed in `HumanOrg`; connector may infer other categories based on patterns | Example: `team‑platform`, `q1‑2026` |
| `issue.fields.components` | `WorkDomain` | Map component names to canonical `WorkDomain` enum; unknown→`core` | |
| `issue.fields.customfield_XXXXX` (KB Scope) | `KBScopes` | Custom field that lists documentation scopes | Optional; used by planner |
| `issue.fields.customfield_YYYYY` (SR&ED) | `Tags.SRED` | Map custom field values to `SREDTag` enum | Only if SR&ED enabled |
| `issue.fields.parent.key` | `Source.ParentKey` | Pass through | For subtasks |
| `issue.fields.epic.link` | `Source.EpicKey` | Pass through | For epics |
| `issue.fields.reporter.displayName` | `Source.Reporter` | Pass through | |
| `issue.fields.assignee.displayName` | `Source.Assignee` | Pass through | |
| `issue.fields.sprint.name` | `Source.Sprint` | Pass through | |
| `issue.fields.created` | `Source.CreatedAt` | Parse ISO8601 | |
| `issue.fields.updated` | `Source.UpdatedAt` | Parse ISO8601 | |

### WorkType Mapping

| Jira Issue Type | Canonical WorkType | Notes |
|-----------------|-------------------|-------|
| `Bug` | `debug` | |
| `Task`, `Chore` | `implementation` | Default for unknown types |
| `Story`, `Feature` | `design` | |
| `Epic`, `Initiative` | `research` | |
| `Spike`, `Investigation` | `analysis` | |
| `Documentation` | `documentation` | |
| `Refactor` | `refactor` | |
| `Security` | `security` | |
| `Test` | `testing` | |
| `Operation`, `Ops` | `operations` | |

### Status Mapping

Status mapping is **project‑configurable** because each Jira project can define its own workflow. The default mapping is:

| Jira Status | Canonical WorkStatus |
|-------------|----------------------|
| `Open`, `To Do` | `requested` |
| `In Analysis` | `analyzing` |
| `Analyzed` | `analyzed` |
| `In Planning` | `planning` |
| `Planned` | `planned` |
| `Pending Approval` | `pending_approval` |
| `Approved` | `approved` |
| `In Progress` | `running` |
| `Blocked` | `blocked` |
| `Done`, `Closed` | `completed` |
| `Rejected` | `canceled` |

The mapping is stored in `config.yaml` under `jira.status_mapping` and can be overridden per project.

## AI Attribution Injection

All AI‑generated content written to Jira must include a structured attribution header (see [ADR‑0005](../architecture/adr/0005‑ai‑attribution‑jira.md)).

**Format:**
```
[zen‑brain | agent: {role} | model: {model} | session: {sessionID} | task: {taskID} | {timestamp}]
```

**Implementation:**

- The `AddComment` method receives a `*contracts.Comment` with an optional `Attribution` field.
- If `Attribution` is present, the connector calls `BaseOffice.FormatAIAttributionHeader()` and prepends the header to the comment body.
- The header is added **before** the actual content, separated by a blank line.
- For description updates, the same header is prepended to the description (if the update is AI‑generated).

**Example Jira comment:**
```
[zen‑brain | agent: planner‑v1 | model: glm‑4.7 | session: abc123 | task: def456 | 2026‑03‑07T14:32:00Z]

I’ve analyzed the request and propose the following plan:
1. ...
```

## Webhook Handling

The Jira connector runs an HTTP server that listens for Jira webhook events. Supported events:

- `jira:issue_created`
- `jira:issue_updated`
- `jira:issue_deleted`
- `comment_created`
- `comment_updated`
- `worklog_updated`

Each event is transformed into a `WorkItemEvent` and published to the internal message bus (Block 3.1). The event includes the full `WorkItem` snapshot (fetched via Jira REST API).

**Webhook registration** can be automated (if the Jira instance supports app installation) or manual (admin configures webhook in Jira settings).

## Authentication and Configuration

The connector supports multiple authentication modes:

1. **Personal Access Token (PAT)** – Recommended for server‑to‑server integration.
2. **OAuth 2.0** – For user‑context operations (rarely needed).
3. **Basic auth** – Fallback for on‑prem Jira instances.

Configuration (`config.yaml` snippet):

```yaml
jira:
  enabled: true
  base_url: "https://your‑domain.atlassian.net"
  project: "ZEN"
  auth:
    type: "pat"
    pat: "${JIRA_PAT}"  # read from env var
  webhook:
    enabled: true
    path: "/webhooks/jira"
    secret: "${JIRA_WEBHOOK_SECRET}"
  field_mapping:
    kb_scope_custom_field: "customfield_10010"
    sred_custom_field: "customfield_10011"
  status_mapping:
    "To Do": "requested"
    "In Progress": "running"
    "Done": "completed"
```

## Error Handling and Retry

- **Rate limiting** – Respect Jira’s `Retry‑After` headers; use exponential backoff (via `zen‑sdk/pkg/retry`).
- **Transient failures** – Retry with jitter for network timeouts, 5xx errors.
- **Permanent failures** – Log and send to dead‑letter queue (`zen‑sdk/pkg/dlq`).
- **Validation errors** – If a Jira issue cannot be mapped to a valid `WorkItem`, log warning and skip (or create a “failed ingestion” ticket).

## Testing Strategy

### Unit Tests

- Test field mapping functions in isolation.
- Test attribution header formatting.
- Test config parsing.

### Integration Tests

- Use a **test Jira instance** (Atlassian Cloud free tier) or **Jira mock server**.
- Test full lifecycle: create issue → fetch → update status → add comment → verify attribution.
- Test webhook handling with simulated events.

### End‑to‑End Tests

- Deploy connector with real configuration.
- Create a test issue, let Zen‑Brain process it, verify results in Jira.
- Verify SR&ED evidence collection (if enabled).

## Dependencies

- `zen‑sdk/pkg/retry` – exponential backoff for API calls
- `zen‑sdk/pkg/http` – shared HTTP client with timeouts
- `zen‑sdk/pkg/logging` – structured logging
- `pkg/contracts` – canonical types
- `pkg/office` – ZenOffice interface
- `internal/office/base` – base implementation helpers

## Open Questions

1. **How to handle Jira custom fields that vary per project?** → Configurable field IDs in `config.yaml`; connector validates that fields exist on startup.
2. **Should we support Jira Data Center (on‑prem) differently?** → Same REST API; authentication may differ (basic auth).
3. **How to handle large‑scale sync (thousands of issues)?** – Implement incremental sync with `updated > last_sync` queries; paginate results.

## Next Steps

1. Implement `internal/connector/jira` package that embeds `BaseOffice`.
2. Write field mapping logic (Jira → `WorkItem`).
3. Implement webhook HTTP server.
4. Add configuration validation.
5. Write unit and integration tests.
6. Integrate with Block 3 (Message Bus) when ready.

---

*This document is a living design spec; update as implementation progresses.*