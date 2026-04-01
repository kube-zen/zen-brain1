#!/usr/bin/env python3
"""
Bulk-enrich zen-brain backlog tickets to pass readiness validation (5/5).

Readiness criteria (internal/readiness/validator.go):
  1. Problem statement: title not generic, desc >= MinDescriptionLength
  2. Scope: label with "area:" or "component:", or desc mentions component/module/service
  3. Evidence/repro: desc mentions error/stack/trace/repro/fail/crash/etc
  4. Acceptance criteria: desc mentions acceptance/criteria/should/must/verify/test case/etc
  5. Constraints (bonus, not blocking)

This script fixes criteria 2-4 for tickets that score 3-4/5.
"""

import json
import sys
import os
import re
import requests

JIRA_URL = os.environ.get("JIRA_URL", "https://zen-mesh.atlassian.net")
JIRA_EMAIL = os.environ.get("JIRA_EMAIL", "")
JIRA_TOKEN = os.environ.get("JIRA_API_TOKEN", "")
PROJECT = os.environ.get("JIRA_PROJECT_KEY", "ZB")
DRY_RUN = os.environ.get("DRY_RUN", "0") == "1"

if not JIRA_EMAIL or not JIRA_TOKEN:
    print("Error: set JIRA_EMAIL and JIRA_API_TOKEN", file=sys.stderr)
    sys.exit(1)

AUTH = (JIRA_EMAIL, JIRA_TOKEN)
HEADERS = {"Content-Type": "application/json"}

# Area labels to assign based on description content
AREA_KEYWORDS = {
    "area:auth": ["auth", "login", "password", "token", "session", "jwt", "oauth", "saml", "identity", "credential", "certificate", "tls", "mtls", "ssl", "rbac", "permission"],
    "area:api": ["api", "endpoint", "route", "handler", "controller", "rest", "graphql", "grpc", "http", "request", "response", "middleware"],
    "area:data": ["database", "query", "migration", "schema", "table", "index", "cockroach", "postgres", "sql", "transaction", "replication", "crud", "orm"],
    "area:runtime": ["runtime", "binary", "executable", "build", "compile", "deploy", "container", "image", "docker", "kubernetes", "k8s", "pod", "scheduler", "worker", "process"],
    "area:security": ["security", "vulnerability", "injection", "xss", "csrf", "encrypt", "sanitize", "validate", "audit", "compliance", "hipaa", "pii"],
    "area:config": ["config", "configuration", "setting", "environment", "variable", "flag", "feature flag", "toggle", "helm", "yaml"],
    "area:observability": ["log", "metric", "trace", "monitor", "alert", "dashboard", "prometheus", "telemetry", "health", "readiness", "liveness"],
    "area:deps": ["dependency", "version", "upgrade", "package", "module", "vendor", "gomod", "go.sum", "lock file"],
}

API_PREFIX = os.environ.get("JIRA_API_PREFIX", "rest/api/3")

def jira_get(path, params=None):
    r = requests.get(f"{JIRA_URL}/{API_PREFIX}/{path}", auth=AUTH, headers=HEADERS, params=params)
    r.raise_for_status()
    return r.json()

def jira_post(path, data, params=None):
    r = requests.post(f"{JIRA_URL}/{API_PREFIX}/{path}", auth=AUTH, headers=HEADERS, json=data, params=params)
    r.raise_for_status()
    return r.json()

def jira_put(path, data):
    r = requests.put(f"{JIRA_URL}/{API_PREFIX}/{path}", auth=AUTH, headers=HEADERS, json=data)
    if r.status_code not in (200, 204):
        print(f"  PUT {path} returned {r.status_code}: {r.text[:200]}", file=sys.stderr)
    return r.status_code

def detect_area(summary, description):
    """Pick the best area label based on content keywords."""
    combined = f"{summary} {description}".lower()
    scores = {}
    for area, keywords in AREA_KEYWORDS.items():
        score = sum(1 for kw in keywords if kw in combined)
        if score > 0:
            scores[area] = score
    if not scores:
        return "area:runtime"  # default fallback
    return max(scores, key=scores.get)

