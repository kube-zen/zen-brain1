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

// FactoryTaskRunnerConfig configures FactoryTaskRunner (Block 4 execution).
type FactoryTaskRunnerConfig struct {
	RuntimeDir          string // e.g. /tmp/zen-brain-factory
	WorkspaceHome       string // e.g. /tmp/zen-brain-factory (workspaces created under WorkspaceHome/workspaces)
	PreferRealTemplates bool   // when true, empty workDomain + supported workType -> use "real" domain
}

// FactoryTaskRunner runs a BrainTask by converting it to FactoryTaskSpec and calling Factory.ExecuteTask.
// When Vault is set, successful runs with proof-of-work are recorded as evidence (Block 4.5 / 5).
type FactoryTaskRunner struct {
	Factory factory.Factory
	cfg     FactoryTaskRunnerConfig
	Vault   evidence.Vault // optional: store proof-of-work evidence on success
}

// NewFactoryTaskRunner builds a FactoryTaskRunner from config (creates/owns FactoryImpl).
func NewFactoryTaskRunner(cfg FactoryTaskRunnerConfig) (*FactoryTaskRunner, error) {
	if cfg.RuntimeDir == "" {
		cfg.RuntimeDir = "/tmp/zen-brain-factory"
	}
	if cfg.WorkspaceHome == "" {
		cfg.WorkspaceHome = cfg.RuntimeDir
	}
	workspaceManager := factory.NewWorkspaceManager(cfg.WorkspaceHome)
	executor := factory.NewBoundedExecutor()
	powManager := factory.NewProofOfWorkManager(cfg.RuntimeDir)
	f := factory.NewFactory(workspaceManager, executor, powManager, cfg.RuntimeDir)
	return &FactoryTaskRunner{Factory: f, cfg: cfg}, nil
}

// NewFactoryTaskRunnerWithFactory returns a TaskRunner that delegates to the given Factory (e.g. for tests).
func NewFactoryTaskRunnerWithFactory(f factory.Factory) *FactoryTaskRunner {
	return &FactoryTaskRunner{Factory: f}
}

// Run converts the BrainTask to FactoryTaskSpec, runs Factory.ExecuteTask, and returns outcome and error.
// On success, if Vault is set and result has proof-of-work path, stores an evidence item.
func (r *FactoryTaskRunner) Run(ctx context.Context, task *v1alpha1.BrainTask) (*TaskRunOutcome, error) {
	if r.Factory == nil {
		return nil, fmt.Errorf("factory is nil")
	}
	spec := r.brainTaskToFactorySpec(task)
	result, err := r.Factory.ExecuteTask(ctx, spec)
	if err != nil {
		return nil, err
	}
	if result != nil && !result.Success {
		return nil, fmt.Errorf("task execution failed: %s", result.Error)
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
	outcome := &TaskRunOutcome{
		WorkspacePath:   result.WorkspacePath,
		ProofOfWorkPath: result.ProofOfWorkPath,
		TemplateKey:     result.TemplateKey,
		FilesChanged:    len(result.FilesChanged),
		ResultStatus:    string(result.Status),
		Recommendation:  result.Recommendation,
		DurationSeconds: int64(result.Duration.Seconds()),
	}
	return outcome, nil
}

func (r *FactoryTaskRunner) brainTaskToFactorySpec(task *v1alpha1.BrainTask) *factory.FactoryTaskSpec {
	now := time.Now()
	spec := &factory.FactoryTaskSpec{
		ID:              task.Name,
		SessionID:       task.Spec.SessionID,
		WorkItemID:      task.Spec.WorkItemID,
		Title:           task.Spec.Title,
		Objective:       task.Spec.Objective,
		Constraints:     task.Spec.Constraints,
		WorkType:        task.Spec.WorkType,
		WorkDomain:      task.Spec.WorkDomain,
		Priority:        task.Spec.Priority,
		TimeoutSeconds:  task.Spec.TimeoutSeconds,
		MaxRetries:      task.Spec.MaxRetries,
		KBScopes:        task.Spec.KBScopes,
		CreatedAt:       now,
		UpdatedAt:       now,
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
	// Prefer real templates when workDomain is empty and we have a matching real template (Block 4).
	if r.cfg.PreferRealTemplates && task.Spec.WorkDomain == "" && hasRealTemplateForWorkType(string(spec.WorkType)) {
		spec.WorkDomain = contracts.WorkDomain("real")
	}
	return spec
}

// hasRealTemplateForWorkType returns true for work types that have a "real" template in useful_templates.
func hasRealTemplateForWorkType(workType string) bool {
	switch workType {
	case "implementation", "docs", "debug", "refactor", "review", "bugfix":
		return true
	default:
		return false
	}
}

// Ensure FactoryTaskRunner implements TaskRunner.
var _ TaskRunner = (*FactoryTaskRunner)(nil)
