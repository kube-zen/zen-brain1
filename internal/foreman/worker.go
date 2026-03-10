// Package foreman implements a worker pool that runs BrainTasks (Block 4.3).
package foreman

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
)

// Worker is a TaskDispatcher that runs tasks in a pool of goroutines and updates status (Block 4.3).
type Worker struct {
	Client     client.Client
	Runner     TaskRunner
	NumWorkers int
	queue      chan types.NamespacedName
	startOnce  sync.Once
	stop       func()
}

// NewWorker returns a Worker that will process up to numWorkers tasks concurrently.
// Call Start(ctx) before using Dispatch.
func NewWorker(c client.Client, runner TaskRunner, numWorkers int) *Worker {
	if numWorkers < 1 {
		numWorkers = 1
	}
	return &Worker{
		Client:     c,
		Runner:     runner,
		NumWorkers: numWorkers,
		queue:      make(chan types.NamespacedName, 256),
	}
}

// Start begins the worker pool. Call once before Dispatch. Cancels when ctx is done.
func (w *Worker) Start(ctx context.Context) {
	w.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(ctx)
		w.stop = cancel
		for i := 0; i < w.NumWorkers; i++ {
			go w.runLoop(ctx)
		}
	})
}

// Stop stops the worker pool (no more Dispatch accepted; in-flight tasks still complete).
func (w *Worker) Stop() {
	if w.stop != nil {
		w.stop()
	}
}

// Dispatch implements TaskDispatcher. It enqueues the task for processing by the pool.
func (w *Worker) Dispatch(ctx context.Context, task *v1alpha1.BrainTask) error {
	nn := types.NamespacedName{Namespace: task.Namespace, Name: task.Name}
	select {
	case w.queue <- nn:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Worker) runLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case nn, ok := <-w.queue:
			if !ok {
				return
			}
			w.processOne(ctx, nn)
		}
	}
}

func (w *Worker) processOne(ctx context.Context, nn types.NamespacedName) {
	logger := log.FromContext(ctx)
	var task v1alpha1.BrainTask
	if err := w.Client.Get(ctx, nn, &task); err != nil {
		if errors.IsNotFound(err) {
			return
		}
		logger.Error(err, "get task for execution", "task", nn.String())
		return
	}
	// Only run if still Scheduled (avoid double-run if reconciler retried).
	if task.Status.Phase != v1alpha1.BrainTaskPhaseScheduled {
		return
	}

	// Patch to Running
	task.Status.Phase = v1alpha1.BrainTaskPhaseRunning
	task.Status.Message = "Running"
	task.Status.ObservedGeneration = task.Generation
	if err := w.Client.Status().Update(ctx, &task); err != nil {
		logger.Error(err, "update task to Running", "task", nn.String())
		return
	}

	// Execute
	err := w.Runner.Run(ctx, &task)
	if err != nil {
		task.Status.Phase = v1alpha1.BrainTaskPhaseFailed
		task.Status.Message = err.Error()
	} else {
		task.Status.Phase = v1alpha1.BrainTaskPhaseCompleted
		task.Status.Message = "Completed"
	}
	task.Status.Conditions = append(task.Status.Conditions, metav1.Condition{
		Type:               "Executed",
		Status:             metav1.ConditionTrue,
		Reason:             "WorkerRun",
		Message:            task.Status.Message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: task.Generation,
	})
	if patchErr := w.Client.Status().Update(ctx, &task); patchErr != nil {
		logger.Error(patchErr, "update task to Completed/Failed", "task", nn.String())
	}
}

// Ensure Worker implements TaskDispatcher.
var _ TaskDispatcher = (*Worker)(nil)
