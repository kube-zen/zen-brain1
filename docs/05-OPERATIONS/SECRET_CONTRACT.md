> **NOTE:** This document references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only.

# Secret Management Contract

**Version:** 1.0
**Date:** 2026-03-26
**Owner:** zen-brain1 operations

## Source of Truth

All production credentials for zen-brain1 are managed through **zen-lock**.

| Aspect | Detail |
|--------|--------|
| Encryption | age (X25519) via zen-lock |
| Source CRD | `ZenLock` custom resources in Git |
| Private key | `~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age` |
| Public key | `~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age` |
| Runtime delivery | Mutating webhook → ephemeral K8s Secret → `/zen-lock/secrets/` |
| Cleanup | Ephemeral secrets deleted on Pod termination + orphan TTL |

## Current Secrets

| Secret | Path | Used By | Status |
|--------|------|---------|--------|
| JIRA_API_TOKEN | `/zen-lock/secrets/JIRA_API_TOKEN` | foreman deployment | ✅ Active |
| JIRA_EMAIL | `/zen-lock/secrets/JIRA_EMAIL` | foreman deployment | ✅ Active |
| JIRA_URL | `/zen-lock/secrets/JIRA_URL` | foreman deployment | ✅ Active |

## Local Runtime Secrets

Local useful-task runtime (24/7 scheduler, L1/L2 workers) currently requires NO external credentials:
- L1/L2 are local llama.cpp instances with no API keys
- No external LLM API keys are used in local mode
- Ollama (L0 fallback) also requires no credentials

When external credentials are needed (cluster mode, future provider keys):
- Encrypt via zen-lock
- Deliver via webhook injection
- Read from `/zen-lock/secrets/`

## Rules

1. **No ad hoc credential files in production.** All secrets flow through zen-lock.
2. **Bootstrap tokens are one-time use.** Delete after successful zen-lock encryption (see `docs/credential-rails.md`).
3. **Local mode is credential-free.** L1/L2 workers need no keys.
4. **Cluster mode requires zen-lock.** Any credential needed by a Pod must come from `/zen-lock/secrets/`.
5. **Future zen-flow jobs use the same contract.** JobFlow step Pods receive secrets via zen-lock injection.

## Rotation

1. Obtain new token/credential
2. Place in bootstrap file (`~/zen/DONOTASKMOREFORTHISSHIT.txt`)
3. Run bootstrap script (`deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh`)
4. Verify access
5. Delete bootstrap file
6. Commit encrypted CRD to Git

See `docs/credential-rails.md` for detailed procedure.
