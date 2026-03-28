#!/usr/bin/env python3
"""
L1 Attribution Pilot v2 — Patch-Oriented Output Contract.

Key change from v1: L1 produces bounded edit PLANS, not full file rewrites.
max_tokens capped at 2048. No new_content blobs.
"""
import json, subprocess, sys, os, time, re
from datetime import datetime

TOKEN = open(os.path.expanduser("~/zen/DONOTASKMOREFORTHISSHIT.txt")).read().strip()
JIRA_URL = "https://zen-mesh.atlassian.net"
JIRA_EMAIL = "zen@kube-zen.io"
JIRA_PROJECT = "ZB"
L1_ENDPOINT = "http://localhost:56227"
L1_MODEL = "Qwen3.5-0.8B-Q4_K_M.gguf"
EVIDENCE_DIR = os.path.expanduser("~/zen/zen-brain1/docs/05-OPERATIONS/evidence/l1-attribution-pilot-v2")

TRANSITION_IDS = {
    "Selected for Development": "21",
    "In Progress": "31",
    "Done": "41",
    "PAUSED": "51",
    "RETRYING": "61",
    "TO_ESCALATE": "71",
}

os.makedirs(EVIDENCE_DIR, exist_ok=True)

# ─── NEW OUTPUT CONTRACT SYSTEM PROMPT ────────────────────────────────

SYSTEM_PROMPT = """You are a remediation planner for zen-brain1. You produce BOUNDED EDIT PLANS only.
You do NOT rewrite entire files. You do NOT produce full file content.

Return ONLY valid JSON with these fields:
{
  "jira_key": "the ticket key",
  "problem_summary": "one sentence describing the issue",
  "target_files": ["path/to/file"],
  "edit_description": "what to change and why, in 1-3 sentences",
  "patch_commands": ["exact sed/awk/echo commands to apply the change"],
  "validation_commands": ["commands to verify the change worked"],
  "expected_outcome": "what success looks like",
  "risk_notes": "any risks or caveats",
  "follow_up_type": "none | needs_review | needs_testing"
}

Rules:
- patch_commands must be concrete, runnable shell commands (sed, echo >>, etc.)
- Do NOT include file content in any field
- Do NOT produce new_content or file_body fields
- Keep total output under 500 tokens
- If you cannot produce a bounded plan, set follow_up_type to "needs_review"
Return JSON only. No markdown fences. No prose."""

# ─── 10 Bounded Tasks (same types as v1 for comparison) ───────────────

