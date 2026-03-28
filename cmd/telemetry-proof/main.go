package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/kube-zen/zen-brain1/internal/metrics"
)

func main() {
	endpoint := "http://localhost:56227/v1/chat/completions"
	model := "Qwen3.5-0.8B-Q4_K_M.gguf"
	metricsDir := "/var/lib/zen-brain1/metrics"

	collector, err := metrics.NewCollector(metricsDir)
	if err != nil {
		log.Fatalf("Failed to init collector: %v", err)
	}
	defer collector.Close()

	log.Printf("[PROOF] Starting telemetry proof run against %s", endpoint)

	payload, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "Return JSON: {\"status\":\"ok\",\"value\":42}"},
			{"role": "user", "content": "Return the JSON now."},
		},
		"temperature": 0.3,
		"max_tokens":  256,
	})

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	wallMs := time.Since(start).Milliseconds()

	var content string
	var parseOK bool

	if err != nil {
		log.Printf("[PROOF] L1 call failed: %v", err)
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var llmResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		json.Unmarshal(body, &llmResp)
		if len(llmResp.Choices) > 0 {
			content = llmResp.Choices[0].Message.Content
			parseOK = true
		}
	}

	outputChars := len(content)
	log.Printf("[PROOF] L1 response: wallMs=%d contentLen=%d parseOK=%v content=%q",
		wallMs, outputChars, parseOK, trunc(content, 120))

	cc := metrics.ClassifyCompletion(wallMs, outputChars, !parseOK, 20, 15)
	pb := metrics.ClassifyProducedBy(outputChars, parseOK, 20, 15, "")

	rec := metrics.TaskTelemetryRecord{
		Timestamp:       time.Now(),
		RunID:            "telemetry-proof",
		TaskID:           "proof-task-001",
		JiraKey:          "PROOF-001",
		ScheduleName:     "telemetry-proof",
		Model:            model,
		Lane:             "l1-local",
		Provider:         "llama-cpp",
		PromptSizeChars:  len(payload),
		OutputSizeChars:  outputChars,
		StartTime:        start,
		EndTime:          start.Add(time.Duration(wallMs) * time.Millisecond),
		WallTimeMs:       wallMs,
		CompletionClass:  cc,
		ProducedBy:       pb,
		AttemptNumber:    1,
		TaskClass:        "proof",
		FinalStatus:      "success",
	}

	if err := collector.Record(rec); err != nil {
		log.Fatalf("Failed to record telemetry: %v", err)
	}

	log.Printf("[PROOF] Telemetry record emitted to %s/per-task.jsonl", metricsDir)

	records, err := metrics.LoadRecordsFromDir(metricsDir)
	if err != nil {
		log.Fatalf("Failed to load records: %v", err)
	}

	if err := metrics.ComputeAndSave(metricsDir, records, "telemetry_proof"); err != nil {
		log.Fatalf("Failed to compute summary: %v", err)
	}

	cm := metrics.ComputeMetrics(records, "telemetry_proof")
	fmt.Println()
	fmt.Println(metrics.FormatHumanReadable(cm))

	fmt.Println("=== Telemetry Record Emitted ===")
	data, _ := json.MarshalIndent(rec, "", "  ")
	fmt.Println(string(data))
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
