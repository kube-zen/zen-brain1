# QMD Adapter - Batch E MVP

## Overview

The QMD Adapter provides a Go implementation for interacting with the qmd (Query-Answer Memory Database) CLI tool. This adapter wraps the qmd CLI as a subprocess, enabling integration between zen-brain agents and the qmd document search system.

**This is the Batch E MVP implementation** of the QMD adapter for Zen‑Brain 1.0.

## Architecture

```
┌─────────────────────────────────────┐
│     Analyzer / Planner Agents       │
└──────────────┬──────────────────────┘
               │ kb.Store interface
               ▼
┌─────────────────────────────────────┐
│          KBStore                  │ ← Filters by scopes/tags
│  (implements kb.Store)            │   Converts to kb.SearchResult
└──────────────┬──────────────────────┘
               │ qmd.Client interface
               ▼
┌─────────────────────────────────────┐
│          Client                   │ ← Wraps qmd CLI
│  (implements qmd.Client)         │   Parses JSON output
└──────────────┬──────────────────────┘
               │ subprocess execution
               ▼
┌─────────────────────────────────────┐
│          qmd CLI                  │ ← External tool
│  (indexes zen-docs repository)    │   Search + Embed commands
└─────────────────────────────────────┘
```

## Key Features

### ✅ **MVP Complete**
- **QMD Client**: Wraps qmd CLI tool (`RefreshIndex`, `Search`)
- **KB Store**: Implements `kb.Store` interface for analyzer/planner consumption
- **Result Parsing**: Parses qmd JSON output into structured search results
- **Scope Filtering**: Filters search results by KBScopes (domain, tags)
- **Tag Matching**: Ensures all required tags are present in results
- **Comprehensive Tests**: 39 passing unit tests covering all functionality
- **Error Handling**: Validates inputs, handles subprocess failures gracefully

### 🔧 **QMD Integration**
- **Refresh Index**: `qmd embed --repo <path> --paths <paths>`
- **Search**: `qmd search --repo <path> --query <query> --json --limit N`
- **Timeout Handling**: Configurable timeout for qmd commands (default 30s)
- **Verbose Logging**: Optional logging for debugging

## Usage

### Basic QMD Client

```go
import (
    "context"
    "github.com/kube-zen/zen-brain1/internal/qmd"
)

// Create qmd client
config := &qmd.Config{
    QMDPath: "qmd",
    Timeout: 30 * time.Second,
    Verbose: true,
    SkipAvailabilityCheck: false, // set true for testing
}

client, err := qmd.NewClient(config)
if err != nil {
    log.Fatal(err)
}

// Refresh search index
err = client.RefreshIndex(ctx, qmd.EmbedRequest{
    RepoPath: "/path/to/zen-docs",
    Paths:    []string{"docs/"},
})

// Search documents
jsonOutput, err := client.Search(ctx, qmd.SearchRequest{
    RepoPath: "/path/to/zen-docs",
    Query:    "microservices architecture",
    Limit:    10,
})

// Parse results
results, err := qmd.ParseSearchResults(jsonOutput)
for _, result := range results {
    fmt.Printf("%s (%.2f): %s\n", result.Path, result.Score, result.Title)
}
```

### Knowledge Base Store

```go
import (
    "context"
    "github.com/kube-zen/zen-brain1/internal/qmd"
    "github.com/kube-zen/zen-brain1/pkg/kb"
)

// Create KB store backed by qmd
qmdClient, _ := qmd.NewClient(qmdConfig)
store, _ := qmd.NewKBStore(&qmd.KBStoreConfig{
    QMDClient: qmdClient,
    RepoPath:  "/path/to/zen-docs",
    Verbose:   true,
})

// Search with scopes and tags
results, err := store.Search(ctx, kb.SearchQuery{
    Query:    "deployment guide",
    KBScopes: []string{"ops", "devops"},
    Tags:     []string{"production"},
    Limit:    5,
})

for _, result := range results {
    fmt.Printf("[%s] %s\n", result.Doc.ID, result.Doc.Title)
    fmt.Printf("  Path: %s\n", result.Doc.Path)
    fmt.Printf("  Score: %.2f\n", result.Score)
    fmt.Printf("  Snippet: %s\n", result.Snippet)
}

// Get specific document
doc, err := store.Get(ctx, "KB-ARCH-0001")
if err == nil {
    fmt.Printf("Found: %s (%s)\n", doc.Title, doc.Path)
}
```

### Integration with Analyzer

```go
// The KB store implements kb.Store interface
// Can be passed to analyzer for knowledge base queries

analyzer, _ := analyzer.New(analyzerConfig, llmGateway, kbStore)

// Analyzer will use KB store to retrieve relevant docs
// based on task context, KB scopes, and tags
```

