#!/usr/bin/env python3
"""
DEPRECATED: This script is no longer the canonical bootstrap path.

Use the canonical bootstrap script instead:
  deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh

Canonical flow:
  1. AGE keys: ~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age
  2. Plaintext token: ~/zen/DONOTASKMOREFORTHISSHIT.txt (bootstrap-only)
  3. ZenLock manifest: deploy/zen-lock/jira-credentials.zenlock.yaml

Install Jira credentials from input YAML file.

This script is non-interactive and must be called with FILE= argument.
It validates required keys, encrypts credentials, and writes both local
runtime file and encrypted git-safe manifest.

Usage:
    python3 scripts/install_jira_credentials.py --input=/absolute/path/to/jira-input.yaml
"""

import os
import sys
import argparse
import subprocess
import tempfile
import shutil
from pathlib import Path


def _print_error(msg: str) -> None:
    print(f"\033[0;31mError: {msg}\033[0m", file=sys.stderr)


def _print_success(msg: str) -> None:
    print(f"\033[0;32m✓ {msg}\033[0m")


def _read_yaml_file(file_path: str) -> dict:
    """Read YAML file and return parsed data."""
    try:
        import yaml
        with open(file_path, 'r') as f:
            return yaml.safe_load(f)
    except ImportError:
        _print_error("PyYAML not installed. Install with: pip install pyyaml")
        sys.exit(1)
    except Exception as e:
        _print_error(f"Failed to read YAML file {file_path}: {e}")
        sys.exit(1)


def _validate_jira_credentials(data: dict) -> dict:
    """Validate Jira credentials and return cleaned data."""
    required_fields = ["JIRA_URL", "JIRA_EMAIL", "JIRA_API_TOKEN", "JIRA_PROJECT_KEY"]

    # Try stringData format first (ZenLock-style)
    if "stringData" in data:
        creds = data["stringData"]
    else:
        creds = data

    # Check required fields
    missing = [field for field in required_fields if field not in creds]
    if missing:
        _print_error(f"Missing required fields: {', '.join(missing)}")
        sys.exit(1)

    # Validate field values
    for field in required_fields:
        if not creds.get(field):
            _print_error(f"Field {field} is empty")
            sys.exit(1)

    # CRITICAL FIX: Normalize credential values to remove KEY= prefix if present
    # This prevents malformed secret key parsing in zen-lock
    normalized_creds = {}
    for key, value in creds.items():
        if isinstance(value, str):
            # Strip KEY= prefix if value starts with it (e.g., "JIRA_URL=https://...")
            if "=" in value:
                normalized_value = value.split("=", 1)[1]
                _print_success(f"Normalized {key}: stripped KEY= prefix")
                normalized_creds[key] = normalized_value
            else:
                normalized_creds[key] = value
        else:
            normalized_creds[key] = value

    return normalized_creds


def _check_zen_lock_keys() -> None:
    """Check that zen-lock private key exists."""
    private_key = Path.home() / ".zen-lock" / "private-key.age"
    if not private_key.exists():
        _print_error(f"zen-lock private key not found: {private_key}")
        _print_error("Generate keypair with: zen-lock keygen --output ~/.zen-lock/private-key.age")
        sys.exit(1)

    _print_success("zen-lock keys found")


def _get_public_key() -> str:
    """Get public key from zen-lock private key."""
    private_key = Path.home() / ".zen-lock" / "private-key.age"

    try:
        result = subprocess.run(
            ["zen-lock", "pubkey", "--input", str(private_key)],
            check=True,
            capture_output=True,
            text=True
        )
        public_key = result.stdout.strip()
        _print_success("Public key derived")
        return public_key
    except subprocess.CalledProcessError as e:
        _print_error(f"Failed to derive public key: {e}")
        sys.exit(1)


def _write_runtime_credentials(creds: dict, runtime_file: Path) -> None:
    """Write credentials to local runtime file with mode 0600."""
    # Create secrets directory if needed
    runtime_file.parent.mkdir(parents=True, exist_ok=True)

    # Create YAML with stringData format
    yaml_content = f"""stringData:
  JIRA_URL: "{creds.get('JIRA_URL', '')}"
  JIRA_EMAIL: "{creds.get('JIRA_EMAIL', '')}"
  JIRA_API_TOKEN: "{creds.get('JIRA_API_TOKEN', '')}"
  JIRA_PROJECT_KEY: "{creds.get('JIRA_PROJECT_KEY', '')}"
"""

    try:
        with open(runtime_file, 'w') as f:
            f.write(yaml_content)
        # Set restrictive permissions
        runtime_file.chmod(0o600)
        _print_success(f"Runtime credentials written: {runtime_file}")
    except Exception as e:
        _print_error(f"Failed to write runtime credentials: {e}")
        sys.exit(1)


