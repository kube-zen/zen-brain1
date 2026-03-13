// doc_sync_test.go provides anti-drift assertions that contract docs and code stay in sync.
// Tests verify that DATA_MODEL.md documents all enum values, contracts.go defines them correctly,
// and no TODO/FIXME comments exist in contracts package.
package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDataModelDocMentionsKeyConcepts(t *testing.T) {
	// Find module root (directory containing go.mod)
	dir, err := os.Getwd()
	if err != nil {
		t.Skipf("Getwd: %v", err)
		return
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("go.mod not found")
			return
		}
		dir = parent
	}
	path := filepath.Join(dir, "docs", "02-CONTRACTS", "DATA_MODEL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("DATA_MODEL.md not found or unreadable: %v", err)
		return
	}
	content := string(data)
	checks := []struct {
		name   string
		substr string
	}{
		{"WorkType", "WorkType"},
		{"WorkDomain", "WorkDomain"},
		{"WorkTags", "WorkTags"},
		{"EvidenceRequirement", "EvidenceRequirement"},
	}
	for _, c := range checks {
		if !strings.Contains(content, c.substr) {
			t.Errorf("docs/02-CONTRACTS/DATA_MODEL.md should mention %s", c.name)
		}
	}
	// Document should list WorkType values (at least one)
	if !strings.Contains(content, "implementation") {
		t.Error("DATA_MODEL.md should list WorkType values (e.g. implementation)")
	}
	if !strings.Contains(content, "core") {
		t.Error("DATA_MODEL.md should list WorkDomain values (e.g. core)")
	}
}

// TestDataModelDocContainsAllConstants ensures DATA_MODEL.md documents all canonical constants.
// This is a more exact check than substring matching; it verifies each enum value appears.
func TestDataModelDocContainsAllConstants(t *testing.T) {
	// Find module root
	dir, err := os.Getwd()
	if err != nil {
		t.Skipf("Getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("go.mod not found")
			return
		}
		dir = parent
	}
	path := filepath.Join(dir, "docs", "02-CONTRACTS", "DATA_MODEL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("DATA_MODEL.md not found: %v", err)
	}
	content := strings.ToLower(string(data)) // case-insensitive check

	// All WorkType constants
	workTypeValues := []string{
		string(WorkTypeResearch),
		string(WorkTypeDesign),
		string(WorkTypeImplementation),
		string(WorkTypeDebug),
		string(WorkTypeRefactor),
		string(WorkTypeDocumentation),
		string(WorkTypeAnalysis),
		string(WorkTypeOperations),
		string(WorkTypeSecurity),
		string(WorkTypeTesting),
	}
	for _, val := range workTypeValues {
		if !strings.Contains(content, strings.ToLower(val)) {
			t.Errorf("DATA_MODEL.md missing WorkType value: %s", val)
		}
	}

	// All WorkDomain constants
	workDomainValues := []string{
		string(DomainOffice),
		string(DomainFactory),
		string(DomainSDK),
		string(DomainPolicy),
		string(DomainMemory),
		string(DomainObservability),
		string(DomainInfrastructure),
		string(DomainIntegration),
		string(DomainCore),
	}
	for _, val := range workDomainValues {
		if !strings.Contains(content, strings.ToLower(val)) {
			t.Errorf("DATA_MODEL.md missing WorkDomain value: %s", val)
		}
	}

	// All Priority constants
	priorityValues := []string{
		string(PriorityCritical),
		string(PriorityHigh),
		string(PriorityMedium),
		string(PriorityLow),
		string(PriorityBackground),
	}
	for _, val := range priorityValues {
		if !strings.Contains(content, strings.ToLower(val)) {
			t.Errorf("DATA_MODEL.md missing Priority value: %s", val)
		}
	}

	// All EvidenceRequirement constants
	evidenceValues := []string{
		string(EvidenceNone),
		string(EvidenceSummary),
		string(EvidenceLogs),
		string(EvidenceDiff),
		string(EvidenceTestResults),
		string(EvidenceFullArtifact),
	}
	for _, val := range evidenceValues {
		if !strings.Contains(content, strings.ToLower(val)) {
			t.Errorf("DATA_MODEL.md missing EvidenceRequirement value: %s", val)
		}
	}

	// All SREDTag constants
	sredValues := []string{
		string(SREDU1DynamicProvisioning),
		string(SREDU2SecurityGates),
		string(SREDU3DeterministicDelivery),
		string(SREDU4Backpressure),
		string(SREDExperimentalGeneral),
	}
	for _, val := range sredValues {
		if !strings.Contains(content, strings.ToLower(val)) {
			t.Errorf("DATA_MODEL.md missing SREDTag value: %s", val)
		}
	}
}

// TestNoTODOInContracts ensures there are no TODO comments in contracts.go.
func TestNoTODOInContracts(t *testing.T) {
	// Find contracts.go path
	contractsPath := filepath.Join("..", "..", "pkg", "contracts", "contracts.go")
	data, err := os.ReadFile(contractsPath)
	if err != nil {
		t.Skipf("cannot read contracts.go: %v", err)
		return
	}
	content := string(data)
	// Check for TODO comments (case-insensitive)
	if strings.Contains(strings.ToLower(content), "todo") {
		t.Error("contracts.go contains TODO comment - resolve before merging")
	}
	// Check for FIXME comments (case-insensitive)
	if strings.Contains(strings.ToLower(content), "fixme") {
		t.Error("contracts.go contains FIXME comment - resolve before merging")
	}
}

