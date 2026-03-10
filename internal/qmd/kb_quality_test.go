package qmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/kb"
)

// goldenQuery represents a test query with expected results
type goldenQuery struct {
	Query                  string   `json:"query"`
	Description            string   `json:"description"`
	Scopes                 []string `json:"scopes,omitempty"`
	Tags                   []string `json:"tags,omitempty"`
	ExpectedDocumentTitles []string `json:"expected_document_titles"`
	ExpectedDomain         string   `json:"expected_domain,omitempty"`
	MinScore               float64  `json:"min_score"`
}

// loadGoldenQueries loads golden queries from testdata
func loadGoldenQueries(t *testing.T) []goldenQuery {
	t.Helper()
	
	dataPath := filepath.Join("testdata", "golden_queries.json")
	data, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("Failed to load golden queries: %v", err)
	}
	
	var queries []goldenQuery
	if err := json.Unmarshal(data, &queries); err != nil {
		t.Fatalf("Failed to parse golden queries: %v", err)
	}
	
	return queries
}

// TestKBQualityGoldenQueries validates that KB search returns expected results
// for a set of golden queries. This test uses the mock qmd client with
// predefined results that match the golden set.
func TestKBQualityGoldenQueries(t *testing.T) {
	queries := loadGoldenQueries(t)
	
	// Create mock client with default search results (which match golden queries)
	mockConfig := &MockConfig{
		Verbose:         false,
		SimulateLatency: 0,
		SearchResults:   defaultSearchResults(),
	}
	
	mockClient, err := NewMockClient(mockConfig)
	if err != nil {
		t.Fatalf("Failed to create mock client: %v", err)
	}
	
	// Create KB store
	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
		Verbose:   false,
	}
	
	store, err := NewKBStore(config)
	if err != nil {
		t.Fatalf("Failed to create KB store: %v", err)
	}
	
	ctx := context.Background()
	
	for _, gq := range queries {
		t.Run(gq.Query, func(t *testing.T) {
			// Build search query
			req := kb.SearchQuery{
				Query:    gq.Query,
				KBScopes: gq.Scopes,
				Tags:     gq.Tags,
				Limit:    10,
			}
			
			results, err := store.Search(ctx, req)
			if err != nil {
				t.Fatalf("Search failed for query '%s': %v", gq.Query, err)
			}
			
			// Validate we got results
			if len(results) == 0 {
				t.Errorf("No results returned for query '%s'", gq.Query)
				return
			}
			
			// Check each expected document title appears in results
			foundTitles := make(map[string]bool)
			for _, result := range results {
				foundTitles[result.Doc.Title] = true
			}
			
			for _, expectedTitle := range gq.ExpectedDocumentTitles {
				if !foundTitles[expectedTitle] {
					t.Errorf("Expected document title '%s' not found in results for query '%s'", 
						expectedTitle, gq.Query)
				}
			}
			
			// Check domain if specified
			if gq.ExpectedDomain != "" {
				domainFound := false
				for _, result := range results {
					if result.Doc.Domain == gq.ExpectedDomain {
						domainFound = true
						break
					}
				}
				if !domainFound {
					t.Errorf("Expected domain '%s' not found in results for query '%s'",
						gq.ExpectedDomain, gq.Query)
				}
			}
			
			// Check minimum score
			for i, result := range results {
				if result.Score < gq.MinScore {
					t.Errorf("Result %d score %.3f below minimum %.3f for query '%s'",
						i, result.Score, gq.MinScore, gq.Query)
				}
			}
			
			// Log success for debugging
			t.Logf("Query '%s': found %d results, expected titles: %v",
				gq.Query, len(results), gq.ExpectedDocumentTitles)
		})
	}
}

