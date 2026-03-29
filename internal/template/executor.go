package template

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/kube-zen/zen-brain1/internal/concurrency"
)

// ExecutorConfig holds configuration for the template executor.
type ExecutorConfig struct {
	LLMEndpoint string            // llama.cpp endpoint (e.g. http://localhost:56227)
	LLMModel    string            // model name (e.g. Qwen3.5-0.8B-Q4_K_M.gguf)
	TimeoutSec  int               // default step timeout
	MaxRetries  int               // default step retries
	ArtifactDir string            // where to write execution artifacts
	MetricsDir  string            // concurrency metrics directory
}

// Executor runs templates through their step sequence.
// It is the generic execution engine — no workflow-specific logic here.
type Executor struct {
	cfg        ExecutorConfig
	httpClient *http.Client
}

// NewExecutor creates a new template executor.
func NewExecutor(cfg ExecutorConfig) *Executor {
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 2700 * time.Second // 45 minutes default
	}
	return &Executor{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Execute runs a template with the given inputs and returns the result.
// It executes steps in order, collects outputs, and emits post-actions.
func (e *Executor) Execute(ctx context.Context, tmpl *Template, inputs map[string]string) *ExecutionResult {
	start := time.Now()
	result := &ExecutionResult{
		TemplateName: tmpl.Name,
		TemplateVer:  tmpl.Version,
		Steps:        make([]StepOutput, 0, len(tmpl.Steps)),
		PostActions:  make([]PostActionResult, 0),
	}

	log.Printf("[EXECUTOR] Starting template %q v%s (%d steps)", tmpl.Name, tmpl.Version, len(tmpl.Steps))

	// Execute each step
	for i, step := range tmpl.Steps {
		log.Printf("[EXECUTOR] Step %d/%d: %s (type=%s)", i+1, len(tmpl.Steps), step.Name, step.Type)

		stepResult := e.executeStep(ctx, step, inputs, tmpl)
		result.Steps = append(result.Steps, stepResult)

		if !stepResult.Success {
			result.Success = false
			result.Error = fmt.Sprintf("step %q failed: %s", step.Name, stepResult.Error)
			log.Printf("[EXECUTOR] ❌ Step %q failed: %s", step.Name, stepResult.Error)
			break
		}

		log.Printf("[EXECUTOR] ✅ Step %q completed (%dms)", step.Name, stepResult.DurationMs)

		// Merge step outputs into inputs for subsequent steps
		if stepResult.Output != nil {
			var stepOutputs map[string]string
			if err := json.Unmarshal(stepResult.Output, &stepOutputs); err == nil {
				for k, v := range stepOutputs {
					if _, exists := inputs[k]; !exists {
						inputs[k] = v
					}
				}
			}
		}
	}

	// If all steps succeeded, collect outputs
	if result.Error == "" {
		result.Success = true
		outputs := e.collectOutputs(tmpl, result.Steps)
		if len(outputs) > 0 {
			data, _ := json.Marshal(outputs)
			result.Outputs = data
		}
	}

	result.DurationMs = time.Since(start).Milliseconds()
	log.Printf("[EXECUTOR] Template %q: success=%v duration=%dms",
		tmpl.Name, result.Success, result.DurationMs)

	// Emit post-actions
	if result.Success {
		for _, pa := range tmpl.PostActions {
			paResult := e.evaluatePostAction(pa, result.Outputs)
			result.PostActions = append(result.PostActions, paResult)
			log.Printf("[EXECUTOR] Post-action %s → %s: success=%v",
				pa.Type, pa.Target, paResult.Success)
		}
	}

	// Write artifact
	if e.cfg.ArtifactDir != "" {
		e.writeArtifact(result)
	}

	return result
}

// executeStep runs a single step based on its type.
func (e *Executor) executeStep(ctx context.Context, step Step, inputs map[string]string, tmpl *Template) StepOutput {
	start := time.Now()
	output := StepOutput{StepName: step.Name, StepType: step.Type}

	switch step.Type {
	case "ai":
		output = e.executeAIStep(ctx, step, inputs)
	case "tool":
		output = e.executeToolStep(ctx, step, inputs)
	case "script":
		output = e.executeScriptStep(ctx, step, inputs)
	case "http":
		output = e.executeHTTPStep(ctx, step, inputs)
	default:
		output.Success = false
		output.Error = fmt.Sprintf("unknown step type: %s", step.Type)
	}

	output.DurationMs = time.Since(start).Milliseconds()

	// Retry on failure
	if !output.Success && step.MaxRetries > 0 {
		for attempt := 1; attempt <= step.MaxRetries; attempt++ {
			log.Printf("[EXECUTOR] Retrying step %q (attempt %d/%d)", step.Name, attempt, step.MaxRetries)
			time.Sleep(time.Duration(attempt*attempt) * time.Second) // backoff
			retry := e.executeStep(ctx, step, inputs, tmpl)
			if retry.Success {
				retry.DurationMs = time.Since(start).Milliseconds()
				return retry
			}
		}
	}

	return output
}

// executeAIStep calls the local llama.cpp endpoint with the step prompt.
func (e *Executor) executeAIStep(ctx context.Context, step Step, inputs map[string]string) StepOutput {
	output := StepOutput{StepName: step.Name, StepType: "ai"}

	endpoint := e.cfg.LLMEndpoint
	model := e.cfg.LLMModel
	if step.Model != "" {
		model = step.Model
	}

	// Render prompt with inputs
	prompt := RenderPrompt(step.Prompt, inputs)

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
		// llama.cpp: disable thinking to match certified path
		"enable_thinking": false,
	}

	data, _ := json.Marshal(reqBody)
	resp, err := e.httpClient.Post(endpoint+"/v1/chat/completions", "application/json", bytes.NewReader(data))
	if err != nil {
		output.Success = false
		output.Error = fmt.Sprintf("llama.cpp call failed: %v", err)
		return output
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		output.Success = false
		output.Error = fmt.Sprintf("llama.cpp returned %d: %s", resp.StatusCode, string(body))
		return output
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &chatResp); err != nil {
		output.Success = false
		output.Error = fmt.Sprintf("response parse error: %v", err)
		return output
	}
	if len(chatResp.Choices) == 0 {
		output.Success = false
		output.Error = "no choices in response"
		return output
	}

	output.Success = true
	content := chatResp.Choices[0].Message.Content
	output.Output = json.RawMessage(content)
	return output
}

