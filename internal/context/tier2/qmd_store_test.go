package tier2

import (
	stdctx "context"
	"fmt"
	"testing"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/kb"
)

// mockKBStore is a mock implementation of kb.Store for testing.
type mockKBStore struct {
	searches []kb.SearchQuery
	results  []kb.SearchResult
}

func newMockKBStore(results []kb.SearchResult) *mockKBStore {
	return &mockKBStore{
		results: results,
	}
}

func (m *mockKBStore) Search(ctx stdctx.Context, q kb.SearchQuery) ([]kb.SearchResult, error) {
	m.searches = append(m.searches, q)
	return m.results, nil
}

func (m *mockKBStore) Get(ctx stdctx.Context, id string) (*kb.DocumentRef, error) {
	// Simple implementation for testing
	for _, result := range m.results {
		if result.Doc.ID == id {
			return &result.Doc, nil
		}
	}
	return nil, fmt.Errorf("document not found: %s", id)
}

func TestNewQMDStore(t *testing.T) {
	kbStore := newMockKBStore(nil)
	config := &Config{
		KBStore: kbStore,
		Verbose: true,
	}
	
	store, err := NewQMDStore(config)
	if err != nil {
		t.Fatalf("NewQMDStore failed: %v", err)
	}
	
	if store == nil {
		t.Fatal("NewQMDStore returned nil")
	}
	
	store.Close()
}

func TestQMDStore_GetSessionContext(t *testing.T) {
	kbStore := newMockKBStore(nil)
	config := &Config{
		KBStore: kbStore,
	}
	store, err := NewQMDStore(config)
	if err != nil {
		t.Fatalf("NewQMDStore failed: %v", err)
	}
	defer store.Close()
	
	ctx := stdctx.Background()
	
	// GetSessionContext should return nil, nil (not an error)
	session, err := store.GetSessionContext(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetSessionContext should not return error: %v", err)
	}
	
	if session != nil {
		t.Error("GetSessionContext should return nil for Tier 2")
	}
}

func TestQMDStore_StoreSessionContext(t *testing.T) {
	kbStore := newMockKBStore(nil)
	config := &Config{
		KBStore: kbStore,
	}
	store, err := NewQMDStore(config)
	if err != nil {
		t.Fatalf("NewQMDStore failed: %v", err)
	}
	defer store.Close()
	
	ctx := stdctx.Background()
	session := &zenctx.SessionContext{
		SessionID: "session-123",
		TaskID:    "task-456",
	}
	
	// StoreSessionContext should return an error (not supported)
	err = store.StoreSessionContext(ctx, "cluster-1", session)
	if err == nil {
		t.Error("StoreSessionContext should return error for Tier 2")
	}
	
	if err.Error() != "StoreSessionContext not supported for Tier 2 (knowledge store)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestQMDStore_QueryKnowledge(t *testing.T) {
	// Create mock search results
	mockResults := []kb.SearchResult{
		{
			Doc: kb.DocumentRef{
				ID:     "doc-1",
				Path:   "/docs/architecture.md",
				Title:  "Architecture Overview",
				Domain: "company",
			},
			Snippet:  "The system uses a three-tier memory architecture.",
			Score:    0.85,
		},
		{
			Doc: kb.DocumentRef{
				ID:     "doc-2",
				Path:   "/docs/deployment.md",
				Title:  "Deployment Guide",
				Domain: "general",
			},
			Snippet:  "Deploy using Helm charts with custom values.",
			Score:    0.72,
		},
	}
	
	kbStore := newMockKBStore(mockResults)
	config := &Config{
		KBStore: kbStore,
		Verbose: true,
	}
	store, err := NewQMDStore(config)
	if err != nil {
		t.Fatalf("NewQMDStore failed: %v", err)
	}
	defer store.Close()
	
	ctx := stdctx.Background()
	
	opts := zenctx.QueryOptions{
		Query:        "three tier architecture",
		Scopes:       []string{"company", "general"},
		Limit:        10,
		MinSimilarity: 0.5,
	}
	
	chunks, err := store.QueryKnowledge(ctx, opts)
	if err != nil {
		t.Fatalf("QueryKnowledge failed: %v", err)
	}
	
	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(chunks))
	}
	
	// Verify first chunk
	chunk1 := chunks[0]
	if chunk1.Content != mockResults[0].Snippet {
		t.Errorf("Chunk content mismatch: got %q, want %q",
			chunk1.Content, mockResults[0].Snippet)
	}
	
	if chunk1.SourcePath != mockResults[0].Doc.Path {
		t.Errorf("SourcePath mismatch: got %s, want %s",
			chunk1.SourcePath, mockResults[0].Doc.Path)
	}
	
	// Similarity score should be derived from mock result
	if chunk1.SimilarityScore != 0.85 {
		t.Errorf("SimilarityScore mismatch: got %f, want %f",
			chunk1.SimilarityScore, 0.85)
	}
	
	// Should have recorded the search
	if len(kbStore.searches) == 0 {
		t.Error("Expected search to be recorded")
	}
	
	if kbStore.searches[0].Query != opts.Query {
		t.Errorf("Search query mismatch: got %s, want %s",
			kbStore.searches[0].Query, opts.Query)
	}
}

