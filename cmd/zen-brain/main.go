package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/analyzer"
	"github.com/kube-zen/zen-brain1/internal/config"
	internalcontext "github.com/kube-zen/zen-brain1/internal/context"
	"github.com/kube-zen/zen-brain1/internal/context/tier1"
	"github.com/kube-zen/zen-brain1/internal/context/tier3"
	"github.com/kube-zen/zen-brain1/internal/evidence"
	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/internal/integration"
	"github.com/kube-zen/zen-brain1/internal/intelligence"
	internalLedger "github.com/kube-zen/zen-brain1/internal/ledger"
	llmgateway "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/internal/messagebus/redis"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/office/jira"
	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/internal/runtime"
	"github.com/kube-zen/zen-brain1/internal/session"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/ledger"
	"github.com/kube-zen/zen-brain1/pkg/llm"
	"github.com/kube-zen/zen-brain1/pkg/messagebus"
)

// Build-time variables (set via Makefile)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	fmt.Printf("zen-brain %s (built %s)\n", Version, BuildTime)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "test":
		runTestQuery()

	case "vertical-slice":
		runVerticalSlice()

	case "intelligence":
		runIntelligence()

	case "office":
		runOfficeCommand()

	case "analyze":
		runAnalyzeCommand()

	case "factory":
		runFactoryCommand()

	case "runtime":
		runRuntime()

	case "self-improvement":
		runSelfImprovementCommand()

	case "compliance":
		runComplianceCommand()

	case "tools":
		runToolsCommand()

	case "worker":
		runWorkerCommand()

	case "version":
		printVersion()

	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: zen-brain <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  test           Run a simple LLM Gateway test query")
	fmt.Println("  vertical-slice Run end-to-end vertical slice (Jira → analyze → plan → execute → update)")
	fmt.Println("  intelligence   Block 5 intelligence: mine, analyze, recommend")
	fmt.Println("  office         Office doctor, search, fetch, watch (Jira)")
	fmt.Println("  analyze        Block 2 analyzer: work-item, history, latest, compare")
	fmt.Println("  factory        Block 4 factory: execute, status, proof, workspaces, cleanup")
	fmt.Println("  runtime        Runtime doctor, report, ping (Block 3 capabilities)")
	fmt.Println("  self-improvement Self-improvement loop: discover, claim, classify, execute safe tasks")
	fmt.Println("  compliance      Governance/Compliance: reporter, gap-hunter (SR&ED/IRAP/ISO/SOC)")
	fmt.Println("  tools          Diagnostic tools: metrics, diagnostics")
	fmt.Println("  worker         Workers: remediate, batch, ticketize")
	fmt.Println("  version        Print version information")
	fmt.Println()
	fmt.Println("For vertical-slice command:")
	fmt.Println("  zen-brain vertical-slice <jira-key>       Process a Jira ticket by key")
	fmt.Println("  zen-brain vertical-slice --mock           Use mock work item (no Jira)")
	fmt.Println("  zen-brain vertical-slice --resume <id>    Resume an existing session (requires persistent store)")
	fmt.Println()
	fmt.Println("For intelligence command:")
	fmt.Println("  zen-brain intelligence mine                      Mine proof-of-work artifacts")
	fmt.Println("  zen-brain intelligence analyze                   Print pattern analysis")
	fmt.Println("  zen-brain intelligence recommend <workType> <workDomain>  Get template and config recommendations")
	fmt.Println("  zen-brain intelligence diagnose <workType> <workDomain>    Print failure statistics for work type/domain")
	fmt.Println("  zen-brain intelligence checkpoint <sessionID>   Print execution checkpoint summary")
	fmt.Println()
	fmt.Println("For office command:")
	fmt.Println("  zen-brain office doctor              Print config, connectors, Jira URL, API reachability")
	fmt.Println("  zen-brain office search <query>      Search work items (JQL or plain text)")
	fmt.Println("  zen-brain office fetch <jira-key>    Fetch one item by Jira key")
	fmt.Println("  zen-brain office watch               Start webhook listener and stream events")
	fmt.Println()
	fmt.Println("For self-improvement command:")
	fmt.Println("  zen-brain self-improvement           Run safe self-improvement loop (one iteration)")
	fmt.Println("                                         Processes: Class A (read/recommend), Class B (safe write-back)")
	fmt.Println()
	fmt.Println("For compliance command:")
	fmt.Println("  zen-brain compliance reporter         Generate SR&ED/IRAP/ISO/SOC evidence reports")
	fmt.Println("  zen-brain compliance gap-hunter     Detect compliance gaps and remediation items")
}

func printVersion() {
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Built: %s\n", BuildTime)
}

func runTestQuery() {
	fmt.Println("Initializing LLM Gateway...")

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

	gateway, err := llmgateway.NewGateway(gatewayConfig)
	if err != nil {
		log.Fatalf("Error creating gateway: %v", err)
	}

	fmt.Println("✓ LLM Gateway initialized")
	fmt.Printf("  - Local worker: %s\n", gatewayConfig.LocalWorkerModel)
	fmt.Printf("  - Planner: %s\n", gatewayConfig.PlannerModel)
	fmt.Printf("  - Fallback chain: %v\n", gatewayConfig.EnableFallbackChain)

	ctx := context.Background()
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are zen-brain, an AI assistant for software engineering tasks."},
			{Role: "user", Content: "Hello! What can you help with?"},
		},
		SessionID: "test-session-mvp",
	}

	resp, err := gateway.Chat(ctx, req)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("\n✓ Test query successful\n")
	fmt.Printf("  Response: %s\n", resp.Content[:min(200, len(resp.Content))])
	fmt.Printf("  Tokens: %d\n", resp.Usage.TotalTokens)
	fmt.Printf("  Latency: %dms\n", resp.LatencyMs)
}

