// Package llm: official Ollama warmup — preload via /api/generate, verify via /api/chat, single-flight coordination.
// Matches Ollama docs and zen-brain 0.1: pull once, preload officially, keep resident (keep_alive), verify on real chat path.

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultKeepAlive is the default keep_alive for preload and verify (OLLAMA_KEEP_ALIVE or "30m").
const DefaultKeepAlive = "30m"

// WarmupResult holds the result of a warmup run for logging and metrics.
type WarmupResult struct {
	Model        string
	Success      bool
	LoadDuration time.Duration
	Duration     time.Duration
	Err          error
	At           time.Time
}

// OllamaWarmupCoordinator runs preload+verify once per process and lets the first real request wait on it.
type OllamaWarmupCoordinator struct {
	baseURL      string
	model        string
	keepAlive    string
	timeoutSec   int
	mu           sync.Mutex
	done         bool
	result       WarmupResult
	warmupStart  time.Time
	waitCh       chan struct{} // closed when warmup finishes
}

// NewOllamaWarmupCoordinator returns a coordinator for the given Ollama base URL and model.
// keepAlive is passed to Ollama (e.g. "30m", "-1"); empty uses DefaultKeepAlive.
func NewOllamaWarmupCoordinator(baseURL, model, keepAlive string, timeoutSec int) *OllamaWarmupCoordinator {
	baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if keepAlive == "" {
		keepAlive = DefaultKeepAlive
	}
	if timeoutSec <= 0 {
		timeoutSec = 300
	}
	return &OllamaWarmupCoordinator{
		baseURL:    baseURL,
		model:      model,
		keepAlive:  keepAlive,
		timeoutSec: timeoutSec,
		waitCh:     make(chan struct{}),
	}
}

// DoWarmup runs preload (/api/generate with keep_alive) then verify (/api/chat tiny message) once (single-flight).
// Safe to call from a goroutine at startup. Records result and closes waitCh when done.
func (c *OllamaWarmupCoordinator) DoWarmup(ctx context.Context) {
	c.mu.Lock()
	if c.done {
		c.mu.Unlock()
		return
	}
	c.warmupStart = time.Now()
	c.mu.Unlock()

	timeout := time.Duration(c.timeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var loadDuration time.Duration
	err := preloadGenerate(ctx, c.baseURL, c.model, c.keepAlive, timeout)
	if err != nil {
		c.recordResult(false, 0, time.Since(c.warmupStart), err)
		return
	}
	loadDuration, err = verifyChat(ctx, c.baseURL, c.model, c.keepAlive, timeout)
	if err != nil {
		c.recordResult(false, loadDuration, time.Since(c.warmupStart), err)
		return
	}
	c.recordResult(true, loadDuration, time.Since(c.warmupStart), nil)
}

func (c *OllamaWarmupCoordinator) recordResult(success bool, loadDuration time.Duration, total time.Duration, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.done = true
	c.result = WarmupResult{
		Model:        c.model,
		Success:      success,
		LoadDuration: loadDuration,
		Duration:     total,
		Err:          err,
		At:           time.Now(),
	}
	close(c.waitCh)
	if success {
		log.Printf("[Ollama] warmup done: model=%s load_duration=%v total=%v keep_alive=%s", c.model, loadDuration.Round(time.Millisecond), total.Round(time.Millisecond), c.keepAlive)
	} else {
		log.Printf("[apiserver] Ollama warmup failed (non-fatal): %v", err)
	}
}

// WaitReady blocks until warmup has finished or ctx/maxWait expires. Use before first local-worker request.
// If warmup already succeeded, returns immediately. If warmup failed, returns without waiting (caller proceeds anyway).
func (c *OllamaWarmupCoordinator) WaitReady(ctx context.Context, maxWait time.Duration) {
	c.mu.Lock()
	done := c.done
	waitCh := c.waitCh
	c.mu.Unlock()
	if done {
		return
	}
	deadline := time.Now().Add(maxWait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-waitCh:
		return
	case <-timer.C:
		return
	case <-ctx.Done():
		return
	}
}

// Result returns the last warmup result (and whether warmup has run).
func (c *OllamaWarmupCoordinator) Result() (WarmupResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.result, c.done
}

// preloadGenerate calls POST /api/generate with empty prompt and keep_alive to load the model (official preload).
func preloadGenerate(ctx context.Context, baseURL, model, keepAlive string, timeout time.Duration) error {
	body := map[string]interface{}{
		"model":      model,
		"prompt":     "",
		"keep_alive": keepAlive,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("preload generate: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("preload generate status %d", resp.StatusCode)
	}
	log.Printf("[Ollama] preload done: model=%s keep_alive=%s", model, keepAlive)
	return nil
}

// verifyChat calls POST /api/chat with a tiny message and keep_alive to verify the real app path; returns load_duration from response.
func verifyChat(ctx context.Context, baseURL, model, keepAlive string, timeout time.Duration) (loadDuration time.Duration, err error) {
	body := map[string]interface{}{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "."}},
		"stream":     false,
		"keep_alive": keepAlive,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("verify chat: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("verify chat status %d", resp.StatusCode)
	}
	var out struct {
		LoadDuration int64 `json:"load_duration"` // nanoseconds
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		return 0, fmt.Errorf("verify chat decode: %w", err)
	}
	loadDuration = time.Duration(out.LoadDuration)
	log.Printf("[Ollama] verify chat done: model=%s load_duration=%v", model, loadDuration.Round(time.Millisecond))
	return loadDuration, nil
}
