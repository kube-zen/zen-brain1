#!/usr/bin/env python3
"""
Gate: Python scripts must be placed under scripts/.

Zen‑Brain rule: All executable Python scripts must reside under scripts/.
Exceptions:
- Test files (under *_test.py) are allowed anywhere
- Non‑executable Python modules (e.g., internal/**, pkg/**) are allowed
- Entrypoints under cmd/ are allowed
"""

import os
import sys
import stat


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def is_python_executable(path: str) -> bool:
    """Check if a file is an executable Python script."""
    if not path.endswith(".py"):
        return False
    if os.path.basename(path).endswith("_test.py"):
        return False  # Test files are exempt
    try:
        st = os.stat(path)
        return bool(st.st_mode & stat.S_IXUSR)
    except OSError:
        return False


def find_python_executables(root: str) -> list[str]:
    """Return paths of executable Python scripts outside scripts/."""
    violations = []
    
    for dirpath, _, filenames in os.walk(root):
        # Skip .git directory
        if ".git" in dirpath:
            continue
        # Skip scripts/ directory (allowed location)
        rel_dir = os.path.relpath(dirpath, root)
        if rel_dir.startswith("scripts"):
            continue
        # Skip cmd/ directory (allowed for entrypoints)
        if rel_dir.startswith("cmd"):
            continue
        # Skip internal/pkg directories (modules, not scripts)
        if rel_dir.startswith("internal") or rel_dir.startswith("pkg"):
            continue
        
        for f in filenames:
            if f.endswith(".py"):
                full = os.path.join(dirpath, f)
                if is_python_executable(full):
                    rel = os.path.relpath(full, root)
                    violations.append(rel)
    
    return violations


def main() -> int:
    root = _repo_root()
    violations = find_python_executables(root)
    
    if violations:
        print("ERROR: Executable Python scripts found outside scripts/ directory.", file=sys.stderr)
        print("Zen‑Brain rule: All executable Python scripts must be placed under scripts/.", file=sys.stderr)
        print("Found the following violations:", file=sys.stderr)
        for v in violations:
            print(f"  • {v}", file=sys.stderr)
        print(file=sys.stderr)
        print("Either:", file=sys.stderr)
        print("  1. Move the script to scripts/", file=sys.stderr)
        print("  2. Remove executable bit if it's a module (chmod -x)", file=sys.stderr)
        print("  3. Rename as _test.py if it's a test file", file=sys.stderr)
        return 1
    
    print("✓ Python script placement gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())