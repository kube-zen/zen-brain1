// Package planner provides the Planner Agent for zen-brain.
package planner

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/internal/agent"
	"github.com/kube-zen/zen-brain1/internal/analyzer"
	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/session"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

// DefaultPlanner is the default implementation of Planner.
type DefaultPlanner struct {
	config  *Config
	mu      sync.RWMutex
	
	// Component references
	officeManager   *office.Manager
	analyzer        analyzer.IntentAnalyzer
	sessionManager  session.Manager
	ledgerClient    ledger.ZenLedgerClient
	zenctx          zenctx.ZenContext
	factory         factory.Factory
	stateManager    *agent.StateManager
	
	// Internal state
	activeSessions  map[string]*contracts.Session
	approvalQueue   []*contracts.Session
	
	// Shutdown
	shutdownChan    chan struct{}
	shutdownWg      sync.WaitGroup
}

// New creates a new DefaultPlanner.
func New(config *Config) (*DefaultPlanner, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	// Validate required components
	if config.OfficeManager == nil {
		return nil, fmt.Errorf("OfficeManager is required")
	}
	if config.Analyzer == nil {
		return nil, fmt.Errorf("Analyzer is required")
	}
	if config.SessionManager == nil {
		return nil, fmt.Errorf("SessionManager is required")
	}
	if config.LedgerClient == nil {
		return nil, fmt.Errorf("LedgerClient is required")
	}
	
	// Initialize StateManager if ZenContext is available
	var stateManager *agent.StateManager
	if config.ZenContext != nil {
		stateManager = agent.NewStateManager(config.ZenContext, "default")
	}
	
	planner := &DefaultPlanner{
		config:         config,
		officeManager:  config.OfficeManager,
		analyzer:       config.Analyzer,
		sessionManager: config.SessionManager,
		ledgerClient:   config.LedgerClient,
		zenctx:         config.ZenContext,
		factory:        config.Factory,
		stateManager:   stateManager,
		activeSessions: make(map[string]*contracts.Session),
		approvalQueue:  make([]*contracts.Session, 0),
		shutdownChan:   make(chan struct{}),
	}
	
	// Start background goroutines
	planner.startBackgroundTasks()
	
	return planner, nil
}

// ProcessWorkItem processes a new work item from an Office connector.
func (p *DefaultPlanner) ProcessWorkItem(ctx context.Context, workItem *contracts.WorkItem) error {
	log.Printf("Planner processing work item: %s - %s", workItem.ID, workItem.Title)
	
	// Step 1: Create session
	session, err := p.sessionManager.CreateSession(ctx, workItem)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	
	p.mu.Lock()
	p.activeSessions[session.ID] = session
	p.mu.Unlock()
	
	// Step 2: Analyze intent (async to avoid blocking)
	go p.analyzeAndPlan(ctx, session.ID, workItem)
	
	return nil
}

// ProcessBatch processes multiple work items in batch.
func (p *DefaultPlanner) ProcessBatch(ctx context.Context, workItems []*contracts.WorkItem) error {
	for _, workItem := range workItems {
		if err := p.ProcessWorkItem(ctx, workItem); err != nil {
			log.Printf("Failed to process work item %s: %v", workItem.ID, err)
			// Continue with remaining items
		}
	}
	return nil
}

// initializeAgentState creates or loads agent state for a session.
// If no state exists, creates new planner agent state.
// If state exists (e.g., from previous run), loads and updates it.
func (p *DefaultPlanner) initializeAgentState(ctx context.Context, sessionID, taskID string) (*agent.AgentState, error) {
	if p.stateManager == nil {
		// ZenContext not configured, agent state not available
		return nil, nil
	}
	
	// Try to load existing agent state
	agentState, err := p.stateManager.LoadAgentState(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent state: %w", err)
	}
	
	if agentState == nil {
		// No existing state, create new one
		agentState = agent.NewAgentState("planner", agent.RolePlanner, sessionID, taskID)
		agentState.UpdateStep("session_initialization")
		
		// Store initial state
		if err := p.stateManager.StoreAgentState(ctx, agentState); err != nil {
			return nil, fmt.Errorf("failed to store initial agent state: %w", err)
		}
		
		log.Printf("Created new agent state for session %s", sessionID)
	} else {
		// Existing state found - agent is resuming work
		agentState.UpdateHeartbeat()
		agentState.UpdateStep("resuming_work")
		
		// Update state to reflect resumption
		if err := p.stateManager.StoreAgentState(ctx, agentState); err != nil {
			log.Printf("Warning: failed to update agent state on resume: %v", err)
		}
		
		log.Printf("Resumed existing agent state for session %s (steps completed: %d)", 
			sessionID, agentState.StepsCompleted)
	}
	
	return agentState, nil
}

