# Zen-Brain Vertical Slice

This guide demonstrates the complete end-to-end pipeline for zen-brain 1.0.

## Overview

The vertical slice demonstrates how zen-brain processes work items from Jira (or mock) through analysis, planning, execution, and proof-of-work generation.

## Prerequisites

### Development
- Go 1.25+ installed
- `go mod tidy` run (no errors)
- `make build` or `go build -o zen-brain cmd/zen-brain/main.go` completes

### Runtime (Optional)
For real Jira integration:
- `JIRA_USERNAME` - Jira username
- `JIRA_API_TOKEN` - Jira API token
- Working Jira instance

For ZenContext with real Redis/S3:
- `AWS_ACCESS_KEY_ID` - AWS access key
- `AWS_SECRET_ACCESS_KEY` - AWS secret key
- `AWS_SESSION_TOKEN` - AWS session token (optional)
- Redis server running

## Quick Start

### Mock Mode (No Jira Required)

```bash
# Build the binary
go build -o zen-brain cmd/zen-brain/main.go

# Run vertical slice with mock work item
./zen-brain vertical-slice --mock
```

### Real Jira Mode

```bash
# Set Jira credentials (optional - can prompt if missing)
export JIRA_USERNAME="your-jira-username"
export JIRA_API_TOKEN="your-jira-api-token"

# Run vertical slice with real Jira ticket
./zen-brain vertical-slice ZB-123
```

## Pipeline Stages

The vertical slice executes a 7-stage pipeline:

### 1. Fetch Work Item
- **Mock Mode:** Creates a mock work item (ZB-001: "Fix authentication bug")
- **Jira Mode:** Fetches real ticket from Jira instance

### 2. Analyze Work Item
- **Component:** LLM Gateway (dual-lane routing)
- **Model:** `qwen3.5:0.8b` (local worker) or `glm-4.7` (planner)
- **Output:** Complexity, estimated effort, recommended approach, risks, dependencies
- **Fallback:** Automatic fallback to planner if local worker fails

### 3. Create Execution Plan
- **Input:** Work item + LLM analysis
- **Output:** Structured execution steps (3 steps by default)
- **Steps:**
  1. Initialize workspace
  2. Execute objective
  3. Validate results

### 4. Execute in Factory
- **Component:** Foreman uses **FactoryTaskRunner** by default (Block 4). BrainTasks are executed through Factory: workspace allocation, bounded execution, proof-of-work.
- **Workspace:** Isolated workspace under `ZEN_FOREMAN_WORKSPACE_HOME` (default `/tmp/zen-brain-factory/workspaces/<session>/<task>`).
- **Execution:** Real shell commands; template selection prefers **real** templates (implementation, docs, debug, refactor, review) when work domain is empty (`ZEN_FOREMAN_PREFER_REAL_TEMPLATES=true`).
- **Safety:** Workspace locking, bounded execution, timeout enforcement.
- **Retry:** Automatic retry with max_retries limit.
- **Outcome:** Run outcome (workspace path, proof path, template key, files changed, duration, recommendation) is written to BrainTask annotations (`zen.kube-zen.com/factory-*`).

### 5. Generate Proof-of-Work
- **Component:** ProofOfWorkManager
- **Output:** Structured artifacts (JSON + Markdown)
- **Content:**
  - Summary: Task metadata, objective, **template key** (e.g. `implementation:real`), complexity
  - Workspace: Path, state, **real git branch/commit** when workspace is inside a git repo (no synthetic `ai/<workItemID>`)
  - Execution: Duration, steps, exit codes; **files changed** sorted for deterministic proof
  - AI Attribution: `[zen-brain agent: <role>]`
- **Optimization:** Uses Factory's proof-of-work when available (eliminates duplicates)

### 6. Update Session State
- **Component:** Session Manager + ZenContext
- **Lifecycle:**
  1. `created` → `analyzed` (after LLM analysis)
  2. `analyzed` → `scheduled` (after execution planning)
  3. `scheduled` → `in_progress` (before Factory execution)
  4. `in_progress` → `completed` (after successful execution + proof-of-work)
  5. `in_progress` → `failed` (if Factory execution fails)
- **ZenContext:** Session stored in three-tier memory (mock implementation)

### 7. Update Jira
- **Mock Mode:** Skips Jira updates
- **Jira Mode:**
  1. Updates Jira status to "Completed"
  2. Adds proof-of-work comment with AI attribution
  3. Comment format: `### Proof-of-Work\n\n<summary>\n\n---\n\n**AI Attribution:** [zen-brain agent: worker]`

## Configuration

### Config File Search Order
1. `config.yaml` (current directory)
2. `config.dev.yaml` (current directory)
3. `~/.zen-brain/config.yaml` (home directory)
4. `../configs/config.dev.yaml` (configs directory)

### Configuration Structure

