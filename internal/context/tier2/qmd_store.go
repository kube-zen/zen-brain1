// Package tier2 provides Tier 2 (Warm) storage implementation using QMD.
// This implements the Store interface for knowledge retrieval and storage.
package tier2

import (
	stdctx "context"
	"fmt"
	"log"
	"sync"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/kb"
	qmdstore "github.com/kube-zen/zen-brain1/internal/qmd"
)

// QMDStore implements the Store interface for Tier 2 (Warm) storage.
// This wraps the qmd KB store for knowledge retrieval and storage.
type QMDStore struct {
	kbStore kb.Store
	verbose bool
	mu      sync.RWMutex
	
	// Statistics
	stats struct {
		queries      int64
		storedChunks int64
		lastQuery    time.Time
		lastStore    time.Time
	}
}

// Config holds configuration for the QMD store.
type Config struct {
	// KBStore is the knowledge base store to use
	KBStore kb.Store
	
	// Verbose enables verbose logging
	Verbose bool
}

// NewQMDStore creates a new Tier 2 store backed by QMD.
func NewQMDStore(config *Config) (*QMDStore, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	
	if config.KBStore == nil {
		return nil, fmt.Errorf("kb store is required")
	}
	
	store := &QMDStore{
		kbStore: config.KBStore,
		verbose: config.Verbose,
	}
	
	return store, nil
}

// GetSessionContext is not supported for Tier 2 (knowledge store).
// Returns nil, nil to indicate session not found (not an error).
func (s *QMDStore) GetSessionContext(ctx stdctx.Context, clusterID, sessionID string) (*zenctx.SessionContext, error) {
	if s.verbose {
		log.Printf("[QMDStore] GetSessionContext called (not supported for Tier 2): cluster=%s, session=%s",
			clusterID, sessionID)
	}
	return nil, nil
}

// StoreSessionContext is not supported for Tier 2 (knowledge store).
// Returns an error indicating this operation is not supported.
func (s *QMDStore) StoreSessionContext(ctx stdctx.Context, clusterID string, session *zenctx.SessionContext) error {
	return fmt.Errorf("StoreSessionContext not supported for Tier 2 (knowledge store)")
}

// DeleteSessionContext is not supported for Tier 2 (knowledge store).
// Returns nil to indicate successful no-op.
func (s *QMDStore) DeleteSessionContext(ctx stdctx.Context, clusterID, sessionID string) error {
	if s.verbose {
		log.Printf("[QMDStore] DeleteSessionContext called (no-op for Tier 2): cluster=%s, session=%s",
			clusterID, sessionID)
	}
	return nil
}

// QueryKnowledge queries the knowledge base for relevant information.
func (s *QMDStore) QueryKnowledge(ctx stdctx.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error) {
	if s.verbose {
		log.Printf("[QMDStore] QueryKnowledge: query=%q, scopes=%v, limit=%d, minSimilarity=%v",
			opts.Query, opts.Scopes, opts.Limit, opts.MinSimilarity)
	}
	
	// Build kb.SearchQuery from zenctx.QueryOptions
	searchQuery := kb.SearchQuery{
		Query:   opts.Query,
		Limit:   opts.Limit,
		KBScopes: opts.Scopes,
		// Note: kb.SearchQuery doesn't have MinSimilarity field
		// We'll filter results after retrieval if needed
	}
	
	// Execute search
	kbResults, err := s.kbStore.Search(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("knowledge search failed: %w", err)
	}
	
	// Convert kb.SearchResult to zenctx.KnowledgeChunk
	chunks := make([]zenctx.KnowledgeChunk, 0, len(kbResults))
	for _, result := range kbResults {
		chunk := s.convertToKnowledgeChunk(result, opts)
		chunks = append(chunks, chunk)
	}
	
	// Apply minSimilarity filter if specified
	if opts.MinSimilarity > 0 {
		filtered := make([]zenctx.KnowledgeChunk, 0, len(chunks))
		for _, chunk := range chunks {
			if chunk.SimilarityScore >= opts.MinSimilarity {
				filtered = append(filtered, chunk)
			}
		}
		chunks = filtered
	}
	
	// Update statistics
	s.mu.Lock()
	s.stats.queries++
	s.stats.lastQuery = time.Now()
	s.mu.Unlock()
	
	if s.verbose {
		log.Printf("[QMDStore] QueryKnowledge: returned %d chunks (after filtering: %d)",
			len(kbResults), len(chunks))
	}
	
	return chunks, nil
}

