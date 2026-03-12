package zencontroller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-zen/zen-sdk/pkg/events"
	zenlog "github.com/kube-zen/zen-sdk/pkg/logging"
	zenobs "github.com/kube-zen/zen-sdk/pkg/observability"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

// ZenClusterReconciler reconciles ZenCluster resources (Block 6: lifecycle + status).
type ZenClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      *zenlog.Logger
	Recorder *events.Recorder
}

// Reconcile updates ZenCluster status (ObservedGeneration, Phase, Ready, LastHeartbeatTime, AvailableCapacity).
func (r *ZenClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Create tracing span for this reconcile operation
	tracer := zenobs.GetTracer("zen-brain.controller")
	ctx, span := tracer.Start(ctx, "ZenCluster.Reconcile")
	defer span.End()

	// Use structured logging with context
	logger := r.Log.WithContext(ctx).With(
		zenlog.String("resource", req.NamespacedName.String()),
	)

	logger.Info("Reconciling ZenCluster",
		zenlog.Operation("reconcile"),
	)

	var cluster v1alpha1.ZenCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cluster.Status.ObservedGeneration = cluster.Generation
	cluster.Status.LastHeartbeatTime = &metav1.Time{Time: time.Now().UTC()}

	// Mirror spec capacity to status as best-effort available capacity (no live probe yet)
	if cluster.Spec.Capacity.CPUCores > 0 || cluster.Spec.Capacity.MemoryGB > 0 ||
		cluster.Spec.Capacity.GPUs > 0 || cluster.Spec.Capacity.StorageGB > 0 {
		cluster.Status.AvailableCapacity = cluster.Spec.Capacity

		logger.Info("Cluster capacity updated",
			zenlog.Int64("cpu_cores", cluster.Spec.Capacity.CPUCores),
			zenlog.Int64("memory_gb", cluster.Spec.Capacity.MemoryGB),
			zenlog.Int64("gpus", cluster.Spec.Capacity.GPUs),
			zenlog.Int64("storage_gb", cluster.Spec.Capacity.StorageGB),
		)
	}

	if cluster.Status.Phase == "" {
		cluster.Status.Phase = "Ready"

		// Record event for cluster becoming ready
		if r.Recorder != nil {
			r.Recorder.Eventf(
				&cluster,
				corev1.EventTypeNormal,
				"ClusterReady",
				"ZenCluster %s is now ready",
				req.Name,
			)
		}
	}

	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciled",
		Message: "ZenCluster reconciled; heartbeat and capacity updated",
	})

	if err := r.Status().Update(ctx, &cluster); err != nil {
		// TODO: Fix zenlog.Error signature
		// logger.Error(err, "Failed to update ZenCluster status",
		// 	zenlog.Operation("status_update"),
		// )
		logger.Error("Failed to update ZenCluster status",
			zenlog.Error(err),
			zenlog.Operation("status_update"),
		)
		return ctrl.Result{}, err
	}

	logger.Info("ZenCluster reconciliation completed",
		zenlog.String("phase", cluster.Status.Phase),
		zenlog.Int64("observed_generation", cluster.Status.ObservedGeneration),
		zenlog.String("last_heartbeat", cluster.Status.LastHeartbeatTime.Time.Format(time.RFC3339)),
	)

	return ctrl.Result{}, nil
}

// SetupWithManager registers reconciler with manager.
func (r *ZenClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ZenCluster{}).
		Complete(r)
}
