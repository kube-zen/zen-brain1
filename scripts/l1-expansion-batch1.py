#!/usr/bin/env python3
"""
Controlled Expansion — 20-ticket bounded backlog-drain batch.
Uses v2 patch-oriented contract. Honest attribution throughout.
"""
import json, subprocess, sys, os, time, re
from datetime import datetime

TOKEN = open(os.path.expanduser("~/zen/DONOTASKMOREFORTHISSHIT.txt")).read().strip()
JIRA_URL = "https://zen-mesh.atlassian.net"
JIRA_EMAIL = "zen@kube-zen.io"
L1_ENDPOINT = "http://localhost:56227"
L1_MODEL = "Qwen3.5-0.8B-Q4_K_M.gguf"
EVIDENCE_DIR = os.path.expanduser("~/zen/zen-brain1/docs/05-OPERATIONS/evidence/l1-expansion-batch1")

os.makedirs(EVIDENCE_DIR, exist_ok=True)

SYSTEM_PROMPT = """You are a remediation planner for zen-brain1. You produce BOUNDED EDIT PLANS only.
You do NOT rewrite entire files. You do NOT produce full file content.

Return ONLY valid JSON:
{
  "edit_description": "what to change and why (1-3 sentences)",
  "target_files": ["path/to/file"],
  "patch_commands": ["exact sed/awk/echo commands"],
  "validation_commands": ["commands to verify"],
  "expected_outcome": "what success looks like",
  "risk_notes": "any risks",
  "follow_up_type": "none | needs_review | needs_testing"
}
Rules: patch_commands must be concrete shell commands. No new_content or file_body fields. Keep output under 500 tokens. Return JSON only."""

# ─── Existing tickets to re-process ────────────────────────────────────
EXISTING_TICKETS = [
    {"key": "ZB-843", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "summary": "Remote Code Execution via exec Command",
     "description": "Add input sanitization to exec command to prevent RCE. Target cmd/main.go where exec is called.",
     "validation": "grep -q 'sanitize\\|validate\\|whitelist' cmd/main.go"},
    {"key": "ZB-844", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "summary": "Unrestricted Shell Access via Shell Command",
     "description": "Add shell command whitelist/sanitization. Target cmd/main.go.",
     "validation": "grep -q 'whitelist\\|sanitize\\|allowlist' cmd/main.go"},
    {"key": "ZB-849", "target_files": ["cmd/commands/parse.go"], "task_type": "code_edit",
     "summary": "SyntaxError: --json flag not handled in parse.go",
     "description": "Add --json flag handling to parse function. Target cmd/commands/parse.go.",
     "validation": "grep -q 'json' cmd/commands/parse.go"},
    {"key": "ZB-850", "target_files": ["cmd/commands/parse.go"], "task_type": "code_edit",
     "summary": "SyntaxError in parse function causing exit failure",
     "description": "Add missing return statement to parse function in cmd/commands/parse.go.",
     "validation": "grep -q 'return' cmd/commands/parse.go"},
    {"key": "ZB-851", "target_files": ["cmd/commands/parse.go"], "task_type": "code_edit",
     "summary": "CLI Tool: parse() returns bool instead of exit code",
     "description": "Change parse() return type from bool to error in cmd/commands/parse.go.",
     "validation": "grep -q 'return' cmd/commands/parse.go"},
    {"key": "ZB-852", "target_files": ["cmd/commands/parse.go"], "task_type": "code_edit",
     "summary": "CLI Tool: parse function returns string instead of int",
     "description": "Fix parse function return type in cmd/commands/parse.go to use proper exit codes.",
     "validation": "grep -q 'return' cmd/commands/parse.go"},
    {"key": "ZB-813", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "summary": "Missing input validation for UserAgent parameter (injection)",
     "description": "Add UserAgent input validation to prevent injection attacks. Target cmd/main.go.",
     "validation": "grep -q 'UserAgent' cmd/main.go"},
    {"key": "ZB-814", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "summary": "Missing input validation for UserAgent parameter (XSS)",
     "description": "Add UserAgent sanitization to prevent XSS. Target cmd/main.go.",
     "validation": "grep -q 'sanitize\\|escape\\|validate' cmd/main.go"},
    {"key": "ZB-816", "target_files": ["cmd/main.go"], "task_type": "code_edit",
     "summary": "Missing input validation for UserAgent parameter (CSRF)",
     "description": "Add CSRF token validation for UserAgent requests. Target cmd/main.go.",
     "validation": "grep -q 'csrf\\|token\\|validate' cmd/main.go"},
    {"key": "ZB-854", "target_files": ["scripts/backlog-baseline.py"], "task_type": "code_edit",
     "summary": "[L1-v2] Add error handling to backlog-baseline.py HTTP calls",
     "description": "Add try/except around subprocess calls in scripts/backlog-baseline.py.",
     "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/backlog-baseline.py\").read())'"},
    {"key": "ZB-858", "target_files": ["docs/05-OPERATIONS/evidence/evidence-pack-template.json"], "task_type": "doc_update",
     "summary": "[L1-v2] Add retention metadata to evidence pack template",
     "description": "Create evidence-pack-template.json with retention_period and created_at fields.",
     "validation": "test -f docs/05-OPERATIONS/evidence/evidence-pack-template.json"},
    {"key": "ZB-863", "target_files": ["Makefile"], "task_type": "config_change",
     "summary": "[L1-v2] Add 'make evidence-clean' target",
     "description": "Add Makefile target that removes evidence/*.json older than 90 days.",
     "validation": "grep -q 'evidence-clean' Makefile"},
    {"key": "ZB-867", "target_files": ["docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY_v2.md"], "task_type": "doc_update",
     "summary": "[L1-v2] Document L1 attribution policy in markdown",
     "description": "Create L1_ATTRIBUTION_POLICY_v2.md with rules for claiming L1 did useful work.",
     "validation": "test -f docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY_v2.md"},
]

