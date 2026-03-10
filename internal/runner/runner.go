package runner

import (
	stdctx "context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/context"
	"github.com/kube-zen/zen-brain1/internal/context/tier1"
	"github.com/kube-zen/zen-brain1/internal/context/tier3"
	"github.com/kube-zen/zen-brain1/internal/llm"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	llmtypes "github.com/kube-zen/zen-brain1/pkg/llm"
)

// Runner holds all zen-brain components.
type Runner struct {
	config      *config.Config
	gateway     *llm.Gateway
	zenctx      zenctx.ZenContext
	shutdownCtx stdctx.Context
	cancel      stdctx.CancelFunc
}

// NewRunner creates a new runner with all components initialized.
func NewRunner(cfg *config.Config) (*Runner, error) {
	r := &Runner{
		config: cfg,
	}

	// Create shutdown context
	r.shutdownCtx, r.cancel = stdctx.WithCancel(stdctx.Background())

	// Initialize ZenContext (three-tier memory)
	if err := r.initZenContext(); err != nil {
		return nil, fmt.Errorf("failed to initialize ZenContext: %w", err)
	}

	// Initialize LLM Gateway
	if err := r.initGateway(); err != nil {
		return nil, fmt.Errorf("failed to initialize LLM gateway: %w", err)
	}

	return r, nil
}

// initZenContext initializes the three-tier memory system.
func (r *Runner) initZenContext() error {
	log.Printf("[Runner] Initializing ZenContext")

	// For MVP, use minimal config (can be nil for graceful degradation)
	zenctxConfig := &context.ZenContextConfig{
		Tier1Redis: &tier1.RedisConfig{
			Addr:     r.config.ZenContext.Tier1Redis.Addr,
			Password: r.config.ZenContext.Tier1Redis.Password,
			DB:       r.config.ZenContext.Tier1Redis.DB,
		},
		Tier2QMD: &context.QMDConfig{
			RepoPath:      r.config.ZenContext.Tier2QMD.RepoPath,
			QMDBinaryPath: r.config.ZenContext.Tier2QMD.QMDBinaryPath,
			Verbose:       r.config.ZenContext.Tier2QMD.Verbose,
		},
		Tier3S3: &tier3.S3Config{
			Bucket: r.config.ZenContext.Tier3S3.Bucket,
			Region: r.config.ZenContext.Tier3S3.Region,
		},
		ClusterID: r.config.ZenContext.ClusterID,
		Verbose:   r.config.ZenContext.Verbose,
	}

	var err error
	r.zenctx, err = context.NewZenContext(zenctxConfig)
	if err != nil {
		log.Printf("[Runner] Warning: ZenContext initialization failed: %v", err)
		log.Printf("[Runner] Continuing without ZenContext (some features may be limited)")
		r.zenctx = nil
	} else {
		log.Printf("[Runner] ZenContext initialized (cluster=%s)", r.config.ZenContext.ClusterID)
	}

	return nil
}

// initGateway initializes the LLM Gateway.
func (r *Runner) initGateway() error {
	log.Printf("[Runner] Initializing LLM Gateway")

	gatewayConfig := &llm.GatewayConfig{
		LocalWorkerModel:         "qwen3.5:0.8b",
		PlannerModel:             r.config.Planner.DefaultModel,
		FallbackModel:            r.config.Planner.DefaultModel,
		LocalWorkerMaxCost:       0.01,
		PlannerMinCost:           0.10,
		LocalWorkerTimeout:       30,
		PlannerTimeout:           60,
		RequestTimeout:           120,
		LocalWorkerSupportsTools: true,
		PlannerSupportsTools:     true,
		AutoEscalateComplexTasks: true,
		RoutingPolicy:            "simple",
		EnableFallbackChain:      true,
		StrictPreferred:          false,
	}

	var err error
	r.gateway, err = llm.NewGateway(gatewayConfig)
	if err != nil {
		return err
	}

	log.Printf("[Runner] LLM Gateway initialized")
	return nil
}

// Run starts the zen-brain runner.
func (r *Runner) Run() error {
	log.Printf("[Runner] Starting zen-brain vertical slice (MVP)")

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// If work item ID provided via env, process it
	workItemID := os.Getenv("ZEN_BRAIN_WORK_ITEM")
	if workItemID != "" {
		log.Printf("[Runner] Processing work item: %s", workItemID)
		return r.processWorkItem(workItemID)
	}

	// Otherwise, run a simple test
	log.Printf("[Runner] No work item specified. Running test query...")
	return r.runTestQuery()
}

// processWorkItem processes a single work item through the vertical slice.
func (r *Runner) processWorkItem(workItemID string) error {
	ctx := r.shutdownCtx

	// Step 1: Fetch work item (mock for now)
	log.Printf("[Runner] Step 1: Fetching work item %s", workItemID)
	workItem := &contracts.WorkItem{
		ID:       workItemID,
		Title:    "Test Work Item",
		Priority: contracts.PriorityMedium,
		Status:   contracts.StatusRequested,
		Source: contracts.SourceMetadata{
			System:   "mock",
			IssueKey: workItemID,
		},
	}
	log.Printf("[Runner] Work item: %s - %s", workItem.ID, workItem.Title)

	// Step 2: Use Gateway to generate a response
	log.Printf("[Runner] Step 2: Querying LLM Gateway")
	req := llmtypes.ChatRequest{
		Messages: []llmtypes.Message{
			{Role: "system", Content: "You are zen-brain, an AI assistant for software engineering tasks."},
			{Role: "user", Content: fmt.Sprintf("Analyze this work item: %s\n\nTitle: %s", workItem.ID, workItem.Title)},
		},
		SessionID: fmt.Sprintf("test-session-%s", workItemID),
	}

	resp, err := r.gateway.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("LLM gateway failed: %w", err)
	}

	log.Printf("[Runner] LLM Response: %s", resp.Content[:min(200, len(resp.Content))])

	// Step 3: Query knowledge if ZenContext available
	if r.zenctx != nil {
		log.Printf("[Runner] Step 3: Querying knowledge base")
		chunks, err := r.zenctx.QueryKnowledge(ctx, zenctx.QueryOptions{Query: "test query", Scopes: []string{"general"}, Limit: 5})
		if err != nil {
			log.Printf("[Runner] Knowledge query failed: %v", err)
		} else {
			log.Printf("[Runner] Retrieved %d knowledge chunks", len(chunks))
		}
	}

	log.Printf("[Runner] Vertical slice complete: %s", workItemID)
	return nil
}

// runTestQuery runs a simple test query through the gateway.
func (r *Runner) runTestQuery() error {
	ctx := r.shutdownCtx

	req := llmtypes.ChatRequest{
		Messages: []llmtypes.Message{
			{Role: "user", Content: "Hello from zen-brain vertical slice! What can you help with?"},
		},
		SessionID: "test-session-mvp",
	}

	resp, err := r.gateway.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("LLM gateway failed: %w", err)
	}

	log.Printf("[Runner] Test query successful")
	log.Printf("[Runner] Response: %s", resp.Content)

	return nil
}

// Close shuts down the runner.
func (r *Runner) Close() error {
	log.Printf("[Runner] Shutting down zen-brain")

	// Gateway doesn't need explicit close
	log.Printf("[Runner] Shutdown complete")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