// updateAgentState stores updated agent state with current progress.
func (p *DefaultPlanner) updateAgentState(ctx context.Context, agentState *agent.AgentState) error {
	if p.stateManager == nil || agentState == nil {
		return nil // No-op if ZenContext not configured or state is nil
	}
	
	agentState.UpdateHeartbeat()
	return p.stateManager.StoreAgentState(ctx, agentState)
}

// queryKnowledge queries knowledge base via ZenContext and records in agent state.
func (p *DefaultPlanner) queryKnowledge(ctx context.Context, agentState *agent.AgentState, query string, scopes []string, limit int) ([]zenctx.KnowledgeChunk, error) {
	if p.stateManager == nil || agentState == nil {
		// Fallback: return empty result
		return []zenctx.KnowledgeChunk{}, nil
	}
	
	return p.stateManager.QueryKnowledge(ctx, agentState.SessionID, query, scopes, limit)
}

// analyzeAndPlan performs analysis and planning for a session.
func (p *DefaultPlanner) analyzeAndPlan(ctx context.Context, sessionID string, workItem *contracts.WorkItem) {
	// Initialize agent state for this session
	agentState, err := p.initializeAgentState(ctx, sessionID, workItem.ID)
	if err != nil {
		log.Printf("Warning: failed to initialize agent state for session %s: %v", sessionID, err)
		// Continue without agent state
		agentState = nil
	}
	
	// Update agent state: starting analysis
	if agentState != nil {
		agentState.UpdateStep("intent_analysis")
		agentState.UpdateProgress(0.1)
		if err := p.updateAgentState(ctx, agentState); err != nil {
			log.Printf("Warning: failed to update agent state: %v", err)
		}
	}
	
	// Create timeout context for analysis
	analysisCtx, cancel := context.WithTimeout(ctx, time.Duration(p.config.AnalysisTimeout)*time.Second)
	defer cancel()
	
	// Step 1: Analyze intent
	analysisResult, err := p.analyzer.Analyze(analysisCtx, workItem)
	if err != nil {
		log.Printf("Analysis failed for session %s: %v", sessionID, err)
		
		// Update agent state: analysis failed
		if agentState != nil {
			agentState.AddError(fmt.Sprintf("Intent analysis failed: %v", err), "intent_analysis", false, "")
			agentState.Complete("failed", "Intent analysis failed")
			p.updateAgentState(ctx, agentState) // Best effort
		}
		
		p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateFailed, 
			fmt.Sprintf("Intent analysis failed: %v", err), "planner")
		return
	}
	
	// Step 2: Update session with analysis results
	session, err := p.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		log.Printf("Failed to get session %s: %v", sessionID, err)
		return
	}
	
	session.AnalysisResult = analysisResult
	session.BrainTaskSpecs = analysisResult.BrainTaskSpecs
	
	if err := p.sessionManager.UpdateSession(ctx, session); err != nil {
		log.Printf("Failed to update session %s: %v", sessionID, err)
		return
	}
	
	// Step 3: Transition to analyzed state
	if err := p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateAnalyzed,
		"Intent analysis complete", "analyzer"); err != nil {
		log.Printf("Failed to transition session %s to analyzed: %v", sessionID, err)
		return
	}
	
	// Step 4: Select optimal model
	modelSelection, err := p.selectOptimalModel(ctx, session, analysisResult)
	if err != nil {
		log.Printf("Model selection failed for session %s: %v", sessionID, err)
		// Use default model as fallback
		modelSelection = &ModelSelection{
			ModelID:         p.config.DefaultModel,
			Reason:          "Fallback due to selection error",
			EstimatedCostUSD: analysisResult.EstimatedTotalCostUSD,
			Confidence:      0.5,
		}
	}
	
	// Step 5: Record model selection in ledger
	if p.ledgerClient != nil {
		if err := p.ledgerClient.RecordPlannedModelSelection(ctx, sessionID, 
			session.WorkItemID, modelSelection.ModelID, modelSelection.Reason); err != nil {
			log.Printf("Failed to record model selection: %v", err)
		}
	}
	
	// Step 6: Check if approval is required
	requiresApproval := p.config.RequireApproval && 
		(analysisResult.RequiresApproval || 
		 analysisResult.EstimatedTotalCostUSD > p.config.AutoApproveCost)
	
	if requiresApproval {
		// Transition to blocked for approval
		if err := p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateBlocked,
			"Awaiting human approval", "planner"); err != nil {
			log.Printf("Failed to block session for approval: %v", err)
			return
		}
		
		// Add to approval queue
		p.mu.Lock()
		p.approvalQueue = append(p.approvalQueue, session)
		p.mu.Unlock()
		
		log.Printf("Session %s requires approval (estimated cost: $%.2f)", 
			sessionID, analysisResult.EstimatedTotalCostUSD)
	} else {
		// Auto-approve and schedule
		if err := p.autoApproveAndSchedule(ctx, sessionID, modelSelection); err != nil {
			log.Printf("Failed to auto-approve session %s: %v", sessionID, err)
			return
		}
	}
}

