# Credential Rails - Canonical Runbook

**Version:** 2.0
**Date:** 2026-03-29
**Status:** Active

## Single Source of Truth

| Item | Path | Purpose |
|------|------|---------|
| Age keypair | `~/zen/keys/zen-brain/credentials.key` | Encrypts all local credentials |
| Age public key | `~/zen/keys/zen-brain/credentials.pub` | Recipient for encryption |
| Encrypted bundle | `~/zen/keys/zen-brain/secrets.d/jira.enc` | **Canonical credential store** (age-encrypted JSON) |
| Token input (ephemeral) | `~/zen/keys/zen-brain/secrets.d/jira-token` | Only exists during rotation, deleted after |
| Rotation script | `scripts/zen-lock-rotate.sh` | Single command to rotate all credentials |
| Source script | `scripts/zen-lock-source.sh` | `eval "$(zen-lock-source.sh)"` for shell |
| Python helper | `scripts/common/zen_lock.py` | `from common.zen_lock import get_jira_credentials()` |
| K8s ZenLock manifest | `deploy/zen-lock/jira-credentials.zenlock.yaml` | K8s ZenLock CRD (for cluster deployments) |

## Canonical Values

| Field | Value |
|-------|-------|
| JIRA_URL | `https://zen-mesh.atlassian.net` |
| JIRA_EMAIL | `zen@zen-mesh.io` |
| JIRA_PROJECT_KEY | `ZB` |
| Auth method | Basic auth (`-u email:token`) |

## Rotation Procedure

```bash
# 1. Place new token (one line, nothing else)
echo -n 'ATATT3x...' > ~/zen/keys/zen-brain/secrets.d/jira-token

# 2. Run rotation (validates, encrypts, updates all consumers, shreds plaintext)
~/zen/zen-brain1/scripts/zen-lock-rotate.sh
```

The script handles everything:
- Validates token against Jira (HTTP 200 check)
- Encrypts into `jira.enc` with age keypair
- Updates: `/etc/zen-brain1/jira.env`, `/etc/default/zen-brain`, systemd drop-ins, K8s ZenLock manifest
- Restarts scheduler and k3d zen-brain services
- **Securely shreds the plaintext token file**

## Credential Consumers

All consumers are updated automatically by the rotation script:

| Consumer | How it gets credentials |
|----------|----------------------|
| `zen-brain1-scheduler` (systemd) | `ExecStartPre` decrypts → `/run/zen-brain1/jira-runtime.env` |
| `zen-brain` k3d service (user systemd) | `ExecStartPre` decrypts → `/run/user/$UID/zen-brain-jira.env` |
| Python scripts | `from common.zen_lock import get_jira_credentials()` |
| Shell scripts | `eval "$(scripts/zen-lock-source.sh)"` |
| Go binaries (factory-fill, scheduler) | Read `JIRA_API_TOKEN` from environment (set by systemd) |
| K8s pods (future) | ZenLock webhook → `/zen-lock/secrets/` |

## DO / DO NOT

### ✓ DO
- Use `scripts/zen-lock-rotate.sh` for ALL credential rotations
- Use `scripts/common/zen_lock.py` in Python scripts
- Use `scripts/zen-lock-source.sh` in shell scripts
- Read credentials from `~/zen/keys/zen-brain/secrets.d/jira.enc` (encrypted)
- Verify auth after rotation: `curl -u zen@zen-mesh.io:$(age -d -i ~/zen/keys/zen-brain/credentials.key ~/zen/keys/zen-brain/secrets.d/jira.enc | python3 -c "import sys,json; print(json.load(sys.stdin)['JIRA_API_TOKEN'])") https://zen-mesh.atlassian.net/rest/api/3/myself`

### ✗ DON'T
- Hardcode tokens in systemd units, shell scripts, or source code
- Read from `~/zen/DONOTASKMOREFORTHISSHIT.txt` (legacy, removed)
- Read from `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` (legacy keypair, different purpose)
- Use `zen@kube-zen.io` (wrong email — causes 401)
- Ask the operator for tokens — they are in the encrypted bundle
- Paste tokens in chat, logs, or commits
- Create alternate credential files or paths

## CI Enforcement

- `scripts/ci/guardrail_jira_email.py` — blocks `zen@kube-zen.io` in non-archived files
- Pre-commit hook checks for hardcoded tokens in staged files

## Legacy Paths (for reference, do not use)

| Old Path | Status | Replacement |
|----------|--------|-------------|
| `~/zen/DONOTASKMOREFORTHISSHIT.txt` | Removed | `~/zen/keys/zen-brain/secrets.d/jira-token` (ephemeral) |
| `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` | Still exists | `~/zen/keys/zen-brain/credentials.key` |
| `~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age` | Still exists | `~/zen/keys/zen-brain/credentials.pub` |
| `zen@kube-zen.io` | Wrong email | `zen@zen-mesh.io` |
| Hardcoded `Environment=JIRA_TOKEN=...` in systemd | Removed | zen-lock drop-in with `ExecStartPre` |

## Security Model

1. Token exists in plaintext **only during rotation** (seconds)
2. After rotation, token lives **only** in the age-encrypted `jira.enc`
3. Decryption requires the age private key (`credentials.key`, mode 600)
4. All consumers derive from this single encrypted file
5. The rotation script is the ONLY way to update credentials
6. Plaintext token is shredded with `shred -u` after successful encryption
