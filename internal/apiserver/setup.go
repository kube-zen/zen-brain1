// Package apiserver provides setup for API server with real components.
// This wires Session Manager, Planner, and WebSocket Hub together.
package apiserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kube-zen/zen-brain1/internal/analyzer"
	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/internal/runtime"
	"github.com/kube-zen/zen-brain1/internal/session"
	"github.com/kube-zen/zen-brain1/internal/websocket"
	"github.com/kube-zen/zen-brain1/pkg/context" as zenctx
	"github.com/kube-zen/zen-brain1/pkg/contracts"

	zenlog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// Setup holds all initialized components for the API server
type Setup struct {
	SessionManager session.Manager
	Planner        *planner.Planner
	WebSocketHub   *websocket.Hub
	StrictRuntime  *runtime.StrictRuntime
	LiveChecker   *runtime.LiveHealthChecker
}

// NewSetup initializes all components for the API server.
// This wires Session Manager, Planner, and WebSocket Hub together.
func NewSetup(ctx context.Context, strictRT *runtime.StrictRuntime, checker *runtime.LiveHealthChecker) (*Setup, error) {
	logger := zenlog.NewLogger("zen-brain.apiserver.setup")

	// 1. Initialize Session Manager
	sessionMgr, err := initSessionManager(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session manager: %w", err)
	}
	logger.Info("Session Manager initialized")

	// 2. Initialize Planner
	planr, err := initPlanner(ctx, sessionMgr, strictRT)
	if err != nil {
		sessionMgr.Close()
		return nil, fmt.Errorf("failed to initialize planner: %w", err)
	}
	logger.Info("Planner initialized")

	// 3. Initialize WebSocket Hub
	wsHub := websocket.NewHub()
	go wsHub.Run(ctx)
	logger.Info("WebSocket Hub initialized and running")

	// 4. Return setup with all components
	return &Setup{
		SessionManager: sessionMgr,
		Planner:       &planr,
		WebSocketHub:  wsHub,
		StrictRuntime:  strictRT,
		LiveChecker:   checker,
	}, nil
}

// Close closes all components
func (s *Setup) Close() error {
	logger := zenlog.NewLogger("zen-brain.apiserver.setup")

	var errs []error

	// Close planner
	if s.Planner != nil {
		if err := (*s.Planner).Close(); err != nil {
			errs = append(errs, err)
			logger.Error(err, "Failed to close planner")
		}
	}

	// Close session manager
	if s.SessionManager != nil {
		if err := s.SessionManager.Close(); err != nil {
			errs = append(errs, err)
			logger.Error(err, "Failed to close session manager")
		}
	}

	// Stop live health checker
	if s.LiveChecker != nil {
		s.LiveChecker.Stop()
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	logger.Info("All components closed successfully")
	return nil
}

// initSessionManager initializes the Session Manager
func initSessionManager(ctx context.Context) (session.Manager, error) {
	// Get data directory
	dataDir := os.Getenv("ZEN_BRAIN_HOME")
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".zen-brain", "sessions")
	}

	// Create data directory if needed
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create session config
	sessionCfg := &session.Config{
		StoreType:       "sqlite",
		DataDir:         dataDir,
		DefaultTimeout:  24 * time.Hour,
		MaxSessionAge:   7 * 24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		StaleThreshold:  2 * time.Hour,
		ClusterID:       "default",
	}

	// Create session manager
	sessionMgr, err := session.NewManager(sessionCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return sessionMgr, nil
}

// initPlanner initializes the Planner with Session Manager
func initPlanner(ctx context.Context, sessionMgr session.Manager, strictRT *runtime.StrictRuntime) (planner.Planner, error) {
	// Get runtime zen context (if available)
	var zenCtx zenctx.ZenContext
	if strictRT != nil {
		rt := strictRT.Runtime()
		zenCtx = rt.ZenContext
	}

	// Create components
	var (
		officeMgr  office.Manager
		analyzer   analyzer.IntentAnalyzer
		factory    factory.Factory
	)

	// For now, use simple implementations
	// Real implementation would use actual Office, Analyzer, Factory
	// from zen-brain architecture

	// Create planner config
	plannerCfg := &planner.Config{
		OfficeManager:  &officeMgr,
		Analyzer:       &analyzer,
		SessionManager:  sessionMgr,
		LedgerClient:   nil, // TODO: Wire real ledger client
		ZenContext:     zenCtx,
		Factory:        &factory,
		DefaultModel:    "glm-4.7",
		FallbackModel:   "glm-4.7",
		MaxCostUSD:      10.0,
		RequireApproval:  true,
		AutoApproveCost: 2.0,
		AnalysisTimeout: 300,
		ExecutionTimeout: 3600,
		MetricsEnabled:  true,
	}

	// Create planner
	planr, err := planner.NewPlanner(plannerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create planner: %w", err)
	}

	return planr, nil
}
