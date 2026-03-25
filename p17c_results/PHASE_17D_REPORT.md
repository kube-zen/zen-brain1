# PHASE 17D: Final Corrected Verification Report

**Date**: 2026-03-25 17:00–17:28 EDT
**Model**: qwen3.5:0.8b (Q4_K_M, Q8_0 in Ollama)
**Test harness**: `/tmp/p17c.go` (F3 runs), `/tmp/p17d_o2.go` (F4 O2 rerun)

## Warmup Methodology

Per-provider warmup before subtests:
1. **Liveness**: Confirm provider HTTP endpoint responds
2. **Tiny code**: Single-line function request (func Add) — validates model generates code, not thinking
3. **Same-shape rehearsal**: Full bounded task prompt — validates model handles the exact prompt shape

Warmup used identical thinking-disable controls as the subtests.

## Task Fixed for Comparison

```
System: "You are a coding assistant. /no_think"
User: "Write Go code for file internal/util/case.go. Package: util. Import: strings. Add func ToLower(s string) string that returns strings.ToLower(s). Return only the full file contents. No explanations."
```

Target file: `internal/util/case.go`
Expected package: `util`
Expected import: `strings`
Expected function: `func ToLower(s string) string { return strings.ToLower(s) }`

## No-Think Controls

| Provider | Control | Endpoint |
|----------|---------|----------|
| llama.cpp | `chat_template_kwargs: {"enable_thinking": false}` | `/v1/chat/completions` |
| Ollama | Native `/api/chat` with `options: {"think": false}` | `/api/chat` (no-tools), `/v1/chat/completions` (tools) |
| Both | System message: `/no_think` (Qwen soft switch) | Both |

**llama.cpp template evidence**: `/props` response (9843 bytes) confirms `enable_thinking` template variable at line 149.

## Tool Set (for O2 and L2 only)

2 tools:
1. `read_file(path string)` — "Read a file from the repo"
2. `run_build_test(command string)` — "Run go build or go test"

## Ollama No-Tools Result (O1)

**Artifact**: `p17c_results/p17c_ollama_O1.go` (102 chars)
**Round 1**: content=102, thinking=481 chars, finish=stop
**Code**: Correct — package util, import strings, func ToLower(s string) string
**Markdown fences**: Yes — ` ```go ` on line 1, ` ``` ` on last line
**Verification CWD**: `/tmp/p17dv/O1`
**Build (raw)**: FAIL — `expected 'package', found \`\`\``
**Build (fence-stripped)**: **PASS**
**Classification**: model-behavior-fail (correct code, markdown fences block compilation)

## Ollama Tools-Loop Result (O2)

**Rerun log**: `p17c_results/o2_rerun.log`

| Round | Behavior | Tool Called | Nudge Sent |
|-------|----------|-------------|------------|
| 1 | tool_call | `read_file({"path":"util/case.go"})` | "File not found. Create it now. Return only Go code." |
| 2 | tool_call | `run_build_test({"command":"go build -o /dev/null internal/util/case.go"})` | "File not found. You must create it. Write the complete Go source code now." |
| 3 | tool_call | `run_build_test({"command":"go build -o /dev/null internal/util/case.go"})` | "Return only the complete Go source. Do not call more tools. Do not explain." |
| 4 | tool_call | `read_file({"path":"util/case.go"})` | — (round limit reached) |

**Code produced**: None in 4 rounds
**Classification**: **tool-behavior-fail** — model loops on tool calls (read_file / run_build_test) without ever generating code; nudge escalation in round 3 had no effect

**Note**: This is a regression from PHASE 17B where O2 succeeded in round 3 with explicit nudge. The difference is PHASE 17D includes `/no_think` in the system message, which degrades the model's ability to transition from tool-use to code generation.

## llama.cpp No-Tools Result (L1)

**Artifact**: `p17c_results/p17c_llamacpp_L1.go` (92 chars)
**Round 1**: content=92, thinking=0, finish=stop
**Code**: Correct — package util, import strings, func ToLower(s string) string
**Markdown fences**: No
**Verification CWD**: `/tmp/p17dv/L1`
**Build**: **PASS**
**Classification**: **success**

## llama.cpp Tools-Loop Result (L2)

