# Jira Connector for Zen-Brain

Implements the `ZenOffice` interface for Atlassian Jira, providing bidirectional synchronization between Jira issues and Zen-Brain work items.

## Features

- ✅ **AI Attribution (V6 Requirement)**: All AI-generated comments include structured attribution headers
- ✅ **Canonical Mapping**: Maps Jira issues to `contracts.WorkItem` with proper type conversion
- ✅ **Status Synchronization**: Bidirectional status updates between Jira and Zen-Brain
- ✅ **Comment Sync**: Comments flow both ways with AI attribution preserved
- 🔄 **Webhook Support**: Real-time updates via Jira webhooks (TODO)
- 🔄 **Attachment Support**: Evidence attachment synchronization (TODO)
- 🔄 **JQL Search**: Advanced search capabilities (TODO)

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