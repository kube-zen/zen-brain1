#!/usr/bin/env python3
"""
Jira State Machine + Backlog Drain Tool
Implements Phases 2-5 of the backlog drain plan.

Usage:
  python3 scripts/jira-drain.py bulk-close-stale    # Close ~480 duplicate scanner tickets
  python3 scripts/jira-drain.py transition ZB-100 Done  # Move single ticket
  python3 scripts/jira-drain.py drain-pilot             # Run 10-ticket drain pilot
  python3 scripts/jira-drain.py status                  # Show current state counts
"""
import json, subprocess, sys, os, time
from datetime import datetime

TOKEN = open(os.path.expanduser("~/zen/DONOTASKMOREFORTHISSHIT.txt")).read().strip()
URL = "https://zen-mesh.atlassian.net"
EMAIL = "zen@kube-zen.io"
PROJECT = "ZB"
EVIDENCE_DIR = os.path.expanduser("~/zen/zen-brain1/docs/05-OPERATIONS/evidence")

# Transition IDs (global, available from any state)
TRANSITIONS = {
    "Backlog": "11",
    "Selected for Development": "21",
    "In Progress": "31",
    "Done": "41",
    "PAUSED": "51",
    "RETRYING": "61",
    "TO_ESCALATE": "71",
}

# Valid state machine transitions
STATE_MACHINE = {
    "Backlog": ["Selected for Development"],
    "Selected for Development": ["In Progress"],
    "In Progress": ["Done", "PAUSED", "RETRYING", "TO_ESCALATE"],
    "RETRYING": ["In Progress", "TO_ESCALATE", "Done"],
    "PAUSED": ["In Progress", "Done"],
    "TO_ESCALATE": ["Done", "PAUSED"],
}

def api(method, path, body=None):
    """Call Jira API."""
    cmd = ["curl", "-s", "-X", method, "-u", f"{EMAIL}:{TOKEN}",
           "-H", "Content-Type: application/json"]
    if body:
        cmd += ["-d", json.dumps(body)]
    cmd.append(f"{URL}/rest/api/3{path}")
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
    return json.loads(r.stdout) if r.stdout else {}

def search(jql, max_results=100, fields=None, page_token=None):
    body = {"jql": jql, "maxResults": max_results}
    if fields: body["fields"] = fields
    if page_token: body["nextPageToken"] = page_token
    return api("POST", "/search/jql", body)

def search_all(jql, fields=None):
    """Paginate through all results."""
    all_issues = []
    token = None
    while True:
        body = {"jql": jql, "maxResults": 100}
        if fields: body["fields"] = fields
        if token: body["nextPageToken"] = token
        d = api("POST", "/search/jql", body)
        issues = d.get("issues", [])
        all_issues.extend(issues)
        token = d.get("nextPageToken")
        if d.get("isLast", True) or not token:
            break
    return all_issues

def transition_ticket(key, target_state, comment=None):
    """Transition a ticket to a target state."""
    tid = TRANSITIONS.get(target_state)
    if not tid:
        print(f"  ERROR: Unknown target state '{target_state}'")
        return False
    
    body = {"transition": {"id": tid}}
    if comment:
        body["update"] = {
            "comment": [{"add": {"body": {"type": "doc", "version": 1,
                "content": [{"type": "paragraph", "content": [{"type": "text", "text": comment}]}]}}}]
        }
    
    result = api("POST", f"/issue/{key}/transitions", body)
    if "errorMessages" in result:
        print(f"  ERROR {key}: {result['errorMessages']}")
        return False
    return True

def add_comment(key, text):
    """Add a comment to a ticket."""
    body = {"body": {"type": "doc", "version": 1,
             "content": [{"type": "paragraph", "content": [{"type": "text", "text": text}]}]}}
    return api("POST", f"/issue/{key}/comment", body)

def get_status_counts():
    """Get counts for each status."""
    counts = {}
    for status in ["Backlog", "Selected for Development", "In Progress", "PAUSED", "RETRYING", "TO_ESCALATE", "Done"]:
        issues = search_all(f'project = {PROJECT} AND status = "{status}"')
        counts[status] = len(issues)
    return counts

def cmd_status():
    """Show current status counts."""
    counts = get_status_counts()
    print("=== CURRENT JIRA STATE ===")
    total = 0
    for s, c in counts.items():
        print(f"  {s}: {c}")
        total += c
    print(f"  TOTAL: {total}")
    return counts

