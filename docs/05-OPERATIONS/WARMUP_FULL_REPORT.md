# Full Report: How to Warm Up the 0.8B Model — **Ollama L0 Fallback**

> **NOTE:** This covers warmup for the Ollama L0 fallback lane. Primary runtime is **llama.cpp** (L1/L2). Model (Ollama)

This document is the single detailed reference for warming up the **qwen3.5:0.8b** model (and other Ollama models) in the zen-brain / zen-brain1 ecosystem. It covers manual steps, curl commands, troubleshooting, and where warmup runs in code.

---

## 1. What Is Warmup and Why It Matters

### 1.1 Cold start vs warm

- **Cold:** The first request to a model after Ollama starts (or after the model was unloaded) triggers **model load** (load weights into memory). This can take **30–120+ seconds** for 0.8B depending on hardware; larger models take longer.
- **Warm:** A prior “warmup” request has already loaded the model. Subsequent requests get **low latency** (typically a few seconds for 0.8B).

### 1.2 What “warmup” does

Warmup is one or more **minimal requests** sent to Ollama **before** the first real user/task request, so that:

1. The model is loaded into memory.
2. The first real request does not pay the cold-start delay.
3. Optionally, the model is kept resident for a period (`keep_alive`) to avoid unloading.

### 1.3 Two main patterns in our systems

| System | Pattern | When it runs |
|--------|---------|----------------|
| **zen-brain1 apiserver** | Preload via `POST /api/generate` + verify via `POST /api/chat` with `keep_alive`. Single-flight at startup. | At apiserver startup when `OLLAMA_BASE_URL` is set. See [OLLAMA_WARMUP_RUNBOOK.md](./OLLAMA_WARMUP_RUNBOOK.md). |
| **zen-brain gateway** | One-shot probe: `POST baseURL/chat/completions` with `max_tokens: 1` (or TTL-based per-request probe). | Before first task run, on `/ai set workhorse`, `/diag warmup`, queue init, etc. |

This report focuses on **how to warm up manually** (curl, scripts) and **how to troubleshoot** (ports, HTTP 000, timeouts). For apiserver startup behavior, use the runbook above.

---

## 2. Prerequisites: Ollama Must Be Running and Reachable

### 2.1 Check that Ollama is listening

Warmup only works if **Ollama is actually running** and bound to the host/port you use.

**On the machine where Ollama runs:**

```bash
# Is anything listening on 11434?
ss -tlnp | grep 11434
# or
netstat -tlnp | grep 11434
```

If nothing appears, Ollama is not listening on 11434. Start Ollama (e.g. `ollama serve` or your service/container) and optionally set:

```bash
export OLLAMA_HOST=0.0.0.0:11434   # listen on all interfaces
# or
export OLLAMA_HOST=127.0.0.1:11434  # local only
```

Then confirm again with `ss` / `netstat`.

### 2.2 Know your Ollama base URL

- **Same machine:** `http://127.0.0.1:11434` or `http://localhost:11434`
- **Remote / different IP:** `http://<host>:11434` (e.g. `http://127.0.1.6:11434` or `http://ollama:11434` in Kubernetes)

If you use the wrong host or port, you will get **connection refused** and curl will return **HTTP 000** (see Troubleshooting).

### 2.3 Pull the model once

```bash
ollama pull qwen3.5:0.8b
```

(Or use whatever host/port your `ollama` CLI is configured for.)

---

## 3. How to Warm Up Manually (Curl)

Use the **same base URL** that your application uses (same host and port).

### 3.1 Option A: Ollama native `/api/chat` (recommended for direct Ollama)

This is the same path zen-brain1 apiserver uses for verify.

```bash
OLLAMA_URL="${OLLAMA_URL:-http://127.0.0.1:11434}"

curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST "${OLLAMA_URL}/api/chat" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3.5:0.8b",
    "messages": [{"role": "user", "content": "."}],
    "stream": false,
    "keep_alive": "30m"
  }' \
  --max-time 120
```

- **Success:** `HTTP_CODE:200` and a JSON body with `message.content`.
- **Optional:** Set `OLLAMA_URL` to your real Ollama base (e.g. `http://127.0.1.6:11434`) before running.