```yaml
# Zen-Brain 1.0 development configuration
home_dir: "~/.zen-brain"

# Logging
logging:
  level: "debug"
  format: "json"
  output: "stdout"

# Knowledge Base settings
kb:
  docs_repo: "../zen-docs"
  search_limit: 10

# QMD settings
qmd:
  binary_path: "qmd"
  refresh_interval: 3600

# Jira connector (optional)
jira:
  enabled: false
  base_url: "https://your-domain.atlassian.net"
  project: "ZEN"
  # Authentication via environment variables:
  # JIRA_USERNAME, JIRA_API_TOKEN

# Confluence publishing (optional)
confluence:
  enabled: false
  base_url: "https://your-domain.atlassian.net/wiki"
  space: "ZB"
  # Authentication via environment variables:
  # CONFLUENCE_USERNAME, CONFLUENCE_API_TOKEN

# Multi-cluster
clusters:
  - id: "cluster-1"
    type: "k3d"
    kubeconfig: "~/.kube/config"
  - id: "cluster-2"
    type: "k3d"
    kubeconfig: "~/.kube/config2"

# SR&ED evidence collection
sred:
  enabled: true
  default_tags:
    - "experimental_general"
  evidence_requirement: "summary"

# ZenLedger (CockroachDB) settings
ledger:
  enabled: false
  host: "localhost"
  port: 26257
  database: "zenledger"
  user: "zen"
  ssl_mode: "disable"

# ZenContext settings (three-tier memory)
zen_context:
  # Tier 1: Hot storage (Redis) - REQUIRED for ZenContext when enabled
  tier1_redis:
    addr: "localhost:6379"  # Example: Must be set explicitly, no default
    password: ""
    db: 0
    pool_size: 10
    min_idle_conns: 5
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s

  # Tier 2: Warm storage (QMD knowledge base)
  tier2_qmd:
    repo_path: "../zen-docs"
    qmd_binary_path: "qmd"
    verbose: false

  # Tier 3: Cold storage (S3 archival)
  tier3_s3:
    bucket: "zen-brain-context"
    region: "us-east-1"
    endpoint: ""  # REQUIRED for S3/MinIO; empty will FAIL CLOSED (set to "http://localhost:9000" for MinIO)
    access_key_id: ""
    secret_access_key: ""
    session_token: ""
    use_path_style: false
    disable_ssl: false
    force_rename_bucket: false
    max_retries: 3
    timeout: 30s
    part_size: 5242880  # 5 MB
    concurrency: 5
    verbose: false

  # Journal integration (optional)
  journal:
    journal_path: "./journal.db"
    enable_query_index: true

  # General settings
  cluster_id: "default"
  verbose: false

# Planner settings
planner:
  default_model: "glm-4.7"
  max_cost_per_task: 10.0
  require_approval: false
```

### Environment Variables

Sensitive values are loaded from environment variables:

| Variable | Description | Required For |
|----------|-------------|---------------|
| `JIRA_USERNAME` | Jira username | Jira mode |
| `JIRA_API_TOKEN` | Jira API token | Jira mode |
| `CONFLUENCE_USERNAME` | Confluence username | Confluence publishing |
| `CONFLUENCE_API_TOKEN` | Confluence API token | Confluence publishing |
| `AWS_ACCESS_KEY_ID` | AWS access key | S3 Tier 3 (if not in config) |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | S3 Tier 3 (if not in config) |
| `AWS_SESSION_TOKEN` | AWS session token | S3 Tier 3 (optional) |

## Integration Tests

Run integration tests to verify all components work together:

```bash
# Run all integration tests
go test ./cmd/zen-brain/... -v

# Specific test
go test ./cmd/zen-brain/... -v -run TestVerticalSlice_EndToEnd
```

### Test Coverage

The vertical slice has 6 comprehensive integration tests:

1. **TestVerticalSlice_EndToEnd** - Validates complete pipeline flow
2. **TestVerticalSlice_ConfigurationLoading** - Tests config file parsing
3. **TestVerticalSlice_SessionManagerIntegration** - Validates ZenContext contract
4. **TestVerticalSlice_FactoryCommandExecution** - Validates real command execution
5. **TestVerticalSlice_ProofOfWorkNoDuplicates** - Validates no duplicate PoW generation
6. **TestVerticalSlice_CompletePipeline** - Validates all components work together

All tests pass with `go test ./cmd/zen-brain/... -v`

## Output Example