func runVerticalSlice() {
	fmt.Println("=== Zen-Brain Vertical Slice ===")
	fmt.Println()
	fmt.Println("This command demonstrates end-to-end pipeline using Planner + Factory:")
	fmt.Println("  1. Fetch work item from Jira (or use mock)")
	fmt.Println("  2. Analyze intent and complexity")
	fmt.Println("  3. Plan execution steps")
	fmt.Println("  4. Create session")
	fmt.Println("  5. Execute in isolated workspace (Factory)")
	fmt.Println("  6. Generate proof-of-work artifacts")
	fmt.Println("  7. Update session state")
	fmt.Println("  8. Update Jira with status and comments")
	fmt.Println()

	// Parse arguments
	useMock := false
	jiraKey := ""
	resumeSessionID := ""
	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "--mock":
			useMock = true
			fmt.Println("Mode: Using mock work item (no Jira required)")
		case "--resume":
			if len(os.Args) < 4 {
				log.Fatal("vertical-slice --resume requires a session ID (e.g. zen-brain vertical-slice --resume session-123-0)")
			}
			resumeSessionID = os.Args[3]
			fmt.Printf("Mode: Resuming session %s\n", resumeSessionID)
		default:
			jiraKey = os.Args[2]
			fmt.Printf("Mode: Fetching real Jira ticket: %s\n", jiraKey)
		}
	} else {
		useMock = true
		fmt.Println("Mode: Using mock work item (no Jira required)")
	}

	fmt.Println()
	fmt.Println("Initializing components...")

	// Step 1: Initialize LLM Gateway
	fmt.Println("[1/7] Initializing LLM Gateway...")
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
		log.Fatalf("Error creating LLM Gateway: %v", err)
	}
	fmt.Println("  ✓ LLM Gateway initialized")

	// Step 2: Initialize Office Manager (config-first, then env fallback)
	fmt.Println("[2/7] Initializing Office Manager...")
	var officeManager *office.Manager
	var jiraMode string // "config", "env", or "mock"
	cfg, cfgErr := config.LoadConfig("")
	if cfgErr == nil && cfg != nil && cfg.Jira.Enabled {
		mgr, err := integration.InitOfficeManagerFromConfig(cfg)
		if err != nil {
			fmt.Printf("  ! Jira from config failed: %v\n", err)
		} else {
			officeManager = mgr
			jiraMode = "config"
		}
	}
	if officeManager == nil {
		officeManager = office.NewManager()
	}
	clusterID := "default"
	if cfg != nil && cfg.ZenContext.ClusterID != "" {
		clusterID = cfg.ZenContext.ClusterID
	} else if v := os.Getenv("CLUSTER_ID"); v != "" {
		clusterID = v
	}
	// FAIL CLOSED: Do not fall back to mock mode silently
	// If --mock was not explicitly set, require real Jira connectivity
	if jiraMode == "" && !useMock {
		jiraConnector, err := jira.NewFromEnv("jira", clusterID)
		if err != nil {
			// FAIL CLOSED: Error instead of falling back to mock
			log.Fatalf("  ✗ Jira connector initialization failed: %v\n  Use --mock flag for testing without Jira", err)
		}
		if err := officeManager.Register("jira", jiraConnector); err != nil {
			log.Fatalf("  ✗ Register Jira failed: %v\n  Use --mock flag for testing without Jira", err)
		}
		if err := officeManager.RegisterForCluster(clusterID, "jira"); err != nil {
			log.Fatalf("  ✗ Register Jira for cluster failed: %v\n  Use --mock flag for testing without Jira", err)
		}
		jiraMode = "env"
	}
	if jiraMode == "config" {
		fmt.Println("  ✓ Jira enabled from config")
	} else if jiraMode == "env" {
		fmt.Println("  ✓ Jira enabled from env")
	} else if useMock {
		fmt.Println("  ✓ Jira mock mode (explicit --mock flag)")
	} else {
		// No Jira configured and not in mock mode - this is an error state
		log.Fatalf("  ✗ No Jira configuration found\n  Use --mock flag for testing without Jira")
	}
	fmt.Println("  ✓ Office Manager initialized")

	// Step 3: Initialize Session Manager
	fmt.Println("[3/7] Initializing Session Manager...")
	sessionConfig := session.DefaultConfig()
	var sessionStore session.Store
	if storeType := os.Getenv("ZEN_BRAIN_SESSION_STORE"); storeType == "sqlite" || resumeSessionID != "" {
		sessionConfig.StoreType = "sqlite"
		if d := os.Getenv("ZEN_BRAIN_DATA_DIR"); d != "" {
			sessionConfig.DataDir = d
		}
		if sessionConfig.DataDir == "" {
			sessionConfig.DataDir = filepath.Join(config.HomeDir(), "sessions")
		}
		if err := os.MkdirAll(sessionConfig.DataDir, 0755); err != nil {
			if resumeSessionID != "" {
				log.Fatalf("Failed to create session data dir: %v", err)
			}
			sessionStore = session.NewMemoryStore()
		} else {
			s, errStore := session.NewSQLiteStore(filepath.Join(sessionConfig.DataDir, "sessions.db"))
			if errStore != nil {
				if resumeSessionID != "" {
					log.Fatalf("Resume requires persistent store; SQLite failed: %v", errStore)
				}
				sessionStore = session.NewMemoryStore()
				log.Printf("Warning: SQLite store failed (%v), using memory", errStore)
			} else {
				sessionStore = s
				fmt.Printf("  ✓ Session store: sqlite (%s)\n", sessionConfig.DataDir)
			}
		}
	}
	if sessionStore == nil {
		sessionConfig.StoreType = "memory"
		sessionStore = session.NewMemoryStore()
	}

	// Block 3: canonical bootstrap from config (or fallback)
	fmt.Println("  - Block 3 runtime bootstrap...")
	var zenContext zenctx.ZenContext
	var ledgerClient ledger.ZenLedgerClient
	var msgBus messagebus.MessageBus
	var rt *runtime.Runtime
	if cfgErr == nil && cfg != nil {
		ctxB := context.Background()
		var errB error
		rt, errB = runtime.Bootstrap(ctxB, cfg)
		if errB != nil {
			log.Printf("Warning: runtime.Bootstrap failed: %v", errB)
		}
		if rt != nil {
			zenContext = rt.ZenContext
			ledgerClient = rt.Ledger
			msgBus = rt.MessageBus
			if rt.Report != nil {
				fmt.Println("  " + runtimeCapabilityBanner(rt.Report))
			}
		}
	}
	// FAIL CLOSED: ZenContext is optional, but we don't silently fall back to mock
	// If ZenContext is required, set ZEN_BRAIN_REQUIRE_ZENCONTEXT=1 or cfg.ZenContext.Required=true
	if zenContext == nil {
		zenContext, err = createRealZenContext()
		if err != nil {
			// ZenContext not available - this is OK for operations that don't need it
			// Block 3 report will show ZenContext=disabled
			log.Printf("  ZenContext not available: %v (continuing without context tiering)", err)
			zenContext = nil // Explicit nil, not mock
		} else if zenContext != nil {
			fmt.Println("  ✓ ZenContext initialized (Redis + MinIO)")
		}
	}
	if ledgerClient == nil {
		ledgerClient = ledgerClientOrNil()
		// Ledger is optional for vertical-slice; Block 5 intelligence will degrade gracefully
	}
	if msgBus == nil && os.Getenv("ZEN_BRAIN_MESSAGE_BUS") == "redis" {
		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			// FAIL CLOSED: Message bus requires explicit configuration
			// Note: runVerticalSlice() does not return errors, so we use log.Fatalf for fatal config errors
			log.Fatalf("ZEN_BRAIN_MESSAGE_BUS=redis but REDIS_URL not set (cannot use default localhost:6379)")
		}
		if bus, errBus := redis.New(&redis.Config{RedisURL: redisURL}); errBus == nil {
			msgBus = bus
			fmt.Println("  ✓ Message bus (Redis) enabled")
		}
	}
	sessionConfig.ZenContext = zenContext
	if cfg != nil && cfg.ZenContext.ClusterID != "" {
		sessionConfig.ClusterID = cfg.ZenContext.ClusterID
	}
	// Block 3: wire message bus for session lifecycle events (journal can be added when available)
	if msgBus != nil {
		sessionConfig.EventBus = msgBus
		sessionConfig.EventStream = "zen-brain.events"
	}

	sessionManager, err := session.New(sessionConfig, sessionStore)
	if err != nil {
		log.Fatalf("Error creating Session Manager: %v", err)
	}
	defer sessionManager.Close()
	fmt.Println("  ✓ Session Manager initialized")

	// Step 4: Initialize Analyzer
	fmt.Println("[4/7] Initializing Analyzer...")
	analyzerConfig := analyzer.DefaultConfig()
	analyzerConfig.LLMProviderName = "glm-4.7"
	analyzerConfig.RequireApproval = false // Auto-approve for vertical slice

	// Task 5: prompt manager for tunable analysis prompts
	promptManager := llmgateway.InitializeDefaultManager()

	// Create simple Analyzer wrapper around LLM Gateway
	intentAnalyzer := &simpleAnalyzer{
		llmGateway:    llmGateway,
		config:        analyzerConfig,
		promptManager: promptManager,
	}
	// Block 2 enterprise: durable, auditable analysis history when Analysis path is set
	if paths := config.DefaultPaths(); paths.Analysis != "" {
		if store, errStore := analyzer.NewFileAnalysisStore(paths.Analysis); errStore == nil {
			intentAnalyzer.historyStore = store
			_ = paths.EnsureAll()
			fmt.Printf("  ✓ Analysis history store: %s\n", paths.Analysis)
		}
	}
	fmt.Println("  ✓ Analyzer initialized")

	// Step 5: Initialize Factory and Block 5 intelligence
	fmt.Println("[5/7] Initializing Factory...")
	// Use ZEN_BRAIN_RUNTIME_DIR if set, otherwise use ZEN_BRAIN_HOME/runtime
	runtimeDir := os.Getenv("ZEN_BRAIN_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(config.HomeDir(), "runtime")
	}
	patternStorePath := filepath.Join(runtimeDir, "patterns")
	patternStore, errPattern := intelligence.NewJSONPatternStore(patternStorePath)
	var miningIntegration *intelligence.MiningIntegration
	if errPattern == nil {
		miningIntegration = intelligence.NewMiningIntegration(runtimeDir, patternStore, nil)
		fmt.Printf("  ✓ Intelligence recommender enabled (pattern store: %s)\n", patternStorePath)
	} else {
		log.Printf("  Warning: pattern store not available (%v); intelligence disabled", errPattern)
	}
	workspaceManager := factory.NewWorkspaceManager(runtimeDir)
	executor := factory.NewBoundedExecutor()
	powManager := factory.NewProofOfWorkManager(runtimeDir)
	factoryImpl := factory.NewFactory(workspaceManager, executor, powManager, runtimeDir)
	if miningIntegration != nil {
		factoryImpl.SetRecommender(miningIntegration.GetFactoryRecommender())
	}
	fmt.Println("  ✓ Factory initialized")

	// Step 6: Initialize Planner (ledgerClient from bootstrap or stub)
	fmt.Println("[6/7] Initializing Planner...")
	if closer, ok := ledgerClient.(interface{ Close() error }); ok && closer != nil {
		defer func() { _ = closer.Close() }()
	}
	if rt != nil {
		defer func() { _ = rt.Close() }()
	}
	plannerConfig := planner.DefaultConfig()
	plannerConfig.OfficeManager = officeManager
	plannerConfig.Analyzer = intentAnalyzer
	plannerConfig.SessionManager = sessionManager
	plannerConfig.LedgerClient = ledgerClient
	plannerConfig.ZenContext = zenContext
	plannerConfig.RequireApproval = false // Auto-approve for vertical slice
	plannerConfig.AutoApproveCost = 100.0 // Approve everything
	// Block 5: cost-aware model routing
	modelRouter := intelligence.NewModelRouter(ledgerClient, plannerConfig.DefaultModel)
	plannerConfig.ModelRecommender = planner.NewModelRouterRecommender(modelRouter)
	// Block 5: hypothesis evidence recording
	plannerConfig.EvidenceVault = evidence.NewMemoryVault()

	plannerAgent, err := planner.New(plannerConfig)
	if err != nil {
		log.Fatalf("Error creating Planner: %v", err)
	}
	defer plannerAgent.Close()
	fmt.Println("  ✓ Planner initialized")

	// Block 5: Wire ZenLedger token recorder to gateway for usage tracking
	if recorder, ok := ledgerClient.(ledger.TokenRecorder); ok {
		llmGateway.SetTokenRecorder(recorder)
		fmt.Println("  ✓ LLM Gateway token recording enabled (ZenLedger)")
	}

	// Message bus set from bootstrap or env above; ensure close on exit
	if msgBus != nil {
		defer func() { _ = msgBus.Close() }()
	}

	// Step 7: Fetch and process work item
	fmt.Println("[7/8] Fetching and processing work item...")
	ctx := context.Background()

	// Watchdog: global timeout for vertical slice (default 15 min)
	sliceTimeout := 15 * 60
	if s := os.Getenv("ZEN_BRAIN_VERTICAL_SLICE_TIMEOUT_SECONDS"); s != "" {
		if n, err := fmt.Sscanf(s, "%d", &sliceTimeout); n == 1 && err == nil && sliceTimeout > 0 {
			// use parsed value
		} else {
			sliceTimeout = 15 * 60
		}
	}
	ctx, cancelSlice := context.WithTimeout(ctx, time.Duration(sliceTimeout)*time.Second)
	defer cancelSlice()

	var workItem *contracts.WorkItem
	var workSession *contracts.Session
	var analysisResult *contracts.AnalysisResult
	var resumeCheckpoint *session.ExecutionCheckpoint
	var proofPathsFromCheckpoint []string

	if resumeSessionID != "" {
		// Resume existing session (persistent store required)
		sess, err := sessionManager.GetSession(ctx, resumeSessionID)
		if err != nil {
			log.Fatalf("Resume failed: session not found: %v", err)
		}
		if sess == nil {
			log.Fatalf("Resume failed: session %s not found", resumeSessionID)
		}
		switch sess.State {
		case contracts.SessionStateCompleted, contracts.SessionStateFailed, contracts.SessionStateCanceled:
			log.Fatalf("Resume failed: session %s is terminal (state=%s)", resumeSessionID, sess.State)
		}
		workSession = sess
		if sess.WorkItem != nil {
			workItem = sess.WorkItem
		} else {
			workItem = &contracts.WorkItem{
				ID:       sess.WorkItemID,
				Title:    "Resumed work item",
				Priority: contracts.PriorityMedium,
				Source:   contracts.SourceMetadata{IssueKey: sess.SourceKey},
			}
		}
		analysisResult = sess.AnalysisResult
		if analysisResult == nil {
			// Re-analyze and store on session
			analysisResult, err = intentAnalyzer.Analyze(ctx, workItem)
			if err != nil {
				log.Fatalf("Error analyzing work item on resume: %v", err)
			}
			currentSession, _ := sessionManager.GetSession(ctx, workSession.ID)
			if currentSession != nil {
				currentSession.BrainTaskSpecs = analysisResult.BrainTaskSpecs
				currentSession.AnalysisResult = analysisResult
				_ = sessionManager.UpdateSession(ctx, currentSession)
			}
		}
		fmt.Printf("✓ Resumed session: %s\n", workSession.ID)
		fmt.Printf("✓ Work item: %s - %s\n", workItem.ID, workItem.Title)
		if analysisResult != nil {
			fmt.Printf("  Analysis: cost $%.2f, confidence %.1f%%\n", analysisResult.EstimatedTotalCostUSD, analysisResult.Confidence*100)
		}
		// Load execution checkpoint for ReMe/resume continuity
		cp, err := sessionManager.GetExecutionCheckpoint(ctx, workSession.ID)
		if err == nil && cp != nil {
			resumeCheckpoint = cp
			proofPathsFromCheckpoint = append([]string(nil), cp.ProofPaths...)
			fmt.Println("Execution checkpoint loaded:")
			fmt.Printf("  stage: %s, tasks: %d, proofs: %d\n", cp.Stage, len(cp.BrainTaskIDs), len(cp.ProofPaths))
			if cp.SelectedModel != "" {
				fmt.Printf("  selected model: %s\n", cp.SelectedModel)
			}
			if cp.LastRecommendation != "" {
				fmt.Printf("  last recommendation: %s\n", cp.LastRecommendation)
			}
		}
	} else {
		if useMock {
			workItem = createMockWorkItem()
		} else {
			fmt.Printf("  Fetching Jira ticket: %s\n", jiraKey)
			fetchedItem, err := officeManager.Fetch(ctx, clusterID, jiraKey)
			if err != nil {
				log.Fatalf("Error fetching work item: %v", err)
			}
			workItem = fetchedItem
		}

		fmt.Printf("✓ Work item: %s - %s\n", workItem.ID, workItem.Title)
		fmt.Printf("  Type: %s, Priority: %s\n", workItem.WorkType, workItem.Priority)
		fmt.Println()

		// Create session
		var err error
		workSession, err = sessionManager.CreateSession(ctx, workItem)
		if err != nil {
			log.Fatalf("Error creating session: %v", err)
		}
		fmt.Printf("✓ Session created: %s\n", workSession.ID)
		// session.created is emitted by session manager when EventBus is configured

		// Analyze work item
		analysisResult, err = intentAnalyzer.Analyze(ctx, workItem)
		if err != nil {
			log.Fatalf("Error analyzing work item: %v", err)
		}
		fmt.Printf("✓ Analysis complete")
		fmt.Printf("  Estimated cost: $%.2f\n", analysisResult.EstimatedTotalCostUSD)
		fmt.Printf("  Confidence: %.1f%%\n", analysisResult.Confidence*100)
		publishVerticalSliceEvent(msgBus, "zen-brain.events", "intent.analyzed", workSession.ID, map[string]string{"session_id": workSession.ID, "work_item_id": workItem.ID, "task_count": fmt.Sprintf("%d", len(analysisResult.BrainTaskSpecs))})

		// Update session with analysis
		if err := sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateAnalyzed, "Work item analyzed", "vertical-slice"); err != nil {
			log.Printf("Warning: Failed to transition session to analyzed: %v", err)
		}

		// analyzed → scheduled → in_progress
		if err := sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateScheduled, "Ready for execution", "vertical-slice"); err != nil {
			log.Printf("Warning: Failed to transition session to scheduled: %v", err)
		}
		if err := sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateInProgress, "Execution in progress", "vertical-slice"); err != nil {
			log.Printf("Warning: Failed to transition session to in_progress: %v", err)
		}

		// Update session with BrainTaskSpecs from analysis
		if len(analysisResult.BrainTaskSpecs) > 0 {
			currentSession, err := sessionManager.GetSession(ctx, workSession.ID)
			if err == nil && currentSession != nil {
				currentSession.BrainTaskSpecs = analysisResult.BrainTaskSpecs
				currentSession.AnalysisResult = analysisResult
				_ = sessionManager.UpdateSession(ctx, currentSession)
			}
		}
	}

	// Block 5: get model recommendation for checkpoint and one-line output
	var selectedModelID, modelSource string
	var modelConfidence float64
	projectID := workItem.ProjectID
	if projectID == "" {
		projectID = "default"
	}
	if modelRouter != nil {
		if modelRec, err := modelRouter.RecommendModel(ctx, projectID, string(workItem.WorkType)); err == nil && modelRec != nil {
			selectedModelID = modelRec.ModelID
			modelSource = modelRec.Source
			modelConfidence = modelRec.Confidence
		}
	}
	if selectedModelID == "" {
		selectedModelID = plannerConfig.DefaultModel
		modelSource = "default"
	}

	skipReplay := session.ShouldSkipReplayForResume(resumeCheckpoint)

	fmt.Println()
	// Step 8: Process work item through Planner + Factory
	fmt.Println("[8/8] Processing work item through Planner + Factory...")
	if skipReplay {
		fmt.Println("Resume loaded prior execution checkpoint; skipping blind task replay")
	}
	startTime := time.Now()

	// If we resumed and session was not yet in_progress, transition now
	if resumeSessionID != "" && workSession.State != contracts.SessionStateInProgress {
		if workSession.State == contracts.SessionStateCreated || workSession.State == contracts.SessionStateAnalyzed {
			_ = sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateScheduled, "Ready for execution (resume)", "vertical-slice")
		}
		_ = sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateInProgress, "Execution in progress (resume)", "vertical-slice")
		workSession, _ = sessionManager.GetSession(ctx, workSession.ID)
	}
	if analysisResult == nil {
		log.Fatalf("No analysis result for session %s", workSession.ID)
	}

	// Once work starts, set Jira status to running (if not mock)
	if !useMock {
		if err := officeManager.UpdateStatus(ctx, clusterID, workItem.ID, contracts.StatusRunning); err != nil {
			log.Printf("Warning: Failed to set Jira status to running: %v", err)
		}
	}

	// Execute tasks through Factory (collect for execution checkpoint)
	var brainTaskIDs []string
	var proofPaths []string
	var lastRecommendation string
	var tasksSucceeded, tasksFailed int
	var commentPosted bool
	var lastPowArtifact *factory.ProofOfWorkArtifact

	fmt.Println()
	fmt.Println("Executing tasks through Factory...")
	if skipReplay {
		brainTaskIDs = append(brainTaskIDs, resumeCheckpoint.BrainTaskIDs...)
		proofPaths = append(proofPaths, proofPathsFromCheckpoint...)
		lastRecommendation = resumeCheckpoint.LastRecommendation
		if selectedModelID == "" && resumeCheckpoint.SelectedModel != "" {
			selectedModelID = resumeCheckpoint.SelectedModel
		}
	} else if len(analysisResult.BrainTaskSpecs) > 0 {
		for _, brainTask := range analysisResult.BrainTaskSpecs {
			brainTaskIDs = append(brainTaskIDs, brainTask.ID)
			fmt.Printf("  Executing task: %s\n", brainTask.ID)

			// Convert BrainTaskSpec to FactoryTaskSpec
			factorySpec := convertToFactoryTaskSpec(brainTask, workSession.ID, workItem.ID)

			// Execute task in Factory
			executionResult, err := factoryImpl.ExecuteTask(ctx, factorySpec)
			if err != nil {
				log.Printf("  ! Factory execution failed: %v", err)
				// Continue with next task (don't fail entire session)
				continue
			}

			// Use proof-of-work from Factory (single source); only generate if Factory did not
			var powArtifact *factory.ProofOfWorkArtifact
			if executionResult.ProofOfWorkPath != "" {
				powArtifact, err = powManager.GetProofOfWork(ctx, executionResult.ProofOfWorkPath)
				if err != nil {
					log.Printf("  ! Could not load Factory proof-of-work from %s: %v", executionResult.ProofOfWorkPath, err)
				}
			}
			if powArtifact == nil {
				powArtifact, err = powManager.CreateProofOfWork(ctx, executionResult, factorySpec)
				if err != nil {
					log.Printf("  ! Proof-of-work generation failed: %v", err)
				}
			}
			if powArtifact != nil {
				lastPowArtifact = powArtifact
				fmt.Printf("  ✓ Proof-of-work generated: %s\n", powArtifact.JSONPath)
				for _, artifactPath := range []string{powArtifact.JSONPath, powArtifact.MarkdownPath, powArtifact.LogPath} {
					if artifactPath != "" {
						proofPaths = append(proofPaths, artifactPath)
						evidence := contracts.EvidenceItem{
							ID:        fmt.Sprintf("pow-%s-%s", brainTask.ID, artifactPath[strings.LastIndex(artifactPath, "/")+1:]),
							SessionID: workSession.ID,
							Type:      contracts.EvidenceTypeProofOfWork,
							Content:   artifactPath,
							Metadata: map[string]string{
								"task_id":  brainTask.ID,
								"title":    brainTask.Title,
								"artifact": artifactPath[strings.LastIndex(artifactPath, "/")+1:],
							},
							CollectedAt: time.Now(),
							CollectedBy: "factory",
						}
						if err := sessionManager.AddEvidence(ctx, workSession.ID, evidence); err != nil {
							log.Printf("  ! Failed to add evidence: %v", err)
						}
					}
				}
				// Post proof-of-work comment to Jira (do not fail slice on error)
				if !useMock {
					proofComment, err := powManager.GenerateComment(ctx, powArtifact)
					if err != nil {
						log.Printf("Warning: could not generate proof comment: %v", err)
					} else if err := officeManager.AddComment(ctx, clusterID, workItem.ID, proofComment); err != nil {
						log.Printf("Warning: could not post proof comment to Jira: %v", err)
					} else {
						commentPosted = true
					}
				}
			}

			// Log execution result
			if executionResult.Recommendation != "" {
				lastRecommendation = executionResult.Recommendation
			}
			if executionResult.Success {
				tasksSucceeded++
				fmt.Printf("  ✓ Task completed: %s (%d steps)\n", executionResult.TaskID, executionResult.CompletedSteps)
			} else {
				tasksFailed++
				fmt.Printf("  ! Task failed: %s - %s\n", executionResult.TaskID, executionResult.Error)
			}
		}
	}
	if !skipReplay && len(analysisResult.BrainTaskSpecs) == 0 {
		fmt.Println("  ! No BrainTaskSpecs from analysis, skipping Factory execution")
	}

	// Block 5: mine proof-of-work so intelligence learns from this run (do not fail vertical slice on mining failure)
	if miningIntegration != nil {
		if _, err := miningIntegration.MineProofOfWorks(ctx); err != nil {
			log.Printf("Warning: intelligence mining failed: %v", err)
		}
	}

	// Attach proof artifacts to Jira (do not fail slice on attachment errors)
	var attachmentsUploaded int
	if !useMock && len(proofPaths) > 0 {
		for _, p := range proofPaths {
			content, err := os.ReadFile(p)
			if err != nil {
				log.Printf("Warning: could not read proof artifact %s: %v", p, err)
				continue
			}
			filename := filepath.Base(p)
			contentType := "application/octet-stream"
			switch strings.ToLower(filepath.Ext(p)) {
			case ".json":
				contentType = "application/json"
			case ".md":
				contentType = "text/markdown"
			case ".log", ".txt":
				contentType = "text/plain"
			}
			att := &contracts.Attachment{
				ID:          fmt.Sprintf("pow-%s", filename),
				WorkItemID:  workItem.ID,
				Filename:    filename,
				ContentType: contentType,
				Size:        int64(len(content)),
				CreatedAt:   time.Now(),
			}
			if err := officeManager.AddAttachment(ctx, clusterID, workItem.ID, att, content); err != nil {
				log.Printf("Warning: could not attach %s to Jira: %v", filename, err)
			} else {
				attachmentsUploaded++
			}
		}
	}

	// Block 5: print model selection provenance (one concise line)
	fmt.Printf("Model: %s (source: %s, confidence: %.2f)\n", selectedModelID, modelSource, modelConfidence)

	// ReMe/resume: write structured execution checkpoint into ZenContext SessionContext.State
	checkpointStage := "proof_attached"
	if ctx.Err() != context.DeadlineExceeded && len(analysisResult.BrainTaskSpecs) > 0 && tasksSucceeded > 0 {
		checkpointStage = "execution_complete"
	}
	var knowledgeChunkIDs, knowledgeSourcePaths []string
	if zenContext != nil {
		if sc, err := zenContext.GetSessionContext(ctx, clusterID, workSession.ID); err == nil && sc != nil && len(sc.RelevantKnowledge) > 0 {
			for _, k := range sc.RelevantKnowledge {
				knowledgeChunkIDs = append(knowledgeChunkIDs, k.ID)
				knowledgeSourcePaths = append(knowledgeSourcePaths, k.SourcePath)
			}
		}
	}
	analysisSummaryShort := ""
	if analysisResult != nil && analysisResult.AnalysisNotes != "" {
		analysisSummaryShort = analysisResult.AnalysisNotes
		if len(analysisSummaryShort) > 500 {
			analysisSummaryShort = analysisSummaryShort[:497] + "..."
		}
	}
	checkpoint := &session.ExecutionCheckpoint{
		Stage:                checkpointStage,
		SessionID:            workSession.ID,
		WorkItemID:           workItem.ID,
		BrainTaskIDs:         brainTaskIDs,
		ProofPaths:           proofPaths,
		LastRecommendation:   lastRecommendation,
		SelectedModel:        selectedModelID,
		AnalysisSummary:      analysisSummaryShort,
		KnowledgeChunkIDs:    knowledgeChunkIDs,
		KnowledgeSourcePaths: knowledgeSourcePaths,
		UpdatedAt:            time.Now(),
	}
	if err := sessionManager.UpdateExecutionCheckpoint(ctx, workSession.ID, checkpoint); err != nil {
		log.Printf("Warning: failed to update execution checkpoint: %v", err)
	}

	// Watchdog: on timeout, mark session failed and exit without updating Jira
	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("Watchdog: vertical slice timeout (%ds)", sliceTimeout)
		if err := sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateFailed, "vertical slice timeout", "watchdog"); err != nil {
			log.Printf("Warning: Failed to transition session to failed: %v", err)
		}
		fmt.Println("  ✗ Session failed (timeout)")
		elapsed := time.Since(startTime)
		fmt.Println()
		fmt.Println("=== Vertical Slice Aborted (Timeout) ===")
		fmt.Printf("  Session: %s\n", workSession.ID)
		fmt.Printf("  Duration: %s\n", elapsed)
		return
	}

	// Determine final Jira status and update (if not mock)
	var statusUpdated bool
	if !useMock {
		finalStatus := contracts.StatusCompleted
		if ctx.Err() == context.DeadlineExceeded || (len(analysisResult.BrainTaskSpecs) > 0 && tasksSucceeded == 0 && tasksFailed > 0) {
			finalStatus = contracts.StatusFailed
		}
		fmt.Println()
		fmt.Println("Updating Jira status...")
		if err := officeManager.UpdateStatus(ctx, clusterID, workItem.ID, finalStatus); err != nil {
			log.Printf("Warning: Failed to update Jira status: %v", err)
		} else {
			statusUpdated = true
			fmt.Println("✓ Jira status updated")
		}
	}

	// Step 3: in_progress → completed
	// Fetch current session state before transitioning
	currentSession, err := sessionManager.GetSession(ctx, workSession.ID)
	if err != nil {
		log.Printf("Warning: Failed to fetch current session state: %v", err)
	} else {
		workSession = currentSession
	}

	if err := sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateCompleted, "Work item processed successfully", "vertical-slice"); err != nil {
		log.Printf("Warning: Failed to transition session to completed: %v", err)
	} else {
		fmt.Println("  ✓ Session completed")
		publishVerticalSliceEvent(msgBus, "zen-brain.events", "session.completed", workSession.ID, map[string]string{"session_id": workSession.ID, "work_item_id": workItem.ID})
	}

	elapsed := time.Since(startTime)

	fmt.Println()
	fmt.Println("=== Vertical Slice Complete ===")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Work item: %s\n", workItem.ID)
	fmt.Printf("  Session: %s\n", workSession.ID)
	fmt.Printf("  Duration: %s\n", elapsed)
	fmt.Printf("  Estimated cost: $%.2f\n", analysisResult.EstimatedTotalCostUSD)
	fmt.Printf("  Jira updated: %v\n", !useMock)
	if !useMock {
		fmt.Printf("  Office: comment posted=%v, attachments=%d, status updated=%v\n", commentPosted, attachmentsUploaded, statusUpdated)
	}
	_ = lastPowArtifact // used for comment generation above
}

