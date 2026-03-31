// Package kb provides knowledge base store implementations.
// Implements V6 Block 3.5 (Real KB Store) with CockroachDB.
package kb

import (
	"context"
	"fmt"

	"github.com/kube-zen/zen-brain1/pkg/kb"
)

// StubStore is a no-op KB store used when CockroachDB is not available.
type StubStore struct{}

// NewStubStore returns a no-op KB store.
func NewStubStore() kb.Store {
	return &StubStore{}
}

// Search returns empty results (stub).
func (s *StubStore) Search(ctx context.Context, q kb.SearchQuery) ([]kb.SearchResult, error) {
	return nil, nil
}

// Get returns nil (stub).
func (s *StubStore) Get(ctx context.Context, id string) (*kb.DocumentRef, error) {
	return nil, fmt.Errorf("kb store: not available in stub mode")
}
