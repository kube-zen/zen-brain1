# Ollama Cleanup Candidates

**Date:** 2026-03-26  
**Based on:** OLLAMA_REFERENCE_INVENTORY.md + OLLAMA_CLASSIFICATION.md  
**Active Runtime:** llama.cpp (L1/L2 primary, L0/Ollama fallback only)

## Scope

147 files with Ollama references. This report identifies candidates for removal, update, or quarantine, organized into bounded cleanup slices.

## Slice A — Docs That Mislead (Present Ollama as Primary)

These docs describe Ollama as the active/primary local worker path when llama.cpp is now the primary runtime. They need updates to clarify fallback status.

| Priority | File | Lines | Issue | Action |
|----------|------|-------|-------|--------|
| P2 | `docs/04-DEVELOPMENT/CONFIGURATION.md` | 254 | Line 66: "local-worker lane uses the real Ollama provider" — no mention of llama.cpp as primary | Add header: "NOTE: Primary runtime is llama.cpp. Ollama is L0 fallback only." |
| P2 | `docs/04-DEVELOPMENT/SETUP.md` | 105+ | Extensive Ollama setup as primary path, no llama.cpp setup | Add llama.cpp as primary, reframe Ollama as fallback |
| P2 | `docs/05-OPERATIONS/RELEASE_CHECKLIST.md` | 85 | "Real inference path validated (Client → Gateway → Local-Worker → Ollama)" | Update to note llama.cpp primary, Ollama fallback |
| P2 | `docs/04-DEVELOPMENT/DEPLOYMENT_VALIDATION.md` | 105 | 105 lines of Ollama validation as if required | Add note that Ollama steps are for fallback validation only |
| P2 | `docs/05-OPERATIONS/PRODUCTION_PATH_DEFAULTS_FIX.md` | 250 | Describes Ollama URL fixes as production defaults | Add context that llama.cpp is now primary |
| P2 | `docs/05-OPERATIONS/QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md` | 83 | "Zen-brain production paths may use Ollama or llama.cpp" — equal framing | Clarify llama.cpp is primary |
| P2 | `docs/05-OPERATIONS/PROOF/PROOF_OF_WORKING_LANE.md` | 304 | "Real Ollama Path Confirmed" as main proof | Add context: this proves fallback lane |
| P2 | `docs/05-OPERATIONS/PROOF/KNOWN_GOOD_CONFIG.md` | 274 | Host Docker Ollama as canonical path | Add context: fallback lane config |
| P2 | `docs/05-OPERATIONS/ZEN_MESH_OPERATOR_GUIDE.md` | 385 | "Trusted Foundation: Host Docker Ollama" | Reframe as fallback |
| P2 | `docs/05-OPERATIONS/24_7_USEFUL_OPERATIONS_RUNBOOK.md` | 169 | L0 as active row in runbook | Keep but clarify fallback-only |

## Slice B — Config Clarification

Config files where Ollama is referenced but not clearly labeled as fallback.

| Priority | File | Issue | Action |
|----------|------|-------|--------|
| P1 | `config/policy/mlq-levels.yaml` | Line 11: "Fallback: Current working control backend (Ollama)" | Add "FALLBACK ONLY — not for regular work" |
| P1 | `config/policy/mlq-levels-local.yaml` | Fallback config, not labeled | Add "FALLBACK ONLY" comment |
| P1 | `deployments/k3d/apiserver.yaml` | Hardcoded OLLAMA env vars | Add comment: "FALLBACK: Ollama L0 — llama.cpp is primary" |
| P1 | `deployments/k3d/foreman.yaml` | `--factory-llm-provider=ollama` hardcoded | Add comment about fallback status |

## Slice C — Dead Code/Cache Cleanup

Safe to remove immediately.

| Priority | File | Why Dead | Action |
|----------|------|----------|--------|
| P3 | `internal/factory/llm_generator_policy.go.broken` | .broken file — never compiled | Delete |
| P3 | `scripts/common/__pycache__/config.cpython-312.pyc` | Bytecode cache | Delete |
| P3 | `scripts/common/__pycache__/env.cpython-312.pyc` | Bytecode cache | Delete |
| P3 | `scripts/common/__pycache__/helmfile_values.cpython-312.pyc` | Bytecode cache | Delete |
| P3 | `scripts/__pycache__/zen.cpython-312.pyc` | Bytecode cache | Delete |

