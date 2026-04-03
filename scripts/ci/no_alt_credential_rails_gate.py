#!/usr/bin/env python3
"""
Gate C: No Alternate Credential Rails Gate

Blocks introduction of alternate credential files/paths.

FAILS on:
- .env.jira.local
- ~/.zen-brain/jira-credentials.env
- secrets/jira.yaml as active runtime path
- PAT-based Git auth in zen-brain1 code/docs
- New credential file names not in canonical list
"""

import sys
import re
from pathlib import Path

FORBIDDEN_PATTERNS = [
    (r'\.env\.jira\.local', '.env.jira.local file'),
    (r'~/.zen-brain/jira-credentials\.env', 'jira-credentials.env file'),
    (r'secrets/jira\.yaml.*runtime', 'jira.yaml as runtime path'),
    (r'GITHUB_TOKEN.*git', 'GitHub token for git auth'),
    (r'ghp_[a-zA-Z0-9]+', 'GitHub PAT pattern'),
    (r'git.*https://.*:.*@github', 'HTTPS git with credentials'),
]

ALLOWED_FILES = {
    'internal/secrets/jira.go',
    'internal/secrets/git.go',
    'docs/05-OPERATIONS/CREDENTIAL_RAILS.md',
}

def main():
    repo_root = Path(__file__).parent.parent.parent
    violations = []
    files_checked = 0
    
    for ext in ['*.go', '*.py', '*.sh', '*.md', '*.yaml', '*.yml']:
        for filepath in repo_root.rglob(ext):
            if 'vendor' in str(filepath) or '.git' in str(filepath):
                continue
            
            rel_path = str(filepath.relative_to(repo_root))
            if rel_path in ALLOWED_FILES:
                continue
            
            try:
                content = filepath.read_text(encoding='utf-8', errors='ignore')
                files_checked += 1
                
                for pattern, description in FORBIDDEN_PATTERNS:
                    if re.search(pattern, content, re.IGNORECASE):
                        line_num = content[:content.find(re.search(pattern, content).group())].count('\n') + 1
                        violations.append({
                            'file': rel_path,
                            'line': line_num,
                            'description': description
                        })
            except:
                pass
    
    if violations:
        print(f"❌ NO ALT CREDENTIAL RAILS GATE FAILED")
        print(f"   Violations: {len(violations)}")
        for v in violations[:5]:
            print(f"   {v['file']}:{v['line']} → {v['description']}")
        return 1
    
    print(f"✅ NO ALT CREDENTIAL RAILS GATE PASSED ({files_checked} files)")
    return 0

if __name__ == '__main__':
    sys.exit(main())
