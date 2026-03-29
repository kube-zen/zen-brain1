# Prompt Engineering Migration from zen-brain 0.1

## What Changed

**Root Cause Identified:** qwen3.5:0.8b was generating junk because generic role prompts don't constrain small models sufficiently.

**Solution:** Migrated zen-brain 0.1's structured prompt patterns to zen-brain1.

## 0.1 Templates Used as Source

The following zen-brain 0.1 templates were analyzed and patterns extracted:

1. **task-templates/quickwin.yaml** - Detailed ticket creation with phased sections
2. **task-templates/planner.yaml** - Separated planning from execution, explicit forbidden actions
3. **task-templates/execute_ticket.yaml** - Trunk-on-main workflow with fail-closed behavior
4. **task-templates/worker_execute.yaml** - Verification commands and output contracts

## Key Patterns Extracted

### 1. Phased Sections
Every task must have:
- Requirements (WHAT needs to be done)
- Expected Behavior (HOW it should work)
- Verification (HOW to test it)

### 2. Explicit Forbidden Actions
- "Do NOT include code examples" (for planning prompts)
- "Do NOT invent new packages/imports"
- "Do NOT modify files outside allowed paths"
- "If required type is missing, report blocker instead of hallucinating"

### 3. Bounded Scope
- Exact allowed paths
- Files to read first
- Existing types/interfaces to use
- Do not modify list

### 4. Verification Commands
- Compile: `go build ./...`
- Tests: `go test ./...`
- Static checks: `grep -r 'type X interface'`

### 5. Output Contract
- Exact files changed
- Verification run output
- Result: SUCCESS | FAILURE
- Blockers reported honestly

## New Components in zen-brain1

### 1. Prompt Builder
**Location:** `internal/promptbuilder/packet.go`

**Purpose:** Constructs structured task packets from:
- YAML task template
- Jira issue fields
- Allowed paths
- Repo facts
- Context files
- Verification commands

**Key Function:** `BuildPrompt(packet TaskPacket) (string, error)`

### 2. Rescue Task Template
**Location:** `config/task-templates/rescue_from_01.yaml`

**Purpose:** Template for porting code from zen-brain 0.1 to 1.0

**Structure:**
- Task Identity (Jira key, summary, source/target)
- Scope (allowed paths, context files)
- Architecture Constraints (existing types, packages)
- Phased Execution (4 phases with Requirements/Behavior/Verification)
- Verification Commands (compile, test, static)
- Output Contract (structured format)

### 3. Live Path Integration

**Current Path (BEFORE):**
```
planner_operational.yaml → generic "You are a Planner Agent"
  ↓
worker_operational.yaml → generic "You are a Worker Agent"
  ↓
qwen3.5:0.8b → hallucinates junk code
```

**New Path (AFTER):**
```
config/task-templates/rescue_from_01.yaml → specific template
  ↓
internal/promptbuilder/packet.go → builds structured packet
  ↓
Inject: Jira fields + allowed paths + context files + repo facts
  ↓
qwen3.5:0.8b → receives bounded specification packet
  ↓
Useful output
```

## Sample Prompt Packet

**File:** `/tmp/sample_prompt_packet.txt`

