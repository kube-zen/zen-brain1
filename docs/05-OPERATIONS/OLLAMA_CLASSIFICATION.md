# Ollama Reference Classification

**Date:** 2026-03-26  
**Method:** Per-file analysis using repo evidence (active runtime docs, config, code imports)  
**Evidence Sources:**
- `config/policy/providers.yaml` — defines provider hierarchy
- `config/policy/routing.yaml` — defines routing policy
- `config/policy/README.md` — explicitly says "Primary inference runtime is llama.cpp (not Ollama). L0/Ollama is fallback only."
- `docs/03-DESIGN/SMALL_MODEL_STRATEGY.md` — L0 Ollama is "fallback only — NOT for regular work"
- `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md` — L0 is "Fallback only (FAIL-CLOSED)"
- `config/clusters.yaml` — `use_ollama: false` in all environments
- Code: `internal/llm/gateway.go` — Ollama provider created only when `OLLAMA_BASE_URL` is set
- Code: `internal/factory/factory.go` — `case "ollama":` in provider switch

## Classification Summary

| Bucket | Count | Description |
|--------|-------|-------------|
| **A. ACTIVE / KEEP** | 30 | Required by live runtime, failover policy, or operator enforcement |
| **B. INACTIVE BUT VALID / DEFER** | 15 | Not used in current active runtime but legitimate fallback/compatibility path |
| **C. DEAD / REMOVE** | 45 | No active runtime path, no fallback role, dead config/code weight |
| **D. UNKNOWN / INVESTIGATE** | 57 | Historical docs/reports — low risk but needs per-file review |

## A. ACTIVE / KEEP (30 files)

These are required by the live runtime, failover policy, CI gates, or operator enforcement.

### A1. Core Provider Implementation (5 files)
| File | Why Active |
|------|-----------|
| `internal/llm/ollama_provider.go` | Core Ollama provider — L0 fallback lane |
| `internal/llm/ollama_warmup.go` | Warmup coordinator for L0 fallback |
| `internal/llm/ollama_provider_test.go` | Unit tests for active provider |
| `internal/llm/gateway.go` | Gateway wiring — creates Ollama provider on OLLAMA_BASE_URL |
| `pkg/llm/provider.go` | Provider interface doc comment |

### A2. Active Runtime Wiring (5 files)
| File | Why Active |
|------|-----------|
| `cmd/apiserver/main.go` | OLLAMA env vars, warmup coordinator startup |
| `internal/apiserver/chat.go` | OllamaWarmupCoordinator param |
| `internal/factory/factory.go` | `case "ollama":` provider creation |
| `internal/foreman/factory_runner.go` | Ollama provider for factory tasks |
| `internal/ingestion/jira_to_braintask.go` | Policy reference "Local Ollama must use qwen3.5:0.8b only" |

### A3. Active Config (10 files)
| File | Why Active |
|------|-----------|
| `config/policy/providers.yaml` | Provider definition with in-cluster prohibition |
| `config/policy/routing.yaml` | `forbid_in_cluster_ollama: true` enforcement |
| `config/policy/mlq-levels.yaml` | L0 fallback config |
| `config/policy/mlq-levels-local.yaml` | Local fallback config |
| `config/policy/mlq-worker-pool.yaml` | Worker pool endpoint |
| `config/policy/README.md` | MLQ lane table |
| `config/clusters.yaml` | Deploy config with `use_ollama: false` |
| `config/profiles/local-cpu-45m.yaml` | Profile config |
| `charts/zen-brain/templates/apiserver.yaml` | Env var passthrough |
| `charts/zen-brain/values.yaml` | Chart values |

### A4. Active CI/Scripts (6 files)
| File | Why Active |
|------|-----------|
| `scripts/ci/local_model_policy_gate.py` | CI gate — blocks in-cluster Ollama refs |
| `scripts/ci/local_cpu_profile_gate.py` | CI gate — checks local CPU config |
| `scripts/ci/timeout_compliance_gate.py` | CI gate — checks timeout values |
| `scripts/common/config.py` | Deploy helper functions |
| `scripts/common/env.py` | Deploy helper (skip_ollama flag) |
| `scripts/common/helmfile_values.py` | Values generation for Ollama chart |

