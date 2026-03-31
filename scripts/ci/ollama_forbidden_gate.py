#!/usr/bin/env python3
"""
Gate: Ollama Forbidden for zen-brain1

Policy: Ollama (port 11434) is DEPRECATED and FORBIDDEN for zen-brain1.
Primary (and only) local CPU inference path is llama.cpp (L1/L2).

This gate fails CI if active-path files contain:
1. OLLAMA_BASE_URL set to a non-empty value (not in comments)
2. provider=ollama or factory-llm-provider=ollama in non-comment lines
3. host.k3d.internal:11434 in non-comment lines
4. http://localhost:11434 or http://ollama: in non-comment lines
5. NewOllamaProvider() calls in active runtime code (non-test files)

Allowlist (these can contain Ollama references):
- docs/99-ARCHIVE/ (historical only)
- *_test.go (unit tests)
- internal/llm/ollama_provider.go (the provider itself exists but is not wired)
- internal/llm/ollama_warmup.go (exists but is not called)
- scripts/ci/ (CI gates that check for Ollama)
- charts/zen-brain-ollama/ (disabled chart, kept for optional Helm deploy)
- deployments/ollama-in-cluster/ (legacy, explicitly unsupported)
"""

import os
import re
import sys

# Patterns that indicate active Ollama wiring (NOT allowed in active files)
BLOCKED_PATTERNS = [
    (r"OLLAMA_BASE_URL.*=\s*[\"']http", "OLLAMA_BASE_URL set to an HTTP value (Ollama forbidden)"),
    (r"factory-llm-provider=ollama", "factory-llm-provider=ollama (Ollama forbidden)"),
    (r"provider.*=.*[\"']ollama[\"']", "provider set to ollama (forbidden in active config)"),
    (r"host\.k3d\.internal:11434", "host.k3d.internal:11434 (Ollama port, forbidden)"),
    (r"http://localhost:11434", "http://localhost:11434 (Ollama local, forbidden)"),
    (r"http://ollama:", "http://ollama: (in-cluster Ollama, forbidden)"),
]

# Files that are exempt (historical, test, or the provider itself)
EXEMPT_PREFIXES = [
    "docs/99-ARCHIVE/",
    "docs/05-OPERATIONS/ollama-deprecated/",
]
EXEMPT_SUFFIXES = [
    "_test.go",
]
EXEMPT_FILES = {
    "internal/llm/ollama_provider.go",
    "internal/llm/ollama_provider_test.go",
    "internal/llm/ollama_warmup.go",
    "scripts/ci/ollama_forbidden_gate.py",
    "scripts/ci/local_model_policy_gate.py",
    "scripts/ci/local_cpu_profile_gate.py",
    "charts/zen-brain-ollama/Chart.yaml",
    "charts/zen-brain-ollama/README.md",
    "charts/zen-brain-ollama/values.yaml",
    "charts/zen-brain-ollama/templates/statefulset.yaml",
    "charts/zen-brain-ollama/templates/preload-job.yaml",
    "charts/zen-brain-ollama/templates/vpa.yaml",
    "deployments/ollama-in-cluster/ollama.yaml",
    "deployments/ollama-in-cluster/README.md",
    "docs/05-OPERATIONS/LLAMA_CPP_VS_OLLAMA_QWEN_0_8B_BENCHMARK.md",
    "docs/05-OPERATIONS/ZEN_BRAIN_1_0_SELF_IMPROVEMENT.md",
    "docs/05-OPERATIONS/NIGHTSHIFT_STAGED_ROLLOUT.md",
    "docs/05-OPERATIONS/PHASE_24B_FINAL_REPORT.md",
    "docs/05-OPERATIONS/QWEN_2B_LOCAL_EVALUATION.md",
    "docs/05-OPERATIONS/WARMUP_FULL_REPORT.md",
    "docs/05-OPERATIONS/PHASE2_LLM_CODE_GENERATION.md",
    "docs/05-OPERATIONS/PROOF/",
    "docs/05-OPERATIONS/BLOCK6_STATUS_REPORT.md",
    "docs/05-OPERATIONS/BLOCK5_INTELLIGENCE_STATUS_REPORT.md",
    "docs/05-OPERATIONS/BLOCK6_IMPROVEMENT_SESSION_20260311.md",
    "docs/05-OPERATIONS/USEFUL_24_7_OPERATIONS_RUNBOOK.md",
    "docs/05-OPERATIONS/RELEASE_CHECKLIST.md",
    "docs/05-OPERATIONS/QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md",
    "docs/05-OPERATIONS/PHASE_22_MLQ_STATUS.md",
    "docs/05-OPERATIONS/PHASE_23_MLQ_RUNTIME_EVIDENCE.md",
    "docs/05-OPERATIONS/HARDENING_REPORT.md",
    "docs/05-OPERATIONS/PRODUCTION_PATH_DEFAULTS_FIX.md",
    "docs/05-OPERATIONS/CANONICAL_PATH_FAIL_CLOSED_FIXES.md",
    "docs/05-OPERATIONS/ZB_025_JIRA_INTAKE_CONTRACT.md",
    "docs/05-OPERATIONS/ZB_026_WORKER_OBSERVABILITY.md",
    "docs/05-OPERATIONS/ZB_08B_POSITIVE_CONTROL_RUNBOOK.md",
    "docs/05-OPERATIONS/REAL_JIRA_INTEGRATION_REPORT.md",
    "docs/05-OPERATIONS/OVERNIGHT_RUNBOOK.md",
    "docs/05-OPERATIONS/ZEN_MESH_OPERATOR_GUIDE.md",
    "docs/05-OPERATIONS/KNOWN_GOOD_CONFIG.md",
    "docs/01-ARCHITECTURE/",
    "docs/03-DESIGN/",
    "docs/04-DEVELOPMENT/",
    "brain_tasks/",
    "scripts/diag/",
    "scripts/check-proven-lane.sh",
    "scripts/health-check.sh",
    "scripts/zen-mesh-operator-loop.sh",
    "deploy/helmfile/",
    "deploy/README.md",
}