// selectOptimalModel selects the optimal model for a session.
func (p *DefaultPlanner) selectOptimalModel(ctx context.Context, session *contracts.Session, 
	analysis *contracts.AnalysisResult) (*ModelSelection, error) {
	
	// Get efficiency data from ledger
	taskType := string(session.WorkItem.WorkType)
	efficiencies, err := p.ledgerClient.GetModelEfficiency(ctx, "default", taskType)
	if err != nil {
		return nil, fmt.Errorf("failed to get model efficiency: %w", err)
	}
	
	// If no efficiency data, use default model
	if len(efficiencies) == 0 {
		return &ModelSelection{
			ModelID:         p.config.DefaultModel,
			Reason:          "No efficiency data available, using default",
			EstimatedCostUSD: analysis.EstimatedTotalCostUSD,
			Confidence:      0.5,
		}, nil
	}
	
	// Find best model based on success rate and cost
	var bestModel ledger.ModelEfficiency
	var bestScore float64
	
	for _, eff := range efficiencies {
		// Skip models with insufficient sample size
		if eff.SampleSize < 10 {
			continue
		}
		
		// Simple scoring: success rate * (1 / normalized cost)
		// Lower cost is better, higher success rate is better
		costScore := 1.0
		if eff.AvgCostPerTask > 0 {
			// Normalize cost (lower is better)
			costScore = 1.0 / eff.AvgCostPerTask
		}
		
		score := eff.SuccessRate * costScore
		
		if score > bestScore || bestScore == 0 {
			bestScore = score
			bestModel = eff
		}
	}
	
	// If no model met criteria, use default
	if bestScore == 0 {
		return &ModelSelection{
			ModelID:         p.config.DefaultModel,
			Reason:          "No suitable model found in efficiency data",
			EstimatedCostUSD: analysis.EstimatedTotalCostUSD,
			Confidence:      0.5,
		}, nil
	}
	
	// Build alternatives (other models with decent scores)
	var alternatives []string
	for _, eff := range efficiencies {
		if eff.ModelID != bestModel.ModelID && eff.SampleSize >= 5 && eff.SuccessRate >= 0.7 {
			alternatives = append(alternatives, eff.ModelID)
		}
	}
	
	return &ModelSelection{
		ModelID:         bestModel.ModelID,
		Reason:          fmt.Sprintf("Best efficiency: %.1f%% success rate, $%.3f avg cost", 
			bestModel.SuccessRate*100, bestModel.AvgCostPerTask),
		EstimatedCostUSD: analysis.EstimatedTotalCostUSD,
		Confidence:       bestModel.SuccessRate,
		Alternatives:     alternatives,
	}, nil
}

