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
	cfg, err := config.LoadConfig("")
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}
	ctx := context.Background()
	rt, err := runtime.Bootstrap(ctx, cfg)
	if err != nil {
		log.Printf("Bootstrap warning: %v", err)
	}
	var report *runtime.RuntimeReport
	if rt != nil {
		report = rt.Report
		defer rt.Close()
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
