package zencontroller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

// ZenClusterReconciler reconciles ZenCluster resources (Block 6: lifecycle + status).
type ZenClusterReconciler struct {
	client.Client
}

// Reconcile updates ZenCluster status (ObservedGeneration, Phase, Ready, LastHeartbeatTime, AvailableCapacity).
func (r *ZenClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
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
	}

	if cluster.Status.Phase == "" {
		cluster.Status.Phase = "Ready"
	}
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciled",
		Message: "ZenCluster reconciled; heartbeat and capacity updated",
	})
	if err := r.Status().Update(ctx, &cluster); err != nil {
		logger.Error(err, "failed to update ZenCluster status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler with the manager.
func (r *ZenClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ZenCluster{}).
		Complete(r)
}
