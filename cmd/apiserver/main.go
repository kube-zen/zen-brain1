// Command apiserver runs the zen-brain API server (Block 3.4).
// Serves /healthz, /readyz and optional future REST endpoints.
// Block 3: StrictRuntime bootstrap; /readyz reflects LIVE dependency state; /api/v1/health returns runtime report.
// Integrated with zen-sdk: observability (OpenTelemetry) and unified logging.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	zenlog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-sdk/pkg/observability"

	"github.com/kube-zen/zen-brain1/internal/apiserver"
	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/cryptoutil"
	"github.com/kube-zen/zen-brain1/internal/dlqmgr"
	"github.com/kube-zen/zen-brain1/internal/llm"
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

	// Initialize crypto
	if err := cryptoutil.Init(); err != nil {
		setupLogger.Warn("Failed to initialize crypto, encryption disabled",
			zenlog.Error(err),
		)
	} else if cryptoutil.IsEnabled() {
		setupLogger.Info("Crypto initialized",
			zenlog.String("status", "enabled"),
		)
	} else {
		setupLogger.Info("Crypto disabled (no AGE keys in environment)")
	}

	// Initialize DLQ
	if err := dlqmgr.Init(ctx); err != nil {
		setupLogger.Warn("Failed to initialize DLQ",
			zenlog.Error(err),
		)
	} else {
		setupLogger.Info("DLQ initialized",
			zenlog.String("status", "enabled"),
		)

		// Start background replay worker
		interval := 5 * time.Minute
		if s := os.Getenv("DLQ_REPLAY_INTERVAL"); s != "" {
			if d, err := time.ParseDuration(s); err == nil {
				interval = d
			}
		}
		go dlqmgr.StartReplayWorker(ctx, interval, nil)
		setupLogger.Info("DLQ replay worker started",
			zenlog.String("interval", interval.String()),
		)
	}

	addr := ":8080"
	if p := os.Getenv("API_SERVER_PORT"); p != "" {
		addr = ":" + p
	}

	setupLogger.Info("Starting zen-brain API server",
		zenlog.String("addr", addr),
		zenlog.String("version", getVersion()),
	)

	// Block 3: Canonical strict runtime bootstrap from config
	profile := os.Getenv("ZEN_RUNTIME_PROFILE")
	if profile == "" {
		profile = detectProfile()
	}

	cfg, errLoad := config.LoadConfig("")
	if errLoad != nil || cfg == nil {
		if errLoad != nil {
			setupLogger.Warn("Config load failed, using defaults",
				zenlog.Error(errLoad),
			)
		}
		cfg = config.DefaultConfig()
	}

	// Use StrictRuntime for fail-closed behavior
	strictRT, errRT := runtime.NewStrictRuntime(ctx, &runtime.StrictRuntimeConfig{
		Profile:        profile,
		Config:         cfg,
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

	// Start live health checker for dynamic readiness
	var healthChecker *runtime.LiveHealthChecker
	if strictRT != nil {
		healthChecker = runtime.NewLiveHealthChecker(&runtime.LiveHealthCheckerConfig{
			StrictRuntime: strictRT,
			RefreshPeriod: 30e9, // 30 seconds
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

	var report *runtime.RuntimeReport
	if strictRT != nil {
		report = strictRT.Report()
	}
	if report != nil {
		logger.Info("Block 3 capability banner",
			zenlog.String("capabilities", capabilityBanner(report)),
		)
	}

	// Use live readiness checker instead of static report
	checker := apiserver.NewLiveRuntimeChecker(strictRT, healthChecker)

	// Create API server with observability middleware
	srv := apiserver.New(addr, checker)

	// Apply observability middleware to HTTP handlers
	if otelShutdown != nil {
		// Wrap main handlers with tracing middleware
		setupObservabilityHandlers(srv, report)
	}

	srv.AuthAPIKey = os.Getenv("ZEN_API_KEY")
	if srv.AuthAPIKey != "" {
		logger.Info("API auth enabled (ZEN_API_KEY set); /healthz and /readyz are exempt")
	}

	srv.Handle("/api/v1/sessions", apiserver.SessionsHandler(nil))
	srv.Handle("/api/v1/sessions/", apiserver.SessionDetailHandler(nil))
	// /api/v1/health registered in setupObservabilityHandlers with tracing middleware
	srv.Handle("/api/v1/evidence", apiserver.EvidenceHandler(nil))

	// LLM Gateway setup
	gwCfg := llm.DefaultGatewayConfig()
	if s := os.Getenv("OLLAMA_TIMEOUT_SECONDS"); s != "" {
		if sec, err := strconv.Atoi(s); err == nil && sec > 0 {
			gwCfg.LocalWorkerTimeout = sec
			if sec > gwCfg.RequestTimeout {
				gwCfg.RequestTimeout = sec
			}
		}
	}
	if s := os.Getenv("OLLAMA_KEEP_ALIVE"); s != "" {
		gwCfg.LocalWorkerKeepAlive = s
	}
	gateway, errGW := llm.NewGateway(gwCfg)
	if errGW != nil {
		logger.Warn("LLM gateway not available",
			zenlog.Error(errGW),
		)
		srv.Handle("/api/v1/chat", apiserver.ChatHandler(nil))
	} else {
		// ZB-024: Log local CPU profile clearly
		logger.Info("ZB-024: Local CPU inference profile active",
			zenlog.String("local_model", gwCfg.LocalWorkerModel),
			zenlog.Int("local_worker_timeout", gwCfg.LocalWorkerTimeout),
			zenlog.Int("request_timeout", gwCfg.RequestTimeout),
			zenlog.String("keep_alive", gwCfg.LocalWorkerKeepAlive),
		)
		// Ollama warmup is DEPRECATED and FORBIDDEN for zen-brain1.
		// Primary inference uses llama.cpp (L1/L2). OLLAMA_BASE_URL should not be set.
		// If set accidentally, log a warning and do NOT start warmup.
		if baseURL := os.Getenv("OLLAMA_BASE_URL"); baseURL != "" {
			logger.Warn("OLLAMA_BASE_URL is set but Ollama is deprecated and forbidden. "+
				"Remove OLLAMA_BASE_URL from deployment. Using llama.cpp L1/L2 instead.",
				zenlog.String("url", baseURL))
		}
		srv.Handle("/api/v1/chat", apiserver.ChatHandler(gateway))
	}

	if v := os.Getenv("API_VERSION"); v != "" {
		srv.Handle("/api/v1/version", apiserver.VersionHandler(v))
	} else {
		srv.Handle("/api/v1/version", apiserver.VersionHandler("dev"))
	}

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
	// Wrap API endpoints (healthz/readyz already registered in Server.New())
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

// getEnvironment returns the deployment environment
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

// getVersion returns the build version
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
