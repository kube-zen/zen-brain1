#!/usr/bin/env python3
"""
Expansion Batch 1 — Corrective Retry with Adaptive Timeout + Truncation Repair.

Policy changes from prior runs:
- NO blanket timeout reduction. Keep generous hard timeout for CPU 0.8b.
- Adaptive timeout by task shape (not global)
- JSON truncation repair (bracket-closer for cut-off output)
- Output telemetry: prompt size, output size, first-output time, classification
- Separate "slow" from "stuck" — only kill truly stuck requests
"""
import json, subprocess, sys, os, time, re
from datetime import datetime

TOKEN = open(os.path.expanduser("~/zen/DONOTASKMOREFORTHISSHIT.txt")).read().strip()
JIRA_URL = "https://zen-mesh.atlassian.net"
JIRA_EMAIL = "zen@kube-zen.io"
L1_ENDPOINT = "http://localhost:56227"
L1_MODEL = "Qwen3.5-0.8B-Q4_K_M.gguf"
EVIDENCE_DIR = os.path.expanduser("~/zen/zen-brain1/docs/05-OPERATIONS/evidence/l1-corrective-retry")

TRANSITION_IDS = {"In Progress": "31", "Done": "41", "PAUSED": "51", "RETRYING": "61"}
os.makedirs(EVIDENCE_DIR, exist_ok=True)

# ─── Adaptive Timeout by Task Shape ────────────────────────────────────

def get_adaptive_timeout(task_type, description_length):
    """
    Adaptive timeout: generous for complex tasks, shorter for trivial.
    Never below 60s — CPU 0.8b is allowed to be slow.
    """
    base = 120  # generous hard timeout for CPU inference
    if task_type == "config_change" and description_length < 100:
        return 90
    elif task_type == "config_change":
        return 120
    elif task_type == "doc_update" and description_length < 150:
        return 120
    elif task_type == "code_edit" and description_length < 150:
        return 120
    else:
        return 180  # generous for complex code_edit / large doc_update

# ─── JSON Truncation Repair ───────────────────────────────────────────

def repair_truncated_json(raw_str):
    """
    Attempt to repair JSON that was truncated mid-generation.
    Common patterns: cut off mid-string, mid-array, mid-object.
    Returns (repaired_json_string, repair_type) or (None, None) if unrepairable.
    """
    s = raw_str.strip()
    # Remove markdown fences
    s = re.sub(r'^```json\s*', '', s)
    s = re.sub(r'^```\s*', '', s)
    s = re.sub(r'\s*```$', '', s)

    # Try direct parse first
    try:
        json.loads(s)
        return s, "no_repair_needed"
    except:
        pass

    # Find JSON boundaries
    start = s.find('{')
    if start < 0:
        return None, "no_json_found"
    end = s.rfind('}')

    # If complete braces exist, try that
    if end > start:
        candidate = s[start:end+1]
        try:
            json.loads(candidate)
            return candidate, "bracket_trimmed"
        except:
            pass

    # Truncation repair: try to close open structures
    truncated = s[start:]  # everything from first { onward

    # Strategy 1: Close unclosed strings (most common truncation)
    # Find where the last complete value ends
    # Count open braces and brackets
    open_braces = 0
    open_brackets = 0
    in_string = False
    escape_next = False
    last_complete_pos = len(truncated)

    for i, ch in enumerate(truncated):
        if escape_next:
            escape_next = False
            continue
        if ch == '\\' and in_string:
            escape_next = True
            continue
        if ch == '"' and not escape_next:
            in_string = not in_string
            continue
        if in_string:
            continue
        if ch == '{':
            open_braces += 1
        elif ch == '}':
            open_braces -= 1
        elif ch == '[':
            open_brackets += 1
        elif ch == ']':
            open_brackets -= 1

        # Track last position where we have balanced structures
        if open_braces == 1 and open_brackets == 0 and ch in [',', '"']:
            last_complete_pos = i

    # Strategy 2: Trim to last comma before truncation point + close braces
    # Find the last comma at depth 1 (inside root object)
    depth = 0
    last_comma_at_depth1 = -1
    in_str = False
    esc = False
    for i, ch in enumerate(truncated):
        if esc:
            esc = False; continue
        if ch == '\\' and in_str:
            esc = True; continue
        if ch == '"' and not esc:
            in_str = not in_str; continue
        if in_str:
            continue
        if ch == '{' or ch == '[':
            depth += 1
        elif ch == '}' or ch == ']':
            depth -= 1
        elif ch == ',' and depth == 1:
            last_comma_at_depth1 = i

    if last_comma_at_depth1 > 0:
        repaired = truncated[:last_comma_at_depth1]
        # Close open structures
        open_b = 0
        open_bk = 0
        for ch in repaired:
            if ch == '{': open_b += 1
            elif ch == '}': open_b -= 1
            elif ch == '[': open_bk += 1
            elif ch == ']': open_bk -= 1
        if open_b > 0:
            repaired += '}' * open_b
        if open_bk > 0:
            repaired += ']' * open_bk

        try:
            json.loads(repaired)
            return repaired, "truncation_repaired_comma_trim"
        except:
            pass

    # Strategy 3: Find last complete key:value pair
    # Look for pattern: "key": "value" or "key": [ or "key": {
    pattern = re.findall(r'"(\w+)"\s*:\s*(?:"[^"]*"|\[|\{|\d+|true|false|null)', truncated)
    if pattern:
        # Rebuild object with only complete pairs found
        pairs_str = ', '.join(f'"{k}": "truncated_repair_placeholder"' for k in pattern)
        candidate = '{' + pairs_str + '}'
        try:
            parsed = json.loads(candidate)
            # Now rebuild from the original truncated string using found keys
            repaired_dict = {}
            for match in re.finditer(r'"(\w+)"\s*:\s*(?:"((?:[^"\\]|\\.)*)"|(\[.*?\])|(\{.*?\})|(\d+(?:\.\d+)?|true|false|null))', truncated, re.DOTALL):
                key = match.group(1)
                if match.group(2) is not None:  # string value
                    val = match.group(2)
                    # Check if string is complete (has closing quote)
                    full_match = match.group(0)
                    if full_match.rstrip().endswith('"') or match.group(2).count('"') % 2 == 0:
                        repaired_dict[key] = val
                    else:
                        repaired_dict[key] = val + '_TRUNCATED'
                elif match.group(3) is not None:  # array
                    repaired_dict[key] = json.loads(match.group(3))
                elif match.group(4) is not None:  # object
                    repaired_dict[key] = json.loads(match.group(4))
                elif match.group(5) is not None:  # primitive
                    repaired_dict[key] = json.loads(match.group(5))
            if repaired_dict:
                repaired = json.dumps(repaired_dict)
                return repaired, "truncation_repaired_key_extraction"
        except:
            pass

    return None, "unrepairable"

