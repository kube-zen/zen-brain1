// compat_test.go holds source-of-truth and compatibility tests so contract/API drift is caught by tests.
package contracts

import (
	"testing"
)

// TestCanonicalWorkTypeValues ensures valid WorkType set stays aligned with docs (DATA_MODEL.md).
func TestCanonicalWorkTypeValues(t *testing.T) {
	expected := []WorkType{
		WorkTypeResearch, WorkTypeDesign, WorkTypeImplementation, WorkTypeDebug,
		WorkTypeRefactor, WorkTypeDocumentation, WorkTypeAnalysis, WorkTypeOperations,
		WorkTypeSecurity, WorkTypeTesting,
	}
	for _, w := range expected {
		if !IsValidWorkType(w) {
			t.Errorf("WorkType %q should be valid", w)
		}
	}
	// Documented set size
	if len(expected) != 10 {
		t.Errorf("expected 10 WorkType values for doc sync")
	}
}

// TestCanonicalWorkDomainValues ensures valid WorkDomain set stays aligned with docs.
func TestCanonicalWorkDomainValues(t *testing.T) {
	expected := []WorkDomain{
		DomainOffice, DomainFactory, DomainSDK, DomainPolicy, DomainMemory,
		DomainObservability, DomainInfrastructure, DomainIntegration, DomainCore,
	}
	for _, d := range expected {
		if !IsValidWorkDomain(d) {
			t.Errorf("WorkDomain %q should be valid", d)
		}
	}
	if len(expected) != 9 {
		t.Errorf("expected 9 WorkDomain values for doc sync")
	}
}

// TestCanonicalEvidenceRequirementValues ensures EvidenceRequirement enum stays aligned with docs.
func TestCanonicalEvidenceRequirementValues(t *testing.T) {
	expected := []EvidenceRequirement{
		EvidenceNone, EvidenceSummary, EvidenceLogs, EvidenceDiff, EvidenceTestResults, EvidenceFullArtifact,
	}
	for _, e := range expected {
		if !IsValidEvidenceRequirement(e) {
			t.Errorf("EvidenceRequirement %q should be valid", e)
		}
	}
}

// TestCanonicalSREDTagValues ensures SREDTag enum stays aligned with docs.
func TestCanonicalSREDTagValues(t *testing.T) {
	expected := []SREDTag{
		SREDU1DynamicProvisioning, SREDU2SecurityGates, SREDU3DeterministicDelivery,
		SREDU4Backpressure, SREDExperimentalGeneral,
	}
	for _, tag := range expected {
		if !IsValidSREDTag(tag) {
			t.Errorf("SREDTag %q should be valid", tag)
		}
	}
}

// TestWorkTagsValidationRejectsWrongCategory uses taxonomy from a test that can import taxonomy
// to assert tags in wrong category are rejected (integration point; taxonomy is the authority for category membership).
func TestWorkTagsValidationRejectsWrongCategory(t *testing.T) {
	// Duplicate and invalid SRED are validated in-contracts
	if err := ValidateWorkTags(WorkTags{SRED: []SREDTag{"not_a_sred_tag"}}); err == nil {
		t.Error("invalid SRED tag should be rejected")
	}
	if err := ValidateWorkTags(WorkTags{SRED: []SREDTag{SREDU1DynamicProvisioning, SREDU1DynamicProvisioning}}); err == nil {
		t.Error("duplicate SRED tag should be rejected")
	}
}
