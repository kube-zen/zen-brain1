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
	Addr    string
	Checker zenhealth.Checker
	srv     *http.Server
}

// New creates a new API server with health/readiness using zen-sdk/pkg/health.
func New(addr string, checker zenhealth.Checker) *Server {
	if checker == nil {
		checker = &alwaysReadyChecker{}
	}
	s := &Server{Addr: addr, Checker: checker}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleLiveness)
	mux.HandleFunc("/readyz", s.handleReadiness)
	mux.HandleFunc("/", s.handleRoot)
	s.srv = &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return s
}

// Start starts the HTTP server (blocking until Shutdown).
func (s *Server) Start() error {
	if s.Addr == "" {
		s.Addr = ":8080"
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
	fmt.Fprintf(w, "  /healthz - liveness\n")
	fmt.Fprintf(w, "  /readyz  - readiness\n")
}

// alwaysReadyChecker implements health.Checker for a server that is always ready (e.g. minimal API).
type alwaysReadyChecker struct{}

func (alwaysReadyChecker) ReadinessCheck(*http.Request) error { return nil }
func (alwaysReadyChecker) LivenessCheck(*http.Request) error  { return nil }
func (alwaysReadyChecker) StartupCheck(*http.Request) error   { return nil }
