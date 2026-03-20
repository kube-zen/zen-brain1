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
	// ClusterID is the cluster identifier for session/context lookups (default: "default").
	ClusterID string
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
	log.Printf("[Worker.Start] ENTRY - before startOnce")
	w.startOnce.Do(func() {
		log.Printf("[Worker.Start] startOnce function executing (numWorkers=%d, sessionAffinity=%v)", w.NumWorkers, w.SessionAffinity)
		ctx, cancel := context.WithCancel(ctx)
		w.stop = cancel
		if w.SessionAffinity {
			log.Printf("[Worker.Start] session-affinity mode: creating %d worker channels", w.NumWorkers)
			w.queues = make([]chan types.NamespacedName, w.NumWorkers)
			w.sessionToWorker = make(map[string]int)
			w.workerLoad = make([]int, w.NumWorkers)
			for i := 0; i < w.NumWorkers; i++ {
				w.queues[i] = make(chan types.NamespacedName, 64)
			}
			for i := 0; i < w.NumWorkers; i++ {
				idx := i
				log.Printf("[Worker.Start] starting worker goroutine %d (session-affinity)", idx)
				go w.runLoopFrom(ctx, w.queues[idx], idx)
			}
			log.Printf("[Worker.Start] all %d session-affinity worker goroutines started", w.NumWorkers)
		} else {
			log.Printf("[Worker.Start] round-robin mode: creating shared queue (capacity=64)")
			w.queue = make(chan types.NamespacedName, 64)
			for i := 0; i < w.NumWorkers; i++ {
				log.Printf("[Worker.Start] starting worker goroutine %d (round-robin)", i)
				go w.runLoop(ctx, w.queue)
			}
			log.Printf("[Worker.Start] all %d round-robin worker goroutines started", w.NumWorkers)
		}
	})
	log.Printf("[Worker.Start] worker pool started successfully")
}

// Stop stops the worker pool (no more Dispatch accepted; in-flight tasks still complete).
func (w *Worker) Stop() {
	if w.stop != nil {
		w.stop()
	}
}

