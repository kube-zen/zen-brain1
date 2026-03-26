#!/usr/bin/env python3
"""CI guardrail: block forbidden Jira email reintroduction.

Scans the repo for occurrences of zen@zen-mesh.io outside of explicitly
allowed contexts (migration notes, "wrong/forbidden" documentation).

Exit code 0 = clean
Exit code 1 = forbidden email found in non-exempt files
"""
import os
import re
import sys

FORBIDDEN_EMAIL = "zen@zen-mesh.io"
CANONICAL_EMAIL = "zen@kube-zen.io"

# Files/patterns where the forbidden email is allowed (as historical reference)
EXEMPT_PATTERNS = [
    r"AGENTS\.md",                          # Contains "FORBIDDEN" note about the wrong email
    r"CLAUDE\.md",                           # Contains "NOT zen@zen-mesh.io" note
    r"REAL_JIRA_INTEGRATION_REPORT",          # Historical proof doc (fully exempt — annotated at top)
    r"CURRENT_STATE\.md",                    # Contains 401 failure note
    r"guardrail_jira_email",                 # This guardrail script itself
]

# Line patterns where the forbidden email is allowed
EXEMPT_LINE_PATTERNS = [
    r"WRONG",
    r"FORBIDDEN",
    r"NOT\s+zen@zen-mesh",
    r"causes\s+401",
    r"HISTORICAL",
    r"wrong.*email",
    r"forbidden",
    r"migration\s+note",
    r"HTTP\s+401",
    r"FAIL.*401",
]

def is_exempt(filepath):
    for pattern in EXEMPT_PATTERNS:
        if re.search(pattern, filepath):
            return True
    return False

def is_line_exempt(line):
    for pattern in EXEMPT_LINE_PATTERNS:
        if re.search(pattern, line, re.IGNORECASE):
            return True
    return False

def scan_repo(repo_root):
    violations = []
    skip_dirs = {".git", "vendor", "node_modules", ".cache", "__pycache__"}

    for root, dirs, files in os.walk(repo_root):
        dirs[:] = [d for d in dirs if d not in skip_dirs]

        for fname in files:
            fpath = os.path.join(root, fname)
            relpath = os.path.relpath(fpath, repo_root)

            # Skip binary files and images
            ext = os.path.splitext(fname)[1].lower()
            if ext in {".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".pdf",
                       ".zip", ".gz", ".tar", ".bin", ".exe", ".so", ".o", ".a",
                       ".gguf", ".age"}:
                continue

            # Skip test files that specifically test wrong email rejection
            if "_test.go" in fname:
                continue

            try:
                with open(fpath, "r", encoding="utf-8", errors="replace") as f:
                    file_exempt = is_exempt(relpath)
                    for lineno, line in enumerate(f, 1):
                        if FORBIDDEN_EMAIL in line:
                            # Fully exempt files skip all checks
                            if file_exempt or is_line_exempt(line):
                                continue
                            violations.append({
                                "file": relpath,
                                "line": lineno,
                                "content": line.strip(),
                            })
            except (IOError, OSError):
                continue

    return violations

def main():
    repo_root = os.environ.get("REPO_ROOT", ".")
    violations = scan_repo(repo_root)

    if violations:
        print(f"❌ FORBIDDEN Jira email '{FORBIDDEN_EMAIL}' found in {len(violations)} location(s):")
        print(f"   Canonical email is: {CANONICAL_EMAIL}")
        print()
        for v in violations:
            print(f"   {v['file']}:{v['line']}: {v['content'][:100]}")
        print()
        print("Fix: Replace with zen@kube-zen.io or add exemption if this is a historical/migration reference.")
        sys.exit(1)
    else:
        print(f"✅ No forbidden Jira email ('{FORBIDDEN_EMAIL}') found. Canonical: {CANONICAL_EMAIL}")
        sys.exit(0)

if __name__ == "__main__":
    main()
