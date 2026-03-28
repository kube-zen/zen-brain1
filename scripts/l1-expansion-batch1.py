#!/usr/bin/env python3
"""
Controlled Expansion — 20-ticket backlog-drain batch.

Patch-oriented v2 contract. Sequential L1 calls to avoid overload.
Retries EXP-01..EXP-07 from prior failed batch, then creates+runs 13 more.
"""
import json, subprocess, sys, os, time, re
from datetime import datetime

TOKEN = open(os.path.expanduser("~/zen/DONOTASKMOREFORTHISSHIT.txt")).read().strip()
JIRA_URL = "https://zen-mesh.atlassian.net"
JIRA_EMAIL = "zen@kube-zen.io"
JIRA_PROJECT = "ZB"
L1_ENDPOINT = "http://localhost:56227"
L1_MODEL = "Qwen3.5-0.8B-Q4_K_M.gguf"
EVIDENCE_DIR = os.path.expanduser("~/zen/zen-brain1/docs/05-OPERATIONS/evidence/l1-expansion-batch1")
BATCH_ID = "expansion-batch1"

TRANSITIONS = {
    "Selected for Development": "21", "In Progress": "31", "Done": "41",
    "PAUSED": "51", "RETRYING": "61", "TO_ESCALATE": "71",
}

os.makedirs(EVIDENCE_DIR, exist_ok=True)

SYSTEM_PROMPT = """You are a remediation planner for zen-brain1. Produce BOUNDED EDIT PLANS only.
Return ONLY valid JSON:
{"problem_summary":"...","target_files":["..."],"edit_description":"...","patch_commands":["sed/echo commands"],"validation_commands":["commands"],"expected_outcome":"...","risk_notes":"...","follow_up_type":"none|needs_review"}
Rules: concrete sed/echo commands in patch_commands. No new_content. No full files. JSON only."""

# ─── 13 new bounded tasks (after 7 retries from EXP-01..07) ───────────