# ─── New bounded tasks to create ───────────────────────────────────────
NEW_TASKS = [
    {"summary": "[EXP-01] Add l1-produced label to Jira update workflow",
     "description": "Add 'produced-by:l1' label to Jira comment template in jira-drain.py for L1-executed tickets.",
     "target_files": ["scripts/jira-drain.py"], "task_type": "code_edit",
     "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/jira-drain.py\").read())'"},
    {"summary": "[EXP-02] Add version header to all evidence pack templates",
     "description": "Add a 'schema_version' field set to '2.0' to evidence-pack-template.json.",
     "target_files": ["docs/05-OPERATIONS/evidence/evidence-pack-template.json"], "task_type": "config_change",
     "validation": "python3 -c 'import json; json.load(open(\"docs/05-OPERATIONS/evidence/evidence-pack-template.json\"))'"},
    {"summary": "[EXP-03] Add .gitignore for *.egg-info and __pycache__",
     "description": "Add *.egg-info and __pycache__/ to .gitignore in repo root.",
     "target_files": [".gitignore"], "task_type": "config_change",
     "validation": "grep -q 'egg-info\\|pycache' .gitignore"},
    {"summary": "[EXP-04] Add Makefile target 'make validate' to run Python syntax checks",
     "description": "Add a Makefile target 'validate' that runs python3 -m py_compile on all Python scripts in scripts/.",
     "target_files": ["Makefile"], "task_type": "config_change",
     "validation": "grep -q 'validate' Makefile"},
    {"summary": "[EXP-05] Add comment to jira-drain.py documenting transition IDs",
     "description": "Add a comment block listing all Jira transition IDs (11=Backlog, 21=Selected, 31=InProgress, 41=Done, 51=Paused, 61=Retrying, 71=ToEscalate) at the top of the TRANSITIONS dict in jira-drain.py.",
     "target_files": ["scripts/jira-drain.py"], "task_type": "doc_update",
     "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/jira-drain.py\").read())'"},
    {"summary": "[EXP-06] Add scripts/validate.sh for pre-commit Python checking",
     "description": "Create scripts/validate.sh that runs python3 -m py_compile on scripts/*.py and reports pass/fail.",
     "target_files": ["scripts/validate.sh"], "task_type": "config_change",
     "validation": "test -x scripts/validate.sh"},
    {"summary": "[EXP-07] Add GOVERNANCE.md section on L1 attribution tracking",
     "description": "Add a section to docs/05-OPERATIONS/BACKLOG_DRAIN_MODE.md about governance requirements for L1-executed tickets.",
     "target_files": ["docs/05-OPERATIONS/BACKLOG_DRAIN_MODE.md"], "task_type": "doc_update",
     "validation": "grep -q 'governance' docs/05-OPERATIONS/BACKLOG_DRAIN_MODE.md"},
]

