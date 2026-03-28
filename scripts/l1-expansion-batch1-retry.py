#!/usr/bin/env python3
"""
Controlled Expansion Batch 1 — Sequential Retry.

Prior parallel run: 6/20 l1-produced (30%) — L1 server overloaded by parallel requests.
This retry runs SEQUENTIALLY (one L1 call at a time) for the 14 failed tickets.
"""
import json, subprocess, sys, os, time, re
from datetime import datetime

TOKEN = open(os.path.expanduser("~/zen/DONOTASKMOREFORTHISSHIT.txt")).read().strip()
JIRA_URL = "https://zen-mesh.atlassian.net"
JIRA_EMAIL = "zen@kube-zen.io"
L1_ENDPOINT = "http://localhost:56227"
L1_MODEL = "Qwen3.5-0.8B-Q4_K_M.gguf"
EVIDENCE_DIR = os.path.expanduser("~/zen/zen-brain1/docs/05-OPERATIONS/evidence/l1-expansion-batch1-retry")

TRANSITION_IDS = {"In Progress": "31", "Done": "41", "PAUSED": "51", "RETRYING": "61"}
os.makedirs(EVIDENCE_DIR, exist_ok=True)

SYSTEM_PROMPT = """You are a remediation planner for zen-brain1. Produce BOUNDED EDIT PLANS only.
Return ONLY valid JSON: {"edit_description":"...","target_files":["..."],"patch_commands":["sed/echo commands"],"validation_commands":["..."],"expected_outcome":"...","risk_notes":"...","follow_up_type":"none|needs_review"}
Rules: patch_commands must be concrete shell commands. No full file content. Under 500 tokens. JSON only."""

# 14 tickets that failed due to server overload
RETRY_TICKETS = [
    {"jira_key": "ZB-883", "target_files": ["cmd/remediation-worker/main.go"], "task_type": "code_edit",
     "description": "Add logic to apply 'l1-produced' or 'supervisor-written' label to Jira update comment based on attribution fields.",
     "validation": "grep -q 'produced_by' cmd/remediation-worker/main.go"},
    {"jira_key": "ZB-884", "target_files": ["docs/05-OPERATIONS/evidence/evidence-pack-template.json"], "task_type": "config_change",
     "description": "Add a version field and schema_version field to all evidence pack JSON templates.",
     "validation": "python3 -c 'import json; json.load(open(\"docs/05-OPERATIONS/evidence/evidence-pack-template.json\"))'"},
    {"jira_key": "ZB-885", "target_files": [".gitignore"], "task_type": "config_change",
     "description": "Add *.egg-info and __pycache__ entries to .gitignore.",
     "validation": "grep -q 'egg-info\\|pycache' .gitignore"},
    {"jira_key": "ZB-886", "target_files": ["Makefile"], "task_type": "config_change",
     "description": "Add Makefile target 'validate' that runs python3 -m py_compile on all Python scripts.",
     "validation": "grep -q 'validate' Makefile"},
    {"jira_key": "ZB-887", "target_files": ["scripts/jira-drain.py"], "task_type": "doc_update",
     "description": "Add a comment near the transition IDs dict documenting what each transition does.",
     "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/jira-drain.py\").read())'"},
    {"jira_key": "ZB-888", "target_files": ["scripts/validate.sh"], "task_type": "config_change",
     "description": "Create scripts/validate.sh that runs python3 -m py_compile on all .py files in scripts/.",
     "validation": "test -f scripts/validate.sh && bash -n scripts/validate.sh"},
    {"jira_key": "ZB-889", "target_files": ["GOVERNANCE.md"], "task_type": "doc_update",
     "description": "Add a section to GOVERNANCE.md about L1 attribution tracking requirements.",
     "validation": "test -f GOVERNANCE.md && grep -q 'attribution' GOVERNANCE.md"},
    {"jira_key": "ZB-843", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "description": "Add input validation to exec command to prevent remote code execution. Validate and sanitize the command parameter.",
     "validation": "grep -q 'exec' cmd/main.go"},
    {"jira_key": "ZB-844", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "description": "Add input sanitization to shell command handler to prevent shell injection attacks.",
     "validation": "grep -q 'shell' cmd/main.go"},
    {"jira_key": "ZB-849", "target_files": ["cmd/commands/parse.go"], "task_type": "code_edit",
     "description": "Add handling for --json flag in the parse function so it outputs JSON format.",
     "validation": "grep -q 'json' cmd/commands/parse.go"},
    {"jira_key": "ZB-854", "target_files": ["scripts/backlog-baseline.py"], "task_type": "code_edit",
     "description": "Add try/except around subprocess calls for HTTP error handling.",
     "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/backlog-baseline.py\").read())'"},
    {"jira_key": "ZB-858", "target_files": ["docs/05-OPERATIONS/evidence/evidence-pack-template.json"], "task_type": "doc_update",
     "description": "Add retention_period and created_at metadata fields to evidence pack template.",
     "validation": "python3 -c 'import json; json.load(open(\"docs/05-OPERATIONS/evidence/evidence-pack-template.json\"))'"},
    {"jira_key": "ZB-867", "target_files": ["docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY_v2.md"], "task_type": "doc_update",
     "description": "Create L1_ATTRIBUTION_POLICY_v2.md documenting attribution rules for L1 work.",
     "validation": "test -f docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY_v2.md"},
    {"jira_key": "ZB-813", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "description": "Add input validation for UserAgent parameter to prevent injection attacks.",
     "validation": "grep -q 'UserAgent' cmd/main.go"},
]

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

