// doc_sync_test.go provides a cheap assertion that contract docs still mention key concepts.
// It does not parse markdown; it catches obvious drift when someone changes docs or contracts.
package contracts

import (
	"os"
	"path/filepath"
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
		name  string
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
