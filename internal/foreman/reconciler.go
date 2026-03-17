// Package foreman implements the Foreman controller (Block 4.2).
// It reconciles BrainTask resources: schedules tasks and updates status.
package foreman

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/pkg/gate"
	"github.com/kube-zen/zen-brain1/pkg/guardian"
	"github.com/kube-zen/zen-brain1/pkg/policy"
)

// Reconciler reconciles BrainTask resources (Block 4.2 Foreman).
type Reconciler struct {
	client.Client
	// Gate is optional; when set, Admit is called before scheduling. Block 4.6.
	Gate gate.ZenGate
	// Guardian is optional; when set, CheckSafety before scheduling and RecordEvent after (Block 4.7).
	Guardian guardian.ZenGuardian
	// Dispatcher is optional; when set, Dispatch is called after scheduling. Block 4.3.
	Dispatcher TaskDispatcher
}

// Reconcile performs the reconciliation loop for a BrainTask.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	defer func() { ReconcileDurationSeconds.Observe(time.Since(start).Seconds()) }()
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
		// Handle stuck "Running" tasks (worker crashed before updating status)
		if task.Status.Phase == v1alpha1.BrainTaskPhaseRunning {
			// Check if task is stale (> 5 minutes in Running without completion)
			scheduledCondition := findCondition(task.Status.Conditions, "Scheduled")
			if scheduledCondition != nil {
				scheduledTime := scheduledCondition.LastTransitionTime.Time
				staleDuration := time.Since(scheduledTime)
				if staleDuration > 5*time.Minute {
					logger.Info("Stale Running task detected, re-dispatching", "task", task.Name, "running_for", staleDuration)
					if r.Dispatcher != nil {
						if err := r.Dispatcher.Dispatch(ctx, &task); err != nil {
							logger.Error(err, "re-dispatch failed for stale task", "task", task.Name)
							return ctrl.Result{}, err
						}
						TasksDispatchedTotal.Inc()
					}
					// Re-check in 1 minute
					return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
				}
			}
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

	// Pending -> Scheduled: optional queue check, admission, then status update and optional dispatch.
	if task.Status.Phase == v1alpha1.BrainTaskPhasePending {
		if task.Spec.QueueName != "" {
			var queue v1alpha1.BrainQueue
			if err := r.Get(ctx, client.ObjectKey{Namespace: task.Namespace, Name: task.Spec.QueueName}, &queue); err == nil && queue.Status.Phase == v1alpha1.BrainQueuePhasePaused {
				logger.Info("queue is paused, requeuing", "task", task.Name, "queue", task.Spec.QueueName)
				return ctrl.Result{Requeue: true}, nil
			}
		}
		if r.Guardian != nil {
			safety, err := r.Guardian.CheckSafety(ctx, task.Spec.SessionID, task.Name, guardian.EventTaskStarted)
			if err != nil {
				logger.Error(err, "guardian safety check failed", "task", task.Name)
				return ctrl.Result{}, err
			}
			if !safety.Allowed {
				task.Status.Conditions = append(task.Status.Conditions, metav1.Condition{
					Type: "GuardianBlocked", Status: metav1.ConditionTrue, Reason: "GuardianSafety",
					Message: safety.Reason, LastTransitionTime: metav1.Now(), ObservedGeneration: task.Generation,
				})
				_ = r.Status().Update(ctx, &task)
				return ctrl.Result{Requeue: true}, nil
			}
		}
		if r.Gate != nil {
			admissionReq := gate.AdmissionRequest{
				RequestID:   string(task.UID),
				WorkItemID:  task.Spec.WorkItemID,
				SessionID:   task.Spec.SessionID,
				TaskID:      task.Name,
				Action:      policy.ActionExecuteTask,
				Resource:    policy.Resource{Type: "task", ID: task.Name, Attributes: map[string]interface{}{"estimated_cost_usd": task.Spec.EstimatedCostUSD}},
				Subject:     policy.Subject{Type: "system", ID: "foreman"},
				Payload:     map[string]interface{}{"model_id": task.Annotations["zen.kube-zen.com/planned-model"]},
				Timestamp:   time.Now(),
			}
			resp, err := r.Gate.Admit(ctx, admissionReq)
			if err != nil {
				logger.Error(err, "admission check failed", "task", task.Name)
				return ctrl.Result{}, err
			}
			if resp != nil && !resp.Allowed {
				TasksAdmissionDeniedTotal.Inc()
				task.Status.Conditions = append(task.Status.Conditions, metav1.Condition{
					Type:               "AdmissionDenied",
					Status:             metav1.ConditionTrue,
					Reason:             "GateDenied",
					Message:            resp.Reason,
					LastTransitionTime: metav1.Now(),
					ObservedGeneration: task.Generation,
				})
				_ = r.Status().Update(ctx, &task)
				return ctrl.Result{Requeue: true}, nil
			}
		}

		task.Status.Phase = v1alpha1.BrainTaskPhaseScheduled
		task.Status.Message = "Scheduled by Foreman"
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
		if r.Guardian != nil {
			_ = r.Guardian.RecordEvent(ctx, guardian.Event{Kind: guardian.EventTaskStarted, SessionID: task.Spec.SessionID, TaskID: task.Name, At: time.Now()})
		}
		TasksScheduledTotal.Inc()
		if r.Dispatcher != nil {
			if err := r.Dispatcher.Dispatch(ctx, &task); err != nil {
				logger.Error(err, "dispatch failed", "task", task.Name)
				// Task remains Scheduled; caller may retry or handle
			}
		}
		logger.Info("BrainTask scheduled", "name", task.Name)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// findCondition finds a condition by type in the conditions slice.
func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// SetupWithManager registers the reconciler with the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BrainTask{}).
		Complete(r)
}
