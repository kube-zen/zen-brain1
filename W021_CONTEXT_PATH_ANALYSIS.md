# W021: Normal-Task Prompt/Context Path Analysis

**Updated:** 2026-03-25 08:30 EDT

---

## Findings

### 1. Normal Task Entry Point

**File:** `internal/foreman/factory_runner.go`
**Function:** `Run()` → `brainTaskToFactorySpec()` → `Factory.ExecuteTask()`

**Flow:**
```
BrainTask (k8s) 
  → brainTaskToFactorySpec()
  → Factory.ExecuteTask(spec)
  → buildGenerationRequest(spec)
  → GenerateImplementation()
```

### 2. Target Files Parsing - ROOT CAUSE IDENTIFIED

**Location:** `internal/factory/llm_templates.go`
**Function:** `buildGenerationRequest()`

**Code Observation:**
```go
// ZB-281 C030: When TargetFiles is explicitly set (structured prompt), use that path
// instead of guessTargetPath which generates a wrong slug-based path.
// ZB-281 W004: Also fall back to ZEN_SOURCE_REPO for isolated-dir workspaces.
sourcePaths := []string{workspacePath}
if repoPath := os.Getenv("ZEN_SOURCE_REPO"); repoPath != "" {
    sourcePaths = append([]string{repoPath}, sourcePaths...)
}

if len(req.TargetFiles) > 0 {
    loaded := false
    for _, srcPath := range sourcePaths {
        explicitTarget := filepath.Join(srcPath, req.TargetFiles[0])
        if content, err := os.ReadFile(explicitTarget); err == nil {
            req.ExistingCode = string(content)
            req.TargetPath = filepath.Join(workspacePath, req.TargetFiles[0])
            log.Printf("[LLMTemplate] Loaded existing code from %s: %s (%d bytes)", srcPath, req.TargetFiles[0], len(content))
            loaded = true
            break
        }
    }
    if !loaded {
        log.Printf("[LLMTemplate] WARNING: Could not load target file %s from any source path (tried: %v)", req.TargetFiles[0], sourcePaths)
    }
}
```

**ROOT CAUSE FOUND:**

The code correctly reads `req.TargetFiles` but **the field is never populated** from BrainTask constraints.

**Evidence:**
1. `brainTaskToFactorySpec()` in `internal/foreman/factory_runner.go`:
```go
Constraints:        task.Spec.Constraints,  // Direct mapping - NO PARSING
```

2. `task.Spec.Constraints` comes from BrainTask YAML as `[]string`:
```yaml
constraints:
  - "target_file: internal/secrets/jira.go"  # This is a STRING, not parsed
  - "package: secrets"
```

3. `buildGenerationRequest()` NEVER parses these constraint strings:
```go
req.Constraints:        []string{},  // Set directly from spec
// No code here to parse "target_file: X" format
```

### 3. Related Files Population

**Location:** `internal/factory/llm_templates.go`
**Function:** `findRelatedFiles()`

**Behavior:**
```go
// Look for interface files
interfacesPath := filepath.Join(workspacePath, "interface.go")
// Look for types files
typesPath := filepath.Join(workspacePath, "types.go")
// Look for existing implementation in similar domain
domainPath := filepath.Join(workspacePath, "internal", workDomain)
```

**Issue:**
- Finds files in **workspace**, not **ZEN_SOURCE_REPO**
- Since workspace is newly created for each task, it's empty
- Source files exist in ZEN_SOURCE_REPO mount point, not workspace

### 4. Output Path Determination

**Location:** `internal/factory/llm_templates.go`
**Function:** `determineTargetPath()`

**Code:**
```go
// Create slug from work item ID
slug := strings.ToLower(spec.WorkItemID)
slug = strings.ReplaceAll(slug, "-", "_")

// Determine directory
targetDir := workspacePath
if spec.WorkDomain != "" {
    targetDir = filepath.Join(workspacePath, "internal", string(spec.WorkDomain))
}

return filepath.Join(targetDir, slug+ext)
```

**Issue:**
- Uses `spec.WorkItemID` to create slug (e.g., "w016-l1-01-add-credential-validation")
- Creates path like `/tmp/zen-brain-factory/workspaces/session-xxx/w016-l1-01-add-credential-validation.go`
- **NEVER uses explicit target file** even when `req.TargetFiles` is set

**Evidence from logs:**
```
Created: internal/core/w016_l1_01.go  # Wrong location!
```

