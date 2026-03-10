// Package apiserver provides REST handlers for Block 3.4 API (sessions, health detail, evidence).
package apiserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/kube-zen/zen-brain1/internal/evidence"
	"github.com/kube-zen/zen-brain1/internal/session"
)

// SessionSummary is a minimal session view for the API.
type SessionSummary struct {
	ID         string `json:"id"`
	WorkItemID string `json:"work_item_id"`
	SourceKey  string `json:"source_key"`
	State      string `json:"state"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// SessionsHandler returns an http.Handler that lists sessions (GET with optional limit query).
// If manager is nil, returns 503.
func SessionsHandler(manager session.Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if manager == nil {
			http.Error(w, "sessions not available", http.StatusServiceUnavailable)
			return
		}
		filter := session.SessionFilter{Limit: 50}
		if limit := r.URL.Query().Get("limit"); limit != "" {
			if n, err := strconv.Atoi(limit); err == nil && n > 0 && n <= 200 {
				filter.Limit = n
			}
		}
		list, err := manager.ListSessions(r.Context(), filter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		summaries := make([]SessionSummary, 0, len(list))
		for _, s := range list {
			summaries = append(summaries, SessionSummary{
				ID:         s.ID,
				WorkItemID: s.WorkItemID,
				SourceKey:  s.SourceKey,
				State:      string(s.State),
				CreatedAt:  formatTime(s.CreatedAt),
				UpdatedAt:  formatTime(s.UpdatedAt),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"sessions": summaries, "count": len(summaries)})
	})
}

// HealthDetailHandler returns a handler that reports health with optional dependency checks.
// If ledgerPing is non-nil, it is called and its error included in the response.
func HealthDetailHandler(ledgerPing func() error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{"status": "ok"}
		if ledgerPing != nil {
			if err := ledgerPing(); err != nil {
				status["ledger"] = err.Error()
				status["status"] = "degraded"
			} else {
				status["ledger"] = "ok"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	})
}

// VersionInfo is returned by GET /api/v1/version (Block 3.4).
type VersionInfo struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

// VersionHandler returns an http.Handler that serves version info (GET only).
// version may be empty (defaults to "dev"); set at build time or via env.
func VersionHandler(version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if version == "" {
			version = "dev"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(VersionInfo{Service: "zen-brain-apiserver", Version: version})
	})
}

// EvidenceHandler returns an http.Handler that lists evidence by session_id (GET /api/v1/evidence?session_id=xxx).
// When vault is nil, returns 503. Optional for API completeness (Block 5 evidence).
func EvidenceHandler(vault evidence.Vault) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if vault == nil {
			http.Error(w, "evidence not available", http.StatusServiceUnavailable)
			return
		}
		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			http.Error(w, "session_id query required", http.StatusBadRequest)
			return
		}
		list, err := vault.GetBySession(r.Context(), sessionID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"evidence": list, "count": len(list)})
	})
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
