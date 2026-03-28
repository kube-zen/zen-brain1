#!/usr/bin/env python3
"""
Runtime Throughput & Observability — Baseline + Parallelism Experiment.

Phase 1: Capture baseline (sequential, 1 worker)
Phase 2: Step +2 parallel workers (3 workers)
Phase 3: Step +2 more (5 workers)
Each phase runs the same 10-task workload for fair comparison.
"""
import json, subprocess, sys, os, time, re, threading
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed

TOKEN = open(os.path.expanduser("~/zen/DONOTASKMOREFORTHISSHIT.txt")).read().strip()
JIRA_URL = "https://zen-mesh.atlassian.net"
JIRA_EMAIL = "zen@kube-zen.io"
L1_ENDPOINT = "http://localhost:56227"
L1_MODEL = "Qwen3.5-0.8B-Q4_K_M.gguf"
EVIDENCE_DIR = os.path.expanduser("~/zen/zen-brain1/docs/05-OPERATIONS/evidence/runtime-throughput-experiment")
os.makedirs(EVIDENCE_DIR, exist_ok=True)

TRANSITION_IDS = {"In Progress": "31", "Done": "41", "PAUSED": "51", "RETRYING": "61"}

SYSTEM_PROMPT = """You are a remediation planner. Produce BOUNDED EDIT PLANS.
Return ONLY JSON: {"edit_description":"what","target_files":["path"],"patch_commands":["sed/echo commands"],"validation_commands":["verify"],"expected_outcome":"result","risk_notes":"caveats","follow_up_type":"none"}
Use sed/echo for patches. No full file content. JSON only."""

# ─── 10 Standardized Benchmark Tasks ───────────────────────────────────

BENCHMARK_TASKS = [
    {"target_files": [".gitignore"], "task_type": "config_change",
     "description": "Add *.log to .gitignore to prevent log files from being committed to the repository.",
     "validation": "grep -q '*.log' .gitignore"},
    {"target_files": ["Makefile"], "task_type": "config_change",
     "description": "Add a Makefile target named 'lint' that runs go vet ./...",
     "validation": "grep -q 'lint' Makefile"},
    {"target_files": ["scripts/"], "task_type": "config_change",
     "description": "Create scripts/check.sh that runs python3 -m py_compile on all .py files in scripts/.",
     "validation": "test -f scripts/check.sh"},
    {"target_files": ["README.md"], "task_type": "doc_update",
     "description": "Add a one-line description of the project purpose at the top of README.md.",
     "validation": "grep -q 'purpose' README.md || grep -q 'zen-brain' README.md"},
    {"target_files": [".gitignore"], "task_type": "config_change",
     "description": "Add __pycache__/ and *.pyc to .gitignore.",
     "validation": "grep -q 'pycache\\|pyc' .gitignore"},
    {"target_files": ["Makefile"], "task_type": "config_change",
     "description": "Add a Makefile target named 'format' that runs gofmt -l .",
     "validation": "grep -q 'format' Makefile"},
    {"target_files": ["GOVERNANCE.md"], "task_type": "doc_update",
     "description": "Add a section header '## Attribution Tracking' to GOVERNANCE.md.",
     "validation": "grep -q 'Attribution' GOVERNANCE.md 2>/dev/null || echo 'file_missing'"},
    {"target_files": ["scripts/jira-drain.py"], "task_type": "doc_update",
     "description": "Add a single-line comment above the main function describing what the script does.",
     "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/jira-drain.py\").read())'"},
    {"target_files": ["Makefile"], "task_type": "config_change",
     "description": "Add a Makefile target named 'clean' that removes evidence/*.json files.",
     "validation": "grep -q 'clean' Makefile"},
    {"target_files": [".gitignore"], "task_type": "config_change",
     "description": "Add vendor/ directory to .gitignore.",
     "validation": "grep -q 'vendor' .gitignore"},
]

# ─── JSON Repair (from corrective retry) ───────────────────────────────

