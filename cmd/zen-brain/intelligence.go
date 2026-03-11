package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/intelligence"
	"github.com/kube-zen/zen-brain1/internal/session"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func runIntelligence() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: zen-brain intelligence <mine|analyze|recommend|diagnose|checkpoint> [args]")
		os.Exit(1)
	}
	subcommand := os.Args[2]

	// Use ZEN_BRAIN_RUNTIME_DIR if set, otherwise use ZEN_BRAIN_HOME/runtime
	// Fall back to /tmp only in dev mode
	runtimeDir := os.Getenv("ZEN_BRAIN_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(config.HomeDir(), "runtime")
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
		fmt.Printf("Failure stats:      %d\n", len(result.FailureStatistics))
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

	case "diagnose":
		if len(os.Args) < 5 {
			fmt.Println("Usage: zen-brain intelligence diagnose <workType> <workDomain>")
			os.Exit(1)
		}
		workType := os.Args[3]
		workDomain := os.Args[4]

		fmt.Printf("Enhanced Failure Diagnosis for %s/%s:\n", workType, workDomain)
		fmt.Println()

		// INTEGRATION: Use enhanced failure analyzer
		failureAnalyzer := intelligence.NewFailureAnalyzer(patternStore)
		
		// Get root cause analysis
		rootCauses, err := failureAnalyzer.AnalyzeRootCauses(ctx)
		if err != nil {
			fmt.Printf("Root cause analysis failed: %v\n", err)
		} else {
			// Filter to relevant work type/domain
			relevantRootCauses := []intelligence.RootCauseAnalysis{}
			for _, rc := range rootCauses {
				if rc.WorkType == workType && rc.WorkDomain == workDomain {
					relevantRootCauses = append(relevantRootCauses, rc)
				}
			}
			
			if len(relevantRootCauses) > 0 {
				fmt.Println("ROOT CAUSE ANALYSIS:")
				for i, rc := range relevantRootCauses {
					fmt.Printf("\n%d. %s (confidence: %.2f)\n", i+1, rc.RootCause, rc.Confidence)
					fmt.Printf("   Failure Mode:  %s\n", rc.FailureMode)
					fmt.Printf("   Occurrences:   %d\n", rc.Occurrences)
					if len(rc.Evidence) > 0 {
						fmt.Printf("   Evidence:\n")
						for _, e := range rc.Evidence {
							fmt.Printf("     - %s\n", e)
						}
					}
					if rc.MitigationStrategy != "" {
						fmt.Printf("   Mitigation:    %s\n", rc.MitigationStrategy)
					}
				}
				fmt.Println()
			}
		}

		// Get correlations
		correlations, err := failureAnalyzer.AnalyzeCorrelations(ctx)
		if err != nil {
			fmt.Printf("Correlation analysis failed: %v\n", err)
		} else {
			// Filter to relevant
			relevantCorrelations := []intelligence.FailureCorrelation{}
			for _, c := range correlations {
				if strings.Contains(c.FailureMode, workType) || strings.Contains(c.CorrelatedFactor, workDomain) {
					relevantCorrelations = append(relevantCorrelations, c)
				}
			}
			
			if len(relevantCorrelations) > 0 {
				fmt.Println("FAILURE CORRELATIONS:")
				for i, c := range relevantCorrelations {
					fmt.Printf("%d. %s <-> %s\n", i+1, c.FailureMode, c.CorrelatedFactor)
					fmt.Printf("   Type:     %s\n", c.CorrelationType)
					fmt.Printf("   Strength: %.2f (sample size: %d)\n", c.Strength, c.SampleSize)
				}
				fmt.Println()
			}
		}

		// Get predictive model
		model, err := failureAnalyzer.BuildPredictiveModel(ctx, workType, workDomain)
		if err != nil {
			fmt.Printf("Predictive model failed: %v\n", err)
		} else if model != nil {
			fmt.Println("PREDICTIVE MODEL:")
			fmt.Printf("   Predicted Failure: %s (probability: %.2f)\n", model.PredictedFailureMode, model.Probability)
			fmt.Printf("   Confidence:        %.2f\n", model.Confidence)
			if len(model.RiskFactors) > 0 {
				fmt.Printf("   Risk Factors:\n")
				for _, rf := range model.RiskFactors {
					fmt.Printf("     - %s\n", rf)
				}
			}
			fmt.Println()
		}

		// Fallback to basic statistics if enhanced analysis not available
		failureStats, err := patternStore.GetFailureStats(ctx, workType, workDomain)
		if err != nil {
			fmt.Printf("No basic failure statistics found: %v\n", err)
			return
		}

		fmt.Println("BASIC STATISTICS:")
		fmt.Printf("Total failures:      %d\n", failureStats.TotalFailures)

		if failureStats.TemplateName != "" {
			fmt.Printf("Template:            %s\n", failureStats.TemplateName)
		}

		// Find top failure mode
		topFailureMode := ""
		topFailureCount := 0
		for mode, count := range failureStats.FailureModes {
			if count > topFailureCount {
				topFailureMode = mode
				topFailureCount = count
			}
		}

		if topFailureMode != "" {
			fmt.Printf("Top failure mode:     %s (%d occurrences)\n", topFailureMode, topFailureCount)
		}

		// Find most common recommended action
		topAction := ""
		topActionCount := 0
		for action, count := range failureStats.RecommendedActions {
			if count > topActionCount {
				topAction = action
				topActionCount = count
			}
		}

		if topAction != "" {
			fmt.Printf("Most common action:   %s (%d times)\n", topAction, topActionCount)
		}

		if !failureStats.LastFailureAt.IsZero() {
			fmt.Printf("Last failure:        %s\n", failureStats.LastFailureAt.Format("2006-01-02 15:04:05"))
		}

		if len(failureStats.FailureModes) > 0 {
			fmt.Println("\nAll failure modes:")
			for mode, count := range failureStats.FailureModes {
				fmt.Printf("  %s: %d\n", mode, count)
			}
		}

	case "checkpoint":
		if len(os.Args) < 4 {
			fmt.Println("Usage: zen-brain intelligence checkpoint <sessionID>")
			os.Exit(1)
		}
		sessionID := os.Args[3]

		// Initialize session manager
		sessionConfig := session.DefaultConfig()
		sessionConfig.DataDir = filepath.Join(runtimeDir, "sessions")
		sessionConfig.ZenContext = getZenContext()

		sessionMgr, err := session.New(sessionConfig, nil)
		if err != nil {
			log.Printf("Failed to create session manager: %v", err)
			os.Exit(1)
		}
		defer sessionMgr.Close()

		summary, err := sessionMgr.GetExecutionCheckpointSummary(ctx, sessionID)
		if err != nil {
			log.Printf("Failed to get checkpoint summary: %v", err)
			os.Exit(1)
		}

		fmt.Print(summary)

	default:
		fmt.Printf("Unknown intelligence subcommand: %s\n", subcommand)
		fmt.Println("Use: mine | analyze | recommend | diagnose | checkpoint")
		os.Exit(1)
	}
}
