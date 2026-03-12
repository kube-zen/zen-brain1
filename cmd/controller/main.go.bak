// Command controller runs the Zen-Brain controller (Block 6). It reconciles ZenProject and ZenCluster CRDs in-cluster.
package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/zencontroller"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

func main() {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: ":8080"},
		HealthProbeBindAddress: ":8081",
	})
	if err != nil {
		os.Exit(1)
	}

	if err = (&zencontroller.ZenProjectReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}
	if err = (&zencontroller.ZenClusterReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}
