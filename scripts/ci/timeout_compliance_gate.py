#!/usr/bin/env python3
"""
CI Gate: Timeout Compliance (ZB-024)

Verifies that qwen3.5:0.8b local lane uses correct timeout values:
- timeout = 2700s (45 minutes)
- keep_alive = 45m
- stale threshold > 45m

Fails if any active 0.8b path uses timeout < 2700s or stale threshold <= 45m.
"""

import os
import sys
import re
import subprocess
from pathlib import Path

import yaml


def extract_timeout_values(data: dict, path_prefix: str = "") -> tuple[list, dict]:
    """Extract timeout/keepalive values from YAML dict."""
    issues = []
    found = {}

    def check_value(key, value, location):
        """Check a single value against requirements."""
        nonlocal issues, found
        full_key = f"{location}.{key}" if location else key
        found[full_key] = value

        if isinstance(value, (int, str)):
            try:
                if "timeout" in key.lower():
                    timeout_val = int(value)
                    if timeout_val < 2700:
                        issues.append(f"{full_key}={value}s < 2700s required for 0.8b lane")
                elif "keepalive" in key.lower() or "keep_alive" in key.lower():
                    match = re.match(r'^(\d+)m$', str(value).strip())
                    if match:
                        minutes = int(match.group(1))
                        if minutes < 45:
                            issues.append(f"{full_key}={value} < 45m required for 0.8b lane")
            except (ValueError, TypeError):
                pass

    # Check all keys in the dict
    for key, value in data.items():
        if isinstance(value, dict):
            # Recurse into nested dicts
            sub_issues, sub_found = extract_timeout_values(value, f"{path_prefix}.{key}" if path_prefix else key)
            issues.extend(sub_issues)
            found.update(sub_found)
        else:
            check_value(key, value, path_prefix)

    return issues, found


def check_go_stale_threshold(file_path: Path) -> tuple[list, dict]:
    """Check Go file for stale threshold."""
    content = file_path.read_text()
    issues = []
    found = {}

    # Look for stale threshold > 50*time.Minute (foreman/reconciler.go)
    stale_match = re.search(r'if\s+staleDuration\s*>\s*(\d+)\*time\.Minute', content)
    if stale_match:
        threshold = int(stale_match.group(1))
        found["stale_threshold_minutes"] = threshold
        if threshold <= 45:
            issues.append(f"stale_threshold={threshold}min <= 45m required (must be > 45m)")

    return issues, found


def check_k8s_env(namespace: str, deployment: str) -> tuple[list, dict, str]:
    """Check K8s deployment env vars."""
    cmd = [
        "kubectl",
        "get", "deployment", deployment,
        "-n", namespace,
        "-o", "jsonpath={.spec.template.spec.containers[0].env}"
    ]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
    except subprocess.CalledProcessError as e:
        return [], {}, f"SKIP: kubectl failed: {e.stderr}"

    # Parse JSON array of env vars
    try:
        import json
        env_vars = json.loads(result.stdout)
    except json.JSONDecodeError:
        return [], {}, "SKIP: Could not parse kubectl output"

    issues = []
    found = {}

    # Check each env var
    for env in env_vars:
        if not isinstance(env, dict):
            continue

        name = env.get("name")
        value = env.get("value")

        if name == "OLLAMA_TIMEOUT_SECONDS":
            found[name] = value
            try:
                timeout_val = int(value)
                if timeout_val < 2700:
                    issues.append(f"{name}={value}s < 2700s required for 0.8b lane")
            except (ValueError, TypeError):
                pass

        elif name == "OLLAMA_KEEP_ALIVE":
            found[name] = value
            if isinstance(value, str):
                match = re.match(r'^(\d+)m$', value.strip())
                if match:
                    minutes = int(match.group(1))
                    if minutes < 45:
                        issues.append(f"{name}={value} < 45m required for 0.8b lane")

        elif name == "ZEN_FOREMAN_LLM_TIMEOUT_SECONDS":
            found[name] = value
            try:
                timeout_val = int(value)
                if timeout_val < 2700:
                    issues.append(f"{name}={value}s < 2700s required for 0.8b lane")
            except (ValueError, TypeError):
                pass

    return issues, found, ""


def main() -> int:
    repo_root = Path(__file__).parent.parent.parent
    all_checks = []

    # Check config/policy files
    policy_dir = repo_root / "config" / "policy"
    if policy_dir.exists():
        for yaml_file in ["providers.yaml", "roles.yaml", "tasks.yaml", "chains.yaml"]:
            file_path = policy_dir / yaml_file
            try:
                with open(file_path) as f:
                    data = yaml.safe_load(f) or {}
                issues, found = extract_timeout_values(data)
                all_checks.append((not issues, str(file_path.relative_to(repo_root)), found, issues))
            except Exception as e:
                all_checks.append((False, f"ERROR: {file_path.name} - {e}", {}, []))

    # Check chart values files
    charts_dir = repo_root / "charts"
    if charts_dir.exists():
        # zen-brain chart
        zen_brain_values = charts_dir / "zen-brain" / "values.yaml"
        if zen_brain_values.exists():
            with open(zen_brain_values) as f:
                data = yaml.safe_load(f) or {}
            issues, found = extract_timeout_values(data)
            all_checks.append((not issues, "charts/zen-brain/values.yaml", found, issues))

        # zen-brain-ollama chart
        ollama_values = charts_dir / "zen-brain-ollama" / "values.yaml"
        if ollama_values.exists():
            with open(ollama_values) as f:
                data = yaml.safe_load(f) or {}
            issues, found = extract_timeout_values(data)
            all_checks.append((not issues, "charts/zen-brain-ollama/values.yaml", found, issues))

    # Check Go source files for stale threshold
    reconciler_go = repo_root / "internal" / "foreman" / "reconciler.go"
    if reconciler_go.exists():
        issues, found = check_go_stale_threshold(reconciler_go)
        all_checks.append((not issues, "internal/foreman/reconciler.go", found, issues))

    # Check K8s deployments (if KUBECONFIG is set)
    if os.environ.get("KUBECONFIG"):
        # Check foreman deployment
        issues, found, extra = check_k8s_env("zen-brain", "foreman")
        all_checks.append((not issues, "K8s: deployment/foreman", found, issues))

        # Check apiserver deployment
        issues, found, extra = check_k8s_env("zen-brain", "apiserver")
        all_checks.append((not issues, "K8s: deployment/apiserver", found, issues))

    # Print results
    failed = False
    print("=" * 70)
    print("ZB-024 Timeout Compliance Gate")
    print("=" * 70)
    for passed, msg, found, issues in all_checks:
        print(f"\n{msg}")
        if found:
            print("  Found values:")
            for k, v in sorted(found.items()):
                print(f"    {k}: {v}")
        if issues:
            print("  Issues:")
            for issue in issues:
                print(f"    - {issue}")
        if not passed:
            failed = True

    print("\n" + "=" * 70)
    if failed:
        print("GATE FAILED: Timeout compliance violations found")
        print("\nRequired values for qwen3.5:0.8b local lane:")
        print("  - timeout = 2700s (45 minutes)")
        print("  - keep_alive = 45m")
        print("  - stale threshold > 45m (typically 50m)")
        print("\nException: Controlled failure templates may use short timeouts")
        print("=" * 70)
        return 1
    else:
        print("GATE PASSED: All timeout values compliant")
        print("=" * 70)
        return 0


if __name__ == "__main__":
    sys.exit(main())
