// Package qmd provides qmd client implementations.
// This file contains a mock qmd client for development/testing when
// the real qmd CLI is not available.
package qmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/qmd"
)

// MockClient is a mock implementation of qmd.Client that returns
// simulated search results without requiring the qmd CLI.
// This is useful for development, testing, and environments where
// qmd is not installed.
type MockClient struct {
	// SearchResults is a map of query patterns to mock results
	SearchResults map[string][]byte

	// Verbose enables verbose logging
	Verbose bool

	// SimulateLatency adds artificial delay to simulate real qmd
	SimulateLatency time.Duration

	// AlwaysFail makes all operations return errors (for testing)
	AlwaysFail bool
}

// MockConfig holds configuration for the mock qmd client.
type MockConfig struct {
	// Verbose enables verbose logging
	Verbose bool

	// SimulateLatency adds artificial delay (default: 100ms)
	SimulateLatency time.Duration

	// SearchResults can be pre-populated with mock results
	SearchResults map[string][]byte
}

// DefaultMockConfig returns the default mock configuration.
func DefaultMockConfig() *MockConfig {
	return &MockConfig{
		Verbose:         false,
		SimulateLatency: 100 * time.Millisecond,
		SearchResults:   make(map[string][]byte),
	}
}

// NewMockClient creates a new mock qmd client.
func NewMockClient(config *MockConfig) (*MockClient, error) {
	if config == nil {
		config = DefaultMockConfig()
	}

	// Initialize with some default search results if none provided
	if len(config.SearchResults) == 0 {
		config.SearchResults = defaultSearchResults()
	}

	return &MockClient{
		SearchResults:   config.SearchResults,
		Verbose:         config.Verbose,
		SimulateLatency: config.SimulateLatency,
	}, nil
}

