# Release process (1.2.3)

## Version and changelog

- **VERSION** (repo root) — Single source of truth for release version (e.g. `1.2.3`). The Makefile uses it when present for `build` / `build-all` (main binary gets this via ldflags).
- **CHANGELOG.md** — [Keep a Changelog](https://keepachangelog.com/) style. Update `[Unreleased]` and add a dated `[1.2.3]` section when cutting the release.

## Cutting 1.2.3

1. Set **VERSION** to `1.2.3` (already set when starting 1.2.3).
2. In **CHANGELOG.md**: move “Unreleased” entries into `[1.2.3] - YYYY-MM-DD` and add the compare link for the tag.
3. Commit: `git add VERSION CHANGELOG.md && git commit -m "chore: release 1.2.3"`.
4. Tag: `git tag -s v1.2.3 -m "Release 1.2.3"` (or `git tag v1.2.3`).
5. Push: `git push origin main && git push origin v1.2.3`.

## API server version

For the API server binary, set `API_VERSION=1.2.3` (env or deploy config) so `GET /api/v1/version` returns this release.

## Optional

- Build artifacts: `make build-all` produces binaries with version in the main binary (when built with VERSION file).
- Pre-commit: fix repo checks so release commits don’t require `--no-verify`.