def repair_truncated_json(raw_str):
    s = raw_str.strip()
    s = re.sub(r'^```json\s*','',s); s = re.sub(r'^```\s*','',s); s = re.sub(r'\s*```$','',s)
    try: json.loads(s); return s, "no_repair"
    except: pass
    start = s.find('{')
    if start < 0: return None, "no_json"
    end = s.rfind('}')
    if end > start:
        try:
            return json.loads(s[start:end+1]), "bracket_trim"
        except:
            pass
    truncated = s[start:]
    last_comma = -1; depth = 0; in_str = False; esc = False
    for i, ch in enumerate(truncated):
        if esc: esc = False; continue
        if ch == '\\' and in_str: esc = True; continue
        if ch == '"': in_str = not in_str; continue
        if in_str: continue
        if ch in '{[': depth += 1
        elif ch in '}]': depth -= 1
        elif ch == ',' and depth == 1: last_comma = i
    if last_comma > 0:
        repaired = truncated[:last_comma]
        ob = repaired.count('{') - repaired.count('}')
        obk = repaired.count('[') - repaired.count(']')
        if ob > 0: repaired += '}' * ob
        if obk > 0: repaired += ']' * obk
        try:
            return json.loads(repaired), "truncation_repaired"
        except:
            pass
    return None, "unrepairable"

# ─── L1 Call with Full Telemetry ──────────────────────────────────────

lock = threading.Lock()
call_counter = 0

def call_l1(task, task_id):
    global call_counter
    with lock:
        call_counter += 1
        my_id = call_counter

    prompt = f"Ticket: BENCH-{my_id}\nTarget: {', '.join(task['target_files'])}\nProblem: {task['description']}\nType: {task['task_type']}\nProduce bounded edit plan. JSON only."
    payload = {"model": L1_MODEL, "messages": [
        {"role":"system","content":SYSTEM_PROMPT},{"role":"user","content":prompt}
    ], "temperature":0.2, "max_tokens":2048, "chat_template_kwargs":{"enable_thinking":False}}
    prompt_size = len(json.dumps(payload))

    start = time.time()
    r = subprocess.run(["curl","-s","--max-time","180","-H","Content-Type: application/json",
        "-d",json.dumps(payload),f"{L1_ENDPOINT}/v1/chat/completions"],
        capture_output=True, text=True, timeout=210)
    elapsed = time.time() - start
    raw = r.stdout; raw_size = len(raw)

    llm_content = ""
    try: llm_content = json.loads(raw)["choices"][0]["message"]["content"]
    except: llm_content = raw

    repaired, repair_type = repair_truncated_json(llm_content)
    parsed = None
    if repaired:
        try: parsed = json.loads(repaired)
        except: pass
    if not parsed:
        js = llm_content.strip()
        js = re.sub(r'^```json\s*','',js); js = re.sub(r'^```\s*','',js); js = re.sub(r'\s*```$','',js)
        si, ei = js.find('{'), js.rfind('}')
        if si >= 0 and ei > si:
            try:
                parsed = json.loads(js[si:ei+1])
            except:
                for a in [re.sub(r',\s*}','}',js[si:ei+1])]:
                    try:
                        parsed = json.loads(a)
                        break
                    except:
                        continue

    # Classify
    if not llm_content.strip(): cls = "no-output"
    elif not parsed:
        cls = "truncated-repaired" if repair_type not in ["no_repair","no_json","unrepairable"] else "parse-fail"
    elif elapsed > 60: cls = "slow-but-productive"
    elif repair_type != "no_repair": cls = "truncated-repaired"
    else: cls = "fast-productive"

    # Score
    score = 0
    if parsed:
        tf = str(parsed.get("target_files",[])) + str(parsed.get("patch_commands",[]))
        score += 5 if any(t in tf for t in task["target_files"]) else 2
        score += 5 if len(parsed.get("edit_description","")) > 20 else (3 if len(parsed.get("edit_description","")) > 5 else 0)
        p = parsed.get("patch_commands",[])
        score += 5 if any(any(c in str(x) for c in ["sed","echo","awk","cat"]) for x in (p if isinstance(p,list) else [p])) else (3 if p else 0)
        v = parsed.get("validation_commands",[])
        score += 5 if v else 0
        score += 0 if any(f in json.dumps(parsed) for f in ["new_content","file_body"]) else 5

    disposition = "l1-produced" if score >= 15 else ("l1-produced-needs-review" if parsed else "l1-failed-parse")

    return {
        "task_id": task_id, "task_type": task["task_type"],
        "model": L1_MODEL, "lane": "l1",
        "prompt_size": prompt_size, "output_size": raw_size,
        "start_time": start, "end_time": time.time(), "wall_time": round(elapsed, 1),
        "completion_class": cls, "repair_type": repair_type,
        "quality_score": score,
        "produced_by": "l1" if score >= 15 else ("l1-partial" if parsed else "l1-failed"),
        "parsed": bool(parsed),
    }

