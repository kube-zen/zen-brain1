# Block 5.1: QMD Population

**Purpose:** Populate the Question-Answer Memory Database (QMD) with curated content and validate semantic search.

## Populating the index

- Use **`qmd.Populate(ctx, client, repoPath, paths)`** from `internal/qmd/populate.go` to refresh the QMD index for a repository.
- `repoPath` is the path to the repo (e.g. zen-docs); `paths` is optional (default in adapter: `docs/`).
- The qmd CLI is invoked via the adapter: `qmd embed --repo <path> --paths <paths>`.

Example (from code that has a `qmd.Client`):

```go
import "github.com/kube-zen/zen-brain1/internal/qmd"

err := qmd.Populate(ctx, qmdClient, "/path/to/zen-docs", []string{"docs/"})
```

## Sources and scope

- **Sources:** Curate which repos/directories are indexed (e.g. zen-docs only). Scope is effectively repository- and path-based.
- **Scope assignment:** Documents are associated with domains/tags via qmd metadata; validate scope filtering with the golden set and `internal/qmd` scope/tag tests.

## Validating search quality

- **Golden set:** `internal/qmd/testdata/golden_queries.json` defines expected queries and expected document titles/domain/min score.
- Run **`go test ./internal/qmd/... -run KBQuality`** to validate KB search against the golden set (uses mock client by default).
- For live validation against a real index, run the same golden-query checks with a real qmd client and repo after `Populate`.

## Block 3.5 (KB Ingestion)

QMD population can be driven by the KB Ingestion Service (Block 3.5) when available: ingestion pipeline calls `Populate` or the qmd adapter’s `RefreshIndex` after syncing repo content.
