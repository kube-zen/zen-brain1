package contracts

import (
	"testing"
)

func TestIsValidWorkType(t *testing.T) {
	valid := []WorkType{WorkTypeResearch, WorkTypeImplementation, WorkTypeDebug}
	for _, w := range valid {
		if !IsValidWorkType(w) {
			t.Errorf("IsValidWorkType(%q) = false, want true", w)
		}
	}
	if IsValidWorkType("unknown") {
		t.Error("IsValidWorkType(\"unknown\") = true, want false")
	}
}

func TestIsValidWorkDomain(t *testing.T) {
	valid := []WorkDomain{DomainCore, DomainFactory, DomainSDK}
	for _, d := range valid {
		if !IsValidWorkDomain(d) {
			t.Errorf("IsValidWorkDomain(%q) = false, want true", d)
		}
	}
	if IsValidWorkDomain("unknown") {
		t.Error("IsValidWorkDomain(\"unknown\") = true, want false")
	}
}

func TestIsValidPriority(t *testing.T) {
	valid := []Priority{PriorityCritical, PriorityHigh, PriorityMedium}
	for _, p := range valid {
		if !IsValidPriority(p) {
			t.Errorf("IsValidPriority(%q) = false, want true", p)
		}
	}
	if IsValidPriority("unknown") {
		t.Error("IsValidPriority(\"unknown\") = true, want false")
	}
}

func TestIsValidSREDTag(t *testing.T) {
	valid := []SREDTag{SREDU1DynamicProvisioning, SREDU2SecurityGates, SREDExperimentalGeneral}
	for _, tag := range valid {
		if !IsValidSREDTag(tag) {
			t.Errorf("IsValidSREDTag(%q) = false, want true", tag)
		}
	}
	if IsValidSREDTag("invalid_sred") {
		t.Error("IsValidSREDTag(\"invalid_sred\") = true, want false")
	}
}

func TestParseWorkType(t *testing.T) {
	w, err := ParseWorkType("implementation")
	if err != nil || w != WorkTypeImplementation {
		t.Errorf("ParseWorkType(\"implementation\") = %q, %v; want implementation, nil", w, err)
	}
	_, err = ParseWorkType("invalid")
	if err == nil {
		t.Error("ParseWorkType(\"invalid\") expected error")
	}
}

func TestParseWorkDomain(t *testing.T) {
	d, err := ParseWorkDomain("core")
	if err != nil || d != DomainCore {
		t.Errorf("ParseWorkDomain(\"core\") = %q, %v; want core, nil", d, err)
	}
	_, err = ParseWorkDomain("invalid")
	if err == nil {
		t.Error("ParseWorkDomain(\"invalid\") expected error")
	}
}

func TestValidateWorkTags(t *testing.T) {
	// Valid: known human_org tag
	if err := ValidateWorkTags(WorkTags{HumanOrg: []string{"team-platform"}}); err != nil {
		t.Errorf("ValidateWorkTags(team-platform) = %v", err)
	}
	// Duplicate in same category
	if err := ValidateWorkTags(WorkTags{HumanOrg: []string{"team-platform", "team-platform"}}); err == nil {
		t.Error("ValidateWorkTags(duplicate) expected error")
	}
	// Invalid SRED
	if err := ValidateWorkTags(WorkTags{SRED: []SREDTag{"bad_sred"}}); err == nil {
		t.Error("ValidateWorkTags(bad SRED) expected error")
	}
	// Valid SRED
	if err := ValidateWorkTags(WorkTags{SRED: []SREDTag{SREDU1DynamicProvisioning}}); err != nil {
		t.Errorf("ValidateWorkTags(valid SRED) = %v", err)
	}
}

func TestValidateWorkItem(t *testing.T) {
	// Nil
	if err := ValidateWorkItem(nil); err == nil {
		t.Error("ValidateWorkItem(nil) expected error")
	}
	// Missing ID
	if err := ValidateWorkItem(&WorkItem{Title: "x"}); err == nil {
		t.Error("ValidateWorkItem(no ID) expected error")
	}
	// Missing Title
	if err := ValidateWorkItem(&WorkItem{ID: "1"}); err == nil {
		t.Error("ValidateWorkItem(no title) expected error")
	}
	// Invalid WorkType
	if err := ValidateWorkItem(&WorkItem{ID: "1", Title: "t", WorkType: "invalid"}); err == nil {
		t.Error("ValidateWorkItem(invalid WorkType) expected error")
	}
	// Source.IssueKey set but Source.System empty
	if err := ValidateWorkItem(&WorkItem{ID: "1", Title: "t", Source: SourceMetadata{IssueKey: "PROJ-1"}}); err == nil {
		t.Error("ValidateWorkItem(issue_key without system) expected error")
	}
	// Valid minimal
	item := &WorkItem{ID: "1", Title: "t"}
	if err := ValidateWorkItem(item); err != nil {
		t.Errorf("ValidateWorkItem(valid minimal) = %v", err)
	}
}

