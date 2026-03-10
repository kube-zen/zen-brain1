package kb

import (
	"context"
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/kb"
)

func TestStubStore_Search(t *testing.T) {
	store := NewStubStore()
	ctx := context.Background()

	// Search should return empty results
	query := kb.SearchQuery{
		Query:    "test query",
		KBScopes: []string{"docs"},
		Limit:    10,
	}

	results, err := store.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if results == nil {
		t.Fatal("Expected non-nil results slice")
	}
	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d items", len(results))
	}
}

func TestStubStore_Get(t *testing.T) {
	store := NewStubStore()
	ctx := context.Background()

	// Get non-existent document should return nil
	doc, err := store.Get(ctx, "non-existent-id")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if doc != nil {
		t.Error("Expected nil for non-existent document")
	}

	// Add a document
	testDoc := kb.DocumentRef{
		ID:      "test-doc-1",
		Title:   "Test Document",
		
	}
	store.AddDocument(testDoc)

	// Get existing document
	doc, err = store.Get(ctx, "test-doc-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if doc == nil {
		t.Fatal("Expected document to exist")
	}
	if doc.ID != "test-doc-1" {
		t.Errorf("Expected ID 'test-doc-1', got '%s'", doc.ID)
	}
	if doc.Title != "Test Document" {
		t.Errorf("Expected Title 'Test Document', got '%s'", doc.Title)
	}
}

func TestStubStore_AddDocument(t *testing.T) {
	store := NewStubStore()

	// Add first document
	doc1 := kb.DocumentRef{
		ID:      "doc-1",
		Title:   "Document 1",
		
	}
	store.AddDocument(doc1)

	// Verify it was added
	retrieved, err := store.Get(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected document to be added")
	}

	// Add second document
	doc2 := kb.DocumentRef{
		ID:      "doc-2",
		Title:   "Document 2",
		
	}
	store.AddDocument(doc2)

	// Verify both exist
	retrieved1, _ := store.Get(context.Background(), "doc-1")
	retrieved2, _ := store.Get(context.Background(), "doc-2")
	if retrieved1 == nil || retrieved2 == nil {
		t.Error("Expected both documents to exist")
	}

	// Overwrite existing document
	doc1Updated := kb.DocumentRef{
		ID:      "doc-1",
		Title:   "Document 1 Updated",
		
	}
	store.AddDocument(doc1Updated)

	retrieved, _ = store.Get(context.Background(), "doc-1")
	if retrieved.Title != "Document 1 Updated" {
		t.Error("Expected document to be updated")
	}
}

func TestStubStore_RemoveDocument(t *testing.T) {
	store := NewStubStore()

	// Add a document
	doc := kb.DocumentRef{
		ID:      "doc-to-remove",
		Title:   "To Remove",
		
	}
	store.AddDocument(doc)

	// Verify it exists
	retrieved, _ := store.Get(context.Background(), "doc-to-remove")
	if retrieved == nil {
		t.Fatal("Expected document to exist")
	}

	// Remove it
	store.RemoveDocument("doc-to-remove")

	// Verify it's gone
	retrieved, _ = store.Get(context.Background(), "doc-to-remove")
	if retrieved != nil {
		t.Error("Expected document to be removed")
	}

	// Removing non-existent document should not error
	store.RemoveDocument("non-existent")
}

func TestStubStore_Clear(t *testing.T) {
	store := NewStubStore()

	// Add multiple documents
	store.AddDocument(kb.DocumentRef{ID: "doc-1", Title: "Doc 1"})
	store.AddDocument(kb.DocumentRef{ID: "doc-2", Title: "Doc 2"})
	store.AddDocument(kb.DocumentRef{ID: "doc-3", Title: "Doc 3"})

	// Verify they exist
	doc1, _ := store.Get(context.Background(), "doc-1")
	doc2, _ := store.Get(context.Background(), "doc-2")
	doc3, _ := store.Get(context.Background(), "doc-3")
	if doc1 == nil || doc2 == nil || doc3 == nil {
		t.Fatal("Expected all documents to exist")
	}

	// Clear all
	store.Clear()

	// Verify all gone
	doc1, _ = store.Get(context.Background(), "doc-1")
	doc2, _ = store.Get(context.Background(), "doc-2")
	doc3, _ = store.Get(context.Background(), "doc-3")
	if doc1 != nil || doc2 != nil || doc3 != nil {
		t.Error("Expected all documents to be cleared")
	}
}

func TestStubStore_Interface(t *testing.T) {
	// Verify StubStore implements kb.Store interface
	var _ kb.Store = NewStubStore()
}
