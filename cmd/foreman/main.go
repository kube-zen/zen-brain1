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
	"github.com/kube-zen/zen-brain1/internal/factory"
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
	var useFactory bool
	var factoryRuntimeDir string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address for metrics.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address for health probes.")
	flag.IntVar(&numWorkers, "workers", 2, "Number of worker goroutines for task execution (Block 4.3).")
	flag.BoolVar(&useFactory, "factory", envBool("ZEN_FOREMAN_FACTORY", false), "Run tasks via Factory (workspace + bounded executor + proof-of-work).")
	sessionAffinity := flag.Bool("session-affinity", envBool("ZEN_FOREMAN_SESSION_AFFINITY", false), "Route tasks by session (same session → same worker).")
	flag.StringVar(&factoryRuntimeDir, "factory-runtime-dir", envStr("ZEN_FACTORY_RUNTIME_DIR", "/tmp/zen-foreman-factory"), "Runtime dir for Factory workspaces and proof-of-work (when -factory).")
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

	var runner foreman.TaskRunner = foreman.PlaceholderRunner{}
	if useFactory {
		workspaceManager := factory.NewWorkspaceManager(factoryRuntimeDir)
		executor := factory.NewBoundedExecutor()
		powManager := factory.NewProofOfWorkManager(factoryRuntimeDir)
		factoryImpl := factory.NewFactory(workspaceManager, executor, powManager, factoryRuntimeDir)
		runner = foreman.NewFactoryTaskRunner(factoryImpl)
		log.Printf("Foreman: Factory enabled (runtime dir %s)", factoryRuntimeDir)
	}

	worker := foreman.NewWorker(mgr.GetClient(), runner, numWorkers)
	worker.SessionAffinity = *sessionAffinity
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
