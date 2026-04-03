# Jira Connector for Zen-Brain

Implements the `ZenOffice` interface for Atlassian Jira, providing bidirectional synchronization between Jira issues and Zen-Brain work items.

## Features

- ✅ **AI Attribution (V6 Requirement)**: All AI-generated comments include structured attribution headers
- ✅ **Canonical Mapping**: Maps Jira issues to `contracts.WorkItem` with proper type conversion
- ✅ **Status Synchronization**: Bidirectional status updates between Jira and Zen-Brain
- ✅ **Comment Sync**: Comments flow both ways with AI attribution preserved
- ✅ **Proof-of-Work Integration**: Complete workflow support for execution results and proof summaries
- ✅ **Status Transitions**: Configurable workflow transitions with proper mapping
- ✅ **Webhook Support**: Real-time updates via Jira webhooks; call `Watch(ctx, clusterID)` to start the webhook server and receive `WorkItemEvent` on the returned channel; HMAC-SHA256 signature validation when `webhook_secret` is set
- ✅ **Attachment Support**: Evidence attachment upload via `AddAttachment`; Jira REST API `/rest/api/3/issue/{id}/attachments`
- ✅ **JQL Search**: `Search(ctx, clusterID, query)` accepts JQL (or free text); optional project scoping via `project_key`; returns `[]WorkItem`

## Configuration

Jira credentials are resolved exclusively through the canonical resolver:
- **Cluster mode**: `/zen-lock/secrets/*` (ZenLock CSI mount)
- **Local mode**: canonical encrypted store via `secrets.ResolveJira()`

Credentials are NEVER read from environment variables or `.env` files at runtime.

```yaml
jira:
  enabled: true
  base_url: "https://your-domain.atlassian.net"
  project_key: "PROJ"
  field_mappings:
    epic_link: "customfield_10014"
    story_points: "customfield_10016"
    sprint: "customfield_10020"
  webhook_url: "https://your-zen-brain.com/webhooks/jira"
  webhook_secret: "your-secret"
```

## Usage

### Create a Jira Connector

```go
import (
    "github.com/kube-zen/zen-brain1/internal/config"
    "github.com/kube-zen/zen-brain1/internal/office/jira"
)

// Load config (resolves credentials from canonical sources)
cfg, err := config.LoadJiraConfig()
if err != nil {
    log.Fatal(err)
}

connector, err := jira.New("jira-prod", "cluster-1", cfg)
if err != nil {
    log.Fatal(err)
}
```

### Fetch a Jira Issue

```go
workItem, err := connector.FetchBySourceKey(ctx, "cluster-1", "PROJ-123")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Fetched: %s - %s\n", workItem.ID, workItem.Title)
```

### Add a Comment with AI Attribution

```go
comment := &contracts.Comment{
    ID:         uuid.New().String(),
    WorkItemID: "PROJ-123",
    Body:       "I've implemented the feature as requested.",
    Author:     "zen-brain",
    CreatedAt:  time.Now(),
    Attribution: &contracts.AIAttribution{
        AgentRole:  "worker-debug",
        ModelUsed:  "glm-4.7",
        SessionID:  "session-123",
        TaskID:     "task-456",
        Timestamp:  time.Now(),
    },
}

err := connector.AddComment(ctx, "cluster-1", "PROJ-123", comment)
```

### Watch for Real-time Updates

```go
events, err := connector.Watch(ctx, "cluster-1")
if err != nil {
    log.Fatal(err)
}

for event := range events {
    fmt.Printf("Event: %s - %s\n", event.Type, event.WorkItem.Title)
}
```

### Proof-of-Work Integration

