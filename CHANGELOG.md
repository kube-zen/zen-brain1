# Changelog

All notable changes to zen-brain are documented here. Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

- (No unreleased changes; next release is 1.2.3.)

## [1.2.3] - TBD

### Added

- **Block 3 complete:** Message bus, state sync (ZenContext/Session/ReMe), ZenJournal, API server (sessions, health, version), KB/QMD adapter and orchestration, ZenLedger, CockroachDB.
- **API:** `GET /api/v1/version` (version from `API_VERSION` env or `dev`).
- **Block 5:** QMD Population (Populate + docs), ReMe protocol (ReMeBinder), agent-context binding, funding evidence aggregator.
- **Block 6:** dev-clean, dev-build make targets; `docs/05-OPERATIONS/DEBUGGING.md`.
- **Block 2:** Jira webhooks, attachments, JQL; Human Gatekeeper (DefaultGatekeeper).
- **Block 0.5:** zen-sdk audit and reuse contract (`docs/01-ARCHITECTURE/BLOCK0_5_ZEN_SDK.md`).
- **Block completion matrix:** `docs/01-ARCHITECTURE/COMPLETENESS_MATRIX.md`.

### Changed

- Block 3 marked complete in progress docs; API server row in Completeness Matrix set to Real.
- Root handler lists `/api/v1/sessions`, `/api/v1/health`, `/api/v1/version`.

### Fixed

- (None in this release.)

---

[Unreleased]: https://github.com/kube-zen/zen-brain1/compare/v1.2.3...HEAD
[1.2.3]: https://github.com/kube-zen/zen-brain1/compare/v1.2.2...v1.2.3
