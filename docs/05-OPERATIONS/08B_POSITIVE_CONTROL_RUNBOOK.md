# 0.8B Positive Control Runbook

**Version:** 1.0
**Status:** Production

## Purpose

This runbook validates that qwen3.5:0.8b-q4 via llama.cpp can produce correct, grounded code for bounded tasks when given the right operating conditions (context injection, warmup, bounded prompt).

It is NOT a capability benchmark. It is an operational health check.

## When to Run

- After any deployment that touches LLM generation code
- After any config change to MLQ levels or factory
- After any llama.cpp server restart
- As part of pre-flight before a batch of L1 tasks

## Quick Start

```bash
# Full run with warmup + default bounded task:
./scripts/run-08b-positive-control.sh

# Warmup only (verify llama.cpp is healthy):
./scripts/run-08b-positive-control.sh --warmup-only

# Run a custom task file:
./scripts/run-08b-positive-control.sh --run /path/to/task.yaml
```

## What the Script Does

1. **Preflight**: Verifies cluster, image, config mount, source repo
2. **Warmup**: Starts llama.cpp if down, runs warmup inference
3. **Source Context**: Copies source files to k3d node mount
4. **Submit**: Creates and submits a bounded BrainTask
5. **Poll**: Waits for completion (10m timeout)
6. **Collect**: Gathers logs, evidence, proof-of-work

## Bounded Task Shape

The default test task is: "Add a Validate() method to the Schedule struct."

This task was chosen because:
- Single file (`internal/scheduler/types.go`)
- Single method, no architecture invention
- Clear input/output contract
- Existing types only
- Trivially testable

## Expected Outcome

With correct operating conditions:
- ✅ Model generates correct Validate() logic
- ✅ Uses existing types (no invention)
- ⚠️ May have wrong package name if source file not injected
- ⚠️ May create file in wrong path if context missing

The Phase 14 run confirmed: correct logic, wrong placement due to missing source context. With the Phase 15 W004 fix (default target-file context injection), placement should also be correct.

## Interpreting Results

| Observation | Meaning | Action |
|-------------|---------|--------|
| Correct logic, correct file | Working as designed | No action |
| Correct logic, wrong file | Source context not injected | Check ZEN_SOURCE_REPO |
| Invented types | No context in prompt | Check context injection logs |
| Shell commands in output | Prompt too complex | Simplify packet |
| Connection refused | llama.cpp down | Start server |
| provider=ollama | MLQ config missing | Check ConfigMap mount |
| Empty output | Model timeout or truncation | Check token limits |

## Do NOT Use This Test For

- ❌ MLQ rescue/adaptation task validation
- ❌ Multi-file refactor validation
- ❌ Architecture invention validation
- ❌ Broad capability claims about 0.8B

## Related

- [L1/L2 Lane Runbook](L1_L2_LANE_RUNBOOK.md)
- [MLQ Lane Routing Matrix](MLQ_LANE_ROUTING_MATRIX.md)
- [Small Model Strategy](../03-DESIGN/SMALL_MODEL_STRATEGY.md)
- [Qwen 0.8B llama.cpp codegen guide](QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md) — inference flags, prompt shape, base vs LoRA calibration notes
