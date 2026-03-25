# W019/W020: Batch Reclassification and L2-01 Status

**Updated:** 2026-03-25 08:15 EDT

---

## Current Batch Status: PROVISIONAL / SETUP-CONTAMINATED

**Conclusion:** The current L1/L2 batch is NOT yet a valid capability benchmark because explicit-target normal tasks are not receiving actual target-file context.

**Dominant Observed Failure Mode:** Setup/Path Failure
- New files created instead of edit-in-place
- Wrong package/file placement (internal/core/* instead of target paths)
- Target-file context injection missing from requests

---

## Task Summary Table

| Task ID | Lane | Provider | Model | Target File(s) | Actual File(s) | Status | Classification |
|----------|-------|----------|----------------|------------------|--------|----------------|
| W016-L1-01 | L1 | llama-cpp | qwen3.5:0.8b-q4 | internal/secrets/jira.go | internal/core/w016_l1_01.go (NEW) | ✅ Completed | ❌ infra-fail |
| W016-L1-02 | L1 | llama-cpp | qwen3.5:0.8b-q4 | internal/funding/aggregator.go | internal/core/w016_l1_02.go (NEW) | ✅ Completed | ❌ context-fail |
| W016-L1-03 | L1 | llama-cpp | qwen3.5:0.8b-q4 | pkg/contracts/validate.go | internal/core/w016_l1_03.go (NEW) | ✅ Completed | ❌ model-fail |
| W016-L2-01 | L2 | llama-cpp | 2b-q4 | internal/secrets/jira.go | internal/core/w016_l2_01.go (NEW) | ✅ Completed | ❓ Unclear |
| W016-L2-02 | L2 | llama-cpp | 2b-q4 | internal/funding/aggregator.go | internal/core/w016_l2_02.go (NEW) | ✅ Completed | ❌ context-fail |

---

## L2-01 Final Status

**Task ID:** W016-L2-01
**Title:** Add credential format validation to Jira resolution
**Status:** ✅ Completed
**Duration:** ~7m 31s
**Elapsed Time:** 17m (from submission to completion)
**Last Log Line:** [LLMTemplate] Completed refactor for W016-L2-01 in 7m31.192760303s, files=2
**Output Landed:** Yes - 2 files created
**Proof_of_Work:** Ran (artifact created)
**Classification:** ❓ Context-Fail (same pattern as other tasks)
**W020 Conclusion:** L2-01 completed cleanly; no hung task issue.

---

## Root Cause Summary (Provisional)

**Primary Failure:** W014 Violation - Tools Not Attached
- Task YAMLs specified constraints and templates but did NOT use quickwin-l1 template
- Factory fell back to default `implementation:llm` template
- Default template has:
  - NO tool definitions
  - NO target-file context injection
  - NO bounded packet constraints

**Secondary Failure:** Target-File Context Not Read
- All tasks created NEW files in internal/core/*
- Zero tasks edited explicit target files
- Model defaulted to "create new file" behavior

**Classification of All Runs:**
- W016-L1-01: ❌ infra-fail (source files not mounted at execution)
- W016-L1-02: ❌ context-fail (target context not injected)
- W016-L1-03: ❌ model-fail (invented types due to missing context)
- W016-L2-01: ❌ context-fail (wrong output path)
- W016-L2-02: ❌ context-fail (wrong domain code)

**Valid Benchmark Evidence Count:** 0/5 (0%)