// convertToKnowledgeChunk converts a kb.SearchResult to a KnowledgeChunk.
func (s *QMDStore) convertToKnowledgeChunk(result kb.SearchResult, opts zenctx.QueryOptions) zenctx.KnowledgeChunk {
	// Extract scope from document domain if available
	scope := "general"
	if result.Doc.Domain != "" {
		scope = result.Doc.Domain
	}
	
	// Generate a unique ID using document path and timestamp
	id := fmt.Sprintf("kb-%s-%d", result.Doc.Path, time.Now().UnixNano())
	
	// Extract heading path from document title
	var headingPath []string
	if result.Doc.Title != "" {
		headingPath = []string{result.Doc.Title}
	}
	
	// Extract similarity score if available (normalize to 0-1)
	similarityScore := 0.7 // default reasonable score
	if result.Score > 0 {
		// Assuming score is already in 0-1 range
		similarityScore = result.Score
		if similarityScore > 1.0 {
			similarityScore = 1.0
		}
	}
	
	return zenctx.KnowledgeChunk{
		ID:              id,
		Scope:           scope,
		Content:         result.Snippet,
		SourcePath:      result.Doc.Path,
		HeadingPath:     headingPath,
		SimilarityScore: similarityScore,
		RetrievedAt:     time.Now(),
	}
}

// StoreKnowledge stores knowledge chunks in the knowledge base.
// Note: This is a simplified implementation - in production, this would
// trigger a full KB refresh or incremental update.
func (s *QMDStore) StoreKnowledge(ctx stdctx.Context, chunks []zenctx.KnowledgeChunk) error {
	if s.verbose {
		log.Printf("[QMDStore] StoreKnowledge: storing %d chunks", len(chunks))
	}
	
	// For now, we just acknowledge the storage request.
	// In a full implementation, this would:
	// 1. Convert chunks to appropriate format
	// 2. Trigger qmd refresh with the new content
	// 3. Update the vector embeddings
	
	// Update statistics
	s.mu.Lock()
	s.stats.storedChunks += int64(len(chunks))
	s.stats.lastStore = time.Now()
	s.mu.Unlock()
	
	if s.verbose {
		log.Printf("[QMDStore] StoreKnowledge: acknowledged storage of %d chunks", len(chunks))
	}
	
	return nil
}

// ArchiveSession is not supported for Tier 2 (knowledge store).
// Returns an error indicating this operation is not supported.
func (s *QMDStore) ArchiveSession(ctx stdctx.Context, clusterID, sessionID string) error {
	return fmt.Errorf("ArchiveSession not supported for Tier 2 (knowledge store)")
}

// Stats returns statistics about the knowledge store.
func (s *QMDStore) Stats(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stats := map[zenctx.Tier]interface{}{
		zenctx.TierWarm: map[string]interface{}{
			"type":           "qmd",
			"queries_total":  s.stats.queries,
			"chunks_stored":  s.stats.storedChunks,
			"last_query":     s.stats.lastQuery.Format(time.RFC3339),
			"last_store":     s.stats.lastStore.Format(time.RFC3339),
			"verbose":        s.verbose,
		},
	}
	
	return stats, nil
}

// Close closes the knowledge store.
func (s *QMDStore) Close() error {
	if s.verbose {
		log.Printf("[QMDStore] Closing")
	}
	
	// Nothing to close for the KB store wrapper
	return nil
}

// Helper function to create a QMDStore from qmd client configuration.
// This simplifies integration with existing qmd setup.
func NewQMDStoreFromConfig(qmdConfig *qmdstore.Config, repoPath string, verbose bool) (*QMDStore, error) {
	// Create qmd client
	qmdClient, err := qmdstore.NewClient(qmdConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create qmd client: %w", err)
	}
	
	// Create KB store
	kbConfig := &qmdstore.KBStoreConfig{
		QMDClient: qmdClient,
		RepoPath:  repoPath,
		Verbose:   verbose,
	}
	
	kbStore, err := qmdstore.NewKBStore(kbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KB store: %w", err)
	}
	
	// Create QMD store wrapper
	config := &Config{
		KBStore: kbStore,
		Verbose: verbose,
	}
	
	return NewQMDStore(config)
}