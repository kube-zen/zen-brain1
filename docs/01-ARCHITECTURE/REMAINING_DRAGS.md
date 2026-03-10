# Remaining Drags

Known gaps and polish items after blocks 0–6 and 1.2.3 start. Not blocking the vertical slice; track here until addressed.

## Current list

1. ~~**internal/factory/workspace.go**~~ — Git implemented: getGitInfo runs `git rev-parse --abbrev-ref HEAD` and `git rev-parse HEAD` in workspace path.
2. ~~**Makefile**~~ — `repo-sync` implemented: `make repo-sync` runs `scripts/repo_sync.py`; set `ZEN_KB_REPO_URL` to clone, `ZEN_KB_REPO_DIR` (default `../zen-docs`) to match `tier2_qmd.repo_path`.
3. **deployments/k3d/README.md** — In-cluster deploy (foreman, apiserver) still TBD; "Current path" documents running binaries locally with kubeconfig (recommended until Helm/manifests exist).
4. **Factory** — getGitInfo real; BoundedExecutor real steps: "run tests"/"go test" and "build"/"go build"/"compile" when go.mod present; FactoryTaskRunner + proof-of-work to Vault; FACTORY_TEMPLATE_TIERS.md. Further: more real steps per work type.
5. **Intelligence** — ReMe wired in Foreman via -zen-context-redis (ReMeBinder); ModelRouter + hypothesis evidence in zen-brain; BLOCK5_INTELLIGENCE_COMPLETENESS.md. Further: richer agent reasoning.

## References

- Completeness: [COMPLETENESS_MATRIX.md](COMPLETENESS_MATRIX.md)
- Suggested fix order: same doc, “Suggested fix order” section
- Block progress: [BLOCK3_4_PROGRESS.md](BLOCK3_4_PROGRESS.md)