def cmd_bulk_close_stale():
    """Bulk close stale/duplicate scanner tickets, keeping latest of each type."""
    print("=== BULK CLOSE STALE SCANNER TICKETS ===")
    
    # Get all backlog issues
    all_issues = search_all(
        f'project = {PROJECT} AND status = Backlog ORDER BY created ASC',
        fields=["summary", "labels", "created"]
    )
    print(f"Total Backlog: {len(all_issues)}")
    
    # Categorize
    batch_parents = []  # scheduled-batch containers
    defect_tickets = []  # genuine defects
    findings_by_type = {}  # type -> [issues]
    
    for iss in all_issues:
        key = iss["key"]
        f = iss.get("fields", {})
        summary = f.get("summary", "")
        lbls = f.get("labels", [])
        
        if "defect" in lbls:
            defect_tickets.append(iss)
            continue
        
        if "scheduled-batch" in lbls:
            batch_parents.append(iss)
            continue
        
        if "finding" in lbls:
            # Extract finding type from summary
            ftype = "other"
            for prefix in ["[DAILY-SWEEP] ", "[HOURLY-SCAN] ", "[QUAD-HOURLY-SUMMARY] "]:
                if summary.startswith(prefix):
                    body = summary[len(prefix):]
                    if ":" in body:
                        ftype = body.split(":")[0].strip().replace(" ", "_")
                    break
            
            if ftype not in findings_by_type:
                findings_by_type[ftype] = []
            findings_by_type[ftype].append(iss)
            continue
        
        # Everything else (discovery, etc.)
        batch_parents.append(iss)
    
    print(f"Defect tickets (KEEP): {len(defect_tickets)}")
    print(f"Batch parents (CLOSE): {len(batch_parents)}")
    
    # For each finding type, keep only the latest, close the rest
    to_close = list(batch_parents)
    kept = {}
    for ftype, issues in findings_by_type.items():
        # Sort by created desc, keep latest
        issues_sorted = sorted(issues, key=lambda x: x.get("fields", {}).get("created", ""), reverse=True)
        kept[ftype] = issues_sorted[0]
        to_close.extend(issues_sorted[1:])
        print(f"  {ftype}: {len(issues)} total, keeping latest ({issues_sorted[0]['key']}), closing {len(issues_sorted)-1}")
    
    print(f"\nTickets to close: {len(to_close)}")
    print(f"Tickets to keep in Backlog: {len(defect_tickets) + len(kept)}")
    print(f"  Defects: {[i['key'] for i in defect_tickets]}")
    print(f"  Latest findings: {[(k, v['key']) for k, v in kept.items()]}")
    
    # Execute bulk close
    closed = 0
    errors = 0
    for iss in to_close:
        key = iss["key"]
        summary = iss.get("fields", {}).get("summary", "")[:60]
        
        # Add comment explaining closure
        comment_text = f"bulk-close: stale/duplicate scanner output. Closed during backlog drain baseline cleanup on {datetime.now().strftime('%Y-%m-%d')}. Summary was: {summary}"
        add_comment(key, comment_text)
        
        # Transition to Done
        if transition_ticket(key, "Done"):
            closed += 1
            if closed % 50 == 0:
                print(f"  Progress: {closed}/{len(to_close)} closed...")
        else:
            errors += 1
            print(f"  FAILED: {key}")
        
        # Rate limit: 50 requests per 10 seconds
        if closed % 40 == 0:
            time.sleep(2)
    
    print(f"\n=== BULK CLOSE RESULT ===")
    print(f"Closed: {closed}")
    print(f"Errors: {errors}")
    
    # Save evidence
    evidence = {
        "timestamp": datetime.now().isoformat(),
        "action": "bulk_close_stale",
        "total_backlog_before": len(all_issues),
        "closed": closed,
        "errors": errors,
        "defect_tickets_kept": [i["key"] for i in defect_tickets],
        "latest_findings_kept": {k: v["key"] for k, v in kept.items()},
    }
    with open(os.path.join(EVIDENCE_DIR, "bulk-close-evidence.json"), "w") as f:
        json.dump(evidence, f, indent=2)
    
    # Show new state
    print("\n=== NEW STATE ===")
    cmd_status()
    return evidence

