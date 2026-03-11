// Package main: analyze subcommands (Block 2 operator-facing surface).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/analyzer"
	"github.com/kube-zen/zen-brain1/internal/config"
	internalllm "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/kb"
	"github.com/kube-zen/zen-brain1/pkg/llm"
)

func runAnalyzeCommand() {
	if len(os.Args) < 3 {
		printAnalyzeUsage()
		os.Exit(1)
	}

	sub := os.Args[2]
	switch sub {
	case "work-item":
		runAnalyzeWorkItem()
	case "history":
		runAnalyzeHistory()
	case "compare":
		runAnalyzeCompare()
	case "latest":
		runAnalyzeLatest()
	default:
		fmt.Printf("Unknown analyze subcommand: %s\n", sub)
		printAnalyzeUsage()
		os.Exit(1)
	}
}

func printAnalyzeUsage() {
	fmt.Println("Usage: zen-brain analyze <subcommand> [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  work-item <jira-key>              Analyze a work item and generate rich output")
	fmt.Println("  history <work-item-id>            Show analysis history for a work item")
	fmt.Println("  latest <work-item-id>             Show latest analysis for a work item")
	fmt.Println("  compare <work-item-id> <id1> <id2> Compare two analyses for a work item")
	fmt.Println()
	fmt.Println("Output formats:")
	fmt.Println("  --json                            Output as JSON")
	fmt.Println("  --summary                         Show executive summary only (default)")
	fmt.Println("  --full                            Show complete analysis with audit trail")
}

func runAnalyzeWorkItem() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zen-brain analyze work-item <jira-key>")
		os.Exit(1)
	}

	jiraKey := os.Args[3]
	showFull := hasFlag("--full")
	showJSON := hasFlag("--json")

	// Fetch work item from Jira
	mgr, err := getOfficeManager()
	if err != nil {
		log.Fatalf("Office: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workItem, err := mgr.Fetch(ctx, "default", jiraKey)
	if err != nil {
		log.Fatalf("Fetch work item: %v", err)
	}

	// Build analyzer with history store
	analyzerInst, historyStore, err := buildAnalyzerWithHistory()
	if err != nil {
		log.Fatalf("Build analyzer: %v", err)
	}

	// Analyze
	result, err := analyzerInst.Analyze(ctx, workItem)
	if err != nil {
		log.Fatalf("Analyze: %v", err)
	}

	// Generate rich output
	richResult := analyzer.EnrichForRichAnalysis(result, workItem)

	// Output
	if showJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(richResult); err != nil {
			log.Fatalf("JSON encode: %v", err)
		}
		return
	}

	// Human-readable output
	printRichAnalysisSummary(richResult, showFull)

	// Show history info if available
	if historyStore != nil {
		fmt.Printf("\nAnalysis ID: %s\n", richResult.AuditTrail.AnalysisID)
		fmt.Printf("Replay ID: %s\n", richResult.ReplayID)
		fmt.Printf("Correlation ID: %s\n", richResult.CorrelationID)
	}
}

func runAnalyzeHistory() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zen-brain analyze history <work-item-id>")
		os.Exit(1)
	}

	workItemID := os.Args[3]
	showJSON := hasFlag("--json")

	_, historyStore, err := buildAnalyzerWithHistory()
	if err != nil {
		log.Fatalf("Build analyzer: %v", err)
	}

	if historyStore == nil {
		log.Fatal("History store not configured")
	}

	ctx := context.Background()
	history, err := historyStore.GetHistory(ctx, workItemID)
	if err != nil {
		log.Fatalf("Get history: %v", err)
	}

	if len(history) == 0 {
		fmt.Printf("No analysis history for work item %s\n", workItemID)
		return
	}

	if showJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(history); err != nil {
			log.Fatalf("JSON encode: %v", err)
		}
		return
	}

	// Human-readable summary
	fmt.Printf("Analysis history for %s (%d analyses):\n\n", workItemID, len(history))
	for i, result := range history {
		fmt.Printf("%d. Analysis at %s\n", i+1, result.AnalyzedAt.Format(time.RFC3339))
		fmt.Printf("   Confidence: %.2f  Tasks: %d  Analyzer: %s\n",
			result.Confidence, len(result.BrainTaskSpecs), result.AnalyzerVersion)
		if result.WorkItemSnapshot != nil {
			fmt.Printf("   Jira Key: %s  Work Type: %s\n",
				result.WorkItemSnapshot.SourceKey, result.WorkItemSnapshot.WorkType)
		}
		fmt.Println()
	}
}

