# W016 EXECUTION REPORT — PHASE 16

**Generated:** 2026-03-25 08:00 EDT
**Phase:** Operationalize L1/L2 lanes on real tasks
**Constraint:** Local/k3d only, llama.cpp only, verified artifacts as benchmark

---

## Operating Model Active

**L1 Lane:** qwen3.5:0.8b-q4 via llama.cpp (cheap workhorse lane)
- Backend: llama-cpp
- API Endpoint: http://host.k3d.internal:56227
- Model File: /home/neves/git/ai/Qwen3.5-0.8B-Q4_K_M.gguf
- Port: 56227

**L2 Lane:** 2b-q4 via llama.cpp (stronger bounded lane)
- Backend: llama-cpp
- API Endpoint: http://host.k3d.internal:60509
- Model File: /home/neves/Downloads/zen-go-q4_k_m.gguf
- Port: 60509

**Foreman Image:** zen-registry:5000/zen-brain:phase15-lanes

**Verified regular-task artifacts are now the benchmark for L1/L2 capacity.**

**MLQ rescue is not the default proof vehicle for 0.8B capability.**

---

## Candidate Task List

### L1 Candidates (3 tasks)
| Task ID | Title | Target File(s) | Package | Task Class |
|----------|--------|----------------|----------|------------|
| W016-L1-01 | Add IsValid() to JiraMaterial | internal/secrets/jira.go | secrets | implementation |
| W016-L1-02 | Enhance error messages in Aggregator | internal/funding/aggregator.go | funding | implementation |
| W016-L1-03 | Add ValidateWorkTags() helper | pkg/contracts/validate.go | contracts | implementation |

### L2 Candidates (2 tasks)
| Task ID | Title | Target File(s) | Package | Task Class |
|----------|--------|----------------|----------|------------|
| W016-L2-01 | Add credential format validation to Jira | internal/secrets/jira.go | secrets | refactor |
| W016-L2-02 | Add evidence validation to funding | internal/funding/aggregator.go | funding | refactor |

---

## L1 Execution Results

| Task ID | Lane | Provider | Model | Duration | Status | Classification | Root Cause |
|----------|-------|----------|---------|--------|----------------|-------------|
| W016-L1-01 | L1 | llama-cpp | qwen3.5:0.8b-q4 | ✅ Completed | ❌ infra-fail | Source files not mounted at execution time |
| W016-L1-02 | L1 | llama-cpp | qwen3.5:0.8b-q4 | ✅ Completed | ❌ context-fail | Target file context not injected |
| W016-L1-03 | L1 | llama-cpp | qwen3.5:0.8b-q4 | ✅ Completed | ❌ model-fail | Invented types, invalid imports |

### Detailed L1 Findings

**W016-L1-01 (IsValid() method)**
- Generated: internal/core/w016_l1_01.go (NEW FILE, WRONG LOCATION)
- Target: internal/secrets/jira.go
- Quality: Good validation logic, but wrong file
- Classification: infra-fail
- Root Cause: Source files copied AFTER task started; /source-repo empty at execution

**W016-L1-02 (Error message enhancement)**
- Generated: internal/core/w016_l1_02.go (NEW FILE, WRONG LOCATION)
- Target: internal/funding/aggregator.go
- Quality: Incomplete output
- Classification: context-fail
- Root Cause: Files mounted but Factory template didn't inject target context

**W016-L1-03 (ValidateWorkTags function)**
- Generated: internal/core/w016_l1_03.go (NEW FILE, WRONG LOCATION)
- Target: pkg/contracts/validate.go
- Quality: Invalid imports, redefined WorkTags struct
- Classification: model-fail
- Root Cause: Model didn't read existing contracts.go, invented duplicate struct

### L1 Artifact Quality
- All tasks created NEW files in internal/core/ instead of editing target files
- All artifacts have wrong package declarations
- Zero out of 5 tasks successfully edited correct file
- Build/test verification path exists but artifacts don't match targets

**L1 Success Rate by Task Class:**
- implementation: 0/3 (0%) — all failed at setup/context layer

---

## L2 Execution Results

| Task ID | Lane | Provider | Model | Duration | Status | Classification | Root Cause |
|----------|-------|----------|---------|--------|----------------|-------------|
| W016-L2-01 | L2 | llama-cpp | 2b-q4 | ⏳ Running | - | In progress (6m+) |
| W016-L2-02 | L2 | llama-cpp | 2b-q4 | ✅ Completed | ❌ context-fail | Generated wrong domain code |

### Detailed L2 Findings

**W016-L2-01 (Jira credential validation)**
- Status: Running (test generation slow)
- Model: 2b-q4 correctly routed
- Expected: Add validateJiraURL, validateJiraEmail, validateJiraProjectKey
- Pending artifact inspection

**W016-L2-02 (Evidence validation)**
- Generated: internal/core/w016_l2_02.go (NEW FILE, WRONG LOCATION)
- Target: internal/funding/aggregator.go
- Quality: Generated Jira/evidence code instead of funding aggregator validation
- Classification: context-fail
- Root Cause: Model didn't read funding/aggregator.go context
- Duration: 54s (fast execution, wrong output)

