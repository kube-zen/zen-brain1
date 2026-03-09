#!/usr/bin/env python3
"""
Gate: Vertical slice contract must pass.

Zen‑Brain rule:
- The integration test that validates the vertical slice pipeline must pass.
- This ensures the thin trusted path through tests:
  * canonical WorkItem creation
  * analyzer/planner contract executes
  * session object created/updated
  * factory accepts task
  * proof‑of‑work artifact created
  * status update contract emitted

Runs two test suites:
1. OfficePipeline integration test (covers first three)
2. Factory integration test (covers factory and proof‑of‑work)
"""

import os
import sys
import subprocess


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


def run_test(root: str, pkg: str, test_pattern: str = "", env=None) -> tuple[bool, str]:
    """Run a Go test package and return (success, error)."""
    cmd = ["go", "test", pkg, "-v"]
    if test_pattern:
        cmd.extend(["-run", test_pattern])
    
    try:
        result = subprocess.run(
            cmd,
            cwd=root,
            capture_output=True,
            text=True,
            timeout=120,
            env=env,
        )
        if result.returncode == 0:
            return True, ""
        else:
            # Extract error lines
            lines = result.stderr.split('\n')
            error_msg = '\n'.join(lines[-20:]) if lines else "Unknown error"
            return False, f"Test {pkg} failed:\n{error_msg}"
    except subprocess.TimeoutExpired:
        return False, f"Test {pkg} timed out (120s)"
    except Exception as e:
        return False, f"Failed to run test {pkg}: {e}"


def main() -> int:
    root = _repo_root()
    env = os.environ.copy()
    env["ZEN_BRAIN_REDIS_DISABLED"] = "1"
    
    print("Running vertical slice contract gate...")
    
    # 1. OfficePipeline integration test
    print("  [1/2] OfficePipeline integration (Redis disabled)")
    success, error = run_test(root, "./internal/integration", "TestOfficePipeline_ProcessWorkItem", env)
    if not success:
        print(f"❌ Vertical slice contract gate failed:", file=sys.stderr)
        print(error, file=sys.stderr)
        print(file=sys.stderr)
        print("Refer to internal/integration/office_test.go for the test.", file=sys.stderr)
        return 1
    print("    ✓ OfficePipeline integration passes")
    
    # 2. Factory integration test (covers factory accepts task and proof-of-work artifact created)
    print("  [2/2] Factory integration")
    success, error = run_test(root, "./internal/factory", "TestFactory", env)
    if not success:
        print(f"❌ Vertical slice contract gate failed:", file=sys.stderr)
        print(error, file=sys.stderr)
        print(file=sys.stderr)
        print("Refer to internal/factory/factory_test.go for the test.", file=sys.stderr)
        return 1
    print("    ✓ Factory integration passes")
    
    print("✅ Vertical slice contract gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())