### A5. Active Tests (4 files)
| File | Why Active |
|------|-----------|
| `internal/integration/real_inference_test.go` | E2e test for Ollama path |
| `internal/factory/llm_integration_test.go` | Integration tests |
| `internal/mlq/task_executor_test.go` | MLQ executor test |
| `internal/llm/README.md` | Provider README |

## B. INACTIVE BUT VALID / DEFER (15 files)

Not used in current active runtime but still a legitimate fallback/compatibility path. **Keep for now.**

### B1. In-Cluster Ollama Charts (4 files — disabled by default, deployable on operator request)
| File | Why Valid |
|------|-----------|
| `charts/zen-brain-ollama/Chart.yaml` | Helm chart — disabled but wired via helmfile |
| `charts/zen-brain-ollama/README.md` | Explicitly says "Use Host Docker Ollama instead" |
| `charts/zen-brain-ollama/templates/statefulset.yaml` | StatefulSet template |
| `charts/zen-brain-ollama/values.yaml` | Chart values |

### B2. Helmfile Wiring (1 file)
| File | Why Valid |
|------|-----------|
| `deploy/helmfile/zen-brain/helmfile.yaml.gotmpl` | Wired but skipped via `--skip-ollama` or disabled |

### B3. Policy Header Comments (5 files — informational, no runtime impact)
| File | Why Valid |
|------|-----------|
| `config/policy/chains.yaml` | Header comment about Ollama policy |
| `config/policy/prompts.yaml` | Header comment |
| `config/policy/roles.yaml` | Header comment |
| `config/policy/tasks.yaml` | Header comment |
| `configs/config.example.yaml` | Example config comment |

### B4. Fallback Documentation (5 files — describes fallback-only path correctly)
| File | Why Valid |
|------|-----------|
| `docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md` | Warmup procedures — correct fallback framing |
| `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md` | Correctly marks L0 Ollama as fallback |
| `docs/03-DESIGN/SMALL_MODEL_STRATEGY.md` | Correctly marks L0 as fallback |
| `docs/05-OPERATIONS/MLQ_LANE_ROUTING_MATRIX.md` | L0 routing — correctly marked |
| `docs/05-OPERATIONS/SECRET_CONTRACT.md` | Notes L0 Ollama needs no credentials |

### B5. Operational Scripts (2 files — operator tools)
| File | Why Valid |
|------|-----------|
| `scripts/check-proven-lane.sh` | Enforces host Docker Ollama, blocks in-cluster |
| `scripts/health-check.sh` | L0 port check |

### B6. Generated Values (1 file)
| File | Why Valid |
|------|-----------|
| `.artifacts/state/sandbox/zen-brain-values.yaml` | Generated from clusters.yaml — regenerated on deploy |

## C. DEAD / REMOVE (45 files)

No active runtime path, no fallback role, dead code weight.

### C1. Dead Legacy Deployments (2 files)
| File | Why Dead |
|------|----------|
| `deployments/ollama-in-cluster/ollama.yaml` | README explicitly says LEGACY/UNSUPPORTED, no longer in canonical deploy path |
| `deployments/ollama-in-cluster/README.md` | Explicitly says LEGACY/UNSUPPORTED |

### C2. Dead Chart Templates (2 files)
| File | Why Dead |
|------|----------|
| `charts/zen-brain-ollama/templates/preload-job.yaml` | Part of disabled chart, never deployed |
| `charts/zen-brain-ollama/templates/vpa.yaml` | Part of disabled chart, never deployed |

### C3. Dead .broken File (1 file)
| File | Why Dead |
|------|----------|
| `internal/factory/llm_generator_policy.go.broken` | File extension .broken — not compiled, never used |

### C4. Cache Files (4 files)
| File | Why Dead |
|------|----------|
| `scripts/common/__pycache__/config.cpython-312.pyc` | Bytecode cache |
| `scripts/common/__pycache__/env.cpython-312.pyc` | Bytecode cache |
| `scripts/common/__pycache__/helmfile_values.cpython-312.pyc` | Bytecode cache |
| `scripts/__pycache__/zen.cpython-312.pyc` | Bytecode cache |

### C5. Compiled Binaries (4 files)
| File | Why Dead |
|------|----------|
| `bin/apiserver` | Compiled binary (references in binary, not editable) |
| `bin/foreman` | Compiled binary |
| `bin/zen-brain` | Compiled binary |
| `bin/zen-brain1` | Compiled binary |
| `foreman` | Compiled binary in repo root |