// RefreshIndex simulates refreshing the search index.
// In mock mode, this is a no-op that logs the action.
func (m *MockClient) RefreshIndex(ctx context.Context, req qmd.EmbedRequest) error {
	if m.AlwaysFail {
		return fmt.Errorf("mock qmd: forced failure for RefreshIndex")
	}

	if m.Verbose {
		log.Printf("[MockQMD] RefreshIndex: repo=%s, paths=%v",
			req.RepoPath, req.Paths)
	}

	// Simulate latency
	if m.SimulateLatency > 0 {
		select {
		case <-time.After(m.SimulateLatency):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// Search returns mock search results based on the query.
// It looks for matching patterns in SearchResults, or returns
// generic mock results if no specific match is found.
func (m *MockClient) Search(ctx context.Context, req qmd.SearchRequest) ([]byte, error) {
	if m.AlwaysFail {
		return nil, fmt.Errorf("mock qmd: forced failure for Search")
	}

	if m.Verbose {
		log.Printf("[MockQMD] Search: repo=%s, query=%q, limit=%d, json=%v",
			req.RepoPath, req.Query, req.Limit, req.JSON)
	}

	// Simulate latency
	if m.SimulateLatency > 0 {
		select {
		case <-time.After(m.SimulateLatency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Check for exact match in SearchResults
	if result, ok := m.SearchResults[req.Query]; ok {
		return result, nil
	}

	// Check for partial matches
	for pattern, result := range m.SearchResults {
		if containsQuery(req.Query, pattern) {
			return result, nil
		}
	}

	// Return generic mock results
	return genericMockResults(req.Query, req.Limit), nil
}

// defaultSearchResults provides default mock search results for common queries.
func defaultSearchResults() map[string][]byte {
	results := make(map[string][]byte)

	// Architecture queries
	results["three tier architecture"] = []byte(`{
		"results": [
			{
				"path": "docs/architecture/three-tier.md",
				"title": "Three‑Tier Memory Architecture",
				"content": "Zen‑Brain uses a three‑tier memory architecture: Tier 1 (Hot) – Redis, Tier 2 (Warm) – QMD knowledge base, Tier 3 (Cold) – S3 archival.",
				"score": 0.95,
				"metadata": {
					"type": "architecture",
					"domain": "core",
					"tags": ["memory", "architecture", "tiered"]
				}
			}
		]
	}`)

	// Factory queries
	results["factory execution bounded loop"] = []byte(`{
		"results": [
			{
				"path": "docs/design/factory.md",
				"title": "Factory: Bounded Execution",
				"content": "The Factory executes tasks in bounded loops with timeout and retry logic. Each task runs in an isolated workspace with proof‑of‑work generation.",
				"score": 0.92,
				"metadata": {
					"type": "design",
					"domain": "execution",
					"tags": ["factory", "execution", "bounded"]
				}
			}
		]
	}`)

	// Jira queries
	results["jira integration"] = []byte(`{
		"results": [
			{
				"path": "docs/integrations/jira.md",
				"title": "Jira Integration",
				"content": "Zen‑Brain integrates with Atlassian Jira for work item intake, status updates, and AI attribution headers.",
				"score": 0.88,
				"metadata": {
					"type": "integration",
					"domain": "office",
					"tags": ["jira", "atlassian", "integration"]
				}
			}
		]
	}`)

	// Proof of work queries
	results["proof of work"] = []byte(`{
		"results": [
			{
				"path": "docs/design/proof-of-work.md",
				"title": "Proof of Work Artifacts",
				"content": "Factory generates three proof‑of‑work artifacts: JSON (structured), Markdown (human‑readable), and Log (execution details).",
				"score": 0.90,
				"metadata": {
					"type": "design",
					"domain": "evidence",
					"tags": ["proof-of-work", "artifacts", "evidence"]
				}
			}
		]
	}`)

	return results
}

// genericMockResults generates generic mock results for any query.
func genericMockResults(query string, limit int) []byte {
	if limit <= 0 {
		limit = 5
	}

	results := []map[string]interface{}{
		{
			"path":    "docs/general/overview.md",
			"title":   "Zen‑Brain Overview",
			"content": fmt.Sprintf("This document provides an overview of Zen‑Brain. Your query '%s' matched general documentation.", query),
			"score":   0.75,
			"metadata": map[string]interface{}{
				"type":   "general",
				"domain": "core",
				"tags":   []string{"overview", "general"},
			},
		},
		{
			"path":    "docs/architecture/design-principles.md",
			"title":   "Design Principles",
			"content": "Zen‑Brain follows principles of simplicity, reliability, and auditability. All execution produces verifiable proof‑of‑work.",
			"score":   0.65,
			"metadata": map[string]interface{}{
				"type":   "architecture",
				"domain": "core",
				"tags":   []string{"design", "principles", "architecture"},
			},
		},
	}

	// Truncate to limit
	if len(results) > limit {
		results = results[:limit]
	}

	// Wrap in "results" object to match qmd JSON format
	wrapped := map[string]interface{}{
		"results": results,
	}
	data, _ := json.Marshal(wrapped)
	return data
}

// containsQuery checks if the query contains the pattern (case-insensitive).
func containsQuery(query, pattern string) bool {
	// Simple substring matching
	queryLower := strings.ToLower(query)
	patternLower := strings.ToLower(pattern)

	// First check if pattern is a substring of query
	if strings.Contains(queryLower, patternLower) {
		return true
	}

	// Also check if query is a substring of pattern (for reversed matches)
	if strings.Contains(patternLower, queryLower) {
		return true
	}

	// For queries with scope suffixes like "(scope: design OR execution)",
	// try to match the main part before the scope
	if idx := strings.Index(queryLower, " (scope:"); idx > 0 {
		mainQuery := queryLower[:idx]
		if strings.Contains(mainQuery, patternLower) || strings.Contains(patternLower, mainQuery) {
			return true
		}
	}

	return false
}

// SetSearchResult adds or updates a mock search result.
func (m *MockClient) SetSearchResult(query string, result []byte) {
	m.SearchResults[query] = result
}

// ClearSearchResults clears all mock search results.
func (m *MockClient) ClearSearchResults() {
	m.SearchResults = make(map[string][]byte)
}
