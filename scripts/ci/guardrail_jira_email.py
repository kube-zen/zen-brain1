#!/usr/bin/env python3
"""CI guardrail: enforce canonical Jira email.

Scans the repo for occurrences of the WRONG Jira email (zen@kube-zen.io)
and flags them — the canonical email is zen@zen-mesh.io.

Exit code 0 = clean
Exit code 1 = forbidden email found in non-exempt files
"""
import os
import re
import sys

CANONICAL_EMAIL = "zen@zen-mesh.io"
FORBIDDEN_EMAIL = "zen@kube-zen.io"

# Files/patterns where the old email is allowed (historical reference, archive)
EXEMPT_PATTERNS = [
    r"99-ARCHIVE/",
    r"guardrail_jira_email",                 # This guardrail itself
    r"bootstrap-jira-zenlock-from-local",    # Legacy bootstrap script
    r"JIRA_INTEGRATION_RUNBOOK",             # Legacy runbook
    r"BREAK_GLASS_RUNBOOK",                  # Break-glass (references old keys)
    r"CLUSTER_RECOVERY",                     # Cluster recovery doc
]

# Line patterns where the old email is allowed
EXEMPT_LINE_PATTERNS = [
    r"HISTORICAL",
    r"was\s+(wrong|incorrect|old|legacy)",
    r"previously",
    r"canonical.*was",
    r"changed\s+from",
    r"migration",
    r"FORBIDDEN",
    r"WRONG",
    r"NOT\s+`?zen@kube-zen",
    r"blocks\s+`?zen@kube-zen",
    r"causes\s+401",
    r"guardrail.*blocks",
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

            ext = os.path.splitext(fname)[1].lower()
            if ext in {".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".pdf",
                       ".zip", ".gz", ".tar", ".bin", ".exe", ".so", ".o", ".a",
                       ".gguf", ".age"}:
                continue

            # Skip compiled binaries (false positives from embedded strings)
            if fname in ("roadmap-steward", "queue-steward", "scheduler",
                         "factory-fill", "useful-batch", "zen-brain",
                         "startup-logging", "test-jira-myself"):
                continue

            if "_test.go" in fname:
                continue

            try:
                with open(fpath, "r", encoding="utf-8", errors="replace") as f:
                    file_exempt = is_exempt(relpath)
                    for lineno, line in enumerate(f, 1):
                        if FORBIDDEN_EMAIL in line:
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
        print(f"Fix: Replace with {CANONICAL_EMAIL} or add exemption for historical references.")
        sys.exit(1)
    else:
        print(f"✅ No forbidden Jira email ('{FORBIDDEN_EMAIL}'). Canonical: {CANONICAL_EMAIL}")
        sys.exit(0)

if __name__ == "__main__":
    main()
