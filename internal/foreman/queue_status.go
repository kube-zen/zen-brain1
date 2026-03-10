// Package foreman provides a reconciler that updates BrainQueue status from BrainTask counts (Block 4).
package foreman

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

// QueueStatusReconciler reconciles BrainQueue to set Status.Depth and Status.InFlight from BrainTask counts.
type QueueStatusReconciler struct {
	client.Client
}

// Reconcile updates the BrainQueue status by counting BrainTasks with spec.queueName == queue.Name.
func (r *QueueStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var queue v1alpha1.BrainQueue
	if err := r.Get(ctx, req.NamespacedName, &queue); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	var taskList v1alpha1.BrainTaskList
	if err := r.List(ctx, &taskList, client.InNamespace(queue.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	var depth, inFlight int32
	for i := range taskList.Items {
		t := &taskList.Items[i]
		if t.Spec.QueueName != queue.Name {
			continue
		}
		switch t.Status.Phase {
		case v1alpha1.BrainTaskPhasePending:
			depth++
		case v1alpha1.BrainTaskPhaseScheduled, v1alpha1.BrainTaskPhaseRunning:
			inFlight++
		}
	}

	if queue.Status.Depth == depth && queue.Status.InFlight == inFlight {
		return ctrl.Result{}, nil
	}
	queue.Status.Depth = depth
	queue.Status.InFlight = inFlight
	if queue.Status.Phase == "" {
		queue.Status.Phase = v1alpha1.BrainQueuePhaseReady
	}
	if err := r.Status().Update(ctx, &queue); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler and watches BrainTask to enqueue the affected BrainQueue.
func (r *QueueStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BrainQueue{}).
		Watches(&v1alpha1.BrainTask{}, handler.EnqueueRequestsFromMapFunc(queueForTask)).
		Complete(r)
}

func queueForTask(ctx context.Context, obj client.Object) []reconcile.Request {
	task, ok := obj.(*v1alpha1.BrainTask)
	if !ok || task.Spec.QueueName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: task.Namespace, Name: task.Spec.QueueName}}}
}
