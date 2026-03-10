// Command apiserver runs the zen-brain API server (Block 3.4).
// Serves /healthz, /readyz and optional future REST endpoints.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kube-zen/zen-brain1/internal/apiserver"
)

func main() {
	addr := ":8080"
	if p := os.Getenv("API_SERVER_PORT"); p != "" {
		addr = ":" + p
	}
	srv := apiserver.New(addr, nil)
	srv.Handle("/api/v1/sessions", apiserver.SessionsHandler(nil))
	srv.Handle("/api/v1/health", apiserver.HealthDetailHandler(nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil && err != context.Canceled {
			log.Printf("API server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down API server...")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}