### 3.2 Option B: Ollama native `/api/generate` (preload only)

Used by zen-brain1 for preload step (no chat response, just load model).

```bash
OLLAMA_URL="${OLLAMA_URL:-http://127.0.0.1:11434}"

curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST "${OLLAMA_URL}/api/generate" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3.5:0.8b",
    "prompt": "",
    "keep_alive": "30m"
  }' \
  --max-time 120
```

### 3.3 Option C: OpenAI-compatible `/v1/chat/completions` (if your stack uses it)

Some setups put an OpenAI-compatible proxy in front of Ollama. If your app talks to `http://host:port/v1/chat/completions`, use:

```bash
BASE_URL="${BASE_URL:-http://127.0.0.1:11434}"

curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3.5:0.8b",
    "messages": [{"role": "user", "content": "."}],
    "max_tokens": 1,
    "stream": false
  }' \
  --max-time 120
```

(Ollama’s default server does not expose `/v1/chat/completions`; this is for proxies or custom deployments.)

### 3.4 One-liners for copy-paste

**Local Ollama (127.0.0.1:11434):**

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST "http://127.0.0.1:11434/api/chat" \
  -H "Content-Type: application/json" \
  -d '{"model": "qwen3.5:0.8b", "messages": [{"role": "user", "content": "."}], "stream": false, "keep_alive": "30m"}' \
  --max-time 120
```

**Remote Ollama (e.g. 127.0.1.6:11434):**

```bash
curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST "http://127.0.1.6:11434/api/chat" \
  -H "Content-Type: application/json" \
  -d '{"model": "qwen3.5:0.8b", "messages": [{"role": "user", "content": "."}], "stream": false, "keep_alive": "30m"}' \
  --max-time 120
```

---

## 4. Troubleshooting

### 4.1 HTTP 000 from curl

**Symptom:** `HTTP_CODE:000` when running the warmup curl.

**Meaning:** Curl did not receive any HTTP response. Usually:

- **Connection refused** – nothing is listening on that host:port.
- **Connection timeout** – host unreachable or firewall dropping.
- **Network unreachable** – wrong IP or routing.

**What to do:**

1. **Confirm the correct host and port.**  
   - zen-brain1 apiserver uses `OLLAMA_BASE_URL` (e.g. `http://ollama:11434` or `http://127.0.1.6:11434`).  
   - Ollama’s default port is **11434**. Port **8080** is often a different service (e.g. zen-brain API server), not Ollama.

2. **Check that Ollama is listening on that host:port:**

   ```bash
   # On the machine that has the Ollama process
   ss -tlnp | grep 11434
   ```

   If 11434 does not appear, start Ollama and/or set `OLLAMA_HOST` so it binds to the desired address.

3. **Test with verbose curl** to see the exact error:

   ```bash
   curl -v -X POST "http://127.0.1.6:11434/api/chat" \
     -H "Content-Type: application/json" \
     -d '{"model": "qwen3.5:0.8b", "messages": [{"role": "user", "content": "."}], "stream": false}' \
     --max-time 10
   ```

   Look for `Connection refused` or `Failed to connect`.

4. **If Ollama is on another host:** Replace the URL in the curl command with that host and 11434 (e.g. `http://<real-ollama-host>:11434`).

### 4.2 Connection refused to 127.0.1.6:11434

**Symptom:** `connect to 127.0.1.6 port 11434 ... Connection refused`.

**Meaning:** On 127.0.1.6, no process is listening on port 11434. Common causes:

- Ollama is not running on that host.
- Ollama is running but bound only to 127.0.0.1 on a different machine (so 127.0.1.6 is not that machine’s Ollama).
- Ollama is listening on a different port (check with `ss -tlnp` / `netstat -tlnp` on the Ollama host).

**What to do:** Start Ollama on the host that has 127.0.1.6 (or the host you intend to use) and ensure it listens on 11434; or point your warmup and app to the host/port where Ollama actually runs.

### 4.3 Timeout on first request

