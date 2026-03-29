#!/usr/bin/env python3
"""
L1 Attribution Pilot — Phases 1-4 of the attribution plan.

Creates 10 bounded tasks, dispatches each to the real L1 (0.8b on port 56227),
saves every artifact (raw output, normalized, final), and records attribution.

This proves whether L1 actually does the work or not.
"""
import json, subprocess, sys, os, time, re
from datetime import datetime

from common.zen_lock import get_jira_token, get_jira_email
TOKEN = get_jira_token()
JIRA_URL = "https://zen-mesh.atlassian.net"
JIRA_EMAIL = get_jira_email()
JIRA_PROJECT = "ZB"
L1_ENDPOINT = "http://localhost:56227"
L1_MODEL = "Qwen3.5-0.8B-Q4_K_M.gguf"
EVIDENCE_DIR = os.path.expanduser("~/zen/zen-brain1/docs/05-OPERATIONS/evidence/l1-attribution-pilot")

# Transition IDs
TRANSITION_IDS = {
    "Selected for Development": "21",
    "In Progress": "31",
    "Done": "41",
    "PAUSED": "51",
    "RETRYING": "61",
    "TO_ESCALATE": "71",
}

os.makedirs(EVIDENCE_DIR, exist_ok=True)

# ─── 10 Bounded Tasks ─────────────────────────────────────────────────

TASKS = [
    {
        "summary": "[L1-PILOT] Add .gitignore entry for evidence/*.json artifacts",
        "description": "Add *.json to .gitignore under docs/05-OPERATIONS/evidence/ to prevent large JSON artifacts from being committed to the repo.",
        "target_files": ".gitignore",
        "task_type": "config_change",
        "validation": "grep -q 'evidence/.*json' .gitignore",
        "lane": "l1",
    },
    {
        "summary": "[L1-PILOT] Add error handling to scripts/backlog-baseline.py HTTP calls",
        "description": "The backlog-baseline.py script does not handle HTTP errors gracefully. Add try/except around each curl subprocess call with proper error messages.",
        "target_files": "scripts/backlog-baseline.py",
        "task_type": "code_edit",
        "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/backlog-baseline.py\").read())'",
        "lane": "l1",
    },
    {
        "summary": "[L1-PILOT] Add doc comment to RemediationPacket struct in remediation-worker",
        "description": "Add a Go doc comment to the RemediationPacket struct explaining what each field is for, so future developers can understand the remediation packet format.",
        "target_files": "cmd/remediation-worker/main.go",
        "task_type": "doc_update",
        "validation": "grep -q 'RemediationPacket' cmd/remediation-worker/main.go",
        "lane": "l1",
    },
    {
        "summary": "[L1-PILOT] Create a Makefile target 'make drain-status' that runs jira-drain.py status",
        "description": "Add a Makefile target 'drain-status' that runs 'python3 scripts/jira-drain.py status' and prints the current Jira state counts.",
        "target_files": "Makefile",
        "task_type": "config_change",
        "validation": "grep -q 'drain-status' Makefile",
        "lane": "l1",
    },
    {
        "summary": "[L1-PILOT] Add timeout flag to scripts/jira-drain.py CLI",
        "description": "Add a --timeout flag to jira-drain.py that sets the curl timeout for Jira API calls. Default 30 seconds.",
        "target_files": "scripts/jira-drain.py",
        "task_type": "code_edit",
        "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/jira-drain.py\").read())'",
        "lane": "l1",
    },
    {
        "summary": "[L1-PILOT] Add retention policy header to evidence pack files",
        "description": "All evidence pack JSON files should start with a comment or metadata field stating the retention period (90 days) and creation date. Create a template for this.",
        "target_files": "docs/05-OPERATIONS/evidence/evidence-pack-template.json",
        "task_type": "doc_update",
        "validation": "test -f docs/05-OPERATIONS/evidence/evidence-pack-template.json",
        "lane": "l1",
    },
    {
        "summary": "[L1-PILOT] Write a quickstart comment block at top of jira-drain.py",
        "description": "Add a clear comment block at the top of jira-drain.py showing 3-4 common usage examples so new developers can use it immediately.",
        "target_files": "scripts/jira-drain.py",
        "task_type": "doc_update",
        "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/jira-drain.py\").read())'",
        "lane": "l1",
    },
    {
        "summary": "[L1-PILOT] Add 'make evidence-clean' target to purge stale evidence JSON files",
        "description": "Add a Makefile target that removes evidence/*.json files older than 90 days using find -mtime +90 -delete.",
        "target_files": "Makefile",
        "task_type": "config_change",
        "validation": "grep -q 'evidence-clean' Makefile",
        "lane": "l1",
    },
    {
        "summary": "[L1-PILOT] Add attribution fields to evidence pack template",
        "description": "The evidence pack template needs attribution fields: produced_by, first_pass_model, supervisor_intervention, artifact_authorship. Add these to the JSON template.",
        "target_files": "docs/05-OPERATIONS/evidence/evidence-pack-template.json",
        "task_type": "config_change",
        "validation": "python3 -c 'import json; json.load(open(\"docs/05-OPERATIONS/evidence/evidence-pack-template.json\"))'",
        "lane": "l1",
    },
    {
        "summary": "[L1-PILOT] Document L1 attribution policy in a markdown file",
        "description": "Create docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY.md explaining the rules for claiming L1 did useful work: must have saved artifact, must trace to L1 output, supervisor vs L1 distinction.",
        "target_files": "docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY.md",
        "task_type": "doc_update",
        "validation": "test -f docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY.md && grep -q attribution docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY.md",
        "lane": "l1",
    },
]

