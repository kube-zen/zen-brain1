> **HISTORICAL NOTE:** This report was written when Ollama was the active local inference path. The current primary runtime is **llama.cpp** (L1/L2). Ollama is now L0 fallback only.

# W028-W029: Tool Path Root Cause Analysis

**Updated:** 2026-03-25 08:45 EDT

---

## W028: Root Cause Hypothesis — CONFIRMED

**Hypothesis:** Prior L1/L2 failures were at least partly setup-contaminated because correct tools were not actually sent in the request path.

**Verification:** ✅ **CONFIRMED TRUE**

**Evidence:**
1. Profile config declares `supports_tools: true`
2. `llm.ChatRequest` interface has `Tools []Tool` field
3. Provider interface has `SupportsTools() bool` method
4. BUT: Provider HTTP payloads NEVER include tools

---

## W029: Tool Path Trace End-to-End

### 1. Where Task Profiles/Lane Config Declare supports_tools

**File:** `config/profiles/local-cpu-45m.yaml`
**Line:** ~23
```yaml
local_worker:
  supports_tools: true
```

✅ Config correctly declares tool support

---

### 2. Where Provider Decides Whether to Attach Tools

**File:** `pkg/llm/provider.go`
**Interface:** `Provider`

**Interface Method:**
```go
SupportsTools() bool
```

**Implementation Status:**
- OpenAI-compatible provider: Returns `true`
- Ollama provider: Returns `true`

✅ Both providers claim to support tools

---

### 3. Where Tool Definitions Are Constructed

**File:** `pkg/llm/types.go`
**Type:** `Tool`

