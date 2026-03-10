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

## Real vs mock

- **Real QMD:** The adapter uses the real `qmd` CLI when it is on PATH and the availability check passes. Tier 2 (ZenContext) then uses a real KB store: `Search` and `RefreshIndex`/`Populate` hit the indexed repo. Set `zen_context.tier2_qmd.repo_path` to the repo (e.g. after `make repo-sync` with `ZEN_KB_REPO_DIR` matching that path).
- **Mock QMD:** If the `qmd` binary is not found at startup, the adapter uses **FallbackToMock** (default `true` in `internal/context/factory.go`): a mock client returns simulated search results. No repo or index is required; useful for local dev without installing qmd.
- **How to run with real QMD:** (1) Install the qmd CLI (see `internal/qmd/README.md`). (2) Set `tier2_qmd.repo_path` in config to your KB repo path. (3) Run `make repo-sync` (and set `ZEN_KB_REPO_URL` if the repo is not yet cloned). (4) Run zen-brain or call `qmd.Populate` so the index is built. Tier 2 will use the real client as long as qmd is available at runtime.

## Validating search quality

- **Golden set:** `internal/qmd/testdata/golden_queries.json` defines expected queries and expected document titles/domain/min score.
- Run **`go test ./internal/qmd/... -run KBQuality`** to validate KB search against the golden set (uses mock client by default).
- For live validation against a real index, run the same golden-query checks with a real qmd client and repo after `Populate`.

## Syncing the repo before population

Run **`make repo-sync`** to clone or pull the KB repo (e.g. zen-docs). Set `ZEN_KB_REPO_URL` to clone from a remote; `ZEN_KB_REPO_DIR` (default `../zen-docs`) should match `tier2_qmd.repo_path` in your config. Then run `qmd embed` (or use `Populate`) to refresh the index. See `scripts/repo_sync.py`.

## Block 3.5 (KB Ingestion)

QMD population can be driven by the KB Ingestion Service (Block 3.5) when available: ingestion pipeline calls `Populate` or the qmd adapter’s `RefreshIndex` after syncing repo content.
