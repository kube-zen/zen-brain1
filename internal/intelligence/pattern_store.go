// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// PatternStore persists and retrieves learned patterns.
type PatternStore interface {
	// StorePatterns persists mining results.
	StorePatterns(ctx context.Context, result *MiningResult) error

	// GetWorkTypeStats retrieves statistics for a work type.
	GetWorkTypeStats(ctx context.Context, workType, workDomain string) (*WorkTypeStatistics, error)

	// GetAllWorkTypeStats retrieves all work type statistics.
	GetAllWorkTypeStats(ctx context.Context) ([]WorkTypeStatistics, error)

	// GetTemplateStats retrieves statistics for a template.
	GetTemplateStats(ctx context.Context, templateName string) (*TemplateStatistics, error)

	// GetAllTemplateStats retrieves all template statistics.
	GetAllTemplateStats(ctx context.Context) ([]TemplateStatistics, error)

	// GetDurationStats retrieves duration statistics for a work type.
	GetDurationStats(ctx context.Context, workType, workDomain string) (*DurationStatistics, error)

	// GetFailureStats retrieves failure statistics for a work type/domain.
	GetFailureStats(ctx context.Context, workType, workDomain string) (*FailureStatistics, error)

	// GetAllFailureStats retrieves all failure statistics.
	GetAllFailureStats(ctx context.Context) ([]FailureStatistics, error)

	// ClearPatterns clears all stored patterns.
	ClearPatterns(ctx context.Context) error
}

// JSONPatternStore is a file-based pattern store using JSON.
type JSONPatternStore struct {
	storeDir     string
	failureStore FailureStore
	mu           sync.RWMutex
}

// NewJSONPatternStore creates a new JSON-based pattern store.
func NewJSONPatternStore(storeDir string) (*JSONPatternStore, error) {
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create pattern store directory: %w", err)
	}

	failureStore, err := NewJSONFailureStore(storeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create failure store: %w", err)
	}

	return &JSONPatternStore{
		storeDir:     storeDir,
		failureStore: failureStore,
	}, nil
}

// StorePatterns persists mining results to JSON files.
func (s *JSONPatternStore) StorePatterns(ctx context.Context, result *MiningResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store mining result metadata
	resultPath := filepath.Join(s.storeDir, "latest-mining-result.json")
	if err := s.writeJSON(resultPath, result); err != nil {
		return fmt.Errorf("failed to write mining result: %w", err)
	}

	// Store work type statistics
	for _, stats := range result.WorkTypeStatistics {
		key := fmt.Sprintf("%s-%s", stats.WorkType, stats.WorkDomain)
		path := filepath.Join(s.storeDir, "worktypes", fmt.Sprintf("%s.json", key))
		if err := s.writeJSON(path, stats); err != nil {
			return fmt.Errorf("failed to write work type stats: %w", err)
		}
	}

	// Store template statistics
	for _, stats := range result.TemplateStatistics {
		path := filepath.Join(s.storeDir, "templates", fmt.Sprintf("%s.json", stats.TemplateName))
		if err := s.writeJSON(path, stats); err != nil {
			return fmt.Errorf("failed to write template stats: %w", err)
		}
	}

	// Store duration statistics
	for _, stats := range result.DurationStatistics {
		key := fmt.Sprintf("%s-%s", stats.WorkType, stats.WorkDomain)
		path := filepath.Join(s.storeDir, "durations", fmt.Sprintf("%s.json", key))
		if err := s.writeJSON(path, stats); err != nil {
			return fmt.Errorf("failed to write duration stats: %w", err)
		}
	}

	// Store failure statistics
	for _, stats := range result.FailureStatistics {
		if err := s.failureStore.StoreFailureStats(ctx, &stats); err != nil {
			return fmt.Errorf("failed to write failure stats: %w", err)
		}
	}

	log.Printf("[PatternStore] Stored patterns: %d work types, %d templates, %d duration stats, %d failure stats",
		len(result.WorkTypeStatistics), len(result.TemplateStatistics), len(result.DurationStatistics), len(result.FailureStatistics))

	return nil
}

