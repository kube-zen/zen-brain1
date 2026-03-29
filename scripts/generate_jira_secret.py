#!/usr/bin/env python3
"""
DEPRECATED: This script is no longer the canonical bootstrap path.

Use the canonical bootstrap script instead:
  deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh

Canonical flow:
  1. AGE keys: ~/zen/keys/zen-brain/credentials.key
  2. Plaintext token: ~/zen/keys/zen-brain/secrets.d/jira-token (bootstrap-only)
  3. ZenLock manifest: deploy/zen-lock/jira-credentials.zenlock.yaml

This script is kept for reference only and will be removed in a future release.
"""

import os
import subprocess
import sys
from pathlib import Path


def _print_error(msg: str) -> None:
    print(f"\033[0;31mError: {msg}\033[0m", file=sys.stderr)


def _print_success(msg: str) -> None:
    print(f"\033[0;32m✓ {msg}\033[0m")


def _print_warning(msg: str) -> None:
    print(f"\033[1;33m{msg}\033[0m")


def _print_header(msg: str) -> None:
    print(f"=== {msg} ===")


def _check_zen_lock() -> bool:
    """Check if zen-lock CLI is available."""
    try:
        subprocess.run(
            ["zen-lock", "--version"],
            capture_output=True,
            check=True
        )
        return True
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False


def _generate_keypair(private_key_path: Path, public_key_path: Path) -> None:
    """Generate zen-lock keypair if not exists."""
    if private_key_path.exists():
        return
    
    _print_warning("Generating zen-lock keypair...")
    subprocess.run(
        ["zen-lock", "keygen", "--output", str(private_key_path)],
        check=True
    )
    _print_success("Keys generated")
    print()
    
    subprocess.run(
        ["zen-lock", "pubkey", "--input", str(private_key_path)],
        check=True,
        stdout=open(public_key_path, "w")
    )
    _print_success(f"Public key: {public_key_path}")
    print()


def _prompt_credentials() -> dict:
    """Prompt user for Jira credentials."""
    print("Enter Jira credentials (will be encrypted and stored as ZenLock CRD):")
    print()
    
    jira_url = input("Jira Base URL (e.g., https://zen-mesh.atlassian.net): ").strip()
    jira_email = input("Jira Email (e.g., zen@zen-mesh.io): ").strip()
    jira_token = input("Jira API Token: ").strip()
    jira_project_key = input("Jira Project Key (e.g., ZB): ").strip()
    print()
    
    return {
        "JIRA_URL": jira_url,
        "JIRA_EMAIL": jira_email,
        "JIRA_API_TOKEN": jira_token,
        "JIRA_PROJECT_KEY": jira_project_key
    }


def _validate_credentials(creds: dict) -> bool:
    """Validate that all required credentials are present."""
    required = ["JIRA_URL", "JIRA_EMAIL", "JIRA_API_TOKEN", "JIRA_PROJECT_KEY"]
    for field in required:
        if not creds.get(field):
            return False
    return True


def _create_secret_yaml(secret_yaml_path: Path, creds: dict) -> None:
    """Create temporary secret YAML file."""
    secret_yaml_path.parent.mkdir(parents=True, exist_ok=True)
    
    content = f"""metadata:
  name: jira-credentials
  namespace: zen-brain
stringData:
  JIRA_URL: "{creds['JIRA_URL']}"
  JIRA_EMAIL: "{creds['JIRA_EMAIL']}"
  JIRA_API_TOKEN: "{creds['JIRA_API_TOKEN']}"
  JIRA_PROJECT_KEY: "{creds['JIRA_PROJECT_KEY']}"
"""
    secret_yaml_path.write_text(content)


