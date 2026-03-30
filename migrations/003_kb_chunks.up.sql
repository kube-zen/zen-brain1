-- kb_chunks: Knowledge Base chunk store with vector embeddings (V6 Block 3.5)
-- Stores document chunks with nomic-embed-text (768d) embeddings for semantic search.
-- Uses C-SPANN vector index for fast similarity search.
--
-- Schema per V6 Section 3.2:
-- - PREFIX COLUMNS (automatically partitioned by C-SPANN)
-- - CONTENT columns for chunk data
-- - METADATA for provenance
-- - VECTOR index for similarity search

CREATE TABLE IF NOT EXISTS kb_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- PREFIX COLUMNS (partitioning key for C-SPANN)
    scope STRING NOT NULL,           -- 'company', 'general', 'zen-brain', 'zen-sdk', etc.
    repo STRING NOT NULL,            -- 'zen-docs', 'zen-sdk', 'zen-brain1', etc.

    -- CONTENT
    path STRING NOT NULL,            -- Original file path within repo
    chunk_index INT NOT NULL,        -- Position in file (0-indexed)
    content TEXT NOT NULL,           -- Chunk text content
    embedding VECTOR(768),           -- nomic-embed-text embedding (768 dimensions)

    -- METADATA
    heading_path STRING[],           -- ['Section', 'Subsection'] for markdown
    token_count INT NOT NULL DEFAULT 0,
    file_type STRING NOT NULL,       -- 'markdown', 'go', 'yaml', 'json', etc.
    language STRING,                 -- Programming language (if code)

    -- PROVENANCE
    content_hash STRING NOT NULL,    -- SHA-256 of content (for dedup/change detection)
    git_commit STRING,               -- Git SHA at ingestion time
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Ensure unique chunks per scope/repo/path/index
    UNIQUE (scope, repo, path, chunk_index)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_kb_chunks_scope_repo ON kb_chunks (scope, repo);
CREATE INDEX IF NOT EXISTS idx_kb_chunks_path ON kb_chunks (path);
CREATE INDEX IF NOT EXISTS idx_kb_chunks_content_hash ON kb_chunks (content_hash);
CREATE INDEX IF NOT EXISTS idx_kb_chunks_ingested ON kb_chunks (ingested_at);

-- Vector similarity search index (CockroachDB 23.2+ C-SPANN)
-- Note: Requires CockroachDB 23.2+ with vector support enabled
-- CREATE VECTOR INDEX IF NOT EXISTS idx_kb_chunks_embedding
--     ON kb_chunks (embedding)
--     WITH (num_neighbors = 10, num_lists = 100);

-- For now, use ivfflat index (more widely supported)
-- This will be upgraded to C-SPANN when available
CREATE INVERTED INDEX IF NOT EXISTS idx_kb_chunks_embedding_cosine
    ON kb_chunks (embedding vector_cosine_ops);

-- Full-text search fallback (for hybrid search)
CREATE INVERTED INDEX IF NOT EXISTS idx_kb_chunks_content_fts
    ON kb_chunks (content);

-- Comments for documentation
COMMENT ON TABLE kb_chunks IS 'Knowledge Base chunk store with 768d embeddings (nomic-embed-text)';
COMMENT ON COLUMN kb_chunks.scope IS 'Partitioning scope: company, general, zen-brain, zen-sdk, project-specific';
COMMENT ON COLUMN kb_chunks.embedding IS '768-dimensional embedding from nomic-embed-text via Ollama';
