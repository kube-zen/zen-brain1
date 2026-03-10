package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/analyzer"
	internalcontext "github.com/kube-zen/zen-brain1/internal/context"
	"github.com/kube-zen/zen-brain1/internal/context/tier1"
	"github.com/kube-zen/zen-brain1/internal/context/tier3"
	"github.com/kube-zen/zen-brain1/internal/factory"
	llmgateway "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/office/jira"
	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/internal/session"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/ledger"
	"github.com/kube-zen/zen-brain1/pkg/llm"
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

	case "version":
		printVersion()

	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: zen-brain <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  test           Run a simple LLM Gateway test query")
	fmt.Println("  vertical-slice Run end-to-end vertical slice (Jira → analyze → plan → execute → update)")
	fmt.Println("  version        Print version information")
	fmt.Println()
	fmt.Println("For vertical-slice command:")
	fmt.Println("  zen-brain vertical-slice <jira-key>   Process a Jira ticket by key")
	fmt.Println("  zen-brain vertical-slice --mock          Use mock work item instead of real Jira")
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
	if len(os.Args) > 2 {
		if os.Args[2] == "--mock" {
			useMock = true
			fmt.Println("Mode: Using mock work item (no Jira required)")
		} else {
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

	// Step 2: Initialize Office Manager
	fmt.Println("[2/7] Initializing Office Manager...")
	officeManager := office.NewManager()

	// Try to initialize Jira connector if not in mock mode
	if !useMock {
		fmt.Println("  - Attempting to initialize Jira connector...")
		jiraConnector, err := jira.NewFromEnv("jira", "default")
		if err != nil {
			fmt.Printf("  ! Jira connector initialization failed: %v\n", err)
			fmt.Println("  ! Falling back to mock mode")
			useMock = true
		} else {
			if err := officeManager.Register("jira", jiraConnector); err != nil {
				log.Fatalf("Error registering Jira connector: %v", err)
			}
			if err := officeManager.RegisterForCluster("default", "jira"); err != nil {
				log.Fatalf("Error registering Jira for cluster: %v", err)
			}
			fmt.Println("  ✓ Jira connector registered")
		}
	}
	fmt.Println("  ✓ Office Manager initialized")

	// Step 3: Initialize Session Manager
	fmt.Println("[3/7] Initializing Session Manager...")
	sessionConfig := session.DefaultConfig()
	sessionConfig.StoreType = "memory"
	sessionStore := session.NewMemoryStore()

	// Create and wire real ZenContext (Redis + MinIO)
	fmt.Println("  - Initializing ZenContext (tiered memory)...")
	zenContext, err := createRealZenContext()
	if err != nil {
		log.Printf("Warning: failed to create real ZenContext: %v", err)
		log.Printf("Falling back to mock ZenContext")
		zenContext = newMockZenContext()
	} else {
		fmt.Println("  ✓ ZenContext initialized (Redis + MinIO)")
	}
	sessionConfig.ZenContext = zenContext

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

	// Create simple Analyzer wrapper around LLM Gateway
	intentAnalyzer := &simpleAnalyzer{
		llmGateway: llmGateway,
		config:     analyzerConfig,
	}
	fmt.Println("  ✓ Analyzer initialized")

	// Step 5: Initialize Factory
	fmt.Println("[5/7] Initializing Factory...")
	runtimeDir := "/tmp/zen-brain-factory"
	workspaceManager := factory.NewWorkspaceManager(runtimeDir)
	executor := factory.NewBoundedExecutor()
	powManager := factory.NewProofOfWorkManager(runtimeDir)
	factoryImpl := factory.NewFactory(workspaceManager, executor, powManager, runtimeDir)
	fmt.Println("  ✓ Factory initialized")

	// Step 6: Initialize Planner
	fmt.Println("[6/7] Initializing Planner...")
	plannerConfig := planner.DefaultConfig()
	plannerConfig.OfficeManager = officeManager
	plannerConfig.Analyzer = intentAnalyzer
	plannerConfig.SessionManager = sessionManager
	plannerConfig.LedgerClient = &mockLedgerClient{}
	plannerConfig.ZenContext = zenContext
	plannerConfig.RequireApproval = false // Auto-approve for vertical slice
	plannerConfig.AutoApproveCost = 100.0 // Approve everything

	plannerAgent, err := planner.New(plannerConfig)
	if err != nil {
		log.Fatalf("Error creating Planner: %v", err)
	}
	defer plannerAgent.Close()
	fmt.Println("  ✓ Planner initialized")

	// Step 7: Fetch and process work item
	fmt.Println("[7/8] Fetching and processing work item...")
	ctx := context.Background()

	var workItem *contracts.WorkItem

	if useMock {
		workItem = createMockWorkItem()
	} else {
		fmt.Printf("  Fetching Jira ticket: %s\n", jiraKey)
		fetchedItem, err := officeManager.Fetch(ctx, "default", jiraKey)
		if err != nil {
			log.Fatalf("Error fetching work item: %v", err)
		}
		workItem = fetchedItem
	}

	fmt.Printf("✓ Work item: %s - %s\n", workItem.ID, workItem.Title)
	fmt.Printf("  Type: %s, Priority: %s\n", workItem.WorkType, workItem.Priority)
	fmt.Println()

	// Step 8: Process work item through Planner + Factory
	fmt.Println("[8/8] Processing work item through Planner + Factory...")

	// Use synchronous processing for vertical slice (not async)
	startTime := time.Now()

	// Create session
	workSession, err := sessionManager.CreateSession(ctx, workItem)
	if err != nil {
		log.Fatalf("Error creating session: %v", err)
	}
	fmt.Printf("✓ Session created: %s\n", workSession.ID)

	// Analyze work item
	analysisResult, err := intentAnalyzer.Analyze(ctx, workItem)
	if err != nil {
		log.Fatalf("Error analyzing work item: %v", err)
	}
	fmt.Printf("✓ Analysis complete")
	fmt.Printf("  Estimated cost: $%.2f\n", analysisResult.EstimatedTotalCostUSD)
	fmt.Printf("  Confidence: %.1f%%\n", analysisResult.Confidence*100)

	// Update session with analysis
	if err := sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateAnalyzed, "Work item analyzed", "vertical-slice"); err != nil {
		log.Printf("Warning: Failed to transition session to analyzed: %v", err)
	}

	// Step 1: analyzed → scheduled
	if err := sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateScheduled, "Ready for execution", "vertical-slice"); err != nil {
		log.Printf("Warning: Failed to transition session to scheduled: %v", err)
	}

	// Step 2: scheduled → in_progress
	if err := sessionManager.TransitionState(ctx, workSession.ID, contracts.SessionStateInProgress, "Execution in progress", "vertical-slice"); err != nil {
		log.Printf("Warning: Failed to transition session to in_progress: %v", err)
	}

	// Update session with BrainTaskSpecs from analysis (after state transitions)
	if len(analysisResult.BrainTaskSpecs) > 0 {
		// Fetch the current session (after state transitions)
		currentSession, err := sessionManager.GetSession(ctx, workSession.ID)
		if err != nil {
			log.Printf("Warning: Failed to fetch session for BrainTaskSpecs update: %v", err)
		} else {
			currentSession.BrainTaskSpecs = analysisResult.BrainTaskSpecs
			currentSession.AnalysisResult = analysisResult
			if err := sessionManager.UpdateSession(ctx, currentSession); err != nil {
				log.Printf("Warning: Failed to update session with BrainTaskSpecs: %v", err)
			}
		}
	}

	// Execute tasks through Factory
	fmt.Println()
	fmt.Println("Executing tasks through Factory...")
	if len(analysisResult.BrainTaskSpecs) > 0 {
		for _, brainTask := range analysisResult.BrainTaskSpecs {
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
				fmt.Printf("  ✓ Proof-of-work generated: %s\n", powArtifact.JSONPath)
				for _, artifactPath := range []string{powArtifact.JSONPath, powArtifact.MarkdownPath, powArtifact.LogPath} {
					if artifactPath != "" {
						evidence := contracts.EvidenceItem{
							ID:        fmt.Sprintf("pow-%s-%s", brainTask.ID, artifactPath[strings.LastIndex(artifactPath, "/")+1:]),
							SessionID: workSession.ID,
							Type:      "proof_of_work",
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
			}

			// Log execution result
			if executionResult.Success {
				fmt.Printf("  ✓ Task completed: %s (%d steps)\n", executionResult.TaskID, executionResult.CompletedSteps)
			} else {
				fmt.Printf("  ! Task failed: %s - %s\n", executionResult.TaskID, executionResult.Error)
			}
		}
	} else {
		fmt.Println("  ! No BrainTaskSpecs from analysis, skipping Factory execution")
	}

	// Update Jira if not in mock mode
	if !useMock {
		fmt.Println()
		fmt.Println("Updating Jira status to completed...")
		if err := officeManager.UpdateStatus(ctx, "default", workItem.ID, contracts.StatusCompleted); err != nil {
			log.Printf("Warning: Failed to update Jira status: %v", err)
		} else {
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
}

// simpleAnalyzer is a simple implementation of IntentAnalyzer
type simpleAnalyzer struct {
	llmGateway *llmgateway.Gateway
	config     *analyzer.Config
}

func (a *simpleAnalyzer) Analyze(ctx context.Context, workItem *contracts.WorkItem) (*contracts.AnalysisResult, error) {
	prompt := fmt.Sprintf(`Analyze this work item and provide a structured assessment:

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

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a technical analyst. Provide structured JSON responses."},
			{Role: "user", Content: prompt},
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
	return []*contracts.AnalysisResult{}, nil
}

func (a *simpleAnalyzer) UpdateAnalysis(ctx context.Context, result *contracts.AnalysisResult) error {
	return nil
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

// createRealZenContext creates a production ZenContext with Redis + MinIO
func createRealZenContext() (zenctx.ZenContext, error) {
	// Use local Docker containers (from docker-compose.zencontext.yml)
	redisConfig := &tier1.RedisConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	s3Config := &tier3.S3Config{
		Bucket:            "zen-brain-context",
		Region:            "us-east-1",
		Endpoint:          "http://localhost:9000",
		AccessKeyID:       "minioadmin",
		SecretAccessKey:   "minioadmin",
		SessionToken:      "",
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
			RepoPath:      "./zen-docs",
			QMDBinaryPath: "",
			Verbose:       false,
		},
		Tier3S3:   s3Config,
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
		TimeoutSeconds: 300, // 5 minutes default timeout
		MaxRetries:     3,   // 3 retries default
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
