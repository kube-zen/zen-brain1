# Qwen3.5 2B local evaluation (vs 0.8B, llama.cpp)

## Purpose

Record a **controlled comparison** between **Qwen3.5 0.8B** and **Qwen3.5 2B** using **llama.cpp** with the **same quantization** (**Q4_K_M**, Unsloth GGUFs) to inform **escalation ladder** sizing—see [LOCAL_LLM_ESCALATION_LADDER.md](../03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md).

This is **not** the certified **Ollama** path (ZB-023); it is an **engineering measurement** of **relative speed and RAM** between model sizes on one host.

## Artifacts

| Model | GGUF file | Hugging Face | Approx. on-disk size |
|-------|-----------|--------------|----------------------|
| 0.8B Q4_K_M | `Qwen3.5-0.8B-Q4_K_M.gguf` | [unsloth/Qwen3.5-0.8B-GGUF](https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF) | ~508 MiB (532,517,120 B) |
| 2B Q4_K_M | `Qwen3.5-2B-Q4_K_M.gguf` | [unsloth/Qwen3.5-2B-GGUF](https://huggingface.co/unsloth/Qwen3.5-2B-GGUF) | ~1.2 GiB (1,280,835,840 B) |

Example local paths (dev machine): `/home/neves/git/ai/Qwen3.5-0.8B-Q4_K_M.gguf`, `/home/neves/git/ai/Qwen3.5-2B-Q4_K_M.gguf`.

## Environment (measurement run)

- **Generated:** 2026-03-22 (America/New_York)
- **Host:** 20 logical threads; **llama-server** `-t 16`, `--parallel 1`, `--reasoning off`
- **Quantization:** Q4_K_M (both)
- **Task:** Same short Go coding prompt (`ParseSemver`-style), **`max_tokens` 256**
- **Warmup:** 2 requests (discarded); **timed runs:** 5
- **GPU:** none (CPU-only)
- **llama.cpp:** build `version: 8467 (990e4d969)` (representative)

## Results (median over timed runs)

| Model | Gen tok/s (median) | Gen time (ms, median) | Prompt eval (ms, median) | RSS peak (KiB, `ps`) |
|-------|---------------------|------------------------|----------------------------|----------------------|
| **0.8B Q4_K_M** | **13.87** | 18,455 | 69 | 4,268,812 (~4.1 GiB) |
| **2B Q4_K_M** | 13.48 | 18,997 | 101 | 5,338,260 (~5.1 GiB) |

## Interpretation

- **Throughput:** Median **generation tokens/s** is **similar** (~14 tok/s band on this host for this prompt/output budget); **0.8B is slightly faster**.
- **Latency:** **2B** is slightly slower per run (**~19.0 s** vs **~18.5 s** median generation time) with somewhat higher **prompt** time (~**102 ms** vs ~**69 ms**).
- **Memory:** **2B** uses roughly **~1.0 GiB more** resident set at peak than **0.8B** for this configuration—supports capping **1–2** concurrent 2B instances when the rest of the budget serves 0.8B workhorses and **external** escalations.

**Caveats:**

- Absolute tok/s differ from longer runs (e.g. `max_tokens` 384) and from **Ollama-in-Docker** paths; compare **0.8B vs 2B** here, not to unrelated benchmarks.
- **Quality** was not scored in this script; escalation value of 2B assumes **higher task success** on harder subtasks (validate with task-class calibration).

## Go codegen harness parity (2026-03-26)

The same **llama.cpp** + OpenAI **`/v1/chat/completions`** pattern as [QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md](QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md) (thinking off, `chat_template_kwargs.enable_thinking=false`, structured quick-win / autowork-style user prompt, `ATTACH_TOOLS` omitted for pure codegen, `go build` verification) was run against **Qwen3.5-2B-Q4_K_M** on **CPU**.

**Task:** bounded Go subtask 1 (`httpx` / `GetJSON`-style), **temperature 0.1**, **single scored request** after warmup (representative dev host).

| Model | Approx. wall clock (subtask 1) | `predicted_ms` (llama.cpp) | Gen tok/s (reported) | Completion tokens | `go build` |
|-------|--------------------------------|-----------------------------|------------------------|-------------------|------------|
| **0.8B Q4_K_M** | ~16–25 s | ~8.3–10.3 s | ~20–25 | ~215 | **OK** |
| **2B Q4_K_M** | **~32 s** | **~18.0 s** | **~14.3** | **256** | **OK** |

**Interpretation:** On this harness, **2B is slower** than **0.8B** (roughly **~2×** longer wall and predicted generation time for this subtask), consistent with expectations for a larger model on CPU. Both passed **`go build`** on the same prompt shape; **2B** often wrapped the answer in a **markdown ` ```go` ** fence while **0.8B** tended toward **raw** source—the verifier accepts both.

**How to run:** see [GO_SUBTASK_LLAMA_CPP_HARNESS.md](GO_SUBTASK_LLAMA_CPP_HARNESS.md) for **`GO_SUBTASK_HARNESS_ROOT`**, the **`scripts/run-go-subtasks-2b-cpu.sh`** wrapper in this repository, and an optional self-contained `run_go_subtasks_suite_2b_cpu.sh` next to `run_go_subtasks_suite.sh` in your harness checkout. Place Unsloth **`Qwen3.5-2B-Q4_K_M.gguf`** beside the harness (or set **`MODEL_GGUF`**).

## Reproduction

Script (example): `/tmp/bench_llama_08b_vs_2b.sh` (or equivalent in repo if checked in).

```bash
# After placing both GGUFs in LLAMA_GGUF_* paths:
BENCH_THREADS=16 BENCH_WARMUP=2 BENCH_RUNS=5 BENCH_MAX_TOKENS=256 \
  /tmp/bench_llama_08b_vs_2b.sh
```

Report output (example): `/tmp/llama_qwen_08b_vs_2b_q4km.md`.

## See also

- [GO_SUBTASK_LLAMA_CPP_HARNESS.md](GO_SUBTASK_LLAMA_CPP_HARNESS.md) — how to run the Go subtask suite (0.8B / 2B) with `llama-server`.
- [QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md](QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md) — inference flags and prompt shape (applies to **2B** as well).
- [LOCAL_LLM_ESCALATION_LADDER.md](../03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md) — when to use 2B vs 0.8B vs external.
- [LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md](./LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md) — 0.8B llama.cpp vs Ollama (2×2 quant matrix).