// simpleAnalyzer is a simple implementation of IntentAnalyzer
type simpleAnalyzer struct {
	llmGateway    *llmgateway.Gateway
	config        *analyzer.Config
	promptManager *llmgateway.PromptManager
	historyStore  analyzer.AnalysisHistoryStore
}

func (a *simpleAnalyzer) Analyze(ctx context.Context, workItem *contracts.WorkItem) (*contracts.AnalysisResult, error) {
	var systemMsg, userMsg string
	if a.promptManager != nil {
		if tpl, err := a.promptManager.GetTemplate("work_item_analysis"); err == nil {
			vars := map[string]string{
				"title":     workItem.Title,
				"summary":   workItem.Summary,
				"work_type": string(workItem.WorkType),
				"priority":  string(workItem.Priority),
			}
			if s, u, err := tpl.Render(vars); err == nil {
				systemMsg, userMsg = s, u
			}
		}
	}
	if systemMsg == "" && userMsg == "" {
		systemMsg = "You are a technical analyst. Provide structured JSON responses."
		userMsg = fmt.Sprintf(`Analyze this work item and provide a structured assessment:

Title: %s
Summary: %s
Type: %s
Priority: %s

Provide:
1. Complexity assessment (low/medium/high)
2. Estimated effort (e.g., "1-2 hours", "half day", "1 day")
3. Recommended approach
4. Key risks
5. Dependencies

Format your response as JSON:
{
  "complexity": "...",
  "estimated_effort": "...",
  "recommended_approach": "...",
  "risks": ["..."],
  "dependencies": ["..."]
}`,
			workItem.Title, workItem.Summary, workItem.WorkType, workItem.Priority)
	}

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: systemMsg},
			{Role: "user", Content: userMsg},
		},
		SessionID: "analysis-" + workItem.ID,
	}

	resp, err := a.llmGateway.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	// Generate BrainTaskSpec for the work item
	brainTaskSpec := contracts.BrainTaskSpec{
		ID:          fmt.Sprintf("task-%s-1", workItem.ID),
		Title:       workItem.Title,
		Description: workItem.Summary,
		WorkItemID:  workItem.ID,
		SourceKey:   workItem.ID,
		WorkType:    workItem.WorkType,
		WorkDomain:  workItem.WorkDomain,
		Priority:    workItem.Priority,
		Objective:   fmt.Sprintf("Complete work item %s: %s", workItem.ID, workItem.Title),
		Constraints: []string{"Use test-driven development", "Follow coding standards"},
	}

	// For MVP, return a simplified analysis with generated BrainTaskSpec
	return &contracts.AnalysisResult{
		WorkItem:              workItem,
		BrainTaskSpecs:        []contracts.BrainTaskSpec{brainTaskSpec},
		Confidence:            0.8,
		AnalysisNotes:         fmt.Sprintf("Complexity: medium, Effort: 2 hours, Approach: %s", resp.Content[:min(200, len(resp.Content))]),
		RequiresApproval:      false,
		RecommendedModel:      "glm-4.7",
		EstimatedTotalCostUSD: 0.05,
	}, nil
}

