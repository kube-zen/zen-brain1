#!/usr/bin/env python3
"""
Gate: zen-sdk ownership – prevent local reimplementation of SDK-owned concerns.

Zen‑Brain rule: Cross‑cutting concerns must come from zen‑sdk.
If zen‑brain needs a new cross‑cutting capability, build it in zen‑sdk first, then import.

SDK‑owned concerns (minimum):
- receiptlog / event ledger foundation
- dedup
- dlq
- retry
- observability
- health
- leader election
- generic logging
- generic crypto helpers
- scheduler
- events

This gate fails if:
- A directory under internal/ or pkg/ matches an SDK‑owned package name (receiptlog, dedup, …)
- A .go file implements functionality that belongs to SDK (detected by keywords) and is not allowlisted.
"""

import os
import sys
import fnmatch


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def load_allowlist(root: str) -> set[str]:
    """Load allowlist from scripts/ci/zen_sdk_allowlist.txt."""
    allowlist_path = os.path.join(root, "scripts", "ci", "zen_sdk_allowlist.txt")
    allowed = set()
    if os.path.isfile(allowlist_path):
        with open(allowlist_path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.split("#")[0].strip()
                if line:
                    allowed.add(line)
    return allowed


def sdk_owned_packages() -> list[str]:
    """Return list of SDK‑owned package names (directory names)."""
    return [
        "receiptlog",
        "dedup",
        "dlq",
        "retry",
        "observability",
        "health",
        "leader",
        "logging",
        "crypto",
        "scheduler",
        "events",
    ]


def find_sdk_like_directories(root: str) -> list[str]:
    """Find directories under internal/ or pkg/ that match SDK‑owned package names."""
    sdk_packages = sdk_owned_packages()
    violations = []
    
    for top in ["internal", "pkg"]:
        top_path = os.path.join(root, top)
        if not os.path.isdir(top_path):
            continue
        for dirpath, dirnames, _ in os.walk(top_path):
            for d in dirnames:
                if d in sdk_packages:
                    rel = os.path.relpath(os.path.join(dirpath, d), root)
                    violations.append(rel)
    return violations


def find_sdk_like_files(root: str) -> list[str]:
    """
    Find .go files that appear to implement SDK‑owned functionality.
    Heuristic: files that define types/functions with SDK keywords.
    """
    # Keywords that indicate SDK‑owned functionality
    keywords = [
        "Deduplicate",
        "Retry",
        "DeadLetterQueue",
        "HealthCheck",
        "LeaderElection",
        "Logger",
        "Encrypt",
        "Decrypt",
        "Schedule",
        "EventBus",
    ]
    violations = []
    
    for top in ["internal", "pkg"]:
        top_path = os.path.join(root, top)
        if not os.path.isdir(top_path):
            continue
        for dirpath, _, filenames in os.walk(top_path):
            for f in filenames:
                if not f.endswith(".go") or f.endswith("_test.go"):
                    continue
                full = os.path.join(dirpath, f)
                rel = os.path.relpath(full, root)
                try:
                    with open(full, "r", encoding="utf-8") as fp:
                        content = fp.read()
                        # Simple keyword detection (could be improved)
                        for kw in keywords:
                            if kw in content:
                                violations.append(rel)
                                break
                except UnicodeDecodeError:
                    pass
    return violations


def main() -> int:
    root = _repo_root()
    allowed = load_allowlist(root)
    
    def is_allowed(path: str) -> bool:
        """Check if a file path is covered by an allowlist entry (exact or directory prefix)."""
        if path in allowed:
            return True
        for entry in allowed:
            if path.startswith(entry + "/") or path.startswith(entry + os.sep):
                return True
        return False
    
    errors = []
    
    # 1. Check for SDK‑like directories
    dir_violations = find_sdk_like_directories(root)
    for viol in dir_violations:
        if not is_allowed(viol):
            errors.append(f"Directory named like SDK package: {viol}")
    
    # 2. Check for SDK‑like files (keyword detection)
    file_violations = find_sdk_like_files(root)
    for viol in file_violations:
        if not is_allowed(viol):
            errors.append(f"File implements SDK‑like functionality: {viol}")
    
    if errors:
        print("ERROR: zen‑sdk ownership gate violations:", file=sys.stderr)
        for e in errors:
            print(f"  • {e}", file=sys.stderr)
        print(file=sys.stderr)
        print("Cross‑cutting concerns must come from zen‑sdk.", file=sys.stderr)
        print("If you need a local implementation, add an ADR and update", file=sys.stderr)
        print("scripts/ci/zen_sdk_allowlist.txt with the allowed path.", file=sys.stderr)
        return 1
    
    print("✓ zen‑sdk ownership gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())