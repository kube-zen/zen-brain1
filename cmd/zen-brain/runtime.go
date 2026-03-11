package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/runtime"
)

func runRuntime() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: zen-brain runtime <doctor|report|ping>")
		os.Exit(1)
	}
	sub := os.Args[2]

	// Block 3: Use StrictRuntime for canonical enforcement
	profile := os.Getenv("ZEN_RUNTIME_PROFILE")
	if profile == "" {
		profile = "dev"
	}

	cfg, err := config.LoadConfig("")
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}

	ctx := context.Background()
	strictRT, err := runtime.NewStrictRuntime(ctx, &runtime.StrictRuntimeConfig{
		Profile:        profile,
		Config:         cfg,
		EnableHealthCh: false, // No need for background health checks in CLI
	})

	if err != nil {
		// In strict mode (prod/staging), fail immediately
		if profile == "prod" || profile == "staging" {
			log.Fatalf("Strict runtime bootstrap failed: %v", err)
		}
		// In dev mode, continue with warning
		log.Printf("Runtime bootstrap warning (dev mode): %v", err)
	}

	var report *runtime.RuntimeReport
	if strictRT != nil {
		report = strictRT.Report()
		defer strictRT.Close()
	}

	switch sub {
	case "doctor":
		runtime.Doctor(ctx, cfg, report)
	case "report":
		if err := runtime.ReportJSON(report); err != nil {
			log.Printf("report: %v", err)
			os.Exit(1)
		}
	case "ping":
		os.Exit(runtime.Ping(report))
	default:
		fmt.Printf("Unknown runtime subcommand: %s\n", sub)
		fmt.Println("Use: doctor | report | ping")
		os.Exit(1)
	}
}