func (a *simpleAnalyzer) AnalyzeBatch(ctx context.Context, workItems []*contracts.WorkItem) ([]*contracts.AnalysisResult, error) {
	results := make([]*contracts.AnalysisResult, len(workItems))
	for i, workItem := range workItems {
		result, err := a.Analyze(ctx, workItem)
		if err != nil {
			return nil, fmt.Errorf("batch analysis failed for item %s: %w", workItem.ID, err)
		}
		results[i] = result
	}
	return results, nil
}

func (a *simpleAnalyzer) GetAnalysisHistory(ctx context.Context, workItemID string) ([]*contracts.AnalysisResult, error) {
	if a.historyStore != nil {
		return a.historyStore.GetHistory(ctx, workItemID)
	}
	return []*contracts.AnalysisResult{}, nil
}

func (a *simpleAnalyzer) UpdateAnalysis(ctx context.Context, result *contracts.AnalysisResult) error {
	if a.historyStore == nil {
		return nil
	}
	if result == nil || result.WorkItem == nil {
		return fmt.Errorf("result and result.WorkItem are required")
	}
	analyzer.EnrichForAudit(result, result.WorkItem, a.config.AnalyzedBy, a.config.AnalyzerVersion)
	return a.historyStore.Store(ctx, result.WorkItem.ID, result)
}

