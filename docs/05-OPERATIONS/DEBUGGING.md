# Debugging Guide (Block 6.4)

How to debug workers, KB (QMD) queries, LLM calls, and k3d-based local development.

---

## Quick reference

| Target | Binary / component | Logs / observability |
|--------|-------------------|----------------------|
| Foreman + workers | `./bin/foreman` | stderr; Prometheus `-metrics-bind-address` |
| API server | `./bin/apiserver` | stderr |
| Zen-brain vertical slice | `./bin/zen-brain` | stderr |
| QMD / KB | qmd CLI + adapter | Set `Verbose: true` in qmd client config; adapter logs `[QMD]` |
| LLM gateway | Used by zen-brain / planner | Provider responses; set log level or verbose in gateway config |
| k3d cluster | Pods in cluster | `make dev-logs` or `kubectl logs` |

---

## 1. Debugging workers (Foreman)

- **Run locally:** `make build-foreman && ./bin/foreman`. Use `KUBECONFIG` pointing at your k3d cluster (or default `~/.kube/config`). Apply CRDs first: `kubectl apply -f deployments/crds/`.
- **Logs:** Foreman and workers log to stderr (controller-runtime `logr`). Task execution (e.g. `processOne`) logs errors with `logger.Error(err, "get task for execution", "task", nn.String())` and similar.
- **Metrics:** Start foreman with `-metrics-bind-address=:8081` (or set in code). Scrape Prometheus for `brain_tasks_*` (scheduled, dispatched, completed, failed), `brain_worker_queue_depth`, reconcile duration.
- **Common issues:**
  - Task stuck in `Scheduled`: Worker not running or not receiving the task; check that Worker is started (`Start(ctx)`), queue depth, and that the task’s namespace/name is correct.
  - Task goes to `Failed`: Inspect `task.Status.Message` (often `err.Error()` from the runner). If using ContextBinder, check ZenContext/Redis connectivity for GetForContinuation/WriteIntermediate.
- **Session affinity:** When `SessionAffinity` is true, same session goes to the same worker; check per-worker queues and `sessionToWorker` mapping if tasks are not balanced.

---

## 2. Debugging KB / QMD queries

- **Adapter:** QMD is invoked via `internal/qmd/adapter.go` (CLI wrapper). Enable verbose logging by creating the client with config that sets verbose (e.g. `Config{Verbose: true}` for the internal adapter).
- **Logs:** Look for `[QMD]` in stdout/stderr: "Running: qmd search ...", "Refresh completed in ...", "qmd search failed ...".
- **CLI availability:** If the `qmd` binary is not in `PATH`, the adapter can fall back to a mock (when `FallbackToMock` is true). Verify with `qmd --version` (or the adapter’s availability check).
- **Search failures:** Timeouts and retries are configured in the adapter (zen-sdk retry). Check repo path, index freshness (`qmd.Populate` or `RefreshIndex`), and JQL/search query shape.
- **Golden tests:** Run `go test ./internal/qmd/... -run KBQuality` to validate search behavior against the golden set; use verbose or test logs to see which queries fail.

---

## 3. Debugging LLM calls

- **Gateway:** LLM calls go through `internal/llm` (gateway, routing, fallback). Logging depends on the provider implementation and any log level you set in config.
- **Retries:** Provider and qmd paths use zen-sdk retry; transient failures are retried with backoff. Check for repeated errors in logs.
- **Planner / zen-brain:** The vertical slice and planner use the gateway for analysis and planning. To see prompts/responses, enable debug or verbose in the LLM gateway or provider (e.g. log request/response in a test or dev config).
- **Cost / ledger:** If ZenLedger is wired, token and cost are recorded; query the ledger or check DB for usage per task/session.

---

## 4. k3d-specific debugging

- **Cluster state:** `kubectl get nodes`, `kubectl get pods -A`, `kubectl get crd | grep brain`.
- **Ports:** dev-up maps `8080:80` (load balancer) and `26257:26257` (CockroachDB). Use `kubectl port-forward` if you need to reach a service not exposed by the load balancer.
- **Logs:** `make dev-logs` tails logs from pods with label `app.kubernetes.io/part-of=zen-brain`. For other pods (e.g. zen-context): `kubectl logs -f -n zen-context -l app=redis` (adjust labels to your manifests).
- **Reset:** Full cluster reset: `make dev-down` then `make dev-up`. Local DB only: `make dev-clean` (runs `db-reset`); if you don’t use local Docker DB, use dev-down/dev-up for a clean k3d state.
- **Foreman/API server in-cluster:** Currently run as local binaries with kubeconfig; to run inside k3d, build an image, load with `k3d image import -c zen-brain-dev <image>`, and deploy a Deployment (see deployments/k3d/README.md TBD).

---

## 5. Development commands (Block 6.2)

- `make dev-up` — Create k3d cluster and apply dependencies.
- `make dev-down` — Delete k3d cluster.
- `make dev-clean` — Reset local Docker DB (db-reset). For full k3d reset use dev-down then dev-up.
- `make dev-logs` — Tail logs from zen-brain pods in the cluster.
- `make dev-build` — Build all binaries (zen-brain, foreman, apiserver). When a Dockerfile exists, build the image and load with `k3d image import ... -c zen-brain-dev`.

---

*Block 6.4 – Developer Experience debugging guide.*
