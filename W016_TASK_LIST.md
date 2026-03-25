# W016: Real Task Intake List for Phase 16 Execution

**Created:** 2026-03-25 06:55 EDT
**Purpose:** L1/L2 lane operational validation with real regular tasks

---

## L1 Candidate Tasks (3)

### L1-01: Add Credential Validation Method to JiraMaterial

**Task ID:** L1-01
**Lane Target:** L1 (0.8B workhorse)
**Title:** Add IsValid() method to JiraMaterial for credential validation

**Target File:**
- `internal/secrets/jira.go` (single file edit)

**Package:** `secrets`

**Task Class:** `implementation`

**Description:**
Add a validation method `IsValid() bool` to the `JiraMaterial` struct that checks:
- BaseURL is not empty
- Email contains "@" character
- APIToken is not empty and has minimum length (10 chars)
- ProjectKey is not empty

This is a simple validation helper that will be used by credential resolution logic.

**Expected Verification Commands:**
```bash
go build ./internal/secrets/
go test ./internal/secrets/ -run TestResolveJira
go test ./internal/secrets/ -v
```

**Why it fits L1:**
- Single file edit (jira.go)
- No new architecture or types
- Target file and existing symbols explicit
- Simple validation logic
- Direct verification path via existing tests

---

### L1-02: Add Error Context to Funding Aggregator

**Task ID:** L1-02
**Lane Target:** L1 (0.8B workhorse)
**Title:** Improve error messages in Aggregator.AggregateForSessions

**Target File:**
- `internal/funding/aggregator.go` (single file edit)

**Package:** `funding`

**Task Class:** `implementation`

**Description:**
Enhance the error message in `AggregateForSessions` to include:
- Which session ID caused the failure
- Total number of sessions being processed
- Current project title

Change from:
```go
return nil, fmt.Errorf("vault is nil")
```

To something like:
```go
return nil, fmt.Errorf("vault is nil: cannot aggregate %d sessions for project %q", len(sessionIDs), projectTitle)
```

**Expected Verification Commands:**
```bash
go build ./internal/funding/
go test ./internal/funding/ -v
```

**Why it fits L1:**
- Single file edit
- Simple string formatting change
- No new types or architecture
- Existing test coverage
- Direct build/test path

---

### L1-03: Add WorkTags Validation Helper

**Task ID:** L1-03
**Lane Target:** L1 (0.8B workhorse)
**Title:** Add ValidateWorkTags() helper function to contracts package

**Target File:**
- `pkg/contracts/validate.go` (single file edit)

**Package:** `contracts`

**Task Class:** `implementation`

**Description:**
Add a validation helper function that checks a WorkTags struct for basic validity:
```go
// ValidateWorkTags returns an error if tags contain invalid values
func ValidateWorkTags(tags WorkTags) error {
    // Check that at least one tag field is set
    if tags.HumanOrg == "" && tags.Routing == "" &&
       tags.Policy == "" && tags.Analytics == "" && tags.SRED == "" {
        return fmt.Errorf("WorkTags: at least one tag must be set")
    }
    return nil
}
```

Add corresponding unit test in `pkg/contracts/validate_test.go`.

**Expected Verification Commands:**
```bash
go build ./pkg/contracts/
go test ./pkg/contracts/ -run TestValidateWorkTags -v
```

**Why it fits L1:**
- Single file edit (+ test file)
- Simple validation logic
- Uses existing WorkTags struct
- Clear test path
- Bounded scope

---

## L2 Candidate Tasks (2)

### L2-01: Enhance Jira Credential Resolution with Format Validation

**Task ID:** L2-01
**Lane Target:** L2 (2B bounded)
**Title:** Add credential format validation to Jira resolution

**Target Files:**
- `internal/secrets/jira.go` (main implementation)
- `internal/secrets/jira_test.go` (add validation tests)

**Package:** `secrets`

**Task Class:** `implementation`

**Description:**
Add comprehensive credential format validation to the `ResolveJira` function:
1. Add private helper `validateJiraURL(url string) error` that checks:
   - URL starts with http:// or https://
   - URL contains a hostname
2. Add private helper `validateJiraEmail(email string) error` that checks:
   - Email contains exactly one "@" symbol
   - Domain part has at least one "." character
3. Add private helper `validateJiraProjectKey(project string) error` that checks:
   - Project key is 2-10 characters uppercase letters
4. Integrate validation into `ResolveJira` before returning successful material
5. Add unit tests for each validation helper
6. Add integration test that validates complete resolution with invalid inputs

**Expected Verification Commands:**
```bash
go build ./internal/secrets/
go test ./internal/secrets/ -v -run TestValidateJira
go test ./internal/secrets/ -v
```

**Why it fits L2:**
- 2 files touched (jira.go + jira_test.go)
- Moderate adaptation extending existing abstractions
- Multiple validation helpers (more complex than L1)
- Still grounded in existing code structure
- Test coverage required

---

### L2-02: Add Evidence Validation to Funding Aggregator

**Task ID:** L2-02
**Lane Target:** L2 (2B bounded)
**Title:** Add evidence validity checking to funding report aggregation

**Target Files:**
- `internal/funding/aggregator.go` (main implementation)
- New test file `internal/funding/evidence_validation_test.go` (or add to existing test)
- Possibly `internal/evidence/vault.go` (for context on evidence types)

**Package:** `funding`

**Task Class:** `implementation`

**Description:**
Add validation to ensure evidence references in funding reports are valid:
1. Add helper `validateEvidenceRefs(refs []EvidenceRef, vault evidence.Vault) error` that checks:
   - Each referenced evidence ID exists in vault
   - Evidence type matches expected types (logs, diff, test_results, etc.)
   - Session ID is valid
2. Integrate validation into `AggregateForSessions` after building the report
3. Add unit tests for:
   - Valid evidence references
   - Missing evidence IDs
   - Mismatched evidence types
4. Ensure existing tests still pass

**Expected Verification Commands:**
```bash
go build ./internal/funding/
go test ./internal/funding/ -v -run TestValidate
go test ./internal/funding/ -v
```

**Why it fits L2:**
- 2-3 files potentially involved
- Moderate complexity involving vault interaction
- Extends existing aggregator abstraction
- Requires understanding of evidence package
- Still bounded with clear verification

---

## Task Selection Summary

**Total L1 Tasks:** 3 (L1-01, L1-02, L1-03)
**Total L2 Tasks:** 2 (L2-01, L2-02)

**Selection Criteria Applied:**
- ✅ L1: Single file or file + test
- ✅ L1: No new architecture
- ✅ L1: Explicit target files
- ✅ L1: Existing packages and symbols
- ✅ L1: Build/test verification exists

- ✅ L2: 1-3 files
- ✅ L2: Moderate adaptation using existing abstractions
- ✅ L2: Explicit target files
- ✅ L2: Verification path exists

**Execution Order:**
1. Warmup L1 (qwen3.5:0.8b-q4 via llama.cpp)
2. Execute L1-01, L1-02, L1-03
3. Warmup L2 (2b-q4 via llama.cpp)
4. Execute L2-01, L2-02
5. Capture telemetry and classify results