func runAnalyzeLatest() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zen-brain analyze latest <work-item-id>")
		os.Exit(1)
	}

	workItemID := os.Args[3]
	showFull := hasFlag("--full")
	showJSON := hasFlag("--json")

	_, historyStore, err := buildAnalyzerWithHistory()
	if err != nil {
		log.Fatalf("Build analyzer: %v", err)
	}

	if historyStore == nil {
		log.Fatal("History store not configured")
	}

	ctx := context.Background()
	history, err := historyStore.GetHistory(ctx, workItemID)
	if err != nil {
		log.Fatalf("Get history: %v", err)
	}

	if len(history) == 0 {
		fmt.Printf("No analysis history for work item %s\n", workItemID)
		return
	}

	latest := history[len(history)-1]

	if showJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(latest); err != nil {
			log.Fatalf("JSON encode: %v", err)
		}
		return
	}

	// Reconstruct work item snapshot for rich output
	var workItem *contracts.WorkItem
	if latest.WorkItemSnapshot != nil {
		workItem = &contracts.WorkItem{
			ID:       latest.WorkItemSnapshot.ID,
			Title:    latest.WorkItemSnapshot.Title,
			WorkType: contracts.WorkType(latest.WorkItemSnapshot.WorkType),
			Source: contracts.SourceMetadata{
				IssueKey: latest.WorkItemSnapshot.SourceKey,
			},
		}
	}

	richResult := analyzer.EnrichForRichAnalysis(latest, workItem)
	printRichAnalysisSummary(richResult, showFull)
}

func runAnalyzeCompare() {
	if len(os.Args) < 6 {
		fmt.Println("Usage: zen-brain analyze compare <work-item-id> <index1> <index2>")
		fmt.Println("  (indices are 1-based, use 'zen-brain analyze history' to see indices)")
		os.Exit(1)
	}

	workItemID := os.Args[3]
	var idx1, idx2 int
	fmt.Sscanf(os.Args[4], "%d", &idx1)
	fmt.Sscanf(os.Args[5], "%d", &idx2)

	_, historyStore, err := buildAnalyzerWithHistory()
	if err != nil {
		log.Fatalf("Build analyzer: %v", err)
	}

	if historyStore == nil {
		log.Fatal("History store not configured")
	}

	ctx := context.Background()
	history, err := historyStore.GetHistory(ctx, workItemID)
	if err != nil {
		log.Fatalf("Get history: %v", err)
	}

	if len(history) == 0 {
		fmt.Printf("No analysis history for work item %s\n", workItemID)
		return
	}

	if idx1 < 1 || idx1 > len(history) || idx2 < 1 || idx2 > len(history) {
		log.Fatalf("Indices out of range (1-%d)", len(history))
	}

	a1 := history[idx1-1]
	a2 := history[idx2-1]

	// Compare
	fmt.Printf("Comparing analyses for %s:\n\n", workItemID)
	fmt.Printf("Analysis 1: %s (confidence %.2f, %d tasks)\n",
		a1.AnalyzedAt.Format(time.RFC3339), a1.Confidence, len(a1.BrainTaskSpecs))
	fmt.Printf("Analysis 2: %s (confidence %.2f, %d tasks)\n\n",
		a2.AnalyzedAt.Format(time.RFC3339), a2.Confidence, len(a2.BrainTaskSpecs))

	// Compare fields
	fmt.Println("Changes:")
	if a1.Confidence != a2.Confidence {
		fmt.Printf("  Confidence: %.2f → %.2f\n", a1.Confidence, a2.Confidence)
	}
	if len(a1.BrainTaskSpecs) != len(a2.BrainTaskSpecs) {
		fmt.Printf("  Task count: %d → %d\n", len(a1.BrainTaskSpecs), len(a2.BrainTaskSpecs))
	}

	// Compare work type
	if a1.WorkItemSnapshot != nil && a2.WorkItemSnapshot != nil {
		if a1.WorkItemSnapshot.WorkType != a2.WorkItemSnapshot.WorkType {
			fmt.Printf("  Work type: %s → %s\n", a1.WorkItemSnapshot.WorkType, a2.WorkItemSnapshot.WorkType)
		}
	}

	fmt.Println("\n✓ Comparison complete")
}

