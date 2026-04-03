#!/usr/bin/env python3
"""
Gate A: Canonical Credential Access Gate

Blocks all raw credential access outside an allowlist.

FAILS on:
- os.Getenv("JIRA_...")
- os.Getenv("JIRA_TOKEN")
- os.Getenv("JIRA_USERNAME")
- os.Getenv("GITHUB_...")
- os.Getenv("GH_...")
- NewFromEnv(
- .env.jira
- jira-credentials.env
- ~/.zen-lock/private-key.age
- ~/zen/DONOTASKMOREFORTHISSHIT.txt (outside bootstrap/rotation)
- ~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age (outside bootstrap)
- Direct secret reads outside canonical resolver

ALLOWED files:
- internal/secrets/jira.go
- internal/secrets/git.go
- internal/config/load.go (uses canonical resolver)
- bootstrap/rotation scripts (explicitly allowlisted)
- archived docs/tests (explicitly allowlisted)
"""

import sys
import os
import re
from pathlib import Path

# Allowlist of files that CAN access credentials directly
ALLOWLIST = {
    # Canonical resolvers
    "internal/secrets/jira.go",
    "internal/secrets/git.go",
    "internal/config/load.go",
    
    # Bootstrap/rotation scripts
    "scripts/install_jira_credentials.py",
    "scripts/load_jira_credentials.py",
    "scripts/generate_jira_secret.py",
    "deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh",
    
    # Archived docs/tests (read-only)
    "docs/05-OPERATIONS/CREDENTIAL_RAILS.md",
    "deploy/zen-lock/RUNBOOK.md",
    "deploy/zen-lock/JIRA_INTEGRATION_RUNBOOK.md",
    
    # TEMPORARY: Files being migrated to canonical resolver (TODO: fix these)
    # These files have existing direct env access that needs migration
    "cmd/factory-fill/main.go",
    "cmd/zen-brain/main.go",
    "cmd/zen-brain/office.go",
    "cmd/admission-gate/main.go",
    "cmd/finding-ticketizer/main.go",
    "cmd/normalizer-demo/main.go",
    "cmd/scheduler/main.go",
    "cmd/queue-steward/main.go",
    "cmd/roadmap-steward/main.go",
    "cmd/useful-batch/main.go",
    "cmd/foreman/main.go",
    "cmd/foreman/worker.go",
    
    # SPECIAL CASES:
    # internal/runtime/strictness.go - GITHUB_ACTIONS for CI detection (not credentials)
    "internal/runtime/strictness.go",
    # internal/office/jira/connector.go - Contains hard-fail NewFromEnv (not actual usage)
    "internal/office/jira/connector.go",
    # Gate scripts themselves document patterns but don't use them
    "scripts/ci/canonical_credential_access_gate.py",
    "scripts/ci/no_secret_echo_gate.py",
    "scripts/ci/no_alt_credential_rails_gate.py",
    "scripts/ci/zenlock_mount_only_gate.py",
    "scripts/ci/docs_drift_credential_rails_gate.py",
}

# Patterns that indicate direct credential access (BAD outside allowlist)
FORBIDDEN_PATTERNS = [
    # Direct env var access
    (r'os\.Getenv\("JIRA_', 'Direct JIRA_ env var access'),
    (r'os\.Getenv\("JIRA_TOKEN"\)', 'Direct JIRA_TOKEN access (use JIRA_API_TOKEN)'),
    (r'os\.Getenv\("JIRA_USERNAME"\)', 'Direct JIRA_USERNAME access (use JIRA_EMAIL)'),
    (r'os\.Getenv\("GITHUB_', 'Direct GITHUB_ env var access'),
    (r'os\.Getenv\("GH_', 'Direct GH_ env var access'),
    
    # Legacy constructors
    (r'NewFromEnv\(', 'NewFromEnv() constructor (use canonical resolver)'),
    
    # Legacy credential files
    (r'\.env\.jira', '.env.jira file (use canonical resolver)'),
    (r'jira-credentials\.env', 'jira-credentials.env file (use canonical resolver)'),
    (r'~/.zen-lock/private-key\.age', '~/.zen-lock/private-key.age (use canonical bootstrap)'),
    (r'~/zen/DONOTASKMOREFORTHISSHIT\.txt', 'DONOTASK file (bootstrap only)'),
    (r'~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT\.age', 'ZENBRAIN private key (bootstrap only)'),
]

def check_file(filepath: str, content: str) -> list:
    """Check a single file for forbidden patterns."""
    violations = []
    
    for pattern, description in FORBIDDEN_PATTERNS:
        matches = re.finditer(pattern, content)
        for match in matches:
            # Get line number
            line_num = content[:match.start()].count('\n') + 1
            violations.append({
                'file': filepath,
                'line': line_num,
                'pattern': pattern,
                'description': description,
                'context': content.split('\n')[line_num - 1].strip()
            })
    
    return violations

def main():
    """Run the gate check."""
    repo_root = Path(__file__).parent.parent.parent
    violations = []
    files_checked = 0
    
    # Walk through Go and Python files
    for ext in ['*.go', '*.py', '*.sh']:
        for filepath in repo_root.rglob(ext):
            # Skip vendor, test files, and allowlisted files
            if 'vendor' in str(filepath):
                continue
            if filepath.name.endswith('_test.go'):
                continue
            if '.git' in str(filepath):
                continue
            
            rel_path = str(filepath.relative_to(repo_root))
            
            # Skip allowlisted files
            if rel_path in ALLOWLIST:
                continue
            
            # Check file
            try:
                content = filepath.read_text(encoding='utf-8', errors='ignore')
                files_checked += 1
                file_violations = check_file(rel_path, content)
                violations.extend(file_violations)
            except Exception as e:
                pass  # Skip unreadable files
    
    # Report results
    if violations:
        print(f"❌ CANONICAL CREDENTIAL ACCESS GATE FAILED")
        print(f"   Files checked: {files_checked}")
        print(f"   Violations found: {len(violations)}")
        print()
        
        for v in violations[:10]:  # Show first 10
            print(f"   {v['file']}:{v['line']}")
            print(f"      → {v['description']}")
            print(f"      → {v['context']}")
            print()
        
        if len(violations) > 10:
            print(f"   ... and {len(violations) - 10} more violations")
        
        print()
        print("   FIX: Use secrets.ResolveJira() or secrets.ResolveGit() instead.")
        print("   See: docs/05-OPERATIONS/CREDENTIAL_RAILS.md")
        
        return 1
    else:
        print(f"✅ CANONICAL CREDENTIAL ACCESS GATE PASSED")
        print(f"   Files checked: {files_checked}")
        return 0

if __name__ == '__main__':
    sys.exit(main())