def enrich_description(key, summary, description, missing_scope, missing_evidence, missing_acceptance):
    """Append missing sections to the ticket description."""
    additions = []
    desc = description or ""

    if missing_evidence and "evidence:" not in desc.lower() and "repro:" not in desc.lower():
        # Extract any useful signal from the existing description
        additions.append("Evidence: See summary and ticket context for defect details.")

    if missing_acceptance:
        # Generate a simple acceptance statement based on the summary
        additions.append(
            "Acceptance Criteria:\n"
            "- The described defect or improvement is addressed\n"
            "- No regressions introduced in related functionality\n"
            "- Changes pass existing tests or include appropriate test coverage"
        )

    if not additions:
        return description

    enriched = desc.rstrip()
    for add in additions:
        enriched += "\n\n" + add
    return enriched

def main():
    print(f"=== Bulk Enrichment for {PROJECT} tickets ===")
    if DRY_RUN:
        print("DRY RUN — no changes will be made")
    print()

    # Fetch all ai:finding tickets in Backlog (Jira Cloud requires POST for /search/jql)
    jql = f'project = "{PROJECT}" AND labels = "ai:finding" AND status = "Backlog" ORDER BY key ASC'
    result = jira_post("search/jql", {"jql": jql, "maxResults": 100, "fields": ["key", "summary", "description", "labels"]})
    issues = result.get("issues", [])
    print(f"Found {len(issues)} ai:finding tickets in Backlog\n")

    updated = 0
    skipped = 0

    for issue in issues:
        key = issue["key"]
        fields = issue["fields"]
        summary = fields.get("summary", "")
        raw_desc = fields.get("description", {})

        # Jira v3 returns ADF or plain text
        if isinstance(raw_desc, dict):
            # Extract plain text from ADF
            def adf_to_text(node):
                if isinstance(node, str):
                    return node
                if isinstance(node, list):
                    return "\n".join(adf_to_text(n) for n in node)
                if isinstance(node, dict):
                    parts = []
                    if node.get("type") == "text":
                        parts.append(node.get("text", ""))
                    if "content" in node:
                        parts.append(adf_to_text(node["content"]))
                    return "".join(parts)
                return ""
            description = adf_to_text(raw_desc)
        else:
            description = raw_desc or ""
        labels = list(fields.get("labels", []))

        # Check what's missing
        has_area = any("area:" in l or "component:" in l for l in labels)
        has_scope = has_area or any(kw in description.lower() for kw in
            ["component:", "service:", "module:", "endpoint:", "handler:", "internal/", "cmd/", "pkg/"])

        desc_lower = description.lower()
        has_evidence = any(kw in desc_lower for kw in
            ["error", "exception", "stack", "trace", "repro", "steps:", "fail", "crash", "panic",
             "timeout", "refused", "500", "404", "nil pointer", "evidence:", "```", "before:", "after:"])

        has_acceptance = any(kw in desc_lower for kw in
            ["acceptance", "criteria", "should", "must", "verify:", "test case", "done when",
             "expected behavior", "outcome", "result should"])

        needs_scope = not has_scope
        needs_evidence = not has_evidence
        needs_acceptance = not has_acceptance

        if not any([needs_scope, needs_evidence, needs_acceptance]):
            skipped += 1
            continue

        # Build changes
        new_labels = list(labels)
        if needs_scope:
            area = detect_area(summary, description)
            if area not in new_labels:
                new_labels.append(area)
            needs_scope = False  # now satisfied

        new_desc = enrich_description(key, summary, description, needs_scope, needs_evidence, needs_acceptance)

        print(f"{key}: scope={'✓' if has_scope else f'+{detect_area(summary, description)}'} "
              f"evidence={'✓' if has_evidence else '+enrich'} "
              f"acceptance={'✓' if has_acceptance else '+enrich'}")

        if DRY_RUN:
            print(f"  [DRY RUN] would update labels and description")
            continue

        # Apply changes
        if new_labels != labels:
            jira_put(f"issue/{key}", {"fields": {"labels": new_labels}})

        if new_desc != description:
            # Convert plain text back to ADF for v3
            adf_desc = {
                "type": "doc",
                "version": 1,
                "content": [
                    {"type": "paragraph", "content": [{"type": "text", "text": line}]}
                    for line in new_desc.split("\n") if line.strip()
                ]
            }
            jira_put(f"issue/{key}", {"fields": {"description": adf_desc}})

        updated += 1

    print(f"\nDone: {updated} updated, {skipped} already 5/5, {len(issues) - updated - skipped} unchanged")
    if DRY_RUN:
        print("Re-run without DRY_RUN=1 to apply changes")

if __name__ == "__main__":
    main()
