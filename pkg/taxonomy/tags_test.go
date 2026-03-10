// Package taxonomy tests tag categories and Block 1 WorkTags/taxonomy integration.
package taxonomy

import (
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// TestValidateTag_WrongCategory ensures tags in the wrong category are rejected (Block 1: category authority).
func TestValidateTag_WrongCategory(t *testing.T) {
	// team-platform belongs to HumanOrg, not Routing
	if ValidateTag(Routing, "team-platform") {
		t.Error("team-platform should not be valid for Routing category")
	}
	if GetCategory("team-platform") != HumanOrg {
		t.Errorf("GetCategory(team-platform) = %q, want human_org", GetCategory("team-platform"))
	}
	// llm-required belongs to Routing, not Policy
	if ValidateTag(Policy, "llm-required") {
		t.Error("llm-required should not be valid for Policy category")
	}
}

// TestValidateTag_ValidCategory ensures known tags validate in their category.
func TestValidateTag_ValidCategory(t *testing.T) {
	if !ValidateTag(HumanOrg, "team-platform") {
		t.Error("team-platform should be valid for HumanOrg")
	}
	if !ValidateTag(Routing, "llm-required") {
		t.Error("llm-required should be valid for Routing")
	}
	if !ValidateTag(SRED, string(contracts.SREDU1DynamicProvisioning)) {
		t.Error("SRED tag should be valid for SRED category")
	}
}

// TestWorkTagsWithTaxonomy ensures WorkTags that use taxonomy-valid tags pass contracts.ValidateWorkTags.
// contracts.ValidateWorkTags does not import taxonomy (no cycle); callers that want strict category
// checks can validate each tag with taxonomy.ValidateTag(category, tag) in addition.
func TestWorkTagsWithTaxonomy(t *testing.T) {
	// Valid per taxonomy in correct categories
	tags := contracts.WorkTags{
		HumanOrg:  []string{"team-platform"},
		Routing:   []string{"llm-required"},
		SRED:      []contracts.SREDTag{contracts.SREDU2SecurityGates},
	}
	if err := contracts.ValidateWorkTags(tags); err != nil {
		t.Errorf("ValidateWorkTags(valid tags): %v", err)
	}
	// Duplicate in same category should still be rejected by contracts
	dup := contracts.WorkTags{HumanOrg: []string{"team-platform", "team-platform"}}
	if err := contracts.ValidateWorkTags(dup); err == nil {
		t.Error("ValidateWorkTags(duplicate in category) should error")
	}
}