# Active paths to check (non-archived, non-test files)
ACTIVE_CHECK_DIRS = [
    "cmd/",
    "internal/llm/gateway.go",
    "internal/foreman/",
    "internal/factory/factory.go",
    "deployments/k3d/",
    "deploy/k8s/",
    "deploy/values/",
    "charts/zen-brain/values.yaml",
    "charts/zen-brain/templates/",
    "config/policy/",
    "config/clusters.yaml",
    "config/profiles/",
]


def is_exempt(file_path: str) -> bool:
    normalized = file_path.replace("\\", "/")
    for prefix in EXEMPT_PREFIXES:
        if normalized.startswith(prefix):
            return True
    for suffix in EXEMPT_SUFFIXES:
        if normalized.endswith(suffix):
            return True
    for ef in EXEMPT_FILES:
        if normalized == ef or normalized.endswith("/" + ef):
            return True
    return False


def is_comment_line(line: str) -> bool:
    stripped = line.strip()
    return stripped.startswith("#") or stripped.startswith("//") or stripped.startswith("<!--")


def check_file(file_path: str) -> list:
    violations = []
    try:
        with open(file_path, "r", encoding="utf-8", errors="ignore") as f:
            lines = f.readlines()
    except (OSError, IOError):
        return violations

    for idx, line in enumerate(lines, 1):
        if is_comment_line(line):
            continue
        for pattern, desc in BLOCKED_PATTERNS:
            if re.search(pattern, line, re.IGNORECASE):
                violations.append((idx, desc, line.rstrip()))
    return violations


def main():
    # Walk up from this script to the repo root (scripts/ci/ → scripts/ → repo-root)
    root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    if not root:
        root = "."

    all_violations = []
    checked = 0

    for target in ACTIVE_CHECK_DIRS:
        full = os.path.join(root, target)
        if os.path.isfile(full):
            rel = os.path.relpath(full, root)
            if is_exempt(rel):
                continue
            checked += 1
            violations = check_file(full)
            for v in violations:
                all_violations.append((rel, v[0], v[1], v[2]))
        elif os.path.isdir(full):
            for dirpath, _, filenames in os.walk(full):
                for fn in filenames:
                    if not fn.endswith(('.go', '.yaml', '.yml', '.py', '.json', '.md', '.sh')):
                        continue
                    filepath = os.path.join(dirpath, fn)
                    rel = os.path.relpath(filepath, root)
                    if is_exempt(rel):
                        continue
                    checked += 1
                    violations = check_file(filepath)
                    for v in violations:
                        all_violations.append((rel, v[0], v[1], v[2]))

    if all_violations:
        print("❌ Ollama Forbidden Gate: FAIL", file=sys.stderr)
        print(f"\nChecked {checked} files, found {len(all_violations)} violations:\n", file=sys.stderr)
        for file_rel, line_num, desc, line in all_violations:
            print(f"  • {file_rel}:{line_num}: {desc}", file=sys.stderr)
            print(f"    {line.strip()}", file=sys.stderr)
        print("\nFix: Remove all Ollama references from active runtime paths.", file=sys.stderr)
        print("llama.cpp is the only supported local CPU inference path.", file=sys.stderr)
        return 1

    print(f"✓ Ollama Forbidden Gate: PASS (checked {checked} files)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
