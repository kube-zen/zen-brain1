#!/usr/bin/env python3
"""Phase 1: Capture Jira backlog baseline. Uses the new /rest/api/3/search/jql endpoint."""
import json, subprocess, sys, os

TOKEN = open(os.path.expanduser("~/zen/DONOTASKMOREFORTHISSHIT.txt")).read().strip()
URL = "https://zen-mesh.atlassian.net"
EMAIL = "zen@kube-zen.io"
PROJECT = "ZB"

def jira_post(jql, max_results=1, fields=None, next_page_token=None):
    body = {"jql": jql, "maxResults": max_results}
    if fields:
        body["fields"] = fields
    if next_page_token:
        body["nextPageToken"] = next_page_token
    
    r = subprocess.run(
        ["curl", "-s", "-u", f"{EMAIL}:{TOKEN}",
         "-H", "Content-Type: application/json",
         "-d", json.dumps(body),
         f"{URL}/rest/api/3/search/jql"],
        capture_output=True, text=True, timeout=30
    )
    return json.loads(r.stdout)

def count_status(status):
    """Count all issues in a given status by paginating."""
    jql = f'project = {PROJECT} AND status = "{status}"'
    total = 0
    token = None
    while True:
        body = {"jql": jql, "maxResults": 100}
        if token:
            body["nextPageToken"] = token
        r = subprocess.run(
            ["curl", "-s", "-u", f"{EMAIL}:{TOKEN}",
             "-H", "Content-Type: application/json",
             "-d", json.dumps(body),
             f"{URL}/rest/api/3/search/jql"],
            capture_output=True, text=True, timeout=30
        )
        d = json.loads(r.stdout)
        if "errorMessages" in d:
            print(f"  ERROR: {d['errorMessages']}", file=sys.stderr)
            return -1
        issues = d.get("issues", [])
        total += len(issues)
        token = d.get("nextPageToken")
        if d.get("isLast", True) or not token:
            break
    return total

def get_issues(status, max_results=50, fields=None):
    """Get issues in a given status."""
    jql = f'project = {PROJECT} AND status = "{status}" ORDER BY created ASC'
    body = {"jql": jql, "maxResults": max_results}
    if fields:
        body["fields"] = fields
    r = subprocess.run(
        ["curl", "-s", "-u", f"{EMAIL}:{TOKEN}",
         "-H", "Content-Type: application/json",
         "-d", json.dumps(body),
         f"{URL}/rest/api/3/search/jql"],
        capture_output=True, text=True, timeout=30
    )
    return json.loads(r.stdout)

# === STATE COUNTS ===
print("=== JIRA STATE COUNTS ===")
statuses = ["Backlog", "Selected for Development", "In Progress", "PAUSED", "RETRYING", "TO_ESCALATE", "Done"]
state_counts = {}
total_all = 0
for s in statuses:
    c = count_status(s)
    state_counts[s] = c
    total_all += c if c > 0 else 0
    print(f"  {s}: {c}")
print(f"  TOTAL: {total_all}")

# === BACKLOG COMPOSITION ===
print("\n=== BACKLOG TICKET COMPOSITION ===")
backlog = get_issues("Backlog", max_results=100, fields=["summary","labels","issuetype","priority","created"])
backlog_issues = backlog.get("issues", [])

types = {}
labels = {}
for iss in backlog_issues:
    f = iss.get("fields", {})
    it = f.get("issuetype", {}).get("name", "unknown")
    types[it] = types.get(it, 0) + 1
    for l in f.get("labels", []):
        labels[l] = labels.get(l, 0) + 1

print(f"\nTotal backlog issues retrieved: {len(backlog_issues)}")
print("\nTicket types:")
for t, c in sorted(types.items(), key=lambda x: -x[1]):
    print(f"  {t}: {c}")

print("\nTop labels:")
for l, c in sorted(labels.items(), key=lambda x: -x[1])[:25]:
    print(f"  {l}: {c}")

# === ANALYSIS ===
ai_exec = sum(1 for i in backlog_issues if any(l.startswith("ai:") for l in i.get("fields",{}).get("labels",[])))
has_evidence = sum(1 for i in backlog_issues if any("evidence" in l.lower() or "sred" in l.lower() or "irap" in l.lower() for l in i.get("fields",{}).get("labels",[])))
governance = sum(1 for i in backlog_issues if any(l.startswith("governance:") or l.startswith("sred:") or l.startswith("irap:") for l in i.get("fields",{}).get("labels",[])))

print(f"\nAI-executable (ai:* labels): {ai_exec}")
print(f"Evidence/compliance labels: {has_evidence}")
print(f"Governance labels: {governance}")

# === LIST BACKLOG TICKETS ===
print("\n=== BACKLOG TICKETS (all retrieved) ===")
for iss in backlog_issues:
    key = iss["key"]
    f = iss.get("fields", {})
    summary = f.get("summary", "")[:100]
    it = f.get("issuetype", {}).get("name", "?")
    lbls = f.get("labels", [])
    print(f"  {key} | {it} | {summary}")
    if lbls:
        print(f"    labels: {', '.join(lbls[:6])}")

# === DONE TICKETS ===
print("\n=== DONE TICKETS ===")
done = get_issues("Done", max_results=50, fields=["summary","updated"])
done_issues = done.get("issues", [])
print(f"Total Done retrieved: {len(done_issues)}")
for iss in done_issues:
    f = iss.get("fields", {})
    print(f"  {iss['key']}: {f.get('summary','')[:80]}")

# Output raw JSON for further processing
output = {
    "state_counts": state_counts,
    "backlog_types": types,
    "backlog_labels": labels,
    "ai_executable_count": ai_exec,
    "evidence_label_count": has_evidence,
    "governance_label_count": governance,
    "backlog_issues": [
        {
            "key": i["key"],
            "summary": i.get("fields",{}).get("summary",""),
            "type": i.get("fields",{}).get("issuetype",{}).get("name","?"),
            "labels": i.get("fields",{}).get("labels",[]),
        }
        for i in backlog_issues
    ]
}
with open(os.path.expanduser("~/zen/zen-brain1/docs/05-OPERATIONS/evidence/backlog-raw.json"), "w") as f:
    json.dump(output, f, indent=2)
print(f"\nRaw data saved to docs/05-OPERATIONS/evidence/backlog-raw.json")
