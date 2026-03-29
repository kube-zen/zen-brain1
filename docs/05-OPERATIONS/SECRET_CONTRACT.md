# Secret Management Contract

**Version:** 2.0
**Date:** 2026-03-29
**Owner:** zen-brain1 operations

## Source of Truth

All credentials for zen-brain1 are managed through **zen-lock local** (age encryption).

| Aspect | Detail |
|--------|--------|
| Encryption | age (X25519) via `~/zen/keys/zen-brain/credentials.key` |
| Canonical store | `~/zen/keys/zen-brain/secrets.d/jira.enc` (age-encrypted JSON) |
| Rotation | `scripts/zen-lock-rotate.sh` (single command) |
| Decryption (shell) | `scripts/zen-lock-source.sh` |
| Decryption (Python) | `scripts/common/zen_lock.py` |
| K8s delivery | ZenLock CRD → mutating webhook → `/zen-lock/secrets/` |
| Systemd delivery | `ExecStartPre` decrypt → `EnvironmentFile` |

## Current Secrets

| Secret | Local Source | K8s Source | Status |
|--------|-------------|------------|--------|
| JIRA_API_TOKEN | `~/zen/keys/zen-brain/secrets.d/jira.enc` | `deploy/zen-lock/jira-credentials.zenlock.yaml` | ✅ Active |
| JIRA_EMAIL | `zen@zen-mesh.io` (hardcoded in scripts) | Same | ✅ Active |
| JIRA_URL | `https://zen-mesh.atlassian.net` (hardcoded) | Same | ✅ Active |

## Canonical Values

- **Email:** `zen@zen-mesh.io` (NOT `zen@kube-zen.io` — that causes 401)
- **Auth:** Basic auth (`-u email:token`), not Bearer
- **Project key:** `ZB`

## Rules

1. **Single source of truth.** The encrypted bundle at `~/zen/keys/zen-brain/secrets.d/jira.enc` is the canonical store.
2. **Rotation via script only.** Run `scripts/zen-lock-rotate.sh` — it validates, encrypts, updates consumers, shreds plaintext.
3. **No hardcoded tokens.** Never in systemd units, shell scripts, or source code.
4. **Python scripts use `common.zen_lock`.** Import from `scripts/common/zen_lock.py`.
5. **Shell scripts use `zen-lock-source.sh`.** Eval its output.
6. **K8s pods use ZenLock.** Webhook injects at `/zen-lock/secrets/`.
7. **No asking for tokens.** They are in the encrypted bundle. Decrypt if needed.

## Rotation

1. Generate new token at https://id.atlassian.com/manage-profile/security/api-tokens
2. `echo -n 'NEW_TOKEN' > ~/zen/keys/zen-brain/secrets.d/jira-token`
3. `~/zen/zen-brain1/scripts/zen-lock-rotate.sh`
4. Done. Plaintext is shredded, all consumers updated.

See `docs/05-OPERATIONS/CREDENTIAL_RAILS.md` for detailed runbook.
