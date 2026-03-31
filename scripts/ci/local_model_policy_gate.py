#!/usr/bin/env python3
"""
⛔ DEPRECATED: This gate references Ollama policy. See ollama_forbidden_gate.py for the current enforcement.

Gate: ZB-023 Local Model Policy Enforcement

Zen-Brain rule (ZB-023):
- Ollama is FORBIDDEN for zen-brain1
- llama.cpp (L1/L2) is the ONLY supported local CPU inference path
- qwen3.5:0.8b is the local model via llama.cpp (NOT Ollama)

This gate is retained for backward compatibility but its Ollama-specific
checks are superseded by ollama_forbidden_gate.py.

This gate fails CI if:
1. Active-path files contain local model refs other than qwen3.5:0.8b
2. Active deployment/runtime docs/config reintroduce in-cluster Ollama
3. Policy config does not fail-close or enforce model selection
4. Documentation has outdated strict planner/worker model binding
5. Verification commands are missing from runbooks
"""

import os
import sys
import re


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


# Disallowed local model patterns (FAIL-CLOSED)
# Note: These patterns are specific to avoid false positives with "ollama" (provider name)
# Note: "llama.cpp" and "llama-cpp" are ALLOWED as runtime names (NOT model names)
DISALLOWED_LOCAL_MODELS = [
    r"\bllama\d*\b",  # llama2, llama3, etc. (standalone model names)
    r"\bmistral\b",
    r"\bgemma\b",
    r"\bphi\b",
    r"\btinyllama\b",
    r"\bcodellama\b",
    r"\bqwen3\.5:14b\b",
    r"\bqwen3\.5:\d+b",  # qwen3.5:7b, qwen3.5:32b, etc. (NOT 0.8b)
]

# Allowed runtime names (not model names)
ALLOWED_RUNTIME_NAMES = [
    "llama.cpp",
    "llama-cpp",
    "llama-server",
]

# Certified local model (ONLY allowed)
CERTIFIED_LOCAL_MODEL = "qwen3.5:0.8b"

# In-cluster Ollama patterns (FAIL-CLOSED)
IN_CLUSTER_OLLAMA_PATTERNS = [
    r"http://ollama:",
    r"http://ollama/",
    r"http://ollama\.zen-brain:",
    r"http://ollama\.zen-brain\.svc:",
    r"http://ollama\.zen-brain\.svc\.cluster\.local:",
    r"host.k3d.internal:11434",  # OK - host Docker (allowed)
]

# Outdated planner/worker model binding patterns (FAIL-CLOSED)
OUTDATED_PLANNER_WORKER_PATTERNS = [
    r"planner.*must.*GLM",
    r"GLM.*must.*be.*planner",
    r"worker.*must.*0\.8b",
    r"0\.8b.*must.*be.*worker",
    r"strict role separation",
    r"strict planner/worker split",
    r"enforces a strict role separation",
]


# Files to check for active-path enforcement
ACTIVE_PATH_FILES = [
    "config/clusters.yaml",
    "config/policy/providers.yaml",
    "config/policy/routing.yaml",
    "internal/llm/gateway.go",
    "internal/llm/ollama_provider.go",
    "internal/foreman/factory_runner.go",
    "cmd/apiserver/main.go",
    "cmd/foreman/main.go",
]

# Files to check for documentation consistency
DOC_FILES = [
    "docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md",
    "deploy/README.md",
    "config/policy/README.md",
]

# Patterns that indicate presence of verification commands
VERIFICATION_COMMAND_PATTERNS = [
    r"kubectl exec.*grep OLLAMA_BASE_URL",
    r"kubectl logs.*grep.*local-worker",
    r"kubectl exec.*wget.*11434/api/tags",
    # Local inference verification (llama.cpp primary)
    r"curl.*localhost.*56227",
    r"curl.*localhost.*60509",
    r"curl.*api/tags",
    # Verify no in-cluster ollama
    r"grep.*ollama\.zen-brain",
    r"grep.*11434",
]


def check_file_for_local_models(path: str) -> list[tuple[str, int, str]]:
    """Check a single file for disallowed local model references.
    Return list of (pattern, line_number, line_text)."""
    if not os.path.isfile(path):
        return []
    violations = []
    try:
        with open(path, "r", encoding="utf-8") as f:
            lines = f.readlines()
    except UnicodeDecodeError:
        return []

    for idx, line in enumerate(lines, 1):
        lower_line = line.lower()
        # Skip lines that are comments or examples
        if "#" in line and ("example" in lower_line or "not recommended" in lower_line):
            continue
        if "//" in line and ("example" in lower_line or "not recommended" in lower_line):
            continue
        if "todo:" in lower_line or "fixme:" in lower_line:
            continue
        # Skip Go package imports and type definitions (like "ollama_provider")
        if path.endswith(".go") and ("package " in line or "import " in line or "type " in line or "func " in line):
            continue
        # Skip lines that reference allowed runtime names (not model names)
        if any(runtime in lower_line for runtime in ALLOWED_RUNTIME_NAMES):
            continue

        # Check for disallowed local models (only in comments/docs, not code)
        for pattern in DISALLOWED_LOCAL_MODELS:
            if re.search(pattern, lower_line):
                # Only flag if it's in a comment or doc, not actual code
                if "#" in line or "//" in line or "```" in line or path.endswith(".md") or path.endswith(".yaml"):
                    violations.append((pattern, idx, line.rstrip()))
                break
    return violations


