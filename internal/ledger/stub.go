// Package ledger provides stub implementations for testing and development.
// These implementations are used when the full ZenLedger is not yet available.
package ledger

import (
	"context"
	"log"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

// StubLedgerClient is a stub implementation of ZenLedgerClient.
// It returns empty data for development and testing.
type StubLedgerClient struct {
	// Records model selections for later inspection
	modelSelections []ModelSelectionRecord
}

// ModelSelectionRecord records a model selection for auditing.
type ModelSelectionRecord struct {
	SessionID string
	TaskID    string
	ModelID   string
	Reason    string
	Timestamp time.Time
}

// NewStubLedgerClient creates a new StubLedgerClient.
func NewStubLedgerClient() *StubLedgerClient {
	return &StubLedgerClient{
		modelSelections: make([]ModelSelectionRecord, 0),
	}
}

// GetModelEfficiency returns empty efficiency data.
func (s *StubLedgerClient) GetModelEfficiency(ctx context.Context, projectID string, taskType string) ([]ledger.ModelEfficiency, error) {
	log.Printf("[StubLedger] GetModelEfficiency called (project=%s, taskType=%s)", projectID, taskType)
	
	// Return empty slice; planner will fall back to default model
	return []ledger.ModelEfficiency{}, nil
}

// GetCostBudgetStatus returns a default budget status.
func (s *StubLedgerClient) GetCostBudgetStatus(ctx context.Context, projectID string) (*ledger.BudgetStatus, error) {
	log.Printf("[StubLedger] GetCostBudgetStatus called (project=%s)", projectID)
	
	now := time.Now()
	return &ledger.BudgetStatus{
		ProjectID:      projectID,
		PeriodStart:    time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()),
		PeriodEnd:      time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()),
		SpentUSD:       0.0,
		BudgetLimitUSD: 1000.0,
		RemainingUSD:   1000.0,
		PercentUsed:    0.0,
	}, nil
}

// RecordPlannedModelSelection logs the model selection.
func (s *StubLedgerClient) RecordPlannedModelSelection(ctx context.Context, sessionID, taskID, modelID, reason string) error {
	log.Printf("[StubLedger] RecordPlannedModelSelection: session=%s, task=%s, model=%s, reason=%s",
		sessionID, taskID, modelID, reason)
	
	s.modelSelections = append(s.modelSelections, ModelSelectionRecord{
		SessionID: sessionID,
		TaskID:    taskID,
		ModelID:   modelID,
		Reason:    reason,
		Timestamp: time.Now(),
	})
	return nil
}

// GetModelSelections returns recorded model selections (for testing).
func (s *StubLedgerClient) GetModelSelections() []ModelSelectionRecord {
	return s.modelSelections
}

// ClearModelSelections clears recorded model selections.
func (s *StubLedgerClient) ClearModelSelections() {
	s.modelSelections = make([]ModelSelectionRecord, 0)
}

// Ensure StubLedgerClient implements ledger.ZenLedgerClient
var _ ledger.ZenLedgerClient = (*StubLedgerClient)(nil)