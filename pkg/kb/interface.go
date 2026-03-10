// Package kb provides the knowledge base interface for document retrieval.
// The KB is a searchable collection of documents that serve as reference
// material for planning and execution.
//
// In Zen-Brain 1.0:
//   - Source of truth: `zen-docs` Git repository
//   - Search/index: qmd over `zen-docs`
//   - Human publishing: one-way sync to Confluence
//   - Jira is the human entry point and navigation hub
//
// This package defines the abstract interface; implementations may wrap
// qmd CLI, direct git access, or other search backends.
package kb

import (
	"context"
)

// DocumentRef references a single document in the knowledge base.
type DocumentRef struct {
	ID     string   `json:"id"`
	Path   string   `json:"path"`
	Title  string   `json:"title"`
	Domain string   `json:"domain,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Source string   `json:"source"` // "git", "confluence", "internal"
}

// SearchQuery represents a query to the knowledge base.
type SearchQuery struct {
	Query    string   `json:"query"`
	KBScopes []string `json:"kb_scopes,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	Doc     DocumentRef `json:"doc"`
	Snippet string      `json:"snippet,omitempty"`
	Score   float64     `json:"score,omitempty"`
}

// Store is the interface for knowledge base search and retrieval.
type Store interface {
	// Search searches the knowledge base with the given query.
	Search(ctx context.Context, q SearchQuery) ([]SearchResult, error)

	// Get retrieves a single document by ID.
	Get(ctx context.Context, id string) (*DocumentRef, error)
}
