// Package foreman provides a TaskRunner that executes BrainTasks via the Factory (Block 4.3).
package foreman

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/evidence"
	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/internal/mlq"
	"github.com/kube-zen/zen-brain1/internal/worktree"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// FactoryTaskRunnerConfig configures FactoryTaskRunner (Block 4 execution).
type FactoryTaskRunnerConfig struct {
	RuntimeDir          string // e.g. /tmp/zen-brain-factory
	WorkspaceHome       string // e.g. /tmp/zen-brain-factory (workspaces created under WorkspaceHome/workspaces)
	PreferRealTemplates bool   // when true, empty workDomain + supported workType -> use "real" domain

	// Git worktree execution (Block 4 real worktree lane)
	UseGitWorktree       bool   // when true, use real git worktrees from SourceRepoPath
	SourceRepoPath       string // path to git repo (required if UseGitWorktree)
	WorktreeBasePath     string // base dir for worktrees (default <RuntimeDir>/worktrees)
	SourceRef            string // git ref e.g. HEAD or main (default HEAD)
	ReuseSessionWorktree bool   // reuse one worktree per session when true

	// LLM-powered Factory execution (ZB-022D)
	EnableFactoryLLM    bool   // when true, Factory uses LLM-powered templates instead of shell-only
	LLMBaseURL          string // Ollama endpoint (e.g. http://host.k3d.internal:11434)
	LLMModel            string // model name (default qwen3.5:0.8b for CPU inference)
	LLMTimeoutSeconds    int    // timeout for LLM requests (default 2700s=45m for qwen3.5:0.8b normal lane; only controlled-failure uses short timeout)
	LLMEnableThinking   bool   // enable chain-of-thought (default false for CPU path)
}

// FactoryTaskRunner runs a BrainTask by converting it to FactoryTaskSpec and calling Factory.ExecuteTask.
// When Vault is set, successful runs with proof-of-work are recorded as evidence (Block 4.5 / 5).
type FactoryTaskRunner struct {
	Factory factory.Factory
	cfg     FactoryTaskRunnerConfig
	Vault   evidence.Vault // optional: store proof-of-work evidence on success
}

