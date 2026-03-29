#!/usr/bin/env python3
"""
ZB-025B: qwen3.5:0.8b Response-Schema Diagnostic Matrix
R012/R013 compliant: observation-only, no code changes.

Classification: protocol mismatch — content empty, reasoning_content populated
when thinking enabled on a slow local model with insufficient completion budget.

This script runs three matrices:
  Matrix A: thinking ON (default)
  Matrix B: thinking OFF (via /no_think suffix)
  Matrix C: thinking constrained (low reasoning budget if supported)

For every run captures:
  - HTTP status
  - total latency
  - content length
  - reasoning_content (thinking) length
  - parseable JSON from content
  - parseable JSON from reasoning_content
  - final classification

Output: JSONL to stdout, summary to stderr
"""

import json
import subprocess
import sys
import time
import urllib.request
import urllib.error

OLLAMA_URL = "http://localhost:11434/api/chat"
MODEL = "qwen3.5:0.8b"
RUNS_PER_CELL = 5

# Prompt classes
PROMPTS = {
    "trivial_json": 'Return ONLY this JSON: {"status":"ok"}',
    "steward_extract": 'Extract from: "Task ZB-025 is BLOCKED due to RBAC error in zen-brain namespace". Return JSON: {"task_id":"","state":"","reason":"","namespace":""}',
    "roadmap_synthesis": 'Given these tasks: ZB-023 (PASS), ZB-024 (PASS), ZB-025 (BLOCKED). Return JSON array of objects with fields: id, status, summary (max 10 words each).',
}

# Matrix A: thinking ON
MATRIX_A_TOKENS = [512, 1024, 2048]
MATRIX_A_TIMEOUTS = [30, 60, 120]

# Matrix B: thinking OFF
MATRIX_B_TOKENS = [512, 1024, 2048]
MATRIX_B_TIMEOUTS = [30, 60, 120]

# Matrix C: thinking constrained (low num_predict for reasoning)
MATRIX_C_TOKENS = [1024, 2048]
MATRIX_C_TIMEOUTS = [60, 120]


def try_parse_json(text):
    """Try to parse text as JSON, return (bool, parsed_or_None)."""
    if not text or not text.strip():
        return False, None
    text = text.strip()
    # Try direct parse
    try:
        return True, json.loads(text)
    except json.JSONDecodeError:
        pass
    # Try extracting JSON from markdown code blocks
    import re
    m = re.search(r'```(?:json)?\s*\n?(.*?)\n?```', text, re.DOTALL)
    if m:
        try:
            return True, json.loads(m.group(1).strip())
        except json.JSONDecodeError:
            pass
    # Try finding first { or [ to last } or ]
    for open_ch, close_ch in [('{', '}'), ('[', ']')]:
        start = text.find(open_ch)
        end = text.rfind(close_ch)
        if start >= 0 and end > start:
            try:
                return True, json.loads(text[start:end+1])
            except json.JSONDecodeError:
                pass
    return False, None


def classify_run(content, thinking, http_status, timed_out):
    """Classify the run result per R013."""
    if timed_out:
        return "timeout"
    if http_status != 200:
        return "transport_failure"

    has_content = content and content.strip()
    has_thinking = thinking and thinking.strip()

    if has_content:
        ok, _ = try_parse_json(content)
        if ok:
            return "success_via_content"
        return "malformed_content"

    if has_thinking and not has_content:
        return "reasoning_only"

    if not has_content and not has_thinking:
        return "transport_failure"

    return "malformed_content"