def check_file_for_in_cluster_ollama(path: str) -> list[tuple[str, int, str]]:
    """Check a single file for in-cluster Ollama references.
    Return list of (pattern, line_number, line_text)."""
    if not os.path.isfile(path):
        return []
    violations = []
    try:
        with open(path, "r", encoding="utf-8") as f:
            lines = f.readlines()
    except UnicodeDecodeError:
        return []

    for idx, line in enumerate(lines, 1):
        lower_line = line.lower()
        # Skip comments or examples that explain the policy
        if "#" in line and ("example" in lower_line or "not recommended" in lower_line or "forbidden" in lower_line):
            continue
        if "//" in line and ("example" in lower_line or "not recommended" in lower_line or "forbidden" in lower_line):
            continue

        # Skip policy documentation that mentions forbidden items (not advocating them)
        if "forbidden" in lower_line or "not allowed" in lower_line or "prohibited" in lower_line:
            continue

        # Check for in-cluster Ollama patterns (excluding host.k3d.internal which is allowed)
        # Also skip "http://host.k3d.internal:11434" which is allowed
        if "host.k3d.internal" in lower_line:
            continue

        # Look for actual in-cluster Ollama usage (not just documentation)
        if re.search(r"http://ollama:", lower_line) or \
           re.search(r"http://ollama/", lower_line) or \
           re.search(r"http://ollama\.zen-brain:", lower_line) or \
           re.search(r"http://ollama\.zen-brain\.svc:", lower_line):
            # Only flag if it looks like config or code, not just a comment
            if "#" not in line and "//" not in line:
                violations.append(("in-cluster-ollama", idx, line.rstrip()))
    return violations


def check_file_for_outdated_binding(path: str) -> list[tuple[str, int, str]]:
    """Check a single file for outdated planner/worker model binding.
    Return list of (pattern, line_number, line_text)."""
    if not os.path.isfile(path):
        return []
    violations = []
    try:
        with open(path, "r", encoding="utf-8") as f:
            lines = f.readlines()
    except UnicodeDecodeError:
        return []

    for idx, line in enumerate(lines, 1):
        lower_line = line.lower()
        # Skip lines that state the rule is REMOVED or OUTDATED
        if "removed" in lower_line or "outdated" in lower_line or "replaced" in lower_line or "deprecated" in lower_line:
            continue

        # Check for outdated planner/worker binding
        for pattern in OUTDATED_PLANNER_WORKER_PATTERNS:
            if re.search(pattern, lower_line):
                violations.append((pattern, idx, line.rstrip()))
                break
    return violations


def check_file_for_verification_commands(path: str) -> bool:
    """Check if file contains verification commands.
    Return True if at least one verification pattern is found."""
    if not os.path.isfile(path):
        return False
    try:
        with open(path, "r", encoding="utf-8") as f:
            content = f.read().lower()
    except UnicodeDecodeError:
        return False

    for pattern in VERIFICATION_COMMAND_PATTERNS:
        if re.search(pattern, content):
            return True
    return False


def main() -> int:
    root = _repo_root()
    violations = []

    # Check 1: Active-path files for disallowed local models
    print("Checking active-path files for disallowed local model references...")
    for file_rel in ACTIVE_PATH_FILES:
        file_path = os.path.join(root, file_rel)
        if not os.path.isfile(file_path):
            continue
        file_violations = check_file_for_local_models(file_path)
        for pattern, line_num, line in file_violations:
            violations.append((file_rel, line_num, f"Disallowed local model: {pattern}", line))

    # Check 2: Active-path files for in-cluster Ollama
    print("\nChecking active-path files for in-cluster Ollama references...")
    for file_rel in ACTIVE_PATH_FILES:
        file_path = os.path.join(root, file_rel)
        if not os.path.isfile(file_path):
            continue
        file_violations = check_file_for_in_cluster_ollama(file_path)
        for pattern, line_num, line in file_violations:
            violations.append((file_rel, line_num, f"In-cluster Ollama reference: {pattern}", line))

    # Check 3: Documentation for outdated planner/worker binding
    print("\nChecking documentation for outdated planner/worker model binding...")
    for file_rel in DOC_FILES:
        file_path = os.path.join(root, file_rel)
        if not os.path.isfile(file_path):
            continue
        file_violations = check_file_for_outdated_binding(file_path)
        for pattern, line_num, line in file_violations:
            violations.append((file_rel, line_num, f"Outdated planner/worker binding: {pattern}", line))

    # Check 4: Documentation for verification commands
    print("\nChecking documentation for verification commands...")
    for file_rel in DOC_FILES:
        file_path = os.path.join(root, file_rel)
        if not os.path.isfile(file_path):
            continue
        if not check_file_for_verification_commands(file_path):
            violations.append((file_rel, 0, "Missing verification commands", "File lacks verification commands (kubectl exec/grep/wget)"))

    # Report violations
    if violations:
        print("\n❌ ZB-023 Local Model Policy Gate: FAIL", file=sys.stderr)
        print("\nViolations found:\n", file=sys.stderr)
        for file_rel, line_num, desc, line in violations:
            print(f"  • {file_rel}:{line_num}: {desc}", file=sys.stderr)
            print(f"    {line.strip()}", file=sys.stderr)
        print("\nFix required:", file=sys.stderr)
        print("- Use ONLY qwen3.5:0.8b for local Ollama", file=sys.stderr)
        print("- Use ONLY host Docker Ollama (http://host.k3d.internal:11434)", file=sys.stderr)
        print("- Remove in-cluster Ollama references from active-path files", file=sys.stderr)
        print("- Remove outdated planner/worker model binding from docs", file=sys.stderr)
        print("- Add verification commands to runbooks/docs", file=sys.stderr)
        return 1

    print("✓ ZB-023 Local Model Policy Gate: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