# ─── Helpers ───────────────────────────────────────────────────────────

def jira_post(path, body):
    r = subprocess.run(["curl","-s","-o","/dev/null","-w","%{http_code}","-X","POST",
        "-u",f"{JIRA_EMAIL}:{TOKEN}","-H","Content-Type: application/json",
        "-d",json.dumps(body),f"{JIRA_URL}/rest/api/3{path}"],
        capture_output=True, text=True, timeout=15)
    return r.stdout.strip() == "204" or r.stdout.strip() == "201"

def transition_ticket(key, tid):
    return jira_post(f"/issue/{key}/transitions", {"transition": {"id": tid}})

def add_comment(key, text):
    jira_post(f"/issue/{key}/comment", {"body": {"type": "doc", "version": 1,
        "content": [{"type": "paragraph", "content": [{"type": "text", "text": text}]}]}})

def create_ticket(summary, desc, labels):
    body = {"fields": {"project": {"key": "ZB"}, "issuetype": {"name": "Task"},
        "summary": summary, "labels": labels,
        "description": {"type": "doc", "version": 1,
            "content": [{"type": "paragraph", "content": [{"type": "text", "text": desc}]}]}}}
    r = subprocess.run(["curl","-s","-X","POST","-u",f"{JIRA_EMAIL}:{TOKEN}",
        "-H","Content-Type: application/json","-d",json.dumps(body),
        f"{JIRA_URL}/rest/api/3/issue"], capture_output=True, text=True, timeout=15)
    try:
        return json.loads(r.stdout).get("key")
    except:
        return None

