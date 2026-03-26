# Qwen 3.5 0.8B: Why local Go LoRA underperformed base (investigation)

**Status:** Engineering analysis  
**Last updated:** 2026-03-26  
**Scope:** Operator-maintained JSONL + LoRA training under `~/git/ai` (not vendored in this repo); findings apply to any similar SFT pipeline.

## Executive summary

The **merged / quantized LoRA GGUF** often looked **worse than base Q4** on strict **`go build`** benchmarks **not** because inference flags were wrong, but because **supervised fine-tuning taught a different task** than production / harness evaluation:

| Dimension | What base Q4 does with a good prompt | What the LoRA was optimized to imitate |
|-----------|----------------------------------------|----------------------------------------|
| **Output shape** | Full-file or coherent raw Go when asked | **Preamble** (“Here is…”) + **` ```go` ** fence |
| **Scope of code** | `package`, `import`, `func` | **~99%** of sampled labels are **function bodies only** (no `package` line) |
| **System message** | “Raw Go only” / quick-win OUTPUT | Training **system** says to use **markdown fenced** ` ```go` |
| **v1 dataset (first run)** | N/A | Many examples are **not valid Go** (e.g. brace fragments, generic users) |

LoRA **does not add “reasoning”**; it **moves logits** toward the **conditional distribution of the training labels**. If labels are **chat + fenced snippets**, inference that asks for **raw full files** will stay **out-of-distribution** no matter how many epochs you run.

## Dataset v1 (`zen_go_training.jsonl`) — “first time prompts not complex enough”

Aggregated stats (measured on a representative file in `~/git/ai`):

- **~3,008** rows.
- **Assistant** messages are **short** (median ~**511** chars), **not** structured like autowork/quick-win.
- **~0.5%** of assistant turns contain **` ```go` ** fences; **~0.5%** contain **`package `**.
- **Example failure mode:** user asks for a function “similar to `main`”; assistant outputs **prose + a ` {` … `}` fragment** without a proper `func` header — **not compilable** as a file or snippet.

So the **first** training run often taught **non-Go or malformed** text, or **non-code** commentary. Complexity was not the only issue; **label quality and format** were inconsistent with **compile-verified** codegen.

## Dataset v2 (`zen_go_training_v2.jsonl`) — “second time, unclear why”

Built by `build_zen_go_training_jsonl.py` (see that script for exact rules). Statistics:

- **~20,799** rows — large enough to **strongly** overwrite base behavior.
- **100%** of assistant turns contain **markdown fences** (` ``` ` / ` ```go` ** **).
- **100%** match the **“Here is the …”** template from `_assistant_file` / `_assistant_func`.
- **~1.9%** of assistant turns contain the substring **`package `** (full-file examples are rare).
- On a **random sample of 500** rows, **~99%** of the extracted code block **does not** start with **`package `**; **~99%** start as **`func`** (function-only chunks).

**System prompt in training** explicitly instructs:

> Respond with … **correct, compilable-shaped Go in markdown fenced code blocks (` ```go`)**.

That **directly conflicts** with evaluation prompts that require **no markdown** and **raw** `package` first line (see [QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md](QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md) and [GO_SUBTASK_LLAMA_CPP_HARNESS.md](GO_SUBTASK_LLAMA_CPP_HARNESS.md)).

So the **second** run did add structure, but the **target distribution** is still **assistant-chat + fenced partial code**, not **single-file module output**.

## Training configuration (Colab / `SFTTrainer`)

Typical settings:

- **`MAX_SEQ = 256`** (or similar) in the notebook — **short** compared to full files and even many imports + one function.
- **Long sequences** are dropped or truncated by the trainer; the model sees **many short completions** → **EOS after short spans** at inference (high **logit** on end-of-turn after a small fence).
- **LoRA** (low-rank adapters) fits **style** (preamble, fences) more easily than learning full **compiler-aligned** semantics.

Together: **data + seq length + LoRA capacity** favor **snippet + chat** over **full-file** behavior.

## Merge / GGUF (secondary)

- **`merge_qwen_lora_to_hf.py --tokenizer-from base`** is correct so **EOS / chat** metadata matches Qwen’s chat template (avoid broken stopping in llama.cpp).
- That fixes **tokenization / stop** issues; it does **not** fix the **distribution** above — merged weights still **prefer** patterns seen in SFT.

## What would make a *next* LoRA more useful

1. **Align labels with evaluation**  
   - Assistant **= only** the file body (or only the ` ```go` ** **block if you must keep fences**, but then **compile** the extracted block).  
   - **Minimum:** include **`package` + imports + code** for every example, or split into **two-stage** data (file header vs body) explicitly.

2. **Match system prompt**  
   - Use the **same** system string as production / quick-win-L1 (`OUTPUT: complete file only`), **no** instruction to always use markdown unless the product truly uses fences.

3. **Filter by `go build`**  
   - Pipeline: generate candidate → **extract** → **`go build`** in temp module → **drop** failures.  
   - Even a few thousand **clean** examples beat tens of thousands of **almost**-Go snippets.

4. **Sequence length**  
   - **`max_seq`** ≥ **longest** typical file (often **512–2048+** tokens for real packages); otherwise the model never sees **end-to-end** structure.

5. **Balance**  
   - Mix **whole-file** (small files) and **targeted edits** if the product needs both; current v2 is **heavily** skewed to **function chunks**.

6. **Judge LoRA on compile metrics**  
   - Same harness as [GO_SUBTASK_LLAMA_CPP_HARNESS.md](GO_SUBTASK_LLAMA_CPP_HARNESS.md); do **not** rely on loss curves alone.

## References

- [QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md](QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md) — base vs LoRA empirical results  
- [GO_SUBTASK_LLAMA_CPP_HARNESS.md](GO_SUBTASK_LLAMA_CPP_HARNESS.md) — how to run the `go build` harness  
- `build_zen_go_training_jsonl.py` (operator repo) — exact v2 label format  
- `config/task-templates/quickwin-l1.yaml` — what “good” L1 output shape looks like in product  

## Related

- [SMALL_MODEL_STRATEGY.md](../03-DESIGN/SMALL_MODEL_STRATEGY.md) — calibration and escalation  
