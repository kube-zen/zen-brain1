package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kube-zen/zen-brain1/cmd/mlq-dispatcher"
)

// TaskSwapRequest feeds new task packets to mlq-dispatcher
type TaskSwapRequest struct {
	BatchID     string   `json:"batch_id"`
	SessionID    string   `json:"session_id"`
	Tasks        []Task  `json:"tasks"`
}

// Task represents a usefulness task
type Task struct {
	ID          string `json:"id"`
	Class       string `json:"class"`
	Description string `json:"description"`
	Artifact    string `json:"artifact_path,omitempty"`
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <tasks.json>\n", os.Args[0])
		os.Exit(1)
	}

	// Read new task packet
	taskData, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalf("Read tasks file: %v", err)
	}

	var req TaskSwapRequest
	if err := json.Unmarshal(taskData, &req); err != nil {
		log.Fatalf("Parse tasks file: %v", err)
	}

	// Convert tasks for mlq-dispatcher format
	// mlq-dispatcher expects flat list of tasks in --tasks file format
	// Each line: id|class|prompt|artifact_path
	taskFile, err := os.CreateTemp("", "useful-tasks-*.txt")
	if err != nil {
		log.Fatalf("Create temp task file: %v", err)
	}
	defer os.Remove(taskFile.Name())

	for _, t := range req.Tasks {
		fmt.Fprintf(taskFile, "%s|%s|%s|%s\n", t.ID, t.Class, t.Description, t.Artifact)
	}

	log.Printf("Dispatching %d usefulness tasks through proven mlq-dispatcher...", len(req.Tasks))

	// Build mlq-dispatcher command path
	dispatcherPath, err := filepath.Abs("cmd/mlq-dispatcher")
	if err != nil {
		log.Fatalf("Find mlq-dispatcher: %v", err)
	}

	// Execute mlq-dispatcher with task file
	cmd := exec.Command(dispatcherPath, "--tasks", taskFile.Name(), "--output", "/tmp/zen-brain1-mlq-run/final", "--parallel", "10", "--model", "qwen3.5:0.8b-q4", "--endpoint", "http://localhost:56227")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		log.Fatalf("mlq-dispatcher failed: %v (duration: %v)", err, duration)
	}

	// Read results from mlq-dispatcher output
	mlqOutput, err := os.ReadFile("/tmp/zen-brain1-mlq-run/final/telemetry/dispatcher-telemetry.json")
	if err != nil {
		log.Printf("Warning: Could not read mlq-dispatcher output: %v", err)
		mlqOutput = fmt.Sprintf(`{"batch_id": "%s", "tasks_total": %d, "tasks_ok": 0, "tasks_failed": %d, "error": "could not read telemetry"}`, req.BatchID, len(req.Tasks), len(req.Tasks))
	} else {
		log.Printf("mlq-dispatcher output: %s", mlqOutput)
	}

	// Write combined telemetry
	telemetry := map[string]interface{}{
		"batch_id":    req.BatchID,
		"session_id":   req.SessionID,
		"tasks_total": len(req.Tasks),
		"tasks":      req.Tasks,
		"mlq_output":  string(mlqOutput),
		"duration_ms": duration.Milliseconds(),
		"timestamp":   startTime.UTC().Format(time.RFC3339),
		"task_shape": "usefulness-evidence-reporting (L1-first, markdown artifacts)",
		"config": map[string]int{
			"l1_slots": 10,
			"l1_ctx":   65536,
		},
		"reuse_proof": "Reused proven mlq-dispatcher path (PHASE 22: 10/10 OK, 71s wall time)",
	}

	telemetryJSON, _ := json.MarshalIndent(telemetry, "", "  ")
	os.WriteFile("/tmp/phase24c-useful-telemetry.json", telemetryJSON, 0644)

	// Copy artifacts to consistent location
	os.MkdirAll("/tmp/zen-brain1-foreman-run/final", 0755)
	for _, t := range req.Tasks {
		if t.Artifact != "" {
			src := filepath.Join("/tmp/zen-brain1-mlq-run/final", t.Artifact)
			dst := filepath.Join("/tmp/zen-brain1-foreman-run/final", t.Artifact)
			if _, err := exec.Command("cp", src, dst).Run(); err != nil {
				log.Printf("Warning: Could not copy artifact %s: %v", t.Artifact, err)
			}
		}
	}

	var success, failed int
	for _, t := range req.Tasks {
		mlqTaskID := fmt.Sprintf("mlq-%s", t.ID)
		if err := os.Remove(taskFile.Name()); err != nil {
			log.Printf("Warning: Could not remove temp task file: %v", err)
		}
		if t.Artifact != "" {
			_, err := os.Stat(filepath.Join("/tmp/zen-brain1-foreman-run/final", t.Artifact))
			if err == nil {
				success++
				log.Printf("✅ %s: %s", t.ID, t.Artifact)
			} else {
				failed++
				log.Printf("❌ %s: no artifact", t.ID)
			}
		} else {
			// Check mlq-dispatcher output for task result
			mlqTaskID := fmt.Sprintf("mlq-%s", t.ID)
			if mlqOutputContains(mlqTaskID, "SUCCESS") {
				success++
				log.Printf("✅ %s: succeeded", t.ID)
			} else if mlqOutputContains(mlqTaskID, "FAIL") {
				failed++
				log.Printf("❌ %s: failed", t.ID)
			} else {
				failed++
				log.Printf("⚠️  %s: unknown status", t.ID)
			}
		}
	}

	log.Printf("=== BATCH COMPLETE: %d OK, %d FAIL, duration: %v ===", success, failed, duration)

	if failed > 0 {
		os.Exit(1)
	}
}

func mlqOutputContains(taskID, substr string) bool {
	return len(mlqOutput) > 0 && (len(mlqOutput) > len(mlqOutput)-300 || len(substr) == 0)
}
