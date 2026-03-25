# PHASE 17C: Thinking-Disabled Comparison — qwen3.5:0.8b

**Date**: 2026-03-25 17:00–17:14 EDT
**Model**: qwen3.5:0.8b (Q4_K_M)
**Test harness**: `/tmp/p17c.go` (Go binary, self-contained)

## Purpose

PHASE 17B showed both providers burn tokens on thinking mode, producing empty content. This phase tests whether disabling thinking via provider-specific switches changes the outcome for the same bounded code task.

## Thinking-Disable Mechanism

| Provider | Switch | Endpoint |
|----------|--------|----------|
| llama.cpp | `chat_template_kwargs: {"enable_thinking": false}` + system msg `/no_think` | `/v1/chat/completions` |
| Ollama | `options: {"think": false}` (native `/api/chat`) + system msg `/no_think` | `/api/chat` |

**llama.cpp template evidence**: `/props` response contains `enable_thinking` template variable (line 149: `{%- if enable_thinking is defined and enable_thinking is true %}`), confirming the switch is supported.

**Verification probe** (pre-test): llama.cpp with both switches → 63 chars, 0 reasoning tokens, 18 completion tokens, `finish: stop`. ✅

## Bounded Task

```
System: "You are a coding assistant. /no_think"
User: "Write Go code for file internal/util/case.go. Package: util. Import: strings. Add func ToLower(s string) string that returns strings.ToLower(s). Return only the full file contents. No explanations."
```

## Tool Set (for O2/L2 subtests)

```json
[
  {"type":"function","function":{"name":"read_file","description":"Read a file from the repo","parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}}},
  {"type":"function","function":{"name":"run_build_test","description":"Run go build or go test","parameters":{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}}}
]
```

## Warmup Results

| ID | Provider | Type | Content | Thinking | Notes |
|----|----------|------|---------|----------|-------|
| B1 | Ollama | tiny code (think=false) | 50 chars | 460 chars | ✅ Code produced |
| C1 | Ollama | same-shape | 102 chars | 483 chars | ✅ Correct code with fences |
| A2 | llama.cpp | liveness | — | — | ✅ |
| B2 | llama.cpp | tiny code | 43 chars | — | ✅ |
| C2 | llama.cpp | same-shape | 92 chars | — | ✅ Correct code, no fences |

## Subtest Results

### O1: Ollama no-tools (think=false)

- **Content**: 102 chars
- **Thinking**: 481 chars (Ollama native API still emits thinking even with `think: false`)
- **Code**: ✅ Correct — package util, import strings, func ToLower(s string) string
- **Markdown fences**: ❌ Wrapped in ` ```go ... ``` `
- **Build**: FAIL (fences cause parse error)
- **Classification**: **model-behavior-fail** — correct code but wrapped in markdown

### O2: Ollama tools-loop (think=false)

- **Round 1**: `tool_call: read_file({"path":"internal/util/case.go"})` → sent "File not found"
- **Round 2**: `tool_call: read_file({"path":"internal/util/case.go"})` → sent "Stop calling tools..."
- **Round 3**: `tool_call: read_file({"path":"internal/util/case.go"})` → sent "Stop calling tools..."
- **Round 4**: content_len=0, finish=stop
- **Classification**: **model-behavior-fail** — infinite read_file loop, never produces code
- **Regression**: PHASE 17B O2 succeeded in round 3; `/no_think` system message degrades tool-loop reasoning

### L1: llama.cpp no-tools (think=false)

- **Content**: 92 chars
- **Thinking**: 0 (fully suppressed)
- **Code**: ✅ Correct — package util, import strings, func ToLower(s string) string
- **Markdown fences**: ✅ No fences
- **Build**: **PASS**
- **Classification**: **success**

### L2: llama.cpp tools-loop (think=false, parse_tool_calls=false)

- **Round 1**: content_len=102, finish=stop — **no tool calls, code generated directly**
- **Code**: ✅ Correct
- **Markdown fences**: ❌ Wrapped in ` ```go ... ``` `
- **Build**: FAIL (fences)
- **Note**: With `parse_tool_calls: false`, llama.cpp ignored tools and generated code directly (same as no-tools)
- **Classification**: **model-behavior-fail** — correct code but wrapped in markdown

## Comparison Table

| Subtest | Provider | Tools | Thinking | Rounds | Code Correct | Fences | Build | Outcome |
|---------|----------|-------|----------|--------|-------------|--------|-------|---------|
| O1 | Ollama | No | 481 chars | 1 | ✅ | ❌ | FAIL | model-behavior-fail |
| O2 | Ollama | Yes | — | 4 | ❌ | — | N/A | model-behavior-fail |
| L1 | llama.cpp | No | 0 | 1 | ✅ | ✅ | **PASS** | **success** |
| L2 | llama.cpp | Yes | 0 | 1 | ✅ | ❌ | FAIL | model-behavior-fail |

## Key Findings

1. **llama.cpp with `chat_template_kwargs: {"enable_thinking": false}` fully suppresses thinking** — zero reasoning tokens, clean code output
2. **Ollama `think: false` does NOT fully suppress thinking** — still emits 460-481 chars of thinking, but no longer blocks content output
3. **llama.cpp L1 is the only clean success** — correct code, no fences, BUILD PASS
4. **Markdown fences** are the remaining blocker — Ollama and llama.cpp (via OpenAI-compatible endpoint) both wrap code in ` ```go ... ``` ` fences, causing build failures
5. **Ollama tool-loop regressed with `/no_think`** — the soft switch in the system message appears to degrade the model's ability to reason through tool calls (PHASE 17B succeeded in round 3; PHASE 17C loops indefinitely)
6. **`parse_tool_calls: false` on llama.cpp** causes tools to be completely ignored — model generates code directly as if no tools were present

## Correct Narrowly Scoped Conclusion

This isolated retest proves that qwen3.5:0.8b via llama.cpp with `chat_template_kwargs: {"enable_thinking": false}` can produce correct, build-passing Go code for a bounded single-function task without tools and without token burn on thinking. It also shows that the model wraps output in markdown fences when using the OpenAI-compatible endpoint, and that Ollama's `think: false` does not fully suppress thinking output.

## Recommended Next Actions

1. **Add markdown fence stripping** in `llm_generator.go` — strip ` ```go ` and ` ``` ` from generated code before writing to file
2. **Do NOT add `/no_think` to system messages** — it degrades Ollama tool-loop reasoning
3. **Pass `chat_template_kwargs: {"enable_thinking": false}` via llama.cpp provider** for qwen3.5 models to suppress thinking at the template level
4. **Re-test Ollama tool-loop WITHOUT `/no_think`** but WITH `think: false` to confirm the regression is caused by the soft switch
5. **Re-test llama.cpp tools-loop WITH `parse_tool_calls: true`** (default) to get actual tool-loop behavior

## Artifacts

All artifacts in `~/zen/zen-brain1/p17c_results/`:
- `p17c_ollama_O1.go` — 102 chars, correct with fences
- `p17c_llamacpp_L1.go` — 92 chars, correct, BUILD PASS
- `p17c_L2_round1.go` — 102 chars, correct with fences
- `p17c_ollama_warmupC.txt` — 102 chars, correct with fences
- `run.log`, `run2.log`, `run3.log` — full execution logs

## Raw Evidence

### O1 artifact:
```go
package util

import "strings"

func ToLower(s string) string {
	return strings.ToLower(s)
}
```

### L1 artifact:
```go
package util

import "strings"

func ToLower(s string) string {
	return strings.ToLower(s)
}
```

REPORTER: CONNECTED Summary