def call_l1(task):
    prompt = f"""Ticket: {task['jira_key']}
Target: {', '.join(task['target_files'])}
Problem: {task['description']}
Type: {task['task_type']}
Validation: {task['validation']}
Produce a bounded edit plan with sed/echo commands. JSON only."""

    payload = {"model": L1_MODEL, "messages": [
        {"role":"system","content":SYSTEM_PROMPT},
        {"role":"user","content":prompt}
    ], "temperature":0.2, "max_tokens":2048, "chat_template_kwargs":{"enable_thinking":False}}

    start = time.time()
    r = subprocess.run(["curl","-s","--max-time","60","-H","Content-Type: application/json",
        "-d",json.dumps(payload),f"{L1_ENDPOINT}/v1/chat/completions"],
        capture_output=True, text=True, timeout=90)
    elapsed = time.time() - start

    llm_content = ""
    try:
        d = json.loads(r.stdout)
        llm_content = d["choices"][0]["message"]["content"]
    except:
        llm_content = r.stdout

    # Extract JSON
    js = llm_content.strip()
    js = re.sub(r'^```json\s*','',js); js = re.sub(r'^```\s*','',js); js = re.sub(r'\s*```$','',js)
    si, ei = js.find('{'), js.rfind('}')
    if si >= 0 and ei > si: js = js[si:ei+1]

    parsed, err = None, None
    try: parsed = json.loads(js)
    except Exception as e:
        for attempt in [re.sub(r',\s*}','}',js), re.sub(r',\s*]',']',js), js.replace('\n',' ')]:
            try: parsed = json.loads(attempt); break
            except: continue
        if not parsed: err = str(e)

    return {"raw":r.stdout[:2000], "content":llm_content[:2000], "parsed":parsed,
            "parse_error":err, "elapsed":round(elapsed,1)}

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

def main():
    print("=== EXPANSION BATCH 1 — SEQUENTIAL RETRY ===")
    print(f"Tickets: {len(RETRY_TICKETS)}")
    print(f"Mode: SEQUENTIAL (one L1 call at a time)")
    print()

    results = []
    for i, task in enumerate(RETRY_TICKETS):
        key = task["jira_key"]
        print(f"[{i+1}/{len(RETRY_TICKETS)}] {key} ({task['task_type']}) — calling L1...")

        # Move to In Progress
        jira_transition(key, "In Progress")
        time.sleep(0.3)

        # Call L1 — SEQUENTIAL
        l1 = call_l1(task)
        print(f"  L1: {l1['elapsed']}s, parse={'OK' if l1['parsed'] else l1['parse_error'] or 'empty'}")

        # Score
        score, issues = score_output(l1["parsed"], task)
        print(f"  Score: {score}/25, issues={issues}")

        # Save raw
        ts = datetime.now().strftime("%Y%m%d-%H%M%S")
        raw_path = os.path.join(EVIDENCE_DIR, f"{key}_raw.json")
        with open(raw_path, "w") as f:
            json.dump({"jira_key":key, "task":task, "l1":l1, "score":score, "issues":issues, "ts":ts}, f, indent=2)

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
                json.dump({"jira_key":key, "output":l1["parsed"], "score":score, "ts":ts}, f, indent=2)

        # Check forbidden
        if l1["parsed"] and any(f in json.dumps(l1["parsed"]) for f in ["new_content","file_body"]):
            disp, final = "l1-produced-needs-review", "PAUSED"
            pby = "l1-forbidden-fields"
            print(f"  WARNING: forbidden fields detected — downgraded")

        # Update Jira
        jira_comment(key, f"[EXPANSION-RETRY-SEQ] contract=v2-patch | produced_by={pby} | model={L1_MODEL} | elapsed={l1['elapsed']}s | score={score}/25 | disposition={disp} | artifact={raw_path}")
        jira_transition(key, final)
        print(f"  -> {final} ({disp})")

        results.append({"jira_key":key, "task_type":task["task_type"], "l1_elapsed":l1["elapsed"],
            "parsed":bool(l1["parsed"]), "score":score, "issues":issues,
            "has_patches":bool(l1["parsed"] and l1["parsed"].get("patch_commands")),
            "has_validation":bool(l1["parsed"] and l1["parsed"].get("validation_commands")),
            "has_forbidden":bool(l1["parsed"] and any(f in json.dumps(l1["parsed"]) for f in ["new_content","file_body"])),
            "produced_by":pby, "disposition":disp, "final_state":final})
        time.sleep(0.5)

    # Scoreboard
    counts = {}
    for r in results:
        counts[r["disposition"]] = counts.get(r["disposition"],0) + 1
    l1_prod = counts.get("l1-produced",0)
    l1_pct = round(l1_prod / len(results) * 100) if results else 0

    board = {"timestamp":datetime.now().isoformat(),"batch":"expansion-batch1-retry","mode":"sequential",
             "total":len(results),"counts":counts,"l1_produced_pct":l1_pct,"results":results}
    with open(os.path.join(EVIDENCE_DIR, "expansion-retry-scoreboard.json"), "w") as f:
        json.dump(board, f, indent=2)

    print("\n" + "="*80)
    print("=== EXPANSION RETRY SCOREBOARD (SEQUENTIAL) ===")
    print(f"Total: {len(results)} | l1-produced: {l1_prod} ({l1_pct}%)")
    print()
    for r in results:
        print(f"  {r['jira_key']} | {r['task_type']:<16} | {r['l1_elapsed']:>5.1f}s | {'OK' if r['parsed'] else 'FAIL':<5} | {r['score']:>3}/25 | {r['disposition']:<26} | {r['final_state']}")
    print(f"\nCounts: {json.dumps(counts)}")

if __name__ == "__main__":
    main()
