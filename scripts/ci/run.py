#!/usr/bin/env python3
"""
Zen‑Brain CI gate runner.

Usage:
    python3 scripts/ci/run.py [--suite SUITE]

Suites:
    default   – run all repo‑hygiene gates (default)
    governance – repo layout, shell scripts, Python placement, executable sprawl
    docs      – docs layout and internal links
    binaries  – large/binary file checks
    all       – all gates (same as default)
"""

import os
import sys
import subprocess
import argparse


GATES = {
    "no_shell_scripts": "scripts/ci/no_shell_scripts_gate.py",
    "python_placement": "scripts/ci/python_script_placement_gate.py",
    "repo_layout": "scripts/ci/repo_layout_gate.py",
    "executable_sprawl": "scripts/ci/no_executable_sprawl_gate.py",
    "no_binaries": "scripts/ci/no_binaries_gate.py",
    "docs_links": "scripts/ci/docs_link_gate.py",
    "canonical_plan": "scripts/ci/canonical_plan_gate.py",
    "zen_sdk_ownership": "scripts/ci/zen_sdk_ownership_gate.py",
    "kb_qmd_direction": "scripts/ci/kb_qmd_direction_gate.py",
    "model_facing_policy": "scripts/ci/model_facing_policy_gate.py",
    "vertical_slice_contract": "scripts/ci/vertical_slice_contract_gate.py",
    "local_model_policy": "scripts/ci/local_model_policy_gate.py",  # ZB-023: Enforce local CPU inference policy
    "local_cpu_profile": "scripts/ci/local_cpu_profile_gate.py",  # ZB-024: Enforce 45m timeout profile
    # LAYER 2: Credential rails gates
    "canonical_credential_access": "scripts/ci/canonical_credential_access_gate.py",
    "no_secret_echo": "scripts/ci/no_secret_echo_gate.py",
    "no_alt_credential_rails": "scripts/ci/no_alt_credential_rails_gate.py",
    "zenlock_mount_only": "scripts/ci/zenlock_mount_only_gate.py",
    "docs_drift_credential_rails": "scripts/ci/docs_drift_credential_rails_gate.py",
}

SUITES = {
    "default": ["no_shell_scripts", "python_placement", "repo_layout",
                "executable_sprawl", "no_binaries", "docs_links", "canonical_plan", "zen_sdk_ownership", "kb_qmd_direction", "model_facing_policy",
                "local_model_policy", "local_cpu_profile",
                "canonical_credential_access", "no_secret_echo", "no_alt_credential_rails", "zenlock_mount_only", "docs_drift_credential_rails"],  # LAYER 2: Credential gates always run
    "governance": ["no_shell_scripts", "python_placement", "repo_layout",
                   "executable_sprawl", "zen_sdk_ownership", "model_facing_policy",
                   "local_model_policy", "local_cpu_profile",
                   "canonical_credential_access", "no_alt_credential_rails"],  # LAYER 2: Credential access and alt rails
    "docs": ["repo_layout", "docs_links", "canonical_plan", "kb_qmd_direction",
              "local_model_policy", "local_cpu_profile",
              "docs_drift_credential_rails"],  # LAYER 2: Docs drift check
    "binaries": ["no_binaries"],
    "credentials": ["canonical_credential_access", "no_secret_echo", "no_alt_credential_rails", "zenlock_mount_only", "docs_drift_credential_rails"],  # LAYER 2: All credential gates
    "all": list(GATES.keys()),
}


def run_gate(name: str, path: str) -> bool:
    """Run a single gate; return True on success."""
    print(f"⏳ Running {name}...", flush=True)
    try:
        result = subprocess.run(
            [sys.executable, path],
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode == 0:
            print(f"✅ {name}: pass")
            if result.stdout.strip():
                print(result.stdout.strip())
            return True
        else:
            print(f"❌ {name}: fail")
            if result.stderr:
                print(result.stderr, file=sys.stderr)
            return False
    except subprocess.TimeoutExpired:
        print(f"⏰ {name}: timeout (exceeded 30s)", file=sys.stderr)
        return False
    except Exception as e:
        print(f"💥 {name}: error: {e}", file=sys.stderr)
        return False


def main() -> int:
    parser = argparse.ArgumentParser(description="Run Zen‑Brain CI gates")
    parser.add_argument(
        "--suite",
        choices=list(SUITES.keys()),
        default="default",
        help="Gate suite to run (default: default)",
    )
    args = parser.parse_args()

    root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    os.chdir(root)

    suite_gates = SUITES[args.suite]
    print(f"Running suite '{args.suite}' ({len(suite_gates)} gates)")
    print()

    failed = []
    for gate_name in suite_gates:
        gate_path = GATES[gate_name]
        full_path = os.path.join(root, gate_path)
        if not os.path.isfile(full_path):
            print(f"⚠️  Gate not found: {gate_path}", file=sys.stderr)
            continue
        if not run_gate(gate_name, full_path):
            failed.append(gate_name)
        print()

    if failed:
        print(f"❌ {len(failed)} gate(s) failed: {', '.join(failed)}")
        return 1
    else:
        print("✅ All gates passed.")
        return 0


if __name__ == "__main__":
    sys.exit(main())