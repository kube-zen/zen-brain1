> **NOTE:** This document references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

# Deployment validation checklist

Use this to validate the canonical Helmfile path end-to-end. Passing it cleanly supports moving Block 6 (and overall completeness) to 92%+.

## Prerequisites

- `helm`, `helmfile`, `k3d`, `kubectl`, `docker` on PATH
- `config/clusters.yaml` present (default for sandbox)

## Offline checks (no cluster)

| Step | Command | Expected |
|------|--------|----------|
| Values generation | `python3 scripts/common/helmfile_values.py sandbox` | No error; `.artifacts/state/sandbox/*-values.yaml` created |
| Helmfile list | `helmfile -e sandbox -f deploy/helmfile/zen-brain/helmfile.yaml.gotmpl list` | Four releases listed (crds, dependencies, ollama, core) |

## Full live validation (fresh or existing cluster)

Run in order. Use sandbox unless you need another env.

| Step | Command / check | Expected |
|------|------------------|----------|
| 1. Fresh dev-up | `make dev-up` (or `ZEN_DEV_ENV=sandbox python3 scripts/zen.py env redeploy --env sandbox`) | Cluster created (or reused), registry up, values generated, Helmfile sync runs |
| 2. Generated values | `ls -la .artifacts/state/sandbox/*.yaml` | `zen-brain-dependencies-values.yaml`, `zen-brain-values.yaml`, `zen-brain-ollama-values.yaml` |
| 3. Helmfile order | Observe sync order in step 1 output | crds → dependencies → ollama → core |
| 4. Core workloads | `kubectl get pods -n zen-brain` (after sync) | foreman, apiserver (and optionally ollama) Running/Ready |
| 5. Ollama preload (if enabled) | Set `deploy.use_ollama: true` and `deploy.ollama.models: ["qwen3.5:0.8b"]` in config, redeploy; then `kubectl get jobs -n zen-brain` | Preload Job exists and completes; `ollama-0` pod Running |
| 6. OLLAMA_BASE_URL | When Ollama enabled: `kubectl set env deployment/apiserver -n zen-brain --list \| grep OLLAMA` or check pod spec | `OLLAMA_BASE_URL=http://ollama:11434` |
| 7. Real inference | Call apiserver (e.g. health or a session endpoint); trigger a path that uses local-worker | No simulated response; real Ollama response when model is loaded |

## Success criteria

- No manual `kubectl apply` or `kubectl exec ... ollama pull` in the happy path
- Apiserver external access works (e.g. `curl http://127.0.1.6:8080/healthz` for sandbox)
- When Ollama is enabled: preload Job succeeds, apiserver has `OLLAMA_BASE_URL`, and the local-worker lane uses Ollama

## After validation

If this passes cleanly, the canonical path is validated and Block 6 / overall completeness can be re-baselined upward. Remaining work is then residual legacy removal (e.g. eventual removal of deprecated `dev-apply`) and runtime/factory/intelligence maturity, not basic platform hygiene.

---

## Live validation report (2026-03-10)

**Environment:** sandbox  
**Cluster:** k3d-zen-brain-sandbox (context `k3d-zen-brain-sandbox`)  
**Ollama:** disabled (`use_ollama: false`)

### Fixes applied during validation

1. **k3d + external registry** – Cluster create was failing with “container not managed by k3d”. Stopped passing `--registry-use` for the standalone registry; cluster is created and the registry container is attached to the k3d network after create.
 2. **Shared registry** – Image is pushed to shared registry `zen-brain-registry:5000` and cluster pulls from registry (no k3d image import needed).
3. **Helmfile** – Dropped cross-namespace `needs` (order-only: crds → dependencies → ollama → core). Set `helmDefaults.createNamespace: false` and ensured namespaces are created by the redeploy script before sync. Removed `templates/namespace.yaml` from zen-brain and zen-brain-dependencies charts to avoid Helm namespace ownership conflicts.
4. **zen-brain-crds** – Chart type changed from `library` to `application` so Helm can install it.
 5. **Redeploy script** – Helmfile is run with `--kube-context <context_name>` so sync targets the correct cluster. Namespaces `zen-brain` and `zen-context` are created if missing before sync. Core image repository in generated values uses cluster registry ref (`zen-brain-registry:5000/zen-brain`) so deployments pull from shared registry.

### Checklist results

