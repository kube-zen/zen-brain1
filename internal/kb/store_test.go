package kb

import (
	"context"
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/kb"
)

func TestStubStore_Search(t *testing.T) {
	store := NewStubStore()
	ctx := context.Background()

	query := kb.SearchQuery{
		Query:    "test query",
		KBScopes: []string{"docs"},
		Limit:    10,
	}

	results, err := store.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if results != nil {
		t.Errorf("Expected nil results from stub, got %d items", len(results))
	}
}

func TestStubStore_Get(t *testing.T) {
	store := NewStubStore()
	ctx := context.Background()

	doc, err := store.Get(ctx, "non-existent-id")
	if err == nil {
		t.Fatal("Expected error from stub Get")
	}
	if doc != nil {
		t.Error("Expected nil document from stub Get")
	}
}

func TestStubStore_Interface(t *testing.T) {
	// Verify StubStore implements kb.Store interface
	var _ kb.Store = NewStubStore()
}