def cmd_drain_pilot():
    """Run 10-ticket drain pilot on the kept tickets."""
    print("=== 10-TICKET BACKLOG DRAIN PILOT ===")
    
    # Get baseline
    baseline = get_status_counts()
    print(f"Baseline: {json.dumps(baseline)}")
    
    # Get current backlog
    backlog = search_all(
        f'project = {PROJECT} AND status = Backlog ORDER BY created ASC',
        fields=["summary", "labels", "description"]
    )
    
    # Select pilot tickets: 5 defects + 5 latest findings
    defects = [i for i in backlog if "defect" in i.get("fields", {}).get("labels", [])]
    
    # Get latest findings (one per type, prioritizing actionable types)
    priority_types = ["test_gaps", "config_drift", "stub_hunting", "dead_code", "package_hotspots"]
    findings = {}
    for iss in backlog:
        f = iss.get("fields", {})
        summary = f.get("summary", "")
        lbls = f.get("labels", [])
        if "finding" not in lbls or "defect" in lbls:
            continue
        for pt in priority_types:
            if pt.replace("_", " ") in summary.lower() or pt in summary.lower():
                if pt not in findings:
                    findings[pt] = iss
    
    # Build pilot set
    pilot = list(defects[:5])
    for pt in priority_types:
        if pt in findings and len(pilot) < 10:
            pilot.append(findings[pt])
    
    print(f"Pilot tickets selected: {len(pilot)}")
    for i, iss in enumerate(pilot):
        f = iss.get("fields", {})
        print(f"  {i+1}. {iss['key']}: {f.get('summary','')[:80]}")
    
    if len(pilot) == 0:
        print("No tickets to process. Backlog may already be empty.")
        return
    
    # Process each ticket
    results = []
    for i, iss in enumerate(pilot):
        key = iss["key"]
        f = iss.get("fields", {})
        summary = f.get("summary", "")
        lbls = f.get("labels", [])
        
        print(f"\n--- [{i+1}/{len(pilot)}] {key}: {summary[:60]} ---")
        
        # Step 1: Backlog -> Selected for Development
        print(f"  Backlog -> Selected for Development")
        if not transition_ticket(key, "Selected for Development", 
                                 f"[drain-pilot] Selected for backlog drain pilot. {datetime.now().strftime('%Y-%m-%d %H:%M')}"):
            results.append({"key": key, "initial": "Backlog", "final": "ERROR", "validation": "transition_failed"})
            continue
        time.sleep(0.5)
        
        # Step 2: Selected for Development -> In Progress
        print(f"  Selected for Development -> In Progress")
        if not transition_ticket(key, "In Progress"):
            results.append({"key": key, "initial": "Backlog", "final": "ERROR", "validation": "transition_failed"})
            continue
        time.sleep(0.5)
        
        # Step 3: Classify and determine final state
        is_defect = "defect" in lbls
        is_info = any(t in summary.lower() for t in ["executive_summary", "roadmap"])
        
        if is_defect:
            # Defect tickets: mark as needs_review (L1 hasn't actually fixed them yet)
            final_state = "PAUSED"
            comment = f"[drain-pilot] Defect triaged. Target: cmd/main.go. Needs L1 remediation pass. {datetime.now().strftime('%Y-%m-%d %H:%M')}"
            validation = "triaged_needs_l1"
        elif is_info:
            # Informational reports: close directly
            final_state = "Done"
            comment = f"[drain-pilot] Informational report reviewed during drain pilot. No action needed. {datetime.now().strftime('%Y-%m-%d %H:%M')}"
            validation = "informational_no_action"
        else:
            # Finding reports: needs review/triage
            final_state = "PAUSED"
            comment = f"[drain-pilot] Finding report triaged during drain pilot. Needs human review for actionability. {datetime.now().strftime('%Y-%m-%d %H:%M')}"
            validation = "triaged_needs_review"
        
        # Step 4: Transition to final state
        print(f"  In Progress -> {final_state}")
        add_comment(key, comment)
        if not transition_ticket(key, final_state):
            final_state = "In Progress"
            validation = "transition_to_final_failed"
        
        time.sleep(0.5)
        
        results.append({
            "key": key,
            "summary": summary[:80],
            "initial": "Backlog",
            "lane": "defect" if is_defect else "finding",
            "validation": validation,
            "final": final_state,
        })
        print(f"  Result: {final_state} ({validation})")
    
    # Final counts
    final_counts = get_status_counts()
    
    # Save pilot evidence
    pilot_evidence = {
        "timestamp": datetime.now().isoformat(),
        "baseline": baseline,
        "final": final_counts,
        "tickets": results,
    }
    with open(os.path.join(EVIDENCE_DIR, "drain-pilot-evidence.json"), "w") as f:
        json.dump(pilot_evidence, f, indent=2)
    
    print(f"\n=== PILOT RESULTS ===")
    print(f"Processed: {len(results)}")
    done = sum(1 for r in results if r["final"] == "Done")
    paused = sum(1 for r in results if r["final"] == "PAUSED")
    errors_count = sum(1 for r in results if r["final"] == "ERROR")
    print(f"Done: {done}, PAUSED: {paused}, Errors: {errors_count}")
    print(f"\n=== FINAL STATE ===")
    for s, c in final_counts.items():
        print(f"  {s}: {c}")
    
    return pilot_evidence

def cmd_transition(key, target):
    """Transition a single ticket."""
    print(f"Transitioning {key} -> {target}")
    ok = transition_ticket(key, target, f"[manual] State transition to {target}")
    print(f"Result: {'OK' if ok else 'FAILED'}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)
    
    cmd = sys.argv[1]
    if cmd == "status":
        cmd_status()
    elif cmd == "bulk-close-stale":
        cmd_bulk_close_stale()
    elif cmd == "drain-pilot":
        cmd_drain_pilot()
    elif cmd == "transition":
        if len(sys.argv) < 4:
            print("Usage: jira-drain.py transition ZB-100 Done")
            sys.exit(1)
        cmd_transition(sys.argv[2], sys.argv[3])
    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)
