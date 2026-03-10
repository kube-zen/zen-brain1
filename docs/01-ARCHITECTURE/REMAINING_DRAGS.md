# Remaining Drags

Known gaps and polish items after blocks 0–6 and 1.2.3 start. Not blocking the vertical slice; track here until addressed.

## Current list

1. ~~**internal/factory/workspace.go**~~ — Git implemented: getGitInfo runs `git rev-parse --abbrev-ref HEAD` and `git rev-parse HEAD` in workspace path.
2. ~~**Makefile**~~ — `repo-sync` implemented: `make repo-sync` runs `scripts/repo_sync.py`; set `ZEN_KB_REPO_URL` to clone, `ZEN_KB_REPO_DIR` (default `../zen-docs`) to match `tier2_qmd.repo_path`.
3. **deployments/k3d/README.md** — In-cluster deploy for core components (foreman, apiserver) still TBD; run binaries locally with kubeconfig for now.
4. **Factory** — Improved: getGitInfo real; BoundedExecutor run-tests step runs go test when go.mod present; FactoryTaskRunner optionally stores proof-of-work in Evidence Vault. Further: template tiers doc, more real steps per work type.
5. **Intelligence** — Improved: ReMe passes JournalEntries on SessionContext; Gateway records token usage to ZenLedger when SetTokenRecorder; Planner budget check and RecordPlannedModelSelection; ModelRouter in internal/intelligence for cost-aware model recommendation. Further: richer agent reasoning, more evidence classes.

## References

- Completeness: [COMPLETENESS_MATRIX.md](COMPLETENESS_MATRIX.md)
- Suggested fix order: same doc, “Suggested fix order” section
- Block progress: [BLOCK3_4_PROGRESS.md](BLOCK3_4_PROGRESS.md)