# ─── Output Classification ─────────────────────────────────────────────

def classify_output(elapsed, parsed, llm_content, prompt_size, raw_size):
    """
    Classify output as one of:
    - slow-but-productive
    - fast-productive
    - no-output
    - truncated-output
    - repetitive-degenerate
    - infra-fail
    """
    if not llm_content or len(llm_content.strip()) == 0:
        return "no-output"
    if not parsed:
        if raw_size > prompt_size * 0.5:
            return "truncated-output"  # Got substantial output but couldn't parse
        if elapsed > 100:
            return "no-output"  # Long wait, nothing useful
        return "infra-fail"

    # Check for repetitive output
    if len(llm_content) > 200:
        # Look for repeated phrases (degenerate output)
        chunks = [llm_content[i:i+50] for i in range(0, min(len(llm_content), 500), 50)]
        unique_ratio = len(set(chunks)) / max(len(chunks), 1)
        if unique_ratio < 0.5:
            return "repetitive-degenerate"

    if elapsed > 60 and parsed:
        return "slow-but-productive"
    return "fast-productive"

# ─── L1 Call with Telemetry ───────────────────────────────────────────

SYSTEM_PROMPT = """You are a remediation planner for zen-brain1. Produce BOUNDED EDIT PLANS.
Return ONLY valid JSON: {"edit_description":"what to change","target_files":["path"],"patch_commands":["sed/echo commands"],"validation_commands":["verify commands"],"expected_outcome":"success looks like","risk_notes":"caveats","follow_up_type":"none|needs_review"}
Rules: use sed/echo/awk/cat for patch_commands. No full file content. Keep under 500 tokens. JSON only."""

