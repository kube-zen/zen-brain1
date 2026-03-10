// Package foreman provides a TaskRunner that executes BrainTasks via the Factory (Block 4.3).
package foreman

import (
	"context"
	"fmt"
	"time"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/evidence"
	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// FactoryTaskRunner runs a BrainTask by converting it to FactoryTaskSpec and calling Factory.ExecuteTask.
// When Vault is set, successful runs with proof-of-work are recorded as evidence (Block 4.5 / 5).
type FactoryTaskRunner struct {
	Factory factory.Factory
	Vault   evidence.Vault // optional: store proof-of-work evidence on success
}

// NewFactoryTaskRunner returns a TaskRunner that delegates to the given Factory.
func NewFactoryTaskRunner(f factory.Factory) *FactoryTaskRunner {
	return &FactoryTaskRunner{Factory: f}
}

// Run converts the BrainTask to FactoryTaskSpec, runs Factory.ExecuteTask, and returns any error.
// On success, if Vault is set and result has proof-of-work path, stores an evidence item.
func (r *FactoryTaskRunner) Run(ctx context.Context, task *v1alpha1.BrainTask) error {
	if r.Factory == nil {
		return fmt.Errorf("factory is nil")
	}
	spec := brainTaskToFactorySpec(task)
	result, err := r.Factory.ExecuteTask(ctx, spec)
	if err != nil {
		return err
	}
	if result != nil && !result.Success {
		return fmt.Errorf("task execution failed: %s", result.Error)
	}
	// Record proof-of-work evidence when vault is configured (Factory completeness)
	if result != nil && result.Success && result.ProofOfWorkPath != "" && r.Vault != nil {
		item := contracts.EvidenceItem{
			ID:          fmt.Sprintf("pow-%s-%s", task.Spec.SessionID, task.Name),
			SessionID:   task.Spec.SessionID,
			Type:        contracts.EvidenceTypeProofOfWork,
			Content:     result.ProofOfWorkPath,
			Metadata:    map[string]string{"task_id": task.Name, "work_item_id": task.Spec.WorkItemID},
			CollectedAt: time.Now(),
			CollectedBy: "factory-runner",
		}
		_ = r.Vault.Store(ctx, item) // best effort
	}
	return nil
}

func brainTaskToFactorySpec(task *v1alpha1.BrainTask) *factory.FactoryTaskSpec {
	now := time.Now()
	spec := &factory.FactoryTaskSpec{
		ID:         task.Name,
		SessionID:  task.Spec.SessionID,
		WorkItemID: task.Spec.WorkItemID,
		Title:      task.Spec.Title,
		Objective:  task.Spec.Objective,
		Constraints: task.Spec.Constraints,
		WorkType:   contracts.WorkType(task.Spec.WorkType),
		WorkDomain: contracts.WorkDomain(task.Spec.WorkDomain),
		Priority:   contracts.Priority(task.Spec.Priority),
		TimeoutSeconds: task.Spec.TimeoutSeconds,
		MaxRetries:     task.Spec.MaxRetries,
		KBScopes:       task.Spec.KBScopes,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if spec.Priority == "" {
		spec.Priority = contracts.PriorityMedium
	}
	if spec.WorkType == "" {
		spec.WorkType = contracts.WorkTypeImplementation
	}
	if spec.WorkDomain == "" {
		spec.WorkDomain = contracts.DomainFactory
	}
	return spec
}

// Ensure FactoryTaskRunner implements TaskRunner.
var _ TaskRunner = (*FactoryTaskRunner)(nil)
