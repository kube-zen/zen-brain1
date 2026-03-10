package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

func TestStubLedgerClient_GetModelEfficiency(t *testing.T) {
	client := NewStubLedgerClient()
	ctx := context.Background()

	// GetModelEfficiency should return empty slice (fallback to default)
	efficiency, err := client.GetModelEfficiency(ctx, "test-project", "code-generation")
	if err != nil {
		t.Fatalf("GetModelEfficiency failed: %v", err)
	}
	if efficiency == nil {
		t.Error("Expected non-nil efficiency slice")
	}
	if len(efficiency) != 0 {
		t.Errorf("Expected empty efficiency slice, got %d items", len(efficiency))
	}
}

func TestStubLedgerClient_GetCostBudgetStatus(t *testing.T) {
	client := NewStubLedgerClient()
	ctx := context.Background()

	// GetCostBudgetStatus should return default budget
	status, err := client.GetCostBudgetStatus(ctx, "test-project")
	if err != nil {
		t.Fatalf("GetCostBudgetStatus failed: %v", err)
	}
	if status == nil {
		t.Fatal("Expected non-nil budget status")
	}

	// Verify default values
	if status.ProjectID != "test-project" {
		t.Errorf("Expected ProjectID 'test-project', got '%s'", status.ProjectID)
	}
	if status.BudgetLimitUSD != 1000.0 {
		t.Errorf("Expected BudgetLimitUSD 1000.0, got %f", status.BudgetLimitUSD)
	}
	if status.SpentUSD != 0.0 {
		t.Errorf("Expected SpentUSD 0.0, got %f", status.SpentUSD)
	}
	if status.RemainingUSD != 1000.0 {
		t.Errorf("Expected RemainingUSD 1000.0, got %f", status.RemainingUSD)
	}
	if status.PercentUsed != 0.0 {
		t.Errorf("Expected PercentUsed 0.0, got %f", status.PercentUsed)
	}

	// Verify time bounds are reasonable
	now := time.Now()
	if status.PeriodStart.After(now) {
		t.Error("PeriodStart should not be in the future")
	}
	if status.PeriodEnd.Before(now) {
		t.Error("PeriodEnd should not be in the past")
	}
}

func TestStubLedgerClient_RecordPlannedModelSelection(t *testing.T) {
	client := NewStubLedgerClient()
	ctx := context.Background()

	// Record a model selection
	err := client.RecordPlannedModelSelection(ctx, "session-1", "task-1", "qwen3.5:0.8b", "cost-optimal")
	if err != nil {
		t.Fatalf("RecordPlannedModelSelection failed: %v", err)
	}

	// Verify it was recorded
	selections := client.GetModelSelections()
	if len(selections) != 1 {
		t.Fatalf("Expected 1 selection, got %d", len(selections))
	}

	// Verify selection details
	sel := selections[0]
	if sel.SessionID != "session-1" {
		t.Errorf("Expected SessionID 'session-1', got '%s'", sel.SessionID)
	}
	if sel.TaskID != "task-1" {
		t.Errorf("Expected TaskID 'task-1', got '%s'", sel.TaskID)
	}
	if sel.ModelID != "qwen3.5:0.8b" {
		t.Errorf("Expected ModelID 'qwen3.5:0.8b', got '%s'", sel.ModelID)
	}
	if sel.Reason != "cost-optimal" {
		t.Errorf("Expected Reason 'cost-optimal', got '%s'", sel.Reason)
	}
	if sel.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}

	// Record another selection
	client.RecordPlannedModelSelection(ctx, "session-2", "task-2", "glm-5", "quality-optimal")
	selections = client.GetModelSelections()
	if len(selections) != 2 {
		t.Errorf("Expected 2 selections, got %d", len(selections))
	}
}

func TestStubLedgerClient_ClearModelSelections(t *testing.T) {
	client := NewStubLedgerClient()
	ctx := context.Background()

	// Add some selections
	client.RecordPlannedModelSelection(ctx, "s1", "t1", "model1", "reason1")
	client.RecordPlannedModelSelection(ctx, "s2", "t2", "model2", "reason2")

	// Verify they exist
	selections := client.GetModelSelections()
	if len(selections) != 2 {
		t.Fatalf("Expected 2 selections, got %d", len(selections))
	}

	// Clear them
	client.ClearModelSelections()

	// Verify they're gone
	selections = client.GetModelSelections()
	if len(selections) != 0 {
		t.Errorf("Expected 0 selections after clear, got %d", len(selections))
	}
}

func TestStubLedgerClient_Record(t *testing.T) {
	recorder := NewStubLedgerClient()
	ctx := context.Background()

	// Record should not error
	record := ledger.TokenRecord{
		SessionID:    "session-1",
		ModelID:      "qwen3.5:0.8b",
		TokensInput:  100,
		TokensOutput: 50,
	}

	err := recorder.Record(ctx, record)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}
}

func TestStubLedgerClient_RecordBatch(t *testing.T) {
	recorder := NewStubLedgerClient()
	ctx := context.Background()

	records := []ledger.TokenRecord{
		{
			SessionID:    "session-1",
			ModelID:      "model1",
			TokensInput:  100,
			TokensOutput: 50,
		},
		{
			SessionID:    "session-2",
			ModelID:      "model2",
			TokensInput:  200,
			TokensOutput: 75,
		},
	}

	// RecordBatch should not error
	err := recorder.RecordBatch(ctx, records)
	if err != nil {
		t.Fatalf("RecordBatch failed: %v", err)
	}
}

func TestStubLedgerClient_MultipleProjects(t *testing.T) {
	client := NewStubLedgerClient()
	ctx := context.Background()

	// Each project should get its own budget status
	status1, _ := client.GetCostBudgetStatus(ctx, "project-1")
	status2, _ := client.GetCostBudgetStatus(ctx, "project-2")

	if status1.ProjectID == status2.ProjectID {
		t.Error("Different projects should have different budget statuses")
	}
}
