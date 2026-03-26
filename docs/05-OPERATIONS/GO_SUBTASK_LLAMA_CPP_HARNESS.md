# Go subtask harness (llama.cpp, Qwen3.5 0.8B / 2B)

**Status:** Operator reference  
**Last updated:** 2026-03-26

## Purpose

This document describes how to run the **structured quick-win / autowork-style** Go codegen checks used to calibrate **Qwen3.5** GGUFs through **llama.cpp** (`llama-server`, OpenAI-compatible **`/v1/chat/completions`**). It mirrors the **packet shape** documented in [QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md](QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md) and the **0.8B vs 2B** measurements in [QWEN_2B_LOCAL_EVALUATION.md](QWEN_2B_LOCAL_EVALUATION.md).

The full harness (prompts, Python builder, verifier, parent `run_go_subtasks_suite.sh`) is **not vendored** in this repository; operators keep it in a separate checkout (for example a machine-local `~/git/ai` tree). This repo **does** ship a thin **2B CPU wrapper** script so the entry point and environment variables are version-controlled—see [scripts/run-go-subtasks-2b-cpu.sh](../../scripts/run-go-subtasks-2b-cpu.sh).

## Prerequisites

- **llama.cpp** `llama-server` on `PATH` or set **`LLAMA_SERVER`** to the binary path.
- **GGUF** artifacts (Unsloth **Q4_K_M** recommended for parity):
  - `Qwen3.5-0.8B-Q4_K_M.gguf`
  - `Qwen3.5-2B-Q4_K_M.gguf` ([Hugging Face](https://huggingface.co/unsloth/Qwen3.5-2B-GGUF))
- **Go** toolchain (for `go build` verification).
- **Python 3** (for `build_go_quickwin_prompt.py` and `verify_go_from_chat_json.py` in the harness checkout).

## Harness checkout layout (external)

Typical files next to each other in one directory (`HARNESS_ROOT`):

| File | Role |
|------|------|
| `run_go_subtasks_suite.sh` | Parent driver: health, warmup, chat JSON, `suite.log`, `go build` verify |
| `run_go_subtasks_suite_cpu.sh` | CPU defaults for **0.8B** (and optional base vs tuned compare) |
| `run_go_subtasks_suite_2b_cpu.sh` | Same as below if you keep a copy beside the parent; or use **this repo’s** wrapper |
| `build_go_quickwin_prompt.py` | Builds Jira-style structured user prompts from `go_subtasks_suite/prompts/NN.txt` |
| `verify_go_from_chat_json.py` | Extracts Go from raw or fenced output; runs `go build` in a temp module |
| `go_subtasks_suite/prompts/*.txt` | Per-subtask specs |

Set:

```bash
export GO_SUBTASK_HARNESS_ROOT=/path/to/harness   # directory that contains run_go_subtasks_suite.sh
```

## Quick start: 0.8B (CPU)

From **`GO_SUBTASK_HARNESS_ROOT`**:

```bash
cd "$GO_SUBTASK_HARNESS_ROOT"
./run_go_subtasks_suite_cpu.sh
# Subset:
FIRST=1 LAST=3 ./run_go_subtasks_suite_cpu.sh
# Single GGUF:
ONLY_MODEL=1 MODEL_GGUF="$GO_SUBTASK_HARNESS_ROOT/Qwen3.5-0.8B-Q4_K_M.gguf" ./run_go_subtasks_suite_cpu.sh
```

## Quick start: 2B (CPU)

**2B is expected to be slower than 0.8B** on the same subtask; use the same prompts for apples-to-apples quality checks.

### Option A — wrapper in this repo (recommended)

From a clone of **zen-brain1**:

```bash
export GO_SUBTASK_HARNESS_ROOT=/path/to/harness
export PATH_TO_ZEN_BRAIN1=/path/to/zen-brain1   # optional if you use absolute path to script

"$PATH_TO_ZEN_BRAIN1/scripts/run-go-subtasks-2b-cpu.sh"
# Examples:
FIRST=1 LAST=5 "$PATH_TO_ZEN_BRAIN1/scripts/run-go-subtasks-2b-cpu.sh"
MODEL_GGUF=/custom/Qwen3.5-2B-Q4_K_M.gguf "$PATH_TO_ZEN_BRAIN1/scripts/run-go-subtasks-2b-cpu.sh"
```

The script sets **`ONLY_MODEL=1`** (no second model pass), points **`BASE_GGUF`** / **`MODEL_GGUF`** at the **2B** file by default next to the harness, and exports CPU-oriented defaults (`N_GPU_LAYERS=0`, context size, warmup rounds).

### Option B — copy `run_go_subtasks_suite_2b_cpu.sh` inside the harness tree

If you maintain `run_go_subtasks_suite_2b_cpu.sh` beside `run_go_subtasks_suite.sh` (same pattern as Option A, self-contained `exec` to the parent script), run:

```bash
cd "$GO_SUBTASK_HARNESS_ROOT"
./run_go_subtasks_suite_2b_cpu.sh
```

## Common environment variables

| Variable | Typical use |
|----------|-------------|
| `GO_SUBTASK_HARNESS_ROOT` | Required by **zen-brain1** `run-go-subtasks-2b-cpu.sh`; directory containing `run_go_subtasks_suite.sh`. |
| `LLAMA_SERVER` | Path to `llama-server` if not on `PATH`. |
| `BASE_GGUF` / `MODEL_GGUF` | GGUF path (**2B** wrapper defaults to `.../Qwen3.5-2B-Q4_K_M.gguf` under harness root). |
| `FIRST` / `LAST` | Subtask index range (prompts `NN.txt`). |
| `ONLY_MODEL` | `1` = single model directory under `OUT_DIR/single` (used for 2B-only runs). |
| `OUT_DIR` | Where to write `suite.log`, per-subtask JSON, `finish_stats.tsv`. |
| `BENCH_TEMPERATURE` | Lower (e.g. **0.1**) for codegen calibration. |
| `ATTACH_TOOLS` | Default **0** for pure codegen; `1` requires `llama-server --jinja` and often hurts 0.8B quality. |
| `IGNORE_EOS` | Default **0** so generation stops at natural EOS (avoid long CPU fills to `max_tokens`). |

## Interpreting output

- **`suite.log`:** wall time, `finish_reason`, `completion_tokens`, `go_verify=OK|FAIL`, compiler errors.
- **`subtask_N.json`:** raw OpenAI-style response for debugging.
- **2B vs 0.8B:** expect **lower tok/s** and **longer** `predicted_ms` for 2B on CPU; both should pass **`go build`** on well-shaped subtasks when prompts follow [quickwin-l1](../../config/task-templates/quickwin-l1.yaml) ideas.

## Related

- [QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md](QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md)
- [QWEN_2B_LOCAL_EVALUATION.md](QWEN_2B_LOCAL_EVALUATION.md)
- [L1_L2_LANE_RUNBOOK.md](L1_L2_LANE_RUNBOOK.md)
- [SMALL_MODEL_STRATEGY.md](../03-DESIGN/SMALL_MODEL_STRATEGY.md)
