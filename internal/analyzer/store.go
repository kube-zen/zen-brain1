// Package analyzer provides durable, auditable analysis history storage (Block 2 enterprise).
package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// AnalysisHistoryStore persists and retrieves analysis results by work item ID.
type AnalysisHistoryStore interface {
	Store(ctx context.Context, workItemID string, result *contracts.AnalysisResult) error
	GetHistory(ctx context.Context, workItemID string) ([]*contracts.AnalysisResult, error)
}

// FileAnalysisStore implements AnalysisHistoryStore using JSONL files under a directory.
// One file per work item (sanitized ID); each line is one analysis result (newest appended last).
type FileAnalysisStore struct {
	dir string
	mu  sync.Mutex
}

// NewFileAnalysisStore creates a store under the given directory (e.g. paths.Analysis).
func NewFileAnalysisStore(dir string) (*FileAnalysisStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("analysis store directory is required")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create analysis store dir: %w", err)
	}
	return &FileAnalysisStore{dir: dir}, nil
}

func sanitizeWorkItemID(id string) string {
	s := strings.ReplaceAll(id, string(filepath.Separator), "_")
	s = strings.ReplaceAll(s, "..", "_")
	if s == "" {
		s = "_empty"
	}
	return s
}

// Store appends the analysis result to the work item's history file (JSONL).
func (s *FileAnalysisStore) Store(ctx context.Context, workItemID string, result *contracts.AnalysisResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fname := filepath.Join(s.dir, sanitizeWorkItemID(workItemID)+".jsonl")
	f, err := os.OpenFile(fname, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open analysis history file: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("write analysis result: %w", err)
	}
	return nil
}

// GetHistory returns all stored analysis results for the work item (oldest first).
func (s *FileAnalysisStore) GetHistory(ctx context.Context, workItemID string) ([]*contracts.AnalysisResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fname := filepath.Join(s.dir, sanitizeWorkItemID(workItemID)+".jsonl")
	data, err := os.ReadFile(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read analysis history: %w", err)
	}
	var results []*contracts.AnalysisResult
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var r contracts.AnalysisResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue // skip malformed lines
		}
		results = append(results, &r)
	}
	return results, nil
}
