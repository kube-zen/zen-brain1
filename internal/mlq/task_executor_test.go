package mlq

import (
	"context"
	"testing"
	"time"
)

func TestWorkerPoolRoundRobin(t *testing.T) {
	level := &Level{Level: 1, Name: "test", Backend: BackendConfig{Provider: "test", Name: "0.8b"}}
	endpoints := []string{"http://localhost:56227", "http://localhost:56228", "http://localhost:56229"}
	pool := NewWorkerPool(level, endpoints)

	if pool.WorkerCount() != 3 {
		t.Fatalf("expected 3 workers, got %d", pool.WorkerCount())
	}

	w1 := pool.SelectWorker()
	w2 := pool.SelectWorker()
	w3 := pool.SelectWorker()
	w4 := pool.SelectWorker() // should wrap around

	if w1.ID == w2.ID && w2.ID == w3.ID {
		t.Error("round-robin should rotate workers")
	}
	if w1.Endpoint != w4.Endpoint {
		t.Error("round-robin should wrap around to first worker")
	}
}

func TestWorkerPoolBusyFree(t *testing.T) {
	level := &Level{Level: 1, Name: "test", Backend: BackendConfig{Provider: "test", Name: "0.8b"}}
	pool := NewWorkerPool(level, []string{"http://localhost:56227"})

	w := pool.SelectWorker()
	pool.MarkBusy(w.ID)

	if !pool.workers[0].Busy {
		t.Error("worker should be marked busy")
	}

	pool.MarkFree(w.ID)
	if pool.workers[0].Busy {
		t.Error("worker should be marked free")
	}
}

func TestTaskExecutorRetryAndEscalation(t *testing.T) {
	config := &MLQConfig{
		MLQLevels: []Level{
			{Level: 1, Name: "l1", Enabled: true, Backend: BackendConfig{Provider: "llama-cpp", Name: "0.8b", APIEndpoint: "http://localhost:56227"}},
			{Level: 2, Name: "l2", Enabled: true, Backend: BackendConfig{Provider: "llama-cpp", Name: "2b", APIEndpoint: "http://localhost:60509"}},
			{Level: 0, Name: "l0", Enabled: true, Backend: BackendConfig{Provider: "ollama", Name: "0.8b", APIEndpoint: "http://localhost:11434"}},
		},
		EscalationRules: []EscalationRule{
			{Trigger: "retry_count", FromLevel: 1, ToLevel: 2, MaxRetries: 2},
			{Trigger: "timeout_or_error", FromLevel: 1, ToLevel: 0, TimeoutThresholdSec: 2700},
		},
		SelectionPolicy: SelectionPolicy{
			DefaultLevelMapping: map[string]int{"implementation": 1},
			FallbackBehavior:    FallbackConfig{Strategy: "fallback_on_failure", MaxFallbackAttempts: 3},
		},
		Logging: LoggingConfig{LogSelection: true, SelectionFormat: "level={level} task={task_id}"},
	}
	m := NewMLQ(config)

	pools := map[int]*WorkerPool{
		1: NewWorkerPool(m.levels[1], []string{"http://localhost:56227"}),
		2: NewWorkerPool(m.levels[2], []string{"http://localhost:60509"}),
		0: NewWorkerPool(m.levels[0], []string{"http://localhost:11434"}),
	}

	te := NewTaskExecutor(m, pools)

	// Test: L1 fails 2x then L2 succeeds
	callCount := 0
	telemetry := te.ExecuteWithRetry(
		context.Background(), "test-task-1", "implementation", "",
		func(_ context.Context, _ string) (string, error) {
			callCount++
			if callCount <= 2 {
				return "", fmt.Errorf("simulated L1 failure")
			}
			return "/tmp/artifact.md", nil
		},
	)

	if !telemetry.Escalated {
		t.Error("expected escalation after 2 L1 failures")
	}
	if telemetry.FinalResult != "success" {
		t.Errorf("expected success, got %s", telemetry.FinalResult)
	}
	if len(telemetry.Attempts) != 3 {
		t.Errorf("expected 3 attempts (2 L1 + 1 L2), got %d", len(telemetry.Attempts))
	}
	if telemetry.Attempts[2].Level != 2 {
		t.Errorf("expected final attempt on level 2, got %d", telemetry.Attempts[2].Level)
	}
}

func TestTaskExecutorL1Success(t *testing.T) {
	config := &MLQConfig{
		MLQLevels: []Level{
			{Level: 1, Name: "l1", Enabled: true, Backend: BackendConfig{Provider: "llama-cpp", Name: "0.8b", APIEndpoint: "http://localhost:56227"}},
		},
		EscalationRules: []EscalationRule{
			{Trigger: "retry_count", FromLevel: 1, ToLevel: 2, MaxRetries: 2},
		},
		SelectionPolicy: SelectionPolicy{
			DefaultLevelMapping: map[string]int{"implementation": 1},
		},
		Logging: LoggingConfig{LogSelection: true},
	}
	m := NewMLQ(config)
	pools := map[int]*WorkerPool{
		1: NewWorkerPool(m.levels[1], []string{"http://localhost:56227"}),
	}
	te := NewTaskExecutor(m, pools)

	telemetry := te.ExecuteWithRetry(
		context.Background(), "test-task-2", "implementation", "",
		func(_ context.Context, _ string) (string, error) {
			return "/tmp/artifact.md", nil
		},
	)

	if telemetry.Escalated {
		t.Error("should not escalate on L1 success")
	}
	if telemetry.FinalResult != "success" {
		t.Errorf("expected success, got %s", telemetry.FinalResult)
	}
	if len(telemetry.Attempts) != 1 {
		t.Errorf("expected 1 attempt, got %d", len(telemetry.Attempts))
	}
}

func TestTaskTelemetryRecord(t *testing.T) {
	telemetry := &TaskTelemetry{
		TaskID:       "t-1",
		TaskClass:    "implementation",
		InitialLevel: 1,
		FinalLevel:   1,
		FinalResult:  "success",
		Attempts: []TaskAttempt{
			{AttemptID: 1, Level: 1, WorkerEndpoint: "http://localhost:56227", StartTime: time.Now(), CompletionTime: time.Now(), Success: true, ArtifactPath: "/tmp/a.md"},
		},
	}
	sink := &LogTelemetrySink{}
	err := sink.Record(context.Background(), telemetry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
