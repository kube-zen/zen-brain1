#!/usr/bin/env python3
"""
Gate: No executable files outside scripts/ (except allowed exceptions).

Zen‑Brain rule: Executable files may only reside under scripts/.
Allowed exceptions:
- .githooks/pre‑commit
- Makefile (if marked executable, though not recommended)
- Entrypoints under cmd/ (Go binaries after build)
- Any file explicitly allowlisted in scripts/EXEC_OUTSIDE_SCRIPTS_ALLOWLIST.txt
"""

import os
import sys
import stat
import subprocess


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def load_allowlist(root: str) -> list[str]:
    """Load allowlist from scripts/EXEC_OUTSIDE_SCRIPTS_ALLOWLIST.txt."""
    allowlist_path = os.path.join(root, "scripts", "EXEC_OUTSIDE_SCRIPTS_ALLOWLIST.txt")
    allowed = [".githooks/pre-commit"]  # default allow
    if os.path.isfile(allowlist_path):
        with open(allowlist_path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.split("#")[0].strip()
                if line:
                    allowed.append(line)
    return allowed


def is_executable(path: str) -> bool:
    """Check if file is executable."""
    try:
        st = os.stat(path)
        return bool(st.st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH))
    except OSError:
        return False


def find_executable_violations(root: str) -> list[str]:
    """Return paths of executable files outside scripts/ (not allowlisted)."""
    allowed = load_allowlist(root)
    violations = []
    
    # Get tracked files only (ignore build artifacts)
    try:
        result = subprocess.run(
            ["git", "ls-files"],
            cwd=root,
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode != 0:
            # Fall back to walking the repo
            print("WARNING: git ls-files failed, falling back to full scan", file=sys.stderr)
            tracked = None
        else:
            tracked = set(result.stdout.strip().splitlines())
    except (subprocess.TimeoutExpired, FileNotFoundError):
        tracked = None
    
    # Walk the repo, skip .git
    for dirpath, _, filenames in os.walk(root):
        rel_dir = os.path.relpath(dirpath, root)
        if ".git" in rel_dir.split(os.sep):
            continue
        
        for f in filenames:
            full = os.path.join(dirpath, f)
            rel = os.path.relpath(full, root)
            
            # Skip if not tracked (build artifacts, temporary files)
            if tracked is not None and rel not in tracked:
                continue
            
            # Skip if under scripts/ (allowed location)
            if rel.startswith("scripts/"):
                continue
            
            # Skip if allowlisted
            if any(rel == a or (a.endswith("/") and rel.startswith(a)) for a in allowed):
                continue
            
            # Check executable bit
            if is_executable(full):
                violations.append(rel)
    
    return violations


def main() -> int:
    root = _repo_root()
    violations = find_executable_violations(root)
    
    if violations:
        print("ERROR: Executable files found outside scripts/ directory.", file=sys.stderr)
        print("Zen‑Brain rule: Executable files may only be placed under scripts/.", file=sys.stderr)
        print("Found the following violations:", file=sys.stderr)
        for v in violations:
            print(f"  • {v}", file=sys.stderr)
        print(file=sys.stderr)
        print("Either:", file=sys.stderr)
        print("  1. Move the file to scripts/", file=sys.stderr)
        print("  2. Remove executable bit (chmod -x)", file=sys.stderr)
        print("  3. Add the path to scripts/EXEC_OUTSIDE_SCRIPTS_ALLOWLIST.txt", file=sys.stderr)
        return 1
    
    print("✓ No executable sprawl gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())