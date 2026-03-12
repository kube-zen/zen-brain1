// Package zencontroller implements the Zen-Brain controller (Block 6) for ZenProject and ZenCluster.
// Integrated with zen-sdk: unified logging and events.
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

// ZenProjectReconciler reconciles ZenProject resources (Block 6: lifecycle + status).
type ZenProjectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    *zenlog.Logger
	// Recorder is the event recorder (nil if not initialized)
	Recorder *events.Recorder
}

// Reconcile updates ZenProject status: validates ClusterRef, sets ObservedGeneration, Phase, conditions, LastSyncTime.
func (r *ZenProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Create tracing span for this reconcile operation
	tracer := zenobs.GetTracer("zen-brain.controller")
	ctx, span := tracer.Start(ctx, "ZenProject.Reconcile")
	defer span.End()

	// Use structured logging with context
	logger := r.Log.WithContext(ctx).With(
		zenlog.String("resource", req.NamespacedName.String()),
	)

	logger.Info("Reconciling ZenProject",
		zenlog.Operation("reconcile"),
	)

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

			logger.Info("Cluster reference validated",
				zenlog.String("cluster", proj.Spec.ClusterRef),
				zenlog.String("status", "valid"),
			)
		} else {
			meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
				Type:    "ClusterRefValid",
				Status:  metav1.ConditionFalse,
				Reason:  "ClusterNotFound",
				Message: "ZenCluster " + proj.Spec.ClusterRef + " not found in namespace " + proj.Namespace,
			})

			logger.Warn("Cluster reference invalid",
				zenlog.String("cluster", proj.Spec.ClusterRef),
				zenlog.String("namespace", proj.Namespace),
				zenlog.Error(err),
			)
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

		// Record Kubernetes event
		if r.Recorder != nil {
			r.Recorder.Eventf(
				&proj,
				corev1.EventTypeNormal,
				"ReconciliationSucceeded",
				"Successfully reconciled ZenProject %s in namespace %s",
				req.Name,
				req.Namespace,
			)
		}

		logger.Info("ZenProject ready",
			zenlog.String("phase", proj.Status.Phase),
		)
	} else {
		proj.Status.Phase = "Pending"
		meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  "ClusterRefInvalid",
			Message: "ClusterRef must reference an existing ZenCluster in the same namespace",
		})

		// Record warning event
		if r.Recorder != nil {
			r.Recorder.Eventf(
				&proj,
				corev1.EventTypeWarning,
				"ClusterRefInvalid",
				"ClusterRef %s not found in namespace %s",
				proj.Spec.ClusterRef,
				proj.Namespace,
			)
		}

		logger.Warn("ZenProject pending",
			zenlog.String("phase", proj.Status.Phase),
			zenlog.String("cluster_ref", proj.Spec.ClusterRef),
		)
	}

	if err := r.Status().Update(ctx, &proj); err != nil {
		// TODO: Fix zenlog.Error signature
		// logger.Error(err, "Failed to update ZenProject status",
		// 	zenlog.Operation("status_update"),
		// )
		logger.Error("Failed to update ZenProject status",
			zenlog.Error(err),
			zenlog.Operation("status_update"),
		)
		return ctrl.Result{}, err
	}

	logger.Info("ZenProject reconciliation completed",
		zenlog.String("phase", proj.Status.Phase),
		zenlog.Int64("observed_generation", proj.Status.ObservedGeneration),
	)

	return ctrl.Result{}, nil
}

// SetupWithManager registers reconciler with manager.
func (r *ZenProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ZenProject{}).
		Complete(r)
}
