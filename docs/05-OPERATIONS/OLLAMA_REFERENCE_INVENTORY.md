# Ollama Reference Inventory

**Date:** 2026-03-26  
**Scope:** Full repo scan of ~/zen/zen-brain1 (excluding .git)  
**Search patterns:** `ollama`, `OLLAMA`, `11434`, `OLLAMA_BASE_URL`, `use_ollama`, `ollama_provider`  
**Command:** `find . -path ./.git -prune -o -type f -print0 | xargs -0 grep -il "ollama"`

## 1. Scope

- **Total files with Ollama references:** 147
- **Total matching lines:** 1,644
- **Binary/cache files:** 8 (bin/*, __pycache__/*.pyc, foreman)

## 2. Evidence Sources

- Full repo text search (grep -il, case-insensitive)
- Per-file line-level detail captured in /tmp/ollama_details.txt (1,644 lines)
- Categorization based on file path and content context

## 3. Summary Counts

| Category | Files | Description |
|----------|-------|-------------|
| **Runtime Code** | 12 | Go source files in cmd/, internal/ — active provider, gateway, factory |
| **Config** | 18 | YAML config files in config/policy/, config/clusters.yaml, chart values |
| **Docs** | 57 | Markdown files across docs/01-ARCHITECTURE, docs/03-DESIGN, docs/04-DEVELOPMENT, docs/05-OPERATIONS |
| **Scripts** | 10 | Shell/Python scripts in scripts/ |
| **Tests** | 4 | Go test files and test configs |
| **Charts/Deployments** | 15 | Helm charts (zen-brain-ollama), deployment manifests |
| **Historical Evidence** | 20 | Status reports, execution reports, brain tasks, rescue tasks |
| **Binary/Cache** | 8 | Compiled binaries and __pycache__ |

## 4. Top Findings

| Priority | Area | File | Evidence | Why it matters | Suggested action |
|----------|------|------|----------|----------------|-------------------|
| P0 | Runtime Code | `internal/llm/ollama_provider.go` | Full Ollama provider implementation (340+ lines) | ACTIVE — real provider for L0 fallback lane | **KEEP** — active runtime |
| P0 | Runtime Code | `internal/llm/ollama_warmup.go` | OllamaWarmupCoordinator implementation | ACTIVE — warmup for L0 fallback lane | **KEEP** — active runtime |
| P0 | Runtime Code | `internal/llm/gateway.go` | OLLAMA_BASE_URL env, Ollama provider creation | ACTIVE — gateway wiring for L0 | **KEEP** — active runtime |
| P0 | Runtime Code | `internal/llm/ollama_provider_test.go` | Tests for Ollama provider | ACTIVE — tests for active provider | **KEEP** — active runtime |
| P0 | Runtime Code | `internal/factory/factory.go` | `case "ollama":` provider creation | ACTIVE — factory routing | **KEEP** — active runtime |
| P0 | Runtime Code | `internal/foreman/factory_runner.go` | Ollama provider creation for factory tasks | ACTIVE — foreman factory path | **KEEP** — active runtime |
| P0 | Runtime Code | `internal/apiserver/chat.go` | `OllamaWarmupCoordinator` param | ACTIVE — apiserver wiring | **KEEP** — active runtime |
| P0 | Runtime Code | `cmd/apiserver/main.go` | OLLAMA env vars, warmup coordinator | ACTIVE — apiserver startup | **KEEP** — active runtime |
| P1 | Config | `config/policy/providers.yaml` | Local Ollama provider definition | FALLBACK config — clearly marked fallback | **KEEP** — mark as fallback-only |
| P1 | Config | `config/policy/routing.yaml` | forbid_in_cluster_ollama rules | POLICY enforcement | **KEEP** — active policy |
| P1 | Config | `config/policy/mlq-levels.yaml` | L0 fallback config pointing to Ollama | FALLBACK config | **KEEP** — mark as fallback-only |
| P1 | Config | `config/clusters.yaml` | use_ollama: false, host_ollama_base_url | DEPLOYMENT config | **KEEP** — correctly disabled |
| P2 | Docs | `docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md` | Full 700-line Ollama operations guide | LARGEST doc — describes Ollama as certified path | **REVIEW** — mark as fallback-only reference |
| P2 | Docs | `docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md` | Ollama warmup procedures | Active warmup reference | **KEEP** — mark as fallback-only |
| P2 | Docs | `docs/04-DEVELOPMENT/CONFIGURATION.md` | Describes Ollama as primary local worker | MISLEADING — llama.cpp is now primary | **UPDATE** — clarify fallback status |
| P2 | Docs | `docs/04-DEVELOPMENT/SETUP.md` | Ollama setup instructions | OUTDATED — still presents as main path | **UPDATE** — clarify fallback status |
| P3 | Charts | `charts/zen-brain-ollama/` (4 files) | Full Helm chart for in-cluster Ollama | DEAD — use_ollama: false everywhere | **QUARANTINE** — mark deprecated |
| P3 | Charts | `deploy/helmfile/zen-brain/helmfile.yaml.gotmpl` | References zen-brain-ollama release | INACTIVE — skipped via selector | **KEEP** — still wired for optional deploy |
| P3 | Deployments | `deployments/ollama-in-cluster/` (2 files) | Legacy in-cluster Ollama manifests | DEAD — README explicitly says LEGACY/UNSUPPORTED | **QUARANTINE** — mark deprecated |
| P3 | Historical | `p17c_results/` (5 files) | Ollama benchmark results | HISTORICAL — benchmark evidence | **KEEP** — historical evidence |
| P3 | Historical | `ZB-026F_status.md`, `ZB-026F_SUCCESS.md` | Status reports mentioning Ollama | HISTORICAL | **KEEP** — historical evidence |
| P3 | Historical | `rescue-tasks*.yaml` (4 files) | "Keep current Ollama/qwen3.5:0.8b path working" | HISTORICAL — task templates | **UPDATE** — clarify current runtime |

## 5. Category Breakdown

### 5.1 Runtime Code (12 files)

| File | Lines | Status |
|------|-------|--------|
| `internal/llm/ollama_provider.go` | ~340 | ACTIVE — core provider |
| `internal/llm/ollama_warmup.go` | ~215 | ACTIVE — warmup coordinator |
| `internal/llm/ollama_provider_test.go` | ~190 | ACTIVE — unit tests |
| `internal/llm/gateway.go` | ~30 | ACTIVE — gateway wiring |
| `internal/llm/local_worker.go` | 1 | COMMENT — mentions Ollama |
| `internal/llm/README.md` | 3 | COMMENT — mentions Ollama |
| `internal/factory/factory.go` | 3 | ACTIVE — provider case |
| `internal/factory/llm_gate_test.go` | 3 | TEST — skip if no Ollama |
| `internal/factory/llm_integration_test.go` | ~30 | TEST — Ollama integration |
| `internal/factory/llm_generator_policy.go.broken` | ~20 | DEAD — .broken file |
| `internal/foreman/factory_runner.go` | ~15 | ACTIVE — factory runner |
| `internal/apiserver/chat.go` | 1 | ACTIVE — param type |
| `internal/ingestion/jira_to_braintask.go` | 1 | COMMENT — policy ref |
| `internal/integration/real_inference_test.go` | ~50 | TEST — integration |
| `internal/mlq/task_executor_test.go` | 2 | TEST — mock config |
| `pkg/llm/provider.go` | 1 | COMMENT — doc string |

### 5.2 Config (18 files)

| File | Lines | Status |
|------|-------|--------|
| `config/policy/providers.yaml` | 12 | ACTIVE — provider definition |
| `config/policy/routing.yaml` | 15 | ACTIVE — routing policy |
| `config/policy/mlq-levels.yaml` | 10 | ACTIVE — L0 fallback config |
| `config/policy/mlq-levels-local.yaml` | 5 | ACTIVE — local fallback |
| `config/policy/mlq-worker-pool.yaml` | 2 | ACTIVE — worker pool |
| `config/policy/chains.yaml` | 3 | COMMENT — policy header |
| `config/policy/prompts.yaml` | 3 | COMMENT — policy header |
| `config/policy/roles.yaml` | 3 | COMMENT — policy header |
| `config/policy/tasks.yaml` | 3 | COMMENT — policy header |
| `config/policy/README.md` | 4 | ACTIVE — MLQ lane table |
| `config/clusters.yaml` | 20 | ACTIVE — deploy config |
| `config/profiles/local-cpu-45m.yaml` | 4 | ACTIVE — profile |
| `configs/config.example.yaml` | 1 | COMMENT — env vars |
| `.artifacts/state/sandbox/zen-brain-values.yaml` | 3 | GENERATED — chart values |

### 5.3 Docs (57 files)

Largest concentrations:
- `docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md` — **710 lines** of Ollama operations
- `docs/05-OPERATIONS/WARMUP_FULL_REPORT.md` — **277 lines** of warmup reference
- `docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md` — **90 lines**
- `docs/04-DEVELOPMENT/CONFIGURATION.md` — **254 lines** (misleading primary status)
- `docs/04-DEVELOPMENT/DEPLOYMENT_VALIDATION.md` — **105 lines**
- `docs/05-OPERATIONS/LLAMA_CPP_VS_OLLAMA_QWEN_0_8B_BENCHMARK.md` — **278 lines** (benchmark)

### 5.4 Scripts (10 files)

| File | Lines | Status |
|------|-------|--------|
| `scripts/check-proven-lane.sh` | 10 | ACTIVE — policy enforcement |
| `scripts/ci/local_model_policy_gate.py` | 35 | ACTIVE — CI gate |
| `scripts/ci/local_cpu_profile_gate.py` | 15 | ACTIVE — CI gate |
| `scripts/ci/timeout_compliance_gate.py` | 8 | ACTIVE — CI gate |
| `scripts/common/config.py` | 65 | ACTIVE — deploy helper |
| `scripts/common/env.py` | 8 | ACTIVE — deploy helper |
| `scripts/common/helmfile_values.py` | 25 | ACTIVE — values generation |
| `scripts/health-check.sh` | 2 | ACTIVE — health check |
| `scripts/zen-mesh-operator-loop.sh` | 6 | ACTIVE — operator loop |
| `scripts/proof_local_worker_chat.py` | 2 | ACTIVE — proof script |

### 5.5 Charts/Deployments (15 files)

| File | Lines | Status |
|------|-------|--------|
| `charts/zen-brain-ollama/Chart.yaml` | 2 | INACTIVE — disabled by default |
| `charts/zen-brain-ollama/README.md` | 15 | INACTIVE — explicitly says use host Docker |
| `charts/zen-brain-ollama/templates/` (3 files) | 80 | INACTIVE — StatefulSet + VPA + preload |
| `charts/zen-brain-ollama/values.yaml` | 21 | INACTIVE — chart values |
| `charts/zen-brain/templates/apiserver.yaml` | 7 | ACTIVE — env var passthrough |
| `charts/zen-brain/values.yaml` | 15 | ACTIVE — chart values |
| `deployments/ollama-in-cluster/ollama.yaml` | 50 | DEAD — explicitly LEGACY/UNSUPPORTED |
| `deployments/ollama-in-cluster/README.md` | 49 | DEAD — explicitly LEGACY/UNSUPPORTED |
| `deployments/k3d/apiserver.yaml` | 7 | ACTIVE — env vars |
| `deployments/k3d/foreman.yaml` | 3 | ACTIVE — foreman flags |
| `deployments/k3d/README.md` | 1 | COMMENT |
| `deployments/k3d/test-braintask.yaml` | 1 | COMMENT |
| `deploy/helmfile/zen-brain/helmfile.yaml.gotmpl` | 6 | ACTIVE — wired for optional deploy |

### 5.6 Tests (4 files)

| File | Lines | Status |
|------|-------|--------|
| `internal/llm/ollama_provider_test.go` | ~190 | ACTIVE — provider tests |
| `internal/factory/llm_gate_test.go` | 3 | ACTIVE — skip guards |
| `internal/factory/llm_integration_test.go` | ~30 | ACTIVE — integration tests |
| `internal/integration/real_inference_test.go` | ~50 | ACTIVE — e2e tests |
| `internal/mlq/task_executor_test.go` | 2 | TEST — mock config |

### 5.7 Historical Evidence (20 files)

Status reports, execution reports, brain task definitions, rescue tasks, memory files — all historical. See full list in `/tmp/ollama_details.txt`.

## 6. Quick Wins (safe to clean immediately)

1. **`internal/factory/llm_generator_policy.go.broken`** — .broken file, no longer compiled
2. **`scripts/common/__pycache__/*.pyc`** (3 files) — compiled bytecode cache
3. **`scripts/__pycache__/zen.cpython-312.pyc`** — compiled bytecode cache
4. **`bin/*`** (4 files) — compiled binaries, references inside cannot be edited

## 7. Higher-Risk Items Needing Review

1. **`docs/04-DEVELOPMENT/CONFIGURATION.md`** line 66 — says "local-worker lane uses the real Ollama provider" as if it's the primary path. Needs rewrite to clarify fallback status.
2. **`docs/04-DEVELOPMENT/SETUP.md`** — extensive Ollama setup instructions presented as primary path
3. **`docs/04-DEVELOPMENT/DEPLOYMENT_VALIDATION.md`** — 105 lines of Ollama validation steps presented as required
4. **`deployments/ollama-in-cluster/`** — README says LEGACY but files could still be accidentally applied
5. **`config/policy/mlq-levels.yaml`** line 11 — says "Fallback: Current working control backend (Ollama)" which could be misread as primary

## 8. Suggested Next Actions

1. **Produce classification report** — classify all 147 files into A/B/C/D buckets
2. **Slice A: Docs-only cleanup** — update misleading docs that present Ollama as primary
3. **Slice B: Config clarification** — add fallback-only labels where missing
4. **Slice C: Quarantine dead deployments** — move or mark deprecated charts/deployments
5. **Slice D: Script cleanup** — clean .broken file, __pycache__
6. **Slice E: Runtime code** — only if proven dead (do NOT touch active provider)