NEW_TASKS = [
    {
        "summary": "[EXP-08] Add TODO.md to track open remediation items",
        "description": "Create a TODO.md at repo root listing current open items: L1 attribution expansion, evidence pack standardization, worker allocation rebalancing.",
        "target_files": ["TODO.md"],
        "task_type": "doc_update",
        "validation": "test -f TODO.md && grep -q 'attribution' TODO.md",
    },
    {
        "summary": "[EXP-09] Add scripts/lint.sh for Go vet and gofmt checking",
        "description": "Create scripts/lint.sh that runs go vet ./... and gofmt -l . in the repo. Make it executable.",
        "target_files": ["scripts/lint.sh"],
        "task_type": "config_change",
        "validation": "test -f scripts/lint.sh && test -x scripts/lint.sh",
    },
    {
        "summary": "[EXP-10] Add .editorconfig for consistent formatting",
        "description": "Create .editorconfig at repo root with standard Go/Python/JSON formatting rules (2-space indent for Python/JSON, tabs for Go, UTF-8, LF line endings).",
        "target_files": [".editorconfig"],
        "task_type": "config_change",
        "validation": "test -f .editorconfig && grep -q 'root' .editorconfig",
    },
    {
        "summary": "[EXP-11] Add CLAUDE.md project conventions doc",
        "description": "Create CLAUDE.md at repo root with project conventions: Go toolchain, test commands, commit message format, L1 contract rules.",
        "target_files": ["CLAUDE.md"],
        "task_type": "doc_update",
        "validation": "test -f CLAUDE.md && grep -q 'contract' CLAUDE.md",
    },
    {
        "summary": "[EXP-12] Add Makefile target 'make fmt' for Go formatting",
        "description": "Add a Makefile target 'fmt' that runs gofmt -w . on Go source files.",
        "target_files": ["Makefile"],
        "task_type": "config_change",
        "validation": "grep -q 'fmt' Makefile",
    },
    {
        "summary": "[EXP-13] Add Makefile target 'make lint' for lint checks",
        "description": "Add a Makefile target 'lint' that runs scripts/lint.sh.",
        "target_files": ["Makefile"],
        "task_type": "config_change",
        "validation": "grep -q 'lint' Makefile",
    },
    {
        "summary": "[EXP-14] Add scripts/clean.sh to remove build artifacts",
        "description": "Create scripts/clean.sh that removes remediation-worker binary, __pycache__ dirs, and .egg-info dirs. Make executable.",
        "target_files": ["scripts/clean.sh"],
        "task_type": "config_change",
        "validation": "test -f scripts/clean.sh && test -x scripts/clean.sh",
    },
    {
        "summary": "[EXP-15] Add comment header to scripts/jira-drain.py",
        "description": "Add a shebang line (#!/usr/bin/env python3) and docstring header to scripts/jira-drain.py if missing.",
        "target_files": ["scripts/jira-drain.py"],
        "task_type": "doc_update",
        "validation": "head -1 scripts/jira-drain.py | grep -q 'python3'",
    },
    {
        "summary": "[EXP-16] Add scripts/backlog-status.sh quick Jira status check",
        "description": "Create scripts/backlog-status.sh that runs python3 scripts/jira-drain.py status and shows only Backlog+Done counts. Make executable.",
        "target_files": ["scripts/backlog-status.sh"],
        "task_type": "config_change",
        "validation": "test -f scripts/backlog-status.sh",
    },
    {
        "summary": "[EXP-17] Add Makefile target 'make clean' for artifact cleanup",
        "description": "Add a Makefile target 'clean' that runs scripts/clean.sh.",
        "target_files": ["Makefile"],
        "task_type": "config_change",
        "validation": "grep -q 'clean' Makefile",
    },
    {
        "summary": "[EXP-18] Add .gitignore entries for Go build artifacts",
        "description": "Add remediation-worker and *.exe to .gitignore to prevent Go binaries from being committed.",
        "target_files": [".gitignore"],
        "task_type": "config_change",
        "validation": "grep -q 'remediation-worker' .gitignore",
    },
    {
        "summary": "[EXP-19] Add scripts/run-pilot.sh for L1 attribution pilot runs",
        "description": "Create scripts/run-pilot.sh that runs python3 scripts/l1-attribution-pilot-v2.py with the v2 contract. Make executable.",
        "target_files": ["scripts/run-pilot.sh"],
        "task_type": "config_change",
        "validation": "test -f scripts/run-pilot.sh",
    },
    {
        "summary": "[EXP-20] Add README section on L1 attribution pilot",
        "description": "Add a section to README.md (or create one if missing) explaining how to run the L1 attribution pilot and interpret the scoreboard.",
        "target_files": ["README.md"],
        "task_type": "doc_update",
        "validation": "test -f README.md && grep -q 'attribution' README.md",
    },
]

# ─── Retry tasks from prior failed batch ──────────────────────────────

