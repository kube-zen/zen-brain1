// Package foreman implements the Foreman controller (Block 4.2).
// It reconciles BrainTask resources: schedules tasks and updates status.
package foreman

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

// Reconciler reconciles BrainTask resources (Block 4.2 Foreman).
type Reconciler struct {
	client.Client
}

// Reconcile performs the reconciliation loop for a BrainTask.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var task v1alpha1.BrainTask
	if err := r.Get(ctx, req.NamespacedName, &task); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Observed generation
	if task.Status.ObservedGeneration == task.Generation {
		// Already reconciled this generation
		if task.Status.Phase == v1alpha1.BrainTaskPhaseCompleted || task.Status.Phase == v1alpha1.BrainTaskPhaseFailed || task.Status.Phase == v1alpha1.BrainTaskPhaseCanceled {
			return ctrl.Result{}, nil
		}
	}

	// Default phase to Pending if unset
	if task.Status.Phase == "" {
		task.Status.Phase = v1alpha1.BrainTaskPhasePending
		task.Status.ObservedGeneration = task.Generation
		if err := r.Status().Update(ctx, &task); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("BrainTask pending", "name", task.Name, "sessionID", task.Spec.SessionID)
		return ctrl.Result{}, nil
	}

	// For now: transition Pending -> Scheduled (actual worker dispatch is Block 4.3+).
	if task.Status.Phase == v1alpha1.BrainTaskPhasePending {
		task.Status.Phase = v1alpha1.BrainTaskPhaseScheduled
		task.Status.Message = "Scheduled by Foreman (worker dispatch not yet implemented)"
		task.Status.ObservedGeneration = task.Generation
		task.Status.Conditions = append(task.Status.Conditions, metav1.Condition{
			Type:               "Scheduled",
			Status:             metav1.ConditionTrue,
			Reason:             "ForemanScheduled",
			Message:            "Scheduled by Foreman",
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: task.Generation,
		})
		if err := r.Status().Update(ctx, &task); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("BrainTask scheduled", "name", task.Name)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler with the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BrainTask{}).
		Complete(r)
}