def _encrypt_credentials(creds: dict, public_key: str, output_manifest: Path) -> None:
    """Encrypt credentials using zen-lock and write manifest."""
    # Create temporary YAML for encryption with metadata
    with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as temp_yaml:
        temp_yaml_path = Path(temp_yaml.name)

    # zen-lock encrypt requires metadata section in addition to stringData
    yaml_content = f"""metadata:
  name: jira-credentials
  namespace: zen-brain
stringData:
  JIRA_URL: "{creds.get('JIRA_URL', '')}"
  JIRA_EMAIL: "{creds.get('JIRA_EMAIL', '')}"
  JIRA_API_TOKEN: "{creds.get('JIRA_API_TOKEN', '')}"
  JIRA_PROJECT_KEY: "{creds.get('JIRA_PROJECT_KEY', '')}"
"""

    try:
        with open(temp_yaml_path, 'w') as f:
            f.write(yaml_content)

        _print_success(f"Temporary credentials created: {temp_yaml_path}")

        # Ensure output directory exists
        output_manifest.parent.mkdir(parents=True, exist_ok=True)

        # Encrypt with zen-lock
        _print_success("Encrypting credentials with zen-lock...")
        subprocess.run(
            [
                "zen-lock", "encrypt",
                "--pubkey", public_key,
                "--input", str(temp_yaml_path),
                "--output", str(output_manifest)
            ],
            check=True
        )
        _print_success(f"Encrypted manifest: {output_manifest}")

    except subprocess.CalledProcessError as e:
        _print_error(f"Encryption failed: {e}")
        sys.exit(1)
    finally:
        # Clean up temp file
        if temp_yaml_path.exists():
            temp_yaml_path.unlink()


def main() -> int:
    parser = argparse.ArgumentParser(description="Install Jira credentials")
    parser.add_argument("--input", required=True, help="Absolute path to jira-input.yaml")
    args = parser.parse_args()

    input_file = Path(args.input)
    if not input_file.exists():
        _print_error(f"Input file not found: {input_file}")
        sys.exit(1)

    if not input_file.is_absolute():
        _print_error("Input file must be absolute path")
        sys.exit(1)

    print("=== Jira Credential Installer ===")
    print()
    print(f"Input file: {input_file}")

    # Step 1: Validate credentials
    data = _read_yaml_file(str(input_file))
    creds = _validate_jira_credentials(data)
    _print_success("Credentials validated")

    # Step 2: Check zen-lock keys
    _check_zen_lock_keys()

    # Step 3: Get public key
    public_key = _get_public_key()

    # Step 4: Write runtime credentials
    runtime_file = Path.home() / ".zen-brain" / "secrets" / "jira.yaml"
    _write_runtime_credentials(creds, runtime_file)

    # Step 5: Encrypt credentials
    # __file__ is in scripts/, so parent.parent is the repo root
    repo_root = Path(__file__).parent.parent
    encrypted_file = repo_root / "deploy" / "zen-lock" / "jira-credentials.zenlock.yaml"
    _encrypt_credentials(creds, public_key, encrypted_file)

    # Step 6: Add allowedSubjects to manifest
    # zen-lock encrypt creates the manifest with encryptedData, but we need to add allowedSubjects
    try:
        import yaml
        with open(encrypted_file, 'r') as f:
            manifest = yaml.safe_load(f)

        # Add allowedSubjects if not present
        if 'spec' in manifest and 'allowedSubjects' not in manifest['spec']:
            manifest['spec']['allowedSubjects'] = [
                {
                    'kind': 'ServiceAccount',
                    'name': 'foreman',
                    'namespace': 'zen-brain'
                }
            ]

            # Write back the updated manifest
            with open(encrypted_file, 'w') as f:
                yaml.dump(manifest, f, default_flow_style=False, sort_keys=False)

            _print_success(f"Allowed subjects added: foreman/zen-brain")
    except Exception as e:
        _print_error(f"Failed to add allowedSubjects: {e}")
        # Don't exit, as the encryption succeeded

    print()
    print("=== Installation Complete ===")
    print()
    print("Runtime credentials:")
    print(f"  {runtime_file}")
    print()
    print("ZenLock CRD manifest:")
    print(f"  {encrypted_file}")
    print()
    print("To deploy to cluster:")
    print(f"  kubectl apply -f {encrypted_file}")
    print()
    print("To validate:")
    print("  ./bin/zen-brain office smoke-real")

    return 0


if __name__ == "__main__":
    sys.exit(main())
