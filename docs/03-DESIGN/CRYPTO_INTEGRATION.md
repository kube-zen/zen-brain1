# Crypto Integration

**Status:** ✅ Implemented (see `internal/cryptoutil/`)

*Note: This document was originally a plan. The implementation has been completed.*

---

## Implementation

Crypto (Age encryption) is implemented via `internal/cryptoutil/`, a wrapper around `zen-sdk/pkg/crypto`:

- **Package:** `internal/cryptoutil/crypto.go`
- **Usage:** `cmd/apiserver/main.go`, `cmd/controller/main.go`
- **Environment Variables:** `AGE_PUBLIC_KEY`, `AGE_PRIVATE_KEY`
- **Status:** Enabled when keys are set, disabled otherwise

---

