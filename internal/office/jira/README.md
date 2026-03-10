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

### Environment Variables
```bash
export JIRA_URL="https://your-domain.atlassian.net"
export JIRA_TOKEN="your-api-token"
export JIRA_EMAIL="your-email@example.com"
export JIRA_PROJECT_KEY="PROJ"  # Optional
```

### YAML Configuration
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
import "github.com/kube-zen/zen-brain1/internal/office/jira"

// From environment variables
connector, err := jira.NewFromEnv("jira-prod", "cluster-1")
if err != nil {
    log.Fatal(err)
}

// Or with explicit config
config := &jira.Config{
    BaseURL:    "https://company.atlassian.net",
    Email:      "zen-brain@company.com",
    APIToken:   "api-token",
    ProjectKey: "PROJ",
}
connector, err := jira.New("jira-prod", "cluster-1", config)
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

**Proof-of-Work Comment Format:**
```markdown
[zen-brain | agent:factory | model:factory-v1 | session:session-123 | task:task-456 | 2026-03-07 14:30:00 UTC]

# Proof-of-Work: PROJ-123

## Objective
Implement feature as described in ticket

## Execution Summary
- **Status**: Success
- **Duration**: 15m 23s
- **Started**: 2026-03-07 14:15:00 UTC
- **Completed**: 2026-03-07 14:30:23 UTC

## Work Done
- **Files Changed**: 5
- **Tests Run**: 10
- **Tests Passed**: 10
- **Git Branch**: ai/PROJ-123
- **Git Commit**: abc123def456...

## Evidence Items
- Implemented core feature logic
- Added unit tests
- Updated documentation

## Unresolved Risks
- RISK: Integration testing pending
- RISK: Performance optimization needed

## Recommended Action
merge
```

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
- Environment variables for authentication