// NewFactoryTaskRunner builds a FactoryTaskRunner from config (creates/owns FactoryImpl).
// When UseGitWorktree is true, uses real git worktrees from SourceRepoPath; otherwise uses WorkspaceHome/workspaces.
// 
// FAIL CLOSED: RuntimeDir must be explicitly set (via ZEN_FOREMAN_RUNTIME_DIR or config).
// No default /tmp fallback allowed, even in dev mode (consistent with cmd/foreman).
func NewFactoryTaskRunner(cfg FactoryTaskRunnerConfig) (*FactoryTaskRunner, error) {
	// FAIL CLOSED: Require explicit RuntimeDir
	if cfg.RuntimeDir == "" {
		// Check environment variable
		cfg.RuntimeDir = os.Getenv("ZEN_FOREMAN_RUNTIME_DIR")
	}
	if cfg.RuntimeDir == "" {
		// No dev-mode fallback - fail closed always
		return nil, fmt.Errorf("FAIL CLOSED: RuntimeDir not set (set ZEN_FOREMAN_RUNTIME_DIR or config.RuntimeDir)")
	}
	if cfg.WorkspaceHome == "" {
		cfg.WorkspaceHome = cfg.RuntimeDir
	}
	var workspaceManager factory.WorkspaceManager
	if cfg.UseGitWorktree {
		if cfg.SourceRepoPath == "" {
			return nil, fmt.Errorf("UseGitWorktree requires SourceRepoPath")
		}
		if cfg.WorktreeBasePath == "" {
			cfg.WorktreeBasePath = filepath.Join(cfg.RuntimeDir, "worktrees")
		}
		if cfg.SourceRef == "" {
			cfg.SourceRef = "HEAD"
		}
		gitCfg := worktree.GitManagerConfig{
			RepoPath:       cfg.SourceRepoPath,
			BasePath:       cfg.WorktreeBasePath,
			DefaultRef:     cfg.SourceRef,
			BranchPrefix:   "ai",
			ReuseSessionWT: cfg.ReuseSessionWorktree,
		}
		gitMgr, err := worktree.NewGitManager(gitCfg)
		if err != nil {
			return nil, fmt.Errorf("git worktree manager: %w", err)
		}
		workspaceManager = factory.NewGitWorkspaceManager(gitMgr)
	} else {
		workspaceManager = factory.NewWorkspaceManager(cfg.WorkspaceHome)
	}
	executor := factory.NewBoundedExecutor()
	powManager := factory.NewProofOfWorkManager(cfg.RuntimeDir)
	f := factory.NewFactory(workspaceManager, executor, powManager, cfg.RuntimeDir)

	// Enable LLM-powered Factory execution (ZB-022D)
	if cfg.EnableFactoryLLM {
		log.Printf("[FactoryTaskRunner] Enabling LLM-powered Factory execution")
		if cfg.LLMModel == "" {
			cfg.LLMModel = "qwen3.5:0.8b"
		}
		if cfg.LLMTimeoutSeconds == 0 {
			cfg.LLMTimeoutSeconds = 2700 // ZB-024: 45 minutes for qwen3.5:0.8b normal lane
		}

		// ZB-023: FAIL-CLOSED - Enforce thinking default for local CPU path
		// Local CPU path should default to thinking=false unless explicitly overridden
		// This is enforced via cfg.LLMEnableThinking default in main.go (false)
		if !cfg.LLMEnableThinking {
			log.Printf("[FactoryTaskRunner] ZB-023: Local CPU path - thinking disabled (recommended for CPU inference)")
		} else {
			log.Printf("[FactoryTaskRunner] ZB-023 WARNING: Local CPU path - thinking enabled (may degrade performance on CPU)")
		}

		// ZB-MLQ-RESCUE: MLQ owns backend selection
		// Create MLQ selector
		mlqSelector := mlq.NewMLQ()

		// Register Ollama backend (current working path)
		ollamaBackend := mlq.Backend{
			ProviderName:   "ollama",
			Model:         cfg.LLMModel,
			BaseURL:       cfg.LLMBaseURL,
			TimeoutSeconds: cfg.LLMTimeoutSeconds,
			EnableThinking: cfg.LLMEnableThinking,
		}
		mlqSelector.RegisterBackend("ollama", ollamaBackend)

		// Select backend via MLQ
		criteria := mlq.SelectionCriteria{
			PreferredProvider: cfg.LLMProvider, // If set, use this specific backend
		}
		selectedBackend, err := mlqSelector.Select(criteria)
		if err != nil {
			return nil, fmt.Errorf("MLQ backend selection failed: %w", err)
		}

		log.Printf("[FactoryTaskRunner] MLQ selected backend: provider=%s model=%s timeout=%ds thinking=%v",
			selectedBackend.ProviderName, selectedBackend.Model, selectedBackend.TimeoutSeconds, selectedBackend.EnableThinking)

		// Create provider based on MLQ selection
		var provider llm.Provider
		providerName, model, baseURL, timeoutSeconds := selectedBackend.CreateProvider()
		
		switch providerName {
		case "ollama":
			provider = llm.NewOllamaProvider(baseURL, model, timeoutSeconds, "30m")
		default:
			return nil, fmt.Errorf("unsupported provider from MLQ: %s", providerName)
		}

		// Create LLM generator with enforced config
		llmGenConfig := factory.DefaultLLMGeneratorConfig(ollamaProvider)
		llmGenConfig.Model = selectedBackend.Model
		llmGenConfig.EnableThinking = selectedBackend.EnableThinking
		llmGenConfig.Timeout = time.Duration(selectedBackend.TimeoutSeconds) * time.Second

		llmGenerator, err := factory.NewLLMGenerator(llmGenConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create LLM generator: %w", err)
		}

		// Enable LLM in Factory (this registers LLM templates)
		f.SetLLMGenerator(llmGenerator)
		log.Printf("[FactoryTaskRunner] LLM-powered Factory execution enabled via MLQ (provider=%s model=%s)",
			selectedBackend.ProviderName, selectedBackend.Model)
	}

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
	mode := "workspace"
	if r.cfg.UseGitWorktree {
		mode = "git-worktree"
	}
	outcome := &TaskRunOutcome{
		WorkspacePath:   result.WorkspacePath,
		ProofOfWorkPath: result.ProofOfWorkPath,
		TemplateKey:     result.TemplateKey,
		FilesChanged:    len(result.FilesChanged),
		ResultStatus:    string(result.Status),
		Recommendation:  result.Recommendation,
		DurationSeconds: int64(result.Duration.Seconds()),
		ExecutionMode:   mode,
	}
	log.Printf("[FactoryTaskRunner] task_id=%s execution_mode=%s workspace=%s template=%s", task.Name, mode, result.WorkspacePath, result.TemplateKey)
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
Runner)(nil)
