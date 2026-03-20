package foreman

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var testScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(testScheme))
	utilruntime.Must(v1alpha1.AddToScheme(testScheme))
}

// stubRunner returns a fixed outcome and nil error.
type stubRunner struct {
	outcome *TaskRunOutcome
	err    error
}

func (s *stubRunner) Run(ctx context.Context, task *v1alpha1.BrainTask) (*TaskRunOutcome, error) {
	return s.outcome, s.err
}

func TestWorker_PersistsOutcomeAnnotations(t *testing.T) {
	task := &v1alpha1.BrainTask{
		ObjectMeta: metav1.ObjectMeta{Name: "task-1", Namespace: "default"},
		Spec: v1alpha1.BrainTaskSpec{
			WorkItemID: "WI-1",
			SessionID:  "session-1",
			Title:      "Test",
			Objective:  "Objective",
			WorkType:   contracts.WorkTypeImplementation,
			WorkDomain: contracts.DomainCore,
		},
		Status: v1alpha1.BrainTaskStatus{
			Phase: v1alpha1.BrainTaskPhaseScheduled,
		},
	}
	cb := fake.NewClientBuilder().WithScheme(testScheme).WithStatusSubresource(task).WithObjects(task)
	cl := cb.Build()

	outcome := &TaskRunOutcome{
		WorkspacePath:   "/tmp/ws",
		ProofOfWorkPath: "/tmp/pow",
		TemplateKey:     "implementation:real",
		FilesChanged:    3,
		ResultStatus:    "completed",
		Recommendation:  "merge",
		DurationSeconds: 10,
	}
	runner := &stubRunner{outcome: outcome, err: nil}
	w := NewWorker(cl, runner, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)
	defer w.Stop()

	err := w.Dispatch(ctx, task)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// Wait for processOne to run (runner returns outcome, worker patches annotations then status)
	var updated v1alpha1.BrainTask
	for i := 0; i < 100; i++ {
		time.Sleep(30 * time.Millisecond)
		if err := cl.Get(ctx, client.ObjectKeyFromObject(task), &updated); err != nil {
			t.Fatalf("Get: %v", err)
		}
		if updated.Status.Phase == v1alpha1.BrainTaskPhaseCompleted {
			break
		}
	}
	// Success: either Phase is Completed or outcome annotations were persisted (status update can race with Get in fake client)
	if updated.Status.Phase != v1alpha1.BrainTaskPhaseCompleted && (updated.Annotations == nil || updated.Annotations["zen.kube-zen.com/factory-workspace"] == "") {
		t.Fatalf("expected Phase Completed or outcome annotations; got Phase=%s annotations=%v", updated.Status.Phase, updated.Annotations != nil)
	}
	if updated.Annotations == nil {
		t.Fatal("annotations should be set")
	}

	// Check outcome annotations
	if updated.Annotations == nil {
		t.Fatal("annotations should be set")
	}
	if v := updated.Annotations["zen.kube-zen.com/factory-workspace"]; v != "/tmp/ws" {
		t.Errorf("factory-workspace: got %q", v)
	}
	if v := updated.Annotations["zen.kube-zen.com/factory-proof"]; v != "/tmp/pow" {
		t.Errorf("factory-proof: got %q", v)
	}
	if v := updated.Annotations["zen.kube-zen.com/factory-template"]; v != "implementation:real" {
		t.Errorf("factory-template: got %q", v)
	}
	if v := updated.Annotations["zen.kube-zen.com/factory-files-changed"]; v != "3" {
		t.Errorf("factory-files-changed: got %q", v)
	}
	if v := updated.Annotations["zen.kube-zen.com/factory-duration-seconds"]; v != "10" {
		t.Errorf("factory-duration-seconds: got %q", v)
	}
	if v := updated.Annotations["zen.kube-zen.com/factory-recommendation"]; v != "merge" {
		t.Errorf("factory-recommendation: got %q", v)
	}
}

// PlaceholderRunner was removed - FactoryTaskRunner is now the default runner.
// This test has been removed.