// GetWorkTypeStats retrieves statistics for a work type.
func (s *JSONPatternStore) GetWorkTypeStats(ctx context.Context, workType, workDomain string) (*WorkTypeStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s-%s", workType, workDomain)
	path := filepath.Join(s.storeDir, "worktypes", fmt.Sprintf("%s.json", key))

	var stats WorkTypeStatistics
	if err := s.readJSON(path, &stats); err != nil {
		return nil, fmt.Errorf("failed to read work type stats: %w", err)
	}

	return &stats, nil
}

// GetAllWorkTypeStats retrieves all work type statistics.
func (s *JSONPatternStore) GetAllWorkTypeStats(ctx context.Context) ([]WorkTypeStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.storeDir, "worktypes")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read work types directory: %w", err)
	}

	stats := make([]WorkTypeStatistics, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		var stat WorkTypeStatistics
		if err := s.readJSON(path, &stat); err != nil {
			continue // Skip invalid files
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// GetTemplateStats retrieves statistics for a template.
func (s *JSONPatternStore) GetTemplateStats(ctx context.Context, templateName string) (*TemplateStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.storeDir, "templates", fmt.Sprintf("%s.json", templateName))

	var stats TemplateStatistics
	if err := s.readJSON(path, &stats); err != nil {
		return nil, fmt.Errorf("failed to read template stats: %w", err)
	}

	return &stats, nil
}

// GetAllTemplateStats retrieves all template statistics.
func (s *JSONPatternStore) GetAllTemplateStats(ctx context.Context) ([]TemplateStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.storeDir, "templates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	stats := make([]TemplateStatistics, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		var stat TemplateStatistics
		if err := s.readJSON(path, &stat); err != nil {
			continue // Skip invalid files
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// GetDurationStats retrieves duration statistics for a work type.
func (s *JSONPatternStore) GetDurationStats(ctx context.Context, workType, workDomain string) (*DurationStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s-%s", workType, workDomain)
	path := filepath.Join(s.storeDir, "durations", fmt.Sprintf("%s.json", key))

	var stats DurationStatistics
	if err := s.readJSON(path, &stats); err != nil {
		return nil, fmt.Errorf("failed to read duration stats: %w", err)
	}

	return &stats, nil
}

// GetFailureStats retrieves failure statistics for a work type/domain.
func (s *JSONPatternStore) GetFailureStats(ctx context.Context, workType, workDomain string) (*FailureStatistics, error) {
	return s.failureStore.GetFailureStats(ctx, workType, workDomain)
}

// GetAllFailureStats retrieves all failure statistics.
func (s *JSONPatternStore) GetAllFailureStats(ctx context.Context) ([]FailureStatistics, error) {
	return s.failureStore.GetAllFailureStats(ctx)
}

// ClearPatterns clears all stored patterns.
func (s *JSONPatternStore) ClearPatterns(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dirs := []string{
		filepath.Join(s.storeDir, "worktypes"),
		filepath.Join(s.storeDir, "templates"),
		filepath.Join(s.storeDir, "durations"),
	}

	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("failed to clear patterns directory %s: %w", dir, err)
		}
	}

	// Recreate directories
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to recreate patterns directory %s: %w", dir, err)
		}
	}

	log.Printf("[PatternStore] Cleared all patterns")

	return nil
}

// writeJSON writes data to a JSON file.
func (s *JSONPatternStore) writeJSON(path string, data interface{}) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

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
func (s *JSONPatternStore) readJSON(path string, data interface{}) error {
	jsonData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonData, data); err != nil {
		return err
	}

	return nil
}

// InMemoryPatternStore is an in-memory pattern store for testing.
type InMemoryPatternStore struct {
	workTypeStats map[string]*WorkTypeStatistics
	templateStats map[string]*TemplateStatistics
	durationStats map[string]*DurationStatistics
	failureStats  map[string]*FailureStatistics
	miningResult  *MiningResult
	mu            sync.RWMutex
}

// NewInMemoryPatternStore creates a new in-memory pattern store.
func NewInMemoryPatternStore() *InMemoryPatternStore {
	return &InMemoryPatternStore{
		workTypeStats: make(map[string]*WorkTypeStatistics),
		templateStats: make(map[string]*TemplateStatistics),
		durationStats: make(map[string]*DurationStatistics),
		failureStats:  make(map[string]*FailureStatistics),
	}
}

