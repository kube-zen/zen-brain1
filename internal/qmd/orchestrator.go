// Package qmd provides orchestration for scheduled QMD index refresh.
package qmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/scheduler"
	"github.com/kube-zen/zen-brain1/pkg/qmd"
)

// Orchestrator manages scheduled QMD index refresh jobs.
type Orchestrator struct {
	client    qmd.Client
	scheduler *scheduler.Scheduler
	config    *OrchestratorConfig
	logger    *log.Logger
}

// OrchestratorConfig holds configuration for the QMD orchestrator.
type OrchestratorConfig struct {
	// RepoPath is the path to the zen-docs repository.
	RepoPath string

	// RefreshInterval is the interval between automatic index refreshes.
	RefreshInterval time.Duration

	// Verbose enables verbose logging.
	Verbose bool

	// SkipAvailabilityCheck skips qmd availability check.
	SkipAvailabilityCheck bool
}

// NewOrchestrator creates a new QMD orchestrator.
func NewOrchestrator(qmdClient qmd.Client, config *OrchestratorConfig) (*Orchestrator, error) {
	if qmdClient == nil {
		return nil, fmt.Errorf("qmd client is required")
	}
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.RepoPath == "" {
		return nil, fmt.Errorf("repo_path is required")
	}
	if config.RefreshInterval <= 0 {
		config.RefreshInterval = time.Hour // default
	}

	logger := log.Default()
	if config.Verbose {
		log.Printf("[QMD Orchestrator] Creating orchestrator for repo: %s, interval: %v",
			config.RepoPath, config.RefreshInterval)
	}

	// Create scheduler with in-memory store (no persistence needed for now)
	sched := scheduler.New(&scheduler.Config{
		Logger: logger,
		OnJobStart: func(job *scheduler.Job) {
			log.Printf("[QMD Orchestrator] Job started: %s", job.Name)
		},
		OnJobEnd: func(job *scheduler.Job, result *scheduler.JobResult) {
			if result.Success {
				log.Printf("[QMD Orchestrator] Job completed: %s, duration: %v",
					job.Name, result.Duration)
			} else {
				log.Printf("[QMD Orchestrator] Job failed: %s, error: %v",
					job.Name, result.Error)
			}
		},
	})

	orc := &Orchestrator{
		client:    qmdClient,
		scheduler: sched,
		config:    config,
		logger:    logger,
	}

	// Register the refresh handler
	sched.RegisterHandler("qmd_refresh", orc.refreshHandler)

	return orc, nil
}

// Start starts the orchestrator and schedules the refresh job.
func (o *Orchestrator) Start() error {
	// Schedule periodic refresh
	job := &scheduler.Job{
		ID:          "qmd_refresh",
		Name:        "QMD Index Refresh",
		Description: "Periodically refreshes the QMD index for the zen-docs repository",
		Schedule:    fmt.Sprintf("@every %ds", int(o.config.RefreshInterval.Seconds())),
		Handler:     "qmd_refresh",
		Args: map[string]interface{}{
			"repo_path": o.config.RepoPath,
		},
		Enabled: true,
	}

	if err := o.scheduler.AddJob(job); err != nil {
		return fmt.Errorf("failed to add refresh job: %w", err)
	}

	// Start scheduler
	if err := o.scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	o.logger.Printf("[QMD Orchestrator] Started with refresh interval %v", o.config.RefreshInterval)
	return nil
}

// Stop stops the orchestrator.
func (o *Orchestrator) Stop() {
	o.scheduler.Stop()
	o.logger.Printf("[QMD Orchestrator] Stopped")
}

// RefreshNow triggers an immediate index refresh.
func (o *Orchestrator) RefreshNow(ctx context.Context) error {
	req := qmd.EmbedRequest{
		RepoPath: o.config.RepoPath,
		Paths:    []string{"docs/"},
	}
	if err := o.client.RefreshIndex(ctx, req); err != nil {
		return fmt.Errorf("qmd refresh failed: %w", err)
	}
	o.logger.Printf("[QMD Orchestrator] Manual refresh completed")
	return nil
}

// refreshHandler is the scheduler handler for the refresh job.
func (o *Orchestrator) refreshHandler(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	repoPath, ok := args["repo_path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid repo_path argument")
	}

	req := qmd.EmbedRequest{
		RepoPath: repoPath,
		Paths:    []string{"docs/"},
	}

	start := time.Now()
	err := o.client.RefreshIndex(ctx, req)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("qmd refresh failed after %v: %w", duration, err)
	}

	return map[string]interface{}{
		"duration": duration,
		"repo":     repoPath,
	}, nil
}

// Stats returns orchestrator statistics.
func (o *Orchestrator) Stats() map[string]interface{} {
	schedStats := o.scheduler.Stats()
	return map[string]interface{}{
		"refresh_interval": o.config.RefreshInterval,
		"repo_path":        o.config.RepoPath,
		"scheduler":        schedStats,
	}
}