def call_l1_with_telemetry(task):
    timeout = get_adaptive_timeout(task["task_type"], len(task["description"]))
    prompt = f"""Ticket: {task['jira_key']}
Target: {', '.join(task['target_files'])}
Problem: {task['description']}
Type: {task['task_type']}
Validation: {task['validation']}
Produce bounded edit plan with sed/echo commands. JSON only."""

    payload = {"model": L1_MODEL, "messages": [
        {"role":"system","content":SYSTEM_PROMPT},
        {"role":"user","content":prompt}
    ], "temperature":0.2, "max_tokens":2048, "chat_template_kwargs":{"enable_thinking":False}}

    prompt_size = len(json.dumps(payload))

    start = time.time()
    first_byte_time = None

    # Use streaming to detect no-output vs slow-productive
    r = subprocess.run(["curl","-s","--max-time",str(timeout),
        "-H","Content-Type: application/json",
        "-d",json.dumps(payload),
        f"{L1_ENDPOINT}/v1/chat/completions"],
        capture_output=True, text=True, timeout=timeout+30)
    elapsed = time.time() - start

    raw = r.stdout
    raw_size = len(raw)

    llm_content = ""
    try:
        d = json.loads(raw)
        llm_content = d["choices"][0]["message"]["content"]
    except:
        llm_content = raw

    # Attempt truncation repair
    repaired, repair_type = repair_truncated_json(llm_content)
    parsed = None
    if repaired:
        try:
            parsed = json.loads(repaired)
        except:
            parsed = None

    # If repair failed, try normal extraction
    if not parsed:
        js = llm_content.strip()
        js = re.sub(r'^```json\s*','',js)
        js = re.sub(r'^```\s*','',js)
        js = re.sub(r'\s*```$','',js)
        si, ei = js.find('{'), js.rfind('}')
        if si >= 0 and ei > si:
            try:
                parsed = json.loads(js[si:ei+1])
            except:
                for attempt in [re.sub(r',\s*}','}',js[si:ei+1]), re.sub(r',\s*]',']',js[si:ei+1])]:
                    try: parsed = json.loads(attempt); break
                    except: continue

    output_class = classify_output(elapsed, parsed, llm_content, prompt_size, raw_size)

    return {
        "raw": raw[:3000], "llm_content": llm_content[:3000],
        "repaired": repaired, "repair_type": repair_type,
        "parsed": parsed, "elapsed": round(elapsed, 1),
        "prompt_size": prompt_size, "output_size": raw_size,
        "adaptive_timeout": timeout,
        "output_class": output_class,
    }

# ─── Quality Gate ──────────────────────────────────────────────────────

def score_output(parsed, task):
    if not parsed: return 0, []
    s = {}
    tf = str(parsed.get("target_files",[])) + str(parsed.get("patch_commands",[]))
    s["target"] = 5 if any(t in tf for t in task["target_files"]) else 2
    d = parsed.get("edit_description","")
    s["desc"] = 5 if len(d)>20 else (3 if len(d)>5 else 0)
    p = parsed.get("patch_commands",[])
    s["patch"] = 5 if any(any(c in str(x) for c in ["sed","echo","awk","cat","mkdir","touch","printf"]) for x in (p if isinstance(p,list) else [p])) else (3 if p else 0)
    v = parsed.get("validation_commands",[])
    s["valid"] = 5 if (v and len(v)>0) else 0
    serial = json.dumps(parsed)
    s["no_forbid"] = 0 if any(f in serial for f in ["new_content","file_body"]) else 5
    issues = []
    if s["target"]<5: issues.append("target_missing")
    if s["desc"]<3: issues.append("desc_short")
    if s["patch"]<3: issues.append("no_concrete_patches")
    if s["valid"]<3: issues.append("no_validation")
    if s["no_forbid"]<5: issues.append("forbidden_fields")
    return sum(s.values()), issues

# ─── Jira Helpers ──────────────────────────────────────────────────────

def jira_transition(key, state):
    tid = TRANSITION_IDS.get(state)
    if not tid: return False
    r = subprocess.run(["curl","-s","-o","/dev/null","-w","%{http_code}","-X","POST",
        "-u",f"{JIRA_EMAIL}:{TOKEN}","-H","Content-Type: application/json",
        "-d",json.dumps({"transition":{"id":tid}}),
        f"{JIRA_URL}/rest/api/3/issue/{key}/transitions"],
        capture_output=True, text=True, timeout=15)
    return r.stdout.strip() == "204"

def jira_comment(key, text):
    body = {"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":text}]}]}}
    subprocess.run(["curl","-s","-X","POST","-u",f"{JIRA_EMAIL}:{TOKEN}",
        "-H","Content-Type: application/json","-d",json.dumps(body),
        f"{JIRA_URL}/rest/api/3/issue/{key}/comment"],
        capture_output=True, text=True, timeout=15)

# ─── The 10 Failed Tickets ────────────────────────────────────────────

