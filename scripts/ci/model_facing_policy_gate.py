#!/usr/bin/env python3
"""
Gate: AGENTS.md and WORKFLOW.md must be advisory, not canonical.

Zen‑Brain rule:
- AGENTS.md and WORKFLOW.md are advisory model‑facing convenience documents.
- They are derived summaries, not canonical source of truth.
- Canonical truth lives in code/contracts, structured config, docs architecture/design.

This gate fails if AGENTS.md or WORKFLOW.md contain language that frames them as:
- canonical source of truth
- definitive reference
- authoritative specification
- required reading (beyond advisory)
"""

import os
import sys
import re


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def disallowed_phrases() -> list[tuple[str, str]]:
    """Return list of (phrase, description) for disallowed language."""
    return [
        (r"this document is the canonical source", "Claims this document is canonical"),
        (r"AGENTS.md is the definitive", "Claims AGENTS.md is definitive"),
        (r"WORKFLOW.md is the authoritative", "Claims WORKFLOW.md is authoritative"),
        (r"must be followed", "Imperative requirement"),
        (r"required reading", "Required reading"),
        (r"official specification", "Claims to be official"),
        (r"final word", "Claims to be final word"),
        (r"this document defines", "Defines (implies authority)"),
        (r"this document specifies", "Specifies (implies authority)"),
    ]


def allowed_patterns() -> list[str]:
    """Return regex patterns that indicate the line is describing where the canonical source is, not claiming itself."""
    return [
        r"canonical source of truth is",
        r"canonical source of truth are",
        r"canonical source of truth lives",
        r"canonical source of truth resides",
        r"source of truth.*:",
        r"> \*\*Advisory Only\*\*",
        r"advisory only",
        r"this file is advisory",
        r"this file summarizes",
        r"always prefer the canonical",
    ]


def check_file(path: str) -> list[tuple[str, int, str]]:
    """Check a single file for disallowed phrases.
    Return list of (phrase_desc, line_number, line_text)."""
    if not os.path.isfile(path):
        return []
    violations = []
    allowed = allowed_patterns()
    try:
        with open(path, "r", encoding="utf-8") as f:
            lines = f.readlines()
    except UnicodeDecodeError:
        return []
    
    for idx, line in enumerate(lines, 1):
        lower_line = line.lower()
        # Skip lines that match allowed patterns (they are okay)
        skip = False
        for pat in allowed:
            if re.search(pat, lower_line):
                skip = True
                break
        if skip:
            continue
        # Check for disallowed phrases
        for phrase, desc in disallowed_phrases():
            if re.search(phrase, lower_line):
                violations.append((desc, idx, line.rstrip()))
                break
    return violations


def main() -> int:
    root = _repo_root()
    files_to_check = [
        os.path.join(root, "AGENTS.md"),
        os.path.join(root, "WORKFLOW.md"),
    ]
    
    violations = []
    for file in files_to_check:
        violations.extend(check_file(file))
    
    if violations:
        print("ERROR: Model‑facing file policy gate violations:", file=sys.stderr)
        print("AGENTS.md or WORKFLOW.md contain language that frames them as canonical.", file=sys.stderr)
        for desc, line_num, line in violations:
            print(f"  • {desc} at line {line_num}: {line}", file=sys.stderr)
        print(file=sys.stderr)
        print("AGENTS.md and WORKFLOW.md must be advisory only.", file=sys.stderr)
        print("Canonical truth lives in code/contracts, structured config, docs architecture/design.", file=sys.stderr)
        return 1
    
    print("✓ Model‑facing file policy gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())