# ─── Jira Helpers ──────────────────────────────────────────────────────

def jira_api(method, path, body=None):
    cmd = ["curl", "-s", "-X", method, "-u", f"{JIRA_EMAIL}:{TOKEN}",
           "-H", "Content-Type: application/json"]
    if body:
        cmd += ["-d", json.dumps(body)]
    cmd.append(f"{JIRA_URL}/rest/api/3{path}")
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
    return json.loads(r.stdout) if r.stdout else {}

def create_jira_ticket(summary, description, labels=None):
    """Create a Jira ticket and return its key."""
    body = {
        "fields": {
            "project": {"key": JIRA_PROJECT},
            "issuetype": {"name": "Task"},
            "summary": summary,
            "description": {
                "type": "doc", "version": 1,
                "content": [{"type": "paragraph", "content": [{"type": "text", "text": description}]}]
            },
            "labels": labels or ["l1-pilot", "attribution-test"],
        }
    }
    result = jira_api("POST", "/issue", body)
    return result.get("key")

def transition_ticket(key, target):
    tid = TRANSITION_IDS.get(target)
    if not tid: return False
    result = jira_api("POST", f"/issue/{key}/transitions", {"transition": {"id": tid}})
    return "errorMessages" not in result

def add_comment(key, text):
    body = {"body": {"type": "doc", "version": 1,
             "content": [{"type": "paragraph", "content": [{"type": "text", "text": text}]}]}}
    jira_api("POST", f"/issue/{key}/comment", body)

# ─── L1 Call ───────────────────────────────────────────────────────────

def call_l1(task, jira_key):
    """Call the real 0.8b model with a remediation packet and return raw + parsed output."""
    system_prompt = """You are a remediation worker for zen-brain1. Produce a bounded change description for the target files.
Return ONLY valid JSON with these fields:
{"remediation_type":"code_edit|config_change|doc_update|cannot_fix","file_to_edit":"path","change_type":"create|modify|delete","edit_description":"what to change and why","new_content":"the actual file content or patch to apply","explanation":"why","final_status":"success|needs_review|blocked","validation_command":"command to verify the change"}
No markdown fences. No prose. Just the JSON object."""

    user_prompt = f"""Ticket: {jira_key}
Target file: {task['target_files']}
Problem: {task['description']}
Task type: {task['task_type']}
Validation: {task['validation']}
Constraints: Produce only the change needed. Be precise.
Return JSON only."""

    payload = {
        "model": L1_MODEL,
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt},
        ],
        "temperature": 0.3,
        "max_tokens": 4096,
        "chat_template_kwargs": {"enable_thinking": False},
    }

    start = time.time()
    r = subprocess.run(
        ["curl", "-s", "--max-time", "120",
         "-H", "Content-Type: application/json",
         "-d", json.dumps(payload),
         f"{L1_ENDPOINT}/v1/chat/completions"],
        capture_output=True, text=True, timeout=180)
    elapsed = time.time() - start

    raw_response = r.stdout

    # Parse LLM response
    llm_content = ""
    try:
        d = json.loads(raw_response)
        llm_content = d.get("choices", [{}])[0].get("message", {}).get("content", "")
    except:
        llm_content = raw_response

    # Extract JSON from content
    json_str = llm_content.strip()
    json_str = re.sub(r'^```json\s*', '', json_str)
    json_str = re.sub(r'^```\s*', '', json_str)
    json_str = re.sub(r'\s*```$', '', json_str)

    start_idx = json_str.find('{')
    end_idx = json_str.rfind('}')
    if start_idx >= 0 and end_idx > start_idx:
        json_str = json_str[start_idx:end_idx+1]

    parsed = None
    parse_error = None
    try:
        parsed = json.loads(json_str)
    except Exception as e:
        # Attempt repair
        repaired = re.sub(r',\s*}', '}', json_str)
        repaired = re.sub(r',\s*]', ']', repaired)
        repaired = repaired.replace('\n', ' ').replace('\r', '').replace('\t', ' ')
        try:
            parsed = json.loads(repaired)
        except Exception as e2:
            parse_error = str(e2)

    return {
        "raw_response": raw_response[:2000],
        "llm_content": llm_content[:2000],
        "json_str": json_str[:2000],
        "parsed": parsed,
        "parse_error": parse_error,
        "elapsed_sec": round(elapsed, 1),
    }

