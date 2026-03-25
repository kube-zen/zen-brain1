package mlq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// TaskAttempt records a single execution attempt against an MLQ level.
type TaskAttempt struct {
	AttemptID       int       `json:"attempt_id"`
	Level           int       `json:"level"`
	WorkerEndpoint  string    `json:"worker_endpoint"`
	RetryCount      int       `json:"retry_count"`
	EscalationCount int       `json:"escalation_count"`
	FallbackCount   int       `json:"fallback_count"`
	StartTime       time.Time `json:"start_time"`
	CompletionTime  time.Time `json:"completion_time"`
	Success         bool      `json:"success"`
	Error           string    `json:"error,omitempty"`
	ArtifactPath    string    `json:"artifact_path,omitempty"`
}

// TaskTelemetry captures the full lifecycle of a task across MLQ levels.
type TaskTelemetry struct {
	TaskID         string        `json:"task_id"`
	TaskClass      string        `json:"task_class"`
	JiraKey        string        `json:"jira_key,omitempty"`
	InitialLevel   int           `json:"initial_level"`
	Attempts       []TaskAttempt `json:"attempts"`
	FinalLevel     int           `json:"final_level"`
	FinalResult    string        `json:"final_result"` // success, l2_escalated, l0_fallback, infra_fail
	TotalRetries   int           `json:"total_retries"`
	Escalated      bool          `json:"escalated"`
	FallbackUsed   bool          `json:"fallback_used"`
}

// WorkerPool manages a pool of worker endpoints for a single MLQ level.
// Each worker is an independent llama.cpp server endpoint.
type WorkerPool struct {
	mu        sync.Mutex
	workers   []Worker
	rrIndex   uint64 // atomic round-robin counter
	baseLevel *Level
}