**L2 Success Rate by Task Class:**
- refactor: 0/2 (0%) — both failed at context layer

---

## Telemetry Summary

### Task Execution Metrics

| Metric | Value |
|---------|--------|
| Total Tasks Executed | 5 |
| Tasks Completed | 4/5 (80%) |
| L1 Tasks | 3/3 completed |
| L2 Tasks | 1/2 completed, 1/2 running |
| Total Execution Time | ~25 minutes |
| Correct Files Modified | 0/5 (0%) |

### Provider/Model Routing

| Lane | Provider | Model | Tasks Routed | Warmup | Tools Available |
|-------|----------|---------|---------------|----------|----------------|
| L1 | llama-cpp | qwen3.5:0.8b-q4 | 3 | ✅ Yes | ❓ Unclear |
| L2 | llama-cpp | 2b-q4 | 2 | ✅ Yes | ❓ Unclear |

### Context Injection Status

| Metric | Status |
|---------|--------|
| Source files mounted to k3d | ✅ Yes (manual fix applied) |
| ZEN_SOURCE_REPO path | /source-repo ✅ |
| Target files accessible | ✅ After manual copy |
| Factory template injects context | ❌ NO - not working |
| Default fallback to new file | ✅ Occurred for all tasks |

---

## Observed Failure Causes

### Primary Failure: Context Injection Not Working

**Root Cause:** Factory LLMTemplate does not inject target file contents into prompt
- Source files exist in /source-repo (after manual fix)
- Prompt builder does not read target files
- Model falls back to default "create new file" behavior
- Task spec fields (constraints.target_files, constraints.package) ignored

**Evidence:**
- L1-01: internal/core/w016_l1_01.go (should be internal/secrets/jira.go)
- L1-02: internal/core/w016_l1_02.go (should be internal/funding/aggregator.go)
- L1-03: internal/core/w016_l1_03.go (should be pkg/contracts/validate.go)
- L2-02: internal/core/w016_l2_02.go (should be internal/funding/aggregator.go)

### Secondary Failure: Model Quality on Poor Context

**Root Cause:** When model lacks context, it invents or drifts
- L1-03: Invented WorkTags struct, invalid imports, duplicate code
- L2-02: Generated Jira/evidence code instead of funding validation

### L2-01 Slow Execution (Running)

**Observation:** L2-01 taking 6m+ vs L2-02 at 54s
- Implementation file created at 3m
- Test generation appears slow or stuck
- Possible model variance or prompt complexity issue

---

## Lane Recommendation

**L1 Lane (0.8B):**
- **Capacity:** Cannot evaluate model quality due to context injection failure
- **Verdict:** Inconclusive — infra block prevents proper assessment
- **Action:** Fix Factory LLMTemplate context injection before further L1 evaluation

**L2 Lane (2B):**
- **Capacity:** Cannot evaluate model quality due to context injection failure
- **Verdict:** Inconclusive — same infra block as L1
- **Action:** Fix context injection path before L2 evaluation

**Overall Assessment:**
- L1 and L2 lanes appear correctly configured and routed
- Both backends (llama.cpp on ports 56227/60509) healthy
- Warmup working for both lanes
- **Critical blocker:** Factory template does not read/inject source context

---

## Recommended Next Action

**Priority 1: Fix Context Injection Path**
1. Investigate Factory LLMTemplate/prompt builder for ZEN_SOURCE_REPO integration
2. Ensure target file contents are read from /source-repo before LLM call
3. Verify prompt includes existing code with package declaration visible
4. Add telemetry to confirm context was injected

**Priority 2: Re-run Phase 16 with Fixed Context**
1. After fix, re-execute all 5 tasks
2. Focus on L1 tasks first (faster iterations)
3. Compare pre/post-fix artifact quality
4. Only then assess L1 vs L2 model capability

**Priority 3: Do Not Escalate to L3 Yet**
- Current failures are setup/infra, not model limitations
- Proper context injection may make L1/L2 fully functional
- MLQ rescue is NOT the proof vehicle — use fixed regular-task artifacts

---

## Conclusion

Phase 16 execution revealed a critical infrastructure issue preventing proper L1/L2 lane evaluation. The operating model (L1=0.8B workhorse, L2=2B bounded) is correctly configured and routing works. Both llama.cpp backends are healthy. However, **verified regular-task artifacts cannot be produced because Factory does not inject target file context**.

**No meaningful capacity assessment of qwen3.5:0.8b-q4 or 2b-q4 is possible until context injection is fixed.** The model output artifacts generated were all in wrong files/locations because the Factory template defaults to "create new file" when no source context is provided.

**L1 = cheap workhorse lane and L2 = stronger bounded lane.**

**MLQ rescue is not the default proof vehicle for 0.8B capability.**

**Next action: Fix context injection path in Factory LLMTemplate, then re-run Phase 16.**

---

REPORTER: CONNECTED Summary