// Dispatch implements TaskDispatcher. It enqueues the task for processing by the pool.
func (w *Worker) Dispatch(ctx context.Context, task *v1alpha1.BrainTask) error {
	log.Printf("[Worker.Dispatch] dispatching task %s (session=%s, affinity=%v)", task.Name, task.Spec.SessionID, w.SessionAffinity)
	nn := types.NamespacedName{Namespace: task.Namespace, Name: task.Name}
	if w.SessionAffinity && w.queues != nil {
		idx := w.sessionWorkerIndex(task)
		ch := w.queues[idx]
		log.Printf("[Worker.Dispatch] session-affinity mode, sending task %s to worker %d (channel depth=%d)", task.Name, idx, len(ch))
		select {
		case ch <- nn:
			log.Printf("[Worker.Dispatch] task %s sent to worker %d channel successfully", task.Name, idx)
			TasksDispatchedTotal.Inc()
			w.setQueueDepth()
			return nil
		case <-ctx.Done():
			log.Printf("[Worker.Dispatch] task %s dispatch cancelled (context done)", task.Name)
			return ctx.Err()
		}
	}
	log.Printf("[Worker.Dispatch] round-robin mode, sending task %s to pool (queue depth=%d)", task.Name, len(w.queue))
	select {
	case w.queue <- nn:
		log.Printf("[Worker.Dispatch] task %s sent to pool queue successfully (queue depth=%d)", task.Name, len(w.queue))
		TasksDispatchedTotal.Inc()
		WorkerQueueDepth.Set(float64(len(w.queue)))
		return nil
	case <-ctx.Done():
		log.Printf("[Worker.Dispatch] task %s dispatch cancelled (context done)", task.Name)
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

// clusterID returns the configured cluster ID for session/context lookups, or "default".
func (w *Worker) clusterID() string {
	if w.ClusterID != "" {
		return w.ClusterID
	}
	return "default"
}

func (w *Worker) runLoop(ctx context.Context, ch chan types.NamespacedName) {
	log.Printf("[Worker.runLoop] starting worker goroutine (round-robin mode)")
	for {
		select {
		case <-ctx.Done():
			log.Printf("[Worker.runLoop] worker goroutine exiting (context done)")
			return
		case nn, ok := <-ch:
			if !ok {
				log.Printf("[Worker.runLoop] worker goroutine exiting (channel closed)")
				return
			}
			log.Printf("[Worker.runLoop] received task %s from channel (queue depth=%d)", nn.String(), len(ch))
			WorkerQueueDepth.Set(float64(len(w.queue)))
			w.processOne(ctx, nn)
		}
	}
}

func (w *Worker) runLoopFrom(ctx context.Context, ch chan types.NamespacedName, workerIdx int) {
	log.Printf("[Worker.runLoopFrom] starting worker goroutine %d (session-affinity mode)", workerIdx)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[Worker.runLoopFrom] worker goroutine %d exiting (context done)", workerIdx)
			return
		case nn, ok := <-ch:
			if !ok {
				log.Printf("[Worker.runLoopFrom] worker goroutine %d exiting (channel closed)", workerIdx)
				return
			}
			log.Printf("[Worker.runLoopFrom] worker %d received task %s from channel (queue depth=%d)", workerIdx, nn.String(), len(ch))
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
	log.Printf("[Worker.processOne] processing task %s", nn.String())
	logger := ctrllog.FromContext(ctx)
	
	// ZB-024B: Atomic claim with optimistic concurrency
	// Retry loop handles race between dispatch and state visibility
	var task v1alpha1.BrainTask
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := w.Client.Get(ctx, nn, &task); err != nil {
			if errors.IsNotFound(err) {
				log.Printf("[Worker.processOne] task %s not found (was deleted)", nn.String())
				return
			}
			log.Printf("[Worker.processOne] error getting task %s: %v", nn.String(), err)
			logger.Error(err, "get task for execution", "task", nn.String())
			return
		}
		
		log.Printf("[Worker.processOne] task %s current phase=%s (attempt %d/%d)", nn.String(), task.Status.Phase, attempt, maxRetries)
		
		// ZB-024B: Only claim if Scheduled OR Pending (reconciler might not have updated yet)
		if task.Status.Phase != v1alpha1.BrainTaskPhaseScheduled && task.Status.Phase != v1alpha1.BrainTaskPhasePending {
			log.Printf("[Worker.processOne] task %s not claimable (phase=%s), skipping", nn.String(), task.Status.Phase)
			return
		}
		
		// ZB-024B: Atomic claim - try to transition to Running
		// This will fail if another worker claimed it or if state changed
		originalPhase := task.Status.Phase
		task.Status.Phase = v1alpha1.BrainTaskPhaseRunning
		task.Status.Message = "Running"
		task.Status.ObservedGeneration = task.Generation
		
		if err := w.Client.Status().Update(ctx, &task); err != nil {
			if errors.IsConflict(err) {
				log.Printf("[Worker.processOne] task %s claim conflict (attempt %d), retrying...", nn.String(), attempt)
				continue // Retry - re-read and try again
			}
			logger.Error(err, "update task to Running", "task", nn.String())
			return
		}
		
		// ZB-024B: Claim succeeded!
		log.Printf("[Worker.processOne] task %s claimed successfully (was %s, now Running)", nn.String(), originalPhase)
		break // Exit retry loop - we own the task now
	}

	log.Printf("[Worker.processOne] task %s calling Runner.Run()", nn.String())
	// Execute: use RunWithContext when binder is set and runner supports it (Block 5.3 agent-context binding)
	var err error
	var outcome *TaskRunOutcome
	if w.ContextBinder != nil {
		if runnerWithCtx, ok := w.Runner.(TaskRunnerWithContext); ok {
			log.Printf("[Worker.processOne] task %s using RunWithContext (binder set, runner supports it)", nn.String())
			var sessionCtx *zenctx.SessionContext
			clusterID := w.clusterID()
			sessionCtx, _ = w.ContextBinder.GetForContinuation(ctx, clusterID, task.Spec.SessionID, task.Name)
			var updated *zenctx.SessionContext
			updated, outcome, err = runnerWithCtx.RunWithContext(ctx, &task, sessionCtx)
			if err == nil && updated != nil {
				_ = w.ContextBinder.WriteIntermediate(ctx, clusterID, updated)
			}
		} else {
			log.Printf("[Worker.processOne] task %s using Run (binder set but runner doesn't support context)", nn.String())
			outcome, err = w.Runner.Run(ctx, &task)
		}
	} else {
		log.Printf("[Worker.processOne] task %s using Run (no binder)", nn.String())
		outcome, err = w.Runner.Run(ctx, &task)
	}
	log.Printf("[Worker.processOne] task %s Runner.Run() returned (err=%v, outcome=%v)", nn.String(), err, outcome != nil)
	if err != nil {
		log.Printf("[Worker.processOne] task %s execution failed: %v", nn.String(), err)
		TasksFailedTotal.Inc()
		task.Status.Phase = v1alpha1.BrainTaskPhaseFailed
		task.Status.Message = err.Error()
	} else {
		log.Printf("[Worker.processOne] task %s execution succeeded, marking Completed", nn.String())
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
	
	// ZB-024B: Re-read task to get latest resource version before final status update
	// This prevents conflicts if reconciler updated the task during execution
	var latestTask v1alpha1.BrainTask
	if getErr := w.Client.Get(ctx, nn, &latestTask); getErr != nil {
		log.Printf("[Worker.processOne] ERROR: failed to re-read task %s before completion: %v", nn.String(), getErr)
		logger.Error(getErr, "re-read task before completion update", "task", nn.String())
		return
	}
	
	// Copy our status changes to the latest task version
	latestTask.Status.Phase = task.Status.Phase
	latestTask.Status.Message = task.Status.Message
	latestTask.Status.Conditions = append(latestTask.Status.Conditions, task.Status.Conditions...)
	
	if patchErr := w.Client.Status().Update(ctx, &latestTask); patchErr != nil {
		log.Printf("[Worker.processOne] ERROR: failed to update task %s to Completed/Failed: %v", nn.String(), patchErr)
		logger.Error(patchErr, "update task to Completed/Failed", "task", nn.String())
		return
	}
	log.Printf("[Worker.processOne] task %s status updated to %s successfully", nn.String(), latestTask.Status.Phase)
}

// Ensure Worker implements TaskDispatcher.
var _ TaskDispatcher = (*Worker)(nil)
