#!/usr/bin/env python3
"""
Gate B: No Secret Echo Gate

Blocks code/docs/scripts that encourage revealing secrets.

FAILS on:
- cat /zen-lock/secrets/JIRA_API_TOKEN
- kubectl get secret .*jsonpath=.*token
- echo $JIRA_API_TOKEN
- print(os.environ["JIRA_API_TOKEN"])
- env | grep JIRA
- set -x in scripts that touch credentials
- Logging token/email pairs

ALLOWED:
- Existence tests only
- Path/capability reporting only
"""

import sys
import os
import re
from pathlib import Path

# Patterns that indicate secret exposure (BAD)
FORBIDDEN_PATTERNS = [
    # Cat secret files
    (r'cat\s+/zen-lock/secrets/', 'Cat command on secret files'),
    (r'cat\s+\$.*JIRA.*TOKEN', 'Cat command on JIRA token variable'),
    
    # Kubectl secret extraction
    (r'kubectl\s+get\s+secret.*jsonpath.*token', 'Kubectl jsonpath token extraction'),
    (r'kubectl\s+get\s+secret.*-o\s+jsonpath.*JIRA', 'Kubectl jsonpath JIRA extraction'),
    
    # Echo/print actual secret VALUES (not env var names)
    (r'echo\s+"\$.*JIRA.*TOKEN"', 'Echo JIRA token variable value'),
    (r'echo\s+"\$.*JIRA.*API"', 'Echo JIRA API variable value'),
    (r'print\s*\(\s*os\.environ\["JIRA_API_TOKEN"\]\s*\)', 'Print JIRA token from os.environ'),
    (r'fmt\.Print.*jiraMaterial\.APIToken', 'Print JIRA token value in Go'),
    (r'log\.Print.*jiraMaterial\.APIToken', 'Log JIRA token value in Go'),
    (r'log\.Printf.*%s.*APIToken', 'Log JIRA token with format'),
    
    # Env dump
    (r'env\s*\|\s*grep\s+JIRA', 'Env dump with JIRA grep'),
    (r'env\s*\|\s*grep\s+TOKEN', 'Env dump with TOKEN grep'),
    
    # Set -x in credential contexts
    (r'set\s+-x.*jira', 'Set -x in JIRA context'),
    (r'set\s+-x.*credential', 'Set -x in credential context'),
]

# Allowed patterns (existence tests, capability reporting)
ALLOWED_PATTERNS = [
    # Existence tests
    (r'os\.Stat\(/zen-lock/secrets/', 'Existence check (allowed)'),
    (r'test\s+-f\s+/zen-lock/secrets/', 'File existence test (allowed)'),
    
    # Capability reporting (no values)
    (r'Jira.*Allowed.*%v', 'Capability boolean reporting (allowed)'),
    (r'Token.*Source.*%s', 'Token source path reporting (allowed)'),
]

def check_file(filepath: str, content: str) -> list:
    """Check a single file for forbidden patterns."""
    violations = []
    
    # First check for forbidden patterns
    for pattern, description in FORBIDDEN_PATTERNS:
        matches = re.finditer(pattern, content, re.IGNORECASE)
        for match in matches:
            line_num = content[:match.start()].count('\n') + 1
            line_content = content.split('\n')[line_num - 1].strip()
            
            # Skip if this is in a comment explaining what NOT to do
            if line_content.startswith('//') or line_content.startswith('#'):
                # Check if it's a "don't do this" comment
                if 'forbidden' in line_content.lower() or 'do not' in line_content.lower():
                    continue
                if 'deprecated' in line_content.lower():
                    continue
            
            violations.append({
                'file': filepath,
                'line': line_num,
                'pattern': pattern,
                'description': description,
                'context': line_content
            })
    
    return violations

def main():
    """Run the gate check."""
    repo_root = Path(__file__).parent.parent.parent
    violations = []
    files_checked = 0
    
    # Allowlist for gate scripts themselves and files being migrated
    allowlist = {
        'scripts/ci/no_secret_echo_gate.py',
        'scripts/ci/canonical_credential_access_gate.py',
        'cmd/factory-fill/main.go',
        'cmd/zen-brain/office.go',
        'cmd/zen-brain/main.go',
        'cmd/normalizer-demo/main.go',
        'cmd/scheduler/main.go',
        'cmd/admission-gate/main.go',
        # Break-glass runbooks (legitimate emergency access docs)
        'deploy/zen-lock/BREAK_GLASS_RUNBOOK.md',
        # AGENTS.md - User documentation (TODO: update to capability reporting)
        'AGENTS.md',
        # Rotation script (tests decryption, not production usage)
        'scripts/zen-lock-rotate.sh',
    }
    
    # Walk through all text files
    for ext in ['*.go', '*.py', '*.sh', '*.md', '*.yaml', '*.yml']:
        for filepath in repo_root.rglob(ext):
            # Skip vendor, binary files, and .git
            if 'vendor' in str(filepath):
                continue
            if '.git' in str(filepath):
                continue
            
            rel_path = str(filepath.relative_to(repo_root))
            
            # Skip allowlisted files
            if rel_path in allowlist:
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
        print(f"❌ NO SECRET ECHO GATE FAILED")
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
        print("   FIX: Remove secret exposure. Use capability reporting only.")
        print("   See: docs/05-OPERATIONS/CREDENTIAL_RAILS.md")
        
        return 1
    else:
        print(f"✅ NO SECRET ECHO GATE PASSED")
        print(f"   Files checked: {files_checked}")
        return 0

if __name__ == '__main__':
    sys.exit(main())