## Configuration

### QMD Client Config

```go
config := &qmd.Config{
    QMDPath:               "qmd",          // path to qmd binary
    Timeout:               30 * time.Second, // command timeout
    Verbose:               false,          // enable logging
    SkipAvailabilityCheck: false,          // skip qmd availability check (testing only)
}
```

### KB Store Config

```go
config := &qmd.KBStoreConfig{
    QMDClient: qmdClient,        // qmd.Client implementation
    RepoPath:  "/path/to/zen-docs",
    Verbose:   true,
}
```

## Data Structures

### SearchResult (from qmd)

```go
type SearchResult struct {
    Path     string     // file path
    Title    string     // document title
    Content  string     // snippet/content
    Score    float64    // relevance score
    Metadata *Metadata  // additional metadata
}

type Metadata struct {
    ID     string   // KB document ID
    Domain string   // domain (architecture, ops, etc.)
    Tags   []string // tags for filtering
    Source string   // source (git, confluence, etc.)
}
```

### kb.SearchResult (from KB Store)

```go
type SearchResult struct {
    Doc     DocumentRef // document reference
    Snippet string      // content snippet
    Score   float64     // relevance score
}

type DocumentRef struct {
    ID     string
    Path   string
    Title  string
    Domain string
    Tags   []string
    Source string
}
```

## Testing

```bash
# Run all QMD adapter tests
go test ./internal/qmd/... -v

# Test coverage
go test ./internal/qmd/... -cover

# Run specific test
go test ./internal/qmd/... -run TestParseSearchResults
```

**Test coverage includes:**
- QMD client initialization and configuration
- Refresh index validation
- Search validation (missing fields)
- Result parsing (valid/invalid JSON)
- Metadata extraction and conversion
- KB store initialization
- Search with scopes and tags
- Document retrieval
- Filter matching logic
- Error handling

## MVP Compromises (Documented)

1. **Mock qmd in tests**: Real qmd CLI not required for unit tests (can be skipped with `SkipAvailabilityCheck`)
2. **Basic query enhancement**: Simple string concatenation for scope/tag filters
3. **No direct get by ID**: Implemented via search + limit 1 (qmd doesn't have direct get)
4. **Placeholder refresh script**: `scripts/qmd_refresh.py` shows usage pattern
5. **No caching**: Results not cached (qmd handles its own indexing)
6. **Simple error handling**: Basic error propagation without retry logic

## Files

- `adapter.go` (7,994 bytes) – QMD client implementation
- `kb_store.go` (5,225 bytes) – KB store implementation
- `adapter_test.go` (11,162 bytes) – Client tests (19 tests)
- `kb_store_test.go` (10,626 bytes) – KB store tests (20 tests)
- **Interface**: `../../pkg/qmd/interface.go`
- **Scripts**: `../../scripts/qmd_refresh.py`
- **Design**: `../../docs/01-ARCHITECTURE/KB_QMD_STRATEGY.md`
- **ADR**: `../../docs/01-ARCHITECTURE/ADR/0007_QMD_FOR_KNOWLEDGE_BASE.md`

## Batch E Completion Status

✅ **MVP Requirements Met:**
- [x] `pkg/qmd/interface.go` – Already exists (qmd.Client interface)
- [x] `internal/qmd/adapter.go` – QMD client wrapper (RefreshIndex, Search)
- [x] `internal/qmd/kb_store.go` – KB store for analyzer/planner consumption
- [x] `scripts/qmd_refresh.py` – Already exists (placeholder refresh script)
- [x] Refresh support – `client.RefreshIndex()` with validation
- [x] Search support – `client.Search()` with JSON output parsing
- [x] Result parsing – `ParseSearchResults()` converts JSON to structs
- [x] Analyzer/planner consumption – `KBStore` implements `kb.Store` interface
- [x] Comprehensive tests – 39 passing unit tests
- [x] All existing tests continue to pass
- [x] Integration ready with existing components

## Next Steps (Post-MVP)

1. **Real qmd integration**: Test with actual qmd CLI tool installed
2. **Enhanced query building**: Better scope/tag query generation
3. **Caching**: Add result caching with TTL
4. **Direct get by ID**: Implement efficient document retrieval
5. **Metrics**: Add Prometheus metrics for search latency, result counts
6. **Retry logic**: Add configurable retry for transient failures
7. **Health checks**: Monitor qmd CLI availability and index freshness
8. **Confluence sync**: Implement one-way zen-docs → Confluence publishing

---

**🎉 BATCH E (QMD Adapter MVP) — COMPLETELY DELIVERED 🎉**  
**BATCHES A, B, C, D, E — ALL COMPLETE**  
**Foundation solid. Knowledge base integrated. Ready for Batch F (Jira vertical slice).**