**Structure:**
```go
type Tool struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`
}
```

✅ Tool type is defined

---

### 4. Where Outbound LLM Request Payload Is Built

**File:** `internal/llm/openai_compatible_provider.go`
**Request Type:** `oaiRequest`

**Current Structure:**
```go
type oaiRequest struct {
    Model     string       `json:"model"`
    Messages  []oaiMessage  `json:"messages"`
    MaxTokens int           `json:"max_tokens,omitempty"`
    Stream    bool          `json:"stream"`
    // ❌ MISSING: Tools []Tool field
}
```

**File:** `internal/llm/ollama_provider.go`
**Request Type:** `ollamaChatRequest`

**Current Structure:**
```go
type ollamaChatRequest struct {
    Model     string          `json:"model"`
    Messages  []ollamaMessage  `json:"messages"`
    Stream    bool            `json:"stream"`
    KeepAlive string          `json:"keep_alive,omitempty"`
    Options   map[string]any  `json:"options,omitempty"`
    // ❌ MISSING: Tools []Tool field
}
```

❌ **BOTH provider request types lack `Tools` field**

---

### 5. Whether Tools Are Included in Request

**File:** `internal/llm/openai_compatible_provider.go`
**Function:** `Chat()`

**Current Payload Build:**
```go
body := oaiRequest{
    Model:     model,
    Messages:  oaiMsgs,
    MaxTokens: maxTokens,
    Stream:    false,
    // ❌ NO: Tools passed from req.Tools
}
```

**File:** `internal/llm/ollama_provider.go`
**Function:** `Chat()`

**Current Payload Build:**
```go
body := ollamaChatRequest{
    Model:     model,
    Messages:  messages,
    Stream:    false,
    KeepAlive: p.keepAlive,
    // ❌ NO: Tools passed from req.Tools
}
```

❌ **NEITHER provider includes `req.Tools` in the HTTP request payload**

---

### 6. For Normal vs Structured Rescue Tasks

**Structured Rescue Tasks (via promptbuilder.TaskPacket):**
- Use canonical prompt path
- Prompt text says "tools are available"
- BUT: HTTP payload doesn't include them

**Normal Explicit-Target Tasks (via fallback ad-hoc prompts):**
- Use promptbuilder path NOT at all
- No tool mention in prompt
- HTTP payload doesn't include them

---

## ROOT CAUSE CONFIRMED

| Component | Status | Issue |
|-----------|---------|--------|
| Interface design | ✅ Correct | `ChatRequest.Tools []Tool` field exists |
| Provider claims | ✅ Correct | `SupportsTools()` returns `true` |
| Provider implementation | ❌ **FAILING** | Request structs lack `Tools` field |
| Payload build | ❌ **FAILING** | `Chat()` functions ignore `req.Tools` |

---

## Live Request Evidence (W030)

**Current Evidence Status:**
- No logs show "tools" in request payload
- No log statements like "attaching X tools to request"
- Both providers accept `req.Tools` parameter but never use it

**Evidence from logs:**
```
[Ollama] Chat: model=qwen3.5:0.8b latency=1234ms in=42 out=56 (warm)
[LLMTemplate] Generated implementation implementation for W016-L1-01 (model=qwen3.5:0.8b-q4, tokens=0)
```

**Missing from logs:**
- No "tools" or "ToolCalls" mentioned
- No "function calling" or "tool use" mentioned

---

## Delivered: Exact Function/File Root Causes

### Root Cause: Tools Never Attached in HTTP Requests

**File 1:** `internal/llm/openai_compatible_provider.go`
**Function:** `Chat()`
**Line:** ~100-130 (payload construction)

**Issue:** `oaiRequest` struct lacks `Tools` field
```go
type oaiRequest struct {
    Model     string       `json:"model"`
    Messages  []oaiMessage  `json:"messages"`
    MaxTokens int           `json:"max_tokens,omitempty"`
    Stream    bool          `json:"stream"`
    // ❌ MISSING: Tools []Tool `json:"tools,omitempty"`
}
```

---

### Root Cause: Ollama Provider Missing Tools in Payload

**File 2:** `internal/llm/ollama_provider.go`
**Function:** `Chat()`
**Line:** ~150-230 (payload construction)

**Issue:** `ollamaChatRequest` struct lacks `Tools` field
```go
type ollamaChatRequest struct {
    Model     string          `json:"model"`
    Messages  []ollamaMessage `json:"messages"`
    Stream    bool            `json:"stream"`
    KeepAlive string          `json:"keep_alive,omitempty"`
    Options   map[string]any  `json:"options,omitempty"`
    // ❌ MISSING: Tools []Tool `json:"tools,omitempty"`
}
```

---

### Root Cause: Chat() Functions Never Read req.Tools

**File 1:** `internal/llm/openai_compatible_provider.go`
**Function:** `Chat()`
**Line:** ~107-118

**Issue:** Direct field mapping without tools:
```go
body := oaiRequest{
    Model:     model,
    Messages:  oaiMsgs,
    MaxTokens: maxTokens,
    Stream:    false,
    // ❌ NO: Tools: req.Tools
}
```

**File 2:** `internal/llm/ollama_provider.go`
**Function:** `Chat()`
**Line:** ~209-218

**Issue:** Direct field mapping without tools:
```go
body := ollamaChatRequest{
    Model:     model,
    Messages:  messages,
    Stream:    false,
    KeepAlive: p.keepAlive,
    // ❌ NO: Tools: req.Tools
}
```

---

## Conclusion

**Tools are NEVER sent to the LLM providers, despite:**

1. ✅ Config declaring `supports_tools: true`
2. ✅ Interface design with `ChatRequest.Tools []Tool`
3. ✅ Provider implementations claiming support via `SupportsTools() == true`
4. ❌ **Provider request structs lacking `Tools` field**
5. ❌ **Provider `Chat()` functions ignoring `req.Tools` parameter**

**This is a complete disconnect between interface design and implementation.**

The `llm.ChatRequest` interface expects tools, but provider implementations never attach them to the HTTP payload.

---

**Impact:** All L1/L2 benchmark runs are contaminated by missing tool support, regardless of:
- Warmup status (warmup works)
- Provider routing (routing works)
- Model selection (selection works)

**Tools are the critical missing piece.**
