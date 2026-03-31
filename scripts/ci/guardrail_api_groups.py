#!/usr/bin/env python3
"""CI guardrail: enforce *.zen-mesh.io API groups.

Scans active source for deprecated API group references:
  - zen.kube-zen.io    (historical typo-era group)
  - zen.kube-zen.com   (old company domain)
  - zenbrain.kube-zen.io (TaskSession artifact)

Exit code 0 = clean
Exit code 1 = deprecated API group found in non-exempt files
"""
import os
import re
import sys

# Deprecated API groups — MUST NOT appear in active source
FORBIDDEN_GROUPS = [
    "zen.kube-zen.io",
    "zen.kube-zen.com",
    "zenbrain.kube-zen.io",
]

# Files/patterns where deprecated groups are allowed
EXEMPT_PATH_PATTERNS = [
    r"99-ARCHIVE/",                                   # Historical archive
    r"guardrail_api_groups",                          # This guardrail itself
    r"0010_API_GROUP_MIGRATION",                      # Migration ADR/portfolio docs
    r"crd-migration",                                 # Migration reports in workspace
    r"internal/labels/labels\.go",                    # Compatibility constants (old keys kept for fallback)
    r"internal/labels/labels_test\.go",               # Tests for compatibility behavior
    r"api_group_baseline\.txt",                        # Baseline file (contains violation fingerprints)
    r"CLUSTER_RECOVERY_RUNBOOK",                      # Recovery doc (references old groups)
    r"run-08b-positive-control\.sh",                  # Test script with legacy label keys
    r"02-crd-braintask\.yaml",                        # Stale CRD file (to be deleted in PATCHSET C)
    r"crd-tasksession\.yaml",                         # Legacy CRD (to be deleted in PATCHSET C)
]

# Line patterns where deprecated groups are allowed in any file
EXEMPT_LINE_PATTERNS = [
    r"HISTORICAL",
    r"was\s+(wrong|incorrect|old|legacy|deprecated)",
    r"previously\s+used",
    r"changed\s+from",
    r"migration\s+to",
    r"FORBIDDEN",
    r"DEPRECATED",
    r"DEPRECATION",
    r"guardrail.*blocks",
    r"allowed.*only.*in",
    r"see\s+ADR",
    r"# DEPRECATED:",              # Go deprecation comments
    r"// DEPRECATED:",
    r"TODO.*migration",
    r"legacy.*key",
    r"old.*key.*fallback",
]

# Go module import paths use github.com/kube-zen/... which is fine
# (module rename is a separate concern from API group migration)
EXEMPT_IMPORT_PATTERNS = [
    r"github\.com/kube-zen/",
]

# Scan extensions
SCAN_EXTENSIONS = {
    ".go", ".yaml", ".yml", ".sh", ".py", ".md", ".txt",
    ".json", ".toml", ".mod", ".sum",
}

# Skip directories
SKIP_DIRS = {".git", "vendor", "node_modules", ".cache", "__pycache__",
             "zz_generated", ".zen"}

# Skip binary files
SKIP_BINARIES = {
    "roadmap-steward", "queue-steward", "scheduler",
    "factory-fill", "useful-batch", "finding-ticketizer",
    "zen-brain", "foreman", "apiserver", "admission-gate",
    "migrate-wrapper", "controller",
}


def is_exempt_path(relpath):
    for pattern in EXEMPT_PATH_PATTERNS:
        if re.search(pattern, relpath):
            return True
    return False


def is_exempt_line(line):
    # Check if line is a Go import statement
    if re.search(r'"github\.com/kube-zen/', line):
        return True
    for pattern in EXEMPT_LINE_PATTERNS:
        if re.search(pattern, line, re.IGNORECASE):
            return True
    return False


def scan_repo(repo_root):
    violations = []

    for root, dirs, files in os.walk(repo_root):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]

        for fname in files:
            fpath = os.path.join(root, fname)
            relpath = os.path.relpath(fpath, repo_root)

            ext = os.path.splitext(fname)[1].lower()
            if ext not in SCAN_EXTENSIONS:
                continue

            if fname in SKIP_BINARIES:
                continue

            path_exempt = is_exempt_path(relpath)

            try:
                with open(fpath, "r", encoding="utf-8", errors="replace") as f:
                    for lineno, line in enumerate(f, 1):
                        for group in FORBIDDEN_GROUPS:
                            if group in line:
                                if path_exempt or is_exempt_line(line):
                                    continue
                                violations.append({
                                    "file": relpath,
                                    "line": lineno,
                                    "group": group,
                                    "content": line.strip(),
                                })
            except (IOError, OSError):
                continue

    return violations


# ---------------------------------------------------------------------------
# Baseline mode: fail only on NEW violations not in the known baseline.
# This prevents the guardrail from breaking CI over known deferred items.
#
# Baseline generation:
#   python3 scripts/ci/guardrail_api_groups.py --generate-baseline
#
# To regenerate after PATCHSET C resolves items, re-run --generate-baseline.
# ---------------------------------------------------------------------------
BASELINE_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                             "api_group_baseline.txt")


def load_baseline():
    """Load known violations from the baseline file. Each line is file:line."""
    baseline = set()
    if os.path.exists(BASELINE_FILE):
        with open(BASELINE_FILE, "r") as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#"):
                    baseline.add(line)
    return baseline


def generate_baseline(violations):
    """Write current violations as the new baseline."""
    with open(BASELINE_FILE, "w") as f:
        f.write("# Auto-generated API group violation baseline.\n")
        f.write("# Regenerate: python3 scripts/ci/guardrail_api_groups.py --generate-baseline\n")
        f.write("# See ADR-0010 (docs/01-ARCHITECTURE/ADR/0010_API_GROUP_MIGRATION.md)\n")
        f.write(f"# Violations: {len(violations)}\n\n")
        for v in violations:
            f.write(f"{v['file']}:{v['line']}\n")
    print(f"📝 Baseline written to {os.path.relpath(BASELINE_FILE)} ({len(violations)} entries)")
    print("   Commit this file alongside the guardrail to enable baseline enforcement.")


def main():
    repo_root = os.environ.get("REPO_ROOT", ".")

    if "--generate-baseline" in sys.argv:
        violations = scan_repo(repo_root)
        generate_baseline(violations)
        sys.exit(0)

    violations = scan_repo(repo_root)
    baseline = load_baseline()

    new_violations = []
    for v in violations:
        key = f"{v['file']}:{v['line']}"
        if key not in baseline:
            new_violations.append(v)

    if new_violations:
        print("❌ NEW legacy API group violation(s) detected.")
        print("   Use *.zen-mesh.io targets or documented migration compatibility paths only.")
        print()
        for v in new_violations:
            print(f"   {v['file']}:{v['line']}: [{v['group']}] {v['content'][:120]}")
        print()
        print(f"   {len(new_violations)} new violation(s). {len(baseline)} baseline entries remain.")
        print()
        print("Fix: Replace with the approved target (brain.zen-mesh.io or platform.zen-mesh.io),")
        print("     or regenerate baseline after resolving items:")
        print("     python3 scripts/ci/guardrail_api_groups.py --generate-baseline")
        sys.exit(1)
    else:
        baseline_count = len(baseline)
        total_count = len(violations)
        if baseline_count > 0:
            print(f"✅ No new violations. {total_count} baseline entries remain (see api_group_baseline.txt).")
            print("   Target: *.zen-mesh.io | Mode: BASELINE")
        else:
            print("✅ No deprecated API group references found. Target: *.zen-mesh.io")
        sys.exit(0)


if __name__ == "__main__":
    main()