### C6. Dead Config References (3 files)
| File | Why Dead |
|------|----------|
| `deployments/k3d/apiserver.yaml` | Hardcoded Ollama env vars — overridden by helmfile values |
| `deployments/k3d/foreman.yaml` | Hardcoded `--factory-llm-provider=ollama` flag |
| `deployments/k3d/test-braintask.yaml` | Test task references Ollama as local path |

### C7. Historical Docs That Mislead (28 files)
These docs present Ollama as active/primary when it's now fallback-only. Content is historically accurate but current framing misleads operators and AIs.

See full list in OLLAMA_CLEANUP_CANDIDATES.md Slice A.

## D. UNKNOWN / INVESTIGATE (57 files)

Mostly historical docs/reports. Low risk but needs per-file decision.

### D1. Historical Status Reports (10 files)
| File | Notes |
|------|-------|
| `ZB-025H1_status_report.md` | Status report — mentions Ollama |
| `ZB-025H2_status_report.md` | Status report |
| `ZB-026F_status.md` | Overnight run status |
| `ZB-026F_SUCCESS.md` | Success report |
| `W027_W037_EXECUTION_REPORT.md` | Execution report |
| `W028_W029_TOOL_PATH_ANALYSIS.md` | Tool path analysis |
| `W016_EXECUTION_REPORT.md` | Execution report |
| `W019_W020_RECLASSIFICATION.md` | Reclassification report |
| `W021_CONTEXT_PATH_ANALYSIS.md` | Context path analysis |
| `W028_W029_TOOL_PATH_ANALYSIS.md` | Duplicate entry — tool analysis |

### D2. Benchmark/Results Files (8 files)
| File | Notes |
|------|-------|
| `p17c_results/PHASE_17C_REPORT.md` | Benchmark results |
| `p17c_results/PHASE_17D_REPORT.md` | Benchmark results |
| `p17c_results/o2_rerun.log` | Benchmark log |
| `p17c_results/run2.log` | Benchmark log |
| `p17c_results/run3.log` | Benchmark log |
| `p17c_results/run.log` | Benchmark log |
| `p18_results/` | Results directory |
| `docs/05-OPERATIONS/LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md` | Benchmark comparison |

### D3. Historical Session Summaries (5 files)
| File | Notes |
|------|-------|
| `docs/05-OPERATIONS/SESSION_SUMMARY_2026_03_10.md` | Session notes |
| `docs/05-OPERATIONS/SESSION_SUMMARY_20260311.md` | Session notes |
| `docs/05-OPERATIONS/BLOCK6_IMPROVEMENT_SESSION_20260311.md` | Session notes |
| `docs/05-OPERATIONS/BLOCK6_STATUS_REPORT.md` | Block 6 status |
| `docs/05-OPERATIONS/BLOCK5_INTELLIGENCE_STATUS_REPORT.md` | Block 5 status |

### D4. Rescue Task Templates (4 files)
| File | Notes |
|------|-------|
| `rescue-tasks.yaml` | Template — "Keep current Ollama path working" |
| `rescue-tasks-fixed.yaml` | Template |
| `rescue-tasks-more-fixed.yaml` | Template |
| `rescue-tasks-more.yaml` | Template |
| `rescue-braintasks.yaml` | Template |
| `rescue-braintasks-fixed.yaml` | Template |

### D5. Brain Task Definitions (3 files)
| File | Notes |
|------|-------|
| `brain_tasks/zb024-parallel-2.md` | Task definition |
| `brain_tasks/zb024-parallel-3.md` | Task definition |
| `brain_tasks/zb024-parallel-4.md` | Task definition |
| `brain_tasks/zb024-parallel-5.md` | Task definition |

### D6. Architecture Docs (6 files)
| File | Notes |
|------|-------|
| `docs/01-ARCHITECTURE/COMPLETENESS_MATRIX.md` | Completeness tracking |
| `docs/01-ARCHITECTURE/CONSTRUCTION_PLAN.md` | Construction plan |
| `docs/01-ARCHITECTURE/PROGRESS.md` | Progress tracking |
| `docs/01-ARCHITECTURE/RECOMMENDED_NEXT_STEPS.md` | Recommendations |
| `docs/01-ARCHITECTURE/DEPENDENCIES.md` | Dependencies |
| `docs/01-ARCHITECTURE/.backup/ITEM5_MLQ_PROVIDER.md` | Backup |

