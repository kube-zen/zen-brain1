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

// TestAllPredefinedTagsAreValid ensures every tag defined in taxonomy package validates in its category.
func TestAllPredefinedTagsAreValid(t *testing.T) {
	// HumanOrgTags
	for _, tag := range HumanOrgTags {
		if !ValidateTag(HumanOrg, tag) {
			t.Errorf("HumanOrg tag %q should be valid in HumanOrg category", tag)
		}
		if GetCategory(tag) != HumanOrg {
			t.Errorf("GetCategory(%q) = %q, want human_org", tag, GetCategory(tag))
		}
	}
	// RoutingTags
	for _, tag := range RoutingTags {
		if !ValidateTag(Routing, tag) {
			t.Errorf("Routing tag %q should be valid in Routing category", tag)
		}
		if GetCategory(tag) != Routing {
			t.Errorf("GetCategory(%q) = %q, want routing", tag, GetCategory(tag))
		}
	}
	// PolicyTags
	for _, tag := range PolicyTags {
		if !ValidateTag(Policy, tag) {
			t.Errorf("Policy tag %q should be valid in Policy category", tag)
		}
		if GetCategory(tag) != Policy {
			t.Errorf("GetCategory(%q) = %q, want policy", tag, GetCategory(tag))
		}
	}
	// AnalyticsTags
	for _, tag := range AnalyticsTags {
		if !ValidateTag(Analytics, tag) {
			t.Errorf("Analytics tag %q should be valid in Analytics category", tag)
		}
		if GetCategory(tag) != Analytics {
			t.Errorf("GetCategory(%q) = %q, want analytics", tag, GetCategory(tag))
		}
	}
	// SREDTags
	for _, tag := range SREDTags {
		if !ValidateTag(SRED, string(tag)) {
			t.Errorf("SRED tag %q should be valid in SRED category", tag)
		}
		if GetCategory(string(tag)) != SRED {
			t.Errorf("GetCategory(%q) = %q, want sred", tag, GetCategory(string(tag)))
		}
	}
}
