#!/usr/bin/env python3
"""
Gate D: ZenLock Mount Only Gate

In zen-brain1 manifests, fails on:
- zen-lock/inject-env: "true" for Jira/Git secrets
- envFrom: secretRef for Jira creds in cluster runtime
- Direct secretRef-based Jira runtime injection
- Any non-/zen-lock/secrets runtime path for Jira in K8s manifests
"""

import sys
import re
from pathlib import Path

def check_yaml_file(filepath: str, content: str) -> list:
    violations = []
    
    # Check for inject-env: "true"
    if re.search(r'zen-lock/inject-env:\s*["\']?true["\']?', content):
        violations.append(('inject-env: true found (forbidden for Jira/Git)', 0))
    
    # Check for envFrom secretRef with jira
    if re.search(r'envFrom:.*secretRef.*jira', content, re.IGNORECASE):
        violations.append(('envFrom secretRef for Jira (use ZenLock mount)', 0))
    
    # Check for non-canonical secret paths
    if re.search(r'mountPath:.*secrets.*jira', content) and '/zen-lock/secrets' not in content:
        violations.append(('Non-canonical secret mount path', 0))
    
    return violations

def main():
    repo_root = Path(__file__).parent.parent.parent
    violations = []
    files_checked = 0
    
    for filepath in repo_root.rglob('*.yaml'):
        if 'vendor' in str(filepath) or '.git' in str(filepath):
            continue
        if 'deploy' not in str(filepath) and 'k8s' not in str(filepath) and 'manifest' not in str(filepath):
            continue
        
        rel_path = str(filepath.relative_to(repo_root))
        files_checked += 1
        
        try:
            content = filepath.read_text(encoding='utf-8', errors='ignore')
            file_violations = check_yaml_file(rel_path, content)
            for desc, line in file_violations:
                violations.append({'file': rel_path, 'line': line, 'description': desc})
        except:
            pass
    
    if violations:
        print(f"❌ ZENLOCK MOUNT ONLY GATE FAILED")
        print(f"   Violations: {len(violations)}")
        for v in violations[:5]:
            print(f"   {v['file']} → {v['description']}")
        print("   FIX: Use ZenLock mount at /zen-lock/secrets only")
        return 1
    
    print(f"✅ ZENLOCK MOUNT ONLY GATE PASSED ({files_checked} manifests)")
    return 0

if __name__ == '__main__':
    sys.exit(main())
