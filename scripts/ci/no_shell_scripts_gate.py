#!/usr/bin/env python3
"""
Gate: No shell scripts anywhere in the repository.

Zen‑Brain rule: No .sh files are allowed anywhere in the repo.
All scripts must be Python‑only and reside under scripts/.
"""

import os
import sys
import subprocess


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def find_shell_scripts(root: str) -> list[str]:
    """Return paths of all .sh files (relative to root). Skip .git and vendor."""
    sh_files = []
    for dirpath, _, filenames in os.walk(root):
        if ".git" in dirpath or "vendor" in dirpath:
            continue
        for f in filenames:
            if f.endswith(".sh"):
                full = os.path.join(dirpath, f)
                rel = os.path.relpath(full, root)
                sh_files.append(rel)
    return sh_files


def main() -> int:
    root = _repo_root()
    sh_files = find_shell_scripts(root)
    
    if sh_files:
        print("ERROR: Shell scripts (.sh) are not allowed in Zen‑Brain repository.", file=sys.stderr)
        print("Found the following .sh files:", file=sys.stderr)
        for f in sh_files:
            print(f"  • {f}", file=sys.stderr)
        print(file=sys.stderr)
        print("All scripts must be Python‑only and reside under scripts/.", file=sys.stderr)
        print("Delete or replace these files with Python equivalents.", file=sys.stderr)
        return 1
    
    print("✓ No shell scripts gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())