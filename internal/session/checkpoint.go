// Package session provides work session management.
package session

import (
	"sort"
	"time"
)

// ExecutionCheckpoint is a structured execution checkpoint for ReMe/resume.
// Stored in ZenContext SessionContext.State as JSON.
// Do not store large blobs or full proof JSON inside the checkpoint.
type ExecutionCheckpoint struct {
	Stage                string    `json:"stage"`
	SessionID            string    `json:"session_id"`
	WorkItemID           string    `json:"work_item_id"`
	BrainTaskIDs         []string  `json:"brain_task_ids,omitempty"`
	ProofPaths           []string  `json:"proof_paths,omitempty"`
	LastRecommendation   string    `json:"last_recommendation,omitempty"`
	SelectedModel        string    `json:"selected_model,omitempty"`
	AnalysisSummary      string    `json:"analysis_summary,omitempty"`
	KnowledgeChunkIDs    []string  `json:"knowledge_chunk_ids,omitempty"`
	KnowledgeSourcePaths []string  `json:"knowledge_source_paths,omitempty"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Normalize sorts and dedupes slice fields for deterministic storage.
// Call before persisting the checkpoint.
func (c *ExecutionCheckpoint) Normalize() {
	c.BrainTaskIDs = sortAndDedupe(c.BrainTaskIDs)
	c.ProofPaths = sortAndDedupe(c.ProofPaths)
	c.KnowledgeChunkIDs = sortAndDedupe(c.KnowledgeChunkIDs)
	c.KnowledgeSourcePaths = sortAndDedupe(c.KnowledgeSourcePaths)
}

func sortAndDedupe(s []string) []string {
	if len(s) == 0 {
		return s
	}
	seen := make(map[string]struct{}, len(s))
	for _, v := range s {
		seen[v] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ShouldSkipReplayForResume returns true when the checkpoint indicates execution already
// reached proof_attached or execution_complete with proof artifacts, so resume should not
// blindly rerun all tasks.
func ShouldSkipReplayForResume(cp *ExecutionCheckpoint) bool {
	if cp == nil || len(cp.ProofPaths) == 0 {
		return false
	}
	return cp.Stage == "proof_attached" || cp.Stage == "execution_complete"
}
