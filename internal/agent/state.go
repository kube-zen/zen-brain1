// Package agent provides agent state management and ZenContext integration.
// This package defines the agent state structure and helpers for persisting
// agent state in ZenContext.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// AgentRole identifies the type of agent.
type AgentRole string

const (
	RolePlanner   AgentRole = "planner"
	RoleAnalyzer  AgentRole = "analyzer"
	RoleWorker    AgentRole = "worker"
	RoleApprover  AgentRole = "approver"
	RoleMonitor   AgentRole = "monitor"
)

// AgentState represents the serializable state of an agent.
// This is stored in ZenContext SessionContext.State field.
type AgentState struct {
	// Identity
	AgentID     string    `json:"agent_id"`
	AgentRole   AgentRole `json:"agent_role"`
	SessionID   string    `json:"session_id"`
	TaskID      string    `json:"task_id"`
	
	// Current activity
	CurrentStep string    `json:"current_step,omitempty"`
	StepStarted time.Time `json:"step_started,omitempty"`
	StepProgress float64  `json:"step_progress,omitempty"` // 0.0 to 1.0
	
	// Reasoning context
	WorkingMemory []string          `json:"working_memory,omitempty"` // Recent thoughts/decisions
	Decisions     []AgentDecision   `json:"decisions,omitempty"`
	Observations  []AgentObservation `json:"observations,omitempty"`
	
	// Knowledge references
	KnowledgeChunkIDs []string `json:"knowledge_chunk_ids,omitempty"` // IDs of retrieved KB chunks
	KnowledgeQueries  []string `json:"knowledge_queries,omitempty"`   // Queries made during session
	
	// Model usage
	ModelUsed    string    `json:"model_used,omitempty"`
	TokenCount   int       `json:"token_count,omitempty"`
	LastModelCall time.Time `json:"last_model_call,omitempty"`
	
	// Performance metrics
	StepsCompleted int       `json:"steps_completed"`
	ErrorsEncountered []AgentError `json:"errors_encountered,omitempty"`
	RetryCount     int       `json:"retry_count,omitempty"`
	
	// Timestamps
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastHeartbeat time.Time `json:"last_heartbeat,omitempty"`
	
	// Completion state
	IsComplete   bool      `json:"is_complete"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	Result       string    `json:"result,omitempty"` // "success", "failed", "canceled"
	ResultReason string    `json:"result_reason,omitempty"`
}

// AgentDecision represents a decision made by the agent.
type AgentDecision struct {
	ID          string    `json:"id"`
	Decision    string    `json:"decision"`
	Reason      string    `json:"reason"`
	Alternatives []string `json:"alternatives,omitempty"`
	Confidence  float64   `json:"confidence"` // 0.0 to 1.0
	MadeAt      time.Time `json:"made_at"`
}

// AgentObservation represents something the agent observed.
type AgentObservation struct {
	ID          string    `json:"id"`
	Observation string    `json:"observation"`
	Source      string    `json:"source"` // "system", "user", "tool", "knowledge"
	Timestamp   time.Time `json:"timestamp"`
	Significance float64  `json:"significance"` // 0.0 to 1.0
}

// AgentError represents an error encountered by the agent.
type AgentError struct {
	ID          string    `json:"id"`
	Error       string    `json:"error"`
	Step        string    `json:"step"`
	Recovered   bool      `json:"recovered"`
	RecoveryAction string `json:"recovery_action,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// NewAgentState creates a new agent state for a session.
func NewAgentState(agentID string, role AgentRole, sessionID, taskID string) *AgentState {
	now := time.Now()
	return &AgentState{
		AgentID:      agentID,
		AgentRole:    role,
		SessionID:    sessionID,
		TaskID:       taskID,
		CreatedAt:    now,
		UpdatedAt:    now,
		LastHeartbeat: now,
		IsComplete:   false,
		StepsCompleted: 0,
		RetryCount:   0,
	}
}

// Serialize serializes the agent state to JSON bytes.
func (s *AgentState) Serialize() ([]byte, error) {
	s.UpdatedAt = time.Now()
	return json.Marshal(s)
}

// DeserializeAgentState deserializes JSON bytes to AgentState.
func DeserializeAgentState(data []byte) (*AgentState, error) {
	var state AgentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to deserialize agent state: %w", err)
	}
	return &state, nil
}

// UpdateStep updates the current step of the agent.
func (s *AgentState) UpdateStep(step string) {
	s.CurrentStep = step
	s.StepStarted = time.Now()
	s.StepProgress = 0.0
	s.UpdatedAt = time.Now()
}

