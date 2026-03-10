# QMD Adapter and Orchestrator

This package provides a Go adapter for the `qmd` CLI tool (Quick Markdown Search) and a scheduler-based orchestrator for periodic index refresh.

**Real vs mock:** When the `qmd` CLI is available, the adapter uses it for real search and index updates. When it is not found, the adapter falls back to a mock client (if `FallbackToMock` is true, the default) so the app runs without qmd. See **docs/01-ARCHITECTURE/BLOCK5_QMD_POPULATION.md** (“Real vs mock”) for how to run with real QMD.

## Components

### Client (`adapter.go`)

Wraps the `qmd` CLI tool, executing commands and parsing JSON output. Implements the `qmd.Client` interface defined in `pkg/qmd/interface.go`.

### KB Store (`kb_store.go`)

Implements the `kb.Store` interface, providing knowledge base search over a qmd-indexed repository.

### Tier 2 Store (`internal/context/tier2/qmd_store.go`)

Integrates the KB store into ZenContext's Tier 2 (Warm) memory layer.

### Orchestrator (`orchestrator.go`)

Provides scheduled index refresh using `zen-sdk/pkg/scheduler`. Can be started and stopped programmatically.

## Usage

### 1. Install qmd

```bash
# Install Bun (JavaScript runtime)
curl -fsSL https://bun.sh/install | bash

# Install qmd globally
bun install -g https://github.com/tobi/qmd

# Verify installation
qmd --version
```

### 2. Configure Zen-Brain

In your `config.yaml`:

```yaml
kb:
  docs_repo: "../zen-docs"   # path to your zen-docs repository

qmd:
  binary_path: "qmd"         # path to qmd binary (defaults to PATH)
  refresh_interval: 3600     # seconds between automatic refreshes

zen_context:
  tier2_qmd:
    repo_path: "../zen-docs" # same as kb.docs_repo
    qmd_binary_path: "qmd"
    verbose: false
```

### 3. Create and start the orchestrator

```go
import "github.com/kube-zen/zen-brain1/internal/qmd"

// Create qmd client (skip availability check if qmd not installed)
client, err := qmd.NewClient(&qmd.Config{
    QMDPath: "qmd",
    SkipAvailabilityCheck: false,
})
if err != nil {
    log.Fatal(err)
}

// Create orchestrator
orc, err := qmd.NewOrchestrator(client, &qmd.OrchestratorConfig{
    RepoPath: "../zen-docs",
    RefreshInterval: time.Hour,
    Verbose: true,
})
if err != nil {
    log.Fatal(err)
}

// Start scheduler
if err := orc.Start(); err != nil {
    log.Fatal(err)
}
defer orc.Stop()

// Optionally trigger immediate refresh
ctx := context.Background()
if err := orc.RefreshNow(ctx); err != nil {
    log.Printf("Warning: initial refresh failed: %v", err)
}
```

### 4. Integrate with ZenContext factory

The factory (`internal/context/factory.go`) already creates a QMD store when `Tier2QMD.RepoPath` is set. Ensure your configuration passes the repo path.

## Testing

Run the adapter tests:

```bash
go test ./internal/qmd -v
```

Tests mock the qmd CLI, so they pass even without qmd installed.

## Notes

- The qmd CLI must be in `PATH` or specified via `QMDPath`.
- Index refresh can be triggered manually via `RefreshNow` or automatically via scheduler.
- If qmd is not available, the client will return an error on initialization (unless `SkipAvailabilityCheck` is true).
- The orchestrator uses `zen-sdk/pkg/scheduler` which supports cron expressions and `@every` intervals.