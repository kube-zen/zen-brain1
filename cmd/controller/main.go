// Command controller runs the Zen-Brain controller (Block 6). It reconciles ZenProject and ZenCluster CRDs in-cluster.
// Integrated with zen-sdk: observability, leader election, unified logging.
package main

import (
	"context"
	"flag"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kube-zen/zen-sdk/pkg/events"
	"github.com/kube-zen/zen-sdk/pkg/leader"
	zenlog "github.com/kube-zen/zen-sdk/pkg/logging"
	zenobs "github.com/kube-zen/zen-sdk/pkg/observability"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/cryptoutil"
	"github.com/kube-zen/zen-brain1/internal/dlqmgr"
	"github.com/kube-zen/zen-brain1/internal/zencontroller"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = zenlog.NewLogger("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {
	ctx := context.Background()

	// Parse flags
	var metricsAddr string
	var probeAddr string
	var enableLeaderElection bool
	var leaderElectionID string
	var leaderElectionNamespace string
	var enableOTEL bool
	var otlpEndpoint string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "zen-brain-controller-lock",
		"Name of the ConfigMap/Lease resource used for leader election.")
	flag.StringVar(&leaderElectionNamespace, "leader-election-namespace", "zen-system",
		"Namespace for leader election ConfigMap/Lease resource.")
	flag.BoolVar(&enableOTEL, "enable-otel", true, "Enable OpenTelemetry tracing.")
	flag.StringVar(&otlpEndpoint, "otlp-endpoint", "", "OTLP collector endpoint (default: from env OTEL_EXPORTER_OTLP_ENDPOINT).")

	flag.Parse()

	setupLog.Info("Starting zen-brain controller")

	// Phase 1: Initialize observability (OpenTelemetry)
	var shutdownOTEL func(context.Context) error
	if enableOTEL {
		// Determine OTEL config
		otelConfig := zenobs.Config{
			ServiceName:    "zen-brain-controller",
			ServiceVersion: getBuildVersion(),
			Environment:    getEnvironment(),
		}

		// Use endpoint from flag or environment
		if otlpEndpoint != "" {
			otelConfig.OTLPEndpoint = otlpEndpoint
		}

		// Determine sampling rate based on environment
		if otelConfig.Environment == "production" {
			otelConfig.SamplingRate = 0.1 // 10% sampling in production
		} else {
			otelConfig.SamplingRate = 1.0 // 100% sampling in dev/staging
		}

		// Initialize OTEL
		var err error
		shutdownOTEL, err = zenobs.Init(ctx, otelConfig)
		if err != nil {
			setupLog.Error("Failed to initialize OpenTelemetry, continuing without tracing", zenlog.Error(err))
			shutdownOTEL = nil
		} else {
			setupLog.Info("OpenTelemetry initialized",
				zenlog.String("endpoint", otelConfig.OTLPEndpoint),
				zenlog.Float64("sampling_rate", otelConfig.SamplingRate),
			)
			defer func() {
				if shutdownOTEL != nil {
					if err := shutdownOTEL(ctx); err != nil {
						setupLog.Error("Failed to shutdown OpenTelemetry", zenlog.Error(err))
					}
				}
			}()
		}
	}

	// Phase 2: Initialize crypto
	if err := cryptoutil.Init(); err != nil {
		setupLog.Warn("Failed to initialize crypto, encryption disabled",
			zenlog.Error(err),
		)
	} else if cryptoutil.IsEnabled() {
		setupLog.Info("Crypto initialized",
			zenlog.String("status", "enabled"),
		)
	} else {
		setupLog.Info("Crypto disabled (no AGE keys in environment)")
	}

	// Phase 3: Initialize DLQ
	if err := dlqmgr.Init(ctx); err != nil {
		setupLog.Warn("Failed to initialize DLQ",
			zenlog.Error(err),
		)
	} else {
		setupLog.Info("DLQ initialized",
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
		setupLog.Info("DLQ replay worker started",
			zenlog.String("interval", interval.String()),
		)
	}

	// Phase 4: Build controller manager options and apply leader election
	mgrOpts := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
	}

	// Apply leader election via zen-sdk/pkg/leader
	if enableLeaderElection {
		leader.ApplyLeaderElection(&mgrOpts, "zen-brain-controller", leaderElectionNamespace, leaderElectionID, true)
		setupLog.Info("Leader election enabled",
			zenlog.String("resource", leaderElectionNamespace+"/"+leaderElectionID),
		)
	}

	// Create manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error("Unable to start manager", zenlog.Error(err))
		os.Exit(1)
	}

	// Create Kubernetes client for event recorder
	k8sClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error("Failed to create Kubernetes client", zenlog.Error(err))
		os.Exit(1)
	}

	// Setup reconcilers
	logger := zenlog.NewLogger("zen-brain.controller")

	// Create event recorder
	eventRecorder := events.NewRecorder(k8sClient, "zen-brain-controller")

	if err = (&zencontroller.ZenProjectReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Log:      logger,
		Recorder: eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", "ZenProject")
		os.Exit(1)
	}

	if err = (&zencontroller.ZenClusterReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Log:      logger,
		Recorder: eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", "ZenCluster")
		os.Exit(1)
	}

	// Add health check endpoints
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")

	// Start manager
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}

	setupLog.Info("Manager stopped")
}

// getBuildVersion returns the build version
func getBuildVersion() string {
	if v := os.Getenv("VERSION"); v != "" {
		return v
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

	// Detect from Kubernetes
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		// Check for production indicators
		if os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != "" {
			return "production"
		}
		return "staging"
	}

	return "development"
}