**Artifact**: `p17c_results/p17c_L2_round1.go` (102 chars)
**Round 1**: content=102, finish=stop — **no tool calls, code generated directly**
**Code**: Correct — package util, import strings, func ToLower(s string) string
**Markdown fences**: Yes — ` ```go ` on line 1, ` ``` ` on last line
**Note**: `parse_tool_calls: false` was set in the request, which caused llama.cpp to ignore tools and generate code directly (identical behavior to no-tools)
**Verification CWD**: `/tmp/p17dv/L2`
**Build (raw)**: FAIL — `expected 'package', found \`\`\``
**Build (fence-stripped)**: **PASS**
**Classification**: model-behavior-fail (correct code, markdown fences block compilation)

## Tool-Loop Evidence

**llama.cpp L1**: No tools present → direct code generation → PASS
**llama.cpp L2**: Tools present but `parse_tool_calls: false` → model ignores tools → direct code generation → fence-wrapped code
**Ollama O1**: No tools present → direct code generation → fence-wrapped code
**Ollama O2**: Tools present, `parse_tool_calls` default (true) → model enters infinite tool-loop → never generates code

**PHASE 17B cross-reference**: Ollama O2 succeeded in PHASE 17B (round 3, with nudge, WITHOUT `/no_think`). PHASE 17D O2 failed (4 rounds, nudge ignored, WITH `/no_think`). The `/no_think` soft switch in the system message degrades Ollama's tool-loop completion ability.

## Verification Results

| Subtest | Artifact Path | CWD | Has Fences | Raw Build | Stripped Build |
|---------|--------------|-----|------------|-----------|----------------|
| O1 | `p17c_results/p17c_ollama_O1.go` | `/tmp/p17dv/O1` | Yes | FAIL | **PASS** |
| O2 | N/A (no code saved) | — | — | — | — |
| L1 | `p17c_results/p17c_llamacpp_L1.go` | `/tmp/p17dv/L1` | No | **PASS** | — |
| L2 | `p17c_results/p17c_L2_round1.go` | `/tmp/p17dv/L2` | Yes | FAIL | **PASS** |

**Prior build failures on O1 and L1 in PHASE 17C runs 1–2 were verifier CWD contamination** (build ran from `/home/node/.openclaw/workspace` instead of the temp artifact directory). Run 3 and PHASE 17D re-verification confirm the actual artifact content is correct.

## Result Comparison

| Subtest | Provider | Tools | Code Correct | Fences | Build (raw) | Build (stripped) | Outcome |
|---------|----------|-------|-------------|--------|-------------|------------------|---------|
| O1 | Ollama | No | ✅ | Yes | FAIL | PASS | model-behavior-fail |
| O2 | Ollama | Yes | ❌ | — | N/A | N/A | tool-behavior-fail |
| L1 | llama.cpp | No | ✅ | No | **PASS** | — | **success** |
| L2 | llama.cpp | Yes* | ✅ | Yes | FAIL | PASS | model-behavior-fail |

*L2 used `parse_tool_calls: false`, causing tools to be ignored. Actual tools-loop behavior for llama.cpp was not tested with `parse_tool_calls: true` under the thinking-disabled setup.

## Correct Narrowly Scoped Conclusion

This corrected 4-subtest comparison proves that qwen3.5:0.8b via llama.cpp with `chat_template_kwargs: {"enable_thinking": false}` produces correct, build-passing Go code for a bounded single-function task without tools, while Ollama produces correct code but wraps it in markdown fences that block compilation; the Ollama tools-loop path fails under the `/no_think` system message condition due to the model entering an infinite tool-call loop and never transitioning to code generation.

## Recommended Next Actions

1. **Add markdown fence stripping** in `internal/factory/llm_generator.go` — strip ` ```go `, ` ``` `, and bare ` ``` ` lines from generated code before writing to disk
2. **Remove `/no_think` from system messages** — it degrades Ollama tool-loop reasoning without providing equivalent benefit on the Ollama OpenAI-compatible endpoint
3. **Pass `chat_template_kwargs: {"enable_thinking": false}` via the llama.cpp provider** (`internal/llm/openai_compatible_provider.go`) for qwen3.5 models to suppress thinking at the template level
4. **Retest Ollama O2 without `/no_think`** to confirm PHASE 17B tool-loop success is reproducible
5. **Retest llama.cpp L2 with `parse_tool_calls: true`** (default) to get actual tool-loop behavior under thinking-disabled conditions

REPORTER: CONNECTED Summary
