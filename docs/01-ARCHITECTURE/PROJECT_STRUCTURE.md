# Project Structure

## Root Layout

```
zen-brain1/
├── cmd/                          # Entry points
│   └── zen-brain/               # Main CLI entry point
├── pkg/                         # Public Go packages
│   ├── contracts/               # Canonical data types (shared language)
│   ├── office/                  # ZenOffice interface (work ingress)
│   ├── journal/                 # ZenJournal interface (immutable event log)
│   ├── ledger/                  # ZenLedger interface (token/cost accounting)
│   ├── context/                 # ZenContext interface (session state)
│   ├── llm/                     # LLM gateway interfaces
│   ├── policy/                  # ZenPolicy interface (authorization, rate limiting)
│   ├── gate/                    # ZenGate interface (request filtering/validation)
│   ├── funding/                 # ZenFunding interface (SR&ED/IRAP alignment)
│   ├── taxonomy/                # Tag categories and validation
│   ├── kb/                      # Knowledge base interface
│   └── qmd/                     # QMD search interface
├── internal/                    # Private Go packages
│   ├── config/                  # Configuration and paths
│   └── office/                  # ZenOffice base implementation (for connectors)
├── api/                         # Kubernetes API definitions
│   └── v1alpha1/                # CRDs (ZenProject, ZenCluster)
├── docs/                        # Documentation
│   ├── architecture/            # Architecture decision records
│   ├── examples/                # Example files
│   └── *.md                     # Top-level docs (this file, data-model, kb-qmd)
├── scripts/                     # Utility scripts
├── deployments/                 # Deployment manifests (k3d, k8s)
├── Makefile                     # Build, test, dev tasks
├── go.mod                       # Go module definition
└── README.md                    # Project overview
```

## Key Directories

### `pkg/contracts`

Contains the **canonical data types** that all components agree on:

- `WorkType`, `WorkDomain`, `Priority`, `ExecutionMode`, `WorkStatus`, `EvidenceRequirement`, `SREDTag`, `ApprovalState`
- `AIAttribution`, `SourceMetadata`, `ExecutionConstraints`
- `WorkTags` (structured tag model)
- `WorkItem`, `Comment`, `Attachment`

No component‑specific types live here.

### `pkg/office`

Defines the `ZenOffice` interface for work ingress from external systems (Jira, Linear, Slack, etc.). Implementations map external issues to canonical `WorkItem`s.

**Rule:** No Jira‑specific types may leak beyond this boundary.

### `pkg/journal`

`ZenJournal` interface for immutable event logging with cryptographic chain hashes. Used for SR&ED evidence collection.

### `pkg/ledger`

`ZenLedgerClient` and `TokenRecorder` interfaces for token/cost accounting and value‑per‑token metrics.

### `pkg/kb`

Knowledge base abstract interface (`Store`). Used by planner to retrieve relevant documentation.

### `pkg/qmd`

QMD search abstract interface (`Client`). Wraps qmd CLI calls for indexing and searching the `zen‑docs` repository.

### `internal/config`

Configuration and path management. `Paths` defines all standard directories (`~/.zen‑brain/journal`, `~/.zen‑brain/ledger`, `~/.zen‑brain/kb`, etc.).

### `api/v1alpha1`

Kubernetes Custom Resource Definitions:

- `ZenProject` – project‑level configuration, includes typed `SREDTags`.
- `ZenCluster` – cluster registration and topology.

## Development Workflow

### Local Development

```bash
make dev-up       # Start k3d cluster
make dev-down     # Stop k3d cluster
make build        # Build binary
make test         # Run tests
make generate     # Generate code (CRDs, deepcopy)
make run          # Run locally
```

### Database Operations

```bash
make db-migrate   # Run database migrations
make db-reset     # Reset database (development only)
```

### Logs

```bash
make dev-logs     # Tail logs
```

## Configuration Paths

Zen‑Brain uses a configurable home directory (default `~/.zen‑brain/`). The following subdirectories are created automatically:

- `journal/` – ZenJournal event logs
- `context/` – ZenContext session state
- `cache/` – temporary/ephemeral data
- `config/` – configuration files
- `logs/` – application logs
- `ledger/` – ZenLedger token/cost records
- `evidence/` – evidence artifacts (logs, diffs, test results)
- `kb/` – knowledge base documents and indexes
- `qmd/` – qmd search indexes and embeddings

Override the base directory with the `ZEN_BRAIN_HOME` environment variable.

## Knowledge Base Integration

The `zen‑docs` repository is the source of truth for documentation. It should be cloned locally (or mounted) and indexed by qmd.

Example `zen‑docs` structure:

```
zen‑docs/
├── docs/
│   ├── architecture/
│   ├── guides/
│   └── runbooks/
├── meta/
│   └── manifest.yaml    # Document metadata registry (future)
└── README.md
```

## Adding a New Package

1. Create directory under `pkg/` (if public) or `internal/` (if private).
2. Define interfaces first, in an `interface.go` file.
3. Add minimal implementation in `internal/` or under the same package.
4. Update `go.mod` if new dependencies are needed.
5. Add unit tests.
6. Update this document if the package is significant.