// StorePatterns persists mining results in memory.
func (s *InMemoryPatternStore) StorePatterns(ctx context.Context, result *MiningResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.miningResult = result

	// Store work type statistics
	for _, stats := range result.WorkTypeStatistics {
		key := fmt.Sprintf("%s-%s", stats.WorkType, stats.WorkDomain)
		copied := stats
		s.workTypeStats[key] = &copied
	}

	// Store template statistics
	for _, stats := range result.TemplateStatistics {
		copied := stats
		s.templateStats[stats.TemplateName] = &copied
	}

	// Store duration statistics
	for _, stats := range result.DurationStatistics {
		key := fmt.Sprintf("%s-%s", stats.WorkType, stats.WorkDomain)
		copied := stats
		s.durationStats[key] = &copied
	}

	// Store failure statistics (for testing, allow access via GetAllFailureStats)
	for _, stats := range result.FailureStatistics {
		key := fmt.Sprintf("%s-%s", stats.WorkType, stats.WorkDomain)
		if stats.TemplateName != "" {
			key = fmt.Sprintf("%s-%s-%s", stats.WorkType, stats.WorkDomain, stats.TemplateName)
		}
		copied := stats
		s.failureStats[key] = &copied
	}

	return nil
}

// GetWorkTypeStats retrieves statistics for a work type.
func (s *InMemoryPatternStore) GetWorkTypeStats(ctx context.Context, workType, workDomain string) (*WorkTypeStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s-%s", workType, workDomain)
	stats, exists := s.workTypeStats[key]
	if !exists {
		return nil, fmt.Errorf("work type stats not found: %s", key)
	}
	return stats, nil
}

// GetAllWorkTypeStats retrieves all work type statistics.
func (s *InMemoryPatternStore) GetAllWorkTypeStats(ctx context.Context) ([]WorkTypeStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make([]WorkTypeStatistics, 0, len(s.workTypeStats))
	for _, stat := range s.workTypeStats {
		stats = append(stats, *stat)
	}
	return stats, nil
}

// GetTemplateStats retrieves statistics for a template.
func (s *InMemoryPatternStore) GetTemplateStats(ctx context.Context, templateName string) (*TemplateStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, exists := s.templateStats[templateName]
	if !exists {
		return nil, fmt.Errorf("template stats not found: %s", templateName)
	}
	return stats, nil
}

// GetAllTemplateStats retrieves all template statistics.
func (s *InMemoryPatternStore) GetAllTemplateStats(ctx context.Context) ([]TemplateStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make([]TemplateStatistics, 0, len(s.templateStats))
	for _, stat := range s.templateStats {
		stats = append(stats, *stat)
	}
	return stats, nil
}

// GetDurationStats retrieves duration statistics for a work type.
func (s *InMemoryPatternStore) GetDurationStats(ctx context.Context, workType, workDomain string) (*DurationStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s-%s", workType, workDomain)
	stats, exists := s.durationStats[key]
	if !exists {
		return nil, fmt.Errorf("duration stats not found: %s", key)
	}
	return stats, nil
}

// GetFailureStats retrieves failure statistics for a work type/domain.
func (s *InMemoryPatternStore) GetFailureStats(ctx context.Context, workType, workDomain string) (*FailureStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s-%s", workType, workDomain)
	stats, exists := s.failureStats[key]
	if !exists {
		return nil, fmt.Errorf("failure stats not found: %s/%s", workType, workDomain)
	}
	return stats, nil
}

// GetAllFailureStats retrieves all failure statistics.
func (s *InMemoryPatternStore) GetAllFailureStats(ctx context.Context) ([]FailureStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make([]FailureStatistics, 0, len(s.failureStats))
	for _, stat := range s.failureStats {
		stats = append(stats, *stat)
	}
	return stats, nil
}

// ClearPatterns clears all stored patterns.
func (s *InMemoryPatternStore) ClearPatterns(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.workTypeStats = make(map[string]*WorkTypeStatistics)
	s.templateStats = make(map[string]*TemplateStatistics)
	s.durationStats = make(map[string]*DurationStatistics)
	s.miningResult = nil

	return nil
}

// Ensure interfaces are implemented
var _ PatternStore = (*JSONPatternStore)(nil)
var _ PatternStore = (*InMemoryPatternStore)(nil)