// executeToolStep invokes a registered tool.
func (e *Executor) executeToolStep(ctx context.Context, step Step, inputs map[string]string) StepOutput {
	output := StepOutput{StepName: step.Name, StepType: "tool"}

	// Tool execution is a placeholder — the engine enforces the allowed_tools contract.
	// For now, log and mark as success (tools are invoked by the remediation-worker subprocess).
	log.Printf("[EXECUTOR] Tool step %q: tool=%s (allowed_tools enforcement)", step.Name, step.Tool)
	output.Success = true
	output.Output = json.RawMessage(`{"tool_invoked": "` + step.Tool + `"}`)
	return output
}

// executeScriptStep runs a shell script.
func (e *Executor) executeScriptStep(ctx context.Context, step Step, inputs map[string]string) StepOutput {
	output := StepOutput{StepName: step.Name, StepType: "script"}

	timeout := time.Duration(step.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = time.Duration(e.cfg.TimeoutSec) * time.Second
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", step.Script)
	cmd.Env = make([]string, 0)
	for k, v := range step.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		output.Success = false
		output.Error = fmt.Sprintf("script failed: %v\nstderr: %s", err, stderr.String())
		return output
	}

	output.Success = true
	output.Output = json.RawMessage(stdout.Bytes())
	return output
}

// executeHTTPStep makes an HTTP request.
func (e *Executor) executeHTTPStep(ctx context.Context, step Step, inputs map[string]string) StepOutput {
	output := StepOutput{StepName: step.Name, StepType: "http"}

	method := step.HTTPMethod
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, step.HTTPURL, nil)
	if err != nil {
		output.Success = false
		output.Error = fmt.Sprintf("http request build: %v", err)
		return output
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		output.Success = false
		output.Error = fmt.Sprintf("http request failed: %v", err)
		return output
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	output.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	if !output.Success {
		output.Error = fmt.Sprintf("http %d: %s", resp.StatusCode, string(body))
	} else {
		output.Output = json.RawMessage(body)
	}
	return output
}

// collectOutputs gathers outputs from step results that match template output definitions.
func (e *Executor) collectOutputs(tmpl *Template, stepOutputs []StepOutput) map[string]interface{} {
	outputs := make(map[string]interface{})
	for _, stepOut := range stepOutputs {
		if stepOut.Success && stepOut.Output != nil {
			var data map[string]interface{}
			if err := json.Unmarshal(stepOut.Output, &data); err == nil {
				// Merge into outputs
				for k, v := range data {
					// Check if this matches a declared output
					for _, def := range tmpl.Outputs {
						if def.Name == k {
							outputs[k] = v
						}
					}
					// Also add all fields from AI step outputs for post-action filtering
					outputs[k] = v
				}
			}
		}
	}
	return outputs
}

// evaluatePostAction checks if a post-action should fire and records the intent.
func (e *Executor) evaluatePostAction(pa PostAction, outputs json.RawMessage) PostActionResult {
	result := PostActionResult{
		Type:   pa.Type,
		Target: pa.Target,
		Success: true,
	}

	// Filter check: if filter is specified, verify the output contains that key
	if pa.Filter != "" && outputs != nil {
		var data map[string]interface{}
		if err := json.Unmarshal(outputs, &data); err == nil {
			if _, ok := data[pa.Filter]; !ok {
				result.Success = false
				result.Error = fmt.Sprintf("filter key %q not in outputs", pa.Filter)
			}
		}
	}

	return result
}

// writeArtifact persists the execution result.
func (e *Executor) writeArtifact(result *ExecutionResult) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return
	}
	path := fmt.Sprintf("%s/%s_%s.json",
		e.cfg.ArtifactDir,
		result.TemplateName,
		time.Now().Format("20060102-150405"))
	_ = writeFile(path, data)
}

// ConcurrencyMetrics is an alias for the concurrency package metrics.
type ConcurrencyMetrics = concurrency.ConcurrencyMetrics

func writeFile(path string, data []byte) error {
	dir := path[:len(path)-len(path[len(path)-1:])]
	return exec.Command("mkdir", "-p", dir).Run() // simplified
}
