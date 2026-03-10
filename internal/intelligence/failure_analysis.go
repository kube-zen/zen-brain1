// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FailureMode represents the type of failure encountered.
type FailureMode string

const (
	FailureUnknown   FailureMode = "unknown"
	FailureTest      FailureMode = "test"
	FailureTimeout   FailureMode = "timeout"
	FailureValidation FailureMode = "validation"
	FailureRuntime   FailureMode = "runtime"
	FailureWorkspace FailureMode = "workspace"
	FailurePolicy    FailureMode = "policy"
	FailureInfra     FailureMode = "infra"
)

// classifyFailure classifies a failure based on the proof-of-work summary.
// Uses deterministic heuristics to identify failure modes.
func classifyFailure(summary *ProofOfWorkSummary) FailureMode {
	// If result is completed, no failure
	if summary.Result == "completed" {
		return "" // Empty string for success
	}

	// Check for test failures
	if len(summary.TestsFailed) > 0 {
		return FailureTest
	}

	// Analyze error log for other failure modes
	errorLog := strings.ToLower(summary.ErrorLog)
	if errorLog == "" && summary.OutputLog != "" {
		errorLog = strings.ToLower(summary.OutputLog)
	}

	// Check for timeout
	if strings.Contains(errorLog, "timeout") || strings.Contains(errorLog, "deadline exceeded") {
		return FailureTimeout
	}

	// Check for validation failures
	if strings.Contains(errorLog, "validation") || strings.Contains(errorLog, "invalid") {
		return FailureValidation
	}

	// Check for workspace failures
	if strings.Contains(errorLog, "workspace") || strings.Contains(errorLog, "git") || strings.Contains(errorLog, "repository") {
		return FailureWorkspace
	}

	// Check for policy failures
	if strings.Contains(errorLog, "policy") || strings.Contains(errorLog, "approval") || strings.Contains(errorLog, "authorized") {
		return FailurePolicy
	}

	// Check for infrastructure failures
	if strings.Contains(errorLog, "connection refused") ||
		strings.Contains(errorLog, "redis") ||
		strings.Contains(errorLog, "cockroach") ||
		strings.Contains(errorLog, "dial tcp") ||
		strings.Contains(errorLog, "no such host") ||
		strings.Contains(errorLog, "network") ||
		strings.Contains(errorLog, "dns") {
		return FailureInfra
	}

	// Default to runtime/unknown
	if errorLog != "" {
		return FailureRuntime
	}

	return FailureUnknown
}

// FailureStatistics tracks failure modes by work type and domain.
type FailureStatistics struct {
	WorkType          string            `json:"work_type"`
	WorkDomain        string            `json:"work_domain"`
	TemplateName      string            `json:"template_name,omitempty"`
	TotalFailures     int               `json:"total_failures"`
	FailureModes      map[string]int    `json:"failure_modes,omitempty"`
	LastFailureAt     time.Time         `json:"last_failure_at,omitempty"`
	RecommendedActions map[string]int    `json:"recommended_actions,omitempty"`
}

// FailureStore persists and retrieves failure statistics.
type FailureStore interface {
	// StoreFailureStats persists failure statistics.
	StoreFailureStats(ctx context.Context, stats *FailureStatistics) error

	// GetFailureStats retrieves failure statistics for a work type/domain.
	GetFailureStats(ctx context.Context, workType, workDomain string) (*FailureStatistics, error)

	// GetAllFailureStats retrieves all failure statistics.
	GetAllFailureStats(ctx context.Context) ([]FailureStatistics, error)

	// ClearFailureStats clears all stored failure statistics.
	ClearFailureStats(ctx context.Context) error
}

// JSONFailureStore is a file-based failure store using JSON.
type JSONFailureStore struct {
	storeDir string
	mu       sync.RWMutex
}

// NewJSONFailureStore creates a new JSON-based failure store.
func NewJSONFailureStore(storeDir string) (*JSONFailureStore, error) {
	failuresDir := filepath.Join(storeDir, "failures")
	if err := os.MkdirAll(failuresDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create failure store directory: %w", err)
	}

	return &JSONFailureStore{
		storeDir: failuresDir,
	}, nil
}

