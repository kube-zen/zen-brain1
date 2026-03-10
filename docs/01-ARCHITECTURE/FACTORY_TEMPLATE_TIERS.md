# Factory template tiers

**Purpose:** Clarify which Factory execution steps are **real** (run real commands or create real artifacts) vs **scaffold/echo** (simulated progress). See COMPLETENESS_MATRIX “Factory execution” row.

## Tier 1: Generic work templates (scaffold by default)

**File:** `internal/factory/work_templates.go`

Steps in these templates are **scaffold/echo** by default: they run shell commands that mostly `echo` progress (e.g. “Analyzing bug…”, “Implementing fix…”). No real code edits or test runs are performed by the template commands themselves.

**Exception:** The **BoundedExecutor** (`internal/factory/bounded_executor.go`) overrides the command for certain step **names** when the template does not set an explicit `Command`:

- **Test:** **“Run tests”**, **“go test”**, **“test”**, **“Test feature”**, **“Execute tests”**, **“Verify refactoring”** → runs **real** `go test ./... -count=1` when `go.mod` exists; otherwise “No go.mod, skipping go test”.
- **Build:** **“build”**, **“go build”**, **“compile”** → runs **real** `go build ./...` when `go.mod` exists; otherwise “No go.mod, skipping go build”.
- **Lint:** **“lint”**, **“go vet”**, **“vet”** → runs **real** `go vet ./...` when `go.mod` exists; otherwise skipped.
- **Format:** **“format”**, **“fmt”**, **“gofmt”** → runs **real** `gofmt -l -w .` when `go.mod` exists; otherwise skipped.

So for any work type that uses those step names (and leaves `Command` unset), execution is **real** for Go workspaces; all other steps in `work_templates.go` remain echo/scaffold unless a template sets an explicit real `Command`. Feature, bug-fix, refactor, and test plans now include **format**, **lint**, **build**, and **Run tests** steps where applicable.

## Tier 2: “Useful” templates (real artifacts)

**File:** `internal/factory/useful_templates.go`

Templates registered here are labeled **real**: they create actual files and directories in the workspace, for example:

- **Real implementation:** `cmd/main.go`, `README.md`, `docs/API.md`, `cmd/main_test.go`, `PROOF_OF_WORK.md`, directory layout (`cmd`, `internal`, `pkg`, `docs`, `tests`). Also includes **build** and **Run tests** steps (real when `go.mod` present).
- **Real documentation:** `docs/README.md`, `examples/example.md`, proof-of-work summary.
- **Real bug fix / refactor / Python / review:** similar real file creation and edits.

All steps in these templates run shell commands that write files (e.g. `echo '...' > file`, `mkdir -p ...`), except steps named **build** or **Run tests** which have no `Command` and are overridden by BoundedExecutor to run real `go build ./...` and `go test ./...` when `go.mod` is present.

## Summary

| Source                 | Default behavior   | Override / exception                          |
|------------------------|--------------------|-----------------------------------------------|
| work_templates.go      | Scaffold/echo      | BoundedExecutor: test/build/lint/format step names → real `go test` / `go build` / `go vet` / `gofmt` when go.mod present |
| useful_templates.go    | Real (create files)| Same override for test steps                 |

## References

- COMPLETENESS_MATRIX.md — “Factory execution” row
- REMAINING_DRAGS.md — Factory bullet (template tiers doc)
- internal/factory/bounded_executor.go — step name → command override
- internal/factory/work_templates.go — package comment