RETRY_TASKS = [
    {"summary": "[EXP-01] Add l1-produced label to Jira update workflow", "jira_key": "ZB-883",
     "description": "Add a 'produced-by:l1' label option to the Jira update workflow in jira-drain.py so L1-attributed tickets get labeled automatically.",
     "target_files": ["scripts/jira-drain.py"], "task_type": "code_edit",
     "validation": "grep -q 'produced-by' scripts/jira-drain.py"},
    {"summary": "[EXP-02] Add version header to evidence pack templates", "jira_key": "ZB-884",
     "description": "Add a version: 2.0 field to all evidence pack JSON templates for tracking.",
     "target_files": ["docs/05-OPERATIONS/evidence/evidence-pack-template.json"], "task_type": "config_change",
     "validation": "grep -q 'version' docs/05-OPERATIONS/evidence/evidence-pack-template.json"},
    {"summary": "[EXP-03] Add .gitignore for *.egg-info and __pycache__", "jira_key": "ZB-885",
     "description": "Add *.egg-info and __pycache__/ to .gitignore.",
     "target_files": [".gitignore"], "task_type": "config_change",
     "validation": "grep -q 'egg-info' .gitignore"},
    {"summary": "[EXP-04] Add Makefile target 'make validate'", "jira_key": "ZB-886",
     "description": "Add a Makefile target 'validate' that runs python3 -m py_compile on all Python scripts in scripts/.",
     "target_files": ["Makefile"], "task_type": "config_change",
     "validation": "grep -q 'validate' Makefile"},
    {"summary": "[EXP-05] Add comment to jira-drain.py documenting transition IDs", "jira_key": "ZB-887",
     "description": "Add a comment block in jira-drain.py listing the Jira transition IDs: Done=41, PAUSED=51, RETRYING=61, TO_ESCALATE=71.",
     "target_files": ["scripts/jira-drain.py"], "task_type": "doc_update",
     "validation": "grep -q 'transition' scripts/jira-drain.py"},
    {"summary": "[EXP-06] Add scripts/validate.sh for pre-commit Python checking", "jira_key": "ZB-888",
     "description": "Create scripts/validate.sh that runs python3 -m py_compile on all .py files under scripts/. Make executable.",
     "target_files": ["scripts/validate.sh"], "task_type": "config_change",
     "validation": "test -f scripts/validate.sh"},
    {"summary": "[EXP-07] Add GOVERNANCE.md section on L1 attribution tracking", "jira_key": "ZB-889",
     "description": "Add a section to GOVERNANCE.md (or create if missing) documenting L1 attribution tracking requirements.",
     "target_files": ["GOVERNANCE.md"], "task_type": "doc_update",
     "validation": "test -f GOVERNANCE.md && grep -q 'attribution' GOVERNANCE.md"},
]

# ─── Jira helpers ─────────────────────────────────────────────────────

def jira_post(path, body):
    r = subprocess.run(["curl","-s","-o","/dev/null","-w","%{http_code}","-X","POST",
        "-u",f"{JIRA_EMAIL}:{TOKEN}","-H","Content-Type: application/json",
        "-d",json.dumps(body),f"{JIRA_URL}/rest/api/3{path}"],
        capture_output=True, text=True, timeout=15)
    return r.stdout.strip()

def transition(key, state):
    tid = TRANSITIONS.get(state)
    if not tid: return False
    code = jira_post(f"/issue/{key}/transitions", {"transition":{"id":tid}})
    return code == "204"

def add_comment(key, text):
    jira_post(f"/issue/{key}/comment", {"body":{"type":"doc","version":1,
        "content":[{"type":"paragraph","content":[{"type":"text","text":text}]}]}})

def create_ticket(summary, description, labels=None):
    r = subprocess.run(["curl","-s","-X","POST","-u",f"{JIRA_EMAIL}:{TOKEN}",
        "-H","Content-Type: application/json",
        "-d",json.dumps({"fields":{"project":{"key":JIRA_PROJECT},"issuetype":{"name":"Task"},
            "summary":summary,
            "description":{"type":"doc","version":1,"content":[{"type":"paragraph",
                "content":[{"type":"text","text":description}]}]},
            "labels":labels or [BATCH_ID,"lane:l1","contract:patch-v2"]}}),
        f"{JIRA_URL}/rest/api/3/issue"], capture_output=True, text=True, timeout=15)
    try:
        d = json.loads(r.stdout)
        return d.get("key")
    except:
        return None

# ─── L1 call ──────────────────────────────────────────────────────────