def run_single(prompt_text, max_tokens, timeout_s, thinking_mode="on"):
    """
    Run a single Ollama chat request.
    thinking_mode: "on" (default), "off" (/no_think), "constrained" (minimal reasoning)
    Returns dict with all R013 measurements.
    """
    model_name = MODEL
    if thinking_mode == "off":
        model_name = MODEL + "/no_think"

    payload = {
        "model": model_name,
        "messages": [{"role": "user", "content": prompt_text}],
        "stream": False,
        "options": {"num_predict": max_tokens},
    }

    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        OLLAMA_URL, data=data, headers={"Content-Type": "application/json"}
    )

    start = time.monotonic()
    timed_out = False
    http_status = 0
    content = ""
    thinking = ""
    eval_count = 0
    done_reason = ""
    total_duration_ns = 0

    try:
        resp = urllib.request.urlopen(req, timeout=timeout_s)
        http_status = resp.status
        body = resp.read().decode("utf-8")
        elapsed = time.monotonic() - start
        result = json.loads(body)

        msg = result.get("message", {})
        content = msg.get("content", "")
        thinking = msg.get("thinking", "")
        eval_count = result.get("eval_count", 0)
        done_reason = result.get("done_reason", "")
        total_duration_ns = result.get("total_duration", 0)

    except urllib.error.HTTPError as e:
        elapsed = time.monotonic() - start
        http_status = e.code
    except (TimeoutError, urllib.error.URLError) as e:
        elapsed = time.monotonic() - start
        timed_out = True
    except Exception as e:
        elapsed = time.monotonic() - start
        http_status = -1

    content_json, _ = try_parse_json(content)
    thinking_json, _ = try_parse_json(thinking)
    classification = classify_run(content, thinking, http_status, timed_out)

    return {
        "http_status": http_status,
        "latency_s": round(elapsed, 2),
        "content_len": len(content),
        "thinking_len": len(thinking),
        "content_json": content_json,
        "thinking_json": thinking_json,
        "eval_count": eval_count,
        "done_reason": done_reason,
        "classification": classification,
    }


def run_matrix(matrix_name, tokens, timeouts, thinking_mode, prompt_classes=None):
    """Run a full matrix of tests. Yields result dicts."""
    if prompt_classes is None:
        prompt_classes = list(PROMPTS.keys())

    total = len(tokens) * len(timeouts) * len(prompt_classes) * RUNS_PER_CELL
    done = 0

    for tok in tokens:
        for tmo in timeouts:
            for pclass in prompt_classes:
                prompt = PROMPTS[pclass]
                for run_idx in range(RUNS_PER_CELL):
                    result = run_single(prompt, tok, tmo, thinking_mode)
                    result.update({
                        "matrix": matrix_name,
                        "thinking_mode": thinking_mode,
                        "max_tokens": tok,
                        "timeout_s": tmo,
                        "prompt_class": pclass,
                        "run_idx": run_idx + 1,
                    })
                    done += 1
                    yield result
                    # Brief pause to avoid overwhelming slow inference
                    time.sleep(0.5)


