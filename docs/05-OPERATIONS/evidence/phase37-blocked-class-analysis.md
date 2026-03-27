# PHASE 37 — Blocked-Class Analysis

**Date:** 2026-03-27 14:16 EDT
**Run ID:** 20260327-121405 (full 3-schedule run via zen-ctl.sh)

## Executive Summary

3 task classes fail intermittently. Root cause for all 3 is **evidence bundle quality**, not validation logic. The evidence commands either produce no content, produce content that 0.8B can't parse, or produce repetitive content that triggers the repetition validator.

## Evidence Bundle Comparison

### 1. `dead_code` — context-fail (quad-hourly) / pass (daily)

**Failure pattern:** The model receives the entire prompt template repeated 15x with `(no evidence gathered)`.

**Root cause:** The evidence commands (`grep -rn '^func [A-Z]' pkg/` and `grep -rn '^func [A-Z]' internal/...`) DO produce output when run manually (25+ real exported functions found). But the `buildEvidenceBundle()` function is dropping it.

**Specifically:** The commands produce output containing test functions (`func Test*`) which are NOT dead code. The grep matches `^func [A-Z]` which catches `Test` functions too. The model receives a mix of real exported functions and test functions, and can't differentiate.

**Worse:** When the evidence bundle IS empty, the prompt + empty evidence gets sent, and 0.8B loops by repeating the prompt back.

**Minimum fix:** 
- Fix the grep pattern to exclude test files: `grep -rn '^func [A-Z]' --include='*.go' --exclude='*_test.go'`
- Add a candidate-extraction approach like defects/bug_hunting (pre-extract unique function names + check references)
- If evidence is empty, short-circuit the L1 call and produce a "no dead code candidates found" report directly

### 2. `roadmap` — context-fail (daily)

**Failure pattern:** Model repeats `"- [x] Project status page updated to reflect current state"` 14x.

**Root cause:** The evidence bundle contains `CURRENT_STATE.md` which is dominated by Jira identity tables and auth check results — not project status. The model latches onto one bullet from the `PROGRESS.md` file and repeats it.

**Evidence quality issue:** 
- `ls docs/` produces directory names (not useful for roadmap)
- `cat CURRENT_STATE.md | head -40` produces Jira identity tables, not project progress
- `cat docs/01-ARCHITECTURE/PROGRESS.md | head -40` DOES have useful progress data but it's mixed with markdown tables

**Minimum fix:**
- Replace `ls docs/` with `cat CURRENT_STATE.md | grep -E '^#{1,3} |^##'` to get section headings
- Replace `cat CURRENT_STATE.md | head -40` with just the "Current Proven State" sections
- Better: use pre-extraction like defects — extract structured `{status, item, detail}` candidates

### 3. `package_hotspots` — context-fail (daily) / success-needs-review (quad)

**Failure pattern:** Model produces table with `pkg/` repeated 8x, all with `N/A` complexity. File grounding fails (0/17 refs exist).

**Root cause:** The evidence commands produce directory names only (from `dirname`) and test-file counts (the exported-func count command returns test files first). The model doesn't get actual package paths or real function data.

**Evidence quality issue:**
- `find ... -exec dirname` produces `internal/factory`, `internal/runtime` — just package names
- The exported-function-count command matches test files (`adapter_test.go` has 20 exports)
- No actual source file paths are in the evidence, so file grounding fails

**Minimum fix:**
- Fix the exported-func command to exclude test files: `--exclude='*_test.go'`
- Include actual source file paths alongside counts
- Pre-format as a table so 0.8B doesn't need to parse raw output

## Stable Classes (for comparison)

| Class | Evidence Strategy | Validation | Notes |
|-------|------------------|------------|-------|
| defects | Pre-extracted candidates | success / success-needs-review | Candidate extraction works |
| bug_hunting | Pre-extracted candidates | success | Same pattern as defects |
| tech_debt | grep TODO/FIXME + wc -l | success | Simple grep, reliable |
| executive_summary | cat CURRENT_STATE.md | success | Reads file directly, model summarizes |
| config_drift | grep config patterns | success | Deterministic grep |
| test_gaps | find test files + comm | success | Simple find, reliable |
| stub_hunting | Pre-extracted candidates | success | Candidate extraction works |

## Pattern

The stable classes have one of:
1. **Pre-extracted candidates** (defects, bug_hunting, stub_hunting) — code does the heavy lifting, model ranks
2. **Simple reliable grep** (tech_debt, config_drift, test_gaps) — output is trivially parseable
3. **Direct file content** (executive_summary) — model gets raw markdown and summarizes

The blocked classes have:
1. **Grep that picks up test files** (dead_code, package_hotspots) — pollutes evidence
2. **Directory listings instead of content** (package_hotspots) — model invents paths
3. **Mixed-quality evidence** (roadmap) — Jira identity tables aren't project status

## Proposed Minimum Fixes

| Class | Fix | Change Type |
|-------|-----|-------------|
| dead_code | Exclude `*_test.go` from grep; add empty-evidence short-circuit | evidence + guard |
| roadmap | Use pre-extracted status items from PROGRESS.md | evidence restructure |
| package_hotspots | Exclude test files; include source paths | evidence fix |
