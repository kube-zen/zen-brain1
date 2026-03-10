package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kube-zen/zen-brain1/internal/intelligence"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func runIntelligence() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: zen-brain intelligence <mine|analyze|recommend> [workType workDomain]")
		os.Exit(1)
	}
	subcommand := os.Args[2]

	runtimeDir := os.Getenv("ZEN_BRAIN_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = "/tmp/zen-brain-factory"
	}
	patternStorePath := filepath.Join(runtimeDir, "patterns")

	patternStore, err := intelligence.NewJSONPatternStore(patternStorePath)
	if err != nil {
		log.Printf("Failed to open pattern store at %s: %v", patternStorePath, err)
		os.Exit(1)
	}

	ctx := context.Background()

	switch subcommand {
	case "mine":
		miner := intelligence.NewMiner(runtimeDir, patternStore)
		result, err := miner.MineProofOfWorks(ctx)
		if err != nil {
			log.Printf("Mining failed: %v", err)
			os.Exit(1)
		}
		fmt.Printf("Artifacts found:    %d\n", result.ArtifactsFound)
		fmt.Printf("Artifacts mined:    %d\n", result.ArtifactsMined)
		fmt.Printf("Patterns extracted: %d\n", result.PatternsExtracted)
		fmt.Printf("Errors:             %d\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}

	case "analyze":
		recommender := intelligence.NewRecommender(patternStore, 3)
		analysis, err := recommender.PatternAnalysis(ctx)
		if err != nil {
			log.Printf("Pattern analysis failed: %v", err)
			os.Exit(1)
		}
		fmt.Print(analysis.FormatAnalysis())

	case "recommend":
		if len(os.Args) < 5 {
			fmt.Println("Usage: zen-brain intelligence recommend <workType> <workDomain>")
			os.Exit(1)
		}
		workType := contracts.WorkType(os.Args[3])
		workDomain := contracts.WorkDomain(os.Args[4])
		recommender := intelligence.NewRecommender(patternStore, 3)
		templateRec, configRec, err := recommender.RecommendAll(ctx, workType, workDomain)
		if err != nil {
			log.Printf("Recommendation failed: %v", err)
			os.Exit(1)
		}
		fmt.Println("Template recommendation:")
		fmt.Printf("  template:   %s\n", templateRec.TemplateName)
		fmt.Printf("  confidence: %.2f\n", templateRec.Confidence)
		fmt.Printf("  reasoning:  %s\n", templateRec.Reasoning)
		fmt.Println("Configuration recommendation:")
		fmt.Printf("  timeout:    %d seconds\n", configRec.TimeoutSeconds)
		fmt.Printf("  retries:    %d\n", configRec.MaxRetries)
		fmt.Printf("  reasoning:  %s\n", configRec.Reasoning)

	default:
		fmt.Printf("Unknown intelligence subcommand: %s\n", subcommand)
		fmt.Println("Use: mine | analyze | recommend")
		os.Exit(1)
	}
}