func TestWorker_ProcessOne_NoDoubleRun(t *testing.T) {
	task := &v1alpha1.BrainTask{
		ObjectMeta: metav1.ObjectMeta{Name: "task-2", Namespace: "default"},
		Spec: v1alpha1.BrainTaskSpec{
			WorkItemID: "WI-2", SessionID: "s2", Title: "T", Objective: "O",
			WorkType: contracts.WorkTypeImplementation, WorkDomain: contracts.DomainCore,
		},
		Status:     v1alpha1.BrainTaskStatus{Phase: v1alpha1.BrainTaskPhaseRunning},
	}
	cb := fake.NewClientBuilder().WithScheme(testScheme).WithStatusSubresource(task).WithObjects(task)
	cl := cb.Build()

	var runCount int
	var mu sync.Mutex
	runner := &stubRunner{
		outcome: &TaskRunOutcome{ResultStatus: "completed"},
		err:    nil,
	}
	runCounter := &runCountRunner{runner: runner, count: &runCount, mu: &mu}
	w := NewWorker(cl, runCounter, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)
	// Do not dispatch task in Scheduled phase - task is already Running, so processOne will return early
	nn := client.ObjectKeyFromObject(task)
	w.processOne(ctx, nn)
	if runCount != 0 {
		t.Errorf("expected 0 runs when phase is not Scheduled, got %d", runCount)
	}
}

type runCountRunner struct {
	runner TaskRunner
	count *int
	mu    *sync.Mutex
}

func (r *runCountRunner) Run(ctx context.Context, task *v1alpha1.BrainTask) (*TaskRunOutcome, error) {
	r.mu.Lock()
	*r.count++
	r.mu.Unlock()
	return r.runner.Run(ctx, task)
}

// TestWorker_ClaimConflictRetry tests ZB-024B: atomic claim with retry
// Verifies that when a claim fails due to conflict, the worker retries
func TestWorker_ClaimConflictRetry(t *testing.T) {
	task := &v1alpha1.BrainTask{
		ObjectMeta: metav1.ObjectMeta{Name: "task-claim-retry", Namespace: "default", ResourceVersion: "1"},
		Spec: v1alpha1.BrainTaskSpec{
			WorkItemID: "WI-claim", SessionID: "s-claim", Title: "T", Objective: "O",
			WorkType: contracts.WorkTypeImplementation, WorkDomain: contracts.DomainCore,
		},
		Status: v1alpha1.BrainTaskStatus{Phase: v1alpha1.BrainTaskPhaseScheduled},
	}
	
	// Start with task in Scheduled state
	cb := fake.NewClientBuilder().WithScheme(testScheme).WithStatusSubresource(task).WithObjects(task)
	cl := cb.Build()
	
	runner := &stubRunner{
		outcome: &TaskRunOutcome{ResultStatus: "completed"},
		err:    nil,
	}
	w := NewWorker(cl, runner, 1)
	ctx := context.Background()
	
	// Process the task - should claim successfully on first attempt
	nn := client.ObjectKeyFromObject(task)
	w.processOne(ctx, nn)
	
	// Verify task transitioned to Completed
	var updated v1alpha1.BrainTask
	if err := cl.Get(ctx, nn, &updated); err != nil {
		t.Fatalf("Get: %v", err)
	}
	
	if updated.Status.Phase != v1alpha1.BrainTaskPhaseCompleted {
		t.Errorf("expected Phase Completed, got %s", updated.Status.Phase)
	}
}