// UpdateProgress updates the progress of the current step.
func (s *AgentState) UpdateProgress(progress float64) {
	if progress < 0.0 {
		progress = 0.0
	}
	if progress > 1.0 {
		progress = 1.0
	}
	s.StepProgress = progress
	s.UpdatedAt = time.Now()
}

// CompleteStep marks the current step as complete.
func (s *AgentState) CompleteStep() {
	s.StepsCompleted++
	s.CurrentStep = ""
	s.StepProgress = 1.0
	s.StepStarted = time.Time{}
	s.UpdatedAt = time.Now()
}

// AddDecision records a decision made by the agent.
func (s *AgentState) AddDecision(decision, reason string, confidence float64, alternatives []string) {
	dec := AgentDecision{
		ID:          fmt.Sprintf("decision-%d", len(s.Decisions)+1),
		Decision:    decision,
		Reason:      reason,
		Alternatives: alternatives,
		Confidence:  confidence,
		MadeAt:      time.Now(),
	}
	s.Decisions = append(s.Decisions, dec)
	s.UpdatedAt = time.Now()
}

// AddObservation records an observation made by the agent.
func (s *AgentState) AddObservation(observation, source string, significance float64) {
	obs := AgentObservation{
		ID:          fmt.Sprintf("observation-%d", len(s.Observations)+1),
		Observation: observation,
		Source:      source,
		Timestamp:   time.Now(),
		Significance: significance,
	}
	s.Observations = append(s.Observations, obs)
	s.UpdatedAt = time.Now()
}

// AddError records an error encountered by the agent.
func (s *AgentState) AddError(errStr, step string, recovered bool, recoveryAction string) {
	err := AgentError{
		ID:            fmt.Sprintf("error-%d", len(s.ErrorsEncountered)+1),
		Error:         errStr,
		Step:          step,
		Recovered:     recovered,
		RecoveryAction: recoveryAction,
		OccurredAt:    time.Now(),
	}
	s.ErrorsEncountered = append(s.ErrorsEncountered, err)
	s.UpdatedAt = time.Now()
}

// AddKnowledgeReference records a knowledge chunk reference.
func (s *AgentState) AddKnowledgeReference(chunkID string) {
	s.KnowledgeChunkIDs = append(s.KnowledgeChunkIDs, chunkID)
	s.UpdatedAt = time.Now()
}

// RecordKnowledgeQuery records a knowledge query made by the agent.
func (s *AgentState) RecordKnowledgeQuery(query string) {
	s.KnowledgeQueries = append(s.KnowledgeQueries, query)
	s.UpdatedAt = time.Now()
}

// RecordModelCall records a model usage event.
func (s *AgentState) RecordModelCall(model string, tokens int) {
	s.ModelUsed = model
	s.TokenCount += tokens
	s.LastModelCall = time.Now()
	s.UpdatedAt = time.Now()
}

// UpdateHeartbeat updates the last heartbeat timestamp.
func (s *AgentState) UpdateHeartbeat() {
	s.LastHeartbeat = time.Now()
	s.UpdatedAt = time.Now()
}

// Complete marks the agent as completed with result.
func (s *AgentState) Complete(result, reason string) {
	s.IsComplete = true
	s.CompletedAt = time.Now()
	s.Result = result
	s.ResultReason = reason
	s.CurrentStep = ""
	s.StepProgress = 1.0
	s.UpdatedAt = time.Now()
}

// StateManager manages agent state persistence in ZenContext.
type StateManager struct {
	zenctx zenctx.ZenContext
	clusterID string
}

// NewStateManager creates a new StateManager.
func NewStateManager(zenctx zenctx.ZenContext, clusterID string) *StateManager {
	if clusterID == "" {
		clusterID = "default"
	}
	return &StateManager{
		zenctx:    zenctx,
		clusterID: clusterID,
	}
}