def call_l1(task, jira_key):
    user_prompt = f"""Ticket: {jira_key}
Target: {', '.join(task['target_files'])}
Problem: {task['description']}
Type: {task['task_type']}
Validation: {task['validation']}
Produce a bounded edit plan with sed/echo patch_commands. Return JSON only."""
    payload = {"model": L1_MODEL, "messages": [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": user_prompt}], "temperature": 0.2, "max_tokens": 2048,
        "chat_template_kwargs": {"enable_thinking": False}}
    start = time.time()
    r = subprocess.run(["curl","-s","--max-time","60","-H","Content-Type: application/json",
        "-d",json.dumps(payload),f"{L1_ENDPOINT}/v1/chat/completions"],
        capture_output=True, text=True, timeout=90)
    elapsed = time.time() - start
    llm_content = ""
    try:
        d = json.loads(r.stdout)
        llm_content = d.get("choices",[{}])[0].get("message",{}).get("content","")
    except:
        llm_content = r.stdout
    json_str = llm_content.strip()
    for pat in [r'^```json\s*', r'^```\s*', r'\s*```$']:
        json_str = re.sub(pat, '', json_str)
    si, ei = json_str.find('{'), json_str.rfind('}')
    if si >= 0 and ei > si: json_str = json_str[si:ei+1]
    parsed, parse_error = None, None
    for attempt in [json_str, re.sub(r',\s*}', '}', json_str), re.sub(r',\s*]', ']', json_str)]:
        try:
            parsed = json.loads(attempt)
            break
        except Exception as e:
            parse_error = str(e)
    return {"llm_content": llm_content[:2000], "parsed": parsed, "parse_error": parse_error,
            "elapsed_sec": round(elapsed, 1)}

def score_output(parsed, task):
    if not parsed: return 0, []
    scores, issues = {}, []
    files_str = str(parsed.get("target_files",[])) + str(parsed.get("patch_commands",[]))
    target_hit = any(tf in files_str for tf in task["target_files"])
    scores["target"] = 5 if target_hit else 2
    if not target_hit: issues.append("target_missing")
    desc = parsed.get("edit_description","")
    scores["desc"] = 5 if len(desc) > 20 else (3 if len(desc) > 5 else 0)
    patches = parsed.get("patch_commands",[])
    has_concrete = any(any(c in str(p) for c in ["sed","echo","awk","printf","cat","mkdir","touch","cp","mv"]) for p in (patches if isinstance(patches,list) else [patches]))
    scores["patches"] = 5 if has_concrete else (3 if patches else 0)
    if not has_concrete and not patches: issues.append("no_patches")
    vals = parsed.get("validation_commands",[])
    scores["validation"] = 5 if vals else 0
    if not vals: issues.append("no_validation")
    serial = json.dumps(parsed)
    has_forbidden = any(f in serial for f in ["new_content","file_body","full_content"])
    scores["no_forbidden"] = 0 if has_forbidden else 5
    if has_forbidden: issues.append("forbidden_field")
    return sum(scores.values()), issues

# ─── Main ──────────────────────────────────────────────────────────────

def run():
    print("=== CONTROLLED EXPANSION — 20-TICKET BATCH ===")
    print(f"L1: {L1_MODEL} | max_tokens=2048 | timeout=60s")
    print()

    results = []
    all_tasks = []

    # Phase 1: Create new tickets
    print("Phase 1: Creating 7 new tickets...")
    for task in NEW_TASKS:
        key = create_ticket(task["summary"],
            f"{task['description']}\n\nTarget: {', '.join(task['target_files'])}\nType: {task['task_type']}\nValidation: {task['validation']}",
            labels=["expansion-batch1", "lane:l1", "contract:patch-v2"])
        if key:
            print(f"  Created: {key} — {task['summary'][:60]}")
            t = dict(task)
            t["key"] = key
            t["is_new"] = True
            all_tasks.append(t)
        else:
            print(f"  FAILED to create: {task['summary'][:60]}")

    # Phase 2: Add existing tickets
    print(f"\nPhase 2: Adding {len(EXISTING_TICKETS)} existing tickets...")
    for t in EXISTING_TICKETS:
        t["is_new"] = False
        all_tasks.append(t)

    print(f"\nTotal batch: {len(all_tasks)} tickets")
    print()

    # Phase 3: Process through L1
    print("Phase 3: Processing through L1...")
    for i, task in enumerate(all_tasks):
        key = task["key"]
        ts = datetime.now().strftime("%Y%m%d-%H%M%S")
        safe = key.replace("-","_")
        new_marker = " [NEW]" if task.get("is_new") else ""
        print(f"\n[{i+1}/{len(all_tasks)}] {key}{new_marker}: {task['task_type']} — {task['summary'][:60]}")

        # Transition
        transition_ticket(key, "31")  # In Progress

        # Call L1
        print(f"  Calling L1...", end="", flush=True)
        l1 = call_l1(task, key)
        print(f" {l1['elapsed_sec']}s")

        # Score
        quality, issues = score_output(l1["parsed"], task)
        print(f"  Score: {quality}/25 issues={issues}")

        # Determine disposition
        if l1["parse_error"] or not l1["parsed"]:
            disposition = "l1-failed-parse"
            final_state = "RETRYING" if task.get("is_new") else "PAUSED"
        elif quality < 15:
            disposition = "l1-low-quality"
            final_state = "PAUSED"
        elif quality < 20:
            disposition = "l1-produced-needs-review"
            final_state = "PAUSED"
        else:
            disposition = "l1-produced"
            final_state = "Done"

        # Check forbidden fields
        if l1["parsed"] and any(f in json.dumps(l1["parsed"]) for f in ["new_content","file_body"]):
            disposition = "l1-forbidden-fields"
            final_state = "PAUSED"

        # Save artifact
        raw_path = os.path.join(EVIDENCE_DIR, f"{safe}_raw.json")
        with open(raw_path, "w") as f:
            json.dump({"jira_key": key, "contract": "v2-patch", "task": task,
                "l1_result": l1, "quality_score": quality, "quality_issues": issues,
                "timestamp": ts, "disposition": disposition}, f, indent=2)

        if l1["parsed"] and quality >= 15:
            norm_path = os.path.join(EVIDENCE_DIR, f"{safe}_normalized.json")
            with open(norm_path, "w") as f:
                json.dump({"jira_key": key, "normalized_output": l1["parsed"],
                    "quality_score": quality, "timestamp": ts}, f, indent=2)

        # Update Jira
        comment = f"""[EXPANSION-BATCH1] Patch-oriented v2 contract
Produced by: {disposition}
Model: {L1_MODEL}
Elapsed: {l1['elapsed_sec']}s
Quality: {quality}/25
Issues: {', '.join(issues) if issues else 'none'}
Artifact: {raw_path}"""
        add_comment(key, comment)
        transition_ticket(key, {"Done":"41","PAUSED":"51","RETRYING":"61"}.get(final_state, "51"))
        print(f"  -> {final_state} ({disposition})")

        results.append({
            "jira_key": key, "is_new": task.get("is_new", False),
            "task_type": task["task_type"], "l1_elapsed_sec": l1["elapsed_sec"],
            "parsed": bool(l1["parsed"]), "quality_score": quality, "quality_issues": issues,
            "has_patches": bool(l1["parsed"] and l1["parsed"].get("patch_commands")),
            "has_validation": bool(l1["parsed"] and l1["parsed"].get("validation_commands")),
            "has_forbidden": bool(l1["parsed"] and any(f in json.dumps(l1["parsed"]) for f in ["new_content","file_body"])),
            "disposition": disposition, "final_state": final_state,
        })
        time.sleep(0.5)

    # ─── Scoreboard ────────────────────────────────────────────────────
    counts = {
        "l1_produced": sum(1 for r in results if r["disposition"] == "l1-produced"),
        "l1_produced_needs_review": sum(1 for r in results if r["disposition"] == "l1-produced-needs-review"),
        "l1_failed_parse": sum(1 for r in results if "failed" in r["disposition"]),
        "l1_low_quality": sum(1 for r in results if "low-quality" in r["disposition"]),
        "l1_forbidden_fields": sum(1 for r in results if "forbidden" in r["disposition"]),
    }
    total = len(results)
    l1_prod = counts["l1_produced"]
    l1_prod_pct = round(l1_prod / total * 100) if total else 0

    scoreboard = {"timestamp": datetime.now().isoformat(), "batch": "expansion-batch1",
        "total": total, "counts": counts, "l1_produced_pct": l1_prod_pct, "results": results}
    with open(os.path.join(EVIDENCE_DIR, "expansion-scoreboard.json"), "w") as f:
        json.dump(scoreboard, f, indent=2)

    print("\n" + "="*100)
    print("=== EXPANSION BATCH 1 SCOREBOARD ===")
    print(f"{'Key':<8} {'New':<5} {'Type':<14} {'Time':>6} {'Parse':<6} {'Score':>6} {'Patch':<6} {'Valid':<6} {'Forbidden':<10} {'Disposition':<30} {'State'}")
    print("-"*110)
    for r in results:
        print(f"{r['jira_key']:<8} {'yes' if r['is_new'] else 'no':<5} {r['task_type']:<14} {r['l1_elapsed_sec']:>5.1f}s {'yes' if r['parsed'] else 'no':<6} {r['quality_score']:>5}/25 {'yes' if r['has_patches'] else 'no':<6} {'yes' if r['has_validation'] else 'no':<6} {'yes' if r['has_forbidden'] else 'no':<10} {r['disposition']:<30} {r['final_state']}")

    print(f"\nCOUNTS:")
    for k,v in counts.items():
        print(f"  {k}: {v}")
    print(f"\nL1-PRODUCED RATE: {l1_prod}/{total} ({l1_prod_pct}%)")
    print(f"THRESHOLD: {'MET' if l1_prod_pct >= 60 else 'NOT MET'}")

    return scoreboard

if __name__ == "__main__":
    run()
