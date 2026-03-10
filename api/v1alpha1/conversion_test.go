package v1alpha1

import (
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestBrainTaskSpecRoundtrip(t *testing.T) {
	c := &contracts.BrainTaskSpec{
		ID:                 "task-1",
		Title:              "Implement feature",
		WorkItemID:         "WI-1",
		Objective:          "Deliver the feature",
		WorkType:           contracts.WorkTypeImplementation,
		WorkDomain:         contracts.DomainCore,
		Priority:           contracts.PriorityHigh,
		EvidenceRequirement: contracts.EvidenceSummary,
		Hypothesis:         "SR&ED hypothesis text",
		EstimatedCostUSD:   1.5,
		TimeoutSeconds:     300,
		MaxRetries:         2,
		SREDTags:           []contracts.SREDTag{contracts.SREDU1DynamicProvisioning},
		DependsOn:          []string{"dep-1"},
		KBScopes:           []string{"kb1"},
	}
	api := BrainTaskSpecFromContract(c)
	if api == nil {
		t.Fatal("BrainTaskSpecFromContract returned nil")
	}
	if api.ID != c.ID || api.Title != c.Title || api.WorkItemID != c.WorkItemID || api.Objective != c.Objective {
		t.Errorf("identity/objective mismatch: %+v", api)
	}
	if api.WorkType != c.WorkType || api.WorkDomain != c.WorkDomain || api.Priority != c.Priority {
		t.Errorf("classification mismatch")
	}
	if api.EvidenceRequirement != c.EvidenceRequirement || api.Hypothesis != c.Hypothesis {
		t.Errorf("evidence/hypothesis mismatch")
	}
	if api.EstimatedCostUSD != c.EstimatedCostUSD || api.TimeoutSeconds != c.TimeoutSeconds || api.MaxRetries != c.MaxRetries {
		t.Errorf("execution params mismatch")
	}
	if len(api.SREDTags) != 1 || api.SREDTags[0] != c.SREDTags[0] {
		t.Errorf("SREDTags: got %v", api.SREDTags)
	}

	back := api.ToContract()
	if back.ID != c.ID || back.WorkType != c.WorkType || back.EvidenceRequirement != c.EvidenceRequirement {
		t.Errorf("roundtrip contract mismatch")
	}
	if back.EstimatedCostUSD != c.EstimatedCostUSD {
		t.Errorf("roundtrip EstimatedCostUSD: got %f", back.EstimatedCostUSD)
	}
	if len(back.SREDTags) != 1 || back.SREDTags[0] != c.SREDTags[0] {
		t.Errorf("roundtrip SREDTags: got %v", back.SREDTags)
	}
}

func TestBrainTaskSpecFromContract_nil(t *testing.T) {
	if BrainTaskSpecFromContract(nil) != nil {
		t.Error("expected nil")
	}
}

func TestBrainTaskSpec_ToContract_nil(t *testing.T) {
	var spec *BrainTaskSpec
	if spec.ToContract() != nil {
		t.Error("expected nil")
	}
}