// mockZenContext is a simple in-memory implementation of zenctx.ZenContext
type mockZenContext struct {
	sessions map[string]*zenctx.SessionContext
}

func newMockZenContext() *mockZenContext {
	return &mockZenContext{
		sessions: make(map[string]*zenctx.SessionContext),
	}
}

func (m *mockZenContext) GetSessionContext(ctx context.Context, clusterID, sessionID string) (*zenctx.SessionContext, error) {
	key := clusterID + ":" + sessionID
	return m.sessions[key], nil
}

func (m *mockZenContext) StoreSessionContext(ctx context.Context, clusterID string, session *zenctx.SessionContext) error {
	key := clusterID + ":" + session.SessionID
	m.sessions[key] = session
	return nil
}

func (m *mockZenContext) DeleteSessionContext(ctx context.Context, clusterID, sessionID string) error {
	key := clusterID + ":" + sessionID
	delete(m.sessions, key)
	return nil
}

func (m *mockZenContext) QueryKnowledge(ctx context.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error) {
	return []zenctx.KnowledgeChunk{}, nil
}

func (m *mockZenContext) StoreKnowledge(ctx context.Context, chunks []zenctx.KnowledgeChunk) error {
	return nil
}

func (m *mockZenContext) ArchiveSession(ctx context.Context, clusterID, sessionID string) error {
	return nil
}

