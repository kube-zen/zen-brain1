// Package qmd provides the implementation of the qmd client adapter.
// This adapter wraps the qmd CLI tool, executing it as a subprocess
// and parsing its JSON output for integration with zen-brain.
package qmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/qmd"
)

const (
	// DefaultTimeout is the default timeout for qmd commands
	DefaultTimeout = 30 * time.Second
	
	// MaxSearchResults is the default maximum number of search results
	MaxSearchResults = 10
)

// Client implements the qmd.Client interface by wrapping the qmd CLI tool.
type Client struct {
	qmdPath    string
	timeout    time.Duration
	verbose    bool
}

// Config holds configuration for the qmd client.
type Config struct {
	// QMDPath is the path to the qmd CLI binary
	QMDPath string

	// Timeout is the maximum duration to wait for qmd commands
	Timeout time.Duration

	// Verbose enables verbose logging
	Verbose bool

	// SkipAvailabilityCheck skips the qmd availability check on initialization.
	// This is useful for testing when qmd is not installed.
	SkipAvailabilityCheck bool
}

// DefaultConfig returns a default configuration for the qmd client.
func DefaultConfig() *Config {
	return &Config{
		QMDPath: "qmd", // assumes qmd is in PATH
		Timeout: DefaultTimeout,
		Verbose: false,
	}
}

// NewClient creates a new qmd client with the given configuration.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	client := &Client{
		qmdPath: config.QMDPath,
		timeout: config.Timeout,
		verbose: config.Verbose,
	}

	// Verify qmd is available (unless skipped)
	if !config.SkipAvailabilityCheck {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.checkQmdAvailable(ctx); err != nil {
			return nil, fmt.Errorf("qmd not available: %w", err)
		}
	}

	return client, nil
}

// checkQmdAvailable verifies that the qmd CLI tool is installed and accessible.
func (c *Client) checkQmdAvailable(ctx context.Context) error {
	// Try to run qmd --version or similar
	cmd := exec.CommandContext(ctx, c.qmdPath, "--version")
	
	if c.verbose {
		log.Printf("[QMD] Checking availability: %s --version", c.qmdPath)
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// qmd might not have --version, try a different approach
		// Try running qmd without arguments to see if it's a valid command
		cmd = exec.CommandContext(ctx, c.qmdPath)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("qmd command failed: %w, output: %s", err, string(output))
		}
	}
	
	if c.verbose {
		log.Printf("[QMD] qmd is available: %s", strings.TrimSpace(string(output)))
	}
	
	return nil
}

// RefreshIndex updates the search index for the given repository/paths.
func (c *Client) RefreshIndex(ctx context.Context, req qmd.EmbedRequest) error {
	if req.RepoPath == "" {
		return fmt.Errorf("repo_path is required")
	}
	
	// Set default paths if not specified
	paths := req.Paths
	if len(paths) == 0 {
		paths = []string{"docs/"}
	}
	
	// Build command: qmd embed --repo <path> --paths <paths> [--verbose]
	args := []string{
		"embed",
		"--repo", req.RepoPath,
		"--paths", strings.Join(paths, ","),
	}
	
	if c.verbose {
		args = append(args, "--verbose")
	}
	
	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, c.qmdPath, args...)
	
	if c.verbose {
		log.Printf("[QMD] Running: %s %s", c.qmdPath, strings.Join(args, " "))
	}
	
	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(startTime)
	
	if c.verbose {
		log.Printf("[QMD] Refresh completed in %v, output: %s", elapsed, string(output))
	}
	
	if err != nil {
		return fmt.Errorf("qmd refresh failed: %w, output: %s", err, string(output))
	}
	
	return nil
}

