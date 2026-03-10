// Package qmd provides the interface for qmd-based document search and indexing.
// qmd (Query Markdown) is used to index and search the `zen-docs` repository.
//
// In Zen-Brain 1.0, qmd is used as a CLI tool with JSON output; no MCP integration
// is required. The primary flow is:
//  1. qmd indexes the `zen-docs` repository.
//  2. Agents query qmd via CLI with `--json` flag.
//  3. Results are parsed and used for planning/execution.
//
// This interface abstracts the qmd interaction, allowing later replacement
// with a direct Go binding or a different search backend.
package qmd

import (
	"context"
)

// EmbedRequest defines a request to (re)generate embeddings for a repository.
type EmbedRequest struct {
	RepoPath string   `json:"repo_path"`
	Paths    []string `json:"paths,omitempty"`
}

// SearchRequest defines a search request.
type SearchRequest struct {
	RepoPath string `json:"repo_path"`
	Query    string `json:"query"`
	Limit    int    `json:"limit,omitempty"`
	JSON     bool   `json:"json,omitempty"`
}

// Client is the interface for interacting with qmd.
type Client interface {
	// RefreshIndex updates the search index for the given repository/paths.
	// This may be a long‑running operation.
	RefreshIndex(ctx context.Context, req EmbedRequest) error

	// Search performs a search and returns raw JSON output (if JSON=true)
	// or plain text (if JSON=false). The caller is responsible for parsing.
	Search(ctx context.Context, req SearchRequest) ([]byte, error)
}
