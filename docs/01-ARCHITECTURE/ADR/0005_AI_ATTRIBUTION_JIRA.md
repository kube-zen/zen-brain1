# ADR 0005: Inject AI attribution headers in all Jira content

## Status

**Accepted** (2026‑03‑07)

## Context

When AI agents generate content (comments, description updates, summaries) in Jira, humans need to know:
- **Which agent** produced the content (planner, worker, debugger, etc.)
- **Which model** was used (glm‑4.7, claude‑sonnet‑4‑6, etc.)
- **When** it was generated
- **For which session/task** (for correlation with ZenJournal events)

This attribution serves multiple purposes:
1. **Transparency** – Humans can understand whether content came from AI or a human.
2. **SR&ED evidence** – Links AI‑generated content to specific experimental sessions, supporting funding claims.
3. **Debugging** – When something goes wrong, we can trace which agent/model produced problematic output.
4. **Cost attribution** – Allows correlating Jira content with ZenLedger token/cost records.

Without structured attribution, this information would be lost or stored separately, breaking the audit trail.

## Decision

All AI‑generated content written to Jira (or any external system) must include a **structured attribution header** injected by the Office connector.

The header follows this format:

```
[zen‑brain | agent: {role} | model: {model} | session: {sessionID} | task: {taskID} | {timestamp}]
```

Example:

```
[zen‑brain | agent: planner‑v1 | model: glm‑4.7 | session: abc123 | task: def456 | 2026‑03‑07T14:32:00Z]

Here is the plan for implementing dynamic provisioning...
```

**Implementation details:**

1. **Attribution struct** – Defined in `pkg/contracts` as `AIAttribution`:
   ```go
   type AIAttribution struct {
       AgentRole  string    `json:"agent_role"`
       ModelUsed  string    `json:"model_used"`
       SessionID  string    `json:"session_id"`
       TaskID     string    `json:"task_id"`
       Timestamp  time.Time `json:"timestamp"`
   }
   ```

2. **Office connector responsibility** – The Jira connector (and any other Office connector) must:
   - Accept `AIAttribution` as part of `Comment` and `WorkItem` updates.
   - Format the attribution header using `BaseOffice.FormatAIAttributionHeader()`.
   - Prepend the header to the comment body or description before writing to Jira.
   - Preserve the original content after the header.

3. **Mandatory attribution** – AI‑generated content without attribution is considered a bug; the Office connector must reject it.

4. **Human‑generated content** – When a human writes a comment (via Jira UI), no attribution header is added. The Office connector can detect the absence of `AIAttribution` and omit the header.

## Consequences

### Positive

- **Complete audit trail** – Every AI‑generated piece of content in Jira is traceable back to a session, task, and model.
- **SR&ED compliance** – Attribution headers provide direct evidence of AI‑assisted experimental work.
- **Transparency** – Humans can immediately see that content came from AI and which agent was responsible.
- **Debugging ease** – When investigating issues, we can find all content from a specific session or task.
- **Cost correlation** – ZenLedger records can be matched with Jira content via `sessionID` and `taskID`.

### Negative

- **Visual clutter** – Attribution headers add noise to Jira comments and descriptions.
- **Character limit impact** – Headers consume part of Jira’s character limits (though the header is ~120 characters).
- **Parsing complexity** – If we need to parse existing comments to extract attribution, we must handle malformed headers.

### Neutral

- Attribution headers are only added when the content is **written** by Zen‑Brain. Content created by humans via the Jira UI remains unchanged.
- The header format is designed to be human‑readable but also machine‑parsable (simple regex).

## Alternatives Considered

### 1. Store attribution in Jira custom fields

- **Pros**: No visual clutter in comments, uses native Jira metadata.
- **Cons**: Requires custom field configuration, not portable to other systems (Linear, Slack), harder to correlate.

### 2. Store attribution separately in ZenJournal only

- **Pros**: Keeps Jira clean, all metadata in one place.
- **Cons**: Breaks the link when viewing Jira alone; humans cannot see attribution without accessing ZenJournal.

### 3. Use HTML comments (hidden in Jira)

- **Pros**: Invisible to humans, still machine‑readable.
- **Cons**: Jira may strip HTML comments, not all systems support hidden content.

The explicit header strikes the right balance between visibility, portability, and simplicity.

## Related Decisions

- [ADR‑0003](0003‑contracts‑package.md) – `AIAttribution` struct defined in `pkg/contracts`.
- Construction Plan V6.0, Section “AI Attribution in Jira”.

## References

- `pkg/contracts/contracts.go` – `AIAttribution` definition
- `internal/office/base.go` – `CreateAIAttribution()` and `FormatAIAttributionHeader()` helpers
- `pkg/office/interface.go` – `AddComment()` includes `*contracts.Comment` with optional `Attribution` field