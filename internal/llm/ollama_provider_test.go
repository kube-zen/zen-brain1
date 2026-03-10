package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/llm"
)

func TestOllamaProvider_Chat(t *testing.T) {
	// Mock Ollama /api/chat response
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "Hello from Ollama",
			},
			"done_reason":           "stop",
			"prompt_eval_count":     10,
			"eval_count":            5,
			"total_duration":        100000000,
			"prompt_eval_duration":  50000000,
			"eval_duration":         50000000,
		})
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	provider := NewOllamaProvider(srv.URL, "test-model", 10)
	resp, err := provider.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "Hello from Ollama" {
		t.Errorf("Content: got %q", resp.Content)
	}
	if resp.Usage == nil || resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 5 {
		t.Errorf("Usage: got %+v", resp.Usage)
	}
	// LatencyMs may be 0 on very fast local/httptest responses
	if resp.LatencyMs < 0 {
		t.Errorf("LatencyMs: got %d", resp.LatencyMs)
	}
}

func TestOllamaProvider_Embed_NotSupported(t *testing.T) {
	provider := NewOllamaProvider("http://localhost:11434", "test", 5)
	_, err := provider.Embed(context.Background(), llm.EmbeddingRequest{Input: "x"})
	if err != llm.ErrEmbeddingNotSupported {
		t.Errorf("Embed: want ErrEmbeddingNotSupported, got %v", err)
	}
}
