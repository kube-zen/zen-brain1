#!/usr/bin/env python3
"""
CI Gate: Check for Jira project key drift.

Ensures only one canonical project key (ZB) is used across active paths.
Legacy keys (e.g., SCRUM) should only appear in example/test/legacy contexts.

Exit codes:
  0: No drift detected
  1: Drift detected or error
"""

import os
import sys
import subprocess
from pathlib import Path

CANONICAL_KEY = "ZB"
LEGACY_KEYS = ["SCRUM"]

# Paths that are considered active (not examples/legacy)
ACTIVE_PATHS = [
    "cmd/",
    "internal/",
    "deploy/",
    "configs/",
]

# Paths that are allowed to have legacy references
ALLOWED_LEGACY_PATHS = [
    "_test.go",
    "/examples/",
    "/test/",
    "docs/06-OPERATIONS/JIRA_PROJECT_KEY_RAILS.md",  # Documentation about legacy
    "docs/CURRENT_STATE.md",  # Documentation about legacy
]

def is_active_path(file_path):
    """Check if file is in an active path."""
    for active in ACTIVE_PATHS:
        if file_path.startswith(active):
            # Check if it's in allowed legacy paths
            for allowed in ALLOWED_LEGACY_PATHS:
                if allowed in file_path:
                    return False
            return True
    return False

def check_canonical_key_in_metadata():
    """Verify canonical project key is in jira-metadata.yaml."""
    metadata_path = "deploy/zen-lock/jira-metadata.yaml"

    if not os.path.exists(metadata_path):
        print(f"ERROR: {metadata_path} not found")
        return False

    with open(metadata_path, 'r') as f:
        content = f.read()

    if f'project_key: "{CANONICAL_KEY}"' in content or f"project_key: '{CANONICAL_KEY}'" in content:
        print(f"✓ Canonical project key {CANONICAL_KEY} found in {metadata_path}")
        return True
    else:
        print(f"ERROR: Canonical project key {CANONICAL_KEY} not found in {metadata_path}")
        return False

def check_for_legacy_keys():
    """Check for legacy project keys in active paths."""
    issues = []

    for legacy_key in LEGACY_KEYS:
        # Search for legacy key references
        result = subprocess.run(
            ["grep", "-r", legacy_key, "--include=*.go", "--include=*.yaml", "--include=*.yml", "--include=*.py"] +
            ["--exclude-dir=vendor", "--exclude-dir=.git"] +
            ["."],
            capture_output=True,
            text=True,
            cwd=os.getcwd()
        )

        if result.returncode == 0:
            lines = result.stdout.strip().split('\n')
            for line in lines:
                if not line:
                    continue

                # Extract file path
                file_path = line.split(':')[0] if ':' in line else line

                # Check if it's an active path
                if is_active_path(file_path):
                    issues.append(f"Legacy key {legacy_key} found in active path: {line}")

    if issues:
        print("ERROR: Legacy project keys found in active paths:")
        for issue in issues[:10]:  # Show first 10
            print(f"  {issue}")
        if len(issues) > 10:
            print(f"  ... and {len(issues) - 10} more")
        return False

    print("✓ No legacy project keys in active paths")
    return True

def check_project_key_consistency():
    """Check that all project key references use the canonical key."""
    issues = []

    # Search for project key assignments
    patterns = [
        r'project[_-]?key.*=.*["\']([A-Z]+)["\']',
        r'PROJECT[_-]?KEY.*=.*["\']([A-Z]+)["\']',
        r'projectKey.*=.*["\']([A-Z]+)["\']',
    ]

    for pattern in patterns:
        result = subprocess.run(
            ["grep", "-rE", pattern, "--include=*.go", "--include=*.yaml", "--include=*.yml", "--include=*.py"] +
            ["--exclude-dir=vendor", "--exclude-dir=.git"] +
            ["."],
            capture_output=True,
            text=True,
            cwd=os.getcwd()
        )

        if result.returncode == 0:
            lines = result.stdout.strip().split('\n')
            for line in lines:
                if not line:
                    continue

                # Skip if it's the canonical key
                if CANONICAL_KEY in line and 'project' in line.lower():
                    continue

                # Skip if it's in allowed paths
                file_path = line.split(':')[0] if ':' in line else line
                if not is_active_path(file_path):
                    continue

                # Check for non-canonical keys
                for legacy_key in LEGACY_KEYS:
                    if legacy_key in line:
                        issues.append(f"Non-canonical project key: {line}")
                        break

    if issues:
        print("ERROR: Inconsistent project key usage:")
        for issue in issues[:10]:
            print(f"  {issue}")
        return False

    print("✓ Project key usage is consistent")
    return True

def main():
    print("=== Jira Project Key Drift Detection ===\n")

    checks = [
        ("Canonical key in metadata", check_canonical_key_in_metadata),
        ("No legacy keys in active paths", check_for_legacy_keys),
        ("Project key consistency", check_project_key_consistency),
    ]

    results = []
    for name, check_func in checks:
        print(f"\nCheck: {name}")
        try:
            result = check_func()
            results.append((name, result))
        except Exception as e:
            print(f"ERROR: {e}")
            results.append((name, False))

    print("\n=== Summary ===")
    passed = sum(1 for _, r in results if r)
    total = len(results)

    for name, result in results:
        status = "✓ PASS" if result else "✗ FAIL"
        print(f"{status}: {name}")

    print(f"\n{passed}/{total} checks passed")

    if passed == total:
        print("\n✅ No project key drift detected")
        return 0
    else:
        print("\n❌ Project key drift detected")
        return 1

if __name__ == "__main__":
    sys.exit(main())