// StoreFailureStats persists failure statistics to JSON files.
func (s *JSONFailureStore) StoreFailureStats(ctx context.Context, stats *FailureStatistics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s-%s", stats.WorkType, stats.WorkDomain)
	if stats.TemplateName != "" {
		key = fmt.Sprintf("%s-%s-%s", stats.WorkType, stats.WorkDomain, stats.TemplateName)
	}
	path := filepath.Join(s.storeDir, fmt.Sprintf("%s.json", key))

	return s.writeJSON(path, stats)
}

// GetFailureStats retrieves failure statistics for a work type/domain.
func (s *JSONFailureStore) GetFailureStats(ctx context.Context, workType, workDomain string) (*FailureStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s-%s", workType, workDomain)
	path := filepath.Join(s.storeDir, fmt.Sprintf("%s.json", key))

	var stats FailureStatistics
	if err := s.readJSON(path, &stats); err != nil {
		return nil, fmt.Errorf("failed to read failure stats: %w", err)
	}

	return &stats, nil
}

// GetAllFailureStats retrieves all failure statistics.
func (s *JSONFailureStore) GetAllFailureStats(ctx context.Context) ([]FailureStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.storeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read failures directory: %w", err)
	}

	stats := make([]FailureStatistics, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(s.storeDir, entry.Name())
		var stat FailureStatistics
		if err := s.readJSON(path, &stat); err != nil {
			continue // Skip invalid files
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// ClearFailureStats clears all stored failure statistics.
func (s *JSONFailureStore) ClearFailureStats(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.RemoveAll(s.storeDir); err != nil {
		return fmt.Errorf("failed to clear failures directory: %w", err)
	}

	// Recreate directory
	if err := os.MkdirAll(s.storeDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate failures directory: %w", err)
	}

	return nil
}

// writeJSON writes data to a JSON file.
func (s *JSONFailureStore) writeJSON(path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// readJSON reads data from a JSON file.
func (s *JSONFailureStore) readJSON(path string, data interface{}) error {
	jsonData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonData, data); err != nil {
		return err
	}

	return nil
}

// InMemoryFailureStore is an in-memory failure store for testing.
type InMemoryFailureStore struct {
	stats map[string]*FailureStatistics
	mu    sync.RWMutex
}

// NewInMemoryFailureStore creates a new in-memory failure store.
func NewInMemoryFailureStore() *InMemoryFailureStore {
	return &InMemoryFailureStore{
		stats: make(map[string]*FailureStatistics),
	}
}

// StoreFailureStats persists failure statistics in memory.
func (s *InMemoryFailureStore) StoreFailureStats(ctx context.Context, stats *FailureStatistics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s-%s", stats.WorkType, stats.WorkDomain)
	if stats.TemplateName != "" {
		key = fmt.Sprintf("%s-%s-%s", stats.WorkType, stats.WorkDomain, stats.TemplateName)
	}
	copied := *stats
	s.stats[key] = &copied
	return nil
}

// GetFailureStats retrieves failure statistics for a work type/domain.
func (s *InMemoryFailureStore) GetFailureStats(ctx context.Context, workType, workDomain string) (*FailureStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s-%s", workType, workDomain)
	stats, exists := s.stats[key]
	if !exists {
		return nil, fmt.Errorf("failure stats not found: %s", key)
	}
	return stats, nil
}

// GetAllFailureStats retrieves all failure statistics.
func (s *InMemoryFailureStore) GetAllFailureStats(ctx context.Context) ([]FailureStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make([]FailureStatistics, 0, len(s.stats))
	for _, stat := range s.stats {
		stats = append(stats, *stat)
	}
	return stats, nil
}

// ClearFailureStats clears all stored failure statistics.
func (s *InMemoryFailureStore) ClearFailureStats(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats = make(map[string]*FailureStatistics)
	return nil
}

// Ensure interfaces are implemented
var _ FailureStore = (*JSONFailureStore)(nil)
var _ FailureStore = (*InMemoryFailureStore)(nil)