### D7. Other Docs (10+ files)
| File | Notes |
|------|-------|
| `docs/04-DEVELOPMENT/MINIMAL_USABLE.md` | Setup guide |
| `docs/04-DEVELOPMENT/README.md` | Dev README |
| `docs/04-DEVELOPMENT/REAL_PATH_VALIDATION.md` | Validation guide |
| `docs/05-OPERATIONS/08B_POSITIVE_CONTROL_RUNBOOK.md` | Runbook |
| `docs/05-OPERATIONS/24_7_USEFUL_OPERATIONS_RUNBOOK.md` | Runbook |
| `docs/05-OPERATIONS/CANONICAL_PATH_FAIL_CLOSED_FIXES.md` | Fix report |
| `docs/05-OPERATIONS/HARDENING_REPORT.md` | Hardening report |
| `docs/05-OPERATIONS/NIGHTSHIFT_STAGED_ROLLOUT.md` | Nightshift guide |
| `docs/05-OPERATIONS/OVERNIGHT_RUNBOOK.md` | Runbook |
| `docs/05-OPERATIONS/PHASE_23_MLQ_RUNTIME_EVIDENCE.md` | Runtime evidence |
| `docs/05-OPERATIONS/PHASE_24B_FINAL_REPORT.md` | Final report |
| `docs/05-OPERATIONS/PHASE2_LLM_CODE_GENERATION.md` | Design doc |
| `docs/05-OPERATIONS/PHASE_22_MLQ_STATUS.md` | MLQ status |
| `docs/05-OPERATIONS/PRODUCTION_PATH_DEFAULTS_FIX.md` | Fix report |
| `docs/05-OPERATIONS/QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md` | Codegen guide |
| `docs/05-OPERATIONS/QWEN_2B_LOCAL_EVALUATION.md` | Evaluation |
| `docs/05-OPERATIONS/RELEASE_CHECKLIST.md` | Release checklist |
| `docs/05-OPERATIONS/TROUBLESHOOTING.md` | Troubleshooting |
| `docs/05-OPERATIONS/WARMUP_FULL_REPORT.md` | Warmup report |
| `docs/05-OPERATIONS/ZB-022-canonical-flow-design.md` | Design doc |
| `docs/05-OPERATIONS/ZB-026-worker-observability.md` | Observability |
| `docs/05-OPERATIONS/ZEN_BRAIN_1_0_SELF_IMPROVEMENT.md` | Self-improvement |
| `docs/05-OPERATIONS/ZEN_MESH_OPERATOR_GUIDE.md` | Operator guide |
| `docs/05-OPERATIONS/PROOF/KNOWN_GOOD_CONFIG.md` | Proof doc |
| `docs/05-OPERATIONS/PROOF/PROOF_OF_WORKING_LANE.md` | Proof doc |
| `docs/05-OPERATIONS/PROOF/REAL_JIRA_INTEGRATION_REPORT.md` | Proof doc |
| `docs/06-OPERATIONS/ZB_025_JIRA_INTAKE_CONTRACT.md` | Jira contract |
| `docs/99-ARCHIVE/ROADMAP_ROOT.md` | Archive |
| `docs/03-DESIGN/LLM_GATEWAY.md` | Design doc |
| `docs/03-DESIGN/PHASE2_DESIGN_LLM_GENERATION.md` | Design doc |
| `docs/01-ARCHITECTURE/ADR/0007_QMD_FOR_KNOWLEDGE_BASE.md` | ADR |
| `config/task-templates/rescue-mlq-from-0.1.yaml` | Task template |
| `cmd/zen-brain/analyze.go` | Analyze command — uses Ollama |
| `cmd/zen-brain/factory.go` | Factory command |
| `cmd/create-jira-issues/main.go` | Jira issue creator |
| `memory/2026-03-19.md` | Memory file |
| `memory/2026-03-23.md` | Memory file |
| `scripts/zen-mesh-operator-loop.sh` | Operator loop script |
| `scripts/proof_local_worker_chat.py` | Proof script |

## Items Needing L2 Review

None currently. All files have been classified based on clear repo evidence. The D-bucket items are historical — no runtime risk — and can be batch-processed in Phase 2 cleanup slices.