def main():
    all_results = []

    print(f"# ZB-025B Diagnostic Matrix — {MODEL}", file=sys.stderr)
    print(f"# Ollama endpoint: {OLLAMA_URL}", file=sys.stderr)
    print(f"# Runs per cell: {RUNS_PER_CELL}", file=sys.stderr)
    print(f"# Started: {time.strftime('%Y-%m-%d %H:%M:%S')}", file=sys.stderr)
    print(file=sys.stderr)

    # Estimate total runs
    total_a = len(MATRIX_A_TOKENS) * len(MATRIX_A_TIMEOUTS) * len(PROMPTS) * RUNS_PER_CELL
    total_b = len(MATRIX_B_TOKENS) * len(MATRIX_B_TIMEOUTS) * len(PROMPTS) * RUNS_PER_CELL
    total_c = len(MATRIX_C_TOKENS) * len(MATRIX_C_TIMEOUTS) * len(PROMPTS) * RUNS_PER_CELL
    total = total_a + total_b + total_c
    print(f"# Matrix A: {total_a} runs | Matrix B: {total_b} runs | Matrix C: {total_c} runs | Total: {total}", file=sys.stderr)
    print(file=sys.stderr)

    # --- MATRIX A: thinking ON ---
    print("# === MATRIX A: thinking ON ===", file=sys.stderr)
    for result in run_matrix("A", MATRIX_A_TOKENS, MATRIX_A_TIMEOUTS, "on"):
        print(json.dumps(result), flush=True)
        all_results.append(result)

    # --- MATRIX B: thinking OFF ---
    print("# === MATRIX B: thinking OFF (/no_think) ===", file=sys.stderr)
    for result in run_matrix("B", MATRIX_B_TOKENS, MATRIX_B_TIMEOUTS, "off"):
        print(json.dumps(result), flush=True)
        all_results.append(result)

    # --- MATRIX C: thinking constrained (same as ON but lower max_tokens as proxy) ---
    # qwen3.5 doesn't have a separate reasoning budget API; we test with
    # thinking ON but lower max_tokens to see where content starts appearing
    print("# === MATRIX C: thinking ON, constrained budget ===", file=sys.stderr)
    # Use additional lower token counts to see the crossover point
    extra_tokens = [256, 384, 768]
    for result in run_matrix("C", extra_tokens + MATRIX_C_TOKENS, MATRIX_C_TIMEOUTS, "on"):
        print(json.dumps(result), flush=True)
        all_results.append(result)

    # --- SUMMARY ---
    print(file=sys.stderr)
    print("# === SUMMARY ===", file=sys.stderr)

    for matrix_name in ["A", "B", "C"]:
        matrix_results = [r for r in all_results if r["matrix"] == matrix_name]
        if not matrix_results:
            continue
        print(f"\n## Matrix {matrix_name} ({len(matrix_results)} runs)", file=sys.stderr)

        # Classification counts
        from collections import Counter
        classes = Counter(r["classification"] for r in matrix_results)
        print(f"  Classifications:", file=sys.stderr)
        for cls, count in sorted(classes.items(), key=lambda x: -x[1]):
            pct = count / len(matrix_results) * 100
            print(f"    {cls}: {count}/{len(matrix_results)} ({pct:.0f}%)", file=sys.stderr)

        # Latency stats
        latencies = [r["latency_s"] for r in matrix_results]
        print(f"  Latency: min={min(latencies):.1f}s median={sorted(latencies)[len(latencies)//2]:.1f}s max={max(latencies):.1f}s", file=sys.stderr)

        # Content vs thinking lengths
        content_lens = [r["content_len"] for r in matrix_results]
        thinking_lens = [r["thinking_len"] for r in matrix_results]
        empty_content = sum(1 for l in content_lens if l == 0)
        has_thinking = sum(1 for l in thinking_lens if l > 0)
        print(f"  Content: empty={empty_content}/{len(matrix_results)} avg_len={sum(content_lens)/max(len(content_lens),1):.0f}", file=sys.stderr)
        print(f"  Thinking: present={has_thinking}/{len(matrix_results)} avg_len={sum(thinking_lens)/max(len(thinking_lens),1):.0f}", file=sys.stderr)

        # Breakdown by max_tokens
        print(f"\n  By max_tokens:", file=sys.stderr)
        tokens_in_matrix = sorted(set(r["max_tokens"] for r in matrix_results))
        for tok in tokens_in_matrix:
            tok_results = [r for r in matrix_results if r["max_tokens"] == tok]
            tok_classes = Counter(r["classification"] for r in tok_results)
            success = tok_classes.get("success_via_content", 0)
            reasoning = tok_classes.get("reasoning_only", 0)
            total_tok = len(tok_results)
            print(f"    max_tokens={tok}: success={success}/{total_tok} reasoning_only={reasoning}/{total_tok}", file=sys.stderr)

    # R018: Decision gate
    print(file=sys.stderr)
    print("# === R018 DECISION GATE ===", file=sys.stderr)

    matrix_b_results = [r for r in all_results if r["matrix"] == "B"]
    matrix_a_results = [r for r in all_results if r["matrix"] == "A"]

    b_success = sum(1 for r in matrix_b_results if r["classification"] == "success_via_content")
    b_total = len(matrix_b_results)

    a_success = sum(1 for r in matrix_a_results if r["classification"] == "success_via_content")
    a_total = len(matrix_a_results)

    print(f"  Matrix A (thinking ON): {a_success}/{a_total} success via content", file=sys.stderr)
    print(f"  Matrix B (thinking OFF): {b_success}/{b_total} success via content", file=sys.stderr)

    if b_success > a_success:
        print(f"\n  DECISION: GO — thinking OFF materially improves content success rate ({b_success} vs {a_success})", file=sys.stderr)
        print(f"  Root cause is configuration/protocol, not model incapacity.", file=sys.stderr)
    elif b_success == 0:
        print(f"\n  DECISION: NO-GO — model/runtime not viable even with think disabled", file=sys.stderr)
    else:
        print(f"\n  DECISION: PARTIAL — think off helps but model still has issues", file=sys.stderr)

    print(f"\n# Completed: {time.strftime('%Y-%m-%d %H:%M:%S')}", file=sys.stderr)


if __name__ == "__main__":
    main()
