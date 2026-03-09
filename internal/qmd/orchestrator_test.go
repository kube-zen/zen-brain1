package qmd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/qmd"
)

type mockOrchestratorQMDClient struct {
	mu            sync.Mutex
	refreshCalled bool
	refreshErr    error
	searchCalled  bool
}

func (m *mockOrchestratorQMDClient) RefreshIndex(ctx context.Context, req qmd.EmbedRequest) error {
	m.mu.Lock()
	m.refreshCalled = true
	m.mu.Unlock()
	return m.refreshErr
}

func (m *mockOrchestratorQMDClient) Search(ctx context.Context, req qmd.SearchRequest) ([]byte, error) {
	m.mu.Lock()
	m.searchCalled = true
	m.mu.Unlock()
	return nil, nil
}

func (m *mockOrchestratorQMDClient) getRefreshCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshCalled
}

func TestNewOrchestrator(t *testing.T) {
	client := &mockOrchestratorQMDClient{}
	config := &OrchestratorConfig{
		RepoPath:              "/path/to/repo",
		RefreshInterval:       30 * time.Second,
		SkipAvailabilityCheck: true,
	}

	orc, err := NewOrchestrator(client, config)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}
	if orc == nil {
		t.Fatal("Orchestrator should not be nil")
	}
}

func TestOrchestrator_StartStop(t *testing.T) {
	client := &mockOrchestratorQMDClient{}
	config := &OrchestratorConfig{
		RepoPath:              "/path/to/repo",
		RefreshInterval:       time.Second,
		SkipAvailabilityCheck: true,
	}

	orc, err := NewOrchestrator(client, config)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	// Start orchestrator
	if err := orc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give scheduler a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop orchestrator
	orc.Stop()

	// Verify orchestrator can be stopped without panic
	// (no assertion about refresh being called, as scheduler may not have triggered within the short window)
}

func TestOrchestrator_RefreshNow(t *testing.T) {
	client := &mockOrchestratorQMDClient{}
	config := &OrchestratorConfig{
		RepoPath:              "/path/to/repo",
		RefreshInterval:       time.Hour,
		SkipAvailabilityCheck: true,
	}

	orc, err := NewOrchestrator(client, config)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = orc.RefreshNow(ctx)
	if err != nil {
		t.Fatalf("RefreshNow failed: %v", err)
	}

	if !client.getRefreshCalled() {
		t.Error("Expected refresh to be called")
	}
}

func TestOrchestrator_InvalidConfig(t *testing.T) {
	client := &mockOrchestratorQMDClient{}

	// Missing RepoPath
	config := &OrchestratorConfig{}
	_, err := NewOrchestrator(client, config)
	if err == nil {
		t.Error("Expected error for missing repo_path")
	}

	// Nil client
	_, err = NewOrchestrator(nil, &OrchestratorConfig{RepoPath: "/path"})
	if err == nil {
		t.Error("Expected error for nil client")
	}
}

func TestOrchestrator_Stats(t *testing.T) {
	client := &mockOrchestratorQMDClient{}
	config := &OrchestratorConfig{
		RepoPath:              "/path/to/repo",
		RefreshInterval:       5 * time.Minute,
		SkipAvailabilityCheck: true,
	}

	orc, err := NewOrchestrator(client, config)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	stats := orc.Stats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	if stats["repo_path"] != "/path/to/repo" {
		t.Errorf("Expected repo_path in stats, got %v", stats["repo_path"])
	}
}