TASKS = [
    {
        "summary": "[L1-v2] Add .gitignore entry for evidence/*.json artifacts",
        "description": "Add *.json to .gitignore under docs/05-OPERATIONS/evidence/ to prevent large JSON artifacts from being committed.",
        "target_files": [".gitignore"],
        "task_type": "config_change",
        "validation": "grep -q 'evidence.*json' .gitignore",
    },
    {
        "summary": "[L1-v2] Add error handling to backlog-baseline.py HTTP calls",
        "description": "The backlog-baseline.py script does not handle HTTP errors. Add try/except around subprocess calls.",
        "target_files": ["scripts/backlog-baseline.py"],
        "task_type": "code_edit",
        "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/backlog-baseline.py\").read())'",
    },
    {
        "summary": "[L1-v2] Add doc comment to RemediationPacket struct",
        "description": "Add a Go doc comment above the RemediationPacket struct in cmd/remediation-worker/main.go.",
        "target_files": ["cmd/remediation-worker/main.go"],
        "task_type": "doc_update",
        "validation": "grep -q 'RemediationPacket' cmd/remediation-worker/main.go",
    },
    {
        "summary": "[L1-v2] Create Makefile target 'drain-status'",
        "description": "Add a Makefile target 'drain-status' that runs python3 scripts/jira-drain.py status.",
        "target_files": ["Makefile"],
        "task_type": "config_change",
        "validation": "grep -q 'drain-status' Makefile",
    },
    {
        "summary": "[L1-v2] Add --timeout flag to jira-drain.py",
        "description": "Add a --timeout CLI flag to jira-drain.py that sets curl timeout. Default 30s.",
        "target_files": ["scripts/jira-drain.py"],
        "task_type": "code_edit",
        "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/jira-drain.py\").read())'",
    },
    {
        "summary": "[L1-v2] Add retention metadata to evidence pack template",
        "description": "Create docs/05-OPERATIONS/evidence/evidence-pack-template.json with retention_period and created_at fields.",
        "target_files": ["docs/05-OPERATIONS/evidence/evidence-pack-template.json"],
        "task_type": "doc_update",
        "validation": "test -f docs/05-OPERATIONS/evidence/evidence-pack-template.json",
    },
    {
        "summary": "[L1-v2] Add quickstart comment to jira-drain.py",
        "description": "Add a comment block at top of jira-drain.py with 3-4 usage examples.",
        "target_files": ["scripts/jira-drain.py"],
        "task_type": "doc_update",
        "validation": "python3 -c 'import ast; ast.parse(open(\"scripts/jira-drain.py\").read())'",
    },
    {
        "summary": "[L1-v2] Add 'make evidence-clean' target to purge stale JSON",
        "description": "Add Makefile target that removes evidence/*.json older than 90 days via find -mtime +90 -delete.",
        "target_files": ["Makefile"],
        "task_type": "config_change",
        "validation": "grep -q 'evidence-clean' Makefile",
    },
    {
        "summary": "[L1-v2] Add attribution fields to evidence pack template",
        "description": "Add produced_by, first_pass_model, supervisor_intervention fields to evidence-pack-template.json.",
        "target_files": ["docs/05-OPERATIONS/evidence/evidence-pack-template.json"],
        "task_type": "config_change",
        "validation": "python3 -c 'import json; json.load(open(\"docs/05-OPERATIONS/evidence/evidence-pack-template.json\"))'",
    },
    {
        "summary": "[L1-v2] Document L1 attribution policy in markdown",
        "description": "Create docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY_v2.md with rules for claiming L1 did useful work.",
        "target_files": ["docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY_v2.md"],
        "task_type": "doc_update",
        "validation": "test -f docs/05-OPERATIONS/L1_ATTRIBUTION_POLICY_v2.md",
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
    try:
        return json.loads(r.stdout)
    except:
        return {"error": r.stdout[:500]}

def create_jira_ticket(summary, description, labels=None):
    body = {
        "fields": {
            "project": {"key": JIRA_PROJECT},
            "issuetype": {"name": "Task"},
            "summary": summary,
            "description": {
                "type": "doc", "version": 1,
                "content": [{"type": "paragraph", "content": [{"type": "text", "text": description}]}]
            },
            "labels": labels or ["l1-pilot-v2", "attribution-test"],
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

# ─── L1 Call (patch-oriented, capped at 2048 tokens) ──────────────────

def call_l1(task, jira_key):
    user_prompt = f"""Ticket: {jira_key}
Target file(s): {', '.join(task['target_files'])}
Problem: {task['description']}
Task type: {task['task_type']}
Validation: {task['validation']}

Produce a bounded edit plan. Use sed/echo commands in patch_commands.
Do NOT include file content. Keep output under 500 tokens.
Return JSON only."""

    payload = {
        "model": L1_MODEL,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": user_prompt},
        ],
        "temperature": 0.2,
        "max_tokens": 2048,
        "chat_template_kwargs": {"enable_thinking": False},
    }

    start = time.time()
    r = subprocess.run(
        ["curl", "-s", "--max-time", "60",
         "-H", "Content-Type: application/json",
         "-d", json.dumps(payload),
         f"{L1_ENDPOINT}/v1/chat/completions"],
        capture_output=True, text=True, timeout=90)
    elapsed = time.time() - start

    raw_response = r.stdout
    llm_content = ""
    try:
        d = json.loads(raw_response)
        llm_content = d.get("choices", [{}])[0].get("message", {}).get("content", "")
    except:
        llm_content = raw_response

    # Extract JSON
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
        # Repair attempts
        for attempt in [json_str,
                        re.sub(r',\s*}', '}', json_str),
                        re.sub(r',\s*]', ']', json_str),
                        json_str.replace('\n', ' ').replace('\r', '').replace('\t', ' ')]:
            try:
                parsed = json.loads(attempt)
                break
            except:
                continue
        if not parsed:
            parse_error = str(e)

    return {
        "raw_response": raw_response[:3000],
        "llm_content": llm_content[:3000],
        "json_str": json_str[:3000],
        "parsed": parsed,
        "parse_error": parse_error,
        "elapsed_sec": round(elapsed, 1),
        "token_budget": 2048,
    }

# ─── Quality Gate (patch-oriented scoring) ────────────────────────────

def score_patch_output(parsed, task):
    """Score 0-25 for patch-oriented output."""
    if not parsed:
        return 0, ["no_parsed_output"]

    scores = {}
    issues = []

    # 1. correct target files (0-5)
    mentioned_files = str(parsed.get("target_files", [])) + str(parsed.get("patch_commands", []))
    target_hit = any(tf in mentioned_files for tf in task["target_files"])
    scores["target_files"] = 5 if target_hit else 2
    if not target_hit: issues.append("target_files_missing")

    # 2. actionable edit_description (0-5)
    desc = parsed.get("edit_description", "")
    scores["edit_description"] = 5 if len(desc) > 20 else (3 if len(desc) > 5 else 0)
    if len(desc) <= 5: issues.append("edit_description_too_short")

    # 3. concrete patch_commands (0-5)
    patches = parsed.get("patch_commands", [])
    has_concrete = any(any(cmd in str(p) for cmd in ["sed", "echo", "awk", "printf", "cat", "mkdir", "touch", "cp", "mv"]) for p in (patches if isinstance(patches, list) else [patches]))
    scores["patch_commands"] = 5 if has_concrete else (3 if len(patches) > 0 else 0)
    if not has_concrete and len(patches) == 0: issues.append("no_patch_commands")

    # 4. concrete validation_commands (0-5)
    validations = parsed.get("validation_commands", [])
    has_validation = len(validations) > 0 if isinstance(validations, list) else bool(validations)
    scores["validation_commands"] = 5 if has_validation else 0
    if not has_validation: issues.append("no_validation_commands")

    # 5. no forbidden fields (0-5) — penalty for new_content / file_body
    serial = json.dumps(parsed)
    has_forbidden = any(f in serial for f in ["new_content", "file_body", "full_content"])
    scores["no_forbidden"] = 0 if has_forbidden else 5
    if has_forbidden: issues.append("contains_forbidden_full_content_field")

    total = sum(scores.values())
    return total, issues

# ─── Main Pilot ────────────────────────────────────────────────────────

def run_pilot():
    print("=== L1 ATTRIBUTION PILOT v2 — PATCH-ORIENTED CONTRACT ===")
    print(f"Tasks: {len(TASKS)}")
    print(f"L1: {L1_ENDPOINT} ({L1_MODEL})")
    print(f"max_tokens: 2048 (down from 4096)")
    print(f"curl timeout: 60s (down from 120s)")
    print(f"Evidence: {EVIDENCE_DIR}")
    print()

    results = []

    for i, task in enumerate(TASKS):
        print(f"\n--- [{i+1}/{len(TASKS)}] {task['summary'][:70]} ---")
        ts = datetime.now().strftime("%Y%m%d-%H%M%S")
        safe_name = task['target_files'][0].replace('/', '_').replace('.', '_')

        # Step 1: Create Jira ticket
        jira_key = create_jira_ticket(
            task['summary'],
            f"{task['description']}\n\nTarget: {', '.join(task['target_files'])}\nType: {task['task_type']}\nValidation: {task['validation']}\n\n[Patch-oriented contract v2]",
            labels=["l1-pilot-v2", "attribution-test", f"lane:l1", "contract:patch-v2"]
        )
        if not jira_key:
            print("  FAILED to create Jira ticket")
            results.append({
                "task": task["summary"], "jira_key": "FAILED",
                "produced_by": "none", "final_disposition": "failed",
            })
            continue

        print(f"  Created: {jira_key}")
        transition_ticket(jira_key, "Selected for Development")
        transition_ticket(jira_key, "In Progress")

        # Step 2: Call L1 with patch-oriented contract
        print(f"  Calling L1 (patch-oriented, max_tokens=2048, timeout=60s)...")
        l1_result = call_l1(task, jira_key)
        print(f"  L1 response: {l1_result['elapsed_sec']}s, parse_error={l1_result['parse_error']}")

        # Step 3: Score the output
        parsed = l1_result.get("parsed")
        quality_score, quality_issues = score_patch_output(parsed, task)
        print(f"  Quality gate: {quality_score}/25, issues={quality_issues}")

        # Step 4: Save raw output
        raw_path = os.path.join(EVIDENCE_DIR, f"{jira_key}_{safe_name}_raw.json")
        with open(raw_path, "w") as f:
            json.dump({
                "jira_key": jira_key,
                "contract_version": "v2-patch-oriented",
                "task": task,
                "l1_result": l1_result,
                "quality_score": quality_score,
                "quality_issues": quality_issues,
                "timestamp": ts,
                "attribution": {
                    "produced_by": "l1" if quality_score >= 15 else "l1-failed",
                    "first_pass_model": L1_MODEL,
                }
            }, f, indent=2)

        # Step 5: Determine attribution
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
        elif quality_score < 15:
            produced_by = "l1-low-quality"
            supervisor_intervention = "quality_gate_rejected"
            artifact_authorship = "l1-partial"
            final_disposition = "l1-produced-needs-review"
            validation_result = f"quality_gate_{quality_score}_of_25"
            final_jira_state = "PAUSED"
        elif quality_score >= 15 and quality_score < 20:
            # Save normalized
            norm_path = os.path.join(EVIDENCE_DIR, f"{jira_key}_{safe_name}_normalized.json")
            with open(norm_path, "w") as f:
                json.dump({
                    "jira_key": jira_key,
                    "contract_version": "v2-patch-oriented",
                    "normalized_output": parsed,
                    "quality_score": quality_score,
                    "quality_issues": quality_issues,
                    "timestamp": ts,
                }, f, indent=2)
            final_disposition = "l1-produced-needs-review"
            validation_result = f"quality_gate_{quality_score}_of_25"
            final_jira_state = "PAUSED"
        else:
            # quality_score >= 20 — l1-produced
            norm_path = os.path.join(EVIDENCE_DIR, f"{jira_key}_{safe_name}_normalized.json")
            with open(norm_path, "w") as f:
                json.dump({
                    "jira_key": jira_key,
                    "contract_version": "v2-patch-oriented",
                    "normalized_output": parsed,
                    "quality_score": quality_score,
                    "quality_issues": quality_issues,
                    "timestamp": ts,
                }, f, indent=2)
            print(f"  Normalized saved: {norm_path}")
            validation_result = f"quality_gate_{quality_score}_of_25"
            final_jira_state = "Done"

        # Step 6: Check for forbidden fields
        if parsed and any(f in json.dumps(parsed) for f in ["new_content", "file_body"]):
            supervisor_intervention = "contains_forbidden_fields"
            if final_disposition == "l1-produced":
                final_disposition = "l1-produced-needs-review"
                final_jira_state = "PAUSED"
                print(f"  WARNING: Contains forbidden full-content fields — downgraded to needs-review")

        # Step 7: Update Jira
        comment_text = f"""[L1-ATTRIBUTION-PILOT-v2] Patch-oriented contract
Produced by: {produced_by}
Model: {L1_MODEL}
max_tokens: 2048
L1 elapsed: {l1_result['elapsed_sec']}s
Parse error: {l1_result['parse_error'] or 'none'}
Quality score: {quality_score}/25
Quality issues: {', '.join(quality_issues) if quality_issues else 'none'}
Final disposition: {final_disposition}
Supervisor intervention: {supervisor_intervention}
Artifact: {raw_path}
Timestamp: {ts}"""
        add_comment(jira_key, comment_text)
        transition_ticket(jira_key, final_jira_state)
        print(f"  -> {final_jira_state} (score: {quality_score}/25, disposition: {final_disposition})")

        result = {
            "jira_key": jira_key,
            "summary": task["summary"],
            "task_type": task["task_type"],
            "target_files": task["target_files"],
            "l1_elapsed_sec": l1_result["elapsed_sec"],
            "l1_parse_error": l1_result["parse_error"],
            "raw_output_path": raw_path,
            "parsed_output": "yes" if parsed else "no",
            "quality_score": quality_score,
            "quality_issues": quality_issues,
            "has_patch_commands": bool(parsed and parsed.get("patch_commands")) if parsed else False,
            "has_validation_commands": bool(parsed and parsed.get("validation_commands")) if parsed else False,
            "has_forbidden_fields": bool(parsed and any(f in json.dumps(parsed) for f in ["new_content", "file_body"])) if parsed else False,
            "validation_result": validation_result,
            "produced_by": produced_by,
            "supervisor_intervention": supervisor_intervention,
            "artifact_authorship": artifact_authorship,
            "final_disposition": final_disposition,
            "final_jira_state": final_jira_state,
        }
        results.append(result)
        time.sleep(0.5)

    # ─── Scoreboard ────────────────────────────────────────────────────
    counts = {
        "l1_produced": sum(1 for r in results if r["final_disposition"] == "l1-produced"),
        "l1_produced_needs_review": sum(1 for r in results if r["final_disposition"] == "l1-produced-needs-review"),
        "supervisor_written": sum(1 for r in results if r["final_disposition"] == "supervisor-written"),
        "script_only": sum(1 for r in results if r["final_disposition"] == "script-only"),
        "failed": sum(1 for r in results if r["final_disposition"] == "failed"),
    }

    scoreboard = {
        "timestamp": datetime.now().isoformat(),
        "contract_version": "v2-patch-oriented",
        "token_budget": 2048,
        "curl_timeout": 60,
        "total_tasks": len(TASKS),
        "results": results,
        "counts": counts,
        "v1_comparison": {
            "v1_l1_produced": 3, "v1_l1_produced_pct": 30,
            "v2_l1_produced": counts["l1_produced"], "v2_l1_produced_pct": round(counts["l1_produced"]/len(TASKS)*100) if TASKS else 0,
        }
    }

    scoreboard_path = os.path.join(EVIDENCE_DIR, "l1-attribution-scoreboard-v2.json")
    with open(scoreboard_path, "w") as f:
        json.dump(scoreboard, f, indent=2)

    print("\n" + "="*80)
    print("=== L1 ATTRIBUTION SCOREBOARD v2 — PATCH-ORIENTED CONTRACT ===")
    print("="*80)
    print()
    print(f"{'Jira Key':<10} {'Type':<16} {'Time':>6} {'Parse':<6} {'Score':>6} {'Patches':<9} {'Valid':<9} {'Forbidden':<10} {'Produced By':<18} {'Final'}")
    print("-"*120)
    for r in results:
        print(f"{r['jira_key']:<10} {r['task_type']:<16} {r['l1_elapsed_sec']:>5.1f}s {'yes' if r['parsed_output']=='yes' else 'no':<6} {r['quality_score']:>5}/25 {'yes' if r['has_patch_commands'] else 'no':<9} {'yes' if r['has_validation_commands'] else 'no':<9} {'yes' if r['has_forbidden_fields'] else 'no':<10} {r['produced_by']:<18} {r['final_jira_state']}")

    print()
    print("COUNTS:")
    for k, v in counts.items():
        print(f"  {k}: {v}")
    print()
    print("V1 vs V2:")
    print(f"  v1: {scoreboard['v1_comparison']['v1_l1_produced']}/10 l1-produced ({scoreboard['v1_comparison']['v1_l1_produced_pct']}%)")
    print(f"  v2: {scoreboard['v1_comparison']['v2_l1_produced']}/10 l1-produced ({scoreboard['v1_comparison']['v2_l1_produced_pct']}%)")
    print(f"  Delta: {scoreboard['v1_comparison']['v2_l1_produced'] - scoreboard['v1_comparison']['v1_l1_produced']:+d} tickets")

    return scoreboard

if __name__ == "__main__":
    run_pilot()
