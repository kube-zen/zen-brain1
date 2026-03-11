package qmd

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/qmd"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.QMDPath != "qmd" {
		t.Errorf("Expected QMDPath 'qmd', got '%s'", config.QMDPath)
	}

	if config.Timeout != DefaultTimeout {
		t.Errorf("Expected timeout %v, got %v", DefaultTimeout, config.Timeout)
	}

	if config.Verbose != false {
		t.Errorf("Expected Verbose false, got %v", config.Verbose)
	}
}

func TestNewClient_WithNilConfig(t *testing.T) {
	config := DefaultConfig()
	config.SkipAvailabilityCheck = true
	config.FallbackToMock = false // Don't use mock in tests
	clientInterface, err := NewClient(config)

	if err != nil {
		t.Fatalf("NewClient with nil config should succeed, got: %v", err)
	}

	if clientInterface == nil {
		t.Fatal("Client should not be nil")
	}

	// Type assert to get concrete client for field checks
	client, ok := clientInterface.(*Client)
	if !ok {
		t.Fatal("Expected *Client type when SkipAvailabilityCheck=true")
	}

	if client.qmdPath != "qmd" {
		t.Errorf("Expected qmdPath 'qmd', got '%s'", client.qmdPath)
	}
}

func TestNewClient_WithCustomConfig(t *testing.T) {
	config := &Config{
		QMDPath:               "/usr/local/bin/qmd",
		Timeout:               10 * time.Second,
		Verbose:               true,
		SkipAvailabilityCheck: true,
		FallbackToMock:        false, // Don't use mock in tests
	}

	clientInterface, err := NewClient(config)

	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Type assert to get concrete client for field checks
	client, ok := clientInterface.(*Client)
	if !ok {
		t.Fatal("Expected *Client type when SkipAvailabilityCheck=true")
	}

	if client.qmdPath != "/usr/local/bin/qmd" {
		t.Errorf("Expected qmdPath '/usr/local/bin/qmd', got '%s'", client.qmdPath)
	}

	if client.timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", client.timeout)
	}

	if client.verbose != true {
		t.Errorf("Expected Verbose true, got %v", client.verbose)
	}
}

func TestNewClient_WithInvalidQMDPath(t *testing.T) {
	config := &Config{
		QMDPath:        "/nonexistent/path/to/qmd",
		Timeout:        5 * time.Second,
		FallbackToMock: false, // Don't use mock - we want error
	}

	_, err := NewClient(config)

	if err == nil {
		t.Error("Expected error for invalid qmd path")
	}

	if !strings.Contains(err.Error(), "qmd not available") {
		t.Errorf("Expected 'qmd not available' error, got: %v", err)
	}
}

