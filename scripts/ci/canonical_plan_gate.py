#!/usr/bin/env python3
"""
Gate: Canonical construction plan must exist and be uniquely referenced.

Zen‑Brain rule:
- Exactly one canonical construction plan file: docs/01‑ARCHITECTURE/CONSTRUCTION_PLAN.md
- No other construction plan files (aliases, copies, symlinks) may exist.
- All references in README, CONTRIBUTING, and docs must point to the canonical file.
"""

import os
import sys
import re


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def check_canonical_plan_exists(root: str) -> tuple[bool, str]:
    """Return (success, error_message)."""
    canonical = os.path.join(root, "docs", "01-ARCHITECTURE", "CONSTRUCTION_PLAN.md")
    if not os.path.isfile(canonical):
        return False, f"Canonical construction plan file does not exist: {canonical}"
    return True, ""


def check_no_extra_plans(root: str) -> tuple[list[str], list[str]]:
    """Return (extra_files, broken_symlinks)."""
    import fnmatch
    
    extra = []
    broken = []
    
    # Patterns that might indicate a construction plan file
    patterns = [
        "*construction*plan*.md",
        "*CONSTRUCTION*PLAN*.md",
        "*construction*plan*.md",
        "*construction*.md",
    ]
    
    for dirpath, _, filenames in os.walk(root):
        # Skip .git directory
        if ".git" in dirpath:
            continue
        for f in filenames:
            full = os.path.join(dirpath, f)
            rel = os.path.relpath(full, root)
            # Skip canonical file
            if rel == "docs/01-ARCHITECTURE/CONSTRUCTION_PLAN.md":
                continue
            # Check if matches any pattern
            for pat in patterns:
                if fnmatch.fnmatch(f.lower(), pat.lower()):
                    extra.append(rel)
                    break
            # Check if symlink
            if os.path.islink(full):
                target = os.readlink(full)
                if not os.path.exists(os.path.join(os.path.dirname(full), target)):
                    broken.append(rel)
    return extra, broken


def check_references(root: str) -> list[str]:
    """Check that known documentation files point to the canonical plan."""
    files_to_check = [
        "README.md",
        "CONTRIBUTING.md",
        "docs/README.md",
        "docs/INDEX.md",
        "docs/01-ARCHITECTURE/README.md",
        "docs/01-ARCHITECTURE/ROADMAP.md",
        "docs/03-DESIGN/README.md",
    ]
    
    errors = []
    canonical_rel = "docs/01-ARCHITECTURE/CONSTRUCTION_PLAN.md"
    
    for rel_path in files_to_check:
        full = os.path.join(root, rel_path)
        if not os.path.isfile(full):
            # Some files may not exist (optional)
            continue
        with open(full, "r", encoding="utf-8") as f:
            content = f.read()
        
        # Find all markdown links
        # pattern: [text](url)
        link_pattern = r'\[([^\]]+)\]\(([^)]+)\)'
        for match in re.finditer(link_pattern, content):
            url = match.group(2)
            # Skip external URLs
            if url.startswith("http://") or url.startswith("https://") or url.startswith("#"):
                continue
            # Normalize path relative to the file's directory
            base_dir = os.path.dirname(full)
            abs_url = os.path.normpath(os.path.join(base_dir, url))
            # Check if URL points to a construction plan file (case-insensitive)
            if "construction" in os.path.basename(abs_url).lower() and "plan" in os.path.basename(abs_url).lower():
                # Expected canonical absolute path
                canonical_abs = os.path.join(root, canonical_rel)
                if not os.path.samefile(abs_url, canonical_abs):
                    errors.append(f"{rel_path}: link '{url}' points to non-canonical construction plan")
    return errors


def main() -> int:
    root = _repo_root()
    errors = []
    warnings = []
    
    # 1. Canonical plan must exist
    ok, msg = check_canonical_plan_exists(root)
    if not ok:
        errors.append(msg)
    
    # 2. No extra construction plan files
    extra, broken = check_no_extra_plans(root)
    if extra:
        errors.append("Extra construction plan files found (only one canonical file allowed):")
        for f in extra:
            errors.append(f"  • {f}")
    if broken:
        warnings.append("Broken symlinks found:")
        for f in broken:
            warnings.append(f"  • {f}")
    
    # 3. References must point to canonical file
    ref_errors = check_references(root)
    if ref_errors:
        errors.extend(ref_errors)
    
    if warnings:
        for w in warnings:
            print(f"WARNING: {w}", file=sys.stderr)
    
    if errors:
        print("ERROR: Canonical plan gate violations:", file=sys.stderr)
        for e in errors:
            print(f"  • {e}", file=sys.stderr)
        print(file=sys.stderr)
        print("Refer to docs/01‑ARCHITECTURE/CONSTRUCTION_PLAN.md as the single source of truth.", file=sys.stderr)
        return 1
    
    print("✓ Canonical plan gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())