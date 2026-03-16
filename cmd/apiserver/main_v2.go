// Command apiserver runs zen-brain API server with real components.
// Serves /healthz, /readyz, /api/v1/chat, /api/v1/ws
// Integrated with: Session Manager, Planner, WebSocket Hub
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/observability"
	zenlog "github.com/kube-zen/zen-sdk/pkg/logging"

	"github.com/kube-zen/zen-brain1/internal/apiserver"
	"github.com/kube-zen/zen-brain1/internal/runtime"
)

var (
	logger       = zenlog.NewLogger("zen-brain.apiserver")
	setupLogger  = zenlog.NewLogger("setup")
	otelShutdown func(context.Context) error
)

func main() {
	ctx := context.Background()

	// Initialize OpenTelemetry
	if err := initObservability(ctx); err != nil {
		setupLogger.Error(err, "Failed to initialize observability, continuing without tracing")
	} else {
		defer func() {
			if otelShutdown != nil {
				if err := otelShutdown(ctx); err != nil {
					logger.Error(err, "Failed to shutdown OpenTelemetry")
				}
			}
		}()
	}

	addr := ":8080"
	if p := os.Getenv("API_SERVER_PORT"); p != "" {
		addr = ":" + p
	}

	setupLogger.Info("Starting zen-brain API server with real components",
		zenlog.String("addr", addr),
		zenlog.String("version", getVersion()),
	)

	// Block 3: Canonical strict runtime bootstrap from config
	profile := os.Getenv("ZEN_RUNTIME_PROFILE")
	if profile == "" {
		profile = detectProfile()
	}

	// Initialize StrictRuntime
	strictRT, errRT := runtime.NewStrictRuntime(ctx, &runtime.StrictRuntimeConfig{
		Profile:        profile,
		Config:         nil, // Use default config
		EnableHealthCh: true, // Enable live health monitoring
	})

	if errRT != nil {
		// In strict mode (prod/staging), fail immediately
		if profile == "prod" || profile == "staging" {
			setupLogger.Error(errRT, "Strict runtime bootstrap failed",
				zenlog.String("profile", profile),
			)
			log.Fatalf("Strict runtime bootstrap failed: %v", errRT)
		}
		// In dev mode, continue with warning
		setupLogger.Warn("Runtime bootstrap warning (dev mode)",
			zenlog.Error(errRT),
			zenlog.String("profile", profile),
		)
	}

	// Start live health checker
	var healthChecker *runtime.LiveHealthChecker
	if strictRT != nil {
		healthChecker = runtime.NewLiveHealthChecker(&runtime.LiveHealthCheckerConfig{
			StrictRuntime: strictRT,
			RefreshPeriod:   30e9, // 30 seconds
		})
		if err := healthChecker.Start(ctx); err != nil {
			logger.Warn("Live health checker failed to start",
				zenlog.Error(err),
			)
		} else {
			defer healthChecker.Stop()
			logger.Info("Live health checker started",
				zenlog.String("refresh_period", "30s"),
			)
		}
	}

	// Create API server with real components setup
	checker := apiserver.NewLiveRuntimeChecker(strictRT, healthChecker)
	srv := apiserver.New(addr, checker)

	// Initialize real components: Session Manager, Planner, WebSocket Hub
	setup, err := apiserver.NewSetup(ctx, strictRT, healthChecker)
	if err != nil {
		log.Fatalf("Failed to initialize components: %v", err)
	}
	defer setup.Close()

	setupLogger.Info("Components initialized",
		zenlog.String("components", "Session Manager, Planner, WebSocket Hub"),
	)

	// Apply observability middleware
	if otelShutdown != nil {
		setupObservabilityHandlers(srv, strictRT.Report())
	}

	// Set auth key
	srv.AuthAPIKey = os.Getenv("ZEN_API_KEY")
	if srv.AuthAPIKey != "" {
		logger.Info("API auth enabled (ZEN_API_KEY set); /healthz and /readyz are exempt")
	}

	// Register handlers with real components
	srv.HandleFunc("/api/v1/sessions", apiserver.SessionsHandler(setup.SessionManager))
	srv.HandleFunc("/api/v1/sessions/", apiserver.SessionDetailHandler(setup.SessionManager))
	srv.HandleFunc("/api/v1/health", apiserver.RuntimeReportHandler(strictRT.Report))
	srv.HandleFunc("/api/v1/evidence", apiserver.EvidenceHandler(nil))

	// Register chat handler with real components
	srv.Handle("/api/v1/chat", apiserver.ChatHandler(setup.SessionManager, setup.Planner, setup.WebSocketHub))

	// Register WebSocket handler for real-time updates
	srv.Handle("/api/v1/ws", apiserver.WebSocketHandler(setup.WebSocketHub))

	// Register help and status handlers
	srv.Handle("/api/v1/help", apiserver.HelpHandler())
	srv.Handle("/api/v1/status", apiserver.StatusHandler(setup.SessionManager))

	// Register version handler
	if v := os.Getenv("API_VERSION"); v != "" {
		srv.Handle("/api/v1/version", apiserver.VersionHandler(v))
	} else {
		srv.Handle("/api/v1/version", apiserver.VersionHandler("dev"))
	}

	// Log registered handlers
	logger.Info("Handlers registered",
		zenlog.String("endpoints", "/healthz, /readyz, /api/v1/chat, /api/v1/ws, /api/v1/sessions, /api/v1/help, /api/v1/status"),
	)

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Starting HTTP server")
		if err := srv.Start(); err != nil && err != context.Canceled {
			logger.Error(err, "API server error")
		}
	}()

	<-sigCtx.Done()
	logger.Info("Shutting down API server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(err, "Shutdown error")
	}

	if strictRT != nil {
		if err := strictRT.Close(); err != nil {
			logger.Error(err, "Failed to close strict runtime")
		}
	}

	logger.Info("Shutdown complete")
}

