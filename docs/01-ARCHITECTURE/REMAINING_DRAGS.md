# Remaining Drags

Known gaps and polish items after blocks 0–6 and 1.2.3 start. Not blocking the vertical slice; track here until addressed.

**Executive call:** Zen-Brain is 1.0-shaped and execution-capable (~92% blended). Remaining work is hardening real paths and reducing fallbacks, not adding missing blocks. **Prod fail-closed:** Use `ZEN_RUNTIME_PROFILE=prod` so runtime does not silently degrade (QMD mock / ledger stub). See [Completeness Matrix](COMPLETENESS_MATRIX.md) executive status.

## Current list

1. ~~**internal/factory/workspace.go**~~ — Git implemented: getGitInfo runs `git rev-parse --abbrev-ref HEAD` and `git rev-parse HEAD` in workspace path.
2. ~~**Makefile**~~ — `repo-sync` implemented: `make repo-sync` runs `scripts/repo_sync.py`; set `ZEN_KB_REPO_URL` to clone, `ZEN_KB_REPO_DIR` (default `../zen-docs`) to match `tier2_qmd.repo_path`.
3. ~~**deployments/k3d/README.md**~~ — In-cluster deploy done: Dockerfile, foreman.yaml, apiserver.yaml, make dev-image; README documents both in-cluster and local run.
4. ~~**Block 2 analysis persistence**~~ — Analysis history durable; audit fields; GetAnalysisHistory/UpdateAnalysis wired.
5. ~~**Block 2 analyzer multi-task breakdown**~~ — combineStageResults now creates multiple BrainTaskSpecs when breakdown stage outputs multiple subtasks (extractSubtasksFromBreakdown).
6. ~~**Jira logging**~~ — Webhook server error and event-channel-full now use log.Printf with [Jira] prefix.
7. **Factory** — getGitInfo real; BoundedExecutor real steps: "run tests"/"go test"/"test feature"/"verify refactoring", "build"/"go build"/"compile", "lint"/"go vet", "format"/"gofmt" when go.mod present; work_templates use format, lint, build, Run tests for feature/bug-fix/refactor/test plans; useful_templates real implementation has build+Run tests; FACTORY_TEMPLATE_TIERS.md; proof signing done. Further: more real steps per work type (e.g. staticcheck, other languages).
8. **Intelligence** — ReMe wired in Foreman via -zen-context-redis (ReMeBinder); ModelRouter + hypothesis evidence in zen-brain; selection reasoning (template/config, recency, failure-aware) in recommender and proof-of-work; see [INTELLIGENCE_MINING.md](../03-DESIGN/INTELLIGENCE_MINING.md). Further: richer agent reasoning (e.g. alternatives, deeper explanations).

## References

- Completeness: [COMPLETENESS_MATRIX.md](COMPLETENESS_MATRIX.md)
- Suggested fix order: same doc, “Suggested fix order” section
- Block progress: [PROGRESS.md](PROGRESS.md)