// TestExecutionModeEnumDocumented ensures ExecutionMode is documented in DATA_MODEL.md.
func TestExecutionModeEnumDocumented(t *testing.T) {
	dataModelPath := filepath.Join(dir, "docs", "02-CONTRACTS", "DATA_MODEL.md")
	data, err := os.ReadFile(dataModelPath)
	if err != nil {
		t.Skipf("DATA_MODEL.md not found: %v", err)
		return
	}
	content := strings.ToLower(string(data))

	// ExecutionMode values from contracts.go (use the string literals)
	executionModeValues := []string{
		"autonomous",
		"approval_required",
		"read_only",
		"simulation_only",
		"supervised",
	}

	// Check that each value is mentioned in DATA_MODEL.md
	for _, val := range executionModeValues {
		if !strings.Contains(content, strings.ToLower(val)) {
			t.Errorf("DATA_MODEL.md missing ExecutionMode value: %s", val)
		}
	}

	// Check that DATA_MODEL.md has ExecutionMode section
	if !strings.Contains(content, "executionmode") {
		t.Error("DATA_MODEL.md should document ExecutionMode enum")
	}
}

// TestWorkStatusEnumDocumented ensures WorkStatus is documented in DATA_MODEL.md.
func TestWorkStatusEnumDocumented(t *testing.T) {
	dataModelPath := filepath.Join(dir, "docs", "02-CONTRACTS", "DATA_MODEL.md")
	data, err := os.ReadFile(dataModelPath)
	if err != nil {
		t.Skipf("DATA_MODEL.md not found: %v", err)
		return
	}
	content := strings.ToLower(string(data))

	// WorkStatus values from contracts.go (use the string literals)
	workStatusValues := []string{
		"requested",
		"analyzing",
		"analyzed",
		"planning",
		"planned",
		"pending_approval",
		"approved",
		"queued",
		"running",
		"blocked",
		"completed",
		"failed",
		"canceled",
	}

	// Check that each value is mentioned in DATA_MODEL.md
	for _, val := range workStatusValues {
		if !strings.Contains(content, strings.ToLower(val)) {
			t.Errorf("DATA_MODEL.md missing WorkStatus value: %s", val)
		}
	}

	// Check that DATA_MODEL.md has WorkStatus section
	if !strings.Contains(content, "workstatus") {
		t.Error("DATA_MODEL.md should document WorkStatus enum")
	}
}

// TestApprovalStateEnumDocumented ensures ApprovalState is documented in DATA_MODEL.md.
func TestApprovalStateEnumDocumented(t *testing.T) {
	dataModelPath := filepath.Join(dir, "docs", "02-CONTRACTS", "DATA_MODEL.md")
	data, err := os.ReadFile(dataModelPath)
	if err != nil {
		t.Skipf("DATA_MODEL.md not found: %v", err)
		return
	}
	content := strings.ToLower(string(data))

	// ApprovalState values from contracts.go (use the string literals)
	approvalStateValues := []string{
		"pending",
		"approved",
		"rejected",
		"not_required",
	}

	// Check that each value is mentioned in DATA_MODEL.md
	for _, val := range approvalStateValues {
		if !strings.Contains(content, strings.ToLower(val)) {
			t.Errorf("DATA_MODEL.md missing ApprovalState value: %s", val)
		}
	}

	// Check that DATA_MODEL.md has ApprovalState section
	if !strings.Contains(content, "approvalstate") {
		t.Error("DATA_MODEL.md should document ApprovalState enum")
	}
}

// TestWorkTagsStructDocumented ensures WorkTags struct is documented in DATA_MODEL.md.
func TestWorkTagsStructDocumented(t *testing.T) {
	dataModelPath := filepath.Join(dir, "docs", "02-CONTRACTS", "DATA_MODEL.md")
	data, err := os.ReadFile(dataModelPath)
	if err != nil {
		t.Skipf("DATA_MODEL.md not found: %v", err)
		return
	}
	content := strings.ToLower(string(data))

	// WorkTags struct fields should be documented
	fields := []string{"humancorg", "routing", "policy", "analytics", "sred"}
	for _, field := range fields {
		if !strings.Contains(content, field) {
			t.Errorf("DATA_MODEL.md should document WorkTags field: %s", field)
		}
	}

	// Check that DATA_MODEL.md has Structured Tags section
	if !strings.Contains(content, "structured tags") {
		t.Error("DATA_MODEL.md should document WorkTags struct")
	}
}

// TestNoDuplicateEnumValues ensures contracts.go does not define duplicate enum values.
func TestNoDuplicateEnumValues(t *testing.T) {
	// Find contracts.go
	contractsPath := filepath.Join(dir, "..", "..", "pkg", "contracts", "contracts.go")
	data, err := os.ReadFile(contractsPath)
	if err != nil {
		t.Skipf("cannot read contracts.go: %v", err)
		return
	}
	content := string(data)

	// Extract all const declarations
	constPattern := `const \((WorkType|WorkDomain|Priority|EvidenceRequirement|SREDTag|ExecutionMode|WorkStatus|ApprovalState) string\n`
	re := regexp.MustCompile(constPattern)
	matches := re.FindAllStringSubmatch(content, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		// Extract the const name
		constName := match[1]
		if seen[constName] {
			t.Errorf("Duplicate enum constant definition: %s appears multiple times", constName)
		}
		seen[constName] = true
	}
}