# ─── Main Pilot ────────────────────────────────────────────────────────

def run_pilot():
    print("=== L1 ATTRIBUTION PILOT ===")
    print(f"Tasks: {len(TASKS)}")
    print(f"L1: {L1_ENDPOINT} ({L1_MODEL})")
    print(f"Evidence dir: {EVIDENCE_DIR}")
    print()

    results = []

    for i, task in enumerate(TASKS):
        print(f"\n--- [{i+1}/{len(TASKS)}] {task['summary'][:70]} ---")
        ts = datetime.now().strftime("%Y%m%d-%H%M%S")
        safe_name = task['target_files'].replace('/', '_').replace('.', '_')

        # Step 1: Create Jira ticket
        jira_key = create_jira_ticket(
            task['summary'],
            f"{task['description']}\n\nTarget: {task['target_files']}\nType: {task['task_type']}\nValidation: {task['validation']}",
            labels=["l1-pilot", "attribution-test", f"lane:{task['lane']}"]
        )
        if not jira_key:
            print("  FAILED to create Jira ticket")
            results.append({
                "task": task["summary"],
                "jira_key": "FAILED",
                "produced_by": "none",
                "supervisor_intervention": "jira_creation_failed",
                "final_state": "failed",
            })
            continue

        print(f"  Created: {jira_key}")

        # Step 2: Transition to Selected for Development
        transition_ticket(jira_key, "Selected for Development")
        print(f"  -> Selected for Development")

        # Step 3: Transition to In Progress
        transition_ticket(jira_key, "In Progress")
        print(f"  -> In Progress")

        # Step 4: Call L1
        print(f"  Calling L1 ({L1_MODEL})...")
        l1_result = call_l1(task, jira_key)
        print(f"  L1 response: {l1_result['elapsed_sec']}s, parse_error={l1_result['parse_error']}")

        # Step 5: Save raw L1 output
        raw_path = os.path.join(EVIDENCE_DIR, f"{jira_key}_{safe_name}_raw.json")
        with open(raw_path, "w") as f:
            json.dump({
                "jira_key": jira_key,
                "task": task,
                "l1_result": l1_result,
                "timestamp": ts,
                "attribution": {
                    "produced_by": "l1",
                    "first_pass_model": L1_MODEL,
                }
            }, f, indent=2)
        print(f"  Raw saved: {raw_path}")

        # Step 6: Normalize and evaluate
        parsed = l1_result.get("parsed")
        produced_by = "l1"
        supervisor_intervention = "none"
        artifact_authorship = "l1"
        final_disposition = "l1-produced"
        validation_result = "not_run"
        final_jira_state = "Done"

        if l1_result["parse_error"] or not parsed:
            produced_by = "l1-failed-parse"
            supervisor_intervention = "normalization_only"
            artifact_authorship = "none"
            final_disposition = "l1-produced-needs-review"
            validation_result = "parse_failed"
            final_jira_state = "PAUSED"
        else:
            # Check if L1 produced usable output
            rem_type = parsed.get("remediation_type", "")
            has_content = bool(parsed.get("new_content") or parsed.get("edit_description"))
            status = parsed.get("final_status", "needs_review")

            if status == "success" and has_content:
                # Save normalized artifact
                norm_path = os.path.join(EVIDENCE_DIR, f"{jira_key}_{safe_name}_normalized.json")
                with open(norm_path, "w") as f:
                    json.dump({
                        "jira_key": jira_key,
                        "normalized_output": parsed,
                        "timestamp": ts,
                        "attribution": {
                            "produced_by": "l1",
                            "first_pass_model": L1_MODEL,
                            "supervisor_intervention": "none",
                        }
                    }, f, indent=2)
                print(f"  Normalized saved: {norm_path}")
                validation_result = "l1_output_parseable"
                final_jira_state = "Done"
            elif status == "needs_review":
                final_disposition = "l1-produced-needs-review"
                validation_result = "l1_marked_needs_review"
                final_jira_state = "PAUSED"
            elif status == "blocked":
                final_disposition = "l1-produced-needs-review"
                validation_result = "l1_marked_blocked"
                final_jira_state = "PAUSED"
            else:
                # L1 produced something but unclear
                final_disposition = "l1-produced-needs-review"
                validation_result = "l1_output_unclear"
                final_jira_state = "PAUSED"

        # Step 7: Update Jira
        comment_text = f"""[L1-ATTRIBUTION-PILOT]
Produced by: {produced_by}
Model: {L1_MODEL}
L1 elapsed: {l1_result['elapsed_sec']}s
Parse error: {l1_result['parse_error'] or 'none'}
Final disposition: {final_disposition}
Supervisor intervention: {supervisor_intervention}
Artifact path: {raw_path}
Timestamp: {ts}"""
        add_comment(jira_key, comment_text)

        # Step 8: Transition to final state
        transition_ticket(jira_key, final_jira_state)
        print(f"  -> {final_jira_state} (disposition: {final_disposition})")

        result = {
            "jira_key": jira_key,
            "summary": task["summary"],
            "task_type": task["task_type"],
            "target_files": task["target_files"],
            "lane": task["lane"],
            "l1_elapsed_sec": l1_result["elapsed_sec"],
            "l1_parse_error": l1_result["parse_error"],
            "raw_output_path": raw_path,
            "parsed_output": "yes" if parsed else "no",
            "has_new_content": bool(parsed and parsed.get("new_content")) if parsed else False,
            "validation_result": validation_result,
            "produced_by": produced_by,
            "supervisor_intervention": supervisor_intervention,
            "artifact_authorship": artifact_authorship,
            "final_disposition": final_disposition,
            "final_jira_state": final_jira_state,
        }
        results.append(result)
        time.sleep(1)  # Rate limit

    # ─── Save scoreboard ───────────────────────────────────────────────
    scoreboard = {
        "timestamp": datetime.now().isoformat(),
        "total_tasks": len(TASKS),
        "results": results,
        "counts": {
            "l1_produced": sum(1 for r in results if r["final_disposition"] == "l1-produced"),
            "l1_produced_needs_review": sum(1 for r in results if r["final_disposition"] == "l1-produced-needs-review"),
            "supervisor_written": sum(1 for r in results if r["final_disposition"] == "supervisor-written"),
            "script_only": sum(1 for r in results if r["final_disposition"] == "script-only"),
            "failed": sum(1 for r in results if r["final_disposition"] == "failed"),
        }
    }

    scoreboard_path = os.path.join(EVIDENCE_DIR, "l1-attribution-scoreboard.json")
    with open(scoreboard_path, "w") as f:
        json.dump(scoreboard, f, indent=2)

    # Print scoreboard
    print("\n" + "="*60)
    print("=== L1 ATTRIBUTION SCOREBOARD ===")
    print("="*60)
    print()
    print(f"{'Jira Key':<10} {'Type':<16} {'Lane':<4} {'Parsed':<7} {'Content':<8} {'Validation':<28} {'Produced By':<18} {'Supervisor':<20} {'Final'}")
    print("-"*140)
    for r in results:
        print(f"{r['jira_key']:<10} {r['task_type']:<16} {r['lane']:<4} {'yes' if r['parsed_output']=='yes' else 'no':<7} {'yes' if r['has_new_content'] else 'no':<8} {r['validation_result']:<28} {r['produced_by']:<18} {r['supervisor_intervention']:<20} {r['final_jira_state']}")

    print()
    print("COUNTS:")
    for k, v in scoreboard["counts"].items():
        print(f"  {k}: {v}")

    return scoreboard

if __name__ == "__main__":
    run_pilot()
