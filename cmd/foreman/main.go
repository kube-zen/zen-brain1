// Command foreman runs the Foreman controller (Block 4.2).
// It watches BrainTask resources and reconciles them (scheduling, status updates).
package main

import (
	"flag"
	"log"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/agent"
	internalcontext "github.com/kube-zen/zen-brain1/internal/context"
	"github.com/kube-zen/zen-brain1/internal/evidence"
	"github.com/kube-zen/zen-brain1/internal/foreman"
	"github.com/kube-zen/zen-brain1/internal/gate"
	"github.com/kube-zen/zen-brain1/internal/guardian"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr, probeAddr string
	var numWorkers int
	var runtimeDir, workspaceHome string
	var preferRealTemplates bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address for metrics.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address for health probes.")
	flag.IntVar(&numWorkers, "workers", 2, "Number of worker goroutines for task execution (Block 4.3).")
	flag.StringVar(&runtimeDir, "factory-runtime-dir", envStr("ZEN_FOREMAN_RUNTIME_DIR", "/tmp/zen-brain-factory"), "Runtime dir for Factory workspaces and proof-of-work.")
	flag.StringVar(&workspaceHome, "factory-workspace-home", envStr("ZEN_FOREMAN_WORKSPACE_HOME", "/tmp/zen-brain-factory"), "Workspace home for Factory (workspaces created under <home>/workspaces).")
	flag.BoolVar(&preferRealTemplates, "factory-prefer-real-templates", envBool("ZEN_FOREMAN_PREFER_REAL_TEMPLATES", true), "Prefer real templates when workDomain is empty (implementation, docs, debug, refactor, review).")
	zenContextRedis := flag.String("zen-context-redis", envStr("ZEN_CONTEXT_REDIS_URL", ""), "Redis URL for ZenContext (ReMe). When set, Worker uses ReMeBinder for session context on continuation.")
	sessionAffinity := flag.Bool("session-affinity", envBool("ZEN_FOREMAN_SESSION_AFFINITY", false), "Route tasks by session (same session → same worker).")
	flag.Parse()

	ctx := ctrl.SetupSignalHandler()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		os.Exit(1)
	}

	cfg := foreman.FactoryTaskRunnerConfig{
		RuntimeDir:          runtimeDir,
		WorkspaceHome:       workspaceHome,
		PreferRealTemplates: preferRealTemplates,
	}
	runner, err := foreman.NewFactoryTaskRunner(cfg)
	if err != nil {
		log.Printf("Foreman: failed to create FactoryTaskRunner: %v", err)
		os.Exit(1)
	}
	runner.Vault = evidence.NewMemoryVault() // proof-of-work evidence stored when tasks succeed
	log.Printf("Foreman: FactoryTaskRunner (runtime %s, workspace %s, prefer-real=%v)", runtimeDir, workspaceHome, preferRealTemplates)

	worker := foreman.NewWorker(mgr.GetClient(), runner, numWorkers)
	worker.SessionAffinity = *sessionAffinity
	if *zenContextRedis != "" {
		zc, err := internalcontext.NewMinimalZenContext(*zenContextRedis, "default")
		if err != nil {
			log.Printf("Warning: ZenContext (ReMe) not available: %v", err)
		} else {
			defer zc.Close()
			worker.ContextBinder = agent.NewReMeBinder(zc, "default")
			log.Printf("Foreman: ReMe enabled (ZenContext Redis)")
		}
	}
	worker.Start(ctx)

	reconciler := &foreman.Reconciler{
		Client:     mgr.GetClient(),
		Gate:       gate.NewStubGate(),
		Guardian:   guardian.NewStubGuardian(),
		Dispatcher: worker,
	}
	if err = reconciler.SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}

	queueStatusReconciler := &foreman.QueueStatusReconciler{Client: mgr.GetClient()}
	if err = queueStatusReconciler.SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}

	if err := mgr.Start(ctx); err != nil {
		os.Exit(1)
	}
}

func envStr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || v == "true" || v == "yes"
	}
	return defaultVal
}
