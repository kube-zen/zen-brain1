// Package foreman implements a worker pool that runs BrainTasks (Block 4.3).
package foreman

import (
	"context"
	"fmt"
	"log"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

// Worker is a TaskDispatcher that runs tasks in a pool of goroutines and updates status (Block 4.3).
type Worker struct {
	Client          client.Client
	Runner          TaskRunner
	NumWorkers      int
	SessionAffinity bool   // when true, route tasks by session to the same worker (Block 4 session-affinity)
	// ContextBinder optional: when set and Runner implements TaskRunnerWithContext, get session before run and write intermediate state after (Block 5.3).
	ContextBinder ContextBinder
	// LedgerClient optional: when set, record task completion in ZenLedger (Block 4 completeness) for cost/audit visibility.
	LedgerClient ledger.ZenLedgerClient
	queue        chan types.NamespacedName   // used when !SessionAffinity
	queues       []chan types.NamespacedName // used when SessionAffinity (one per worker)
	affinityMu   sync.Mutex
	sessionToWorker map[string]int // sessionID -> worker index
	workerLoad     []int          // in-flight + queued per worker (for least-loaded)
	startOnce      sync.Once
	stop           func()
}

// NewWorker returns a Worker that will process up to numWorkers tasks concurrently.
// Call Start(ctx) before using Dispatch. Set SessionAffinity true to route by session (same session → same worker).
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
		if w.SessionAffinity {
			w.queues = make([]chan types.NamespacedName, w.NumWorkers)
			w.sessionToWorker = make(map[string]int)
			w.workerLoad = make([]int, w.NumWorkers)
			for i := 0; i < w.NumWorkers; i++ {
				w.queues[i] = make(chan types.NamespacedName, 64)
			}
			for i := 0; i < w.NumWorkers; i++ {
				idx := i
				go w.runLoopFrom(ctx, w.queues[idx], idx)
			}
		} else {
			for i := 0; i < w.NumWorkers; i++ {
				go w.runLoop(ctx, w.queue)
			}
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
	if w.SessionAffinity && w.queues != nil {
		idx := w.sessionWorkerIndex(task)
		ch := w.queues[idx]
		select {
		case ch <- nn:
			TasksDispatchedTotal.Inc()
			w.setQueueDepth()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	select {
	case w.queue <- nn:
		TasksDispatchedTotal.Inc()
		WorkerQueueDepth.Set(float64(len(w.queue)))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// sessionWorkerIndex returns the worker index for this task (session-affinity: same session → same worker).
func (w *Worker) sessionWorkerIndex(task *v1alpha1.BrainTask) int {
	sessionID := task.Spec.SessionID
	if sessionID == "" {
		sessionID = task.Namespace + "/" + task.Name
	}
	w.affinityMu.Lock()
	defer w.affinityMu.Unlock()
	if idx, ok := w.sessionToWorker[sessionID]; ok {
		w.workerLoad[idx]++
		return idx
	}
	// Assign least-loaded worker
	idx := 0
	for i := 1; i < w.NumWorkers; i++ {
		if w.workerLoad[i] < w.workerLoad[idx] {
			idx = i
		}
	}
	w.sessionToWorker[sessionID] = idx
	w.workerLoad[idx]++
	return idx
}

func (w *Worker) setQueueDepth() {
	if w.SessionAffinity && w.queues != nil {
		var n int
		for _, ch := range w.queues {
			n += len(ch)
		}
		WorkerQueueDepth.Set(float64(n))
	}
}

func (w *Worker) runLoop(ctx context.Context, ch chan types.NamespacedName) {
	for {
		select {
		case <-ctx.Done():
			return
		case nn, ok := <-ch:
			if !ok {
				return
			}
			WorkerQueueDepth.Set(float64(len(w.queue)))
			w.processOne(ctx, nn)
		}
	}
}

func (w *Worker) runLoopFrom(ctx context.Context, ch chan types.NamespacedName, workerIdx int) {
	for {
		select {
		case <-ctx.Done():
			return
		case nn, ok := <-ch:
			if !ok {
				return
			}
			w.setQueueDepth()
			w.processOne(ctx, nn)
			w.affinityMu.Lock()
			if w.workerLoad[workerIdx] > 0 {
				w.workerLoad[workerIdx]--
			}
			w.affinityMu.Unlock()
		}
	}
}

func (w *Worker) processOne(ctx context.Context, nn types.NamespacedName) {
	logger := ctrllog.FromContext(ctx)
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

	// Execute: use RunWithContext when binder is set and runner supports it (Block 5.3 agent-context binding)
	var err error
	var outcome *TaskRunOutcome
	if w.ContextBinder != nil {
		if runnerWithCtx, ok := w.Runner.(TaskRunnerWithContext); ok {
			var sessionCtx *zenctx.SessionContext
			clusterID := "default"
			sessionCtx, _ = w.ContextBinder.GetForContinuation(ctx, clusterID, task.Spec.SessionID, task.Name)
			var updated *zenctx.SessionContext
			updated, outcome, err = runnerWithCtx.RunWithContext(ctx, &task, sessionCtx)
			if err == nil && updated != nil {
				_ = w.ContextBinder.WriteIntermediate(ctx, clusterID, updated)
			}
		} else {
			outcome, err = w.Runner.Run(ctx, &task)
		}
	} else {
		outcome, err = w.Runner.Run(ctx, &task)
	}
	if err != nil {
		TasksFailedTotal.Inc()
		task.Status.Phase = v1alpha1.BrainTaskPhaseFailed
		task.Status.Message = err.Error()
	} else {
		TasksCompletedTotal.Inc()
		task.Status.Phase = v1alpha1.BrainTaskPhaseCompleted
		task.Status.Message = "Completed"
	}
	// Record task outcome in ZenLedger when configured (Block 4 completeness)
	if w.LedgerClient != nil {
		sessionID := task.Spec.SessionID
		if sessionID == "" {
			sessionID = task.Namespace + "/" + task.Name
		}
		reason := "completed"
		if task.Status.Phase == v1alpha1.BrainTaskPhaseFailed {
			reason = "failed"
		}
		if recordErr := w.LedgerClient.RecordPlannedModelSelection(ctx, sessionID, task.Name, "factory", reason); recordErr != nil {
			log.Printf("[Worker] ledger RecordPlannedModelSelection: %v", recordErr)
		}
	}
	task.Status.Conditions = append(task.Status.Conditions, metav1.Condition{
		Type:               "Executed",
		Status:             metav1.ConditionTrue,
		Reason:             "WorkerRun",
		Message:            task.Status.Message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: task.Generation,
	})
	// Persist run outcome to annotations when available (Block 4 factory execution)
	if outcome != nil {
		base := task.DeepCopy()
		if task.Annotations == nil {
			task.Annotations = make(map[string]string)
		}
		if outcome.WorkspacePath != "" {
			task.Annotations["zen.kube-zen.com/factory-workspace"] = outcome.WorkspacePath
		}
		if outcome.ProofOfWorkPath != "" {
			task.Annotations["zen.kube-zen.com/factory-proof"] = outcome.ProofOfWorkPath
		}
		if outcome.TemplateKey != "" {
			task.Annotations["zen.kube-zen.com/factory-template"] = outcome.TemplateKey
		}
		if outcome.FilesChanged > 0 {
			task.Annotations["zen.kube-zen.com/factory-files-changed"] = fmt.Sprintf("%d", outcome.FilesChanged)
		}
		if outcome.DurationSeconds > 0 {
			task.Annotations["zen.kube-zen.com/factory-duration-seconds"] = fmt.Sprintf("%d", outcome.DurationSeconds)
		}
		if outcome.Recommendation != "" {
			task.Annotations["zen.kube-zen.com/factory-recommendation"] = outcome.Recommendation
		}
		if outcome.ExecutionMode != "" {
			task.Annotations["zen.kube-zen.com/factory-execution-mode"] = outcome.ExecutionMode
		}
		if patchErr := w.Client.Patch(ctx, &task, client.MergeFrom(base)); patchErr != nil {
			logger.Error(patchErr, "patch task annotations with outcome", "task", nn.String())
		}
	}
	if patchErr := w.Client.Status().Update(ctx, &task); patchErr != nil {
		logger.Error(patchErr, "update task to Completed/Failed", "task", nn.String())
	}
}

// Ensure Worker implements TaskDispatcher.
var _ TaskDispatcher = (*Worker)(nil)
