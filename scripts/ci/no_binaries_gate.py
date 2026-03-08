#!/usr/bin/env python3
"""
Gate: No large or binary files in the repository.

Zen‑Brain rule: Avoid committing large binary files (>5 MiB) and
executable binaries (ELF/Mach‑O/PE) unless they are explicitly allowed.
Allowed binaries (if any) should be documented and kept small.
"""

import os
import sys
import subprocess


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def check_large_files(root: str, max_size_mb: int = 5) -> list[str]:
    """Return paths of files larger than max_size_mb."""
    max_bytes = max_size_mb * 1024 * 1024
    large_files = []
    
    # Get list of tracked files only (ignore build artifacts)
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
            tracked = None
        else:
            tracked = set(result.stdout.strip().splitlines())
    except (subprocess.TimeoutExpired, FileNotFoundError):
        tracked = None
    
    for dirpath, _, filenames in os.walk(root):
        if ".git" in dirpath:
            continue
        for f in filenames:
            full = os.path.join(dirpath, f)
            rel = os.path.relpath(full, root)
            
            # Skip if not tracked (build artifacts, temporary files)
            if tracked is not None and rel not in tracked:
                continue
            
            try:
                size = os.path.getsize(full)
                if size > max_bytes:
                    large_files.append(f"{rel} ({size // (1024*1024)} MiB)")
            except OSError:
                pass
    
    return large_files


def check_binary_executables(root: str) -> list[str]:
    """
    Use `file` command to detect ELF, Mach‑O, PE executables.
    Returns paths of detected binary executables.
    """
    binary_files = []
    
    # Check if `file` command is available
    try:
        subprocess.run(["file", "--version"], capture_output=True, timeout=2)
    except (FileNotFoundError, subprocess.TimeoutExpired):
        # `file` not available, skip binary detection
        return binary_files
    
    # Get list of tracked files only (ignore build artifacts)
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
            tracked = None
        else:
            tracked = set(result.stdout.strip().splitlines())
    except (subprocess.TimeoutExpired, FileNotFoundError):
        tracked = None
    
    # Check tracked files (or all files if git not available)
    for dirpath, _, filenames in os.walk(root):
        if ".git" in dirpath:
            continue
        for f in filenames:
            full = os.path.join(dirpath, f)
            rel = os.path.relpath(full, root)
            
            # Skip if not tracked (build artifacts, temporary files)
            if tracked is not None and rel not in tracked:
                continue
            
            # Skip files under .git, and maybe under vendor/, node_modules/, etc.
            if any(part.startswith(".") and part != "." for part in rel.split(os.sep)):
                continue
            if "node_modules" in rel or "vendor" in rel:
                continue
            
            try:
                result = subprocess.run(
                    ["file", "-b", full],
                    capture_output=True,
                    text=True,
                    timeout=2,
                )
                if result.returncode == 0:
                    output = result.stdout.lower()
                    # Detect common executable binary types
                    if ("elf" in output and "executable" in output) or \
                       ("mach‑o" in output) or \
                       ("pe32" in output and "executable" in output):
                        binary_files.append(rel)
            except (OSError, subprocess.TimeoutExpired):
                pass
    
    return binary_files


def main() -> int:
    root = _repo_root()
    errors = []
    warnings = []
    
    # Check large files
    large = check_large_files(root)
    if large:
        warnings.append("Large files (>5 MiB) found in repository:")
        warnings.extend(f"  • {f}" for f in large)
        warnings.append("  Consider using Git LFS or excluding these files.")
    
    # Check binary executables
    binaries = check_binary_executables(root)
    if binaries:
        errors.append("Binary executable files found in repository:")
        errors.extend(f"  • {f}" for f in binaries)
        errors.append("  Binary executables should not be committed. Use build‑time generation.")
    
    if warnings:
        for w in warnings:
            print(f"WARNING: {w}", file=sys.stderr)
    
    if errors:
        print("ERROR: Binary file violations:", file=sys.stderr)
        for e in errors:
            print(e, file=sys.stderr)
        print(file=sys.stderr)
        print("Refer to repo policy on binary files.", file=sys.stderr)
        return 1
    
    print("✓ No binaries gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())