func TestQMDStore_QueryKnowledge_MinSimilarityFilter(t *testing.T) {
	// Create mock search results with varying scores
	mockResults := []kb.SearchResult{
		{
			Doc: kb.DocumentRef{
				ID:   "doc-high",
				Path: "/docs/arch.md",
			},
			Snippet:  "High score result",
			Score:    0.9,
		},
		{
			Doc: kb.DocumentRef{
				ID:   "doc-low",
				Path: "/docs/low.md",
			},
			Snippet:  "Low score result",
			Score:    0.3,
		},
		{
			Doc: kb.DocumentRef{
				ID:   "doc-medium",
				Path: "/docs/medium.md",
			},
			Snippet:  "Medium score result",
			Score:    0.6,
		},
	}
	
	kbStore := newMockKBStore(mockResults)
	config := &Config{
		KBStore: kbStore,
	}
	store, err := NewQMDStore(config)
	if err != nil {
		t.Fatalf("NewQMDStore failed: %v", err)
	}
	defer store.Close()
	
	ctx := stdctx.Background()
	
	opts := zenctx.QueryOptions{
		Query:        "test query",
		MinSimilarity: 0.5, // Filter out scores < 0.5
	}
	
	chunks, err := store.QueryKnowledge(ctx, opts)
	if err != nil {
		t.Fatalf("QueryKnowledge failed: %v", err)
	}
	
	// Should have 2 chunks (scores 0.9 and 0.6)
	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks after minSimilarity filter, got %d", len(chunks))
	}
	
	// Verify chunks are filtered correctly
	for _, chunk := range chunks {
		if chunk.SimilarityScore < 0.5 {
			t.Errorf("Chunk with score %f should have been filtered out",
				chunk.SimilarityScore)
		}
	}
}

func TestQMDStore_StoreKnowledge(t *testing.T) {
	kbStore := newMockKBStore(nil)
	config := &Config{
		KBStore: kbStore,
		Verbose: true,
	}
	store, err := NewQMDStore(config)
	if err != nil {
		t.Fatalf("NewQMDStore failed: %v", err)
	}
	defer store.Close()
	
	ctx := stdctx.Background()
	
	chunks := []zenctx.KnowledgeChunk{
		{
			ID:              "chunk-1",
			Scope:           "company",
			Content:         "First knowledge chunk",
			SourcePath:      "/docs/chunk1.md",
			HeadingPath:     []string{"Introduction"},
			SimilarityScore: 0.8,
			RetrievedAt:     time.Now(),
		},
		{
			ID:              "chunk-2",
			Scope:           "general",
			Content:         "Second knowledge chunk",
			SourcePath:      "/docs/chunk2.md",
			HeadingPath:     []string{"Architecture", "Overview"},
			SimilarityScore: 0.9,
			RetrievedAt:     time.Now(),
		},
	}
	
	err = store.StoreKnowledge(ctx, chunks)
	if err != nil {
		t.Fatalf("StoreKnowledge failed: %v", err)
	}
	
	// Verify stats were updated
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	
	tierStats, ok := stats[zenctx.TierWarm]
	if !ok {
		t.Fatal("Expected TierWarm stats")
	}
	
	statsMap, ok := tierStats.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map[string]interface{} for stats")
	}
	
	// Should have recorded 2 stored chunks
	if chunksStored, ok := statsMap["chunks_stored"].(int64); !ok || chunksStored != 2 {
		t.Errorf("Expected 2 chunks stored, got %v", statsMap["chunks_stored"])
	}
}

func TestQMDStore_Stats(t *testing.T) {
	kbStore := newMockKBStore(nil)
	config := &Config{
		KBStore: kbStore,
	}
	store, err := NewQMDStore(config)
	if err != nil {
		t.Fatalf("NewQMDStore failed: %v", err)
	}
	defer store.Close()
	
	ctx := stdctx.Background()
	
	// Initial stats
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	
	if _, ok := stats[zenctx.TierWarm]; !ok {
		t.Error("Expected TierWarm in stats")
	}
	
	// Perform a query to update stats
	opts := zenctx.QueryOptions{
		Query: "test query",
	}
	_, err = store.QueryKnowledge(ctx, opts)
	if err != nil {
		t.Fatalf("QueryKnowledge failed: %v", err)
	}
	
	// Stats should show 1 query
	stats2, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed after query: %v", err)
	}
	
	tierStats, ok := stats2[zenctx.TierWarm]
	if !ok {
		t.Fatal("Expected TierWarm stats")
	}
	
	statsMap, ok := tierStats.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map[string]interface{} for stats")
	}
	
	if queries, ok := statsMap["queries_total"].(int64); !ok || queries != 1 {
		t.Errorf("Expected 1 query total, got %v", statsMap["queries_total"])
	}
}

func TestQMDStore_Close(t *testing.T) {
	kbStore := newMockKBStore(nil)
	config := &Config{
		KBStore: kbStore,
		Verbose: true,
	}
	store, err := NewQMDStore(config)
	if err != nil {
		t.Fatalf("NewQMDStore failed: %v", err)
	}
	
	// Close should not panic
	err = store.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	
	// Can call close multiple times without issue
	err = store.Close()
	if err != nil {
		t.Fatalf("Second Close failed: %v", err)
	}
}