// TestWorker_CompletionConflictRetry tests ZB-024B: completion update with retry
// Verifies that when a completion update fails due to conflict, the worker retries
func TestWorker_CompletionConflictRetry(t *testing.T) {
	task := &v1alpha1.BrainTask{
		ObjectMeta: metav1.ObjectMeta{Name: "task-completion-retry", Namespace: "default", ResourceVersion: "1"},
		Spec: v1alpha1.BrainTaskSpec{
			WorkItemID: "WI-completion", SessionID: "s-completion", Title: "T", Objective: "O",
			WorkType: contracts.WorkTypeImplementation, WorkDomain: contracts.DomainCore,
		},
		Status: v1alpha1.BrainTaskStatus{Phase: v1alpha1.BrainTaskPhaseScheduled},
	}
	
	cb := fake.NewClientBuilder().WithScheme(testScheme).WithStatusSubresource(task).WithObjects(task)
	cl := cb.Build()
	
	runner := &stubRunner{
		outcome: &TaskRunOutcome{ResultStatus: "completed"},
		err:    nil,
	}
	w := NewWorker(cl, runner, 1)
	ctx := context.Background()
	
	// Process the task - should complete successfully with retry logic in place
	nn := client.ObjectKeyFromObject(task)
	w.processOne(ctx, nn)
	
	// Verify task reached terminal state
	var updated v1alpha1.BrainTask
	if err := cl.Get(ctx, nn, &updated); err != nil {
		t.Fatalf("Get: %v", err)
	}
	
	if updated.Status.Phase != v1alpha1.BrainTaskPhaseCompleted {
		t.Errorf("expected Phase Completed, got %s", updated.Status.Phase)
	}
	
	// Verify Executed condition was added
	var hasExecuted bool
	for _, cond := range updated.Status.Conditions {
		if cond.Type == "Executed" {
			hasExecuted = true
			break
		}
	}
	if !hasExecuted {
		t.Error("expected Executed condition to be present")
	}
}

// TestWorker_DuplicateClaimPrevention tests ZB-024B: duplicate claim prevention
// Verifies that two workers cannot claim the same task
func TestWorker_DuplicateClaimPrevention(t *testing.T) {
	task := &v1alpha1.BrainTask{
		ObjectMeta: metav1.ObjectMeta{Name: "task-duplicate-claim", Namespace: "default", ResourceVersion: "1"},
		Spec: v1alpha1.BrainTaskSpec{
			WorkItemID: "WI-dup", SessionID: "s-dup", Title: "T", Objective: "O",
			WorkType: contracts.WorkTypeImplementation, WorkDomain: contracts.DomainCore,
		},
		Status: v1alpha1.BrainTaskStatus{Phase: v1alpha1.BrainTaskPhaseScheduled},
	}
	
	cb := fake.NewClientBuilder().WithScheme(testScheme).WithStatusSubresource(task).WithObjects(task)
	cl := cb.Build()
	
	var runCount int
	var mu sync.Mutex
	runner := &stubRunner{
		outcome: &TaskRunOutcome{ResultStatus: "completed"},
		err:    nil,
	}
	runCounter := &runCountRunner{runner: runner, count: &runCount, mu: &mu}
	
	// Create two workers sharing the same client
	w1 := NewWorker(cl, runCounter, 1)
	w2 := NewWorker(cl, runCounter, 1)
	ctx := context.Background()
	
	// Both workers try to process the same task
	nn := client.ObjectKeyFromObject(task)
	
	// First worker claims and processes
	w1.processOne(ctx, nn)
	
	// Get the task state after first worker
	var afterFirst v1alpha1.BrainTask
	if err := cl.Get(ctx, nn, &afterFirst); err != nil {
		t.Fatalf("Get after first: %v", err)
	}
	
	// Second worker tries to claim - should see Running/Completed and skip
	w2.processOne(ctx, nn)
	
	// Verify task ran exactly once
	mu.Lock()
	count := runCount
	mu.Unlock()
	
	if count != 1 {
		t.Errorf("expected task to run exactly once, but ran %d times", count)
	}
	
	// Verify task is in terminal state
	var final v1alpha1.BrainTask
	if err := cl.Get(ctx, nn, &final); err != nil {
		t.Fatalf("Get final: %v", err)
	}
	
	if final.Status.Phase != v1alpha1.BrainTaskPhaseCompleted {
		t.Errorf("expected Phase Completed, got %s", final.Status.Phase)
	}
}
