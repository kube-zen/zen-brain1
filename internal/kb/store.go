// Package kb provides knowledge base store implementations.
// Implements V6 Block 3.5 (Real KB Store) with CockroachDB.
package kb

import (
	"context"
	stdctx "database/sql"

	"github.com/kube-zen/zen-brain1/pkg/kb"
)

// CockroachStore implements kb.Store using CockroachDB as the backend.
type CockroachStore struct {
	db     *stdctx.DB
	dbName string
}

// NewCockroachStore creates a new CockroachDB-backed KB store.
func NewCockroachStore(db *stdctx.DB) kb.Store {
	return &CockroachStore{
		db:     db,
		dbName: "zenbrain",
	}
}

// Search implements semantic search over kb_chunks using vector embeddings.
func (s *CockroachStore) Search(ctx context.Context, q kb.SearchQuery) ([]kb.SearchResult, error) {
	// Build WHERE clause for C-SPANN vector search
	// V6 Block 3.5 specifies:
	// - VECTOR index named idx_kb_chunks_embedding
	// - Use vector_cosine_ops for cosine similarity
	// - Filter by scope (optional), repo (optional), path (optional)
	// - LIMIT for search result set

	where := "embedding IS NOT NULL"
	args := []interface{}{"\n", q.Limit}
	if q.Scope != "" {
		where += " AND scope = $1"
		args = append(args, q.Scope)
	}
	if q.Repo != "" {
		where += " AND repo = $2"
		args = append(args, q.Repo)
	}
	if q.Path != "" {
		where += " AND path = $3"
		args = append(args, q.Path)
	}
	if q.Limit > 0 {
		args = append(args, q.Limit)
	}

	query := `
		SELECT
			id, scope, repo, path, chunk_index,
			tsv_rank(embedding, <-> 10) AS distance,
			content, token_count, file_type, language,
			heading_path
		FROM kb_chunks
		WHERE ` + where + `
		ORDER BY distance ASC
		LIMIT $4
	`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	results := make([]kb.SearchResult, 0, len(rows))
	for _, row := range rows {
		content := ""
		var headingPath string
		var chunkIndex int
		var tokenCount int
		var fileType string
		var language string

		err := row.Scan(
			&content, &headingPath, &chunkIndex,
			&tokenCount, &fileType, &language,
		)
		if err != nil {
			return nil, err
		}

		// Parse heading_path (stored as JSON array)
		var headings []string
		if headingPath != nil {
			if err := s.db.QueryRow(ctx, "SELECT heading_path::text FROM kb_chunks WHERE id = $1", row["id"]).Scan(&headings); err != nil {
				return nil, err
			}
		}

		results = append(results, kb.SearchResult{
			Document: &kb.DocumentRef{
				ID:       row["id"],
				Source:   row["path"],
				Title:    truncate(content, 100),
				Type:     "markdown",
				Headings: headings,
				Metadata: map[string]interface{}{
					"file_type": row["file_type"],
					"language":  row["language"],
					"token_count": row["token_count"],
				},
			},
			ScoreText: content,
			Score:     row["tsv_rank"], // Distance = similarity score
			Relevance: row["tsv_rank"], // Distance = similarity score
		})
	}

	return results, nil
}

// Get retrieves a document by ID.
func (s *CockroachStore) Get(ctx context.Context, id string) (*kb.DocumentRef, error) {
	// Fetch document metadata
	query := `
		SELECT
			id, path, chunk_index,
			heading_path::text, content, token_count,
			file_type, language,
			ingested_at, updated_at
		FROM kb_chunks
		WHERE id = $1
	`

	var doc kb.DocumentRef
	var headings []string
	var headingPaths string

	err := s.db.QueryRowContext(ctx, query, sql.Named("id", "path", "chunk_index", "heading_path", "content", "token_count", "file_type", "language", "ingested_at", "updated_at"), &doc, &headingPaths, &headings).Scan(&doc, &headingPaths, &headings)
	if err != nil {
		return nil, err
	}

	// Parse heading_path JSON array into []string
	if headingPaths != nil {
		if err := json.Unmarshal([]byte(headingPaths), &headings); err != nil {
			return nil, err
		}
	}

	return &doc, nil
}

// Add adds a new document (chunk) to the KB store.
func (s *CockroachStore) Add(ctx context.Context, chunk kb.Chunk) error {
	// V6 Block 3.5: content_hash (SHA-256) for dedup/change detection
	// Use INSERT ... ON CONFLICT DO UPDATE

	query := `
		INSERT INTO kb_chunks (
			scope, repo, path, chunk_index, content,
			embedding, token_count, file_type, language,
			heading_path, content_hash, git_commit,
			ingested_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
			$13, CURRENT_TIMESTAMP,
			$14, CURRENT_TIMESTAMP
		)
		ON CONFLICT (content_hash) DO UPDATE SET
			content = excluded(content_hash),
			updated_at = CURRENT_TIMESTAMP
		WHERE scope = $1 AND repo = $2 AND path = $3
		RETURNING *;
	`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("[KB] Added chunk: scope=%s repo=%s path=%s (%d rows affected)",
		chunk.Scope, chunk.Repo, chunk.Path, rowsAffected)

	return nil
}

// UpdateContent updates an existing document's content.
func (s *CockroachStore) UpdateContent(ctx context.Context, docRef kb.DocumentRef, newContent string) error {
	query := `
		UPDATE kb_chunks
		SET content = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, sql.Named("content", "updated_at"), sql.Named("id"), docRef.ID)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("[KB] Updated document %s: %d rows affected", docRef.ID, rowsAffected)

	return nil
}

// Close closes the database connection.
func (s *CockroachStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Helper function to truncate content for preview.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