// autoApproveAndSchedule automatically approves and schedules a session.
func (p *DefaultPlanner) autoApproveAndSchedule(ctx context.Context, sessionID string, 
	modelSelection *ModelSelection) error {
	
	// Transition to scheduled
	if err := p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateScheduled,
		fmt.Sprintf("Auto-approved (model: %s, cost: $%.2f)", 
			modelSelection.ModelID, modelSelection.EstimatedCostUSD), "planner"); err != nil {
		return fmt.Errorf("failed to schedule session: %w", err)
	}
	
	// Get session to retrieve BrainTaskSpecs
	session, err := p.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	
	// If Factory is not configured, skip execution (legacy mode)
	if p.factory == nil {
		log.Printf("Factory not configured, skipping execution (legacy mode)")
		if err := p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateInProgress,
			"Scheduled for execution (no factory)", "planner"); err != nil {
			return fmt.Errorf("failed to start execution: %v", err)
		}
		return nil
	}
	
	// Execute each BrainTaskSpec
	for _, brainSpec := range session.BrainTaskSpecs {
		factorySpec := p.convertBrainTaskSpecToFactoryTaskSpec(session, &brainSpec)
		result, err := p.factory.ExecuteTask(ctx, factorySpec)
		if err != nil {
			log.Printf("Factory task execution failed: task_id=%s error=%v", factorySpec.ID, err)
			// Transition session to failed
			p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateFailed,
				fmt.Sprintf("Factory execution failed: %v", err), "factory")
			return fmt.Errorf("factory execution failed: %w", err)
		}
		log.Printf("Factory task completed: task_id=%s status=%s duration=%s", 
			result.TaskID, result.Status, result.Duration)
	}
	
	// All tasks completed successfully
	if err := p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateCompleted,
		"All factory tasks completed successfully", "factory"); err != nil {
		return fmt.Errorf("failed to transition to completed: %v", err)
	}
	
	log.Printf("Session %s auto-approved and executed with model %s", 
		sessionID, modelSelection.ModelID)
	
	return nil
}

// convertBrainTaskSpecToFactoryTaskSpec converts a BrainTaskSpec to a FactoryTaskSpec.
func (p *DefaultPlanner) convertBrainTaskSpecToFactoryTaskSpec(session *contracts.Session, brainSpec *contracts.BrainTaskSpec) *factory.FactoryTaskSpec {
	return &factory.FactoryTaskSpec{
		ID:          brainSpec.ID,
		SessionID:   session.ID,
		WorkItemID:  session.WorkItemID,
		Title:       brainSpec.Title,
		Objective:   brainSpec.Objective,
		Constraints: brainSpec.Constraints,
		WorkType:    brainSpec.WorkType,
		WorkDomain:  brainSpec.WorkDomain,
		Priority:    brainSpec.Priority,
		TimeoutSeconds: brainSpec.TimeoutSeconds,
		MaxRetries:    brainSpec.MaxRetries,
		KBScopes:     brainSpec.KBScopes,
		CreatedAt:    brainSpec.CreatedAt,
		UpdatedAt:    brainSpec.UpdatedAt,
	}
}

// GetSessionStatus returns the current status of a session.
func (p *DefaultPlanner) GetSessionStatus(ctx context.Context, sessionID string) (*SessionStatus, error) {
	session, err := p.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	
	status := &SessionStatus{
		Session:      session,
		WorkItem:     session.WorkItem,
		Analysis:     session.AnalysisResult,
		BrainTaskSpecs: session.BrainTaskSpecs,
		Evidence:     session.EvidenceItems,
	}
	
	// Calculate metrics
	if session.AnalysisResult != nil {
		status.EstimatedCostUSD = session.AnalysisResult.EstimatedTotalCostUSD
	}
	
	// Calculate progress based on state
	switch session.State {
	case contracts.SessionStateCreated:
		status.ProgressPercent = 0
	case contracts.SessionStateAnalyzed:
		status.ProgressPercent = 25
	case contracts.SessionStateScheduled:
		status.ProgressPercent = 50
	case contracts.SessionStateInProgress:
		status.ProgressPercent = 75
	case contracts.SessionStateCompleted:
		status.ProgressPercent = 100
	default:
		status.ProgressPercent = 0
	}
	
	// Calculate time elapsed
	if session.StartedAt != nil {
		status.TimeElapsed = time.Since(*session.StartedAt).Round(time.Second).String()
	}
	
	return status, nil
}