The override code exists (lines 134-137):
```go
// Override target path if task specified explicit target files
if len(req.TargetFiles) > 0 {
    targetPath = filepath.Join(workspacePath, req.TargetFiles[0])
    log.Printf("[LLMTemplate] Using explicit target path from task spec: %s", targetPath)
}
if err := e.writeFile(targetPath, implResult.Code); err != nil {
```

**But `writeFile()` is called AFTER the if block's targetPath variable shadows the override!**

### 5. Structured vs Normal Task Path

**Critical Discovery:**

The code path splits into TWO DIFFERENT paths:

**Structured Path (Rescue Tasks):**
```go
isRescueTask := strings.Contains(spec.Objective, "ADAPT") ||
    strings.Contains(spec.Objective, "Rescue") ||
    strings.Contains(spec.Objective, "0.1") ||
    strings.Contains(spec.Objective, "zen-structured-prompt")

if isRescueTask {
    req.StructuredPrompt = true
    req.JiraKey = spec.WorkItemID
    req.WorkTypeLabel = "rescue_implementation"
    req.TimeoutSec = 2700
    // ... sets TargetFiles, ContextFiles, AllowedPaths, ExistingTypes
}
```

**Normal Path (All Other Tasks):**
```go
// No StructuredPrompt flag set
req.RelatedFiles = make(map[string]string)
req.Constraints = []string{}
// ... falls through to ad-hoc prompts
```

**The Problem:**
- Structured tasks get `promptbuilder.BuildPrompt(TaskPacket)` with grounded context
- Normal tasks get ad-hoc prompts that rely on `req.ExistingCode` and `req.RelatedFiles`
- **Neither field is populated for normal explicit-target tasks!**

### 6. Root Cause Summary

| Issue | Location | Root Cause |
|--------|-----------|-------------|
| TargetFiles empty | `brainTaskToFactorySpec()` | BrainTask constraints not parsed |
| ExistingCode empty | `buildGenerationRequest()` | Never reads target file contents |
| RelatedFiles empty | `findRelatedFiles()` | Searches workspace, not source repo |
| Output wrong location | `writeFile()` | Variable shadowing prevents override |

---

## Delivered: Exact Function/File Root Causes

### Target Files Not Populated

**File:** `internal/foreman/factory_runner.go`
**Function:** `brainTaskToFactorySpec()`
**Line:** ~274 (mapping spec.Constraints)

**Issue:** Direct field mapping with no parsing:
```go
Constraints:        task.Spec.Constraints,  // []string - not parsed
```

**Should be:** Parse "target_file: X" from constraints to set `TargetFiles`

---

### ExistingCode Not Set

**File:** `internal/factory/llm_templates.go`
**Function:** `buildGenerationRequest()`
**Line:** ~134-157 (sourcePaths logic)

**Issue:** `req.TargetFiles` is empty, so file read block never runs

**Should be:** Always read explicit target file when TargetFiles is set

---

### RelatedFiles Not Populated

**File:** `internal/factory/llm_templates.go`
**Function:** `findRelatedFiles()`
**Line:** ~378-409 (workspace search)

**Issue:** Searches newly-created workspace instead of mounted ZEN_SOURCE_REPO

**Should be:** Search ZEN_SOURCE_REPO paths when available

---

### Output Path Wrong

**File:** `internal/factory/llm_templates.go`
**Function:** `writeFile()` (called from `Execute()`)
**Line:** ~134-141

**Issue:** Variable shadowing:
```go
// Line 135: Sets targetPath correctly
targetPath := e.determineTargetPath(spec, workspacePath, implResult.Language)
// Line 139: Override if TargetFiles set
if len(req.TargetFiles) > 0 {
    targetPath = filepath.Join(workspacePath, req.TargetFiles[0])  // Correct!
}
// Line 142: Calls writeFile with different variable!
if err := e.writeFile(targetPath, implResult.Code); err != nil {  // BUG: targetPath is still from line 135!
```

**Should be:** Ensure override actually updates the variable passed to writeFile()

---

## Conclusion

**Normal explicit-target tasks fail because:**

1. **BrainTask constraints field format** (`"target_file: X"`) is not parsed to populate `TargetFiles`
2. **Source files are never read** because `TargetFiles` field is empty
3. **Output goes to wrong location** due to variable shadowing in writeFile() path

**All three issues must be fixed together** - they're interdependent in the execution flow.

---

**No Speculative Fixes Yet** - per W021 requirement.