**Symptom:** Curl (or the app) times out after 60–120 s on the first request.

**Meaning:** Model is loading (cold start). 0.8B can take 30–90+ seconds on first load depending on CPU/memory.

**What to do:**

- Use a longer `--max-time` (e.g. 300) for the warmup curl.
- In zen-brain1, set `OLLAMA_TIMEOUT_SECONDS` (e.g. 600) so the first request has time to complete.
- After a successful warmup, subsequent requests should be fast; if they are slow again, check `keep_alive` (model may be unloading).

### 4.4 Model not found / 404

**Symptom:** Ollama returns 404 or “model not found”.

**What to do:** On the Ollama host run `ollama pull qwen3.5:0.8b` (or the exact model name you use).

### 4.5 Wrong server on port 8080

**Symptom:** You use `http://127.0.1.6:8080` for warmup and get 404 or unexpected responses.

**Meaning:** Port 8080 in our setups is often the **zen-brain API server** (sessions, health, evidence), not Ollama. That server does not expose `/api/chat` or model inference.

**What to do:** Use the **Ollama** URL and port **11434** for warmup (e.g. `http://127.0.1.6:11434` if Ollama is on that host). Use 8080 only for the API server’s own endpoints (e.g. `/healthz`, `/api/v1/...`).

---

## 5. Where Warmup Happens in Code (Reference)

### 5.1 zen-brain1 (apiserver)

- **Startup:** When `OLLAMA_BASE_URL` is set, a warmup goroutine runs:
  - `POST /api/generate` with `model`, empty `prompt`, `keep_alive`
  - Then `POST /api/chat` with one message and `keep_alive`
- **First request:** Local-worker requests can wait on this warmup (WaitReady) so the first user request does not race cold load.
- **Details:** [OLLAMA_WARMUP_RUNBOOK.md](./OLLAMA_WARMUP_RUNBOOK.md).

### 5.2 zen-brain (gateway)

- **Pre-execution:** `Factory.WarmupOllamaIfNeeded(ctx, providerName, model)` → `providers.WarmupOllama(ctx, baseURL, model)`. Called:
  - Before first task in a run (`execution_runner.go`)
  - On `/ai set primary` or `/ai set workhorse` with Ollama (goroutine)
  - On preset `workhorse` (Ollama)
  - On `/diag warmup`
- **Probe:** One-shot `POST baseURL/chat/completions` with `max_tokens: 1`, 1200s header timeout. Base URL comes from provider spec (Ollama ports 11434, 11448, 11449, 11450).
- **Per-request (Ollama only):** `OpenAIProvider.ensureOllamaWarmed` runs a TTL-based probe (5 min) before Chat/Complete when the provider base URL is Ollama.

A more detailed call-site report for zen-brain lives in the zen-brain repo (e.g. `docs/0.8B_WARMUP_AND_CALLS_REPORT.md`).

---

## 6. Quick Checklist for “Warmup returns 200”

1. **Ollama is running** on the host you use (`ss -tlnp | grep 11434` on that host).
2. **URL and port are correct:** base URL = `http://<ollama-host>:11434` (not 8080 unless you explicitly deployed Ollama there).
3. **Model is pulled:** `ollama pull qwen3.5:0.8b` on the Ollama host.
4. **Warmup request:** Use Option A (e.g. `/api/chat` with `keep_alive`) and a long enough `--max-time` (e.g. 120).
5. **Interpret result:** `HTTP_CODE:200` and JSON with `message.content` = success; `HTTP_CODE:000` = connection problem (see Troubleshooting).

---

## 7. Related Docs

| Document | Content |
|----------|---------|
| [OLLAMA_WARMUP_RUNBOOK.md](./OLLAMA_WARMUP_RUNBOOK.md) | zen-brain1 apiserver warmup at startup, config, single-flight, keep_alive. |
| zen-brain `docs/0.8B_WARMUP_AND_CALLS_REPORT.md` | All warmup call sites in zen-brain Go code and where 0.8B is referenced. |

---

*Report generated for zen-brain1 operations. For zen-brain gateway warmup behavior and call sites, see the zen-brain repo.*
