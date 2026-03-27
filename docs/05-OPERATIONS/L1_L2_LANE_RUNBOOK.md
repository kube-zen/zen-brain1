> **NOTE:** This document references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only.

# L1/L2 Lane Operations Runbook

**Version:** 3.0
**Status:** Production
**Updated:** 2026-03-26 (PHASE 26)

## Overview

This runbook covers the operational procedure for running tasks on the L1 (0.8B workhorse) and L2 (2B bounded) lanes via llama.cpp. Factory routes through TaskExecutor with automatic retry/escalation.

### Certified Runtime

| Lane | Endpoint | Model | Slots | Context/Slot | Role |
|-------|----------|-------|-------|-------------|------|
| L1 | http://localhost:56227 | Qwen3.5-0.8B-Q4_K_M.gguf | 10 parallel | 6656 tokens | Default — all regular useful tasks |
| L2 | http://localhost:60509 | zen-go-q4_k_m-latest.gguf | 4 slots | 16384 tokens | Earned by repeated L1 failure |
| L0 | http://localhost:11434 | qwen3.5:0.8b (Ollama) | 1 | — | Fallback only (FAIL-CLOSED) |

### Execution flow
```
Task → Factory.executeTask()
     → executeWithLLM()
     → executeWithLLMRetry() [if TaskExecutor available]
       → TaskExecutor.ExecuteWithRetry()
         → SelectLevel() → L1 (default)
         → Create generator for L1 worker endpoint
         → Execute template
         → On failure: retry (up to max_retries per escalation_rules)
         → After repeated failure: escalate to L2
         → On provider outage: fallback to L0
         → Record telemetry on every attempt
```

### Escalation policy (from mlq-levels.yaml)
- L1 → L2: after 2 consecutive failures (retry_count trigger)
- L1 → L0: on timeout/error (timeout_or_error trigger)
- L2 is earned by L1 failure evidence, not guessed in advance

### Worker endpoints
| Level | Endpoint | Model | Slots |
|-------|----------|-------|-------|
| L1 | http://localhost:56227 | Qwen3.5-0.8B-Q4_K_M.gguf | 10 parallel |
| L2 | http://localhost:60509 | zen-go-q4_k_m.gguf | 1 |
| L0 | http://localhost:11434 | qwen3.5:0.8b (Ollama) | 1 |

### Proven evidence (2026-03-25)
- **10-task batch**: 10/10 L1 success, 71s wall time (parallel)
- **Escalation test**: L1 failed ×2 → escalated to L2 → L2 succeeded (3 attempts, 2.3s total)
- **No-think fix**: stub_hunting went from 0 bytes to 1826 bytes after enabling thinking=false
- **Concurrency proof**: 10 requests in 3.4s = true parallel (same as single request time)

## Quick Reference

| Step | Action | Command/Check |
|------|--------|---------------|
| 1 | Preflight | `scripts/run-08b-positive-control.sh --warmup-only` |
| 2 | Route task | See [Routing Matrix](MLQ_LANE_ROUTING_MATRIX.md) |
| 3 | Shape packet | Use `quickwin-l1.yaml` template for L1 |
| 4 | Verify context | Confirm target file is in `ZEN_SOURCE_REPO` mount |
| 5 | Submit task | `kubectl apply -f <task.yaml> -n zen-brain` |
| 6 | Monitor | `kubectl logs -n zen-brain deploy/foreman -f` |
| 7 | Verify | Check output, build, test |

## 1. Preflight Checklist

Before any L1/L2 task, verify ALL of these:

```
[ ] Foreman image is from zen-registry:5000 (not :500)
[ ] MLQ config mounted at /tmp/zen-brain1/config/policy/mlq-levels.yaml
[ ] ZEN_SOURCE_REPO is set and source files are mounted
[ ] llama.cpp is running and reachable from cluster
[ ] Warmup completed on the target model lane
[ ] Tools enabled for the lane (supports_tools: true)
```

### Verify Foreman Image

```bash
kubectl --context k3d-zen-brain-sandbox describe deploy foreman -n zen-brain | grep Image:
# MUST be: zen-registry:5000/zen-brain:<tag>
# MUST NOT be: zen-registry:500/*
```