func (m *mockZenContext) ReconstructSession(ctx context.Context, req zenctx.ReMeRequest) (*zenctx.ReMeResponse, error) {
	key := req.ClusterID + ":" + req.SessionID
	if session, exists := m.sessions[key]; exists {
		return &zenctx.ReMeResponse{
			SessionContext:  session,
			JournalEntries:  []interface{}{},
			ReconstructedAt: time.Now(),
		}, nil
	}

	newSession := &zenctx.SessionContext{
		SessionID:         req.SessionID,
		TaskID:            req.TaskID,
		ClusterID:         req.ClusterID,
		ProjectID:         req.ProjectID,
		CreatedAt:         time.Now(),
		LastAccessedAt:    time.Now(),
		State:             nil,
		RelevantKnowledge: nil,
		Scratchpad:        nil,
	}

	m.sessions[key] = newSession

	return &zenctx.ReMeResponse{
		SessionContext:  newSession,
		JournalEntries:  []interface{}{},
		ReconstructedAt: time.Now(),
	}, nil
}

func (m *mockZenContext) Stats(ctx context.Context) (map[zenctx.Tier]interface{}, error) {
	stats := make(map[zenctx.Tier]interface{})
	stats[zenctx.TierHot] = map[string]interface{}{
		"session_count": len(m.sessions),
		"type":          "mock-memory",
	}
	stats[zenctx.TierWarm] = map[string]interface{}{
		"type": "mock-qmd",
	}
	stats[zenctx.TierCold] = map[string]interface{}{
		"type": "mock-s3",
	}
	return stats, nil
}

