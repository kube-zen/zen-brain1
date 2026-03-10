// Package session provides work session management.
package session

import "time"

// ExecutionCheckpoint is a structured execution checkpoint for ReMe/resume.
// Stored in ZenContext SessionContext.State as JSON.
type ExecutionCheckpoint struct {
	Stage                  string    `json:"stage"`
	SessionID              string    `json:"session_id"`
	WorkItemID             string    `json:"work_item_id"`
	BrainTaskIDs           []string  `json:"brain_task_ids,omitempty"`
	ProofPaths             []string  `json:"proof_paths,omitempty"`
	LastRecommendation     string    `json:"last_recommendation,omitempty"`
	KnowledgeChunkIDs      []string  `json:"knowledge_chunk_ids,omitempty"`
	KnowledgeSourcePaths   []string  `json:"knowledge_source_paths,omitempty"`
	UpdatedAt              time.Time `json:"updated_at"`
}
