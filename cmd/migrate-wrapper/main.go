package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	migrateV4 "github.com/kube-zen/zen-brain1/internal/migrate"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate [create|up|down|version]")
		os.Exit(1)
	}

	command := os.Args[1]
	migrationsDir := filepath.Join(os.Getenv("GOPATH"), "src", "github.com", "kube-zen", "zen-brain1", "migrations")

	var args []string
	switch command {
	case "create":
		args = []string{
			"path", migrationsDir,
			"database", "zenbrain",
			"config", "migrate.yaml",
		}
	case "up":
		args = []string{
			"path", migrationsDir,
			"database", "zenbrain",
		}
	case "down":
		args = []string{
			"path", migrationsDir,
			"database", "zenbrain",
		}
	case "version":
		args = []string{"--version"}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}

	// Create migrations directory if it doesn't exist
	if command == "create" {
		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			log.Fatalf("Failed to create migrations directory: %v", err)
		}
		log.Printf("Created migrations directory: %s\n", migrationsDir)
	}

	if err := m.Exec(args...); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	fmt.Printf("Migration %s complete!\n", command)
}