## Slice D — Dead/Deprecated Charts and Deployments

| Priority | File | Why Dead | Action |
|----------|------|----------|--------|
| P3 | `deployments/ollama-in-cluster/ollama.yaml` | Explicitly LEGACY/UNSUPPORTED, no longer in canonical path | Quarantine: move to `deployments/ollama-in-cluster/.deprecated/` or add prominent deprecation header |
| P3 | `deployments/ollama-in-cluster/README.md` | Explicitly LEGACY/UNSUPPORTED | Add deprecation header or quarantine |
| P3 | `charts/zen-brain-ollama/templates/preload-job.yaml` | Part of disabled chart | Keep (part of B-bucket chart) but add comment |
| P3 | `charts/zen-brain-ollama/templates/vpa.yaml` | Part of disabled chart | Keep (part of B-bucket chart) but add comment |
| P3 | `deployments/k3d/test-braintask.yaml` | Test references Ollama as local path | Update or delete |

## Slice E — Runtime Code (ONLY if proven dead)

**⚠️ DO NOT execute without explicit supervisor approval.**

| Priority | File | Evidence | Action |
|----------|------|----------|--------|
| P0 | `internal/llm/local_worker.go` | Line 52: comment "In production, this would call Ollama, vLLM, or similar" | Update comment to reflect llama.cpp is primary |
| P0 | `cmd/zen-brain/analyze.go` | Uses Ollama provider for analysis | **KEEP** — active code path |
| P0 | `cmd/zen-brain/factory.go` | Line 55: "requires OLLAMA_BASE_URL" help text | Update to mention llama.cpp also supported |

## Items to NOT Remove

These must stay regardless:

| File | Why |
|------|-----|
| `internal/llm/ollama_provider.go` | Active L0 fallback provider |
| `internal/llm/ollama_warmup.go` | Active warmup coordinator |
| `internal/llm/ollama_provider_test.go` | Active tests |
| `internal/llm/gateway.go` | Active gateway wiring |
| `internal/factory/factory.go` | Active provider routing |
| `internal/foreman/factory_runner.go` | Active factory runner |
| `cmd/apiserver/main.go` | Active apiserver startup |
| `internal/apiserver/chat.go` | Active handler |
| `config/policy/providers.yaml` | Active policy |
| `config/policy/routing.yaml` | Active policy enforcement |
| `config/policy/mlq-levels.yaml` | Active MLQ config |
| `scripts/ci/local_model_policy_gate.py` | Active CI gate |
| `scripts/ci/local_cpu_profile_gate.py` | Active CI gate |
| `scripts/common/config.py` | Active deploy helper |
| `scripts/common/helmfile_values.py` | Active values generation |
| `scripts/check-proven-lane.sh` | Active policy enforcement |
| `docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md` | Comprehensive fallback reference |
| `docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md` | Active fallback procedures |
| `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md` | Correctly marks L0 as fallback |
| `docs/03-DESIGN/SMALL_MODEL_STRATEGY.md` | Correctly marks L0 as fallback |
| `charts/zen-brain-ollama/*` | Wired via helmfile, deployable on operator request |
| All p17c_results/* | Historical benchmark evidence |
| All ZB-*_status_report.md | Historical evidence |

## Recommended Execution Order

1. **Slice C** (cache/.broken cleanup) — zero risk, immediate
2. **Slice A** (docs clarification) — medium effort, high value
3. **Slice B** (config clarification) — low effort, medium value
4. **Slice D** (deprecated deployments) — low effort, reduces confusion
5. **Slice E** (runtime code comments) — only if explicitly approved

## Validation After Each Slice

```bash
# 1. Search validation
rg -n "ollama|OLLAMA|11434|OLLAMA_BASE_URL|use_ollama" . | wc -l

# 2. Build validation
cd ~/zen/zen-brain1 && go build ./...

# 3. Test validation
cd ~/zen/zen-brain1 && go test ./internal/llm/... ./internal/factory/... -count=1
```