# ─── Run Phase (baseline or step) ─────────────────────────────────────

def run_phase(name, max_workers):
    global call_counter
    call_counter = 0

    print(f"\n{'='*70}")
    print(f"=== PHASE: {name} (max_workers={max_workers}) ===")
    print(f"{'='*70}")

    phase_start = time.time()
    results = []

    if max_workers == 1:
        # Sequential
        for i, task in enumerate(BENCHMARK_TASKS):
            t_start = time.time()
            r = call_l1(task, i+1)
            t_end = time.time()
            r["phase_start"] = phase_start
            r["task_start"] = t_start
            r["task_end"] = t_end
            results.append(r)
            print(f"  [{i+1}/10] {r['task_type']:<16} | {r['wall_time']:>5.1f}s | {r['completion_class']:<24} | score={r['quality_score']}/25 | {r['produced_by']}")
    else:
        # Parallel
        with ThreadPoolExecutor(max_workers=max_workers) as pool:
            futures = {}
            for i, task in enumerate(BENCHMARK_TASKS):
                t_start = time.time()
                futures[pool.submit(call_l1, task, i+1)] = t_start
            for future in as_completed(futures):
                r = future.result()
                r["phase_start"] = phase_start
                r["task_start"] = futures[future]
                r["task_end"] = time.time()
                results.append(r)
                print(f"  [{r['task_id']}/10] {r['task_type']:<16} | {r['wall_time']:>5.1f}s | {r['completion_class']:<24} | score={r['quality_score']}/25 | {r['produced_by']}")

    phase_end = time.time()
    phase_elapsed = round(phase_end - phase_start, 1)

    # Compute metrics
    wall_times = [r["wall_time"] for r in results]
    l1_prod = sum(1 for r in results if r["produced_by"] == "l1")
    timeouts = sum(1 for r in results if r["completion_class"] == "no-output")
    truncs = sum(1 for r in results if "truncated" in r["completion_class"])
    slow_prod = sum(1 for r in results if r["completion_class"] == "slow-but-productive")
    fast_prod = sum(1 for r in results if r["completion_class"] == "fast-productive")

    metrics = {
        "phase": name, "max_workers": max_workers,
        "phase_elapsed_sec": phase_elapsed,
        "total_tasks": len(results),
        "throughput_tasks_per_min": round(len(results) / (phase_elapsed/60), 2) if phase_elapsed > 0 else 0,
        "l1_produced": l1_prod, "l1_produced_pct": round(l1_prod/len(results)*100) if results else 0,
        "timeout_count": timeouts, "truncation_count": truncs,
        "slow_but_productive": slow_prod, "fast_productive": fast_prod,
        "avg_wall_time": round(sum(wall_times)/len(wall_times), 1) if wall_times else 0,
        "p50_wall_time": round(sorted(wall_times)[len(wall_times)//2], 1) if wall_times else 0,
        "p95_wall_time": round(sorted(wall_times)[int(len(wall_times)*0.95)] if len(wall_times) > 1 else wall_times[-1], 1) if wall_times else 0,
        "max_wall_time": round(max(wall_times), 1) if wall_times else 0,
        "min_wall_time": round(min(wall_times), 1) if wall_times else 0,
        "avg_prompt_size": round(sum(r["prompt_size"] for r in results)/len(results)) if results else 0,
        "avg_output_size": round(sum(r["output_size"] for r in results)/len(results)) if results else 0,
        "chars_per_sec": round(sum(r["output_size"] for r in results) / sum(wall_times), 1) if wall_times and sum(wall_times) > 0 else 0,
        "completion_classes": {},
        "results": results,
    }
    for r in results:
        cls = r["completion_class"]
        metrics["completion_classes"][cls] = metrics["completion_classes"].get(cls, 0) + 1

    return metrics

# ─── Main ──────────────────────────────────────────────────────────────

def main():
    all_phases = []

    # Phase 1: Baseline (sequential)
    m1 = run_phase("baseline-sequential", max_workers=1)
    all_phases.append(m1)
    time.sleep(5)

    # Phase 2: +2 parallel (3 workers)
    m2 = run_phase("step-2-parallel", max_workers=3)
    all_phases.append(m2)
    time.sleep(5)

    # Phase 3: +2 more (5 workers)
    m3 = run_phase("step-4-parallel", max_workers=5)
    all_phases.append(m3)
    time.sleep(5)

    # Phase 4: +2 more (7 workers)
    m4 = run_phase("step-6-parallel", max_workers=7)
    all_phases.append(m4)

    # ─── Comparison Table ──────────────────────────────────────────
    print("\n" + "="*80)
    print("=== PARALLELISM EXPERIMENT — COMPARISON ===")
    print("="*80)
    print()
    print(f"{'Phase':<25} {'Workers':>8} {'Total':>6} {'Elapsed':>8} {'T/min':>6} {'L1%':>5} {'AvgT':>6} {'P50T':>6} {'P95T':>6} {'MaxT':>6} {'Timeout':>8} {'Trunc':>6} {'SlowProd':>9} {'FastProd':>9}")
    print("-"*140)
    for m in all_phases:
        print(f"{m['phase']:<25} {m['max_workers']:>8} {m['total_tasks']:>6} {m['phase_elapsed_sec']:>7.1f}s {m['throughput_tasks_per_min']:>5.1f} {m['l1_produced_pct']:>4}% {m['avg_wall_time']:>5.1f}s {m['p50_wall_time']:>5.1f}s {m['p95_wall_time']:>5.1f}s {m['max_wall_time']:>5.1f}s {m['timeout_count']:>8} {m['truncation_count']:>6} {m['slow_but_productive']:>9} {m['fast_productive']:>9}")

    # ─── Recommendation ───────────────────────────────────────────
    # Find the phase with best throughput while maintaining >=80% l1-produced
    best = None
    for m in all_phases:
        if m["l1_produced_pct"] >= 80:
            if best is None or m["throughput_tasks_per_min"] > best["throughput_tasks_per_min"]:
                best = m

    if best:
        rec_workers = best["max_workers"]
        rec_throughput = best["throughput_tasks_per_min"]
        rec_rate = best["l1_produced_pct"]
    else:
        rec_workers = 1
        rec_throughput = all_phases[0]["throughput_tasks_per_min"]
        rec_rate = all_phases[0]["l1_produced_pct"]

    print()
    print("=== RECOMMENDATION ===")
    print(f"  Recommended parallel workers: {rec_workers}")
    print(f"  Expected throughput: {rec_throughput} tasks/min")
    print(f"  Expected L1-produced rate: {rec_rate}%")

    # Check for degradation
    baseline_rate = all_phases[0]["l1_produced_pct"]
    for m in all_phases[1:]:
        if m["l1_produced_pct"] < baseline_rate - 20:
            print(f"  WARNING: Quality degraded at {m['max_workers']} workers ({m['l1_produced_pct']}% vs baseline {baseline_rate}%)")
        if m["timeout_count"] > all_phases[0]["timeout_count"] * 2:
            print(f"  WARNING: Timeout rate increased at {m['max_workers']} workers")

    # ─── Save ─────────────────────────────────────────────────────
    experiment = {
        "timestamp": datetime.now().isoformat(),
        "model": L1_MODEL,
        "machine": "i9-13900H 20-core, 64GB",
        "phases": all_phases,
        "recommendation": {
            "workers": rec_workers,
            "throughput": rec_throughput,
            "l1_produced_rate": rec_rate,
            "rationale": f"Best throughput ({rec_throughput} tasks/min) at {rec_rate}% l1-produced rate with {rec_workers} parallel workers."
        }
    }
    out_path = os.path.join(EVIDENCE_DIR, "parallelism-experiment.json")
    with open(out_path, "w") as f:
        json.dump(experiment, f, indent=2)
    print(f"\nSaved: {out_path}")

if __name__ == "__main__":
    main()
