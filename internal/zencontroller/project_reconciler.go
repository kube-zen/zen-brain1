// Package zencontroller implements the Zen-Brain controller (Block 6) for ZenProject and ZenCluster.
package zencontroller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

// ZenProjectReconciler reconciles ZenProject resources (status-only; Block 6 in-cluster).
type ZenProjectReconciler struct {
	client.Client
}

// Reconcile updates ZenProject status (ObservedGeneration, Phase, Ready condition).
func (r *ZenProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var proj v1alpha1.ZenProject
	if err := r.Get(ctx, req.NamespacedName, &proj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if proj.Status.Phase == "Ready" && len(proj.Status.Conditions) > 0 {
		return ctrl.Result{}, nil
	}
	if proj.Status.Phase == "" {
		proj.Status.Phase = "Ready"
	}
	meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
		Type:   "Ready",
		Status: metav1.ConditionTrue,
		Reason: "Reconciled",
		Message: "ZenProject reconciled by zen-brain controller",
	})
	if err := r.Status().Update(ctx, &proj); err != nil {
		logger.Error(err, "failed to update ZenProject status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler with the manager.
func (r *ZenProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ZenProject{}).
		Complete(r)
}