func (m *mockZenContext) Close() error {
	return nil
}

// mockLedgerClient is a mock implementation of ledger.ZenLedgerClient
type mockLedgerClient struct{}

func (m *mockLedgerClient) GetModelEfficiency(ctx context.Context, projectID string, taskType string) ([]ledger.ModelEfficiency, error) {
	return []ledger.ModelEfficiency{}, nil
}

func (m *mockLedgerClient) GetCostBudgetStatus(ctx context.Context, projectID string) (*ledger.BudgetStatus, error) {
	return &ledger.BudgetStatus{
		ProjectID:      projectID,
		SpentUSD:       0.05,
		BudgetLimitUSD: 100.0,
		RemainingUSD:   99.95,
		PercentUsed:    0.05,
	}, nil
}

func (m *mockLedgerClient) RecordPlannedModelSelection(ctx context.Context, sessionID, taskID, modelID, reason string) error {
	return nil
}

// ledgerClientOrNil returns a ZenLedgerClient if configured, or nil if not.
// FAIL CLOSED: Never silently use a mock. Callers must handle nil ledger.
func ledgerClientOrNil() ledger.ZenLedgerClient {
	dsn := os.Getenv("ZEN_LEDGER_DSN")
	if dsn == "" {
		dsn = os.Getenv("LEDGER_DATABASE_URL")
	}
	if dsn == "" {
		// No ledger configured - check if stub allowed
		strictMode := runtime.IsStrictProfile()
		allowStubLedger := os.Getenv("ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER") == "1"

		if strictMode {
			// In strict mode, we cannot use stub; return nil (fail-closed)
			return nil
		}
		if allowStubLedger {
			// Explicit stub opt-in via environment variable
			log.Printf("Ledger stub enabled via ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1")
			return internalLedger.NewStubLedgerClient()
		}
		// No ledger configured and stub not allowed
		return nil
	}
	cl, err := internalLedger.NewCockroachLedger(dsn)
	if err != nil || cl == nil {
		// Ledger configured but failed - log and return nil (fail-closed)
		log.Printf("Warning: Ledger configured but init failed: %v", err)
		return nil
	}
	return cl
}