// ApproveSession approves a session that's pending approval.
func (p *DefaultPlanner) ApproveSession(ctx context.Context, sessionID string, approver string, notes string) error {
	// Get session
	session, err := p.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	
	// Check if session is blocked for approval
	if session.State != contracts.SessionStateBlocked {
		return fmt.Errorf("session is not awaiting approval (state: %s)", session.State)
	}
	
	// Transition to scheduled
	if err := p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateScheduled,
		fmt.Sprintf("Approved by %s: %s", approver, notes), approver); err != nil {
		return fmt.Errorf("failed to schedule approved session: %w", err)
	}
	
	// Remove from approval queue
	p.mu.Lock()
	for i, s := range p.approvalQueue {
		if s.ID == sessionID {
			p.approvalQueue = append(p.approvalQueue[:i], p.approvalQueue[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
	
	log.Printf("Session %s approved by %s", sessionID, approver)
	
	// TODO: Schedule with Factory (Block 3)
	// For now, transition to in_progress
	if err := p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateInProgress,
		"Starting execution after approval", "planner"); err != nil {
		return fmt.Errorf("failed to start execution: %v", err)
	}
	
	return nil
}

// RejectSession rejects a session that's pending approval.
func (p *DefaultPlanner) RejectSession(ctx context.Context, sessionID string, rejector string, reason string) error {
	// Get session
	session, err := p.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	
	// Check if session is blocked for approval
	if session.State != contracts.SessionStateBlocked {
		return fmt.Errorf("session is not awaiting approval (state: %s)", session.State)
	}
	
	// Transition to canceled
	if err := p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateCanceled,
		fmt.Sprintf("Rejected by %s: %s", rejector, reason), rejector); err != nil {
		return fmt.Errorf("failed to cancel rejected session: %w", err)
	}
	
	// Remove from approval queue
	p.mu.Lock()
	for i, s := range p.approvalQueue {
		if s.ID == sessionID {
			p.approvalQueue = append(p.approvalQueue[:i], p.approvalQueue[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
	
	log.Printf("Session %s rejected by %s: %s", sessionID, rejector, reason)
	return nil
}

// CancelSession cancels an active session.
func (p *DefaultPlanner) CancelSession(ctx context.Context, sessionID string, canceller string, reason string) error {
	// Check if session is active
	session, err := p.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	
	// Check if session can be canceled
	cancelableStates := map[contracts.SessionState]bool{
		contracts.SessionStateCreated:    true,
		contracts.SessionStateAnalyzed:   true,
		contracts.SessionStateScheduled:  true,
		contracts.SessionStateInProgress: true,
		contracts.SessionStateBlocked:    true,
	}
	
	if !cancelableStates[session.State] {
		return fmt.Errorf("session cannot be canceled in state %s", session.State)
	}
	
	// Transition to canceled
	if err := p.sessionManager.TransitionState(ctx, sessionID, contracts.SessionStateCanceled,
		fmt.Sprintf("Canceled by %s: %s", canceller, reason), canceller); err != nil {
		return fmt.Errorf("failed to cancel session: %w", err)
	}
	
	log.Printf("Session %s canceled by %s: %s", sessionID, canceller, reason)
	return nil
}

// GetPendingApprovals returns sessions waiting for approval.
func (p *DefaultPlanner) GetPendingApprovals(ctx context.Context) ([]*contracts.Session, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Filter only blocked sessions
	var pending []*contracts.Session
	for _, session := range p.approvalQueue {
		if session.State == contracts.SessionStateBlocked {
			pending = append(pending, session)
		}
	}
	
	return pending, nil
}

// Close closes the planner.
func (p *DefaultPlanner) Close() error {
	close(p.shutdownChan)
	p.shutdownWg.Wait()
	return nil
}

// startBackgroundTasks starts background monitoring tasks.
func (p *DefaultPlanner) startBackgroundTasks() {
	// Monitor for stuck sessions
	p.shutdownWg.Add(1)
	go func() {
		defer p.shutdownWg.Done()
		p.monitorStuckSessions()
	}()
}

// monitorStuckSessions periodically checks for stuck sessions.
func (p *DefaultPlanner) monitorStuckSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			p.checkStuckSessions(ctx)
		case <-p.shutdownChan:
			return
		}
	}
}

// checkStuckSessions checks for sessions that have been stuck too long.
func (p *DefaultPlanner) checkStuckSessions(ctx context.Context) {
	// Check sessions that have been in progress for too long
	filter := session.SessionFilter{
		State: &[]contracts.SessionState{contracts.SessionStateInProgress}[0],
	}
	
	sessions, err := p.sessionManager.ListSessions(ctx, filter)
	if err != nil {
		log.Printf("Failed to list in-progress sessions: %v", err)
		return
	}
	
	for _, s := range sessions {
		if s.StartedAt != nil {
			elapsed := time.Since(*s.StartedAt)
			if elapsed > time.Duration(p.config.ExecutionTimeout)*time.Second {
				log.Printf("Session %s stuck in progress for %v, failing", s.ID, elapsed)
				p.sessionManager.TransitionState(ctx, s.ID, contracts.SessionStateFailed,
					fmt.Sprintf("Execution timeout after %v", elapsed), "planner-monitor")
			}
		}
	}
}