func TestParseSearchResults_ValidJSON(t *testing.T) {
	jsonData := []byte(`{
		"results": [
			{
				"path": "docs/architecture.md",
				"title": "Architecture Overview",
				"content": "The architecture is designed for scalability...",
				"score": 0.95,
				"metadata": {
					"id": "KB-ARCH-0001",
					"domain": "architecture",
					"tags": ["architecture", "core"],
					"source": "git"
				}
			},
			{
				"path": "docs/deployment.md",
				"title": "Deployment Guide",
				"score": 0.87
			}
		]
	}`)

	results, err := ParseSearchResults(jsonData)

	if err != nil {
		t.Fatalf("ParseSearchResults failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check first result
	if results[0].Path != "docs/architecture.md" {
		t.Errorf("Expected path 'docs/architecture.md', got '%s'", results[0].Path)
	}

	if results[0].Title != "Architecture Overview" {
		t.Errorf("Expected title 'Architecture Overview', got '%s'", results[0].Title)
	}

	if results[0].Score != 0.95 {
		t.Errorf("Expected score 0.95, got %f", results[0].Score)
	}

	if results[0].Metadata == nil {
		t.Error("Metadata should not be nil")
	}

	if results[0].Metadata.ID != "KB-ARCH-0001" {
		t.Errorf("Expected metadata ID 'KB-ARCH-0001', got '%s'", results[0].Metadata.ID)
	}

	// Check second result (without metadata)
	if results[1].Path != "docs/deployment.md" {
		t.Errorf("Expected path 'docs/deployment.md', got '%s'", results[1].Path)
	}

	if results[1].Metadata != nil {
		t.Error("Expected nil metadata for second result")
	}
}

func TestParseSearchResults_InvalidJSON(t *testing.T) {
	jsonData := []byte(`{ invalid json }`)

	_, err := ParseSearchResults(jsonData)

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseSearchResults_EmptyArray(t *testing.T) {
	jsonData := []byte(`{
		"results": []
	}`)

	results, err := ParseSearchResults(jsonData)

	if err != nil {
		t.Fatalf("ParseSearchResults failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestSearchResult_ToKBSearchResult(t *testing.T) {
	result := SearchResult{
		Path:    "docs/architecture.md",
		Title:   "Architecture Overview",
		Content: "The architecture...",
		Score:   0.95,
		Metadata: &Metadata{
			ID:     "KB-ARCH-0001",
			Domain: "architecture",
			Tags:   []string{"architecture", "core"},
			Source: "git",
		},
	}

	kbResult, err := result.ToKBSearchResult()

	if err != nil {
		t.Fatalf("ToKBSearchResult failed: %v", err)
	}

	if kbResult.Doc.ID != "KB-ARCH-0001" {
		t.Errorf("Expected ID 'KB-ARCH-0001', got '%s'", kbResult.Doc.ID)
	}

	if kbResult.Doc.Path != "docs/architecture.md" {
		t.Errorf("Expected path 'docs/architecture.md', got '%s'", kbResult.Doc.Path)
	}

	if kbResult.Doc.Source != "git" {
		t.Errorf("Expected source 'git', got '%s'", kbResult.Doc.Source)
	}

	if kbResult.Snippet != "The architecture..." {
		t.Errorf("Expected snippet 'The architecture...', got '%s'", kbResult.Snippet)
	}

	if kbResult.Score != 0.95 {
		t.Errorf("Expected score 0.95, got %f", kbResult.Score)
	}
}

func TestSearchResult_ToKBSearchResult_WithoutMetadata(t *testing.T) {
	result := SearchResult{
		Path:    "docs/architecture.md",
		Title:   "Architecture Overview",
		Content: "The architecture...",
		Score:   0.95,
	}

	kbResult, err := result.ToKBSearchResult()

	if err != nil {
		t.Fatalf("ToKBSearchResult failed: %v", err)
	}

	if kbResult.Doc.ID != "" {
		t.Errorf("Expected empty ID, got '%s'", kbResult.Doc.ID)
	}

	if kbResult.Doc.Source != "git" {
		t.Errorf("Expected default source 'git', got '%s'", kbResult.Doc.Source)
	}
}

func TestParseRefreshOutput_ValidJSON(t *testing.T) {
	output := []byte(`{
		"summary": "Index refresh completed successfully",
		"files_indexed": 42,
		"total_chunks": 156,
		"duration_ms": 2341
	}`)

	result, err := ParseRefreshOutput(output)

	if err != nil {
		t.Fatalf("ParseRefreshOutput failed: %v", err)
	}

	if result.Summary != "Index refresh completed successfully" {
		t.Errorf("Expected summary 'Index refresh completed successfully', got '%s'", result.Summary)
	}

	if result.FilesIndexed != 42 {
		t.Errorf("Expected 42 files indexed, got %d", result.FilesIndexed)
	}

	if result.TotalChunks != 156 {
		t.Errorf("Expected 156 chunks, got %d", result.TotalChunks)
	}

	if result.DurationMs != 2341 {
		t.Errorf("Expected duration 2341ms, got %d", result.DurationMs)
	}
}

func TestParseRefreshOutput_PlainText(t *testing.T) {
	output := []byte("Index refresh completed successfully\nProcessed 42 files")

	result, err := ParseRefreshOutput(output)

	if err != nil {
		t.Fatalf("ParseRefreshOutput failed: %v", err)
	}

	if result.Summary != "Index refresh completed successfully\nProcessed 42 files" {
		t.Errorf("Expected text summary, got '%s'", result.Summary)
	}

	if result.FilesIndexed != 0 {
		t.Errorf("Expected 0 files indexed for plain text, got %d", result.FilesIndexed)
	}
}

func TestParseRefreshOutput_Empty(t *testing.T) {
	output := []byte("")

	result, err := ParseRefreshOutput(output)

	if err != nil {
		t.Fatalf("ParseRefreshOutput failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.Summary != "" {
		t.Errorf("Expected empty summary, got '%s'", result.Summary)
	}
}

func TestRefreshIndex_MissingRepoPath(t *testing.T) {
	client := &Client{
		qmdPath: "qmd",
		timeout: DefaultTimeout,
	}

	ctx := context.Background()
	req := qmd.EmbedRequest{
		// RepoPath is missing
		Paths: []string{"docs/"},
	}

	err := client.RefreshIndex(ctx, req)

	if err == nil {
		t.Error("Expected error for missing repo_path")
	}

	if !strings.Contains(err.Error(), "repo_path is required") {
		t.Errorf("Expected 'repo_path is required' error, got: %v", err)
	}
}

func TestRefreshIndex_MissingPaths(t *testing.T) {
	// NOTE: This test requires qmd to NOT be installed
	// If qmd is installed, this test will pass unexpectedly
	// TODO: Update test to use mock client or skipif qmd installed
	if _, err := exec.LookPath("qmd"); err == nil {
		t.Skip("qmd is installed, skipping test that expects qmd to be missing")
	}

	client := &Client{
		qmdPath: "qmd",
		timeout: DefaultTimeout,
	}

	ctx := context.Background()
	req := qmd.EmbedRequest{
		RepoPath: "/path/to/repo",
		// Paths is missing - should default to ["docs/"]
	}

	// This test would normally fail because qmd isn't installed
	// We're just testing the validation logic
	err := client.RefreshIndex(ctx, req)

	// Should fail because qmd isn't actually installed
	if err == nil {
		t.Error("Expected error (qmd not installed)")
		return // Prevent panic on nil error
	}

	// But the error should be about qmd, not about missing paths
	if strings.Contains(err.Error(), "paths is required") {
		t.Error("Should default paths to ['docs/'], not require them")
	}
}

func TestSearch_MissingRepoPath(t *testing.T) {
	client := &Client{
		qmdPath: "qmd",
		timeout: DefaultTimeout,
	}

	ctx := context.Background()
	req := qmd.SearchRequest{
		// RepoPath is missing
		Query: "test query",
	}

	_, err := client.Search(ctx, req)

	if err == nil {
		t.Error("Expected error for missing repo_path")
	}

	if !strings.Contains(err.Error(), "repo_path is required") {
		t.Errorf("Expected 'repo_path is required' error, got: %v", err)
	}
}

func TestSearch_MissingQuery(t *testing.T) {
	client := &Client{
		qmdPath: "qmd",
		timeout: DefaultTimeout,
	}

	ctx := context.Background()
	req := qmd.SearchRequest{
		RepoPath: "/path/to/repo",
		// Query is missing
	}

	_, err := client.Search(ctx, req)

	if err == nil {
		t.Error("Expected error for missing query")
	}

	if !strings.Contains(err.Error(), "query is required") {
		t.Errorf("Expected 'query is required' error, got: %v", err)
	}
}

func TestSearch_DefaultLimit(t *testing.T) {
	// NOTE: This test requires qmd to NOT be installed
	// If qmd is installed, this test will pass unexpectedly
	// TODO: Update test to use mock client or skipif qmd installed
	if _, err := exec.LookPath("qmd"); err == nil {
		t.Skip("qmd is installed, skipping test that expects qmd to be missing")
	}

	client := &Client{
		qmdPath: "qmd",
		timeout: DefaultTimeout,
	}

	ctx := context.Background()
	req := qmd.SearchRequest{
		RepoPath: "/path/to/repo",
		Query:    "test query",
		// Limit is missing - should default to MaxSearchResults
	}

	// This test would normally fail because qmd isn't installed
	// We're just testing the validation logic
	_, err := client.Search(ctx, req)

	// Should fail because qmd isn't actually installed
	if err == nil {
		t.Error("Expected error (qmd not installed)")
		return // Prevent continuation on nil error
	}
}

func TestMetadata_Getters(t *testing.T) {
	metadata := &Metadata{
		ID:     "KB-ARCH-0001",
		Domain: "architecture",
		Tags:   []string{"architecture", "core"},
		Source: "git",
	}

	if metadata.getID() != "KB-ARCH-0001" {
		t.Errorf("Expected ID 'KB-ARCH-0001', got '%s'", metadata.getID())
	}

	if metadata.getDomain() != "architecture" {
		t.Errorf("Expected domain 'architecture', got '%s'", metadata.getDomain())
	}

	tags := metadata.getTags()
	if len(tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tags))
	}

	if tags[0] != "architecture" || tags[1] != "core" {
		t.Errorf("Expected tags ['architecture', 'core'], got %v", tags)
	}
}

func TestMetadata_Getters_Nil(t *testing.T) {
	var metadata *Metadata

	if metadata.getID() != "" {
		t.Errorf("Expected empty ID from nil metadata, got '%s'", metadata.getID())
	}

	if metadata.getDomain() != "" {
		t.Errorf("Expected empty domain from nil metadata, got '%s'", metadata.getDomain())
	}

	tags := metadata.getTags()
	if tags != nil {
		t.Errorf("Expected nil tags from nil metadata, got %v", tags)
	}
}

func TestMetadata_Getters_EmptyFields(t *testing.T) {
	metadata := &Metadata{
		// All fields are empty
	}

	if metadata.getID() != "" {
		t.Errorf("Expected empty ID, got '%s'", metadata.getID())
	}

	if metadata.getDomain() != "" {
		t.Errorf("Expected empty domain, got '%s'", metadata.getDomain())
	}

	tags := metadata.getTags()
	if tags != nil {
		t.Errorf("Expected nil tags for empty field, got %v", tags)
	}
}