```go
// Step 1: Fetch the issue (canonical work)
workItem, err := connector.FetchBySourceKey(ctx, "cluster-1", "PROJ-123")
if err != nil {
    log.Fatal(err)
}

// Step 2: Factory executes task and generates proof-of-work
// (factory internally calls ProofOfWorkManager.GenerateJiraComment)
proofComment, err := factory.Execute(ctx, spec)
if err != nil {
    log.Fatal(err)
}

// Step 3: Add proof-of-work as Jira comment
err = connector.AddComment(ctx, "cluster-1", "PROJ-123", proofComment)
if err != nil {
    log.Fatal(err)
}

// Step 4: Update issue status based on execution result
var targetStatus contracts.WorkStatus
if proofComment.RequiresApproval {
    targetStatus = contracts.StatusRequested  // Needs human review
} else {
    targetStatus = contracts.StatusCompleted  // Ready to merge
}

err = connector.UpdateStatus(ctx, "cluster-1", "PROJ-123", targetStatus)
if err != nil {
    log.Fatal(err)
}
```

**Complete Proof-of-Work Workflow:**
1. **Fetch** issue → Get canonical work from Jira
2. **Execute** task → Factory runs bounded execution in isolated workspace
3. **Generate** proof → ProofOfWorkManager creates JSON/Markdown artifacts
4. **Comment** → Jira receives execution summary with AI attribution
5. **Update** status → Issue transitions based on execution outcome

## AI Attribution Format

All AI-generated content includes a structured attribution header:

```
[zen-brain | agent:worker-debug | model:glm-4.7 | session:550e8400-e29b-41d4-a716-446655440000 | task:123e4567-e89b-12d3-a456-426614174000 | 2026-03-07 14:30:00 EST]
```

This header:
1. **Identifies the source** as `zen-brain`
2. **Specifies the agent role** (planner, worker, debug, etc.)
3. **Records the model used** for transparency
4. **Includes session and task IDs** for audit trail
5. **Timestamps the generation** for SR&ED evidence

## Field Mappings

| Jira Field | Canonical Field | Notes |
|------------|-----------------|-------|
| `key` | `ID` | Jira issue key (PROJ-123) |
| `summary` | `Title`, `Summary` | |
| `description` | `Body` | |
| `issuetype.name` | `WorkType` | Mapped via `mapWorkType()` |
| `priority.name` | `Priority` | Mapped via `mapPriority()` |
| `status.name` | `Status` | Mapped via `mapStatus()` |
| `labels` | `Tags.HumanOrg` | |
| `created` | `CreatedAt` | |
| `updated` | `UpdatedAt` | |
| `reporter.displayName` | `Source.Reporter` | |
| `assignee.displayName` | `Source.Assignee` | |

## Status Mapping

| Jira Status | WorkStatus |
|-------------|------------|
| To Do, Backlog | `StatusRequested` |
| In Progress, In Development | `StatusRunning` |
| Review, Testing | `StatusRunning` |
| Done, Completed, Closed | `StatusCompleted` |
| Blocked, On Hold, Paused | `StatusBlocked` |
| Failed | `StatusFailed` |
| Canceled | `StatusCanceled` |

## Office CLI (zen-brain office)

The vertical slice and config bootstrap are the main integration path. For operational use:

- **zen-brain office doctor** – config source, connectors, cluster mapping, Jira base URL (sanitized), project key, webhook config, credentials present, API reachability (Ping).
- **zen-brain office search &lt;query&gt;** – search work items (JQL or plain text); prints key, title, status, work type, priority.
- **zen-brain office fetch &lt;jira-key&gt;** – fetch one item; prints canonical mapping (ID, title, status, work type, work domain, source metadata, tags).
- **zen-brain office watch** – start Jira webhook listener and stream events until interrupted.

Vertical-slice now posts proof-of-work comments and attachments back to Jira by default (when not in mock mode), and reports Office summary (comment posted, attachments count, status updated).

## Development

### Running Tests

```bash
go test ./internal/office/jira/...
```

### Adding Custom Fields

Edit `types.go` to add custom field mappings for your Jira instance.

### Webhook Development

The webhook handler requires:
1. Jira webhook configuration pointing to your endpoint
2. Secret verification for security
3. Event parsing and conversion to `WorkItemEvent`

## Dependencies

- Go 1.25+
- Jira REST API v3
- Canonical credential resolver (`secrets.ResolveJira()`)
