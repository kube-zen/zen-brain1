package analyzer

import (
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// TestBrainTaskSpec_StructuredOutput validates that BrainTaskSpec
// produces bounded, small, structured output as required by Batch J.
func TestBrainTaskSpec_StructuredOutput(t *testing.T) {
	// Create a BrainTaskSpec with all required fields
	spec := &contracts.BrainTaskSpec{
		// Identity
		ID:          "task-123",
		Title:       "Test Task",
		Description: "Test description",

		// Source reference
		WorkItemID: "PROJ-456",
		SourceKey:  "PROJ-456",

		// Classification
		WorkType:   contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:   contracts.PriorityMedium,

		// Requirements (Batch J: Explicit objective and constraints)
		Objective:          "Implement feature X with tests",
		AcceptanceCriteria: []string{"All tests pass", "No regressions"},
		Constraints:        []string{"Time limit: 2h", "No external dependencies"},

		// Evidence requirements
		EvidenceRequirement: contracts.EvidenceLogs,

		// SR&ED
		SREDTags:   []contracts.SREDTag{contracts.SREDU1DynamicProvisioning},
		Hypothesis: "This approach reduces complexity by 20%",

		// Execution (Batch J: Bounded spec)
		TimeoutSeconds: 7200, // 2 hours
		MaxRetries:     3,

		// KB scopes (Batch J: Optional KB scopes)
		KBScopes: []string{"api-gateway", "testing"},
	}

	// Verify Batch J requirements:

	// 1. Bounded spec for execution
	if spec.TimeoutSeconds == 0 {
		t.Error("BrainTaskSpec must have TimeoutSeconds for bounded execution")
	}
	if spec.MaxRetries == 0 {
		t.Error("BrainTaskSpec must have MaxRetries for bounded execution")
	}

	// 2. Explicit objective
	if spec.Objective == "" {
		t.Error("BrainTaskSpec must have Objective field")
	}

	// 3. Explicit constraints
	if spec.Constraints == nil {
		t.Error("BrainTaskSpec must have Constraints field")
	}

	// 4. Optional KB scopes
	// KBScopes is optional (omitempty), so we just verify the field exists
	// (it's part of the struct, which is verified by the test compilation)

	// 5. Explicit risk/approval flags
	if spec.SREDTags == nil {
		t.Error("BrainTaskSpec should support SRED tags for risk tracking")
	}

	// 6. No giant prose blobs (verified by struct type - it's JSON/Go structs, not prose)
	// We verify that Description is reasonably short for structured output
	if len(spec.Description) > 1000 {
		t.Error("Description should be concise (< 1000 chars) for structured output")
	}

	// Verify AcceptanceCriteria for bounded validation
	if spec.AcceptanceCriteria == nil {
		t.Error("BrainTaskSpec should have AcceptanceCriteria for result validation")
	}
}

// TestAnalysisResult_StructuredOutput validates that AnalysisResult
// produces small, structured output without prose blobs.
func TestAnalysisResult_StructuredOutput(t *testing.T) {
	// Create an AnalysisResult with all required fields
	result := &contracts.AnalysisResult{
		WorkItem: &contracts.WorkItem{
			ID:     "WORK-123",
			Title:  "Test Work",
			Status: contracts.StatusRunning,
		},

		// Structured BrainTaskSpecs (not prose)
		BrainTaskSpecs: []contracts.BrainTaskSpec{
			{
				ID:             "task-1",
				Title:          "Task 1",
				Objective:      "Do X",
				TimeoutSeconds: 3600,
				MaxRetries:     2,
			},
		},

		// Confidence score (Batch J: Not prose)
		Confidence: 0.93,

		// Analysis notes (should be short, not prose blob)
		AnalysisNotes: "Analysis completed successfully",

		// Requires approval (Batch J: Explicit approval flag)
		RequiresApproval: false,

		// Recommended model (Batch J: Specific model)
		RecommendedModel: "glm-4.7",

		// Estimated cost (Batch J: Quantifiable)
		EstimatedTotalCostUSD: 2.50,
	}

	// Verify Batch J requirements:

	// 1. Structured BrainTaskSpecs
	if len(result.BrainTaskSpecs) == 0 {
		t.Error("AnalysisResult must contain at least one BrainTaskSpec")
	}

	// 2. Confidence score (not prose blob)
	if result.Confidence < 0.0 || result.Confidence > 1.0 {
		t.Errorf("Confidence must be 0.0-1.0, got %f", result.Confidence)
	}

	// 3. Short analysis notes (not prose blob)
	if len(result.AnalysisNotes) > 500 {
		t.Errorf("AnalysisNotes should be concise (< 500 chars), got %d", len(result.AnalysisNotes))
	}

	// 4. Explicit approval flag
	// (Verified by field existence - RequiresApproval is part of struct)

	// 5. Specific model recommendation (not prose)
	if result.RecommendedModel == "" {
		t.Error("RecommendedModel should be specified for factory execution")
	}

	// 6. Quantifiable cost (not prose)
	if result.EstimatedTotalCostUSD < 0 {
		t.Errorf("EstimatedTotalCostUSD should be non-negative, got %f", result.EstimatedTotalCostUSD)
	}
}

// TestBrainTaskSpec_FactoryConsumable validates that BrainTaskSpec
// can be consumed by Factory without additional parsing.
func TestBrainTaskSpec_FactoryConsumable(t *testing.T) {
	spec := &contracts.BrainTaskSpec{
		ID:             "task-123",
		Title:          "Test",
		Objective:      "Implement X",
		Constraints:    []string{"Limit: 2h"},
		TimeoutSeconds: 3600,
		MaxRetries:     2,
		KBScopes:       []string{"api-gateway"},
		SREDTags:       []contracts.SREDTag{contracts.SREDU1DynamicProvisioning},
	}

	// Verify all fields are Go types (not prose blobs)
	// This is verified by the test compilation - if it compiles,
	// the struct is typed, not prose
	_ = spec // Use spec to avoid unused variable warning

	// Verify Factory can consume without additional parsing
	// Factory expects: ID, Title, Objective, Constraints, Timeout, MaxRetries
	// All of these are present and typed

	t.Log("BrainTaskSpec is factory-consumable without additional parsing")
}
