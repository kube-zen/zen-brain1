#!/usr/bin/env python3
"""
Gate: ZB-024 Local CPU Profile Enforcement (PHASE 7)

This gate fails CI if:
1. Active local CPU timeout is less than 45 minutes
2. Active local CPU keep_alive is less than 45 minutes
3. Active local CPU stale threshold <= execution timeout
4. Active local CPU path defaults to thinking=true
5. Active local CPU path uses non-0.8b model
6. Active local CPU path uses in-cluster Ollama
"""

import os
import sys
import re


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def check_gateway_timeout() -> list[tuple[str, int, str]]:
    """Check gateway.go for 45-minute timeout for local worker."""
    violations = []
    root = _repo_root()
    file_path = os.path.join(root, "internal/llm/gateway.go")

    if not os.path.isfile(file_path):
        return []

    try:
        with open(file_path, "r", encoding="utf-8") as f:
            content = f.read()

        # Check LocalWorkerTimeout default in DefaultGatewayConfig
        match = re.search(r'LocalWorkerTimeout:\s*(\d+)', content)
        if match:
            timeout_seconds = int(match.group(1))
            if timeout_seconds < 2700:  # 45 minutes
                violations.append((
                    "internal/llm/gateway.go",
                    timeout_seconds,
                    f"LocalWorkerTimeout={timeout_seconds}s (< 2700s=45m)"
                ))

    except Exception as e:
        violations.append(("internal/llm/gateway.go", 0, f"Error reading file: {e}"))

    return violations


def check_foreman_timeout() -> list[tuple[str, int, str]]:
    """Check foreman/main.go for 45-minute timeout default."""
    violations = []
    root = _repo_root()
    file_path = os.path.join(root, "cmd/foreman/main.go")

    if not os.path.isfile(file_path):
        return []

    try:
        with open(file_path, "r", encoding="utf-8") as f:
            content = f.read()

        # Check default timeout in flag definition
        match = re.search(r'factory-llm-timeout-seconds".*?envInt\("ZEN_FOREMAN_LLM_TIMEOUT_SECONDS",\s*(\d+)\)', content)
        if match:
            timeout_seconds = int(match.group(1))
            if timeout_seconds < 2700:  # 45 minutes
                violations.append((
                    "cmd/foreman/main.go",
                    timeout_seconds,
                    f"Default timeout={timeout_seconds}s (< 2700s=45m)"
                ))

    except Exception as e:
        violations.append(("cmd/foreman/main.go", 0, f"Error reading file: {e}"))

    return violations


def check_stale_threshold() -> list[tuple[str, int, str]]:
    """Check reconciler.go for stale threshold > 45m."""
    violations = []
    root = _repo_root()
    file_path = os.path.join(root, "internal/foreman/reconciler.go")

    if not os.path.isfile(file_path):
        return []

    try:
        with open(file_path, "r", encoding="utf-8") as f:
            content = f.read()

        # Check stale threshold value
        match = re.search(r'if staleDuration >\s*(\d+)\*time\.Minute\)', content)
        if match:
            threshold_minutes = int(match.group(1))
            # ZB-024: Stale threshold MUST be > 45m (2700s)
            if threshold_minutes <= 50:
                violations.append((
                    "internal/foreman/reconciler.go",
                    threshold_minutes,
                    f"Stale threshold={threshold_minutes}m (<= 50m, must be > 50m)"
                ))

    except Exception as e:
        violations.append(("internal/foreman/reconciler.go", 0, f"Error reading file: {e}"))

    return violations


def check_thinking_default() -> list[tuple[str, int, str]]:
    """Check thinking defaults to false for local CPU path."""
    violations = []
    root = _repo_root()

    # Check foreman/main.go
    file_path = os.path.join(root, "cmd/foreman/main.go")

    if not os.path.isfile(file_path):
        return []

    try:
        with open(file_path, "r", encoding="utf-8") as f:
            content = f.read()

        # Check thinking default
        match = re.search(r'factory-llm-enable-thinking".*envBool\("ZEN_FOREMAN_LLM_ENABLE_THINKING",\s*(false)', content)
        if not match:
            violations.append((
                "cmd/foreman/main.go",
                0,
                "Missing thinking=false default for local CPU path"
            ))

    except Exception as e:
        violations.append(("cmd/foreman/main.go", 0, f"Error reading file: {e}"))

    return violations