**Structure:**
```
You are executing Jira issue ZB-281.

=== TASK IDENTITY ===
Goal: Rescue MLQ from zen-brain 0.1
Source (0.1): /home/neves/zen-old/zen-brain/internal/queue/multi_level_queue.go
Target (1.0): /home/neves/zen/zen-brain1/internal/mlq/selector.go
Timeout: 2700 seconds

=== SCOPE ===
Allowed paths: internal/mlq/*, pkg/llm/types.go, internal/foreman/factory_runner.go
Read first: source file, target file, existing interfaces, existing types

=== ARCHITECTURE CONSTRAINTS ===
Use existing types: pkg/llm.Provider, ChatRequest, ChatResponse
Use existing packages: internal/llm, pkg/llm, internal/mlq

=== PHASED EXECUTION ===
Phase 0: Pre-flight Check
Phase 1: Analyze 0.1 Source
Phase 2: Analyze 1.0 Target
Phase 3: Port with Adaptation
Phase 4: Report Results

Each phase has:
- Requirements
- Expected Behavior
- Verification

=== VERIFICATION COMMANDS ===
Compile: go build ./...
Tests: go test ./...

=== OUTPUT CONTRACT ===
Format: structured
CRITICAL: Do NOT create fake artifacts
CRITICAL: Report exact files changed
CRITICAL: Report blockers honestly

FORBIDDEN:
- Do NOT invent packages/imports
- Do NOT create fake modules
- Do NOT modify outside allowed paths
- Do NOT claim success if verification fails
```

## How to Use

### 1. For Rescue Tasks
```go
packet := promptbuilder.RescueTaskTemplate(
    "ZB-281",
    "Rescue MLQ from zen-brain 0.1",
    "/home/neves/zen-old/zen-brain/internal/queue/multi_level_queue.go",
    "/home/neves/zen/zen-brain1/internal/mlq/selector.go",
    []string{"internal/mlq/*", "pkg/llm/types.go"},
)

prompt, err := promptbuilder.BuildPrompt(packet)
// Send prompt to qwen3.5:0.8b
```

### 2. For General Tasks
```go
packet := promptbuilder.TaskPacket{
    JiraKey: "ZB-123",
    Summary: "Fix authentication bug",
    WorkType: "bug_fix",
    AllowedPaths: []string{"internal/auth/*"},
    ContextFiles: []string{"internal/auth/provider.go"},
    Phases: []promptbuilder.Phase{
        {
            Name: "Identify bug",
            Requirements: []string{"Read auth flow"},
            ExpectedBehavior: []string{"Find root cause"},
            Verification: []string{"Confirm bug location"},
        },
    },
    CompileCmd: "go build ./...",
    NoFakeArtifacts: true,
    ReportFiles: true,
}

prompt, err := promptbuilder.BuildPrompt(packet)
```

## Rescue Task to Retry First

**Task:** ZB-281 (MLQ rescue from 0.1)

**Why First:**
1. Most critical architecture piece
2. Already partially implemented (internal/mlq/selector.go exists)
3. Clear source/target mapping
4. Bounded scope (3 files max)

**New Prompt Will Include:**
- Source: `/home/neves/zen-old/zen-brain/internal/queue/multi_level_queue.go`
- Target: `internal/mlq/selector.go`
- Existing types: `pkg/llm.Provider`, `ChatRequest`, `ChatResponse`
- Phased execution: Pre-flight → Analyze 0.1 → Analyze 1.0 → Port → Report
- Verification: `go build ./...` must pass
- Forbidden: No fake imports, no invented packages

## Expected Outcome

**BEFORE (with generic prompts):**
```
Generated: zb_281.go with fake imports:
- "github.com/alexmiller/zb281/examples/module"
- "github.com/stretchr/testify/mock"
- "github.com/tidwall/god"

Result: Junk code, not useful
```

**AFTER (with structured packet):**
```
Generated: bounded changes to internal/mlq/selector.go
- Reuses existing pkg/llm types
- No fake imports
- Compiles successfully
- Reports exact files changed

Result: Useful, integrate-able code
```

## Next Steps

1. Wire promptbuilder into factory_runner.go
2. Load templates from config/task-templates/
3. Inject context files before execution
4. Retry ZB-281 with new prompt structure
5. Verify output quality
6. Roll out to all rescue tasks

## References

- 0.1 Source: `~/zen-old/zen-brain/task-templates/`
- 1.0 Builder: `~/zen/zen-brain1/internal/promptbuilder/packet.go`
- 1.0 Template: `~/zen/zen-brain1/config/task-templates/rescue_from_01.yaml`
- Sample Packet: `/tmp/sample_prompt_packet.txt`
