package main

import (
	"fmt"
	"os"

	"github.com/kube-zen/zen-brain1/internal/config"
)

// Build-time variables (set via Makefile)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	fmt.Printf("zen-brain %s (built %s)\n", Version, BuildTime)
	fmt.Printf("Home directory: %s\n", config.HomeDir())

	// Ensure home directory exists
	if err := config.EnsureHomeDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating home directory: %v\n", err)
		os.Exit(1)
	}

	// Ensure all standard paths exist
	paths := config.DefaultPaths()
	if err := paths.EnsureAll(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directories: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Directories initialized successfully.")
}