// Search performs a search and returns raw JSON output.
func (c *Client) Search(ctx context.Context, req qmd.SearchRequest) ([]byte, error) {
	if req.RepoPath == "" {
		return nil, fmt.Errorf("repo_path is required")
	}
	
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	
	// Set default limit if not specified
	limit := req.Limit
	if limit <= 0 {
		limit = MaxSearchResults
	}
	
	// Build command: qmd search --repo <path> --query <query> --json [--limit N] [--verbose]
	args := []string{
		"search",
		"--repo", req.RepoPath,
		"--query", req.Query,
		"--json",
		"--limit", fmt.Sprintf("%d", limit),
	}
	
	if c.verbose {
		args = append(args, "--verbose")
	}
	
	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, c.qmdPath, args...)
	
	if c.verbose {
		log.Printf("[QMD] Running: %s %s", c.qmdPath, strings.Join(args, " "))
	}
	
	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(startTime)
	
	if c.verbose {
		log.Printf("[QMD] Search completed in %v", elapsed)
	}
	
	if err != nil {
		return nil, fmt.Errorf("qmd search failed: %w, output: %s", err, string(output))
	}
	
	return output, nil
}

// ParseSearchResults parses the JSON output from qmd search into a structured format.
func ParseSearchResults(jsonData []byte) ([]SearchResult, error) {
	var results struct {
		Results []SearchResult `json:"results"`
	}
	
	if err := json.Unmarshal(jsonData, &results); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}
	
	return results.Results, nil
}

// SearchResult represents a single search result from qmd.
type SearchResult struct {
	Path     string  `json:"path"`
	Title    string  `json:"title,omitempty"`
	Content  string  `json:"content,omitempty"`
	Score    float64 `json:"score,omitempty"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

// Metadata contains additional metadata about a search result.
type Metadata struct {
	ID       string   `json:"id,omitempty"`
	Domain   string   `json:"domain,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Source   string   `json:"source,omitempty"`
}

// ToKBSearchResult converts a qmd SearchResult to a kb.SearchResult.
func (sr *SearchResult) ToKBSearchResult() (KBSearchResult, error) {
	return KBSearchResult{
		Doc: DocumentRef{
			ID:     sr.Metadata.getID(),
			Path:   sr.Path,
			Title:  sr.getTitle(),
			Domain: sr.Metadata.getDomain(),
			Tags:   sr.Metadata.getTags(),
			Source: "git", // qmd indexes git repos
		},
		Snippet: sr.Content,
		Score:   sr.Score,
	}, nil
}

func (m *Metadata) getID() string {
	if m != nil && m.ID != "" {
		return m.ID
	}
	return ""
}

func (m *Metadata) getDomain() string {
	if m != nil && m.Domain != "" {
		return m.Domain
	}
	return ""
}

func (m *Metadata) getTags() []string {
	if m != nil && len(m.Tags) > 0 {
		return m.Tags
	}
	return nil
}

// getTitle returns the title of the search result, falling back to the path if not set.
func (sr *SearchResult) getTitle() string {
	if sr.Title != "" {
		return sr.Title
	}
	// Extract filename from path as fallback
	parts := strings.Split(sr.Path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return sr.Path
}

// KBSearchResult is the knowledge base search result format.
// This type is defined here to avoid circular imports with pkg/kb.
type KBSearchResult struct {
	Doc     DocumentRef
	Snippet string
	Score   float64
}

// DocumentRef is a lightweight reference to a document.
// This type is defined here to avoid circular imports with pkg/kb.
type DocumentRef struct {
	ID     string
	Path   string
	Title  string
	Domain string
	Tags   []string
	Source string
}

// ParseRefreshOutput parses the output from qmd embed command.
func ParseRefreshOutput(output []byte) (*RefreshResult, error) {
	// qmd output is typically just logging, but try to parse any JSON
	var result RefreshResult

	// Check if output is valid JSON
	if bytes.HasPrefix(output, []byte("{")) || bytes.HasPrefix(output, []byte("[")) {
		if err := json.Unmarshal(output, &result); err != nil {
			// Not valid JSON, use text output as summary
			result.Summary = string(output)
		}
		// If JSON parsing succeeded, the result fields are already populated
	} else {
		// Parse stats from text output (fallback)
		result.Summary = string(output)
	}

	return &result, nil
}

// RefreshResult represents the result of a refresh operation.
type RefreshResult struct {
	Summary     string `json:"summary"`
	FilesIndexed int    `json:"files_indexed"`
	TotalChunks int    `json:"total_chunks"`
	DurationMs  int64  `json:"duration_ms"`
}

// ensure Client implements qmd.Client interface
var _ qmd.Client = (*Client)(nil)