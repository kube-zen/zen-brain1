# Development Documentation

This directory contains practical guides for developers working on Zen‑Brain.

## Getting Started

- **[Setup Guide](SETUP.md)** – Step‑by‑step instructions to set up a complete development environment (prerequisites, k3d cluster, configuration, testing).

## Reference

- **[Contributing Guide](../../CONTRIBUTING.md)** – Coding standards, commit conventions, pull request process.
- **[Project Structure](../01-ARCHITECTURE/PROJECT_STRUCTURE.md)** – Directory layout and package organization.
- **[Configuration Reference](CONFIGURATION.md)** – All configurable options across components.

## Testing

- **Unit Tests** – `make test`
- **Integration Tests** – `go test -tags=integration ./...`
- **End‑to‑End Tests** – `tests/e2e/`

## Database Operations

- `make db‑up` – start local CockroachDB (Docker)
- `make db‑down` – stop local CockroachDB
- `make db‑migrate` – run database migrations
- `make db‑reset` – reset database (development only)

## Local Cluster Management

- `make dev‑up` – create k3d cluster and deploy dependencies
- `make dev‑down` – delete k3d cluster
- `make dev‑build` – build Docker image and load into k3d registry
- `make dev‑logs` – tail logs of all Zen‑Brain pods

## Debugging

### Common Issues

**k3d cluster fails to start:** Ensure Docker is running and ports 8080/26257 are free.

**Database connection refused:** Verify CockroachDB pod is running: `kubectl get pods -n dependencies`.

**Ollama models not loaded:** Manually pull models:

```bash
kubectl exec -n dependencies deployment/ollama -- ollama pull glm‑4.7
kubectl exec -n dependencies deployment/ollama -- ollama pull nomic‑embed‑text
```

**Insufficient memory:** Increase k3d memory limit with `--memory 8G`.

### Logs

- **Zen‑Brain controller:** `kubectl logs -n zen‑brain deployment/zen‑brain‑controller`
- **Worker pods:** `kubectl logs -n zen‑brain deployment/zen‑brain‑worker`
- **CockroachDB:** `kubectl logs -n dependencies deployment/cockroachdb`

## Performance Profiling

### CPU Profile

```bash
go tool pprof bin/zen‑brain cpu.pprof
```

### Memory Profile

```bash
go tool pprof bin/zen‑brain mem.pprof
```

## Generating Code

### CRDs

```bash
make generate
```

Regenerates Kubernetes client code after changes to `api/v1alpha1/` types.

### Protobuf (if used)

```bash
make proto
```

## IDE Configuration

### VS Code

Install extensions: Go, Kubernetes, Docker, YAML.

### GoLand / IntelliJ

Enable Go modules, Kubernetes plugin.

## Continuous Integration

CI is configured via GitHub Actions (`.github/workflows/`). Runs on every pull request:

- `make repo-check` – repo hygiene gates
- `go test`
- `go build`
- `golangci‑lint` (if configured)
- `go vet`

## Release Process

1. Update version in `Makefile` and `go.mod`.
2. Run `make test` and `make integration`.
3. Tag the release: `git tag v1.0.0‑alpha.1`.
4. Push tag: `git push origin v1.0.0‑alpha.1`.
5. GitHub Actions builds and publishes Docker images.

---

*For architectural documentation, see [Architecture](../01-ARCHITECTURE/).*