#!/usr/bin/env python3
"""
Gate: KB/QMD direction must stay Git‑first, not database‑first.

Zen‑Brain rule:
- Git is the source of truth for documentation.
- QMD adapter provides vector search over Git content.
- CockroachDB is a vector store (implementation detail), not the default QMD path.
- Custom DB ingestion must not be described as the 1.0 default.

Allowed default language:
- Git source of truth
- qmd adapter
- qmd refresh/orchestration
- optional Confluence mirror later

Disallowed default language:
- CockroachDB-backed QMD as 1.0 default
- custom graph-backed KB as 1.0 default
- Any phrasing that makes a database the primary source of truth.
"""

import os
import sys
import re


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def load_allowlist(root: str) -> set[str]:
    """Load allowlist from scripts/ci/kb_qmd_allowlist.txt."""
    allowlist_path = os.path.join(root, "scripts", "ci", "kb_qmd_allowlist.txt")
    allowed = set()
    if os.path.isfile(allowlist_path):
        with open(allowlist_path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.split("#")[0].strip()
                if line:
                    allowed.add(line)
    return allowed


def disallowed_phrases() -> list[tuple[str, str]]:
    """Return list of (phrase, description) for disallowed language."""
    return [
        (r"CockroachDB-backed QMD", "CockroachDB-backed QMD as default"),
        (r"custom DB ingestion as the default", "Custom DB ingestion as default"),
        (r"CockroachDB as the default QMD path", "CockroachDB as default QMD path"),
        (r"graph-backed KB as default", "Graph-backed KB as default"),
        (r"database.*source of truth", "Database as source of truth"),
        (r"primary source.*database", "Database as primary source"),
    ]


def scan_file(path: str, allowed: set[str]) -> list[tuple[str, int, str]]:
    """Scan a single file for disallowed phrases.
    Return list of (phrase, line_number, line_text)."""
    rel = os.path.relpath(path, _repo_root())
    if rel in allowed:
        return []
    
    violations = []
    try:
        with open(path, "r", encoding="utf-8") as f:
            lines = f.readlines()
    except UnicodeDecodeError:
        return []  # skip binary files
    
    for idx, line in enumerate(lines, 1):
        lower_line = line.lower()
        for phrase, desc in disallowed_phrases():
            # Case-insensitive search
            if re.search(phrase, lower_line, re.IGNORECASE):
                violations.append((desc, idx, line.rstrip()))
                break  # only report first phrase per line
    return violations


def main() -> int:
    root = _repo_root()
    allowed = load_allowlist(root)
    
    # Scan markdown files (skip .git and vendor to stay within timeout)
    md_files = []
    for dirpath, _, filenames in os.walk(root):
        if ".git" in dirpath or "vendor" in dirpath:
            continue
        for f in filenames:
            if f.endswith(".md"):
                md_files.append(os.path.join(dirpath, f))
    
    # Scan Go source files for comments (skip .git and vendor)
    go_files = []
    for dirpath, _, filenames in os.walk(root):
        if ".git" in dirpath or "vendor" in dirpath:
            continue
        for f in filenames:
            if f.endswith(".go"):
                go_files.append(os.path.join(dirpath, f))
    
    violations = []
    for file in md_files + go_files:
        violations.extend(scan_file(file, allowed))
    
    if violations:
        print("ERROR: KB/QMD direction gate violations:", file=sys.stderr)
        print("The following lines contain disallowed language about QMD defaults:", file=sys.stderr)
        for desc, line_num, line in violations:
            print(f"  • {desc} at line {line_num}: {line}", file=sys.stderr)
        print(file=sys.stderr)
        print("Allowed default language: Git source of truth, qmd adapter, qmd refresh/orchestration.", file=sys.stderr)
        print("Disallowed: CockroachDB‑backed QMD as 1.0 default, custom DB ingestion as default.", file=sys.stderr)
        return 1
    
    print("✓ KB/QMD direction gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())