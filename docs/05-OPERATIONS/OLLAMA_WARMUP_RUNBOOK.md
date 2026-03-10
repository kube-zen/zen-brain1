# Ollama warmup runbook (Block 5 local-worker)

This runbook describes how zen-brain apiserver warms the Ollama model so the first `/api/v1/chat` request does not hit a cold load. It matches Ollama’s official preload behavior and the pattern used in zen-brain 0.1.

## Pattern in brief

1. **Preload** using Ollama’s official path: `POST /api/generate` with empty prompt and `keep_alive`.
2. **Verify** on the real app path: one tiny `POST /api/chat` with `keep_alive` so the first user request uses the same endpoint.
3. **Single-flight**: warmup runs once at startup in a goroutine; the first local-worker request can wait briefly on that same warmup instead of racing it.
4. **Keep resident**: `keep_alive` (e.g. `"30m"`) keeps the model in memory so the first real request is warm.

## Config (ownership)

| Env / config | Purpose | Default / recommendation |
|-------------|---------|---------------------------|
| `OLLAMA_BASE_URL` | Ollama server URL (e.g. `http://ollama:11434`) | Set when Ollama is deployed |
| `OLLAMA_TIMEOUT_SECONDS` | HTTP timeout for local-worker (and warmup) | Chart default e.g. 600 for sandbox |
| `OLLAMA_KEEP_ALIVE` | Duration to keep model resident after preload/verify | `"30m"`; `-1` for indefinite in dedicated sandbox |

Helm: `ollama.baseUrl`, `ollama.timeoutSeconds`, `ollama.keepAlive` in zen-brain chart values.

## Manual preload (for verification or one-off)

Preload (official minimal path):

```bash
curl -sS -X POST http://ollama:11434/api/generate -H "Content-Type: application/json" -d '{
  "model": "qwen3.5:0.8b",
  "keep_alive": "30m"
}'
```

Verify on the chat path (same endpoint the app uses):

```bash
curl -sS -X POST http://ollama:11434/api/chat -H "Content-Type: application/json" -d '{
  "model": "qwen3.5:0.8b",
  "messages": [{"role":"user","content":"."}],
  "stream": false,
  "keep_alive": "30m"
}'
```

Check the response for `load_duration`: non-zero means the model was cold (just loaded); zero or absent means warm.

## What the apiserver does at startup

When `OLLAMA_BASE_URL` is set:

1. A **warmup coordinator** is created (single-flight per process).
2. A goroutine runs **DoWarmup**:
   - `POST /api/generate` with `model`, `prompt: ""`, `keep_alive`.
   - Then `POST /api/chat` with one message `"."`, `stream: false`, `keep_alive`.
3. Logs: `[Ollama] preload done`, `[Ollama] verify chat done`, then `[Ollama] warmup done: model=... load_duration=... total=... keep_alive=...` or `[apiserver] Ollama warmup failed (non-fatal): ...`.

## First user request

- Local-worker requests (including `X-LLM-Provider: local-worker`) call **WaitReady** with a bounded wait (e.g. 60s). If warmup is still in progress, the request waits on the same single-flight; if warmup already finished, it returns immediately.
- Chat responses from Ollama are logged with `load_duration` when present: `(cold)` vs `(warm)` so you can confirm the model was already loaded.

## Troubleshooting

| Symptom | Check |
|--------|--------|
| First request times out | Increase `OLLAMA_TIMEOUT_SECONDS`; ensure warmup completed (logs). |
| Model unloads between warmup and first request | Increase `OLLAMA_KEEP_ALIVE` (e.g. `"30m"` or `-1`). |
| Warmup fails | Logs: `[apiserver] Ollama warmup failed`. Verify `OLLAMA_BASE_URL` and Ollama reachability; check Ollama logs. |
| Cold on every request | Verify `keep_alive` is set on preload/verify and that Ollama is not restarted between requests. |

## References

- Ollama API: `/api/generate` (preload with empty prompt), `/api/chat`, `keep_alive`.
- zen-brain 0.1: warmup with TTL, 1-token probe, doctor check against chat endpoint.
