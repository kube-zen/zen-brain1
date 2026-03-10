// Package apiserver provides optional API key auth so the slice is not open by default (Block 3.4).
package apiserver

import (
	"net/http"
	"strings"
)

// RequireAPIKey returns a middleware that requires X-API-Key or Authorization: Bearer <key>
// when key is non-empty. When key is empty, all requests are allowed (auth disabled).
// Paths in skipPaths (e.g. /healthz, /readyz) are never checked so probes work.
func RequireAPIKey(apiKey string, skipPaths map[string]bool) func(http.Handler) http.Handler {
	if apiKey == "" {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			got := ""
			if k := r.Header.Get("X-API-Key"); k != "" {
				got = k
			} else if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				got = strings.TrimPrefix(auth, "Bearer ")
			}
			if got != apiKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized","message":"missing or invalid API key"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// DefaultSkipPaths are paths that are never subject to API key check.
var DefaultSkipPaths = map[string]bool{
	"/healthz": true,
	"/readyz":  true,
	"/":        true,
}
