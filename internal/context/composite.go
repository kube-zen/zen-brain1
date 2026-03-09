// Package context provides the composite ZenContext implementation.
// This combines Tier 1 (Redis), Tier 2 (QMD), and Tier 3 (S3) storage
// to provide a unified interface for session context, knowledge retrieval,
// and the ReMe (Recursive Memory) protocol.
package context

import (
	stdctx "context"
	"fmt"
	"strings"
	"sync"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// Journal is an interface for querying journal entries (optional for ReMe protocol).
// This allows the composite to work without Block 1.1 implementation.
type Journal interface {
	Query(ctx stdctx.Context, opts interface{}) ([]interface{}, error)
}

// Composite implements the ZenContext interface by combining
// Tier 1 (Hot), Tier 2 (Warm), and Tier 3 (Cold) storage.
type Composite struct {
	hot  Store  // Tier 1: Redis/tmpfs
	warm Store  // Tier 2: QMD (via adapter)
	cold Store  // Tier 3: S3 archival

	// Optional: Journal for ReMe protocol (Block 1.1)
	journal Journal

	mu sync.RWMutex

	verbose bool
}

// Store is the interface for tier-specific storage.
// This is a subset of the full ZenContext interface.
type Store interface {
	// GetSessionContext retrieves session context.
	GetSessionContext(ctx stdctx.Context, clusterID, sessionID string) (*zenctx.SessionContext, error)

	// StoreSessionContext stores session context.
	StoreSessionContext(ctx stdctx.Context, clusterID string, session *zenctx.SessionContext) error

	// DeleteSessionContext deletes session context.
	DeleteSessionContext(ctx stdctx.Context, clusterID, sessionID string) error

	// QueryKnowledge queries knowledge (Tier 2 only).
	QueryKnowledge(ctx stdctx.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error)

	// StoreKnowledge stores knowledge (Tier 2 only).
	StoreKnowledge(ctx stdctx.Context, chunks []zenctx.KnowledgeChunk) error

	// ArchiveSession archives session (Tier 3 only).
	ArchiveSession(ctx stdctx.Context, clusterID, sessionID string) error

	// Stats returns tier statistics.
	Stats(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error)

	// Close closes the store.
	Close() error
}

// Config holds configuration for the composite ZenContext.
type Config struct {
	// Hot is the Tier 1 (Redis) store.
	Hot Store

	// Warm is the Tier 2 (QMD) store.
	Warm Store

	// Cold is the Tier 3 (S3) store.
	Cold Store

	// Journal is the ZenJournal for ReMe protocol (optional).
	Journal Journal

	// Verbose enables verbose logging.
	Verbose bool
}

// NewComposite creates a new composite ZenContext implementation.
func NewComposite(config *Config) (*Composite, error) {
	if config == nil {
		config = &Config{}
	}

	if config.Hot == nil {
		return nil, fmt.Errorf("Tier 1 (Hot) store is required")
	}

	return &Composite{
		hot:     config.Hot,
		warm:    config.Warm,
		cold:    config.Cold,
		journal: config.Journal,
		verbose: config.Verbose,
	}, nil
}

// GetSessionContext retrieves session context from Tier 1 (Hot).
// Returns nil if session does not exist.
func (c *Composite) GetSessionContext(ctx stdctx.Context, clusterID, sessionID string) (*zenctx.SessionContext, error) {
	return c.hot.GetSessionContext(ctx, clusterID, sessionID)
}

// StoreSessionContext stores session context in Tier 1 (Hot).
func (c *Composite) StoreSessionContext(ctx stdctx.Context, clusterID string, session *zenctx.SessionContext) error {
	return c.hot.StoreSessionContext(ctx, clusterID, session)
}

// DeleteSessionContext deletes session context from all tiers.
func (c *Composite) DeleteSessionContext(ctx stdctx.Context, clusterID, sessionID string) error {
	var errs []error

	// Delete from Tier 1 (Hot)
	if err := c.hot.DeleteSessionContext(ctx, clusterID, sessionID); err != nil {
		errs = append(errs, fmt.Errorf("Tier 1 delete failed: %w", err))
	}

	// Delete from Tier 3 (Cold) - ignore if not found
	if c.cold != nil {
		if err := c.cold.DeleteSessionContext(ctx, clusterID, sessionID); err != nil &&
			!strings.Contains(err.Error(), "not found") {
			errs = append(errs, fmt.Errorf("Tier 3 delete failed: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple delete errors: %v", errs)
	}

	return nil
}

// QueryKnowledge queries Tier 2 (Warm) for relevant knowledge.
func (c *Composite) QueryKnowledge(ctx stdctx.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error) {
	if c.warm == nil {
		return nil, fmt.Errorf("Tier 2 (Warm) store not configured")
	}

	if c.verbose {
		fmt.Printf("[ZenContext] QueryKnowledge: query=%q, scopes=%v, limit=%d\n",
			opts.Query, opts.Scopes, opts.Limit)
	}

	return c.warm.QueryKnowledge(ctx, opts)
}

// StoreKnowledge stores knowledge in Tier 2 (Warm).
func (c *Composite) StoreKnowledge(ctx stdctx.Context, chunks []zenctx.KnowledgeChunk) error {
	if c.warm == nil {
		return fmt.Errorf("Tier 2 (Warm) store not configured")
	}

	if c.verbose {
		fmt.Printf("[ZenContext] StoreKnowledge: %d chunks\n", len(chunks))
	}

	return c.warm.StoreKnowledge(ctx, chunks)
}

// ArchiveSession archives session context to Tier 3 (Cold).
// Retrieves from Tier 1, compresses, and stores in S3.
func (c *Composite) ArchiveSession(ctx stdctx.Context, clusterID, sessionID string) error {
	if c.cold == nil {
		return fmt.Errorf("Tier 3 (Cold) store not configured")
	}

	if c.verbose {
		fmt.Printf("[ZenContext] ArchiveSession: sessionID=%s, clusterID=%s\n", sessionID, clusterID)
	}

	// Retrieve session from Tier 1
	session, err := c.hot.GetSessionContext(ctx, clusterID, sessionID)
	if err != nil {
		return fmt.Errorf("failed to retrieve session from Tier 1: %w", err)
	}

	if session == nil {
		return fmt.Errorf("session not found in Tier 1: %s", sessionID)
	}

	// Store in Tier 3
	if err := c.cold.StoreSessionContext(ctx, clusterID, session); err != nil {
		return fmt.Errorf("failed to archive to Tier 3: %w", err)
	}

	// Optionally delete from Tier 1 after archival
	if err := c.hot.DeleteSessionContext(ctx, clusterID, sessionID); err != nil {
		// Non-fatal: archival succeeded but Tier 1 deletion failed
		if c.verbose {
			fmt.Printf("[ZenContext] Warning: failed to delete from Tier 1: %v\n", err)
		}
	}

	return nil
}

// ReconstructSession implements the ReMe protocol.
// Reconstructs session state from ZenJournal and Tier 2/3.
func (c *Composite) ReconstructSession(ctx stdctx.Context, req zenctx.ReMeRequest) (*zenctx.ReMeResponse, error) {
	startTime := time.Now()

	if c.verbose {
		fmt.Printf("[ZenContext] ReconstructSession: sessionID=%s, taskID=%s, clusterID=%s\n",
			req.SessionID, req.TaskID, req.ClusterID)
	}

	// Step 1: Try to retrieve from Tier 1 (Hot) - fastest path
	sessionCtx, err := c.hot.GetSessionContext(ctx, req.ClusterID, req.SessionID)
	if err == nil && sessionCtx != nil {
		if c.verbose {
			fmt.Printf("[ZenContext] Session found in Tier 1 (Hot) - skipping reconstruction\n")
		}
		return &zenctx.ReMeResponse{
			SessionContext: sessionCtx,
			JournalEntries: []interface{}{},
			ReconstructedAt: time.Now(),
		}, nil
	}

	// Step 2: If not in Tier 1, check Tier 3 (Cold) for archived sessions
	if c.cold != nil {
		sessionCtx, err = c.cold.GetSessionContext(ctx, req.ClusterID, req.SessionID)
		if err == nil && sessionCtx != nil {
			if c.verbose {
				fmt.Printf("[ZenContext] Session found in Tier 3 (Cold) - restored from archive\n")
			}
			// Store in Tier 1 for faster future access
			if err := c.hot.StoreSessionContext(ctx, req.ClusterID, sessionCtx); err != nil && c.verbose {
				fmt.Printf("[ZenContext] Warning: failed to store in Tier 1: %v\n", err)
			}
			return &zenctx.ReMeResponse{
				SessionContext: sessionCtx,
				JournalEntries: []interface{}{},
				ReconstructedAt: time.Now(),
			}, nil
		}
	}

	// Step 3: Create a fresh session (neither Tier 1 nor Tier 3 has it)
	sessionCtx = &zenctx.SessionContext{
		SessionID:     req.SessionID,
		TaskID:        req.TaskID,
		ClusterID:     req.ClusterID,
		ProjectID:     req.ProjectID,
		CreatedAt:     time.Now(),
		LastAccessedAt: time.Now(),
	}

	// Step 4: Query ZenJournal for relevant events (if available)
	var journalEntries []interface{}
	if c.journal != nil && req.TaskID != "" {
		// Simple query for task-related entries
		queryOpts := map[string]interface{}{
			"task_id": req.TaskID,
			"limit":   100,
		}
		entries, err := c.journal.Query(ctx, queryOpts)
		if err == nil {
			journalEntries = entries
			if c.verbose {
				fmt.Printf("[ZenContext] Retrieved %d journal entries for task=%s\n", len(entries), req.TaskID)
			}
		} else if c.verbose {
			fmt.Printf("[ZenContext] Warning: journal query failed: %v\n", err)
		}
	}

	// Step 5: Query Tier 2 (Warm) for relevant knowledge (if configured)
	if c.warm != nil && req.TaskID != "" {
		// Use a simple query based on task ID
		opts := zenctx.QueryOptions{
			Query:   fmt.Sprintf("task: %s", req.TaskID),
			Limit:   5,
			ClusterID: req.ClusterID,
			ProjectID: req.ProjectID,
		}
		kbChunks, err := c.warm.QueryKnowledge(ctx, opts)
		if err == nil {
			sessionCtx.RelevantKnowledge = kbChunks
			if c.verbose {
				fmt.Printf("[ZenContext] Retrieved %d KB chunks for context\n", len(kbChunks))
			}
		} else if c.verbose {
			fmt.Printf("[ZenContext] Warning: KB query failed: %v\n", err)
		}
	}

	// Step 6: Store reconstructed session in Tier 1 for fast access
	if err := c.hot.StoreSessionContext(ctx, req.ClusterID, sessionCtx); err != nil && c.verbose {
		fmt.Printf("[ZenContext] Warning: failed to store in Tier 1: %v\n", err)
	}

	if c.verbose {
		fmt.Printf("[ZenContext] ReconstructSession completed in %v\n", time.Since(startTime))
	}

	return &zenctx.ReMeResponse{
		SessionContext: sessionCtx,
		JournalEntries: journalEntries,
		ReconstructedAt: time.Now(),
	}, nil
}

// Stats returns memory usage statistics for all tiers.
func (c *Composite) Stats(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
	stats := make(map[zenctx.Tier]interface{})

	// Get Tier 1 (Hot) stats
	if hotStats, err := c.hot.Stats(ctx); err == nil {
		for tier, data := range hotStats {
			stats[tier] = data
		}
	}

	// Get Tier 2 (Warm) stats if configured
	if c.warm != nil {
		if warmStats, err := c.warm.Stats(ctx); err == nil {
			for tier, data := range warmStats {
				stats[tier] = data
			}
		}
	}

	// Get Tier 3 (Cold) stats if configured
	if c.cold != nil {
		if coldStats, err := c.cold.Stats(ctx); err == nil {
			for tier, data := range coldStats {
				stats[tier] = data
			}
		}
	}

	return stats, nil
}

// Close closes all tier stores.
func (c *Composite) Close() error {
	var errs []error

	if c.hot != nil {
		if err := c.hot.Close(); err != nil {
			errs = append(errs, fmt.Errorf("Tier 1 close failed: %w", err))
		}
	}

	if c.warm != nil {
		if err := c.warm.Close(); err != nil {
			errs = append(errs, fmt.Errorf("Tier 2 close failed: %w", err))
		}
	}

	if c.cold != nil {
		if err := c.cold.Close(); err != nil {
			errs = append(errs, fmt.Errorf("Tier 3 close failed: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple close errors: %v", errs)
	}

	return nil
}