def _encrypt_secret(secret_yaml_path: Path, encrypted_yaml_path: Path, public_key_path: Path) -> None:
    """Encrypt secret using zen-lock."""
    _print_warning("Encrypting secret with zen-lock...")
    
    public_key = public_key_path.read_text().strip()
    subprocess.run(
        [
            "zen-lock", "encrypt",
            "--pubkey", public_key,
            "--input", str(secret_yaml_path),
            "--output", str(encrypted_yaml_path)
        ],
        check=True
    )


def _create_zenlock_manifest(zenlock_yaml_path: Path, encrypted_yaml_path: Path) -> None:
    """Create ZenLock CRD manifest."""
    zenlock_content = """apiVersion: security.kube-zen.io/v1alpha1
kind: ZenLock
metadata:
  name: jira-credentials
  namespace: zen-brain
spec:
  algorithm: age
  allowedSubjects:
    - kind: ServiceAccount
      name: zb-nightshift-sa
      namespace: zen-brain
    - kind: ServiceAccount
      name: zb-reporter-sa
      namespace: zen-brain
    - kind: ServiceAccount
      name: zb-planner-sa
      namespace: zen-brain
"""
    
    zenlock_yaml_path.write_text(zenlock_content)
    
    # Append encrypted data
    encrypted_content = encrypted_yaml_path.read_text()
    if "encryptedData:" in encrypted_content:
        # Extract encrypted data section
        lines = encrypted_content.split("\n")
        start_idx = next(i for i, line in enumerate(lines) if line.strip() == "encryptedData:")
        encrypted_data_section = "\n".join(lines[start_idx:])
        with open(zenlock_yaml_path, "a") as f:
            f.write("  encryptedData:")
            f.write(encrypted_data_section[len("encryptedData:"):])


def main() -> int:
    _print_header("Jira ZenLock Secret Generator")
    print()
    
    # Check for zen-lock CLI
    if not _check_zen_lock():
        _print_error("zen-lock CLI not found")
        print("Install zen-lock first: https://github.com/kube-zen/zen-lock")
        return 1
    
    # Setup paths
    repo_root = Path(__file__).parent.parent.parent
    private_key_path = Path.home() / ".zen-lock" / "private-key.age"
    public_key_path = Path.home() / ".zen-lock" / "public-key.age"
    secrets_dir = repo_root / "deploy" / "zen-lock" / "secrets"
    secret_yaml_path = secrets_dir / "jira-secret.yaml.tmp"
    encrypted_yaml_path = secrets_dir / "jira-zenlock.yaml"
    zenlock_yaml_path = repo_root / "deploy" / "zen-lock" / "jira-zenlock.yaml"
    
    # Generate keypair if needed
    _generate_keypair(private_key_path, public_key_path)
    
    # Prompt for credentials
    creds = _prompt_credentials()
    
    # Validate credentials
    if not _validate_credentials(creds):
        _print_error("All fields are required")
        return 1
    
    # Create secret YAML
    _create_secret_yaml(secret_yaml_path, creds)
    
    # Encrypt secret
    _encrypt_secret(secret_yaml_path, encrypted_yaml_path, public_key_path)
    
    # Clean up temp file
    secret_yaml_path.unlink(missing_ok=True)
    
    # Create ZenLock manifest
    _create_zenlock_manifest(zenlock_yaml_path, encrypted_yaml_path)
    
    print()
    _print_success("ZenLock resource created")
    print(f"  Location: {zenlock_yaml_path}")
    print(f"  Encrypted secret: {encrypted_yaml_path}")
    print()
    print("To deploy to cluster:")
    print(f"  kubectl apply -f {zenlock_yaml_path}")
    print()
    print("To rotate credentials:")
    print("  1. Run this script again with new credentials")
    print(f"  2. kubectl apply -f {zenlock_yaml_path}")
    print()
    print("Allowed service accounts:")
    print("  - zb-nightshift-sa")
    print("  - zb-reporter-sa")
    print("  - zb-planner-sa")
    print()
    print(f"To add more service accounts, edit the allowedSubjects list in {zenlock_yaml_path}")
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
