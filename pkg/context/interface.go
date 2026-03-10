// Package context provides the ZenContext interface for tiered memory.
// ZenContext enables agents to retrieve relevant historical information and
// pick up where they left off.
//
// Three-tier memory architecture:
// - Tier 1 (Hot): Redis/tmpfs for sub-millisecond access to session context
// - Tier 2 (Warm): Vector database (QMD) for fast knowledge and procedure lookups
// - Tier 3 (Cold): Object storage for archival logs
package context

import (
	"context"
	"time"
)

// Tier represents the memory tier.
type Tier string

const (
	TierHot  Tier = "hot"  // Tier 1: Redis/tmpfs
	TierWarm Tier = "warm" // Tier 2: Vector database (QMD)
	TierCold Tier = "cold" // Tier 3: Object storage
)

// SessionContext contains the complete context for an agent session.
type SessionContext struct {
	// SessionID uniquely identifies the session
	SessionID string `json:"session_id"`

	// TaskID identifies the current task
	TaskID string `json:"task_id,omitempty"`

	// ClusterID for multi-cluster context
	ClusterID string `json:"cluster_id,omitempty"`

	// ProjectID for project context
	ProjectID string `json:"project_id,omitempty"`

	// CreatedAt is when the session was created
	CreatedAt time.Time `json:"created_at"`

	// LastAccessedAt is when the session was last accessed
	LastAccessedAt time.Time `json:"last_accessed_at"`

	// State is the current agent state (serialized)
	State []byte `json:"state,omitempty"`

	// RelevantKnowledge contains retrieved KB chunks for this session
	RelevantKnowledge []KnowledgeChunk `json:"relevant_knowledge,omitempty"`

	// JournalEntries are causal-chain entries from ReMe (ZenJournal) for this task/session.
	// Agents can use this to resume with full context.
	JournalEntries []interface{} `json:"journal_entries,omitempty"`

	// Scratchpad contains intermediate reasoning (Tier 1 only)
	Scratchpad []byte `json:"scratchpad,omitempty"`
}

// KnowledgeChunk represents a retrieved knowledge piece from QMD.
type KnowledgeChunk struct {
	// ID is the unique identifier of the chunk
	ID string `json:"id"`

	// Scope is the KB scope (company, general, project)
	Scope string `json:"scope"`

	// Content is the text content
	Content string `json:"content"`

	// SourcePath is the original file path
	SourcePath string `json:"source_path"`

	// HeadingPath is the heading hierarchy
	HeadingPath []string `json:"heading_path"`

	// SimilarityScore is the relevance score (0-1)
	SimilarityScore float64 `json:"similarity_score"`

	// RetrievedAt is when this chunk was retrieved
	RetrievedAt time.Time `json:"retrieved_at"`
}

// QueryOptions for retrieving knowledge.
type QueryOptions struct {
	// Query is the natural language query
	Query string `json:"query"`

	// Scopes filters by scope (company, general, project)
	Scopes []string `json:"scopes,omitempty"`

	// Limit limits the number of results
	Limit int `json:"limit,omitempty"`

	// MinSimilarity filters by minimum similarity score
	MinSimilarity float64 `json:"min_similarity,omitempty"`

	// ClusterID for multi-cluster context
	ClusterID string `json:"cluster_id,omitempty"`

	// ProjectID for project context
	ProjectID string `json:"project_id,omitempty"`
}

// ReMeRequest is a request for Recursive Memory reconstruction.
type ReMeRequest struct {
	// SessionID to reconstruct
	SessionID string `json:"session_id"`

	// TaskID to reconstruct
	TaskID string `json:"task_id"`

	// ClusterID for multi-cluster context
	ClusterID string `json:"cluster_id,omitempty"`

	// ProjectID for project context
	ProjectID string `json:"project_id,omitempty"`

	// UpToTime reconstruct up to this time (default: now)
	UpToTime time.Time `json:"up_to_time,omitempty"`
}

// ReMeResponse contains reconstructed context.
type ReMeResponse struct {
	// SessionContext is the reconstructed session context
	SessionContext *SessionContext `json:"session_context"`

	// JournalEntries are the relevant journal entries for reconstruction
	JournalEntries []interface{} `json:"journal_entries"`

	// ReconstructedAt is when reconstruction occurred
	ReconstructedAt time.Time `json:"reconstructed_at"`
}

// ZenContext is the interface for tiered memory.
type ZenContext interface {
	// GetSessionContext retrieves session context from Tier 1 (Hot).
	// Returns nil if session does not exist.
	GetSessionContext(ctx context.Context, clusterID, sessionID string) (*SessionContext, error)

	// StoreSessionContext stores session context in Tier 1 (Hot).
	StoreSessionContext(ctx context.Context, clusterID string, session *SessionContext) error

	// DeleteSessionContext deletes session context from Tier 1.
	DeleteSessionContext(ctx context.Context, clusterID, sessionID string) error

	// QueryKnowledge queries Tier 2 (Warm) for relevant knowledge.
	QueryKnowledge(ctx context.Context, opts QueryOptions) ([]KnowledgeChunk, error)

	// StoreKnowledge stores knowledge in Tier 2 (Warm).
	// Used by KB ingestion service.
	StoreKnowledge(ctx context.Context, chunks []KnowledgeChunk) error

	// ArchiveSession archives session context to Tier 3 (Cold).
	ArchiveSession(ctx context.Context, clusterID, sessionID string) error

	// ReconstructSession implements the ReMe protocol.
	// Reconstructs session state from ZenJournal and Tier 2/3.
	ReconstructSession(ctx context.Context, req ReMeRequest) (*ReMeResponse, error)

	// Stats returns memory usage statistics.
	Stats(ctx context.Context) (map[Tier]interface{}, error)

	// Close closes the context store.
	Close() error
}
