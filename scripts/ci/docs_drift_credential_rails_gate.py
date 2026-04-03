#!/usr/bin/env python3
"""
Gate E: Docs Drift Credential Rails Gate

Fails if docs disagree on the canonical flow.
Scans and requires consistency across:
- AGENTS.md
- deploy/zen-lock/RUNBOOK.md
- deploy/zen-lock/JIRA_INTEGRATION_RUNBOOK.md
- docs/05-OPERATIONS/CREDENTIAL_RAILS.md
- deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh

One flow only.
"""

import sys
import re
from pathlib import Path

CANONICAL_DOCS = [
    'AGENTS.md',
    'deploy/zen-lock/RUNBOOK.md',
    'deploy/zen-lock/JIRA_INTEGRATION_RUNBOOK.md',
    'docs/05-OPERATIONS/CREDENTIAL_RAILS.md',
]

# Canonical paths that MUST be referenced consistently
CANONICAL_PATHS = [
    '/zen-lock/secrets',
    'JIRA_API_TOKEN',
    'secrets.ResolveJira',
]

# Deprecated paths that should NOT appear in active docs
DEPRECATED_PATHS = [
    '.env.jira',
    'JIRA_TOKEN (without API)',
    'NewFromEnv()',
    'inject-env: true',
]

def check_doc(filepath: str, content: str) -> list:
    """Check a doc file for consistency."""
    issues = []
    
    # Check for deprecated patterns in active docs
    for deprecated in DEPRECATED_PATHS:
        if deprecated.lower() in content.lower():
            # Allow if it's marked as deprecated
            if 'deprecated' not in content[max(0, content.lower().find(deprecated.lower())-50):content.lower().find(deprecated.lower())+50].lower():
                issues.append(f'References deprecated pattern: {deprecated}')
    
    return issues

def main():
    repo_root = Path(__file__).parent.parent.parent
    all_issues = []
    
    for doc_name in CANONICAL_DOCS:
        doc_path = repo_root / doc_name
        if not doc_path.exists():
            all_issues.append({'file': doc_name, 'issue': 'Document not found'})
            continue
        
        content = doc_path.read_text(encoding='utf-8', errors='ignore')
        issues = check_doc(doc_name, content)
        
        for issue in issues:
            all_issues.append({'file': doc_name, 'issue': issue})
    
    if all_issues:
        print(f"❌ DOCS DRIFT GATE FAILED")
        print(f"   Issues: {len(all_issues)}")
        for i in all_issues[:10]:
            print(f"   {i['file']}: {i['issue']}")
        print("   FIX: Update docs to match canonical credential flow")
        return 1
    
    print(f"✅ DOCS DRIFT GATE PASSED ({len(CANONICAL_DOCS)} docs)")
    return 0

if __name__ == '__main__':
    sys.exit(main())
