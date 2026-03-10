# ADR 0009: Approve zen-sdk ownership gate allowlist for domain usage

## Title

Approve zen-sdk ownership gate allowlist for domain usage and approved wrappers.

## Status

**Accepted** (2026-03-10)

## Context

The zen-sdk ownership rule requires that cross-cutting concerns (receiptlog, dedup, retry, health, scheduler, etc.) be implemented via zen-sdk, not reimplemented in zen-brain. The gate `scripts/ci/zen_sdk_ownership_gate.py` enforces this by flagging directories that match SDK package names and .go files that contain SDK-like keywords (e.g. Retry, Schedule, HealthCheck, EventBus).

Some zen-brain files legitimately use the same vocabulary for **domain concepts** (e.g. factory step retry, session EventPublisher, Foreman scheduling) while **importing** zen-sdk for the actual implementation. One path (`internal/journal/receiptlog`) is an approved **wrapper** around zen-sdk receiptlog. Without an explicit, approved allowlist, the gate would block these files and hinder development despite no actual reimplementation.

## Decision

- **Allowlist:** Maintain `scripts/ci/zen_sdk_allowlist.txt` as the single source of allowed paths. Each entry is justified as either:
  - **Domain usage:** File uses SDK-like keywords for domain types/interfaces (EventBus, Schedule, Retry, Health, etc.) but does not reimplement SDK logic; zen-sdk is used where the capability is needed.
  - **Approved wrapper:** Path is a thin wrapper or adapter around a zen-sdk package (e.g. `internal/journal/receiptlog` around zen-sdk receiptlog).
- **Governance:** New allowlist entries require a comment in the file. Any entry that is a true local reimplementation of an SDK concern requires a separate ADR. The allowlist and rationale are documented in [DEPENDENCIES.md](../DEPENDENCIES.md).
- **Enforcement:** The zen_sdk_ownership gate runs in CI and in the pre-commit hook so every commit is checked.

## Consequences

- **Positive:** Clear governance for when local code may use SDK-like names; repo is operationally and governance-clean; gate prevents accidental reimplementation.
- **Negative:** New files that use similar keywords may be flagged until allowlisted; developers must add a comment when adding an entry.
- **Neutral:** Allowlist is the contract; DEPENDENCIES.md and this ADR explain the rationale.

## Alternatives Considered

- **No allowlist:** Gate would fail on many valid domain files; rejected.
- **Move all flagged logic to zen-sdk:** Would push domain-specific types (e.g. session EventPublisher) into the SDK; rejected.
- **Disable the gate:** Would lose protection against real reimplementation; rejected.

## Related Decisions

- [DEPENDENCIES.md](../DEPENDENCIES.md) – zen-sdk reuse contract and allowlist subsection
- [REPO_RULES.md](../../04-DEVELOPMENT/REPO_RULES.md) – Enforcement and allowlist usage
- [RECOMMENDED_NEXT_STEPS.md](../RECOMMENDED_NEXT_STEPS.md) – Wave 2 governance completion

## References

- `scripts/ci/zen_sdk_ownership_gate.py` – Gate implementation
- `scripts/ci/zen_sdk_allowlist.txt` – Allowed paths and inline comments
