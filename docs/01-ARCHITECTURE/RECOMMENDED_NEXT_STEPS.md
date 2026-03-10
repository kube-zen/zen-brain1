# Recommended Next Steps

**Purpose:** Map high-ROI waves to current status and execution order. Keeps repo operationally clean, governance-clean, and deployment-coherent before deeper intelligence work.

**Overall:** Helm/Helmfile is the canonical deployment path; `config/clusters.yaml` is the env contract; Ollama enablement and model preload are declarative.

---

## Wave 1 — Quick cleanup, highest ROI ✅ DONE

| Step | Status | Notes |
|------|--------|--------|
| Move all runtime state under ZEN_BRAIN_HOME (sessions, cache-like output); repo tree source-only | ✅ | Sessions: `$ZEN_BRAIN_HOME/sessions` or `ZEN_BRAIN_DATA_DIR`; no repo-local runtime state |
| Move coverage out of root | ✅ | `.artifacts/coverage/coverage.out`; Makefile + .gitignore updated |
| Standardize config ownership | ✅ | Canonical runtime config: `~/.zen-brain/config.yaml`; `configs/` templates only; loader uses single path, no broad search fallback |
| Retire docker-compose.zencontext.yml | ✅ | Removed; k8s (k3d + deployments/zencontext-in-cluster) is the only local dependency path |

---

## Wave 2 — Restore governance and repo credibility

| Step | Status | Notes |
|------|--------|--------|
| Fix zen_sdk_ownership | ✅ | Gate runs in CI and pre-commit; allowlist documented in DEPENDENCIES.md; [ADR-0009](ADR/0009_ZEN_SDK_OWNERSHIP_ALLOWLIST.md) approves allowlist for domain usage and approved wrappers. |

---

## Wave 3 — Make deployment coherent ✅

| Step | Status | Notes |
|------|--------|--------|
| Add Python deployment CLI | ✅ | `config/clusters.yaml`, `scripts/zen.py`, `scripts/common/{config,k3d,hosts,registry,env}.py`, 127.0.1.x addressing; `make dev-up` / `dev-down` / `dev-image` are thin wrappers (default env: sandbox; override with `ZEN_DEV_ENV`). |

---

## Wave 4 — Make Block 5 real ✅

| Step | Status | Notes |
|------|--------|--------|
| Implement actual Ollama provider wiring | ✅ | `internal/llm/ollama_provider.go`; gateway uses it when `OLLAMA_BASE_URL` is set |
| In-cluster Ollama deployment | ✅ | **Helm/Helmfile canonical.** Charts: `zen-brain-crds`, `zen-brain-dependencies`, `zen-brain`, `zen-brain-ollama`. `config/clusters.yaml` → generated values → Helmfile sync. Ollama model preload via Helm hook Job (declarative); no manual `kubectl exec ... ollama pull` in standard path. |

---

## Practical execution order

1. **Cleanup paths and repo outputs** — ✅ Done (Wave 1).
2. **Delete compose path and normalize docs** — ✅ Done (Wave 1).
3. **Fix zen_sdk_ownership gate** — ✅ Enforce in pre-commit; allowlist documented in DEPENDENCIES.md; ADR-0009 approves allowlist.
4. **Add Python deployment CLI** — ✅ Done (Wave 3); Makefile dev-up/dev-down/dev-image are thin wrappers.
5. **Implement real Ollama provider + k8s deployment** — ✅ Done (Wave 4).

---

## Bottom line

- **Architecturally strong;** current truth:
  - **Operationally clean:** Yes (Wave 1 done).
  - **Governance-clean:** Yes (zen_sdk_ownership in pre-commit; ADR-0009 + DEPENDENCIES.md).
  - **Ollama-backed:** Yes; optional in-cluster Ollama via `deploy.use_ollama` and `deployments/ollama-in-cluster`.
  - **Fully k8s-opinionated:** Yes for local dev (k3d); Python CLI will standardize dev-up/dev-down.

Waves 1–4 complete. Next: deeper intelligence work or further Block 5 tuning (e.g. model selection, resource limits).
