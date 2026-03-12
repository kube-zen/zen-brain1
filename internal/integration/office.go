// Package integration provides integration helpers for wiring up zen-brain components.
package integration

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/analyzer"
	"github.com/kube-zen/zen-brain1/internal/gatekeeper"
	"github.com/kube-zen/zen-brain1/internal/kb"
	"github.com/kube-zen/zen-brain1/internal/ledger"
	llmgateway "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/internal/messagebus/redis"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/internal/session"
	"github.com/kube-zen/zen-brain1/pkg/messagebus"
)

// OfficePipeline holds all components of the Office lane.
type OfficePipeline struct {
	OfficeManager  *office.Manager
	Analyzer       analyzer.IntentAnalyzer
	SessionManager session.Manager
	Planner        planner.Planner
	Gatekeeper     gatekeeper.Gatekeeper
	MessageBus     messagebus.MessageBus // Optional Redis message bus
}

// NewOfficePipeline creates a new Office pipeline with stub dependencies.
// This is suitable for development and testing before full ledger and KB are available.
// 
// FAIL CLOSED: In production mode (ZEN_RUNTIME_PROFILE=prod), this function will fail
// if real implementations are not available. Use only in dev/test environments.
func NewOfficePipeline() (*OfficePipeline, error) {
	// FAIL CLOSED: Prevent production use of stub pipeline
	if os.Getenv("ZEN_RUNTIME_PROFILE") == "prod" || os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
		return nil, fmt.Errorf("NewOfficePipeline with stubs not allowed in production mode (ZEN_RUNTIME_PROFILE=prod or ZEN_BRAIN_STRICT_RUNTIME set)")
	}

	log.Println("Initializing Office pipeline (DEV MODE - using stubs)...")

	// 1. LLM Gateway
	log.Println("  - LLM Gateway")
	gatewayConfig := &llmgateway.GatewayConfig{
		LocalWorkerModel:         "qwen3.5:0.8b",
		PlannerModel:             "glm-4.7",
		FallbackModel:            "glm-4.7",
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
	llmGateway, err := llmgateway.NewGateway(gatewayConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM Gateway: %w", err)
	}

	// 2. Knowledge Base (stub - DEV MODE ONLY)
	// FAIL CLOSED: Real production pipelines must use qmd-backed KB
	log.Println("  - Knowledge Base (stub - DEV MODE ONLY)")
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

	// 6. Message Bus (Redis)
	log.Println("  - Message Bus (Redis)")
	redisURL := os.Getenv("ZEN_BRAIN_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
		log.Printf("    ! Using default Redis URL: %s (set ZEN_BRAIN_REDIS_URL to override)", redisURL)
	}
	var msgBus messagebus.MessageBus
	redisConfig := &redis.Config{
		RedisURL:     redisURL,
		MaxPending:   1000,
		ConsumerName: "",
		BlockTimeout: 5 * time.Second,
		ClaimTimeout: 30 * time.Second,
	}
	if os.Getenv("ZEN_BRAIN_REDIS_DISABLED") == "" {
		bus, err := redis.New(redisConfig)
		if err != nil {
			log.Printf("    ! Redis message bus initialization failed: %v (continuing without message bus)", err)
		} else {
			msgBus = bus
			log.Println("    ✓ Redis message bus initialized")
		}
	} else {
		log.Println("    (Redis disabled by environment variable)")
	}

	// 7. Office Manager
	log.Println("  - Office Manager")
	officeManager := office.NewManager()

	// 8. Planner
	log.Println("  - Planner")
	plannerConfig := &planner.Config{
		OfficeManager:    officeManager,
		Analyzer:         intentAnalyzer,
		SessionManager:   sessionManager,
		LedgerClient:     ledgerClient,
		ZenContext:       nil, // Optional for now
		DefaultModel:     "glm-4.7",
		FallbackModel:    "glm-4.7",
		MaxCostUSD:       10.0,
		RequireApproval:  false, // Auto-approve for testing
		AutoApproveCost:  5.0,
		AnalysisTimeout:  300,
		ExecutionTimeout: 3600,
		MetricsEnabled:   false,
	}
	plannerAgent, err := planner.New(plannerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Planner: %w", err)
	}

	// 9. Gatekeeper
	log.Println("  - Gatekeeper")
	gatekeeperConfig := gatekeeper.DefaultConfig()
	gatekeeperConfig.Planner = plannerAgent
	gatekeeperConfig.ReminderInterval = 0 // disabled for now
	gatekeeperConfig.EscalationInterval = 0
	gatekeeperConfig.AuditLogEnabled = false
	gatekeeperAgent, err := gatekeeper.New(gatekeeperConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gatekeeper: %w", err)
	}

	return &OfficePipeline{
		OfficeManager:  officeManager,
		Analyzer:       intentAnalyzer,
		SessionManager: sessionManager,
		Planner:        plannerAgent,
		Gatekeeper:     gatekeeperAgent,
		MessageBus:     msgBus,
	}, nil
}

// Close closes all resources held by the pipeline.
// Callers should call this when the pipeline is no longer needed.
func (p *OfficePipeline) Close() error {
	if p.MessageBus != nil {
		if err := p.MessageBus.Close(); err != nil {
			return fmt.Errorf("failed to close message bus: %w", err)
		}
	}
	// Note: other components (Planner, Gatekeeper, SessionManager) have their own Close methods.
	// Callers are responsible for closing them individually.
	return nil
}
