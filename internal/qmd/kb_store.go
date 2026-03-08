// Package qmd provides a knowledge base store implementation backed by qmd.
// This package bridges the qmd adapter and the kb.Store interface.
package qmd

import (
	"context"
	"fmt"
	"log"

	"github.com/kube-zen/zen-brain1/pkg/kb"
	qmdpkg "github.com/kube-zen/zen-brain1/pkg/qmd"
)

// KBStore implements kb.Store using qmd as the backend.
type KBStore struct {
	qmdClient qmdpkg.Client
	repoPath  string
	verbose   bool
}

// Config holds configuration for the KB store.
type KBStoreConfig struct {
	// QMDClient is the qmd client to use for search
	QMDClient qmdpkg.Client
	
	// RepoPath is the path to the zen-docs repository
	RepoPath string
	
	// Verbose enables verbose logging
	Verbose bool
}

// NewKBStore creates a new knowledge base store backed by qmd.
func NewKBStore(config *KBStoreConfig) (*KBStore, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	
	if config.QMDClient == nil {
		return nil, fmt.Errorf("qmd client is required")
	}
	
	if config.RepoPath == "" {
		return nil, fmt.Errorf("repo_path is required")
	}
	
	store := &KBStore{
		qmdClient: config.QMDClient,
		repoPath:  config.RepoPath,
		verbose:   config.Verbose,
	}
	
	return store, nil
}

// Search searches the knowledge base with the given query.
func (s *KBStore) Search(ctx context.Context, q kb.SearchQuery) ([]kb.SearchResult, error) {
	if s.verbose {
		log.Printf("[KBStore] Searching: query=%q, scopes=%v, tags=%v, limit=%d",
			q.Query, q.KBScopes, q.Tags, q.Limit)
	}
	
	// Build enhanced query with scopes and tags
	enhancedQuery := s.buildEnhancedQuery(q)
	
	// Call qmd search
	req := qmdpkg.SearchRequest{
		RepoPath: s.repoPath,
		Query:    enhancedQuery,
		Limit:    q.Limit,
		JSON:     true,
	}
	
	jsonOutput, err := s.qmdClient.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("qmd search failed: %w", err)
	}
	
	// Parse qmd results
	qmdResults, err := ParseSearchResults(jsonOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse qmd results: %w", err)
	}
	
	// Convert to kb.SearchResult format
	results := make([]kb.SearchResult, 0, len(qmdResults))
	for _, qr := range qmdResults {
		kbResult, err := qr.ToKBSearchResult()
		if err != nil {
			log.Printf("[KBStore] Warning: failed to convert result: %v", err)
			continue
		}
		
		// Filter by scopes if specified
		if len(q.KBScopes) > 0 && !s.matchesScopes(kbResult.Doc, q.KBScopes) {
			continue
		}
		
		// Filter by tags if specified
		if len(q.Tags) > 0 && !s.matchesTags(kbResult.Doc, q.Tags) {
			continue
		}
		
		results = append(results, kb.SearchResult{
			Doc:     s.convertDocRef(kbResult.Doc),
			Snippet: kbResult.Snippet,
			Score:   kbResult.Score,
		})
	}
	
	if s.verbose {
		log.Printf("[KBStore] Found %d results", len(results))
	}
	
	return results, nil
}

// Get retrieves a single document by ID.
func (s *KBStore) Get(ctx context.Context, id string) (*kb.DocumentRef, error) {
	if s.verbose {
		log.Printf("[KBStore] Getting document: id=%q", id)
	}
	
	// qmd doesn't have a direct "get by ID" command
	// We can search for the ID in the query and pick the best match
	req := kb.SearchQuery{
		Query:    id,
		Limit:    1,
	}
	
	results, err := s.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search for document: %w", err)
	}
	
	if len(results) == 0 {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	
	return &results[0].Doc, nil
}

// buildEnhancedQuery builds an enhanced search query that includes scopes and tags.
func (s *KBStore) buildEnhancedQuery(q kb.SearchQuery) string {
	query := q.Query
	
	// Add scope filters to query
	if len(q.KBScopes) > 0 {
		scopeText := fmt.Sprintf(" (scope: %s)", joinOr(q.KBScopes))
		query += scopeText
	}
	
	// Add tag filters to query
	if len(q.Tags) > 0 {
		tagText := fmt.Sprintf(" (tags: %s)", joinOr(q.Tags))
		query += tagText
	}
	
	return query
}

// matchesScopes checks if a document matches the given scopes.
func (s *KBStore) matchesScopes(doc DocumentRef, scopes []string) bool {
	if len(scopes) == 0 {
		return true
	}
	
	// Check if document has a scope that matches
	for _, scope := range scopes {
		if doc.Domain == scope {
			return true
		}
		
		// Check tags for scope matches
		for _, tag := range doc.Tags {
			if tag == scope {
				return true
			}
		}
	}
	
	return false
}

// matchesTags checks if a document matches the given tags.
func (s *KBStore) matchesTags(doc DocumentRef, tags []string) bool {
	if len(tags) == 0 {
		return true
	}
	
	// Check if document has all required tags
	tagMap := make(map[string]bool)
	for _, tag := range doc.Tags {
		tagMap[tag] = true
	}
	
	for _, requiredTag := range tags {
		if !tagMap[requiredTag] {
			return false
		}
	}
	
	return true
}

// convertDocRef converts a DocumentRef to kb.DocumentRef.
func (s *KBStore) convertDocRef(doc DocumentRef) kb.DocumentRef {
	return kb.DocumentRef{
		ID:     doc.ID,
		Path:   doc.Path,
		Title:  doc.Title,
		Domain: doc.Domain,
		Tags:   doc.Tags,
		Source: doc.Source,
	}
}

// joinOr joins strings with " OR " for query building.
func joinOr(items []string) string {
	if len(items) == 0 {
		return ""
	}
	
	result := items[0]
	for i := 1; i < len(items); i++ {
		result += " OR " + items[i]
	}
	
	return result
}

// ensure KBStore implements kb.Store interface
var _ kb.Store = (*KBStore)(nil)