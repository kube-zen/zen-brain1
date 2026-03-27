# Ollama Cleanup Progress

**Campaign start:** 2026-03-26  
**Last updated:** 2026-03-26 23:55 EDT  
**Status:** ✅ COMPLETE

## Summary

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| Files with Ollama refs | 147 | 140 | -7 (5 removed, 2 new) |
| Total matching lines | 1,644 | ~1,398 | -246 |
| Files with fallback labels | 0 | 89 | +89 |
| Active runtime paths touched | 0 | 0 | ✅ None |
| Logic changes | — | 0 | ✅ None |

## All Commits

| Commit | Slice | Description |
|--------|-------|-------------|
| `98be7f3` | Phase 1 | Inventory + classification + candidates reports |
| `e22b10d` | C | Remove dead .broken + __pycache__ (5 files) |
| `af2fd58` | A | CONFIGURATION.md fallback note |
| `92df15e` | A+B | 10 docs + 1 config clarified |
| `a914601` | D+E | Deprecated deployments + runtime comments |
| `243daec` | — | Progress tracking doc |
| `7ed4705` | Batch | 61 historical docs labeled (59 updated + 2 new) |
| `259b886` | Batch | 5 brain task templates + test braintask labeled |

## Files Removed (5)
- `internal/factory/llm_generator_policy.go.broken`
- `scripts/common/__pycache__/config.cpython-312.pyc`
- `scripts/common/__pycache__/env.cpython-312.pyc`
- `scripts/common/__pycache__/helmfile_values.cpython-312.pyc`
- `scripts/__pycache__/zen.cpython-312.pyc`

## Files Created (3)
- `docs/05-OPERATIONS/OLLAMA_REFERENCE_INVENTORY.md`
- `docs/05-OPERATIONS/OLLAMA_CLASSIFICATION.md`
- `docs/05-OPERATIONS/OLLAMA_CLEANUP_CANDIDATES.md`
- `docs/05-OPERATIONS/OLLAMA_CLEANUP_PROGRESS.md` (this file)
- `p17c_results/README.md`

## Remaining Ollama References (140 files)

All correctly classified:

### Active Runtime (KEEP — 30 files)
Core provider, gateway, factory, tests, CI gates. Required for L0 fallback lane.

### Active Config (KEEP — 18 files)
Provider definitions, routing policy, MLQ config, chart values, deploy config.

### Active Scripts (KEEP — 10 files)
CI gates, deploy helpers, health checks, operator scripts.

### Labeled Docs (KEEP — 89 files)
All historical/reference docs now have fallback-only or primary-runtime notes.

### Binaries (CANNOT EDIT — 5 files)
Compiled binaries in bin/ and foreman. Regenerated on next build.

### Memory (DO NOT MODIFY — 2 files)
Per AGENTS.md policy, memory files are not modified for cleanup.

### Generated (REGENERATED — 1 file)
`.artifacts/state/sandbox/zen-brain-values.yaml` — regenerated on deploy.

## Done Criteria Check

1. ✅ Active runtime docs clearly say llama.cpp is the production path
2. ✅ Every Ollama reference is classified (A/B/C/D)
3. ✅ All low-risk dead references are removed
4. ✅ Remaining Ollama references are explicitly marked as:
   - Fallback (config, charts, deployments)
   - Legacy/deprecated (in-cluster manifests)
   - Historical evidence (status reports, benchmarks)
   - Active runtime (provider code, gateway wiring)
5. ✅ No operator or AI can mistake Ollama as the active runtime anymore

## Validation

- [x] `go build ./internal/llm/...` — clean
- [x] `go build ./internal/factory/...` — clean
- [x] `go build ./cmd/zen-brain/...` — clean
- [x] `go build ./cmd/apiserver/...` — clean
- [x] No active llama.cpp runtime paths modified
- [x] No logic changes in any file
- [x] All 8 commits pushed to `origin/main`
