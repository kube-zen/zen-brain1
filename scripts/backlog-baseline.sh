#!/usr/bin/env bash
# Phase 1: Capture Jira backlog baseline counts and ticket composition
set -euo pipefail

JIRA_URL="https://zen-mesh.atlassian.net"
JIRA_EMAIL="zen@kube-zen.io"
JIRA_TOKEN=$(tr -d '\r\n' < ~/zen/DONOTASKMOREFORTHISSHIT.txt)
PROJECT="ZB"

api() {
  local jql="$1"
  curl -s -u "${JIRA_EMAIL}:${JIRA_TOKEN}" \
    -H "Content-Type: application/json" \
    -G --data-urlencode "jql=${jql}" \
    --data-urlencode "maxResults=0" \
    "${JIRA_URL}/rest/api/3/search" 2>/dev/null
}

echo "=== JIRA STATE COUNTS ==="
for status in "Backlog" "Selected for Development" "In Progress" "PAUSED" "RETRYING" "TO_ESCALATE" "Done"; do
  jql="project = ${PROJECT} AND status = \"${status}\""
  total=$(api "${jql}" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('total','?'))" 2>/dev/null || echo "ERR")
  echo "${status}: ${total}"
done

echo ""
echo "=== TOTAL PROJECT TICKETS ==="
api "project = ${PROJECT}" | python3 -c "import json,sys; print(json.load(sys.stdin).get('total','?'))"

echo ""
echo "=== BACKLOG TICKET DETAILS (first 50) ==="
curl -s -u "${JIRA_EMAIL}:${JIRA_TOKEN}" \
  -H "Content-Type: application/json" \
  -G --data-urlencode "jql=project = ${PROJECT} AND status = Backlog ORDER BY created DESC" \
  --data-urlencode "maxResults=50" \
  --data-urlencode "fields=summary,labels,issuetype,created,priority" \
  "${JIRA_URL}/rest/api/3/search" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(f'Total backlog: {d[\"total\"]}')
print(f'Showing: {len(d.get(\"issues\",[]))}')
print()
labels_seen = {}
types_seen = {}
for iss in d.get('issues', []):
    key = iss['key']
    summary = iss['fields']['summary'][:80]
    itype = iss['fields']['issuetype']['name']
    lbls = iss['fields'].get('labels', [])
    pri = iss['fields'].get('priority', {}).get('name', 'none') if iss['fields'].get('priority') else 'none'
    types_seen[itype] = types_seen.get(itype, 0) + 1
    for l in lbls:
        labels_seen[l] = labels_seen.get(l, 0) + 1
    print(f'{key} | {itype} | {pri} | {summary}')
    if lbls:
        print(f'  labels: {\", \".join(lbls[:8])}')

print()
print('=== TICKET TYPES ===')
for t, c in sorted(types_seen.items(), key=lambda x: -x[1]):
    print(f'  {t}: {c}')

print()
print('=== TOP LABELS ===')
for l, c in sorted(labels_seen.items(), key=lambda x: -x[1])[:20]:
    print(f'  {l}: {c}')
" 2>/dev/null

echo ""
echo "=== DONE TICKETS ==="
curl -s -u "${JIRA_EMAIL}:${JIRA_TOKEN}" \
  -H "Content-Type: application/json" \
  -G --data-urlencode "jql=project = ${PROJECT} AND status = Done ORDER BY updated DESC" \
  --data-urlencode "maxResults=10" \
  --data-urlencode "fields=summary,labels,updated" \
  "${JIRA_URL}/rest/api/3/search" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(f'Total Done: {d[\"total\"]}')
for iss in d.get('issues', []):
    print(f'  {iss[\"key\"]}: {iss[\"fields\"][\"summary\"][:80]}')
" 2>/dev/null
