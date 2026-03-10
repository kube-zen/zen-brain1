# Factory Real Review Lane

**Date:** 2026-03-10  
**Template key:** `review:real`  
**Status:** Canonical trustworthy internal-use execution lane for a real repo/worktree.

## Purpose

The `review:real` template is the **canonical trustworthy vertical-slice lane** when execution runs in a real git worktree or repo-backed workspace. It produces honest, repo-aware artifacts instead of synthetic placeholders.

## Steps

1. **Workspace and git inventory**
   - `mkdir -p review`
   - `pwd > review/workspace.txt`
   - `find . -maxdepth 4 -type f` (excluding `.git`, `.zen-*`) → `review/files.txt`
   - If git repo: write `review/git-branch.txt`, `review/git-commit.txt`, `review/git-status.txt`, `review/git-diff-stat.txt`, `review/git-diff-files.txt`
   - If not a git repo: write explicit `"not a git repo"` markers in those files

2. **Language-aware safe checks**
   - If `go.mod` exists and `go` in PATH: `go test ./... -count=1` → `review/go-test.txt` (or skipped marker)
   - If Python project (pyproject.toml/setup.py/requirements.txt) and `python3` in PATH: `python3 -m py_compile` on found `.py` files → `review/python-test.txt` (or skipped marker)
   - Do not fail the whole task if a tool is unavailable

3. **REVIEW.md from real observations**
   - Work item ID, title, objective
   - Workspace path
   - Whether repo is git-backed
   - Whether Go/Python checks ran
   - Diff stat location
   - Explicit next action recommendation

## Honesty rules

- Do not claim code was changed if no diff exists
- Do not claim tests passed if they were skipped
- Do not use placeholder prose like “analysis pending”

## Integration with proof-of-work

When the workspace contains `review/git-status.txt` and `review/git-diff-stat.txt`, the proof-of-work bundle records:

- `GitStatusPath` and `GitDiffStatPath` on `ExecutionResult` and `ProofOfWorkSummary`
- Git evidence section in the proof markdown

## When to use

- **Use `review:real`** when you want a single, defensible “what actually ran in this workspace” lane (e.g. internal verification, vertical slice, audit).
- **Other templates** (implementation:real, bugfix:real, etc.) may still be less trustworthy or more synthetic; `review:real` is the one that prioritizes honesty and repo evidence.

## Related

- Block 4 worktree execution: `internal/worktree/git_manager.go`, Foreman `ZEN_FOREMAN_USE_GIT_WORKTREE`
- Proof-of-work: `internal/factory/proof.go` (artifact paths, OutputLog from steps, git evidence)
- Template registration: `internal/factory/useful_templates.go` → `registerRealReviewTemplate()`
