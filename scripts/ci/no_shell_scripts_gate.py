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


def load_allowlist(root: str) -> list[str]:
    """Load allowlist from scripts/ALLOWED_SHELL_SCRIPTS.txt."""
    allowlist_path = os.path.join(root, "scripts", "ALLOWED_SHELL_SCRIPTS.txt")
    allowed = []
    if os.path.isfile(allowlist_path):
        with open(allowlist_path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.split("#")[0].strip()
                if line:
                    allowed.append(line)
    return allowed


def main() -> int:
    root = _repo_root()
    allowed = load_allowlist(root)
    sh_files = find_shell_scripts(root)
    
    # Filter out allowlisted files
    violations = [f for f in sh_files if f not in allowed]
    
    if violations:
        print("ERROR: Shell scripts (.sh) are not allowed in Zen‑Brain repository.", file=sys.stderr)
        print("Found the following .sh files:", file=sys.stderr)
        for f in violations:
            print(f"  • {f}", file=sys.stderr)
        print(file=sys.stderr)
        print("All scripts must be Python‑only and reside under scripts/.", file=sys.stderr)
        print("Delete or replace these files with Python equivalents.", file=sys.stderr)
        print(f"Or add to scripts/ALLOWED_SHELL_SCRIPTS.txt ({len(allowed)} currently allowed).", file=sys.stderr)
        return 1
    
    if sh_files:
        print(f"✓ No shell scripts gate: pass ({len(sh_files)} allowed via allowlist)")
    else:
        print("✓ No shell scripts gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())