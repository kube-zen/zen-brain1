// contract_sync_test.go ensures CRD BrainTaskSpec stays in sync with pkg/contracts.BrainTaskSpec.
package v1alpha1

import (
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// TestBrainTaskSpecRoundtripPreservesCoreFields ensures contract -> CRD -> contract preserves core fields.
func TestBrainTaskSpecRoundtripPreservesCoreFields(t *testing.T) {
	c := &contracts.BrainTaskSpec{
		ID:                 "id-1",
		Title:              "Title",
		WorkItemID:         "WI-1",
		Objective:          "Objective",
		WorkType:           contracts.WorkTypeImplementation,
		WorkDomain:         contracts.DomainCore,
		Priority:           contracts.PriorityHigh,
		EvidenceRequirement: contracts.EvidenceLogs,
		Hypothesis:         "hypothesis",
		EstimatedCostUSD:    2.5,
		SREDTags:           []contracts.SREDTag{contracts.SREDU2SecurityGates},
	}
	api := BrainTaskSpecFromContract(c)
	back := api.ToContract()
	if back.ID != c.ID || back.Title != c.Title || back.WorkItemID != c.WorkItemID || back.Objective != c.Objective {
		t.Errorf("identity/objective roundtrip mismatch")
	}
	if back.WorkType != c.WorkType || back.WorkDomain != c.WorkDomain || back.Priority != c.Priority {
		t.Errorf("classification roundtrip mismatch")
	}
	if back.EvidenceRequirement != c.EvidenceRequirement || back.Hypothesis != c.Hypothesis {
		t.Errorf("evidence/hypothesis roundtrip mismatch")
	}
	if back.EstimatedCostUSD != c.EstimatedCostUSD {
		t.Errorf("EstimatedCostUSD roundtrip: got %f", back.EstimatedCostUSD)
	}
	if len(back.SREDTags) != len(c.SREDTags) || (len(c.SREDTags) > 0 && back.SREDTags[0] != c.SREDTags[0]) {
		t.Errorf("SREDTags roundtrip: got %v", back.SREDTags)
	}
}

// TestEstimatedCostUSDRemainsNumeric ensures we use float64, not string, for cost.
func TestEstimatedCostUSDRemainsNumeric(t *testing.T) {
	c := &contracts.BrainTaskSpec{
		ID: "x", Title: "t", WorkItemID: "w", Objective: "o",
		WorkType: contracts.WorkTypeImplementation, WorkDomain: contracts.DomainCore,
		EstimatedCostUSD: 3.14,
	}
	api := BrainTaskSpecFromContract(c)
	if api.EstimatedCostUSD != 3.14 {
		t.Errorf("EstimatedCostUSD: got %f", api.EstimatedCostUSD)
	}
	back := api.ToContract()
	if back.EstimatedCostUSD != 3.14 {
		t.Errorf("roundtrip EstimatedCostUSD: got %f", back.EstimatedCostUSD)
	}
}

// TestAllEnumValuesRoundtrip ensures every possible enum value survives contract ↔ CRD conversion.
func TestAllEnumValuesRoundtrip(t *testing.T) {
	workTypes := []contracts.WorkType{
		contracts.WorkTypeResearch,
		contracts.WorkTypeDesign,
		contracts.WorkTypeImplementation,
		contracts.WorkTypeDebug,
		contracts.WorkTypeRefactor,
		contracts.WorkTypeDocumentation,
		contracts.WorkTypeAnalysis,
		contracts.WorkTypeOperations,
		contracts.WorkTypeSecurity,
		contracts.WorkTypeTesting,
	}
	workDomains := []contracts.WorkDomain{
		contracts.DomainOffice,
		contracts.DomainFactory,
		contracts.DomainSDK,
		contracts.DomainPolicy,
		contracts.DomainMemory,
		contracts.DomainObservability,
		contracts.DomainInfrastructure,
		contracts.DomainIntegration,
		contracts.DomainCore,
	}
	priorities := []contracts.Priority{
		contracts.PriorityCritical,
		contracts.PriorityHigh,
		contracts.PriorityMedium,
		contracts.PriorityLow,
		contracts.PriorityBackground,
	}
	evidenceReqs := []contracts.EvidenceRequirement{
		contracts.EvidenceNone,
		contracts.EvidenceSummary,
		contracts.EvidenceLogs,
		contracts.EvidenceDiff,
		contracts.EvidenceTestResults,
		contracts.EvidenceFullArtifact,
	}
	sredTags := []contracts.SREDTag{
		contracts.SREDU1DynamicProvisioning,
		contracts.SREDU2SecurityGates,
		contracts.SREDU3DeterministicDelivery,
		contracts.SREDU4Backpressure,
		contracts.SREDExperimentalGeneral,
	}

	// Test each combination (simplified: test each enum category independently)
	for _, wt := range workTypes {
		c := &contracts.BrainTaskSpec{
			ID:         "test",
			Title:      "Test",
			WorkItemID: "WI-1",
			Objective:  "Test roundtrip",
			WorkType:   wt,
			WorkDomain: contracts.DomainCore,
			Priority:   contracts.PriorityMedium,
		}
		api := BrainTaskSpecFromContract(c)
		if api == nil {
			t.Errorf("BrainTaskSpecFromContract returned nil for WorkType %s", wt)
			continue
		}
		back := api.ToContract()
		if back.WorkType != wt {
			t.Errorf("WorkType roundtrip mismatch: original %s, got %s", wt, back.WorkType)
		}
	}

	for _, wd := range workDomains {
		c := &contracts.BrainTaskSpec{
			ID:         "test",
			Title:      "Test",
			WorkItemID: "WI-1",
			Objective:  "Test roundtrip",
			WorkType:   contracts.WorkTypeImplementation,
			WorkDomain: wd,
			Priority:   contracts.PriorityMedium,
		}
		api := BrainTaskSpecFromContract(c)
		if api == nil {
			t.Errorf("BrainTaskSpecFromContract returned nil for WorkDomain %s", wd)
			continue
		}
		back := api.ToContract()
		if back.WorkDomain != wd {
			t.Errorf("WorkDomain roundtrip mismatch: original %s, got %s", wd, back.WorkDomain)
		}
	}

	for _, prio := range priorities {
		c := &contracts.BrainTaskSpec{
			ID:         "test",
			Title:      "Test",
			WorkItemID: "WI-1",
			Objective:  "Test roundtrip",
			WorkType:   contracts.WorkTypeImplementation,
			WorkDomain: contracts.DomainCore,
			Priority:   prio,
		}
		api := BrainTaskSpecFromContract(c)
		if api == nil {
			t.Errorf("BrainTaskSpecFromContract returned nil for Priority %s", prio)
			continue
		}
		back := api.ToContract()
		if back.Priority != prio {
			t.Errorf("Priority roundtrip mismatch: original %s, got %s", prio, back.Priority)
		}
	}

	for _, ev := range evidenceReqs {
		c := &contracts.BrainTaskSpec{
			ID:                  "test",
			Title:               "Test",
			WorkItemID:          "WI-1",
			Objective:           "Test roundtrip",
			WorkType:            contracts.WorkTypeImplementation,
			WorkDomain:          contracts.DomainCore,
			Priority:            contracts.PriorityMedium,
			EvidenceRequirement: ev,
		}
		api := BrainTaskSpecFromContract(c)
		if api == nil {
			t.Errorf("BrainTaskSpecFromContract returned nil for EvidenceRequirement %s", ev)
			continue
		}
		back := api.ToContract()
		if back.EvidenceRequirement != ev {
			t.Errorf("EvidenceRequirement roundtrip mismatch: original %s, got %s", ev, back.EvidenceRequirement)
		}
	}

	for _, st := range sredTags {
		c := &contracts.BrainTaskSpec{
			ID:         "test",
			Title:      "Test",
			WorkItemID: "WI-1",
			Objective:  "Test roundtrip",
			WorkType:   contracts.WorkTypeImplementation,
			WorkDomain: contracts.DomainCore,
			Priority:   contracts.PriorityMedium,
			SREDTags:   []contracts.SREDTag{st},
		}
		api := BrainTaskSpecFromContract(c)
		if api == nil {
			t.Errorf("BrainTaskSpecFromContract returned nil for SREDTag %s", st)
			continue
		}
		back := api.ToContract()
		if len(back.SREDTags) != 1 || back.SREDTags[0] != st {
			t.Errorf("SREDTag roundtrip mismatch: original %s, got %v", st, back.SREDTags)
		}
	}
}