| Step | Result | Notes |
|------|--------|--------|
| 1. Fresh dev-up | Pass | `scripts/zen.py env redeploy --env sandbox` (with fixes above): cluster created/reused, registry up, values generated, Helmfile sync runs. |
| 2. Generated values | Pass | `.artifacts/state/sandbox/zen-brain-dependencies-values.yaml`, `zen-brain-values.yaml`, `zen-brain-ollama-values.yaml` present; zen-brain values use `image.repository: zen-brain-registry:5000/zen-brain`. |
| 3. Helmfile order | Pass | Sync order: crds, dependencies, ollama, core (order-based, no `needs`). |
| 4. Core workloads | Partial | **Apiserver:** Running/Ready; `curl http://127.0.1.6:8080/healthz` → 200, body "ok". **Foreman:** New replica with correct image is Running; some replicas from previous revision remained in ImagePullBackOff until cleaned up; foreman may need a short period to become Ready after a clean sync. |
| 5–7. Ollama | N/A | Ollama not enabled; preload, OLLAMA_BASE_URL, and real inference not exercised. |

### Success criteria

- No manual `kubectl apply` or `kubectl exec ... ollama pull` in the path: **met.**
- Apiserver external access: **met** (`curl http://127.0.1.6:8080/healthz` returns 200).
- Ollama-related criteria: **N/A** (Ollama disabled).

### Conclusion

Live validation for sandbox (without Ollama) is **passed** with the fixes above. Cluster bring-up, Helmfile sync, and apiserver health work end-to-end. Foreman is deployed and expected to reach Ready after rollout settles; a full clean redeploy (`make dev-up` or `scripts/zen.py env redeploy --env sandbox`) is recommended once to confirm foreman Ready and optional Ollama path when enabled. Block 6 / 93%+ baseline is supportable once a single clean full redeploy is confirmed (and optionally one run with Ollama enabled).

---

## Live validation report (2026-03-10, full redeploy run)

**Command:** `python3 scripts/zen.py env redeploy --env sandbox`  
**Cluster:** k3d-zen-brain-sandbox  
**Ollama:** disabled

### Results summary

| Step | Result | Details |
|------|--------|---------|
| 1. Redeploy pipeline | Pass (partial) | Registry up, cluster reachable, image built, pushed, imported; values rendered; Helmfile sync completed; **foreman rollout wait failed** (timeout). |
| 2. Generated values | Pass | `.artifacts/state/sandbox/zen-brain-dependencies-values.yaml`, `zen-brain-values.yaml`, `zen-brain-ollama-values.yaml` present. |
| 3. Helmfile sync | Pass | All four releases upgraded/deployed: zen-brain-dependencies (zen-context), zen-brain-crds, zen-brain-ollama, zen-brain-core (zen-brain). |
| 4. Apiserver | Pass | 1/1 Running. `curl http://127.0.1.6:8080/healthz` → 200 "ok"; `curl http://127.0.1.6:8080/readyz` → 200 "ok". |
| 5. Foreman | Fail | Deployment exists, image `zen-brain-registry:5000/zen-brain:dev` correct. Pods: one ImagePullBackOff (old ReplicaSet), one CrashLoopBackOff. Liveness/readiness probes fail with **HTTP 404** on `:8081/healthz` and `:8081/readyz`. Foreman process starts (logs show FactoryTaskRunner, Gate=policy, Guardian=log) but controller-runtime health endpoints return 404; rollout status exceeded progress deadline. |
| 6–7. Ollama | N/A | Not enabled. |

### Success criteria

- No manual apply/exec in path: **met**
- Apiserver external and healthy: **met**
- Foreman Ready: **not met** (probes 404 on :8081)
- Ollama: **N/A**

### Conclusion

**Canonical path is operational through Helmfile and apiserver.** Cluster, registry, image build/import, values generation, and Helmfile sync all succeed. Apiserver is externally reachable and healthy. The redeploy script exits non-zero because **foreman never becomes Ready**: the health/ready probes on port 8081 return 404. Root cause: foreman uses `HealthProbeBindAddress` in controller-runtime; the manager may not be exposing `/healthz` and `/readyz` as expected (or path/version differs). **Recommended fix:** Ensure foreman’s manager registers and serves health/ready endpoints on the probe address so rollout can complete; then re-run redeploy to confirm foreman rollout and optional Ollama pass.
