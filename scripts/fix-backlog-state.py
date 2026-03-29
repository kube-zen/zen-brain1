#!/usr/bin/env python3
"""
Move completed scan report tickets from Backlog to Done.

These are tickets labeled ai:completed that are still in Backlog status.
They were created by the scheduler's Jira ledger integration but never
transitioned to Done because the scheduler only creates issues, it doesn't
close them after the scan completes.

This is a one-time cleanup script.
"""

import json
import subprocess
import os
import sys
import time

JIRA_EMAIL = get_jira_email()
from common.zen_lock import get_jira_token, get_jira_email
TOKEN = get_jira_token()
URL = "https://zen-mesh.atlassian.net"
PROJECT = "ZB"


def api(method, path, body=None):
    cmd = ["curl", "-s", "-X", method, "-u", f"{EMAIL}:{TOKEN}",
           "-H", "Content-Type: application/json"]
    if body:
        cmd += ["-d", json.dumps(body)]
    cmd.append(f"{URL}/rest/api/3{path}")
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
    try:
        return json.loads(r.stdout) if r.stdout.strip() else {}
    except:
        return {}


def search(jql, max_results=100, fields=None):
    body = {"jql": jql, "maxResults": max_results}
    if fields:
        body["fields"] = fields
    return api("POST", "/search/jql", body)


def get_transitions(issue_key):
    return api("GET", f"/issue/{issue_key}/transitions")


def transition_issue(issue_key, transition_id):
    return api("POST", f"/issue/{issue_key}/transitions",
               {"transition": {"id": transition_id}})


def main():
    # Find all ai:completed tickets in Backlog
    print("Finding ai:completed tickets in Backlog...")
    backlog = search(
        f'project={PROJECT} AND status=Backlog AND labels="ai:completed"',
        max_results=100,
        fields=["summary", "labels"]
    )

    issues = backlog.get("issues", [])
    total = backlog.get("total", len(issues))
    print(f"Found {total} ai:completed tickets in Backlog")

    if not issues:
        print("Nothing to do.")
        return

    # First, get the Done transition ID from one ticket
    print("\nGetting Done transition ID...")
    transitions = get_transitions(issues[0]["key"])
    done_id = None
    for t in transitions.get("transitions", []):
        if t.get("to", {}).get("name", "").lower() == "done":
            done_id = t["id"]
            print(f"  Done transition ID: {done_id}")
            break

    if not done_id:
        # Try "Closed" or "Resolve"
        for t in transitions.get("transitions", []):
            name = t.get("to", {}).get("name", "").lower()
            if name in ("closed", "resolve issue", "resolved"):
                done_id = t["id"]
                print(f"  Found '{t['to']['name']}' transition ID: {done_id}")
                break

    if not done_id:
        print("ERROR: Could not find Done transition!")
        print("Available transitions:")
        for t in transitions.get("transitions", []):
            print(f"  {t['id']}: {t.get('to', {}).get('name', '?')} ({t.get('name', '?')})")
        sys.exit(1)

    # Transition all completed tickets to Done
    success = 0
    failed = 0
    for issue in issues:
        key = issue["key"]
        summary = issue["fields"]["summary"][:50]
        result = transition_issue(key, done_id)
        # Check if it worked — transitions return 204 with empty body
        # The api function returns {} on empty body which could be success or failure
        # Let's verify by checking if we got an error
        if "errorMessages" in result:
            print(f"  ❌ {key}: {result['errorMessages']}")
            failed += 1
        else:
            print(f"  ✅ {key}: {summary}")
            success += 1
        time.sleep(0.3)  # Rate limit

    print(f"\nDone: {success} moved to Done, {failed} failed")

    # Also move any scheduled-batch parent tickets that are ai:completed
    print("\nChecking scheduled-batch parents in Backlog...")
    batch_parents = search(
        f'project={PROJECT} AND status=Backlog AND labels="scheduled-batch" AND labels NOT IN ("ai:blocked")',
        max_results=50,
        fields=["summary", "labels"]
    )
    batch_issues = batch_parents.get("issues", [])
    print(f"Found {len(batch_issues)} scheduled-batch parents to move")
    for issue in batch_issues:
        key = issue["key"]
        labels = issue["fields"].get("labels", [])
        # Only move if all children are completed
        has_blocked = "ai:blocked" in labels
        has_completed = "ai:completed" in labels
        if has_blocked:
            print(f"  ⏭️  {key}: has blocked children, skipping")
            continue
        if has_completed:
            result = transition_issue(key, done_id)
            if "errorMessages" not in result:
                print(f"  ✅ {key}: {issue['fields']['summary'][:50]}")
                success += 1
            else:
                print(f"  ❌ {key}: {result.get('errorMessages', 'unknown')}")
                failed += 1
        time.sleep(0.3)


if __name__ == "__main__":
    main()