// createRealZenContext creates a production ZenContext with Redis + MinIO
func createRealZenContext() (zenctx.ZenContext, error) {
	// Get home directory with real-path discipline
	homeDir := filepath.Join(os.Getenv("HOME"), ".zen", "zen-brain1")

	// Read Redis config from environment (FAIL CLOSED: no default)
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		return nil, fmt.Errorf("REDIS_URL not set (cannot use default localhost:6379)")
	}
	redisConfig := &tier1.RedisConfig{
		Addr:         redisAddr,
		Password:     os.Getenv("REDIS_PASSWORD"),
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	// Read S3 config from environment (FAIL CLOSED: no default)
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		return nil, fmt.Errorf("S3_ENDPOINT not set (cannot use default http://localhost:9000)")
	}
	s3AccessKey := os.Getenv("S3_ACCESS_KEY_ID")
	if s3AccessKey == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY_ID not set (cannot use default minioadmin)")
	}
	s3SecretKey := os.Getenv("S3_SECRET_ACCESS_KEY")
	if s3SecretKey == "" {
		return nil, fmt.Errorf("S3_SECRET_ACCESS_KEY not set (cannot use default minioadmin)")
	}
	s3Config := &tier3.S3Config{
		Bucket:            os.Getenv("S3_BUCKET"),
		Region:            os.Getenv("S3_REGION"),
		Endpoint:          s3Endpoint,
		AccessKeyID:       s3AccessKey,
		SecretAccessKey:   s3SecretKey,
		SessionToken:      os.Getenv("S3_SESSION_TOKEN"),
		UsePathStyle:      true,
		DisableSSL:        true,
		ForceRenameBucket: false,
		MaxRetries:        3,
		Timeout:           30 * time.Second,
		PartSize:          5 * 1024 * 1024, // 5 MB
		Concurrency:       5,
		Verbose:           true,
	}

	zenCtxConfig := &internalcontext.ZenContextConfig{
		Tier1Redis: redisConfig,
		Tier2QMD: &internalcontext.QMDConfig{
			RepoPath:      filepath.Join(homeDir, "zen-docs"),
			QMDBinaryPath: "",
			Verbose:       false,
		},
		Tier3S3: s3Config,
		Journal: &internalcontext.JournalConfig{
			JournalPath:      filepath.Join(homeDir, "journal"),
			EnableQueryIndex: true,
		},
		ClusterID: "default",
		Verbose:   true,
	}

	return internalcontext.NewZenContext(zenCtxConfig)
}

func createMockWorkItem() *contracts.WorkItem {
	now := time.Now()
	return &contracts.WorkItem{
		ID:            "MOCK-001",
		Title:         "Fix authentication bug in login flow",
		Summary:       "Users are unable to login when using special characters in passwords",
		Body:          "## Problem\n\nSeveral users have reported login failures when their passwords contain special characters (!@#$%). The error message is 'Invalid credentials' even though the password is correct.\n\n## Reproduction\n\n1. Navigate to login page\n2. Enter username\n3. Enter password with special characters\n4. Click login\n5. Error occurs\n\n## Expected Behavior\n\nUsers should be able to login with any valid password, including those with special characters.",
		WorkType:      contracts.WorkTypeDebug,
		WorkDomain:    contracts.DomainCore,
		Priority:      contracts.PriorityHigh,
		ExecutionMode: contracts.ModeApprovalRequired,
		Status:        contracts.StatusRequested,
		CreatedAt:     now,
		UpdatedAt:     now,
		ClusterID:     "default",
		ProjectID:     "MOCK",
	}
}

// publishVerticalSliceEvent publishes an event to the message bus when enabled (Block 3.1).
func publishVerticalSliceEvent(bus messagebus.MessageBus, stream, eventType, correlation string, payload interface{}) {
	if bus == nil {
		return
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Warning: message bus payload marshal: %v", err)
		return
	}
	ev := &messagebus.Event{
		Type:        eventType,
		Source:      "vertical-slice",
		Correlation: correlation,
		Payload:     payloadBytes,
		Timestamp:   time.Now(),
	}
	if err := bus.Publish(context.Background(), stream, ev); err != nil {
		log.Printf("Warning: message bus publish %s: %v", eventType, err)
	}
}

// convertToFactoryTaskSpec converts a BrainTaskSpec to a FactoryTaskSpec
func convertToFactoryTaskSpec(brainTask contracts.BrainTaskSpec, sessionID, workItemID string) *factory.FactoryTaskSpec {
	now := time.Now()

	return &factory.FactoryTaskSpec{
		ID:             brainTask.ID,
		SessionID:      sessionID,
		WorkItemID:     workItemID,
		Title:          brainTask.Title,
		Objective:      brainTask.Objective,
		Constraints:    brainTask.Constraints,
		WorkType:       brainTask.WorkType,
		WorkDomain:     brainTask.WorkDomain,
		Priority:       brainTask.Priority,
		TimeoutSeconds: 2700, // ZB-024: 45 minutes for qwen3.5:0.8b normal lane (only controlled-failure uses short timeout)
		MaxRetries:     3,    // 3 retries default
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getZenContext returns a ZenContext for use by other commands (e.g. intelligence).
// Uses strict runtime bootstrap when possible, otherwise falls back to real or mock.
func getZenContext() zenctx.ZenContext {
	profile := os.Getenv("ZEN_RUNTIME_PROFILE")
	if profile == "" {
		profile = "dev"
	}

	cfg, err := config.LoadConfig("")
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Block 3: Use StrictRuntime for canonical behavior
	strictRT, err := runtime.NewStrictRuntime(context.Background(), &runtime.StrictRuntimeConfig{
		Profile:        profile,
		Config:         cfg,
		EnableHealthCh: false, // No background checks for utility function
	})

	// FAIL CLOSED: No dev-mode fallback to mock - fail-closed on errors
	if err != nil {
		// FAIL CLOSED: Do not silently fall back to mock in dev mode
		// Caller should handle nil explicitly; use --mock flag for testing
		return nil
	}

	if strictRT != nil && strictRT.Runtime() != nil {
		return strictRT.Runtime().ZenContext
	}

	return nil
}

func runtimeCapabilityBanner(r *runtime.RuntimeReport) string {
	if r == nil {
		return "ZenContext=? Ledger=? MessageBus=?"
	}
	zc := string(r.ZenContext.Mode)
	if zc == "" {
		zc = "disabled"
	}
	if r.Tier1Hot.Healthy {
		zc += " (tier1 ok)"
	}
	if r.Tier2Warm.Mode == runtime.ModeReal && !r.Tier2Warm.Healthy {
		zc += ", tier2 degraded"
	}
	if r.Tier3Cold.Mode == runtime.ModeDisabled {
		zc += ", tier3 disabled"
	}
	ledgerMode := string(r.Ledger.Mode)
	if ledgerMode == "" {
		ledgerMode = "stub"
	}
	mbMode := string(r.MessageBus.Mode)
	if mbMode == "" {
		mbMode = "disabled"
	}
	return "ZenContext=" + zc + " Ledger=" + ledgerMode + " MessageBus=" + mbMode
}
