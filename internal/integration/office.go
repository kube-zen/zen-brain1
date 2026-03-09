// Package integration provides integration helpers for wiring up zen-brain components.
package integration

import (
	"fmt"
	"log"

	"github.com/kube-zen/zen-brain1/internal/analyzer"
	"github.com/kube-zen/zen-brain1/internal/gatekeeper"
	"github.com/kube-zen/zen-brain1/internal/kb"
	"github.com/kube-zen/zen-brain1/internal/ledger"
	llmgateway "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/internal/session"
)

// OfficePipeline holds all components of the Office lane.
type OfficePipeline struct {
	OfficeManager *office.Manager
	Analyzer      analyzer.IntentAnalyzer
	SessionManager session.Manager
	Planner       planner.Planner
	Gatekeeper    gatekeeper.Gatekeeper
}

// NewOfficePipeline creates a new Office pipeline with stub dependencies.
// This is suitable for development and testing before full ledger and KB are available.
func NewOfficePipeline() (*OfficePipeline, error) {
	log.Println("Initializing Office pipeline...")
	
	// 1. LLM Gateway
	log.Println("  - LLM Gateway")
	gatewayConfig := &llmgateway.GatewayConfig{
		LocalWorkerModel:        "qwen3.5:0.8b",
		PlannerModel:            "glm-4.7",
		FallbackModel:           "glm-4.7",
		LocalWorkerMaxCost:     0.01,
		PlannerMinCost:          0.10,
		LocalWorkerTimeout:       30,
		PlannerTimeout:           60,
		RequestTimeout:           120,
		LocalWorkerSupportsTools: true,
		PlannerSupportsTools:     true,
		AutoEscalateComplexTasks:   true,
		RoutingPolicy:            "simple",
		EnableFallbackChain:     true,
		StrictPreferred:         false,
	}
	llmGateway, err := llmgateway.NewGateway(gatewayConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM Gateway: %w", err)
	}
	
	// 2. Knowledge Base (stub)
	log.Println("  - Knowledge Base (stub)")
	kbStore := kb.NewStubStore()
	
	// 3. Intent Analyzer
	log.Println("  - Intent Analyzer")
	analyzerConfig := analyzer.DefaultConfig()
	intentAnalyzer, err := analyzer.New(analyzerConfig, llmGateway, kbStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create Intent Analyzer: %w", err)
	}
	
	// 4. Session Manager (memory store)
	log.Println("  - Session Manager (memory)")
	sessionStore := session.NewMemoryStore()
	sessionConfig := session.DefaultConfig()
	sessionConfig.StoreType = "memory"
	sessionManager, err := session.New(sessionConfig, sessionStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create Session Manager: %w", err)
	}
	
	// 5. Ledger (stub)
	log.Println("  - Ledger (stub)")
	ledgerClient := ledger.NewStubLedgerClient()
	
	// 6. Office Manager
	log.Println("  - Office Manager")
	officeManager := office.NewManager()
	
	// 7. Planner
	log.Println("  - Planner")
	plannerConfig := &planner.Config{
		OfficeManager:   officeManager,
		Analyzer:        intentAnalyzer,
		SessionManager:  sessionManager,
		LedgerClient:    ledgerClient,
		ZenContext:      nil, // Optional for now
		DefaultModel:    "glm-4.7",
		FallbackModel:   "glm-4.7",
		MaxCostUSD:      10.0,
		RequireApproval: false, // Auto-approve for testing
		AutoApproveCost: 5.0,
		AnalysisTimeout:  300,
		ExecutionTimeout: 3600,
		MetricsEnabled:   false,
	}
	plannerAgent, err := planner.New(plannerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Planner: %w", err)
	}
	
	// 8. Gatekeeper
	log.Println("  - Gatekeeper")
	gatekeeperConfig := gatekeeper.DefaultConfig()
	gatekeeperConfig.Planner = plannerAgent
	gatekeeperConfig.ReminderInterval = 0      // disabled for now
	gatekeeperConfig.EscalationInterval = 0
	gatekeeperConfig.AuditLogEnabled = false
	gatekeeperAgent, err := gatekeeper.New(gatekeeperConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gatekeeper: %w", err)
	}
	
	return &OfficePipeline{
		OfficeManager: officeManager,
		Analyzer:      intentAnalyzer,
		SessionManager: sessionManager,
		Planner:       plannerAgent,
		Gatekeeper:    gatekeeperAgent,
	}, nil
}