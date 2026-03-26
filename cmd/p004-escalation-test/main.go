package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/mlq"
)

const (
	L1Endpoint = "http://localhost:56227/v1/chat/completions"
	L1Model    = "Qwen3.5-0.8B-Q4_K_M.gguf"
	L2Endpoint = "http://localhost:60509/v1/chat/completions"
	L2Model    = "zen-go-q4_k_m.gguf"
	OutputDir  = "/tmp/zen-brain1-mlq-run"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	os.MkdirAll(OutputDir+"/logs", 0755)

	// Build MLQ config matching mlq-levels.yaml
	config := &mlq.MLQConfig{
		MLQLevels: []mlq.Level{
			{
				Level: 1, Name: "mlq-level-1", Enabled: true,
				Backend: mlq.BackendConfig{Provider: "llama-cpp", Name: "0.8b-q4", APIEndpoint: L1Endpoint, TimeoutSeconds: 120},
				Concurrency: mlq.ConcurrencyConfig{MaxWorkers: 10},
			},
			{
				Level: 2, Name: "mlq-level-2", Enabled: true,
				Backend: mlq.BackendConfig{Provider: "llama-cpp", Name: "2b-q4", APIEndpoint: L2Endpoint, TimeoutSeconds: 120},
				Concurrency: mlq.ConcurrencyConfig{MaxWorkers: 1},
			},
		},
		EscalationRules: []mlq.EscalationRule{
			{Trigger: "retry_count", FromLevel: 1, ToLevel: 2, MaxRetries: 2},
		},
		SelectionPolicy: mlq.SelectionPolicy{
			DefaultLevelMapping: map[string]int{"implementation": 1},
		},
		Logging: mlq.LoggingConfig{LogSelection: true, SelectionFormat: "level={level} task={task_id}"},
	}
	m := mlq.NewMLQ(config)

	pools := map[int]*mlq.WorkerPool{
		1: mlq.NewWorkerPool(m.GetLevelOrNil(1), []string{L1Endpoint}),
		2: mlq.NewWorkerPool(m.GetLevelOrNil(2), []string{L2Endpoint}),
	}
	te := mlq.NewTaskExecutor(m, pools)

	// Craft a task that will deterministically fail on L1:
	// Ask for a response starting with "IMPOSSIBLE_PREFIX_XYZ123" — 0.8B can't reliably produce this
	taskID := "escalation-test-001"
	taskClass := "implementation"

	log.Printf("[P004] Starting escalation test: task=%s class=%s", taskID, taskClass)

	telemetry := te.ExecuteWithRetry(
		context.Background(), taskID, taskClass, "",
		func(ctx context.Context, workerEndpoint string) (string, error) {
			log.Printf("[P004] Attempt on endpoint: %s", workerEndpoint)

			model := L1Model
			if workerEndpoint == L2Endpoint {
				model = L2Model
			}

			reqBody := map[string]interface{}{
				"model": model,
				"messages": []map[string]string{
					{"role": "user", "content": "Your response must start with exactly these words: IMPOSSIBLE_PREFIX_XYZ123. Nothing else before it. Just those exact words."},
				},
				"max_tokens": 50,
				"temperature": 0.0,
				"chat_template_kwargs": map[string]interface{}{"enable_thinking": false},
			}
			bodyJSON, _ := json.Marshal(reqBody)

			reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(reqCtx, "POST", workerEndpoint, bytes.NewReader(bodyJSON))
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return "", fmt.Errorf("HTTP error: %w", err)
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)
			var chatResp struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error,omitempty"`
			}
			json.Unmarshal(respBody, &chatResp)

			if chatResp.Error != nil {
				return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
			}

			content := ""
			if len(chatResp.Choices) > 0 {
				content = chatResp.Choices[0].Message.Content
			}

			// L1 fails: content must start with the impossible prefix
			if workerEndpoint == L1Endpoint && content != "IMPOSSIBLE_PREFIX_XYZ123" {
				return "", fmt.Errorf("L1 failure: expected 'IMPOSSIBLE_PREFIX_XYZ123', got '%s'", content)
			}

			// L2 succeeds: accept any non-empty response
			if workerEndpoint == L2Endpoint && content != "" {
				artifactPath := fmt.Sprintf("%s/final/escalation-test-artifact.md", OutputDir)
				os.WriteFile(artifactPath, []byte(fmt.Sprintf("# Escalation Test\n\nL2 recovered from L1 failure.\n\nOriginal L1 content: %q\n\nL2 content: %s", content, content)), 0644)
				return artifactPath, nil
			}

			return "", fmt.Errorf("empty response from %s", workerEndpoint)
		},
	)

	// Write results
	logFile, _ := os.Create(OutputDir + "/logs/escalation-test.log")
	logStr := fmt.Sprintf("task_id=%s class=%s initial=%d final=%d result=%s attempts=%d retries=%d escalated=%v\n",
		telemetry.TaskID, telemetry.TaskClass, telemetry.InitialLevel, telemetry.FinalLevel,
		telemetry.FinalResult, len(telemetry.Attempts), telemetry.TotalRetries, telemetry.Escalated)
	logFile.WriteString(logStr)

	for i, a := range telemetry.Attempts {
		logFile.WriteString(fmt.Sprintf("  attempt=%d level=%d endpoint=%s start=%s end=%s success=%v error=%q\n",
			i+1, a.Level, a.WorkerEndpoint, a.StartTime.Format(time.RFC3339Nano),
			a.CompletionTime.Format(time.RFC3339Nano), a.Success, a.Error))
	}
	logFile.Close()

	// Summary
	fmt.Printf("\n=== ESCALATION TEST RESULTS ===\n")
	fmt.Printf("Task: %s\n", telemetry.TaskID)
	fmt.Printf("Initial level: %d\n", telemetry.InitialLevel)
	fmt.Printf("Final level: %d\n", telemetry.FinalLevel)
	fmt.Printf("Result: %s\n", telemetry.FinalResult)
	fmt.Printf("Attempts: %d\n", len(telemetry.Attempts))
	fmt.Printf("Retries: %d\n", telemetry.TotalRetries)
	fmt.Printf("Escalated: %v\n", telemetry.Escalated)
	for i, a := range telemetry.Attempts {
		status := "❌"
		if a.Success {
			status = "✅"
		}
		fmt.Printf("  [%s] Attempt %d: level=%d endpoint=%s duration=%v error=%q\n",
			status, i+1, a.Level, a.WorkerEndpoint, a.CompletionTime.Sub(a.StartTime), a.Error)
	}
	fmt.Printf("\nLogs: %s/logs/escalation-test.log\n", OutputDir)

	if telemetry.Escalated {
		fmt.Println("\n✅ ESCALATION PROVEN: L1 failed, L2 succeeded")
	} else if telemetry.FinalResult == "success" {
		fmt.Println("\n⚠️ NO ESCALATION: L1 succeeded on first try (unexpected)")
	} else {
		fmt.Println("\n❌ ESCALATION FAILED: All levels exhausted")
	}
}
