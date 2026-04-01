#!/usr/bin/env python3
"""Bulk delete Jira tickets by label. DESTRUCTIVE - no undo."""
import requests, sys, os, time

JIRA_URL = os.environ.get("JIRA_URL", "https://zen-mesh.atlassian.net")
JIRA_EMAIL = os.environ.get("JIRA_EMAIL", "")
JIRA_API_TOKEN = os.environ.get("JIRA_API_TOKEN", "")

AUTH = (JIRA_EMAIL, JIRA_API_TOKEN)
HEADERS = {"Content-Type": "application/json"}

def fetch_all(jql):
    """Paginated fetch of all issues matching JQL."""
    issues = []
    token = None
    while True:
        body = {"jql": jql, "maxResults": 100, "fields": ["key", "status"]}
        if token:
            body["nextPageToken"] = token
        r = requests.post(f"{JIRA_URL}/rest/api/3/search/jql", auth=AUTH, json=body, headers=HEADERS)
        data = r.json()
        issues.extend(data.get("issues", []))
        if data.get("isLast", True):
            break
        token = data.get("nextPageToken")
    return issues

def delete_issue(key):
    r = requests.delete(f"{JIRA_URL}/rest/api/3/issue/{key}?deleteSubtasks=true", auth=AUTH, headers=HEADERS)
    if r.status_code in (200, 204):
        return True
    if r.status_code == 403:
        print(f"  403 forbidden on {key} — may need project admin permissions")
        return False
    if r.status_code == 404:
        return True  # already gone
    print(f"  DELETE {key} returned {r.status_code}: {r.text[:150]}")
    return False

def main():
    labels = sys.argv[1:] if len(sys.argv) > 1 else ["hourly-scan", "quad-hourly-summary", "daily-sweep"]
    
    total_deleted = 0
    total_failed = 0
    
    for label in labels:
        print(f"\n=== Deleting tickets with label: {label} ===")
        issues = fetch_all(f'project=ZB AND labels="{label}"')
        print(f"Found {len(issues)} tickets")
        
        for i, issue in enumerate(issues):
            key = issue["key"]
            status = issue["fields"].get("status", {}).get("name", "?")
            if delete_issue(key):
                total_deleted += 1
            else:
                total_failed += 1
            
            if (i + 1) % 50 == 0:
                print(f"  ... {i+1}/{len(issues)} processed ({total_deleted} deleted)")
            # Small delay to avoid rate limiting
            if (i + 1) % 20 == 0:
                time.sleep(1)
        
        print(f"  {label}: {len(issues)} found, {total_deleted} deleted, {total_failed} failed")
    
    print(f"\n=== TOTAL: {total_deleted} deleted, {total_failed} failed ===")

if __name__ == "__main__":
    main()
