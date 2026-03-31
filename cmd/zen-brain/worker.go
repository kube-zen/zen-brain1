// Package main: worker subcommands (remediate, batch, ticketize).
// Consolidated from cmd/remediation-worker, cmd/useful-batch, cmd/finding-ticketizer.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func runWorkerCommand() {
	if len(os.Args) < 3 {
		printWorkerUsage()
		os.Exit(1)
	}

	sub := os.Args[2]
	switch sub {
	case "remediate":
		runWorkerRemediate()
	case "batch":
		runWorkerBatch()
	case "ticketize":
		runWorkerTicketize()
	case "-h", "--help":
		printWorkerUsage()
	default:
		fmt.Printf("Unknown worker subcommand: %s\n", sub)
		printWorkerUsage()
		os.Exit(1)
	}
}

func printWorkerUsage() {
	fmt.Println("Usage: zen-brain worker <subcommand> [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  remediate   Run remediation worker (fix tasks from Jira)")
	fmt.Println("  batch       Run useful-batch worker (continuous discovery tasks)")
	fmt.Println("  ticketize   Run finding-ticketizer (convert findings to Jira tickets)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  zen-brain worker remediate        (reads MODE/PILOT_KEYS env)")
	fmt.Println("  zen-brain worker batch --once")
	fmt.Println("  zen-brain worker ticketize --dir /var/lib/zen-brain1/artifacts")
}

// ═══════════════════════════════════════════════════════════════════════════════
func runWorkerRemediate() {
	// Thin passthrough to real remediation-worker binary.
	// Factory-fill passes context via env vars (MODE, PILOT_KEYS, L1_ENDPOINT,
	// L1_MODEL, REPO_ROOT, ARTIFACT_ROOT, EVIDENCE_ROOT, JIRA_*, REMEDIATION_TIMEOUT,
	// RESULT_DIR) which the real binary reads directly.
	// The --ticket-key flag is only used for standalone invocation (not subprocess mode).
	bin := findImplementationBinary("remediation-worker")

	cmd := exec.Command(bin)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		log.Fatalf("[REMEDIATE] failed to run %s: %v", bin, err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKER: BATCH (from cmd/useful-batch)
// ═══════════════════════════════════════════════════════════════════════════════

// runWorkerBatch exec's into the real useful-batch binary.
// The scheduler passes context via env vars (TASKS, BATCH_NAME, OUTPUT_ROOT,
// WORKERS, TIMEOUT) which the real binary reads directly.
// This makes `zen-brain worker batch` the canonical invocation contract
// while the implementation binary provides the actual logic.
func runWorkerBatch() {
	bin := findImplementationBinary("useful-batch")
	args := []string{}
	for _, a := range os.Args[3:] {
		args = append(args, a)
	}

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
		log.Fatalf("[BATCH] failed to run %s: %v", bin, err)
	}
}

// findImplementationBinary locates a runtime implementation binary.
// Search order: same directory as current binary, then PATH.
func findImplementationBinary(name string) string {
	// Same directory as current executable
	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// PATH lookup
	if p, err := exec.LookPath(name); err == nil {
		return p
	}

	log.Fatalf("[CANONICAL] implementation binary %q not found (searched same-dir and PATH)", name)
	return ""
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKER: TICKETIZE (from cmd/finding-ticketizer)
// ═══════════════════════════════════════════════════════════════════════════════

// runWorkerTicketize exec's into the real finding-ticketizer binary.
// Passes through all args; the real binary reads env vars for Jira config.
func runWorkerTicketize() {
	bin := findImplementationBinary("finding-ticketizer")
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
		log.Fatalf("[TICKETIZE] failed to run %s: %v", bin, err)
	}
}
