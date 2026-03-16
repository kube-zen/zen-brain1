package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kube-zen/zen-brain1/internal/tui"
)

func main() {
	fmt.Println("zen-brain TUI - Thin Terminal Interface")
	fmt.Println()

	// Get server URL from environment or default
	serverURL := os.Getenv("ZEN_BRAIN_SERVER")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// Create TUI client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT/SIGTERM for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create and run TUI
	client := tui.New(serverURL)
	if err := client.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