// initObservability initializes OpenTelemetry tracing
func initObservability(ctx context.Context) error {
	// Check if OTEL is disabled
	if os.Getenv("DISABLE_OTEL") == "true" {
		setupLogger.Info("OpenTelemetry disabled via DISABLE_OTEL=true")
		return nil
	}

	// Determine environment
	env := getEnvironment()

	// Build OTEL config
	otelConfig := observability.Config{
		ServiceName:    "zen-brain-apiserver",
		ServiceVersion: getVersion(),
		Environment:    env,
	}

	// Use endpoint from environment or default
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		otelConfig.OTLPEndpoint = endpoint
	}

	// Determine sampling rate based on environment
	if env == "production" {
		otelConfig.SamplingRate = 0.1 // 10% sampling in production
	} else {
		otelConfig.SamplingRate = 1.0 // 100% sampling in dev/staging
	}

	// Use insecure endpoint in dev/staging, secure in production
	otelConfig.Insecure = (env != "production")

	// Initialize OTEL
	var err error
	otelShutdown, err = observability.Init(ctx, otelConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
	}

	setupLogger.Info("OpenTelemetry initialized",
		zenlog.String("endpoint", otelConfig.OTLPEndpoint),
		zenlog.Float64("sampling_rate", otelConfig.SamplingRate),
		zenlog.String("environment", env),
	)

	return nil
}

// setupObservabilityHandlers applies OTEL tracing middleware to HTTP handlers
func setupObservabilityHandlers(srv *apiserver.Server, report *runtime.RuntimeReport) {
	// Wrap health and ready endpoints
	srv.Handle("/healthz", observability.HTTPTracingMiddleware("zen-brain.apiserver", "/healthz")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			srv.HealthzHandler(w, r)
		}),
	))
	srv.Handle("/readyz", observability.HTTPTracingMiddleware("zen-brain.apiserver", "/readyz")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			srv.ReadyzHandler(w, r)
		}),
	))

	// Wrap API endpoints
	if report != nil {
		srv.Handle("/api/v1/health", observability.HTTPTracingMiddleware("zen-brain.apiserver", "/api/v1/health")(
			apiserver.RuntimeReportHandler(report),
		))
	}
}

// detectProfile detects runtime profile from environment
func detectProfile() string {
	if os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
		return "prod"
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		if os.Getenv("ZEN_BRAIN_ENV") == "production" {
			return "prod"
		}
		return "staging"
	}
	return "dev"
}

// getEnvironment returns deployment environment
func getEnvironment() string {
	// Check environment variable
	if env := os.Getenv("DEPLOYMENT_ENV"); env != "" {
		return env
	}
	if env := os.Getenv("ZEN_BRAIN_ENV"); env != "" {
		return env
	}
	return detectProfile()
}

// getVersion returns build version
func getVersion() string {
	if v := os.Getenv("VERSION"); v != "" {
		return v
	}
	return "dev"
}

// capabilityBanner returns a formatted capability string
func capabilityBanner(r *runtime.RuntimeReport) string {
	if r == nil {
		return "ZenContext=? Ledger=? MessageBus=?"
	}
	zc := "disabled"
	if r.ZenContext.Mode != "" {
		zc = string(r.ZenContext.Mode)
		if r.Tier1Hot.Healthy {
			zc += " (tier1 ok)"
		}
		if r.Tier2Warm.Mode == runtime.ModeReal && !r.Tier2Warm.Healthy {
			zc += ", tier2 degraded"
		}
		if r.Tier3Cold.Mode == runtime.ModeDisabled {
			zc += ", tier3 disabled"
		}
	}
	ledger := string(r.Ledger.Mode)
	if r.Ledger.Mode == "" {
		ledger = "stub"
	}
	mb := string(r.MessageBus.Mode)
	if r.MessageBus.Mode == "" {
		mb = "disabled"
	}
	return fmt.Sprintf("ZenContext=%s Ledger=%s MessageBus=%s", zc, ledger, mb)
}