// Worker represents a single backend endpoint.
type Worker struct {
	ID       string `json:"id"`
	Endpoint string `json:"endpoint"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Busy     bool   `json:"busy"`
}

// NewWorkerPool creates a pool of workers for a given MLQ level.
// workers is a list of endpoint URLs (e.g. http://localhost:56227).
func NewWorkerPool(level *Level, endpoints []string) *WorkerPool {
	pool := &WorkerPool{
		baseLevel: level,
		workers:   make([]Worker, 0, len(endpoints)),
	}
	for i, ep := range endpoints {
		pool.workers = append(pool.workers, Worker{
			ID:       fmt.Sprintf("%s-w%d", level.Name, i+1),
			Endpoint: ep,
			Provider: level.Backend.Provider,
			Model:    level.Backend.Name,
			Busy:     false,
		})
	}
	log.Printf("[WorkerPool] Created pool for level %d (%s): %d workers", level.Level, level.Name, len(pool.workers))
	return pool
}

// SelectWorker picks a worker using round-robin.
func (wp *WorkerPool) SelectWorker() *Worker {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	idx := atomic.AddUint64(&wp.rrIndex, 1) - 1
	w := &wp.workers[idx%uint64(len(wp.workers))]
	return w
}

// SelectLeastBusy picks a worker that is not busy, falls back to round-robin.
func (wp *WorkerPool) SelectLeastBusy() *Worker {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	for i := range wp.workers {
		if !wp.workers[i].Busy {
			idx := atomic.AddUint64(&wp.rrIndex, 1) - 1
			return &wp.workers[idx%uint64(len(wp.workers))]
		}
	}
	// All busy: round-robin anyway
	idx := atomic.AddUint64(&wp.rrIndex, 1) - 1
	return &wp.workers[idx%uint64(len(wp.workers))]
}

// MarkBusy marks a worker as in-use.
func (wp *WorkerPool) MarkBusy(workerID string) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	for i := range wp.workers {
		if wp.workers[i].ID == workerID {
			wp.workers[i].Busy = true
			return
		}
	}
}

// MarkFree marks a worker as available.
func (wp *WorkerPool) MarkFree(workerID string) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	for i := range wp.workers {
		if wp.workers[i].ID == workerID {
			wp.workers[i].Busy = false
			return
		}
	}
}

// WorkerCount returns the number of workers in the pool.
func (wp *WorkerPool) WorkerCount() int {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return len(wp.workers)
}

// WorkerEndpoints returns all worker endpoint URLs.
func (wp *WorkerPool) WorkerEndpoints() []string {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	endpoints := make([]string, len(wp.workers))
	for i, w := range wp.workers {
		endpoints[i] = w.Endpoint
	}
	return endpoints
}

// TaskExecutor handles task-level retry and escalation across MLQ levels.
// It wraps the factory's task execution path to add:
// - Task-level retry (not just step-level)
// - Automatic escalation from L1 to L2 after repeated failures
// - Fallback to L0 on provider outage
// - Per-attempt telemetry
type TaskExecutor struct {
	mlq           *MLQ
	workerPools   map[int]*WorkerPool // level → worker pool
	telemetrySink TelemetrySink
}

// TelemetrySink receives task telemetry for analysis.
type TelemetrySink interface {
	Record(ctx context.Context, t *TaskTelemetry) error
}

// LogTelemetrySink records telemetry to structured logs.
type LogTelemetrySink struct{}

func (l *LogTelemetrySink) Record(_ context.Context, t *TaskTelemetry) error {
	log.Printf("[MLQ-Telemetry] task_id=%s class=%s initial_level=%d final=%d result=%s attempts=%d retries=%d escalated=%v fallback=%v",
		t.TaskID, t.TaskClass, t.InitialLevel, t.FinalLevel, t.FinalResult,
		len(t.Attempts), t.TotalRetries, t.Escalated, t.FallbackUsed)
	return nil
}

// NewTaskExecutor creates a TaskExecutor with MLQ config and worker pools.
func NewTaskExecutor(m *MLQ, pools map[int]*WorkerPool) *TaskExecutor {
	return &TaskExecutor{
		mlq:         m,
		workerPools: pools,
		telemetrySink: &LogTelemetrySink{},
	}
}

// GetEscalationRules returns escalation rules from config.
func (te *TaskExecutor) GetEscalationRules() []EscalationRule {
	return te.mlq.config.EscalationRules
}

// ExecuteWithRetry runs a task with task-level retry and escalation.
//
// Behavior:
//  1. Select initial MLQ level (L1 by default for regular tasks)
//  2. Execute task on selected level + worker
//  3. On failure: retry on same level up to max_retries
//  4. After repeated failures: escalate to next level per escalation_rules
//  5. On provider outage: fallback to L0 if allowed
//  6. Record telemetry on every attempt
//
// The executeFn receives the selected worker endpoint and must return success/failure.
func (te *TaskExecutor) ExecuteWithRetry(
	ctx context.Context,
	taskID, taskClass, jiraKey string,
	executeFn func(ctx context.Context, workerEndpoint string) (artifactPath string, err error),
) *TaskTelemetry {
	// Select initial level
	level, err := te.mlq.SelectLevel(taskID, jiraKey, taskClass)
	if err != nil {
		log.Printf("[MLQ] No level available for task %s: %v", taskID, err)
		return &TaskTelemetry{TaskID: taskID, TaskClass: taskClass, JiraKey: jiraKey, FinalResult: "infra_fail"}
	}

	telemetry := &TaskTelemetry{
		TaskID:       taskID,
		TaskClass:    taskClass,
		JiraKey:      jiraKey,
		InitialLevel: level.Level,
		FinalLevel:   level.Level,
		Attempts:     make([]TaskAttempt, 0),
	}

	currentLevel := level
	maxRetries := te.getMaxRetriesForLevel(currentLevel.Level)

	log.Printf("[MLQ] Task %s starting on level %d (%s), max_retries=%d",
		taskID, currentLevel.Level, currentLevel.Name, maxRetries)

	attemptID := 0
	for {
		attemptID++
		worker := te.selectWorker(currentLevel.Level)
		if worker == nil {
			log.Printf("[MLQ] No worker available for level %d, attempting fallback", currentLevel.Level)
			if fbLevel, ok := te.mlq.GetLevel(0); ok && fbLevel.Enabled {
				currentLevel = fbLevel
				telemetry.FallbackUsed = true
				log.Printf("[MLQ] Falling back to level 0 (%s) for task %s", fbLevel.Name, taskID)
				continue
			}
			telemetry.FinalResult = "infra_fail"
			break
		}

		log.Printf("[MLQ] Task %s attempt %d: level=%d worker=%s endpoint=%s",
			taskID, attemptID, currentLevel.Level, worker.ID, worker.Endpoint)

		if worker != nil {
			te.workerPools[currentLevel.Level].MarkBusy(worker.ID)
		}

		startTime := time.Now()
		artifactPath, execErr := executeFn(ctx, worker.Endpoint)
		completionTime := time.Now()

		if worker != nil {
			te.workerPools[currentLevel.Level].MarkFree(worker.ID)
		}

		attempt := TaskAttempt{
			AttemptID:      attemptID,
			Level:          currentLevel.Level,
			WorkerEndpoint: worker.Endpoint,
			StartTime:      startTime,
			CompletionTime: completionTime,
			Success:        execErr == nil,
			ArtifactPath:   artifactPath,
		}
		if execErr != nil {
			attempt.Error = execErr.Error()
		}
		telemetry.Attempts = append(telemetry.Attempts, attempt)

		if execErr == nil {
			// Success
			telemetry.FinalLevel = currentLevel.Level
			telemetry.FinalResult = "success"
			telemetry.TotalRetries = attemptID - 1
			log.Printf("[MLQ] Task %s SUCCEEDED on level %d worker %s (attempts=%d)",
				taskID, currentLevel.Level, worker.ID, attemptID)
			break
		}

		// Failure
		log.Printf("[MLQ] Task %s FAILED on level %d worker %s: %v",
			taskID, currentLevel.Level, worker.ID, execErr)

		// Check if we should escalate
		if attemptID >= maxRetries {
			nextLevel := te.findEscalationLevel(currentLevel.Level, taskClass)
			if nextLevel != nil {
				telemetry.Escalated = true
				telemetry.Attempts[len(telemetry.Attempts)-1].EscalationCount++
				log.Printf("[MLQ] Task %s ESCALATING from level %d to level %d after %d failures",
					taskID, currentLevel.Level, nextLevel.Level, attemptID)
				currentLevel = nextLevel
				maxRetries = te.getMaxRetriesForLevel(nextLevel.Level)
				attemptID = 0 // Reset attempt counter for new level
			} else {
				// Try L0 fallback
				if fbLevel, ok := te.mlq.GetLevel(0); ok && fbLevel.Enabled && currentLevel.Level != 0 {
					telemetry.FallbackUsed = true
					log.Printf("[MLQ] Task %s FALLING BACK to level 0 (%s) after exhaustion",
						taskID, fbLevel.Name)
					currentLevel = fbLevel
					maxRetries = te.getMaxRetriesForLevel(0)
					attemptID = 0
				} else {
					telemetry.FinalLevel = currentLevel.Level
					telemetry.FinalResult = "exhausted"
					telemetry.TotalRetries = len(telemetry.Attempts) - 1
					log.Printf("[MLQ] Task %s EXHAUSTED all retries and escalation paths", taskID)
					break
				}
			}
		}
	}

	// Record telemetry
	if te.telemetrySink != nil {
		if err := te.telemetrySink.Record(ctx, telemetry); err != nil {
			log.Printf("[MLQ] Failed to record telemetry: %v", err)
		}
	}

	return telemetry
}

// selectWorker picks a worker for a given level.
func (te *TaskExecutor) selectWorker(level int) *Worker {
	pool, ok := te.workerPools[level]
	if !ok || pool.WorkerCount() == 0 {
		return nil
	}
	return pool.SelectWorker()
}

// getMaxRetries returns max retries for a level from escalation rules.
func (te *TaskExecutor) getMaxRetriesForLevel(level int) int {
	for _, rule := range te.mlq.config.EscalationRules {
		if rule.FromLevel == level && rule.Trigger == "retry_count" {
			return rule.MaxRetries
		}
	}
	return 2 // default
}

// findEscalationLevel returns the next level to escalate to, or nil.
func (te *TaskExecutor) findEscalationLevel(fromLevel int, taskClass string) *Level {
	for _, rule := range te.mlq.config.EscalationRules {
		if rule.FromLevel == fromLevel && rule.Trigger == "retry_count" {
			if nextLevel, ok := te.mlq.GetLevel(rule.ToLevel); ok && nextLevel.Enabled {
				// Check if manual approval required
				if rule.RequireManualApproval {
					log.Printf("[MLQ] Escalation from %d to %d requires manual approval, skipping", fromLevel, rule.ToLevel)
					return nil
				}
				// Check if task class is allowed (if restricted)
				if len(rule.AllowedForTaskClass) > 0 {
					allowed := false
					for _, tc := range rule.AllowedForTaskClass {
						if tc == taskClass {
							allowed = true
							break
						}
					}
					if !allowed {
						continue
					}
				}
				return nextLevel
			}
		}
	}
	return nil
}
