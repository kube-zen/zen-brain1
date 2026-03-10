package qmd

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/kb"
	qmdpkg "github.com/kube-zen/zen-brain1/pkg/qmd"
)

// mockQMDClient is a mock qmd.Client for testing KBStore.
type mockQMDClient struct {
	searchResults []SearchResult
	searchError   error
}

func (m *mockQMDClient) RefreshIndex(ctx context.Context, req qmdpkg.EmbedRequest) error {
	return nil
}

func (m *mockQMDClient) Search(ctx context.Context, req qmdpkg.SearchRequest) ([]byte, error) {
	if m.searchError != nil {
		return nil, m.searchError
	}

	// Convert mock results to JSON
	type MockResult struct {
		Path     string    `json:"path"`
		Title    string    `json:"title"`
		Score    float64   `json:"score"`
		Metadata *Metadata `json:"metadata,omitempty"`
	}

	mockWrapper := struct {
		Results []MockResult `json:"results"`
	}{
		Results: make([]MockResult, len(m.searchResults)),
	}

	for i, result := range m.searchResults {
		mockWrapper.Results[i] = MockResult{
			Path:     result.Path,
			Title:    result.Title,
			Score:    result.Score,
			Metadata: result.Metadata,
		}
	}

	return json.Marshal(mockWrapper)
}

func TestNewKBStore(t *testing.T) {
	mockClient := &mockQMDClient{}
	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
		Verbose:   true,
	}

	store, err := NewKBStore(config)

	if err != nil {
		t.Fatalf("NewKBStore failed: %v", err)
	}

	if store == nil {
		t.Fatal("Store should not be nil")
	}

	if store.qmdClient != mockClient {
		t.Error("QMDClient not set correctly")
	}

	if store.repoPath != "/path/to/repo" {
		t.Errorf("Expected repoPath '/path/to/repo', got '%s'", store.repoPath)
	}

	if store.verbose != true {
		t.Error("Expected verbose true")
	}
}

func TestNewKBStore_NilConfig(t *testing.T) {
	_, err := NewKBStore(nil)

	if err == nil {
		t.Error("Expected error for nil config")
	}

	if !strings.Contains(err.Error(), "config is required") {
		t.Errorf("Expected 'config is required' error, got: %v", err)
	}
}

func TestNewKBStore_NilQMDClient(t *testing.T) {
	config := &KBStoreConfig{
		QMDClient: nil,
		RepoPath:  "/path/to/repo",
	}

	_, err := NewKBStore(config)

	if err == nil {
		t.Error("Expected error for nil qmd client")
	}

	if !strings.Contains(err.Error(), "qmd client is required") {
		t.Errorf("Expected 'qmd client is required' error, got: %v", err)
	}
}

func TestNewKBStore_EmptyRepoPath(t *testing.T) {
	config := &KBStoreConfig{
		QMDClient: &mockQMDClient{},
		RepoPath:  "",
	}

	_, err := NewKBStore(config)

	if err == nil {
		t.Error("Expected error for empty repo path")
	}

	if !strings.Contains(err.Error(), "repo_path is required") {
		t.Errorf("Expected 'repo_path is required' error, got: %v", err)
	}
}

func TestKBStore_Search_Success(t *testing.T) {
	mockResults := []SearchResult{
		{
			Path:  "docs/architecture.md",
			Title: "Architecture Overview",
			Score: 0.95,
			Metadata: &Metadata{
				ID:     "KB-ARCH-0001",
				Domain: "architecture",
				Tags:   []string{"architecture", "core"},
			},
		},
		{
			Path:  "docs/deployment.md",
			Title: "Deployment Guide",
			Score: 0.87,
		},
	}

	mockClient := &mockQMDClient{
		searchResults: mockResults,
	}

	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
	}

	store, _ := NewKBStore(config)

	ctx := context.Background()
	req := kb.SearchQuery{
		Query: "architecture",
		Limit: 10,
	}

	results, err := store.Search(ctx, req)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results[0].Doc.Title != "Architecture Overview" {
		t.Errorf("Expected title 'Architecture Overview', got '%s'", results[0].Doc.Title)
	}

	if results[0].Doc.Source != "git" {
		t.Errorf("Expected source 'git', got '%s'", results[0].Doc.Source)
	}
}

func TestKBStore_Search_WithScopes(t *testing.T) {
	mockResults := []SearchResult{
		{
			Path:  "docs/architecture.md",
			Title: "Architecture Overview",
			Score: 0.95,
			Metadata: &Metadata{
				ID:     "KB-ARCH-0001",
				Domain: "architecture",
				Tags:   []string{"architecture", "core"},
			},
		},
		{
			Path:  "docs/ops.md",
			Title: "Operations Guide",
			Score: 0.88,
			Metadata: &Metadata{
				ID:     "KB-OPS-0001",
				Domain: "ops",
				Tags:   []string{"operations", "devops"},
			},
		},
	}

	mockClient := &mockQMDClient{
		searchResults: mockResults,
	}

	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
	}

	store, _ := NewKBStore(config)

	ctx := context.Background()
	req := kb.SearchQuery{
		Query:    "guide",
		KBScopes: []string{"architecture"},
		Limit:    10,
	}

	results, err := store.Search(ctx, req)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should only return architecture docs due to scope filter
	if len(results) != 1 {
		t.Errorf("Expected 1 result (filtered by scope), got %d", len(results))
	}

	if results[0].Doc.Domain != "architecture" {
		t.Errorf("Expected domain 'architecture', got '%s'", results[0].Doc.Domain)
	}
}

