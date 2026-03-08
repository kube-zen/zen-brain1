# Knowledge Base & QMD Strategy (1.0)

## Principles

1. **Git repo is the source of truth** – `zen‑docs` repository holds all canonical documentation.
2. **qmd is search/index over git** – qmd indexes the `zen‑docs` repo; agents query via CLI with JSON output.
3. **Confluence is a published mirror** – one‑way sync from `zen‑docs` → Confluence for human‑friendly browsing.
4. **Jira is the human entry point** – tickets link to KB scopes/docs; planner uses scopes to narrow retrieval.
5. **No CockroachDB for KB/QMD in 1.0** – CockroachDB holds structured runtime data (ZenLedger, session state, policies), not the document corpus.

## Architecture Flow

```
Human creates/updates work in Jira
    ↓
Jira item → ZenOffice → canonical WorkItem
    ↓
Planner reads KBScopes / tags / project context
    ↓
Planner queries qmd over `zen‑docs`
    ↓
Work may update code and/or docs
    ↓
KB changes land in Git via PR/merge
    ↓
qmd index refresh (after repo changes or on schedule)
    ↓
Publish job mirrors selected docs to Confluence
    ↓
Jira item links back to canonical docs and/or Confluence pages
```

## Interfaces

### Knowledge Base (`pkg/kb`)

Abstract interface for document retrieval:

```go
type Store interface {
    Search(ctx context.Context, q SearchQuery) ([]SearchResult, error)
    Get(ctx context.Context, id string) (*DocumentRef, error)
}
```

- `SearchQuery` includes `KBScopes` (from `WorkItem.KBScopes`) and `Tags`.
- `DocumentRef` includes `Path`, `Title`, `Domain`, `Tags`, `Source`.

### QMD (`pkg/qmd`)

Abstract interface for qmd interaction:

```go
type Client interface {
    RefreshIndex(ctx context.Context, req EmbedRequest) error
    Search(ctx context.Context, req SearchRequest) ([]byte, error)
}
```

- `RefreshIndex` updates embeddings for a repository/paths.
- `Search` returns raw JSON output (when `JSON=true`) or plain text.

## Implementation Notes (1.0)

### What We Build Now

- `pkg/kb` and `pkg/qmd` interface packages.
- Starter scripts for qmd refresh and Confluence publishing (placeholders).
- Documentation that clearly states the above principles.
- `KBScopes` field in `WorkItem` and mapping from Jira fields.

### What We Defer

- Custom KB ingestion service.
- Cockroach‑backed vector search for KB.
- Knowledge graph / relationship inference.
- Bidirectional Git ↔ Confluence sync.
- MCP‑first KB architecture.
- Heavy document relationship inference.

## Document Metadata

Each document in `zen‑docs` should eventually support metadata such as:

```yaml
id: KB-ARCH-0012
title: ZenOffice Jira Mapping
type: architecture
domain: office
tags:
  human_org: [architecture, jira, office]
  routing: [planner, office]
  policy: [internal]
  analytics: [kb, core]
owners: [platform]
status: active
source_of_truth: git
jira_projects: [ZEN]
jira_labels: [office, jira]
confluence_space: ZB
confluence_parent: Architecture
related_docs:
  - KB-DATA-0003
  - KB-OPS-0011
review_every_days: 90
```

This metadata enables:

- Clean qmd search context.
- Clean Jira linking.
- Predictable Confluence publishing.
- Future analytics without a graph DB.

## Jira Integration

Jira work items should have fields like:

- **KB Domain** – which domain the work belongs to (e.g., `office`, `factory`, `sdk`).
- **KB Scope** – which specific documentation areas are relevant.
- **Primary KB Doc** – the main document that describes the work.
- **Related KB Docs** – additional reference documents.
- **Doc Required?** – whether a doc update is required.
- **Doc Update Required?** – whether an existing doc must be updated.

The planner uses these fields to:

- Narrow KB retrieval.
- Pick the correct `zen‑docs` paths.
- Attach the right references to the task plan.

## Confluence Publishing

One‑way sync from `zen‑docs` → Confluence:

- Edits in Confluence are **disabled** for synced pages (or allowed only in designated human‑only spaces).
- No bidirectional sync in 1.0 – avoids merge complexity.
- Publishing can be scheduled (e.g., nightly) or triggered on merge.

## QMD Usage

qmd is invoked as a CLI tool:

```bash
# Index a repository
qmd embed --repo /path/to/zen-docs --paths docs/

# Search with JSON output
qmd search --repo /path/to/zen-docs --query "dynamic provisioning" --json --limit 5
```

The agent (or a thin Go wrapper) parses the JSON output and maps it to `kb.SearchResult`.

## CockroachDB Role

CockroachDB is used for:

- ZenLedger (token/cost accounting).
- Budget tracking.
- Session state.
- Structured planner/runtime metadata.
- Possibly document metadata registry (not the KB corpus itself).

It is **not** used as the vector store for KB/QMD in 1.0.