// TestKBQualityScopeFiltering tests that scope filtering works correctly
func TestKBQualityScopeFiltering(t *testing.T) {
	// Create mock client with mixed domain results
	mockConfig := &MockConfig{
		Verbose: false,
		SearchResults: map[string][]byte{
			"mixed query": []byte(`{
				"results": [
					{
						"path": "docs/architecture.md",
						"title": "Architecture Document",
						"content": "Architecture content",
						"score": 0.9,
						"metadata": {
							"type": "architecture",
							"domain": "architecture",
							"tags": ["architecture"]
						}
					},
					{
						"path": "docs/operations.md",
						"title": "Operations Document",
						"content": "Operations content",
						"score": 0.8,
						"metadata": {
							"type": "operations",
							"domain": "ops",
							"tags": ["operations"]
						}
					}
				]
			}`),
		},
	}
	
	mockClient, err := NewMockClient(mockConfig)
	if err != nil {
		t.Fatalf("Failed to create mock client: %v", err)
	}
	
	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
		Verbose:   false,
	}
	
	store, err := NewKBStore(config)
	if err != nil {
		t.Fatalf("Failed to create KB store: %v", err)
	}
	
	ctx := context.Background()
	
	// Test with architecture scope
	req := kb.SearchQuery{
		Query:    "mixed query",
		KBScopes: []string{"architecture"},
		Limit:    10,
	}
	
	results, err := store.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Errorf("Expected 1 result with architecture scope, got %d", len(results))
	}
	
	if len(results) > 0 && results[0].Doc.Domain != "architecture" {
		t.Errorf("Expected domain 'architecture', got '%s'", results[0].Doc.Domain)
	}
}

// TestKBQualityTagFiltering tests that tag filtering works correctly
func TestKBQualityTagFiltering(t *testing.T) {
	mockConfig := &MockConfig{
		Verbose: false,
		SearchResults: map[string][]byte{
			"tag query": []byte(`{
				"results": [
					{
						"path": "docs/core.md",
						"title": "Core Document",
						"content": "Core content",
						"score": 0.9,
						"metadata": {
							"type": "core",
							"domain": "core",
							"tags": ["core", "architecture"]
						}
					},
					{
						"path": "docs/ops.md",
						"title": "Ops Document",
						"content": "Ops content",
						"score": 0.8,
						"metadata": {
							"type": "ops",
							"domain": "ops",
							"tags": ["operations", "devops"]
						}
					}
				]
			}`),
		},
	}
	
	mockClient, err := NewMockClient(mockConfig)
	if err != nil {
		t.Fatalf("Failed to create mock client: %v", err)
	}
	
	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
		Verbose:   false,
	}
	
	store, err := NewKBStore(config)
	if err != nil {
		t.Fatalf("Failed to create KB store: %v", err)
	}
	
	ctx := context.Background()
	
	// Test with core tag
	req := kb.SearchQuery{
		Query: "tag query",
		Tags:  []string{"core"},
		Limit: 10,
	}
	
	results, err := store.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if len(results) != 1 {
		t.Errorf("Expected 1 result with core tag, got %d", len(results))
	}
	
	if len(results) > 0 && results[0].Doc.Domain != "core" {
		t.Errorf("Expected domain 'core', got '%s'", results[0].Doc.Domain)
	}
}

// TestKBQualityEmptyResults tests handling of queries with no matches
func TestKBQualityEmptyResults(t *testing.T) {
	mockConfig := &MockConfig{
		Verbose: false,
		SearchResults: map[string][]byte{
			"no match query": []byte(`{"results": []}`),
		},
	}
	
	mockClient, err := NewMockClient(mockConfig)
	if err != nil {
		t.Fatalf("Failed to create mock client: %v", err)
	}
	
	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
		Verbose:   false,
	}
	
	store, err := NewKBStore(config)
	if err != nil {
		t.Fatalf("Failed to create KB store: %v", err)
	}
	
	ctx := context.Background()
	
	req := kb.SearchQuery{
		Query: "no match query",
		Limit: 10,
	}
	
	results, err := store.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if len(results) != 0 {
		t.Errorf("Expected 0 results for no-match query, got %d", len(results))
	}
}