def call_l1(task, jira_key):
    user_prompt = f"""Ticket: {jira_key}
Target: {', '.join(task['target_files'])}
Problem: {task['description']}
Type: {task['task_type']}
Validation: {task['validation']}

Produce a bounded edit plan. Use sed/echo/printf commands.
Return JSON only. No markdown. No prose."""

    payload = {
        "model": L1_MODEL,
        "messages": [
            {"role":"system","content":SYSTEM_PROMPT},
            {"role":"user","content":user_prompt},
        ],
        "temperature": 0.2,
        "max_tokens": 2048,
        "chat_template_kwargs": {"enable_thinking": False},
    }

    start = time.time()
    r = subprocess.run(
        ["curl","-s","--max-time","60","-H","Content-Type: application/json",
         "-d",json.dumps(payload),f"{L1_ENDPOINT}/v1/chat/completions"],
        capture_output=True, text=True, timeout=90)
    elapsed = time.time() - start

    raw = r.stdout
    content = ""
    try:
        d = json.loads(raw)
        content = d.get("choices",[{}])[0].get("message",{}).get("content","")
    except:
        content = raw

    # Extract JSON
    js = content.strip()
    js = re.sub(r'^```json\s*','', js)
    js = re.sub(r'^```\s*','', js)
    js = re.sub(r'\s*```$','', js)
    si, ei = js.find('{'), js.rfind('}')
    if si >= 0 and ei > si:
        js = js[si:ei+1]

    parsed, parse_error = None, None
    try:
        parsed = json.loads(js)
    except Exception as e:
        for attempt in [js, re.sub(r',\s*}','}',js), re.sub(r',\s*]',']',js),
                        js.replace('\n',' ').replace('\r','')]:
            try:
                parsed = json.loads(attempt); break
            except: continue
        if not parsed:
            parse_error = str(e)

    return {"raw":raw[:2000],"content":content[:2000],"parsed":parsed,
            "parse_error":parse_error,"elapsed":round(elapsed,1)}

# ─── Quality gate ─────────────────────────────────────────────────────

def score(parsed, task):
    if not parsed: return 0, []
    issues = []
    s = {}
    files_mentioned = str(parsed.get("target_files",[])) + str(parsed.get("patch_commands",[]))
    s["target"] = 5 if any(t in files_mentioned for t in task["target_files"]) else 2
    if s["target"] < 5: issues.append("target_missing")
    desc = parsed.get("edit_description","")
    s["desc"] = 5 if len(desc)>20 else (3 if len(desc)>5 else 0)
    if len(desc)<=5: issues.append("desc_short")
    patches = parsed.get("patch_commands",[])
    has_cmd = any(any(c in str(p) for c in ["sed","echo","awk","printf","cat","mkdir","touch","cp","mv","grep","write","create"]) for p in (patches if isinstance(patches,list) else [patches]))
    s["patches"] = 5 if has_cmd else (3 if len(patches)>0 else 0)
    if not has_cmd: issues.append("no_concrete_patches")
    vals = parsed.get("validation_commands",[])
    s["validation"] = 5 if (isinstance(vals,list) and len(vals)>0) else 0
    if not (isinstance(vals,list) and len(vals)>0): issues.append("no_validation")
    serial = json.dumps(parsed)
    s["no_forbidden"] = 0 if any(f in serial for f in ["new_content","file_body"]) else 5
    if s["no_forbidden"]==0: issues.append("forbidden_field")
    return sum(s.values()), issues

# ─── Process one ticket ───────────────────────────────────────────────