func printRichAnalysisSummary(rich *analyzer.RichAnalysisResult, showFull bool) {
	fmt.Println("=== Analysis Result ===")
	fmt.Println()

	// Executive summary
	if rich.ExecutiveSummary != "" {
		fmt.Println("📋 EXECUTIVE SUMMARY")
		fmt.Println(rich.ExecutiveSummary)
		fmt.Println()
	}

	// Technical summary
	if rich.TechnicalSummary != "" {
		fmt.Println("🔧 TECHNICAL SUMMARY")
		fmt.Println(rich.TechnicalSummary)
		fmt.Println()
	}

	// Task summary
	fmt.Printf("📊 TASK BREAKDOWN\n")
	fmt.Printf("  Total tasks: %d\n", len(rich.BrainTaskSpecs))
	if len(rich.BrainTaskSpecs) > 0 {
		fmt.Printf("  Estimated cost: $%.2f\n", rich.EstimatedTotalCostUSD)
	}
	fmt.Println()

	// Risk assessment
	if rich.RiskAssessment != nil {
		fmt.Printf("⚠️  RISK ASSESSMENT\n")
		fmt.Printf("  Overall risk: %s\n", rich.RiskAssessment.OverallRisk)
		if len(rich.RiskAssessment.RiskFactors) > 0 {
			fmt.Printf("  Risk factors: %d\n", len(rich.RiskAssessment.RiskFactors))
		}
		fmt.Println()
	}

	// Audit trail
	if rich.AuditTrail != nil && showFull {
		fmt.Printf("🔍 AUDIT TRAIL\n")
		fmt.Printf("  Analysis ID: %s\n", rich.AuditTrail.AnalysisID)
		if rich.AuditTrail.JiraKey != "" {
			fmt.Printf("  Jira key: %s\n", rich.AuditTrail.JiraKey)
		}
		fmt.Printf("  Work item source: %s\n", rich.AuditTrail.WorkItemSource)
		fmt.Printf("  Analyzed at: %s\n", rich.AuditTrail.CustodyStart.Format(time.RFC3339))
		fmt.Printf("  Analyzer version: %s\n", rich.AnalyzerVersion)
		if len(rich.AuditTrail.ChainOfTrust) > 0 {
			fmt.Printf("  Chain of trust: %s\n", rich.AuditTrail.ChainOfTrust)
		}
		fmt.Println()
	}

	// Action items (if full output)
	if showFull && len(rich.ActionItems) > 0 {
		fmt.Printf("✅ ACTION ITEMS (%d)\n", len(rich.ActionItems))
		for i, item := range rich.ActionItems[:min(5, len(rich.ActionItems))] {
			fmt.Printf("  %d. [%s] %s\n", i+1, item.Priority, item.Title)
		}
		if len(rich.ActionItems) > 5 {
			fmt.Printf("  ... and %d more\n", len(rich.ActionItems)-5)
		}
		fmt.Println()
	}

	// Confidence
	fmt.Printf("📈 CONFIDENCE: %.2f\n", rich.Confidence)
}

func buildAnalyzerWithHistory() (*analyzer.DefaultAnalyzer, analyzer.AnalysisHistoryStore, error) {
	// Build LLM provider (use defaults)
	llmProvider := buildLLMProvider(nil)

	// Build analyzer config
	analyzerConfig := analyzer.DefaultConfig()

	// Build KB store (optional)
	var kbStore kb.Store // nil is acceptable for basic analysis

	// Create analyzer
	a, err := analyzer.New(analyzerConfig, llmProvider, kbStore)
	if err != nil {
		return nil, nil, fmt.Errorf("create analyzer: %w", err)
	}

	// Attach history store
	historyStore, err := analyzer.NewFileAnalysisStore("/tmp/zen-brain-analysis-history")
	if err != nil {
		// Non-fatal: continue without history
		fmt.Fprintf(os.Stderr, "Warning: could not create history store: %v\n", err)
		return a, nil, nil
	}

	a.HistoryStore = historyStore
	return a, historyStore, nil
}

func buildLLMProvider(cfg *config.Config) llm.Provider {
	// Default to Ollama for local development
	ollamaURL := "http://localhost:11434"
	model := "llama3"
	timeoutSecs := 120
	apiKey := ""

	// Create Ollama provider
	provider := internalllm.NewOllamaProvider(ollamaURL, model, timeoutSecs, apiKey)
	return provider
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args {
		if arg == flag {
			return true
		}
	}
	return false
}
