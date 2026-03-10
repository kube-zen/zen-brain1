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