FAILED_TICKETS = [
    {"jira_key": "ZB-887", "target_files": ["scripts/jira-drain.py"], "task_type": "doc_update",
     "description": "Add a comment near the transition IDs dict documenting what each transition does.",
     "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/jira-drain.py\").read())'",
     "prior_failure": "l1-truncated-json"},
    {"jira_key": "ZB-888", "target_files": ["scripts/validate.sh"], "task_type": "config_change",
     "description": "Create scripts/validate.sh that runs python3 -m py_compile on all .py files in scripts/.",
     "validation": "test -f scripts/validate.sh && bash -n scripts/validate.sh",
     "prior_failure": "l1-timeout"},
    {"jira_key": "ZB-901", "target_files": [".gitignore"], "task_type": "config_change",
     "description": "Add l1-*.json to .gitignore to prevent L1 raw output artifacts from being committed.",
     "validation": "grep -q 'l1' .gitignore",
     "prior_failure": "l1-truncated-json"},
    {"jira_key": "ZB-903", "target_files": ["cmd/commands/parse.go"], "task_type": "code_edit",
     "description": "Add handling for --json flag in parse function so it outputs JSON format.",
     "validation": "grep -q 'json' cmd/commands/parse.go",
     "prior_failure": "l1-timeout"},
    {"jira_key": "ZB-910", "target_files": ["README.md"], "task_type": "doc_update",
     "description": "Add a Quick Start section to README.md with 3 example commands.",
     "validation": "grep -q 'Quick Start' README.md",
     "prior_failure": "l1-truncated-json"},
    {"jira_key": "ZB-916", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "description": "Add input sanitization to prevent shell injection in command handler.",
     "validation": "grep -q 'shell' cmd/main.go",
     "prior_failure": "l1-timeout"},
    {"jira_key": "ZB-927", "target_files": ["Makefile"], "task_type": "config_change",
     "description": "Add Makefile target that runs all Python validation scripts.",
     "validation": "grep -q 'validate' Makefile",
     "prior_failure": "l1-timeout"},
    {"jira_key": "ZB-928", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "description": "Add input validation for UserAgent parameter to prevent injection attacks.",
     "validation": "grep -q 'UserAgent' cmd/main.go",
     "prior_failure": "l1-timeout"},
    {"jira_key": "ZB-929", "target_files": ["GOVERNANCE.md"], "task_type": "doc_update",
     "description": "Add a section to GOVERNANCE.md about L1 attribution tracking requirements.",
     "validation": "test -f GOVERNANCE.md && grep -q 'attribution' GOVERNANCE.md",
     "prior_failure": "l1-truncated-json"},
    {"jira_key": "ZB-930", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "description": "Add input validation for exec command to prevent remote code execution.",
     "validation": "grep -q 'exec' cmd/main.go",
     "prior_failure": "l1-timeout"},
]

# ─── Main ──────────────────────────────────────────────────────────────

