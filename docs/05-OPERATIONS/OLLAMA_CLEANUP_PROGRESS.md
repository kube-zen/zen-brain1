# Ollama Cleanup Progress

**Campaign start:** 2026-03-26  
**Last updated:** 2026-03-26 22:50 EDT  
**Status:** Phase 2 complete — all planned slices executed

## Summary

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| Files with Ollama refs | 147 | 145 | -2 (removed) |
| Total matching lines | 1,644 | 1,398 | -246 |
| Files updated with fallback labels | 0 | 18 | +18 |
| Active runtime paths touched | 0 | 0 | ✅ None |

## Completed Slices

### Slice C — Dead Cache/Code Cleanup ✅
- **Commit:** `e22b10d`
- **Files removed:** 5
  - `internal/factory/llm_generator_policy.go.broken`
  - `scripts/common/__pycache__/config.cpython-312.pyc`
  - `scripts/common/__pycache__/env.cpython-312.pyc`
  - `scripts/common/__pycache__/helmfile_values.cpython-312.pyc`
  - `scripts/__pycache__/zen.cpython-312.pyc`
- **Validation:** Clean

### Slice A — Docs Clarification ✅
- **Commit:** `92df15e`
- **Files updated:** 10
  - `docs/04-DEVELOPMENT/CONFIGURATION.md` — fallback-only note added
  - `docs/04-DEVELOPMENT/SETUP.md` — primary runtime note added
  - `docs/05-OPERATIONS/RELEASE_CHECKLIST.md` — Ollama items marked fallback
  - `docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md` — FALLBACK ONLY header
  - `docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md` — FALLBACK ONLY header
  - `docs/05-OPERATIONS/TROUBLESHOOTING.md` — FALLBACK ONLY header
  - `docs/05-OPERATIONS/OVERNIGHT_RUNBOOK.md` — primary runtime note
  - `docs/05-OPERATIONS/WARMUP_FULL_REPORT.md` — fallback header
  - `docs/05-OPERATIONS/LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md` — primary note
  - `docs/05-OPERATIONS/PROOF/PROOF_OF_WORKING_LANE.md` — fallback lane label
- **Validation:** Build clean

### Slice B — Config Clarification ✅
- **Commit:** `92df15e`
- **Files updated:** 1
  - `config/policy/mlq-levels.yaml` — L0 marked FALLBACK ONLY
- **Validation:** N/A (comment only)

### Slice D — Deprecated Deployments ✅
- **Commit:** `a914601`
- **Files updated:** 3
  - `deployments/ollama-in-cluster/README.md` — DEPRECATED header
  - `deployments/ollama-in-cluster/ollama.yaml` — DEPRECATED header
  - `charts/zen-brain-ollama/README.md` — DISABLED BY DEFAULT header
- **Validation:** N/A (docs only)

### Slice E — Runtime Code Comments ✅
- **Commit:** `a914601`
- **Files updated:** 2
  - `internal/llm/local_worker.go` — comment updated (llama.cpp primary)
  - `cmd/zen-brain/factory.go` — help text updated (mention llama.cpp)
- **Validation:** `go build ./internal/llm/... ./internal/factory/... ./cmd/zen-brain/...` — clean

## References Still Remaining

### Active (KEEP) — 30 files
Core provider, gateway wiring, config, CI gates, tests. These are required for the L0 fallback lane and must stay.

### Inactive but Valid (DEFER) — 15 files
Disabled charts, helmfile wiring, policy headers, fallback docs. Legitimate fallback/compatibility path.

### Historical (LOW PRIORITY) — ~100 files
Status reports, benchmarks, execution reports, memory files. These document historical work and could be batch-updated with a simple "this describes the L0 fallback lane" note, but have no operator confusion risk.

## Not Done (Deferred)

- Historical docs (D-bucket, ~100 files) could get batch fallback notes, but they're status reports and benchmark results — no operator confusion risk from their current framing
- The 4 compiled binaries (`bin/*`, `foreman`) contain embedded Ollama references that can't be edited — they'll be regenerated on next build

## Runtime Validation

- [x] `go build ./internal/llm/...` — clean
- [x] `go build ./internal/factory/...` — clean
- [x] `go build ./cmd/zen-brain/...` — clean
- [x] `go build ./cmd/apiserver/...` — clean
- [x] No active llama.cpp runtime paths modified
- [x] No logic changes in any file
- [x] All commits pushed to `origin/main`

## Commit History

| Commit | Description |
|--------|-------------|
| `98be7f3` | Phase 1: inventory + classification + candidates reports |
| `e22b10d` | Slice C: remove dead .broken + __pycache__ |
| `af2fd58` | Slice A partial: CONFIGURATION.md fallback note |
| `92df15e` | Slices A+B: 10 docs + 1 config clarified |
| `a914601` | Slices D+E: deprecated deployments + runtime comments |