### Verify MLQ Config

```bash
POD=$(kubectl --context k3d-zen-brain-sandbox get pods -n zen-brain --no-headers -o custom-columns=':metadata.name' | grep foreman | head -1)
kubectl --context k3d-zen-brain-sandbox exec -n zen-brain $POD -- cat /tmp/zen-brain1/config/policy/mlq-levels.yaml | head -5
```

### Verify Source Context

```bash
kubectl --context k3d-zen-brain-sandbox exec -n zen-brain $POD -- ls /source-repo/internal/scheduler/
kubectl --context k3d-zen-brain-sandbox exec -n zen-brain $POD -- ls /source-repo/pkg/llm/
```

### Warmup llama.cpp

```bash
# Use the script for full warmup:
./scripts/run-08b-positive-control.sh --warmup-only

# Or manual warmup:
curl -s http://localhost:56227/health  # Must return {"status":"ok"}
curl -s http://localhost:56227/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen3.5:0.8b-q4","messages":[{"role":"user","content":"say ok"}],"max_tokens":5}'
```

### Verify Cluster Reachability

```bash
kubectl --context k3d-zen-brain-sandbox exec -n zen-brain deploy/foreman -- \
  wget -qO- --timeout=5 http://host.k3d.internal:56227/health
# Must return: {"status":"ok"}
```

## 2. Task Routing

### Primary Task Classes for 24/7 Operations

These are the production task classes that define zen-brain1 usefulness:

| Task Class | Description | Template |
|-----------|-------------|----------|
| Dead code | Unreferenced exported functions scan | usefulness-l1.yaml |
| Defects | Common defect patterns: nil, unchecked errors, race conditions | usefulness-l1.yaml |
| Tech debt | TODO/FIXME/HACK, deprecated APIs, large functions | usefulness-l1.yaml |
| Roadmap | Milestone extraction from docs | usefulness-l1.yaml |
| Bug hunting | Suspicious patterns: race conditions, memory leaks, logic errors | usefulness-l1.yaml |
| Stub hunting | Empty function bodies, panic(not implemented), hardcoded returns | usefulness-l1.yaml |
| Package hotspots | Export frequency, dependency graph metrics | usefulness-l1.yaml |
| Test gaps | Untested packages, missing edge case coverage | usefulness-l1.yaml |
| Config drift | Policy vs actual config comparison | usefulness-l1.yaml |
| Executive summary | Rollup of other report findings | usefulness-l1.yaml |

**Production success criterion:** zen-brain1 is "working" when it continuously produces useful artifacts for these task classes through the real runtime. Standalone Go codegen is NOT the benchmark.

### Route tasks using the [MLQ Lane Routing Matrix](MLQ_LANE_ROUTING_MATRIX.md).

### L1 (0.8B workhorse) — Default for bounded tasks
- Single file, edit-in-place
- No new architecture
- Target file explicit and known
- `workType: implementation` → L1 by default (MLQ config)

### L2 (2B bounded) — Medium tasks
- 1–3 files
- Moderate adaptation
- Still grounded and explicit

### Escalation triggers
- Architecture invention needed
- >3 files affected
- Target file unclear
- L1/L2 failure under correct conditions

## 3. Packet Shaping

### L1 Quick-Win Packet (use `config/task-templates/quickwin-l1.yaml`)

Keep the packet short:
- 1-line goal
- Target file + package name
- Existing code (auto-injected from ZEN_SOURCE_REPO)
- One adjacent context file if needed
- Allowed imports list
- "Do not invent" note
- Verification commands
- Output contract

### L2 Medium Bounded Packet

Similar structure but:
- May list 2–3 target files
- More context budget (up to 5000 chars)
- May include brief architectural note

### FORBIDDEN Packet Shapes for L1
- ❌ 4-phase rescue/adaptation packets
- ❌ Giant FORBIDDEN/CRITICAL block walls
- ❌ Multi-file architecture invention requests
- ❌ Tasks without an explicit target file
- ❌ Tasks without existing code context

## 4. Context and Tool Verification

### Target File Context (W004)

