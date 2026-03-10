// Package apiserver provides the Block 3.4 API server: REST surface, health and readiness.
package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	zenhealth "github.com/kube-zen/zen-sdk/pkg/health"
)

// Server is the zen-brain API server (Block 3.4).
type Server struct {
	Addr       string
	Checker    zenhealth.Checker
	AuthAPIKey string // when set, require X-API-Key or Authorization: Bearer (skipPaths exempt)
	mux        *http.ServeMux
	srv        *http.Server
}

// New creates a new API server with health/readiness using zen-sdk/pkg/health.
func New(addr string, checker zenhealth.Checker) *Server {
	if checker == nil {
		checker = &alwaysReadyChecker{}
	}
	s := &Server{Addr: addr, Checker: checker, mux: http.NewServeMux()}
	s.mux.HandleFunc("/healthz", s.handleLiveness)
	s.mux.HandleFunc("/readyz", s.handleReadiness)
	s.mux.HandleFunc("/", s.handleRoot)
	s.srv = &http.Server{Addr: addr, Handler: s.mux, ReadHeaderTimeout: 5 * time.Second}
	return s
}

// Handle registers an handler for the given pattern (Block 3.4 extended API).
func (s *Server) Handle(pattern string, handler http.Handler) {
	if s.mux != nil {
		s.mux.Handle(pattern, handler)
	}
}

// HandleFunc registers a handler function for the given pattern.
func (s *Server) HandleFunc(pattern string, fn func(http.ResponseWriter, *http.Request)) {
	if s.mux != nil {
		s.mux.HandleFunc(pattern, fn)
	}
}

// Start starts the HTTP server (blocking until Shutdown).
// If AuthAPIKey is set, wraps the mux with API key auth (healthz/readyz/ excluded).
func (s *Server) Start() error {
	if s.Addr == "" {
		s.Addr = ":8080"
	}
	if s.AuthAPIKey != "" {
		s.srv.Handler = RequireAPIKey(s.AuthAPIKey, DefaultSkipPaths)(s.mux)
	}
	return s.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if err := s.Checker.LivenessCheck(r); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if err := s.Checker.ReadinessCheck(r); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "zen-brain API server\n")
	fmt.Fprintf(w, "  /healthz       - liveness\n")
	fmt.Fprintf(w, "  /readyz       - readiness\n")
	fmt.Fprintf(w, "  /api/v1/sessions   - list sessions\n")
	fmt.Fprintf(w, "  /api/v1/sessions/:id - get session by id\n")
	fmt.Fprintf(w, "  /api/v1/health    - health detail (optional ledger ping)\n")
	fmt.Fprintf(w, "  /api/v1/version   - service version\n")
	fmt.Fprintf(w, "  /api/v1/evidence  - list evidence by session_id query (optional vault)\n")
}

// alwaysReadyChecker implements health.Checker for a server that is always ready (e.g. minimal API).
type alwaysReadyChecker struct{}

func (alwaysReadyChecker) ReadinessCheck(*http.Request) error { return nil }
func (alwaysReadyChecker) LivenessCheck(*http.Request) error  { return nil }
func (alwaysReadyChecker) StartupCheck(*http.Request) error   { return nil }