// StoreAgentState stores agent state in ZenContext.
func (m *StateManager) StoreAgentState(ctx context.Context, state *AgentState) error {
	if m.zenctx == nil {
		return fmt.Errorf("ZenContext not configured")
	}
	
	// Serialize state
	stateBytes, err := state.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize agent state: %w", err)
	}
	
	// Get or create SessionContext
	sessionCtx, err := m.zenctx.GetSessionContext(ctx, m.clusterID, state.SessionID)
	if err != nil || sessionCtx == nil {
		// SessionContext doesn't exist, create it via ReMe reconstruction
		req := zenctx.ReMeRequest{
			SessionID: state.SessionID,
			TaskID:    state.TaskID,
			ClusterID: m.clusterID,
		}
		resp, err := m.zenctx.ReconstructSession(ctx, req)
		if err != nil || resp.SessionContext == nil {
			return fmt.Errorf("failed to reconstruct session context: %w", err)
		}
		sessionCtx = resp.SessionContext
	}
	
	// Update SessionContext with agent state
	sessionCtx.State = stateBytes
	sessionCtx.LastAccessedAt = time.Now()
	
	// Store back to ZenContext
	if err := m.zenctx.StoreSessionContext(ctx, m.clusterID, sessionCtx); err != nil {
		return fmt.Errorf("failed to store session context: %w", err)
	}
	
	return nil
}

// LoadAgentState loads agent state from ZenContext.
func (m *StateManager) LoadAgentState(ctx context.Context, sessionID string) (*AgentState, error) {
	if m.zenctx == nil {
		return nil, fmt.Errorf("ZenContext not configured")
	}
	
	// Try to get SessionContext directly
	sessionCtx, err := m.zenctx.GetSessionContext(ctx, m.clusterID, sessionID)
	if err != nil || sessionCtx == nil {
		// Try reconstruction via ReMe protocol
		req := zenctx.ReMeRequest{
			SessionID: sessionID,
			ClusterID: m.clusterID,
		}
		resp, err := m.zenctx.ReconstructSession(ctx, req)
		if err != nil || resp.SessionContext == nil {
			return nil, fmt.Errorf("failed to reconstruct session: %w", err)
		}
		sessionCtx = resp.SessionContext
	}
	
	// If no state stored, return nil (not an error)
	if len(sessionCtx.State) == 0 {
		return nil, nil
	}
	
	// Deserialize agent state
	state, err := DeserializeAgentState(sessionCtx.State)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize agent state: %w", err)
	}
	
	return state, nil
}

// QueryKnowledge queries knowledge base via ZenContext and records it in agent state.
func (m *StateManager) QueryKnowledge(ctx context.Context, sessionID string, query string, scopes []string, limit int) ([]zenctx.KnowledgeChunk, error) {
	if m.zenctx == nil {
		return nil, fmt.Errorf("ZenContext not configured")
	}
	
	// Load agent state first
	agentState, err := m.LoadAgentState(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent state: %w", err)
	}
	
	// Execute query
	opts := zenctx.QueryOptions{
		Query:   query,
		Scopes:  scopes,
		Limit:   limit,
	}
	
	chunks, err := m.zenctx.QueryKnowledge(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("knowledge query failed: %w", err)
	}
	
	// Record query and chunk references in agent state
	if agentState != nil {
		agentState.RecordKnowledgeQuery(query)
		for _, chunk := range chunks {
			agentState.AddKnowledgeReference(chunk.ID)
		}
		// Store updated state
		if err := m.StoreAgentState(ctx, agentState); err != nil {
			// Log but don't fail the query
			fmt.Printf("Warning: failed to store agent state after knowledge query: %v\n", err)
		}
	}
	
	return chunks, nil
}

// ReconstructAgent reconstructs agent state using ReMe protocol.
// This is called when an agent wakes up and needs to resume work.
func (m *StateManager) ReconstructAgent(ctx context.Context, sessionID, taskID string) (*AgentState, []zenctx.KnowledgeChunk, error) {
	if m.zenctx == nil {
		return nil, nil, fmt.Errorf("ZenContext not configured")
	}
	
	// Reconstruct session using ReMe protocol
	req := zenctx.ReMeRequest{
		SessionID: sessionID,
		TaskID:    taskID,
		ClusterID: m.clusterID,
	}
	
	resp, err := m.zenctx.ReconstructSession(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("ReMe reconstruction failed: %w", err)
	}
	
	// Deserialize agent state if exists
	var agentState *AgentState
	if len(resp.SessionContext.State) > 0 {
		agentState, err = DeserializeAgentState(resp.SessionContext.State)
		if err != nil {
			// Log but continue with fresh state
			fmt.Printf("Warning: failed to deserialize agent state: %v\n", err)
		}
	}
	
	// If no agent state exists, create fresh one
	if agentState == nil {
		agentState = NewAgentState("", RolePlanner, sessionID, taskID)
	}
	
	return agentState, resp.SessionContext.RelevantKnowledge, nil
}