As of Phase 15, target file contents are auto-injected from `ZEN_SOURCE_REPO` for any task with explicit `TargetFiles`. Verify in logs:

```
[LLMTemplate] Loaded existing code from /source-repo: internal/scheduler/types.go (1234 bytes)
```

### Tools (W006)

Verify the lane has tools enabled:
```
# In config/profiles/local-cpu-45m.yaml:
supports_tools: true

# In config/policy/roles.yaml:
capabilities:
  - file-operations
  - shell-execution
```

Check in logs that tools were available for the task.

## 5. Task Submission

```bash
# Apply task
kubectl --context k3d-zen-brain-sandbox apply -f <task.yaml> -n zen-brain

# Monitor logs
kubectl --context k3d-zen-brain-sandbox logs -n zen-brain deploy/foreman -f --since=1m
```

### Required Log Evidence for Every Task

Must see these lines:
1. `[MLQ] Selected: ... level=<N> backend=llama-cpp model=<model>`
2. `[LLMTemplate] Loaded existing code from ...`
3. `provider=llama-cpp` (NOT `provider=ollama`)
4. `target-path resolution`
5. `proof_of_work`
6. `Postflight checks passed`

## 6. Post-Task Verification

```bash
# Check task status
kubectl --context k3d-zen-brain-sandbox get braintask <task-name> -n zen-brain -o yaml

# Check generated files
POD=$(kubectl --context k3d-zen-brain-sandbox get pods -n zen-brain --no-headers -o custom-columns=':metadata.name' | grep foreman | head -1)
kubectl --context k3d-zen-brain-sandbox exec -n zen-brain $POD -- find /tmp/zen-brain-factory/workspaces -path "*<task-name>*" -name "*.go" -exec wc -l {} \;

# Check for common failure modes
kubectl --context k3d-zen-brain-sandbox exec -n zen-brain $POD -- cat <generated-file> | grep -E "package |import |type "
# Should match the expected package name and real imports
```

## 7. Failure Classification

| Failure Symptom | Likely Cause | Action |
|----------------|-------------|--------|
| `connection refused` | llama.cpp down | Start llama.cpp, verify port |
| `provider=ollama` | MLQ config not loaded | Check ConfigMap mount |
| Wrong package name | Source file not injected | Check ZEN_SOURCE_REPO mount |
| Invented types | No context files in prompt | Check source file injection logs |
| Shell commands in .go | Prompt too complex for 0.8B | Simplify to quick-win packet |
| Empty/greenfield code | `ExistingCode` not loaded | Check target file path |
| Missing imports | Model didn't see context | Inject adjacent dependency file |

## 8. Telemetry

Track each task with:
- Timestamp, provider, model, lane level
- Packet type, tools yes/no, warmup yes/no
- Context present yes/no, target-file present yes/no
- Task class, files changed, build/test pass/fail
- Result classification (success / context-fail / infra-fail / model-fail)

See `docs/05-OPERATIONS/08B_POSITIVE_CONTROL_RUNBOOK.md` for the test script.

---

## Related Documents

- [MLQ Lane Routing Matrix](MLQ_LANE_ROUTING_MATRIX.md)
- [08B Positive Control Runbook](08B_POSITIVE_CONTROL_RUNBOOK.md)
- [Small Model Strategy](../03-DESIGN/SMALL_MODEL_STRATEGY.md)
- [Warmup Full Report](WARMUP_FULL_REPORT.md)
- [llama.cpp vs Ollama Benchmark](LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md)
- [Qwen 0.8B llama.cpp codegen guide](QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md)
- [Go subtask llama.cpp harness](GO_SUBTASK_LLAMA_CPP_HARNESS.md) — 0.8B / 2B CPU runs with external checkout

## Related Architecture

- [ZEN_LOCK_ZEN_FLOW_INTEGRATION_DECISION.md](./ZEN_LOCK_ZEN_FLOW_INTEGRATION_DECISION.md) — platform integration model
- [SECRET_CONTRACT.md](./SECRET_CONTRACT.md) — secret management contract
- [24_7_USEFUL_OPERATIONS_RUNBOOK.md](./24_7_USEFUL_OPERATIONS_RUNBOOK.md) — 24/7 operations runbook