def process_task(task, jira_key, initial_state, is_retry=False):
    ts = datetime.now().strftime("%Y%m%d-%H%M%S")
    safe = task['target_files'][0].replace('/','_').replace('.','_')

    print(f"\n{'[RETRY]' if is_retry else '[NEW]   '} {jira_key}: {task['summary'][:60]}")
    print(f"  Initial: {initial_state}")

    # Transition to In Progress
    transition(jira_key, "Selected for Development")
    transition(jira_key, "In Progress")
    print(f"  -> In Progress")

    # Call L1
    print(f"  Calling L1...")
    l1 = call_l1(task, jira_key)
    print(f"  L1: {l1['elapsed']}s, parse={'ok' if l1['parsed'] else l1['parse_error'] or 'empty'}")

    # Score
    q_score, q_issues = score(l1['parsed'], task)
    print(f"  Quality: {q_score}/25 {q_issues if q_issues else ''}")

    # Save evidence
    raw_path = os.path.join(EVIDENCE_DIR, f"{jira_key}_{safe}_raw.json")
    with open(raw_path, "w") as f:
        json.dump({"jira_key":jira_key,"batch":BATCH_ID,"initial_state":initial_state,
            "task":task,"l1_result":l1,"quality_score":q_score,"quality_issues":q_issues,
            "timestamp":ts,"attribution":{"produced_by":"l1" if q_score>=15 else "l1-failed",
            "first_pass_model":L1_MODEL}}, f, indent=2)

    # Determine attribution
    if not l1['parsed'] or l1['parse_error']:
        disposition, final_state = "l1-produced-needs-review", "RETRYING"
        produced_by, intervention = "l1-failed-parse", "normalization_only"
    elif q_score < 15:
        disposition, final_state = "l1-produced-needs-review", "PAUSED"
        produced_by, intervention = "l1-low-quality", "quality_gate_rejected"
    elif q_score < 20:
        norm_path = os.path.join(EVIDENCE_DIR, f"{jira_key}_{safe}_normalized.json")
        with open(norm_path, "w") as f:
            json.dump({"jira_key":jira_key,"batch":BATCH_ID,"normalized":l1['parsed'],
                "quality_score":q_score,"timestamp":ts}, f, indent=2)
        disposition, final_state = "l1-produced-needs-review", "PAUSED"
        produced_by, intervention = "l1", "none"
    else:
        norm_path = os.path.join(EVIDENCE_DIR, f"{jira_key}_{safe}_normalized.json")
        with open(norm_path, "w") as f:
            json.dump({"jira_key":jira_key,"batch":BATCH_ID,"normalized":l1['parsed'],
                "quality_score":q_score,"timestamp":ts}, f, indent=2)
        disposition, final_state = "l1-produced", "Done"
        produced_by, intervention = "l1", "none"

    # Check forbidden
    if l1['parsed'] and any(f in json.dumps(l1['parsed']) for f in ["new_content","file_body"]):
        intervention = "contains_forbidden"
        if disposition == "l1-produced":
            disposition, final_state = "l1-produced-needs-review", "PAUSED"

    # Jira update
    comment = f"""[{Batch_ID.upper()}] Patch-oriented v2 contract
Initial: {initial_state} | Final: {final_state}
Produced by: {produced_by} | Model: {L1_MODEL}
Elapsed: {l1['elapsed']}s | Quality: {q_score}/25
Issues: {', '.join(q_issues) if q_issues else 'none'}
Disposition: {disposition} | Supervisor: {intervention}
Artifact: {raw_path}"""
    add_comment(jira_key, comment)
    transition(jira_key, final_state)
    print(f"  -> {final_state} ({disposition}, {produced_by})")

    return {"jira_key":jira_key,"summary":task["summary"],"task_type":task["task_type"],
        "initial_state":initial_state,"l1_elapsed":l1['elapsed'],"parsed":"yes" if l1['parsed'] else "no",
        "quality_score":q_score,"quality_issues":q_issues,
        "has_patches":bool(l1['parsed'] and l1['parsed'].get("patch_commands")),
        "has_validation":bool(l1['parsed'] and l1['parsed'].get("validation_commands")),
        "produced_by":produced_by,"supervisor_intervention":intervention,
        "final_disposition":disposition,"final_state":final_state,
        "artifact_path":raw_path,"evidence_pack":f"{safe}_evidence.json"}

# ─── Main ─────────────────────────────────────────────────────────────

