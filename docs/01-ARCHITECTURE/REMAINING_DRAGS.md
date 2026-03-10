# Remaining Drags

Known gaps and polish items after blocks 0–6 and 1.2.3 start. Not blocking the vertical slice; track here until addressed.

## Current list

1. **internal/factory/workspace.go** — Git not implemented (e.g. clone/status in workspace).
2. **Makefile** — `repo-sync` still TODO (clone/pull of configured KB repos for QMD population).
3. **deployments/k3d/README.md** — In-cluster deploy for core components (foreman, apiserver) still TBD; run binaries locally with kubeconfig for now.
4. **Factory** — Much better wired, but not yet “fully real, polished execution lane everywhere” (e.g. template tiers, real run-tests path).
5. **Intelligence** — Improved: ReMe passes JournalEntries on SessionContext; Gateway records token usage to ZenLedger when SetTokenRecorder; Planner budget check and RecordPlannedModelSelection; ModelRouter in internal/intelligence for cost-aware model recommendation. Further: richer agent reasoning, more evidence classes.

## References

- Completeness: [COMPLETENESS_MATRIX.md](COMPLETENESS_MATRIX.md)
- Suggested fix order: same doc, “Suggested fix order” section
- Block progress: [BLOCK3_4_PROGRESS.md](BLOCK3_4_PROGRESS.md)
