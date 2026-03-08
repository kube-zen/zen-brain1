package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/kb"
	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// mockLLMProvider is a mock LLM provider for testing.
type mockLLMProvider struct {
	name           string
	response       string
	embedding      []float32
	embeddingError error
}

func (m *mockLLMProvider) Name() string {
	return m.name
}

func (m *mockLLMProvider) SupportsTools() bool {
	return false
}

func (m *mockLLMProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Content:         m.response,
		ReasoningContent: "",
		FinishReason:    "stop",
		Model:           "mock-model",
		Usage: &llm.TokenUsage{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
		LatencyMs: 100,
	}, nil
}

func (m *mockLLMProvider) ChatStream(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) (*llm.ChatResponse, error) {
	return m.Chat(ctx, req)
}

func (m *mockLLMProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	if m.embeddingError != nil {
		return nil, m.embeddingError
	}
	
	return &llm.EmbeddingResponse{
		Embedding: m.embedding,
		Model:     "mock-embedding-model",
		Dimension: len(m.embedding),
	}, nil
}

// mockKBStore is a mock knowledge base store for testing.
type mockKBStore struct{}

func (m *mockKBStore) Search(ctx context.Context, q kb.SearchQuery) ([]kb.SearchResult, error) {
	return []kb.SearchResult{}, nil
}

func (m *mockKBStore) Get(ctx context.Context, id string) (*kb.DocumentRef, error) {
	return nil, nil
}

func TestDefaultAnalyzer_Analyze(t *testing.T) {
	// Create mock LLM with classification response
	mockLLM := &mockLLMProvider{
		name: "mock",
		response: `WorkType: implementation
WorkDomain: core
Priority: medium
KBScopes: api-gateway, testing
Confidence: 0.85`,
	}

	config := DefaultConfig()
	config.EnabledStages = []Stage{StageClassification, StageFinalization} // Minimal pipeline for test
	
	analyzer, err := New(config, mockLLM, &mockKBStore{})
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	// Create a test work item
	workItem := &contracts.WorkItem{
		ID:        "TEST-123",
		Title:     "Test Work Item",
		Summary:   "Test summary",
		Body:      "This is a test work item description.",
		WorkType:  contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:  contracts.PriorityMedium,
		Status:    contracts.StatusRequested,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:    "test",
			IssueKey:  "TEST-123",
			IssueType: "Task",
		},
		EvidenceRequirement: contracts.EvidenceSummary,
	}

	// Analyze the work item
	ctx := context.Background()
	result, err := analyzer.Analyze(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to analyze work item: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("Result is nil")
	}

	if result.WorkItem.ID != workItem.ID {
		t.Errorf("Expected work item ID %s, got %s", workItem.ID, result.WorkItem.ID)
	}

	if len(result.BrainTaskSpecs) == 0 {
		t.Error("Expected at least one BrainTaskSpec")
	}

	if result.Confidence <= 0 || result.Confidence > 1 {
		t.Errorf("Confidence should be between 0 and 1, got %f", result.Confidence)
	}

	// Verify BrainTaskSpec
	spec := result.BrainTaskSpecs[0]
	if spec.WorkItemID != workItem.ID {
		t.Errorf("Expected WorkItemID %s, got %s", workItem.ID, spec.WorkItemID)
	}

	if spec.SourceKey != workItem.Source.IssueKey {
		t.Errorf("Expected SourceKey %s, got %s", workItem.Source.IssueKey, spec.SourceKey)
	}

	t.Logf("Analysis successful: %d tasks, confidence %.2f", len(result.BrainTaskSpecs), result.Confidence)
	t.Logf("BrainTaskSpec: %s - %s", spec.ID, spec.Title)
}

func TestDefaultAnalyzer_AnalyzeBatch(t *testing.T) {
	mockLLM := &mockLLMProvider{
		name: "mock",
		response: `WorkType: documentation
WorkDomain: sdk
Priority: low
KBScopes: docs, examples
Confidence: 0.9`,
	}

	config := DefaultConfig()
	config.EnabledStages = []Stage{StageClassification, StageFinalization}
	
	analyzer, err := New(config, mockLLM, &mockKBStore{})
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	// Create multiple test work items
	workItems := []*contracts.WorkItem{
		{
			ID:        "TEST-1",
			Title:     "Test 1",
			Body:      "First test item",
			WorkType:  contracts.WorkTypeDocumentation,
			WorkDomain: contracts.DomainSDK,
			Priority:  contracts.PriorityLow,
			Status:    contracts.StatusRequested,
			CreatedAt: time.Now(),
			Source: contracts.SourceMetadata{
				System:   "test",
				IssueKey: "TEST-1",
			},
		},
		{
			ID:        "TEST-2",
			Title:     "Test 2",
			Body:      "Second test item",
			WorkType:  contracts.WorkTypeImplementation,
			WorkDomain: contracts.DomainCore,
			Priority:  contracts.PriorityMedium,
			Status:    contracts.StatusRequested,
			CreatedAt: time.Now(),
			Source: contracts.SourceMetadata{
				System:   "test",
				IssueKey: "TEST-2",
			},
		},
	}

	// Analyze batch
	ctx := context.Background()
	results, err := analyzer.AnalyzeBatch(ctx, workItems)
	if err != nil {
		t.Fatalf("Failed to analyze batch: %v", err)
	}

	if len(results) != len(workItems) {
		t.Errorf("Expected %d results, got %d", len(workItems), len(results))
	}

	for i, result := range results {
		t.Logf("Result %d: %s, %d tasks, confidence %.2f", 
			i, result.WorkItem.ID, len(result.BrainTaskSpecs), result.Confidence)
	}
}

func TestStageProcessors(t *testing.T) {
	mockLLM := &mockLLMProvider{
		name: "mock",
		response: `Test response for all stages`,
	}

	workItem := &contracts.WorkItem{
		ID:        "TEST",
		Title:     "Test",
		Body:      "Test body",
		WorkType:  contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:  contracts.PriorityMedium,
		Status:    contracts.StatusRequested,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System: "test",
		},
	}

	ctx := context.Background()
	prevResults := make(map[Stage]StageResult)

	// Test classification stage
	classStage := &classificationStage{llm: mockLLM}
	classResult, err := classStage.Process(ctx, workItem, prevResults)
	if err != nil {
		t.Errorf("Classification stage failed: %v", err)
	}
	if classResult.Stage != StageClassification {
		t.Errorf("Expected stage Classification, got %s", classResult.Stage)
	}

	// Test cost estimation stage
	costStage := &costEstimationStage{llm: mockLLM, config: DefaultConfig()}
	costResult, err := costStage.Process(ctx, workItem, prevResults)
	if err != nil {
		t.Errorf("Cost estimation stage failed: %v", err)
	}
	if costResult.Stage != StageCostEstimation {
		t.Errorf("Expected stage CostEstimation, got %s", costResult.Stage)
	}

	// Check that cost is estimated
	if cost, ok := costResult.Output["estimated_cost_usd"].(float64); !ok || cost <= 0 {
		t.Errorf("Expected positive estimated cost, got %v", costResult.Output["estimated_cost_usd"])
	}

	t.Logf("Stage tests passed: classification confidence %.2f, estimated cost $%.2f", 
		classResult.Confidence, costResult.Output["estimated_cost_usd"].(float64))
}