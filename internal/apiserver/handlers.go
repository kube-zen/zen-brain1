// Package apiserver provides REST handlers for Block 3.4 API (sessions, health detail).
package apiserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

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

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
