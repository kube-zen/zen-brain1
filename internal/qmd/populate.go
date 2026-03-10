// Package qmd provides QMD client adapter, KB store, and population helpers (Block 5.1).
//
// Population: use Populate to refresh the QMD index for a repo/paths. Validate semantic
// search with the golden set via internal/qmd golden-query tests (see testdata/golden_queries.json).
package qmd

import (
	"context"
	"errors"

	"github.com/kube-zen/zen-brain1/pkg/qmd"
)

var ErrRepoPathRequired = errors.New("repo_path is required for QMD population")

// Populate refreshes the QMD search index for the given repository and paths (Block 5.1).
// Sources should be curated (e.g. zen-docs); scope assignment is repository/path-based
// and can be validated via golden-query tests in kb_quality_test.go.
func Populate(ctx context.Context, client qmd.Client, repoPath string, paths []string) error {
	if repoPath == "" {
		return ErrRepoPathRequired
	}
	req := qmd.EmbedRequest{RepoPath: repoPath, Paths: paths}
	return client.RefreshIndex(ctx, req)
}