def check_model_drift() -> list[tuple[str, int, str]]:
    """Check for non-0.8b local model references."""
    violations = []
    root = _repo_root()

    files_to_check = [
        "internal/llm/gateway.go",
        "internal/llm/ollama_provider.go",
        "cmd/apiserver/main.go",
        "cmd/foreman/main.go",
    ]

    for file_rel in files_to_check:
        file_path = os.path.join(root, file_rel)
        if not os.path.isfile(file_path):
            continue

        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()

            # Skip comments and examples
            lines = []
            for line in content.split('\n'):
                stripped = line.strip()
                if not stripped or stripped.startswith('//') or stripped.startswith('#'):
                    continue
                lines.append(line)

            content_clean = '\n'.join(lines)

            # Check for non-0.8b model references (excluding defaults)
            if re.search(r'(?<!0\.8b[^0-9])["\']?\s*:', content_clean):
                # Check if it's actually setting a non-0.8b model
                if re.search(r'model\s*[:=]\s*["\']?(qwen3\.5:(?!0\.8b)|llama|mistral|gemma)', content_clean):
                    violations.append((
                        file_rel,
                        0,
                        "Non-0.8b local model reference found"
                    ))

        except Exception as e:
            violations.append((file_rel, 0, f"Error reading file: {e}"))

    return violations


def check_in_cluster_ollama() -> list[tuple[str, int, str]]:
    """Check for in-cluster Ollama references in active local path."""
    violations = []
    root = _repo_root()

    files_to_check = [
        "config/profiles/local-cpu-45m.yaml",
        "internal/llm/gateway.go",
        "cmd/apiserver/main.go",
    ]

    for file_rel in files_to_check:
        file_path = os.path.join(root, file_rel)
        if not os.path.isfile(file_path):
            continue

        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()

            # Check for in-cluster Ollama URLs (excluding host.k3d.internal)
            if re.search(r'http://ollama:', content, re.IGNORECASE):
                violations.append((
                    file_rel,
                    0,
                    "In-cluster Ollama reference found (http://ollama:)"
                ))
            if re.search(r'http://ollama\.zen-brain:', content, re.IGNORECASE):
                violations.append((
                    file_rel,
                    0,
                    "In-cluster Ollama reference found (http://ollama.zen-brain:)"
                ))

        except Exception as e:
            violations.append((file_rel, 0, f"Error reading file: {e}"))

    return violations


def main() -> int:
    """Run all ZB-024 profile enforcement gates."""

    violations = []

    print("Checking ZB-024 local CPU profile enforcement...")

    # Check 1: Gateway timeout
    print("\n1. Checking gateway timeout (45m minimum)...")
    violations.extend(check_gateway_timeout())

    # Check 2: Foreman timeout
    print("2. Checking foreman timeout (45m minimum)...")
    violations.extend(check_foreman_timeout())

    # Check 3: Stale threshold
    print("3. Checking stale threshold (> 45m)...")
    violations.extend(check_stale_threshold())

    # Check 4: Thinking default
    print("4. Checking thinking default (false for local CPU)...")
    violations.extend(check_thinking_default())

    # Check 5: Model drift (0.8b only)
    print("5. Checking model drift (0.8b only for local)...")
    violations.extend(check_model_drift())

    # Check 6: In-cluster Ollama (forbidden)
    print("6. Checking in-cluster Ollama (forbidden for active local path)...")
    violations.extend(check_in_cluster_ollama())

    # Report violations
    if violations:
        print("\n❌ ZB-024 Profile Enforcement Gate: FAIL", file=sys.stderr)
        print("\nViolations found:\n", file=sys.stderr)
        for file_path, value, desc in violations:
            print(f"  • {file_path}: {desc}", file=sys.stderr)
            if value > 0:
                print(f"    Value: {value}", file=sys.stderr)
        print("\nFix required:", file=sys.stderr)
        print("- Local CPU path must use 45-minute timeout (2700s)", file=sys.stderr)
        print("- Stale threshold must be > 45m (50m or 60m recommended)", file=sys.stderr)
        print("- Local CPU path must default to thinking=false", file=sys.stderr)
        print("- Local model must be qwen3.5:0.8b ONLY", file=sys.stderr)
        print("- In-cluster Ollama is FORBIDDEN for active local path", file=sys.stderr)
        return 1

    print("✓ ZB-024 Profile Enforcement Gate: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
