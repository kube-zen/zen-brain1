# Contributing to Zen-Brain 1.0

Thank you for your interest in contributing to Zen‑Brain! This document outlines the development workflow, coding standards, and how to submit changes.

## Development Philosophy

Zen‑Brain follows the **Office + Factory** architectural pattern:
- **Jira is the human front door** – work originates in Jira, but the internal execution model uses canonical `WorkItem` types.
- **ZenOffice is the abstraction boundary** – external system connectors live here; no Jira‑specific types leak into Factory or Planner.
- **Git‑based knowledge base** – `zen‑docs` repository is the source of truth; qmd indexes it for search; Confluence is a one‑way published mirror.
- **SR&ED evidence collection default ON** – every action is recorded for funding‑ready audit trails.
- **Multi‑cluster aware** – control plane, data plane agents, and workload placement across heterogeneous Kubernetes clusters.

Before making changes, familiarize yourself with the [architecture documentation](docs/architecture/CONSTRUCTION‑PLAN.md) and [data model](docs/data‑model.md).

## Development Environment

### Prerequisites

- **Go 1.25+** – Zen‑Brain is written in Go. Install from [go.dev](https://go.dev/dl/).
- **k3d v5.6.0+** – Local Kubernetes cluster for development. Install from [k3d.io](https://k3d.io/).
- **Docker** – Container runtime for local databases and dependencies.
- **git** – Version control.
- **make** – Build automation.

### Repository Setup

1. Clone the repository:

   ```bash
   git clone git@github.com:kube‑zen/zen‑brain1.git
   cd zen‑brain1
   ```

2. Install Go dependencies:

   ```bash
   make deps
   ```

3. (Optional) Set up a local k3d cluster for integration testing:

   ```bash
   make dev‑up
   ```

   This creates a k3d cluster named `zen‑brain‑dev` with CockroachDB and Redis pre‑deployed.

### Building and Testing

- **Build the binary**: `make build`
- **Run unit tests**: `make test`
- **Run tests with coverage**: `make coverage`
- **Format code**: `make fmt`
- **Run linter** (requires `golangci‑lint`): `make lint`

### Database Operations

For local development, a single‑node CockroachDB instance can be started with Docker:

```bash
make db‑up    # Start database
make db‑down  # Stop database
make db‑reset # Reset database (stop, remove, start)
```

Migrations are managed via `golang‑migrate`. The `make db‑migrate` target is a placeholder for now.

### Running Locally

After building, you can run Zen‑Brain directly:

```bash
make run
```

The binary expects a configuration file at `~/.zen‑brain/config.yaml` (or a custom location set via `ZEN_BRAIN_HOME`). See `configs/config.dev.yaml` for a template.

## Code Organization

```
zen‑brain1/
├── api/v1alpha1/          # CRD definitions (ZenProject, ZenCluster)
├── cmd/zen‑brain/         # Main entrypoint
├── pkg/                   # Public Go packages (interfaces and contracts)
│   ├── contracts/         # Canonical data types (WorkItem, WorkTags, SREDTag, etc.)
│   ├── office/            # ZenOffice interface (work ingress)
│   ├── journal/           # ZenJournal interface (immutable event log)
│   ├── ledger/            # ZenLedger interface (token/cost accounting)
│   ├── context/           # ZenContext interface (session state)
│   ├── llm/               # LLM gateway interfaces
│   ├── policy/            # ZenPolicy interface
│   ├── gate/              # ZenGate interface
│   ├── funding/           # ZenFunding interface (SR&ED/IRAP alignment)
│   ├── taxonomy/          # Tag categories and validation
│   ├── kb/                # Knowledge base interface
│   └── qmd/               # QMD search interface
├── internal/              # Private Go packages
│   ├── config/            # Configuration and paths
│   └── office/            # ZenOffice base implementation (for connectors)
├── docs/                  # Documentation
├── configs/               # Configuration templates
├── deployments/           # Kubernetes manifests
├── scripts/               # Utility scripts
└── Makefile               # Build, test, dev tasks
```

## Coding Standards

### Go Style

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).
- Use `gofmt` (or `goimports`) for formatting. The `make fmt` target applies formatting automatically.
- Write unit tests for new functionality. Aim for >80% coverage for business logic.
- Prefer composition over inheritance.

### Interface‑First Design

Zen‑Brain uses **interface‑first design**:
1. Define the interface in a `pkg/` package (e.g., `pkg/office/interface.go`).
2. Provide a base implementation in `internal/` if needed (e.g., `internal/office/base.go`).
3. Implement concrete adapters in `internal/connector/` (e.g., Jira, Linear, Slack).

**Critical rule:** No Jira‑specific types may leak beyond the `ZenOffice` abstraction boundary. The Factory operates on canonical `WorkItem` types only.

### Error Handling

- Use the `errors` package from `zen‑sdk/pkg/errors` for consistent error wrapping.
- Log errors with structured logging (`zen‑sdk/pkg/logging`).
- Return meaningful error messages that help debugging.

### Logging

Use structured logging via `zen‑sdk/pkg/logging`. Include correlation IDs (session, task) in log fields.

### Dependencies

- Cross‑cutting concerns (logging, retry, dedup, etc.) come from `zen‑sdk`. **If zen‑sdk has it, use it.**
- If zen‑brain needs a new cross‑cutting capability, build it in zen‑sdk first, then import it here.
- Keep domain logic in zen‑brain (LLM interfaces, agent types, work orders).

## Branching and Commits

### Trunk‑Based Development

Zen‑Brain uses **trunk‑based development**:
- The `main` branch is always deployable.
- Work happens in short‑lived feature branches (or directly on `main` for small changes).
- Each commit should be atomic and pass tests.

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) style:

