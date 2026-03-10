# Factory template tiers

**Purpose:** Clarify which Factory execution steps are **real** (run real commands or create real artifacts) vs **scaffold/echo** (simulated progress). See COMPLETENESS_MATRIX “Factory execution” row.

## Tier 1: Generic work templates (scaffold by default)

**File:** `internal/factory/work_templates.go`

Steps in these templates are **scaffold/echo** by default: they run shell commands that mostly `echo` progress (e.g. “Analyzing bug…”, “Implementing fix…”). No real code edits or test runs are performed by the template commands themselves.

**Exception:** The **BoundedExecutor** (`internal/factory/bounded_executor.go`) overrides the command for certain step **names** when the template does not set an explicit `Command`:

- Step name **“Run tests”**, **“go test”**, or **“test”** → runs **real** `go test ./... -count=1` in the workspace when `go.mod` exists; otherwise prints “No go.mod, skipping go test”.
- Step name **“build”**, **“go build”**, or **“compile”** → runs **real** `go build ./...` when `go.mod` exists; otherwise prints “No go.mod, skipping go build”.

So for any work type that uses those step names, execution is **real** for Go workspaces; all other steps in `work_templates.go` remain echo/scaffold unless a template sets an explicit real `Command`.

## Tier 2: “Useful” templates (real artifacts)

**File:** `internal/factory/useful_templates.go`

Templates registered here are labeled **real**: they create actual files and directories in the workspace, for example:

- **Real implementation:** `cmd/main.go`, `README.md`, `docs/API.md`, `cmd/main_test.go`, `PROOF_OF_WORK.md`, directory layout (`cmd`, `internal`, `pkg`, `docs`, `tests`).
- **Real documentation:** `docs/README.md`, `examples/example.md`, proof-of-work summary.
- **Real bug fix / refactor / Python / review:** similar real file creation and edits.

All steps in these templates run shell commands that write files (e.g. `echo '...' > file`, `mkdir -p ...`). The **“Run tests”** / **“go test”** step name is still overridden by BoundedExecutor to run real `go test ./...` when `go.mod` is present.

## Summary

| Source                 | Default behavior   | Override / exception                          |
|------------------------|--------------------|-----------------------------------------------|
| work_templates.go      | Scaffold/echo      | BoundedExecutor: “Run tests”/“go test”/“test” → real `go test` when go.mod present |
| useful_templates.go    | Real (create files)| Same override for test steps                 |

## References

- COMPLETENESS_MATRIX.md — “Factory execution” row
- REMAINING_DRAGS.md — Factory bullet (template tiers doc)
- internal/factory/bounded_executor.go — step name → command override
- internal/factory/work_templates.go — package comment
