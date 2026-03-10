// Package zencontroller implements the Zen-Brain controller (Block 6) for ZenProject and ZenCluster.
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

// ZenProjectReconciler reconciles ZenProject resources (Block 6: lifecycle + status).
type ZenProjectReconciler struct {
	client.Client
}

// Reconcile updates ZenProject status: validates ClusterRef, sets ObservedGeneration, Phase, conditions, LastSyncTime.
func (r *ZenProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var proj v1alpha1.ZenProject
	if err := r.Get(ctx, req.NamespacedName, &proj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	proj.Status.ObservedGeneration = proj.Generation
	now := metav1.NewTime(time.Now().UTC())
	proj.Status.LastSyncTime = &now

	// Validate ClusterRef: referenced ZenCluster must exist in the same namespace
	clusterRefValid := false
	if proj.Spec.ClusterRef != "" {
		var cluster v1alpha1.ZenCluster
		err := r.Get(ctx, client.ObjectKey{Namespace: proj.Namespace, Name: proj.Spec.ClusterRef}, &cluster)
		if err == nil {
			clusterRefValid = true
			meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
				Type:    "ClusterRefValid",
				Status:  metav1.ConditionTrue,
				Reason:  "ClusterFound",
				Message: "Referenced ZenCluster exists",
			})
		} else {
			meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
				Type:    "ClusterRefValid",
				Status:  metav1.ConditionFalse,
				Reason:  "ClusterNotFound",
				Message: "ZenCluster " + proj.Spec.ClusterRef + " not found in namespace " + proj.Namespace,
			})
		}
	}

	if clusterRefValid {
		proj.Status.Phase = "Ready"
		meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "Reconciled",
			Message: "ZenProject reconciled; cluster ref valid",
		})
	} else {
		proj.Status.Phase = "Pending"
		meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  "ClusterRefInvalid",
			Message: "ClusterRef must reference an existing ZenCluster in the same namespace",
		})
	}

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
