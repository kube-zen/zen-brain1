// Package apiserver provides Block 3.4 API server handler tests.
package apiserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/internal/evidence"
	"github.com/kube-zen/zen-brain1/internal/session"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestVersionHandler(t *testing.T) {
	handler := VersionHandler("1.2.3")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/version: got status %d", rec.Code)
	}
	var v VersionInfo
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	if v.Service != "zen-brain-apiserver" || v.Version != "1.2.3" {
		t.Errorf("version: got %+v", v)
	}
	// POST not allowed
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/version", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got status %d", rec2.Code)
	}
}

func TestVersionHandler_DefaultDev(t *testing.T) {
	handler := VersionHandler("")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var v VersionInfo
	_ = json.NewDecoder(rec.Body).Decode(&v)
	if v.Version != "dev" {
		t.Errorf("empty version should default to dev, got %q", v.Version)
	}
}

func TestHealthDetailHandler(t *testing.T) {
	handler := HealthDetailHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d", rec.Code)
	}
	var m map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["status"] != "ok" {
		t.Errorf("status: got %v", m["status"])
	}
}

func TestHealthDetailHandler_WithLedgerPing(t *testing.T) {
	handler := HealthDetailHandler(func() error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var m map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&m)
	if m["ledger"] != "ok" {
		t.Errorf("ledger: got %v", m["ledger"])
	}
}

func TestEvidenceHandler_NilVault(t *testing.T) {
	handler := EvidenceHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evidence?session_id=s1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil vault: got status %d", rec.Code)
	}
}

func TestEvidenceHandler_NoSessionID(t *testing.T) {
	handler := EvidenceHandler(evidence.NewMemoryVault())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evidence", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing session_id: got status %d", rec.Code)
	}
}

func TestEvidenceHandler_WithVault(t *testing.T) {
	v := evidence.NewMemoryVault()
	_ = v.Store(context.Background(), contracts.EvidenceItem{
		ID:          "e1",
		SessionID:   "sess-1",
		Type:        contracts.EvidenceTypeProofOfWork,
		Content:     "path/to/pow",
		CollectedAt: time.Now(),
		CollectedBy: "test",
	})
	handler := EvidenceHandler(v)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evidence?session_id=sess-1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d", rec.Code)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["count"].(float64) != 1 {
		t.Errorf("count: got %v", out["count"])
	}
}

func TestSessionsHandler_NilManager(t *testing.T) {
	handler := SessionsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil manager: got status %d", rec.Code)
	}
}

func TestSessionsHandler_WithManager(t *testing.T) {
	cfg := session.DefaultConfig()
	cfg.CleanupInterval = 0
	mgr, err := session.New(cfg, session.NewMemoryStore())
	if err != nil {
		t.Fatalf("session manager: %v", err)
	}
	handler := SessionsHandler(mgr)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d", rec.Code)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["count"].(float64) != 0 {
		t.Errorf("count: got %v", out["count"])
	}
}

func TestSessionsHandler_LimitQuery(t *testing.T) {
	cfg := session.DefaultConfig()
	cfg.CleanupInterval = 0
	mgr, err := session.New(cfg, session.NewMemoryStore())
	if err != nil {
		t.Fatalf("session manager: %v", err)
	}
	handler := SessionsHandler(mgr)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?limit=10", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d", rec.Code)
	}
}

func TestRequireAPIKey_EmptyKey(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	wrapped := RequireAPIKey("", DefaultSkipPaths)(next)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("empty key should pass: got %d", rec.Code)
	}
}

func TestRequireAPIKey_SkipPaths(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	wrapped := RequireAPIKey("secret", DefaultSkipPaths)(next)
	for _, path := range []string{"/healthz", "/readyz", "/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("%s: got %d", path, rec.Code)
		}
	}
}

func TestRequireAPIKey_RequiresKey(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	wrapped := RequireAPIKey("secret", DefaultSkipPaths)(next)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no key: got %d", rec.Code)
	}
	req.Header.Set("X-API-Key", "secret")
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req)
	if rec2.Code != 200 {
		t.Errorf("with X-API-Key: got %d", rec2.Code)
	}
}

func TestServer_Handle(t *testing.T) {
	srv := New(":0", nil)
	srv.HandleFunc("/custom", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Errorf("custom: %d %s", rec.Code, rec.Body.String())
	}
}

func TestServer_HandleRoot(t *testing.T) {
	srv := New(":0", nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.handleRoot(rec, req)
	if rec.Code != 200 {
		t.Errorf("root: %d", rec.Code)
	}
	if rec.Body.String() == "" {
		t.Error("root should return body")
	}
}
