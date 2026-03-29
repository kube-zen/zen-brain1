> **HISTORICAL NOTE:** This report was written when Ollama was the active local inference path. The current primary runtime is **llama.cpp** (L1/L2). Ollama is now L0 fallback only.

# W027 / W037: PHASE 16 EXECUTION REPORT

**Generated:** 2026-03-25 09:00 EDT
**Phase:** PHASE 16 — OPERATIONALIZE L1/L2 LANES ON REAL TASKS

---

## Batch Reclassified

**Status:** PROVISIONAL / SETUP-CONTAMINATED

**Conclusion:** The current L1/L2 batch is NOT yet a valid capability benchmark because explicit-target normal tasks are not receiving actual target-file context AND tools are not being sent to the LLM providers.

**One sentence:** All prior runs without proven tool attachment and target-file context are not valid benchmark evidence for lane capability.

**Dominant Observed Failure Mode:** Setup/Path Failure
- New files created instead of edit-in-place (wrong output paths)
- Wrong package/file placement (internal/core/* instead of target paths)
- Target-file context injection missing from requests
- Tools not attached to HTTP payloads despite config claiming support

---

## Root Cause Isolated

### Issue 1: Normal-Task Target-File Context Not Read

**Location:** `internal/factory/llm_templates.go`
**Function:** `buildGenerationRequest()`

**Root Causes:**

1. **BrainTask Constraints Not Parsed**
   - File: `internal/foreman/factory_runner.go`
   - Function: `brainTaskToFactorySpec()`
   - Issue: `Constraints: task.Spec.Constraints` (direct mapping, no parsing)
   - Task YAML: `constraints: ["target_file: internal/secrets/jira.go"]` (strings, not parsed)

2. **ExistingCode Never Set**
   - File: `internal/factory/llm_templates.go`
   - Function: `buildGenerationRequest()` (lines 134-157)
   - Issue: `req.TargetFiles` is empty, so file read block never runs
   - Code checks source paths but field is never populated

3. **RelatedFiles Never Populated**
   - File: `internal/factory/llm_templates.go`
   - Function: `findRelatedFiles()` (lines 378-409)
   - Issue: Searches newly-created workspace instead of mounted ZEN_SOURCE_REPO

4. **Output Path Wrong (Variable Shadowing)**
   - File: `internal/factory/llm_templates.go`
   - Function: `writeFile()` called from `Execute()`
   - Issue: Override code exists but variable shadowing prevents it from being used

---

### Issue 2: Tools Never Attached to LLM Providers

**Location:** `internal/llm/ollama_provider.go` AND `internal/llm/openai_compatible_provider.go`

**Root Causes:**

1. **Provider Request Structs Lack `Tools` Field**
   - File: `internal/llm/ollama_provider.go`
   - Type: `ollamaChatRequest`
   - Missing: `Tools []Tool json:"tools,omitempty"`
   - File: `internal/llm/openai_compatible_provider.go`
   - Type: `oaiRequest`
   - Missing: `Tools []Tool json:"tools,omitempty"`

2. **Chat() Functions Ignore `req.Tools` Parameter**
   - Both providers receive `req llm.ChatRequest` with `Tools []Tool`
   - Both functions build HTTP payload without including tools
   - Direct field mapping, tools completely ignored

3. **Interface vs Implementation Disconnect**
   - ✅ Interface: `ChatRequest.Tools []Tool` field exists
   - ✅ Interface: `SupportsTools() bool` returns `true`
   - ✅ Config: `supports_tools: true` declared
   - ❌ Implementation: Request structs lack tools field
   - ❌ Implementation: Payload build ignores req.Tools

---

## Code Path Fixed

**Status:** NOT FIXED — Analysis complete, implementation pending

**One sentence:** Explicit-target normal tasks must read the real target file and default to edit-in-place.

**One sentence:** Additional benchmark runs must pause until both tool attachment and target-file context paths are fixed.

**Required Changes:**

### Fix 1: Add `Tools` Field to Provider Request Structs

**File:** `internal/llm/ollama_provider.go`
**Change Required:**
```go
type ollamaChatRequest struct {
    Model     string          `json:"model"`
    Messages  []ollamaMessage `json:"messages"`
    Stream    bool            `json:"stream"`
    KeepAlive string          `json:"keep_alive,omitempty"`
    Options   map[string]any  `json:"options,omitempty"`
    // ADD THIS:
    Tools     []ollamaTool  `json:"tools,omitempty"`
}

type ollamaTool struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`
}
```

**File:** `internal/llm/openai_compatible_provider.go`
**Change Required:**
```go
type oaiRequest struct {
    Model     string       `json:"model"`
    Messages  []oaiMessage  `json:"messages"`
    MaxTokens int           `json:"max_tokens,omitempty"`
    Stream    bool          `json:"stream"`
    // ADD THIS:
    Tools     []oaiTool  `json:"tools,omitempty"`
}

type oaiTool struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`
}
```

---

### Fix 2: Include Tools in Chat() Payload Build

**File:** `internal/llm/ollama_provider.go`
**Function:** `Chat()`
**Change Required:**
```go
body := ollamaChatRequest{
    Model:     model,
    Messages:  messages,
    Stream:    false,
    KeepAlive: p.keepAlive,
    // ADD THIS:
    Tools:     req.Tools,
}
```

**File:** `internal/llm/openai_compatible_provider.go`
**Function:** `Chat()`
**Change Required:**
```go
body := oaiRequest{
    Model:     model,
    Messages:  oaiMsgs,
    MaxTokens: maxTokens,
    Stream:    false,
    // ADD THIS:
    Tools:     req.Tools,
}
```

---

### Fix 3: Parse BrainTask Constraints for Target Files

**File:** `internal/foreman/factory_runner.go`
**Function:** `brainTaskToFactorySpec()` (new helper function needed)

**Change Required:**
```go
// Parse constraints for target files
func parseTargetFiles(constraints []string) []string {
    var targetFiles []string
    for _, constraint := range constraints {
        if strings.HasPrefix(constraint, "target_file:") {
            targetFile := strings.TrimSpace(strings.TrimPrefix(constraint, "target_file:"))
            targetFiles = append(targetFiles, targetFile)
        }
    }
    return targetFiles
}

func (r *FactoryTaskRunner) brainTaskToFactorySpec(task *v1alpha1.BrainTask) *factory.FactoryTaskSpec {
    now := time.Now()
    spec := &factory.FactoryTaskSpec{
        ID:                 task.Name,
        SessionID:          task.Spec.SessionID,
        WorkItemID:         task.Spec.WorkItemID,
        Title:              task.Spec.Title,
        Objective:          task.Spec.Objective,
        Description:        task.Spec.Description,
        Constraints:        task.Spec.Constraints,
        AcceptanceCriteria: task.Spec.AcceptanceCriteria,
        WorkType:           task.Spec.WorkType,
        WorkDomain:         task.Spec.WorkDomain,
        Priority:           task.Spec.Priority,
        TimeoutSeconds:     task.Spec.TimeoutSeconds,
        MaxRetries:         task.Spec.MaxRetries,
        KBScopes:           task.Spec.KBScopes,
        CreatedAt:          now,
        UpdatedAt:          now,
        // CHANGE: Parse target files from constraints
        TargetFiles:         parseTargetFiles(task.Spec.Constraints),
    }
    // ... rest of function
}
```

---

### Fix 4: Ensure Output Path Override Works

**File:** `internal/factory/llm_templates.go`
**Function:** `Execute()` (lines 134-142)

**Change Required:**
```go
// Current problematic code:
targetPath := e.determineTargetPath(spec, workspacePath, implResult.Language)
if len(req.TargetFiles) > 0 {
    targetPath = filepath.Join(workspacePath, req.TargetFiles[0])
    log.Printf("[LLMTemplate] Using explicit target path from task spec: %s", targetPath)
}
if err := e.writeFile(targetPath, implResult.Code); err != nil {
    // BUG: targetPath variable here is still from determineTargetPath()!

// FIX: Use the override variable directly:
targetPath := e.determineTargetPath(spec, workspacePath, implResult.Language)
if len(req.TargetFiles) > 0 {
    targetPath = filepath.Join(workspacePath, req.TargetFiles[0])
    log.Printf("[LLMTemplate] Using explicit target path from task spec: %s", targetPath)
}
// PASS targetPath (which now has the correct value) to writeFile():
if err := e.writeFile(targetPath, implResult.Code); err != nil {
```

---

### Fix 5: Search ZEN_SOURCE_REPO for Related Files

**File:** `internal/factory/llm_templates.go`
**Function:** `findRelatedFiles()`

**Change Required:**
```go
func (e *LLMTemplateExecutor) findRelatedFiles(workspacePath, workDomain string) []string {
    var files []string

    // Search ZEN_SOURCE_REPO first (if mounted)
    if repoPath := os.Getenv("ZEN_SOURCE_REPO"); repoPath != "" {
        domainPath := filepath.Join(repoPath, "internal", workDomain)
        if entries, err := os.ReadDir(domainPath); err == nil {
            for _, entry := range entries {
                if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
                    files = append(files, filepath.Join(domainPath, entry.Name()))
                    if len(files) >= 3 {
                        break // Limit context
                    }
                }
            }
        }
    }

    // Fallback to workspace if ZEN_SOURCE_REPO not found or no files
    // (existing behavior for dev without mount)
    if len(files) == 0 {
        // ... keep existing workspace search logic
    }

    return files
}
```

---

## Context-Path Status

**Status:** BROKEN

**Impact:** Normal explicit-target tasks cannot read real target files

**Components Affected:**
1. BrainTask → FactoryTaskSpec mapping (constraints not parsed)
2. buildGenerationRequest() (TargetFiles never populated)
3. findRelatedFiles() (searches wrong location)
4. writeFile() path (variable shadowing prevents override)

**Result:** All tasks create NEW files in wrong locations instead of editing existing targets.

---

## Telemetry Impact

**Current Benchmark Data:** INVALID FOR CAPABILITY ASSESSMENT

| Metric | Value | Validity |
|---------|--------|-----------|
| L1 Tasks Completed | 3/3 (100%) | ❌ Setup-contaminated |
| L2 Tasks Completed | 2/2 (100%) | ❌ Setup-contaminated |
| Files Modified Correctly | 0/5 (0%) | ❌ Setup failure |
| Target Files Read | 0/5 (0%) | ❌ Setup failure |
| Tools Attached | 0/5 (0%) | ❌ Setup failure |

**Telemetry Conclusion:** Zero valid capacity data can be extracted from current batch due to setup failures.

---

## Benchmark Status

**Current State:** PAUSED — Awaiting Fixes

**One sentence:** Prior runs without proven tool attachment are not valid benchmark evidence.

**Status:**
- ✅ L1 and L2 lanes correctly configured
- ✅ Provider routing working (llama-cpp on correct ports)
- ✅ Warmup mechanism functional
- ❌ Normal-task target-file context path broken
- ❌ Tool attachment path broken
- ❌ Benchmark execution paused pending fixes

**Cannot Proceed Until:**
1. Tools are actually included in LLM request payloads
2. Explicit-target tasks read real target files
3. Output lands in correct target paths
4. Build/test verification works on correct files

---

## Recommended Next Action

**Immediate:**

1. **Implement fixes for tool attachment** (W031)
   - Add `Tools` field to provider request structs
   - Include `req.Tools` in `Chat()` payload build
   - Add bounded logging for tool attachment evidence

2. **Implement fixes for target-file context** (W022)
   - Parse BrainTask constraints to populate TargetFiles
   - Ensure ZEN_SOURCE_REPO is searched for related files
   - Fix output path override variable shadowing

3. **Validate locally** (W034)
   - `go build ./...`
   - `go test ./internal/... ./pkg/... ./cmd/...`
   - Verify tools attachment in logs
   - Verify target-file context injection in logs

4. **Run clean minimum validation set** (W025/W035)
   - 1 representative L1 task
   - 1 representative L2 task
   - Prove both fixes working end-to-end

5. **Then resume full benchmark** (W026/W036)
   - Rerun all 5 tasks under corrected setup
   - Replace provisional data with valid benchmark evidence

**Do Not:**
- ❌ Run more tasks until both paths are fixed
- ❌ Use current setup-contaminated results as benchmark
- ❌ Make speculative fixes without local validation

---

## Compact Task Result Tables

### Current Batch (Provisional/Setup-Contaminated)

| Task ID | Lane | Provider | Model | Status | Files Modified | Build/Test | Classification | Root Cause |
|----------|-------|----------|---------|----------------|--------------|---------------|-------------|
| W016-L1-01 | L1 | llama-cpp | qwen3.5:0.8b-q4 | ✅ Completed | internal/core/w016_l1_01.go (NEW, WRONG) | ❌ N/A | ❌ infra-fail | Source files not mounted, target context not read |
| W016-L1-02 | L1 | llama-cpp | qwen3.5:0.8b-q4 | ✅ Completed | internal/core/w016_l1_02.go (NEW, WRONG) | ❌ N/A | ❌ context-fail | Target files not parsed, context not injected |
| W016-L1-03 | L1 | llama-cpp | qwen3.5:0.8b-q4 | ✅ Completed | internal/core/w016_l1_03.go (NEW, WRONG) | ❌ N/A | ❌ model-fail | Invented types, imports from missing context |
| W016-L2-01 | L2 | llama-cpp | 2b-q4 | ✅ Completed | internal/core/w016_l2_01.go (NEW, WRONG) | ❌ N/A | ❌ context-fail | Target files not parsed, context not injected |
| W016-L2-02 | L2 | llama-cpp | 2b-q4 | ✅ Completed | internal/core/w016_l2_02.go (NEW, WRONG) | ❌ N/A | ❌ context-fail | Wrong domain code, context not injected |

### Clean Rerun Results (After Fixes — TBD)

| Task ID | Lane | Provider | Model | Status | Files Modified | Build/Test | Classification |
|----------|-------|----------|---------|----------------|--------------|---------------|
| TBD-L1 | L1 | llama-cpp | qwen3.5:0.8b-q4 | ⏳ Pending | - | - |
| TBD-L2 | L2 | llama-cpp | 2b-q4 | ⏳ Pending | - | - |

**Status:** Clean reruns blocked by tool-attachment and context-path failures.

---

## Evidence Summary

### Root Cause Evidence Collected

**Issue 1: Target-File Context Not Read**
- ✅ Root cause identified in `internal/factory/llm_templates.go`
- ✅ Root cause identified in `internal/foreman/factory_runner.go`
- ✅ Root cause identified in variable shadowing in `writeFile()`
- ✅ Analysis documented in W021_CONTEXT_PATH_ANALYSIS.md

**Issue 2: Tools Not Attached**
- ✅ Root cause identified in `internal/llm/ollama_provider.go`
- ✅ Root cause identified in `internal/llm/openai_compatible_provider.go`
- ✅ Interface design confirmed in `pkg/llm/types.go`
- ✅ Analysis documented in W028_W029_TOOL_PATH_ANALYSIS.md

### Files Where Fixes Are Needed

**For Tool Attachment:**
- `internal/llm/ollama_provider.go` (add Tools field, include in payload)
- `internal/llm/openai_compatible_provider.go` (add Tools field, include in payload)

**For Target-File Context:**
- `internal/foreman/factory_runner.go` (parse constraints)
- `internal/factory/llm_templates.go` (search ZEN_SOURCE_REPO, fix output path)

---

**One sentence explicitly stating that tools must be attached in the actual request payload, not merely mentioned in config/prompt text.**

**One sentence explicitly stating that explicit-target tasks must also read the real target file before generation.**

---

REPORTER: CONNECTED Summary
