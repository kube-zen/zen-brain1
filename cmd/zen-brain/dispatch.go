// Package main: dispatch subcommands (scheduler, factory, mlq, queue, roadmap).
// Thin wrapper around specialized dispatch binaries for unified CLI.
//
// These are continuous supervisors with complex policy logic — keeping them
// as separate binaries allows systemd supervision while providing CLI convenience.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func runDispatchCommand() {
	if len(os.Args) < 3 {
		printDispatchUsage()
		os.Exit(1)
	}

	sub := os.Args[2]
	var bin string

	switch sub {
	case "scheduler":
		bin = "scheduler"
	case "factory":
		bin = "factory-fill"
	case "mlq":
		bin = "mlq-dispatcher"
	case "queue":
		bin = "queue-steward"
	case "roadmap":
		bin = "roadmap-steward"
	case "-h", "--help":
		printDispatchUsage()
		return
	default:
		fmt.Printf("Unknown dispatch subcommand: %s\n", sub)
		printDispatchUsage()
		os.Exit(1)
	}

	// Forward remaining args to the binary
	args := os.Args[3:]
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		fmt.Fprintf(os.Stderr, "Failed to run %s: %v\n", bin, err)
		os.Exit(1)
	}
}

func printDispatchUsage() {
	fmt.Println("Usage: zen-brain dispatch <subcommand> [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  scheduler    Run recurring schedule (useful-task cadence)")
	fmt.Println("  factory      Run factory-fill (backlog-aware dispatch, keep L1 filled)")
	fmt.Println("  mlq          Run mlq-dispatcher (MLQ task dispatcher)")
	fmt.Println("  queue        Run queue-steward (Jira queue state inspector)")
	fmt.Println("  roadmap      Run roadmap-steward (roadmap planning/backlog shaping)")
	fmt.Println()
	fmt.Println("These are continuous supervisors. For detailed options, run:")
	fmt.Println("  zen-brain dispatch <subcommand> --help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  zen-brain dispatch scheduler --force-run")
	fmt.Println("  zen-brain dispatch factory --dry-run")
	fmt.Println("  zen-brain dispatch queue --once")
}
