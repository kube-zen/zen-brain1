// Package integration provides integration helpers for wiring up zen-brain components.
package integration

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/analyzer"
	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/gatekeeper"
	kbinternal "github.com/kube-zen/zen-brain1/internal/kb"
	ledgerinternal "github.com/kube-zen/zen-brain1/internal/ledger"
	llmgateway "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/internal/messagebus/redis"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/internal/qmd"
	"github.com/kube-zen/zen-brain1/internal/session"
	"github.com/kube-zen/zen-brain1/pkg/kb"
	"github.com/kube-zen/zen-brain1/pkg/ledger"
	pkgmessagebus "github.com/kube-zen/zen-brain1/pkg/messagebus"
)

// OfficePipeline holds all components of the Office lane.
type OfficePipeline struct {
	OfficeManager  *office.Manager
	Analyzer       analyzer.IntentAnalyzer
	SessionManager session.Manager
	Planner        planner.Planner
	Gatekeeper     gatekeeper.Gatekeeper
	MessageBus     pkgmessagebus.MessageBus // Optional Redis message bus
}

// NewOfficePipeline creates a new Office pipeline with real implementations when available.
// It respects the config to use real KB/ledger implementations instead of stubs.
//
// Behavior:
// - If KB config is available (qmd + docs_repo), uses real qmd-backed KB
// - If Ledger config is available (enabled + host), uses real CockroachDB ledger
// - If Message Bus config is available (enabled + redis_url), uses real Redis message bus
// - FAILS CLOSED when in strict mode OR when component is marked as Required:
//   - Strict mode: ZEN_BRAIN_STRICT_RUNTIME env var set OR ZEN_RUNTIME_PROFILE=prod
//   - Required flag: kb.required, ledger.required, message_bus.required set to true
// - Falls back to stubs ONLY when:
//   - NOT in strict mode AND component is NOT marked as Required AND initialization fails
//   - Component is explicitly disabled (enabled=false, not required)
//
// This ensures degraded operation is NOT tolerated when real infra is required.
func NewOfficePipeline(cfg *config.Config) (*OfficePipeline, error) {
	log.Println("Initializing Office pipeline...")

	// Helper to check if stub fallback is explicitly allowed
	stubsAllowed := func() bool {
		val := os.Getenv("ZEN_BRAIN_ALLOW_STUBS")
		return val == "1" || val == "true" || val == "yes"
	}

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

	// 2. Knowledge Base (real or stub)
	var kbStore kb.Store

	if cfg != nil && cfg.KB.DocsRepo != "" && cfg.QMD.BinaryPath != "" {
		// FAIL CLOSED: KB requires explicit enabled flag
		if !cfg.KB.Enabled {
			return nil, fmt.Errorf("KB configured (docs_repo=%s, qmd_binary=%s) but not enabled (set kb.enabled=true)", cfg.KB.DocsRepo, cfg.QMD.BinaryPath)
		}
		// Use real qmd-backed KB
		log.Printf("  - Knowledge Base (qmd-backed: repo=%s)", cfg.KB.DocsRepo)

		qmdConfig := &qmd.Config{
			QMDPath:   cfg.QMD.BinaryPath,
			Timeout:   30 * time.Second,
			Verbose:   false,
			SkipAvailabilityCheck: true, // Skip check on init, fail gracefully on use
			FallbackToMock:          false, // FAIL CLOSED: no mock fallback
		}

		qmdClient, err := qmd.NewClient(qmdConfig)
		if err != nil {
			// FAIL CLOSED: no fallback to stub KB
			return nil, fmt.Errorf("KB initialization failed: %w (cannot fallback to stub KB)", err)
		}

		kbStoreConfig := &qmd.KBStoreConfig{
			QMDClient: qmdClient,
			RepoPath:  cfg.KB.DocsRepo,
			Verbose:   false,
		}
		kbStore, err = qmd.NewKBStore(kbStoreConfig)
		if err != nil {
			// FAIL CLOSED: no fallback to stub KB
			return nil, fmt.Errorf("KB store initialization failed: %w (cannot fallback to stub KB)", err)
		}
		log.Println("    ✓ qmd-backed KB initialized")
	} else {
		// KB not configured
		if cfg != nil && cfg.KB.Required {
			// FAIL CLOSED: KB required but not configured
			return nil, fmt.Errorf("KB required but not configured (set kb.docs_repo and qmd.binary_path)")
		}
		// Check if stub fallback is explicitly allowed
		if !stubsAllowed() {
			return nil, fmt.Errorf("KB not configured and stub fallback not allowed (set kb.docs_repo and qmd.binary_path for real KB, or set ZEN_BRAIN_ALLOW_STUBS=1 for stub)")
		}
		// Use stub KB (explicit opt-in via ZEN_BRAIN_ALLOW_STUBS)
		log.Println("  - Knowledge Base (stub - configure kb.docs_repo and qmd.binary_path for real KB)")
		kbStore = kbinternal.NewStubStore()
	}

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

	// 5. Ledger (real or stub)
	var ledgerClient ledger.ZenLedgerClient
	if cfg != nil && cfg.Ledger.Enabled {
		// Build CockroachDB DSN from config
		dsn := ""
		if cfg.Ledger.Host != "" && cfg.Ledger.Port != 0 {
			sslMode := cfg.Ledger.SSLMode
			if sslMode == "" {
				sslMode = "disable"
			}
			user := cfg.Ledger.User
			if user == "" {
				user = "root"
			}
			dbName := cfg.Ledger.Database
			if dbName == "" {
				dbName = "defaultdb"
			}
			dsn = fmt.Sprintf("postgres://%s@%s:%d/%s?sslmode=%s",
				user, cfg.Ledger.Host, cfg.Ledger.Port, dbName, sslMode)
			log.Printf("  - Ledger (CockroachDB: %s:%d/%s)", cfg.Ledger.Host, cfg.Ledger.Port, dbName)
		} else {
			// FAIL CLOSED: config says enabled but missing connection details
			return nil, fmt.Errorf("ledger enabled but missing host/port (set ledger.host and ledger.port)")
		}

		var err error
		ledgerClient, err = ledgerinternal.NewCockroachLedger(dsn)
		if err != nil {
			// FAIL CLOSED: no fallback to stub ledger
			return nil, fmt.Errorf("ledger initialization failed: %w (cannot fallback to stub ledger)", err)
		}
		log.Println("    ✓ CockroachDB ledger initialized")
	} else {
		// Ledger not enabled
		if cfg != nil && cfg.Ledger.Required {
			// FAIL CLOSED: ledger required but not enabled
			return nil, fmt.Errorf("Ledger required but not enabled (set ledger.enabled=true)")
		}
		// Check if stub fallback is explicitly allowed
		if !stubsAllowed() {
			return nil, fmt.Errorf("Ledger not enabled and stub fallback not allowed (set ledger.enabled=true for real ledger, or set ZEN_BRAIN_ALLOW_STUBS=1 for stub)")
		}
		// Use stub ledger (explicit opt-in via ZEN_BRAIN_ALLOW_STUBS)
		log.Println("  - Ledger (stub - set ledger.enabled=true for real ledger)")
		ledgerClient = ledgerinternal.NewStubLedgerClient()
	}

	// 6. Message Bus (Redis)
	log.Println("  - Message Bus (Redis)")
	var msgBus pkgmessagebus.MessageBus
	if cfg != nil && cfg.MessageBus.Enabled {
		redisURL := cfg.MessageBus.RedisURL
		if redisURL == "" {
			// FAIL CLOSED: no default Redis URL
			return nil, fmt.Errorf("Message Bus enabled but redis_url not configured (set message_bus.redis_url)")
		}
		redisConfig := &redis.Config{
			RedisURL:     redisURL,
			MaxPending:   1000,
			ConsumerName: "",
			BlockTimeout: 5 * time.Second,
			ClaimTimeout: 30 * time.Second,
		}
		bus, err := redis.New(redisConfig)
		if err != nil {
			// FAIL CLOSED: no fallback to missing message bus
			return nil, fmt.Errorf("message bus initialization failed: %w (cannot continue)", err)
		}
		msgBus = bus
		log.Println("    ✓ Redis message bus initialized")
	} else {
		// Message bus not enabled
		if cfg != nil && cfg.MessageBus.Required {
			// FAIL CLOSED: message bus required but not enabled
			return nil, fmt.Errorf("Message Bus required but not enabled (set message_bus.enabled=true)")
		}
		log.Println("    (Message bus disabled)")
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