```
=== Zen-Brain Vertical Slice ===

This command demonstrates end-to-end pipeline:
  1. Fetch work item from Jira (or use mock)
  2. Analyze intent and complexity
  3. Plan execution steps
  4. Execute in isolated workspace
  5. Generate proof-of-work
  6. Update session state
  7. Update Jira with status and comments

Mode: Using mock work item (no Jira required)

Loading configuration...
  ✓ Configuration loaded (logging: info, planner: glm-4.7)

Initializing components...
[1/7] Initializing LLM Gateway...
✓ LLM Gateway initialized
  - Local worker: qwen3.5:0.8b
  - Planner: glm-4.7
  - Fallback chain: true

[2/7] Initializing Office Manager...
[3/7] Initializing Session Manager...
  - Initializing ZenContext (tiered memory)...
  ✓ ZenContext initialized (mock implementation)
  ✓ Session Manager initialized (memory store)

[4/7] Fetching work item...
✓ Work item: MOCK-001 - Fix authentication bug in login flow
  Type: Debug, Priority: High

[5/7] Analyzing work item...
✓ Analysis complete
  Complexity: Medium
  Estimated effort: 2-4 hours
  Recommended approach: Debug authentication flow with test users
2026/03/09 14:00:00 Session session-12345-6789-0 transitioned: created -> analyzed

[6/7] Creating execution plan...
✓ Execution plan created
  Steps: 3
  Estimated cost: $0.05
2026/03/09 14:00:01 Session session-12345-6789-0 transitioned: analyzed -> scheduled

[7/7] Executing in isolated workspace with Factory...
2026/03/09 14:00:01 Session session-12345-6789-0 transitioned: scheduled -> in_progress
[BoundedExecutor] Executing step: step_id=MOCK-001-step-1 name=Initialize workspace
[BoundedExecutor] Step completed: step_id=MOCK-001-step-1 status=completed exit_code=0
[BoundedExecutor] Executing step: step_id=MOCK-001-step-2 name=Execute objective
[BoundedExecutor] Step completed: step_id=MOCK-001-step-2 status=completed exit_code=0
[BoundedExecutor] Executing step: step_id=MOCK-001-step-3 name=Validate results
[BoundedExecutor] Step completed: step_id=MOCK-001-step-3 status=completed exit_code=0
[BoundedExecutor] Execution plan completed: total_steps=3 completed_steps=3 status=completed
[ProofOfWorkManager] Created proof-of-work: task_id=MOCK-001 artifact=/tmp/zen-brain-factory-1234567890/proof-of-work/20260309-140001
[Factory] Task execution completed: task_id=MOCK-001 status=completed duration=300ms proof=/tmp/zen-brain-factory-1234567890/proof-of-work/20260309-140001

✓ Execution complete
  Duration: 300.833155ms
  Files changed: 0
  Tests passed: 0/0

[8/7] Generating proof-of-work...
  ✓ Using Factory's proof-of-work
✓ Proof-of-work generated
  JSON: /tmp/zen-brain-factory-1234567890/proof-of-work/20260309-140001/proof-of-work.json
  Markdown: /tmp/zen-brain-factory-1234567890/proof-of-work/20260309-140001/proof-of-work.md
2026/03/09 14:00:01 Added evidence evidence-1 to session session-12345-6789-0 (type: proof_of_work)
2026/03/09 14:00:01 Session session-12345-6789-0 transitioned: in_progress -> completed

=== Vertical Slice Complete ===

Summary:
  Work item: MOCK-001
  Session: session-12345-6789-0
  Proof-of-work: generated
  Jira updated: false
```

## Architecture

### Components

| Component | File | Purpose |
|-----------|------|---------|
| LLM Gateway | internal/llm/gateway.go | Dual-lane routing with fallback |
| Office Manager | internal/office/manager.go | Work item intake (Jira/Confluence) |
| Session Manager | internal/session/manager.go | Session lifecycle + ZenContext |
| Factory | internal/factory/factory.go | Bounded execution + proof-of-work |
| ZenContext | internal/context/composite.go | Three-tier memory (Redis/QMD/S3) |
| Config | internal/config/load.go | YAML config loading + env vars |

### Data Flow

```
Jira Work Item → Office Manager → LLM Gateway → Analysis
Analysis → Planner → Execution Plan → Factory → BoundedExecutor
Execution → ProofOfWorkManager → Proof-of-Work → Session Manager + ZenContext
Session Manager → Evidence → Jira (status update + comment)
```

## Troubleshooting

### Build Errors

**Error:** `go build: cannot find package`
**Solution:** Run `go mod tidy` to update dependencies

### Runtime Errors

**Error:** `Failed to load config: no such file`
**Solution:** Create a config file (see Configuration section) or use defaults

**Error:** `Jira connector initialization failed`
**Solution:** Set `JIRA_USERNAME` and `JIRA_API_TOKEN` environment variables, or use `--mock` mode

**Error:** `Factory execution failed`
**Solution:** Check workspace permissions, Factory logs, or increase timeout

### Test Failures

**Error:** Integration tests fail
**Solution:** Check CI gates with `python3 scripts/ci/run.py --suite default`

## Next Steps

After the vertical slice is working:

1. **Deploy to production** - Use real Jira instance
2. **Enable real Redis/S3** - Configure production three-tier memory
3. **Add real commands** - Replace mock commands with actual `go test`, `git commit`, etc.
4. **Monitor sessions** - Track session lifecycle in production
5. **Proof-of-work analysis** - Collect SR&ED evidence from real tasks

## See Also

- [../01-ARCHITECTURE/ROADMAP.md](../01-ARCHITECTURE/ROADMAP.md) - Project roadmap
- [../01-ARCHITECTURE/CONSTRUCTION_PLAN.md](../01-ARCHITECTURE/CONSTRUCTION_PLAN.md) - Build plan
- [../03-DESIGN/ZEN_CONTEXT.md](../03-DESIGN/ZEN_CONTEXT.md) - Three-tier memory design
- [../01-ARCHITECTURE/COMPONENT_JOURNAL.md](../01-ARCHITECTURE/COMPONENT_JOURNAL.md) - Journal schema and component design
