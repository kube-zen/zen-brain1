// Package apiserver: chat handler for local-worker inference proof (Block 5).
package apiserver

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	llmgateway "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// ChatHandler returns an http.Handler that runs one chat request through the LLM gateway.
// When gateway is nil, responds 503. Uses POST JSON body: {"messages":[{"role":"user","content":"..."}]}.
// If warmup is non-nil and the request forces local-worker, the first request waits briefly for in-progress warmup.
func ChatHandler(gateway *llmgateway.Gateway, warmup *llmgateway.OllamaWarmupCoordinator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if gateway == nil {
			http.Error(w, "gateway not available", http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if len(body.Messages) == 0 {
			http.Error(w, "messages required", http.StatusBadRequest)
			return
		}
		messages := make([]llm.Message, len(body.Messages))
		for i, m := range body.Messages {
			messages[i] = llm.Message{Role: m.Role, Content: m.Content}
		}
		req := llm.ChatRequest{Messages: messages}
		preferred := r.Header.Get("X-LLM-Provider")
		// First local-worker request can wait on in-progress warmup (bounded time).
		if preferred == "local-worker" && warmup != nil {
			warmup.WaitReady(r.Context(), 60*time.Second)
		}
		var resp *llm.ChatResponse
		var err error
		if preferred == "local-worker" {
			resp, err = gateway.ChatWithPreferred(r.Context(), req, "local-worker")
		} else {
			resp, err = gateway.Chat(r.Context(), req)
		}
		if err != nil {
			log.Printf("[apiserver] gateway Chat failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}
