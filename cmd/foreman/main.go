// Command foreman runs the Foreman controller (Block 4.2).
// It watches BrainTask resources and reconciles them (scheduling, status updates).
package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/foreman"
	"github.com/kube-zen/zen-brain1/internal/gate"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr, probeAddr string
	var numWorkers int
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address for metrics.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address for health probes.")
	flag.IntVar(&numWorkers, "workers", 2, "Number of worker goroutines for task execution (Block 4.3).")
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

	worker := foreman.NewWorker(mgr.GetClient(), foreman.PlaceholderRunner{}, numWorkers)
	worker.Start(ctx)

	reconciler := &foreman.Reconciler{
		Client:     mgr.GetClient(),
		Gate:       gate.NewStubGate(),
		Dispatcher: worker,
	}
	if err = reconciler.SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}

	if err := mgr.Start(ctx); err != nil {
		os.Exit(1)
	}
}
