// Package factory provides a CLI tool for repo analysis.
// This is designed to be called from shell templates.
//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kube-zen/zen-brain1/internal/factory"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: repo-analyzer <command> [args...]")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  detect              - Detect repo structure and output JSON")
		fmt.Fprintln(os.Stderr, "  select-impl <id>    - Select implementation target path and package name")
		fmt.Fprintln(os.Stderr, "  select-bugfix <objective> <title> - Select bugfix target files")
		fmt.Fprintln(os.Stderr, "  select-refactor     - Select refactor target files")
		os.Exit(1)
	}

	command := os.Args[1]

	// Use current directory as workdir unless specified
	workdir := "."
	for i, arg := range os.Args {
		if arg == "--workdir" && i+1 < len(os.Args) {
			workdir = os.Args[i+1]
			break
		}
	}
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting absolute path for %s: %v\n", workdir, err)
		os.Exit(1)
	}

	switch command {
	case "detect":
		detectRepo(absWorkdir)
	case "select-impl":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: repo-analyzer select-impl <work_item_id> [--workdir <dir>]")
			os.Exit(1)
		}
		selectImplementation(absWorkdir, os.Args[2])
	case "select-bugfix":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: repo-analyzer select-bugfix <objective> <title> [--workdir <dir>]")
			os.Exit(1)
		}
		selectBugfix(absWorkdir, os.Args[2], os.Args[3])
	case "select-refactor":
		selectRefactor(absWorkdir)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func detectRepo(workdir string) {
	info, err := factory.DetectRepoStructure(workdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting repo: %v\n", err)
		os.Exit(1)
	}
	output, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(output))
}

func selectImplementation(workdir, workItemID string) {
	targetPath, packageName, err := factory.SelectImplementationTarget(workdir, workItemID, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error selecting implementation target: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("TARGET_PATH=%s\n", targetPath)
	fmt.Printf("PACKAGE_NAME=%s\n", packageName)
}

func selectBugfix(workdir, objective, title string) {
	candidates, err := factory.SelectBugfixTarget(workdir, objective, title, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error selecting bugfix target: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("CANDIDATE_FILES:")
	for _, c := range candidates {
		fmt.Printf("- %s\n", c)
	}
}

func selectRefactor(workdir string) {
	targets, err := factory.SelectRefactorTarget(workdir, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error selecting refactor target: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("TARGET_FILES:")
	for _, t := range targets {
		fmt.Printf("- %s\n", t)
	}
}
