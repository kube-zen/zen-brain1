package qmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/qmd"
)

// ValidateConfig checks that repo path and binary path are present and valid.
func ValidateConfig(repoPath, binaryPath string) error {
	if repoPath == "" {
		return errors.New("repo path is required")
	}
	if fi, err := os.Stat(repoPath); err != nil {
		if os.IsNotExist(err) {
			return errors.New("repo path does not exist")
		}
		return err
	} else if !fi.IsDir() {
		return errors.New("repo path is not a directory")
	}
	if binaryPath != "" {
		if _, err := exec.LookPath(binaryPath); err != nil {
			return errors.New("qmd binary not found in PATH")
		}
	}
	return nil
}

// Ping performs a lightweight check that QMD is available (binary exists and can be invoked).
// Uses a short timeout. If client is nil or mock, returns nil.
func Ping(ctx context.Context, client qmd.Client, repoPath string) error {
	if client == nil || repoPath == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Lightweight search with limit 1 to verify connectivity
	req := qmd.SearchRequest{RepoPath: repoPath, Query: "health", Limit: 1}
	_, err := client.Search(ctx, req)
	return err
}
