// Package kb provides knowledge base store implementations.
package kb

import (
	"context"
	"log"

	"github.com/kube-zen/zen-brain1/pkg/kb"
)

// StubStore is a stub implementation of kb.Store that returns empty results.
// Used for development before qmd integration is complete.
type StubStore struct {
	// Can be extended with in-memory documents for testing
	documents map[string]kb.DocumentRef
}

// NewStubStore creates a new StubStore.
func NewStubStore() *StubStore {
	return &StubStore{
		documents: make(map[string]kb.DocumentRef),
	}
}

// Search always returns empty results.
func (s *StubStore) Search(ctx context.Context, q kb.SearchQuery) ([]kb.SearchResult, error) {
	log.Printf("[StubKB] Search called: query=%q, scopes=%v, limit=%d", q.Query, q.KBScopes, q.Limit)
	
	// Return empty slice; analyzer will work without KB results
	return []kb.SearchResult{}, nil
}

// Get returns a document by ID (if it exists in the stub).
func (s *StubStore) Get(ctx context.Context, id string) (*kb.DocumentRef, error) {
	log.Printf("[StubKB] Get called: id=%s", id)
	
	if doc, ok := s.documents[id]; ok {
		return &doc, nil
	}
	
	return nil, nil // Not found; return nil, nil (consistent with interface)
}

// AddDocument adds a document to the stub store for testing.
func (s *StubStore) AddDocument(doc kb.DocumentRef) {
	s.documents[doc.ID] = doc
}

// RemoveDocument removes a document from the stub store.
func (s *StubStore) RemoveDocument(id string) {
	delete(s.documents, id)
}

// Clear removes all documents from the stub store.
func (s *StubStore) Clear() {
	s.documents = make(map[string]kb.DocumentRef)
}

// Ensure StubStore implements kb.Store
var _ kb.Store = (*StubStore)(nil)