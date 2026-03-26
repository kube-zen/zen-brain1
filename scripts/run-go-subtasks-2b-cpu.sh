#!/usr/bin/env bash
# Run the external Go subtask harness against Qwen3.5-2B-Q4_K_M (llama-server, CPU).
# Requires a checkout that contains run_go_subtasks_suite.sh (prompts, Python builder, verify).
#
# Usage:
#   export GO_SUBTASK_HARNESS_ROOT=/path/to/harness
#   ./scripts/run-go-subtasks-2b-cpu.sh
#   FIRST=1 LAST=5 ./scripts/run-go-subtasks-2b-cpu.sh
#
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${GO_SUBTASK_HARNESS_ROOT:?Set GO_SUBTASK_HARNESS_ROOT to the directory containing run_go_subtasks_suite.sh}"

PARENT="$ROOT/run_go_subtasks_suite.sh"
[[ -f "$PARENT" ]] || { echo "ERROR: missing $PARENT (bad GO_SUBTASK_HARNESS_ROOT=$ROOT)" >&2; exit 1; }

export N_GPU_LAYERS="${N_GPU_LAYERS:-0}"
export THREADS="${THREADS:-$(nproc 2>/dev/null || echo 8)}"
export LLAMA_CTX_SIZE="${LLAMA_CTX_SIZE:-32768}"
export MAX_TOKENS_PER_SUBTASK="${MAX_TOKENS_PER_SUBTASK:-8192}"
export BASE_GGUF="${BASE_GGUF:-$ROOT/Qwen3.5-2B-Q4_K_M.gguf}"
export LLAMA_SERVER="${LLAMA_SERVER:-$HOME/git/llama.cpp/build/bin/llama-server}"
export COOLDOWN_SEC="${COOLDOWN_SEC:-5}"
export WARMUP_ROUNDS="${WARMUP_ROUNDS:-4}"
export ONLY_MODEL="${ONLY_MODEL:-1}"
export MODEL_GGUF="${MODEL_GGUF:-$BASE_GGUF}"

exec "$PARENT"