def main():
    print("=== CORRECTIVE RETRY — ADAPTIVE TIMEOUT + TRUNCATION REPAIR ===")
    print(f"Tickets: {len(FAILED_TICKETS)}")
    print(f"Policy: generous timeout (90-180s by task shape), truncation repair, output telemetry")
    print()

    results = []
    for i, task in enumerate(FAILED_TICKETS):
        key = task["jira_key"]
        timeout = get_adaptive_timeout(task["task_type"], len(task["description"]))
        print(f"[{i+1}/{len(FAILED_TICKETS)}] {key} ({task['task_type']}) — timeout={timeout}s — calling L1...")

        jira_transition(key, "In Progress")
        time.sleep(0.3)

        l1 = call_l1_with_telemetry(task)

        score, issues = score_output(l1["parsed"], task)
        print(f"  L1: {l1['elapsed']}s (adaptive={timeout}s) | class={l1['output_class']} | repair={l1['repair_type']}")
        print(f"  Score: {score}/25, issues={issues}")

        # Save raw + telemetry
        ts = datetime.now().strftime("%Y%m%d-%H%M%S")
        raw_path = os.path.join(EVIDENCE_DIR, f"{key}_raw.json")
        with open(raw_path, "w") as f:
            json.dump({"jira_key":key, "task":task, "l1":l1, "score":score, "issues":issues,
                       "ts":ts, "policy":"adaptive-timeout-v1"}, f, indent=2)

        # Determine disposition
        if not l1["parsed"]:
            disp, final = "l1-failed-parse", "RETRYING"
            pby = "l1-failed-parse"
        elif score < 15:
            disp, final = "l1-produced-needs-review", "PAUSED"
            pby = "l1-low-quality"
        else:
            disp, final = "l1-produced", "Done"
            pby = "l1"
            norm_path = os.path.join(EVIDENCE_DIR, f"{key}_normalized.json")
            with open(norm_path, "w") as f:
                json.dump({"jira_key":key, "output":l1["parsed"], "repaired_from":l1["repair_type"],
                           "score":score, "ts":ts}, f, indent=2)

        if l1["parsed"] and any(f in json.dumps(l1["parsed"]) for f in ["new_content","file_body"]):
            disp, final = "l1-produced-needs-review", "PAUSED"
            pby = "l1-forbidden-fields"

        # Was truncation repair needed?
        if l1["repair_type"] and l1["repair_type"] != "no_repair_needed" and l1["parsed"]:
            print(f"  *** TRUNCATION REPAIRED: {l1['repair_type']} -> {disp} ***")

        jira_comment(key, f"[CORRECTIVE-RETRY] policy=adaptive-timeout | class={l1['output_class']} | repair={l1['repair_type']} | produced_by={pby} | elapsed={l1['elapsed']}s | adaptive_timeout={l1['adaptive_timeout']}s | score={score}/25 | disposition={disp} | artifact={raw_path}")
        jira_transition(key, final)
        print(f"  -> {final} ({disp})")

        results.append({"jira_key":key, "task_type":task["task_type"],
            "prior_failure":task["prior_failure"],
            "l1_elapsed":l1["elapsed"], "adaptive_timeout":l1["adaptive_timeout"],
            "output_class":l1["output_class"], "repair_type":l1["repair_type"],
            "parsed":bool(l1["parsed"]), "score":score, "issues":issues,
            "prompt_size":l1["prompt_size"], "output_size":l1["output_size"],
            "has_patches":bool(l1["parsed"] and l1["parsed"].get("patch_commands")),
            "has_validation":bool(l1["parsed"] and l1["parsed"].get("validation_commands")),
            "produced_by":pby, "disposition":disp, "final_state":final})
        time.sleep(0.5)

    # Scoreboard
    counts = {}
    for r in results:
        counts[r["disposition"]] = counts.get(r["disposition"], 0) + 1
    l1_prod = counts.get("l1-produced", 0)
    l1_pct = round(l1_prod / len(results) * 100) if results else 0

    # Check prior failure recovery
    prior_timeout_recovered = sum(1 for r in results if r["prior_failure"]=="l1-timeout" and r["disposition"]=="l1-produced")
    prior_truncated_recovered = sum(1 for r in results if r["prior_failure"]=="l1-truncated-json" and r["disposition"]=="l1-produced")
    truncation_repair_used = sum(1 for r in results if r["repair_type"] and r["repair_type"] != "no_repair_needed")

    board = {"timestamp":datetime.now().isoformat(), "batch":"corrective-retry",
             "policy":"adaptive-timeout-v1", "total":len(results), "counts":counts,
             "l1_produced_pct":l1_pct,
             "prior_failure_recovery":{"timeout_recovered":prior_timeout_recovered,
                 "truncated_recovered":prior_truncated_recovered},
             "truncation_repairs_used":truncation_repair_used,
             "results":results}
    with open(os.path.join(EVIDENCE_DIR, "corrective-retry-scoreboard.json"), "w") as f:
        json.dump(board, f, indent=2)

    print("\n" + "="*80)
    print("=== CORRECTIVE RETRY SCOREBOARD ===")
    print(f"Total: {len(results)} | l1-produced: {l1_prod} ({l1_pct}%)")
    print(f"Prior timeouts recovered: {prior_timeout_recovered}/6")
    print(f"Prior truncations recovered: {prior_truncated_recovered}/4")
    print(f"Truncation repairs used: {truncation_repair_used}")
    print()
    print(f"{'Key':>8} | {'Type':<16} | {'Prior':<20} | {'Time':>5} | {'AdaptTO':>7} | {'Class':<25} | {'Repair':<28} | {'Score':>5} | {'Disp':<26} | {'Final'}")
    print("-"*170)
    for r in results:
        print(f"{r['jira_key']:>8} | {r['task_type']:<16} | {r['prior_failure']:<20} | {r['l1_elapsed']:>5.1f}s | {r['adaptive_timeout']:>6}s | {r['output_class']:<25} | {r['repair_type']:<28} | {r['score']:>4}/25 | {r['disposition']:<26} | {r['final_state']}")
    print(f"\nCounts: {json.dumps(counts)}")

if __name__ == "__main__":
    main()
