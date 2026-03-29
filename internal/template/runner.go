package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Runner executes a template against a local llama.cpp endpoint.
// It is the engine: it enforces safety, leases, retries, permissions, and audit.
// Templates define intent; the runner enforces policy.
type Runner struct {
	LLMEndpoint string
	Timeout     time.Duration
	DryRun      bool
}

// RunnerConfig holds runner configuration.
type RunnerConfig struct {
	LLMEndpoint string
	Timeout     time.Duration
	DryRun      bool
}

// NewRunner creates a new template runner.
func NewRunner(cfg RunnerConfig) *Runner {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2700 * time.Second // 45m default (ZB-024)
	}
	return &Runner{
		LLMEndpoint: cfg.LLMEndpoint,
		Timeout:     cfg.Timeout,
		DryRun:      cfg.DryRun,
	}
}

// Run executes a template with the given inputs and returns the result.
// Steps are executed in order. On step failure, execution stops and returns the partial result.
func (r *Runner) Run(tmpl *Template, inputs map[string]string) *ExecutionResult {
	start := time.Now()
	result := &ExecutionResult{
		TemplateName: tmpl.Name,
		TemplateVer:  tmpl.Version,
		Steps:        make([]StepOutput, 0, len(tmpl.Steps)),
	}

	// Step 1: AI call (render prompt with inputs, call llama.cpp)
	for i, step := range tmpl.Steps {
		stepStart := time.Now()

		var stepOutput StepOutput
		stepOutput.StepName = step.Name
		stepOutput.StepType = step.Type

		switch step.Type {
		case "ai":
			stepOutput = r.executeAIStep(step, inputs)
		case "tool":
			stepOutput = r.executeToolStep(step, inputs)
		case "script":
			stepOutput = StepOutput{
				StepName: step.Name,
				StepType: step.Type,
				Success:  false,
				Error:    "script steps not yet implemented in template runner",
			}
		case "http":
			stepOutput = r.executeHTTPStep(step, inputs)
		default:
			stepOutput = StepOutput{
				StepName: step.Name,
				StepType: step.Type,
				Success:  false,
				Error:    fmt.Sprintf("unknown step type: %s", step.Type),
			}
		}

		stepOutput.DurationMs = time.Since(stepStart).Milliseconds()
		result.Steps = append(result.Steps, stepOutput)

		if !stepOutput.Success {
			result.Success = false
			result.Error = fmt.Sprintf("step %q (%d/%d) failed: %s",
				step.Name, i+1, len(tmpl.Steps), stepOutput.Error)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		// If step produced output, store it for subsequent steps
		if stepOutput.Output != nil {
			// Merge step output into inputs for subsequent steps
			var stepData map[string]interface{}
			if err := json.Unmarshal(stepOutput.Output, &stepData); err == nil {
				for k, v := range stepData {
					if s, ok := v.(string); ok {
						inputs[k] = s
					} else if b, err := json.Marshal(v); err == nil {
						inputs[k] = string(b)
					}
				}
			}
		}

		log.Printf("[RUNNER] Step %q (%d/%d) OK (%dms)",
			step.Name, i+1, len(tmpl.Steps), stepOutput.DurationMs)
	}

	result.Success = true
	result.DurationMs = time.Since(start).Milliseconds()

	// Step 2: Evaluate post-actions
	result.PostActions = r.evaluatePostActions(tmpl, inputs)

	return result
}

// executeAIStep calls the llama.cpp endpoint with the rendered prompt.
func (r *Runner) executeAIStep(step Step, inputs map[string]string) StepOutput {
	if r.DryRun {
		return StepOutput{
			StepName: step.Name,
			StepType: step.Type,
			Success:  true,
			Output:   json.RawMessage(`{"dry_run": true, "note": "skipped LLM call"}`),
		}
	}

	prompt := RenderPrompt(step.Prompt, inputs)
	model := step.Model
	if model == "" {
		model = "default" // llama.cpp uses whatever model is loaded
	}

	// Build OpenAI-compatible request
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		// llama.cpp with enable_thinking=false — no thinking tokens
		"enable_thinking": false,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return StepOutput{StepName: step.Name, StepType: step.Type, Error: fmt.Sprintf("marshal: %v", err)}
	}

	url := r.LLMEndpoint + "/v1/chat/completions"

	client := &http.Client{Timeout: r.Timeout}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return StepOutput{StepName: step.Name, StepType: step.Type, Error: fmt.Sprintf("LLM call failed: %v", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return StepOutput{StepName: step.Name, StepType: step.Type, Error: fmt.Sprintf("read response: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return StepOutput{StepName: step.Name, StepType: step.Type,
			Error: fmt.Sprintf("LLM returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))}
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return StepOutput{StepName: step.Name, StepType: step.Type,
			Error: fmt.Sprintf("parse LLM response: %v", err)}
	}
	if len(chatResp.Choices) == 0 {
		return StepOutput{StepName: step.Name, StepType: step.Type, Error: "LLM returned no choices"}
	}

	content := chatResp.Choices[0].Message.Content
	return StepOutput{
		StepName: step.Name,
		StepType: step.Type,
		Success:  true,
		Output:   json.RawMessage(content),
	}
}

// executeToolStep is a placeholder for tool invocation steps.
// In the full implementation, this would dispatch to registered tool handlers.
func (r *Runner) executeToolStep(step Step, inputs map[string]string) StepOutput {
	if r.DryRun {
		return StepOutput{
			StepName: step.Name,
			StepType: step.Type,
			Success:  true,
			Output:   json.RawMessage(fmt.Sprintf(`{"tool": %q, "dry_run": true}`, step.Tool)),
		}
	}

	// Placeholder: tool execution will be implemented when the tool registry is built
	return StepOutput{
		StepName: step.Name,
		StepType: step.Type,
		Success:  false,
		Error:    fmt.Sprintf("tool %q not yet registered in template runner", step.Tool),
	}
}

// executeHTTPStep makes an HTTP request.
func (r *Runner) executeHTTPStep(step Step, inputs map[string]string) StepOutput {
	if r.DryRun {
		return StepOutput{
			StepName: step.Name,
			StepType: step.Type,
			Success:  true,
			Output:   json.RawMessage(fmt.Sprintf(`{"url": %q, "dry_run": true}`, step.HTTPURL)),
		}
	}

	url := RenderPrompt(step.HTTPURL, inputs)
	method := step.HTTPMethod
	if method == "" {
		method = "GET"
	}

	client := &http.Client{Timeout: r.Timeout}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return StepOutput{StepName: step.Name, StepType: step.Type, Error: err.Error()}
	}

	resp, err := client.Do(req)
	if err != nil {
		return StepOutput{StepName: step.Name, StepType: step.Type, Error: err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return StepOutput{StepName: step.Name, StepType: step.Type, Error: err.Error()}
	}

	if resp.StatusCode >= 400 {
		return StepOutput{StepName: step.Name, StepType: step.Type,
			Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))}
	}

	return StepOutput{
		StepName: step.Name,
		StepType: step.Type,
		Success:  true,
		Output:   json.RawMessage(body),
	}
}

// evaluatePostActions processes the template's post-actions and returns results.
// Post-actions are emitted but NOT executed by the runner — that's the caller's responsibility.
// The runner only determines which post-actions should fire based on conditions.
func (r *Runner) evaluatePostActions(tmpl *Template, inputs map[string]string) []PostActionResult {
	results := make([]PostActionResult, 0, len(tmpl.PostActions))

	for _, pa := range tmpl.PostActions {
		// Evaluate condition if present
		if pa.Cond != "" {
			// Simple condition: check if the referenced key exists in inputs with non-empty value
			if val, ok := inputs[pa.Cond]; !ok || val == "" {
				continue // skip post-action if condition not met
			}
		}

		results = append(results, PostActionResult{
			Type:   pa.Type,
			Target: pa.Target,
			// Success is true because emission succeeded (execution is caller's job)
			Success: true,
		})
	}

	return results
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