func TestKBStore_Search_WithTags(t *testing.T) {
	mockResults := []SearchResult{
		{
			Path:  "docs/architecture.md",
			Title: "Architecture Overview",
			Score: 0.95,
			Metadata: &Metadata{
				ID:     "KB-ARCH-0001",
				Domain: "architecture",
				Tags:   []string{"architecture", "core"},
			},
		},
		{
			Path:  "docs/deployment.md",
			Title: "Deployment Guide",
			Score: 0.87,
			Metadata: &Metadata{
				ID:     "KB-DEPLOY-0001",
				Domain: "ops",
				Tags:   []string{"deployment"},
			},
		},
	}

	mockClient := &mockQMDClient{
		searchResults: mockResults,
	}

	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
	}

	store, _ := NewKBStore(config)

	ctx := context.Background()
	req := kb.SearchQuery{
		Query: "overview",
		Tags:  []string{"core"},
		Limit: 10,
	}

	results, err := store.Search(ctx, req)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should only return docs with "core" tag
	if len(results) != 1 {
		t.Errorf("Expected 1 result (filtered by tags), got %d", len(results))
	}

	if results[0].Doc.Title != "Architecture Overview" {
		t.Errorf("Expected title 'Architecture Overview', got '%s'", results[0].Doc.Title)
	}
}

func TestKBStore_Search_QMDError(t *testing.T) {
	mockClient := &mockQMDClient{
		searchError: errors.New("qmd error"),
	}

	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
	}

	store, _ := NewKBStore(config)

	ctx := context.Background()
	req := kb.SearchQuery{
		Query: "test",
	}

	_, err := store.Search(ctx, req)

	if err == nil {
		t.Error("Expected error from qmd client")
	}
}

func TestKBStore_Get(t *testing.T) {
	mockResults := []SearchResult{
		{
			Path:  "docs/architecture.md",
			Title: "Architecture Overview",
			Score: 0.95,
			Metadata: &Metadata{
				ID:     "KB-ARCH-0001",
				Domain: "architecture",
			},
		},
	}

	mockClient := &mockQMDClient{
		searchResults: mockResults,
	}

	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
	}

	store, _ := NewKBStore(config)

	ctx := context.Background()
	doc, err := store.Get(ctx, "KB-ARCH-0001")

	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if doc == nil {
		t.Fatal("Doc should not be nil")
	}

	if doc.ID != "KB-ARCH-0001" {
		t.Errorf("Expected ID 'KB-ARCH-0001', got '%s'", doc.ID)
	}

	if doc.Title != "Architecture Overview" {
		t.Errorf("Expected title 'Architecture Overview', got '%s'", doc.Title)
	}
}

func TestKBStore_Get_NotFound(t *testing.T) {
	mockClient := &mockQMDClient{
		searchResults: []SearchResult{}, // Empty results
	}

	config := &KBStoreConfig{
		QMDClient: mockClient,
		RepoPath:  "/path/to/repo",
	}

	store, _ := NewKBStore(config)

	ctx := context.Background()
	_, err := store.Get(ctx, "KB-NONEXISTENT")

	if err == nil {
		t.Error("Expected error for non-existent document")
	}

	if !strings.Contains(err.Error(), "document not found") {
		t.Errorf("Expected 'document not found' error, got: %v", err)
	}
}

func TestKBStore_matchesScopes(t *testing.T) {
	config := &KBStoreConfig{
		QMDClient: &mockQMDClient{},
		RepoPath:  "/path/to/repo",
	}
	store, _ := NewKBStore(config)

	doc := DocumentRef{
		Domain: "architecture",
		Tags:   []string{"architecture", "core"},
	}

	tests := []struct {
		name   string
		scopes []string
		want   bool
	}{
		{
			name:   "matching domain",
			scopes: []string{"architecture"},
			want:   true,
		},
		{
			name:   "matching tag",
			scopes: []string{"core"},
			want:   true,
		},
		{
			name:   "no match",
			scopes: []string{"ops"},
			want:   false,
		},
		{
			name:   "empty scopes",
			scopes: []string{},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := store.matchesScopes(doc, tt.scopes); got != tt.want {
				t.Errorf("matchesScopes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKBStore_matchesTags(t *testing.T) {
	config := &KBStoreConfig{
		QMDClient: &mockQMDClient{},
		RepoPath:  "/path/to/repo",
	}
	store, _ := NewKBStore(config)

	doc := DocumentRef{
		Tags: []string{"architecture", "core"},
	}

	tests := []struct {
		name string
		tags []string
		want bool
	}{
		{
			name: "all tags match",
			tags: []string{"architecture"},
			want: true,
		},
		{
			name: "multiple tags match",
			tags: []string{"architecture", "core"},
			want: true,
		},
		{
			name: "not all tags match",
			tags: []string{"architecture", "ops"},
			want: false,
		},
		{
			name: "no tags match",
			tags: []string{"ops"},
			want: false,
		},
		{
			name: "empty tags",
			tags: []string{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := store.matchesTags(doc, tt.tags); got != tt.want {
				t.Errorf("matchesTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJoinOr(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  string
	}{
		{
			name:  "single item",
			items: []string{"a"},
			want:  "a",
		},
		{
			name:  "multiple items",
			items: []string{"a", "b", "c"},
			want:  "a OR b OR c",
		},
		{
			name:  "empty items",
			items: []string{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinOr(tt.items); got != tt.want {
				t.Errorf("joinOr() = %v, want %v", got, tt.want)
			}
		})
	}
}