func TestValidateBrainTaskSpec(t *testing.T) {
	// Nil
	if err := ValidateBrainTaskSpec(nil); err == nil {
		t.Error("ValidateBrainTaskSpec(nil) expected error")
	}
	// Missing required
	if err := ValidateBrainTaskSpec(&BrainTaskSpec{}); err == nil {
		t.Error("ValidateBrainTaskSpec(empty) expected error")
	}
	// Invalid WorkType
	if err := ValidateBrainTaskSpec(&BrainTaskSpec{
		ID: "1", Title: "t", WorkItemID: "w1", Objective: "o", WorkType: "bad", WorkDomain: DomainCore,
	}); err == nil {
		t.Error("ValidateBrainTaskSpec(invalid WorkType) expected error")
	}
	// Negative timeout
	if err := ValidateBrainTaskSpec(&BrainTaskSpec{
		ID: "1", Title: "t", WorkItemID: "w1", Objective: "o", WorkType: WorkTypeImplementation,
		WorkDomain: DomainCore, TimeoutSeconds: -1,
	}); err == nil {
		t.Error("ValidateBrainTaskSpec(negative timeout) expected error")
	}
	// Valid
	spec := &BrainTaskSpec{
		ID: "1", Title: "t", WorkItemID: "w1", Objective: "o",
		WorkType: WorkTypeImplementation, WorkDomain: DomainCore,
	}
	if err := ValidateBrainTaskSpec(spec); err != nil {
		t.Errorf("ValidateBrainTaskSpec(valid) = %v", err)
	}
}

func TestValidateExecutionConstraints(t *testing.T) {
	if err := ValidateExecutionConstraints(ExecutionConstraints{MaxCostUSD: -1}); err == nil {
		t.Error("ValidateExecutionConstraints(negative cost) expected error")
	}
	if err := ValidateExecutionConstraints(ExecutionConstraints{TimeoutSeconds: -1}); err == nil {
		t.Error("ValidateExecutionConstraints(negative timeout) expected error")
	}
	if err := ValidateExecutionConstraints(ExecutionConstraints{MaxCostUSD: 0, TimeoutSeconds: 0}); err != nil {
		t.Errorf("ValidateExecutionConstraints(valid) = %v", err)
	}
}

func TestNormalizeWorkItem(t *testing.T) {
	item := &WorkItem{
		ID:        "  id  ",
		Title:     " title ",
		DependsOn: []string{"b", "a", "a"},
		KBScopes:  []string{"x", "x", "y"},
	}
	NormalizeWorkItem(item)
	if item.ID != "id" || item.Title != "title" {
		t.Errorf("trim failed: id=%q title=%q", item.ID, item.Title)
	}
	if len(item.DependsOn) != 2 || item.DependsOn[0] != "a" || item.DependsOn[1] != "b" {
		t.Errorf("depends_on dedupe/sort: %v", item.DependsOn)
	}
	if len(item.KBScopes) != 2 || item.KBScopes[0] != "x" || item.KBScopes[1] != "y" {
		t.Errorf("kb_scopes dedupe/sort: %v", item.KBScopes)
	}
}

func TestNormalizeBrainTaskSpec(t *testing.T) {
	spec := &BrainTaskSpec{
		ID:        "  id  ",
		DependsOn: []string{"z", "a", "a"},
		KBScopes:  []string{"k1", "k1"},
		SREDTags:  []SREDTag{SREDU2SecurityGates, SREDU1DynamicProvisioning, SREDU1DynamicProvisioning},
	}
	NormalizeBrainTaskSpec(spec)
	if spec.ID != "id" {
		t.Errorf("trim id: %q", spec.ID)
	}
	if len(spec.DependsOn) != 2 || spec.DependsOn[0] != "a" || spec.DependsOn[1] != "z" {
		t.Errorf("depends_on: %v", spec.DependsOn)
	}
	if len(spec.KBScopes) != 1 || spec.KBScopes[0] != "k1" {
		t.Errorf("kb_scopes: %v", spec.KBScopes)
	}
	if len(spec.SREDTags) != 2 {
		t.Errorf("sred_tags dedupe: %v", spec.SREDTags)
	}
}