```
<type>[optional scope]: <description>

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Changes that do not affect the meaning (formatting, missing semicolons, etc.)
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `test`: Adding or correcting tests
- `chore`: Changes to the build process, tooling, or dependencies

Example:

```
feat(office): add Jira connector with AI attribution

- Implements ZenOffice interface for Jira Cloud
- Injects AIAttribution header in all AI‑generated comments
- Adds webhook listener for issue updates

Refs: ZEN‑42
```

### Pull Requests

1. Create a branch from `main`.
2. Implement your change with tests.
3. Update documentation as needed.
4. Run `make test` and `make lint` (if available).
5. Push your branch and open a pull request.
6. Ensure CI passes (if configured).
7. Request review from maintainers.

## Testing

### Unit Tests

Unit tests live alongside the code they test (e.g., `pkg/office/interface_test.go`). Use the standard Go `testing` package.

### Integration Tests

Integration tests that require a Kubernetes cluster or external services are tagged with `// +build integration`. Run them with:

```bash
go test -tags=integration ./...
```

### End‑to‑End Tests

End‑to‑end tests are located in `tests/e2e/` and require a full k3d cluster with dependencies. Use `make dev‑up` before running e2e tests.

## Documentation

### Updating Documentation

- Architecture decisions go in `docs/architecture/`.
- API and interface documentation go in the respective `pkg/` directories as Go doc comments.
- User‑facing guides go in the `docs/` directory.
- Update the `README.md` if the change affects the project overview.

### Generating Code Documentation

To view Go documentation locally:

```bash
go doc ./pkg/office
```

## SR&ED Evidence Collection

**Default behavior:** Every session produces SR&ED‑eligible records unless explicitly disabled with `sred_disabled: true`. When adding new features or modifying existing ones, consider whether the change affects evidence collection or funding eligibility.

## Getting Help

- Check the [architecture documentation](docs/architecture/CONSTRUCTION‑PLAN.md) for design decisions.
- Review existing issues and pull requests.
- Contact the maintainers via GitHub Discussions or Slack (if available).

## License

By contributing to Zen‑Brain, you agree that your contributions will be licensed under the project’s existing license (see `LICENSE` file).