def main():
    print("="*70)
    print(f"CONTROLLED EXPANSION — 20-ticket batch")
    print(f"Contract: v2 patch-oriented | max_tokens: 2048 | timeout: 60s")
    print(f"Evidence: {EVIDENCE_DIR}")
    print("="*70)

    results = []

    # Phase 1: Retry EXP-01..07
    print(f"\n{'='*70}")
    print("PHASE 1: Retrying 7 failed tickets from prior batch")
    print("="*70)
    for task in RETRY_TASKS:
        result = process_task(task, task["jira_key"], "RETRYING", is_retry=True)
        results.append(result)
        time.sleep(1)

    # Phase 2: Create + run EXP-08..20
    print(f"\n{'='*70}")
    print("PHASE 2: Creating and running 13 new tickets")
    print("="*70)
    for task in NEW_TASKS:
        key = create_ticket(task["summary"],
            f"{task['description']}\n\nTarget: {', '.join(task['target_files'])}\n"
            f"Type: {task['task_type']}\nValidation: {task['validation']}\n\n"
            f"[Controlled expansion batch1 — patch-oriented v2 contract]")
        if not key:
            print(f"  FAILED to create ticket for: {task['summary'][:50]}")
            results.append({"jira_key":"FAILED","summary":task["summary"],"task_type":task["task_type"],
                "initial_state":"Backlog","l1_elapsed":0,"parsed":"no","quality_score":0,
                "produced_by":"none","final_disposition":"failed","final_state":"failed"})
            continue
        result = process_task(task, key, "Backlog")
        results.append(result)
        time.sleep(1)

    # ─── Scoreboard ────────────────────────────────────────────────────
    counts = {
        "l1_produced": sum(1 for r in results if r["final_disposition"]=="l1-produced"),
        "l1_produced_needs_review": sum(1 for r in results if r["final_disposition"]=="l1-produced-needs-review"),
        "supervisor_written": 0, "script_only": 0, "failed": sum(1 for r in results if r["final_disposition"]=="failed"),
    }
    total_done = sum(1 for r in results if r["final_state"]=="Done")

    scoreboard = {"timestamp":datetime.now().isoformat(),"batch":BATCH_ID,
        "contract":"v2-patch-oriented","total_tasks":len(results),"results":results,"counts":counts,
        "metrics":{"l1_produced_rate":round(counts["l1_produced"]/max(len(results),1)*100),
            "done_count":total_done,"avg_cycle_time":round(sum(r["l1_elapsed"] for r in results)/max(len(results),1),1)}}

    sb_path = os.path.join(EVIDENCE_DIR, "expansion-scoreboard.json")
    with open(sb_path, "w") as f:
        json.dump(scoreboard, f, indent=2)

    # Print results
    print(f"\n{'='*70}")
    print("EXPANSION SCOREBOARD")
    print("="*70)
    print(f"{'Key':<10} {'Initial':<12} {'Type':<16} {'Time':>6} {'Parse':<6} {'Score':>6} {'Patches':<9} {'Produced':<18} {'Final':<12}")
    print("-"*110)
    for r in results:
        print(f"{r['jira_key']:<10} {r['initial_state']:<12} {r['task_type']:<16} {r['l1_elapsed']:>5.1f}s "
            f"{'yes' if r['parsed']=='yes' else 'no':<6} {r['quality_score']:>5}/25 "
            f"{'yes' if r['has_patches'] else 'no':<9} {r['produced_by']:<18} {r['final_state']:<12}")

    print(f"\nCOUNTS:")
    for k,v in counts.items(): print(f"  {k}: {v}")
    print(f"\nMETRICS:")
    print(f"  L1-produced rate: {scoreboard['metrics']['l1_produced_rate']}%")
    print(f"  Done count: {total_done}")
    print(f"  Avg cycle time: {scoreboard['metrics']['avg_cycle_time']}s")

    # Decision gate
    rate = scoreboard['metrics']['l1_produced_rate']
    print(f"\nDECISION GATE:")
    if rate >= 60 and total_done >= 10:
        print(f"  -> SCENARIO A: Continue expansion (rate={rate}%, done={total_done})")
    elif rate >= 50:
        print(f"  -> SCENARIO B: Hold size, inspect failures (rate={rate}%)")
    elif total_done < 5 and sum(1 for r in results if r['final_state'] in ['RETRYING','PAUSED']) > 10:
        print(f"  -> SCENARIO D: Execution bottleneck (done={total_done}, retrying/paused high)")
    else:
        print(f"  -> SCENARIO C: Stop expansion (rate={rate}%)")

    return scoreboard

if __name__ == "__main__":
    main()
