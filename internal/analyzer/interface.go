// Package analyzer provides the Intent Analyzer for zen-brain.
// The Intent Analyzer understands what humans want from work items
// and produces structured BrainTask specifications for execution.
package analyzer

import (
	"context"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// IntentAnalyzer analyzes work items to understand intent and produce task specifications.
type IntentAnalyzer interface {
	// Analyze analyzes a work item and produces BrainTask specifications.
	// This is a multi-stage process that may use the LLM Gateway for understanding.
	Analyze(ctx context.Context, workItem *contracts.WorkItem) (*contracts.AnalysisResult, error)

	// AnalyzeBatch analyzes multiple work items in batch.
	AnalyzeBatch(ctx context.Context, workItems []*contracts.WorkItem) ([]*contracts.AnalysisResult, error)

	// GetAnalysisHistory returns analysis history for a work item.
	GetAnalysisHistory(ctx context.Context, workItemID string) ([]*contracts.AnalysisResult, error)

	// UpdateAnalysis updates an analysis based on new information.
	UpdateAnalysis(ctx context.Context, result *contracts.AnalysisResult) error
}

// Stage represents a stage in the multi-stage analysis pipeline.
type Stage string

const (
	StageClassification Stage = "classification"
	StageRequirements   Stage = "requirements"
	StageBreakdown      Stage = "breakdown"
	StageEvidence       Stage = "evidence"
	StageCostEstimation Stage = "cost_estimation"
	StageFinalization   Stage = "finalization"
)

// StageResult represents the result of a single analysis stage.
type StageResult struct {
	Stage      Stage                  `json:"stage"`
	Input      *contracts.WorkItem    `json:"input,omitempty"`
	Output     map[string]interface{} `json:"output"`
	Confidence float64                `json:"confidence"` // 0.0-1.0
	Notes      string                 `json:"notes,omitempty"`
	Errors     []string               `json:"errors,omitempty"`
	DurationMs int64                  `json:"duration_ms"`
}

// Config holds configuration for the Intent Analyzer.
type Config struct {
	// LLM provider configuration
	LLMProviderName string  `yaml:"llm_provider_name" json:"llm_provider_name"`
	LLMModel        string  `yaml:"llm_model" json:"llm_model"`
	Temperature     float64 `yaml:"temperature" json:"temperature"`
	MaxTokens       int     `yaml:"max_tokens" json:"max_tokens"`

	// Pipeline configuration
	EnabledStages   []Stage `yaml:"enabled_stages" json:"enabled_stages"`
	MaxStages       int     `yaml:"max_stages" json:"max_stages"`
	RequireApproval bool    `yaml:"require_approval" json:"require_approval"`

	// Cost estimation
	DefaultCostPerToken float64 `yaml:"default_cost_per_token" json:"default_cost_per_token"`
	MaxCostUSD          float64 `yaml:"max_cost_usd" json:"max_cost_usd"`

	// Knowledge Base
	KBSearchEnabled bool `yaml:"kb_search_enabled" json:"kb_search_enabled"`
	MaxKBResults    int  `yaml:"max_kb_results" json:"max_kb_results"`

	// Audit (Block 2 enterprise)
	AnalyzedBy      string `yaml:"analyzed_by" json:"analyzed_by"`
	AnalyzerVersion string `yaml:"analyzer_version" json:"analyzer_version"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		LLMProviderName: "glm-4.7",
		LLMModel:        "",
		Temperature:     0.2,
		MaxTokens:       2000,
		EnabledStages: []Stage{
			StageClassification,
			StageRequirements,
			StageBreakdown,
			StageEvidence,
			StageCostEstimation,
			StageFinalization,
		},
		MaxStages:           6,
		RequireApproval:     true,
		DefaultCostPerToken: 0.00002, // $0.02 per 1K tokens
		MaxCostUSD:          10.0,
		KBSearchEnabled:     true,
		MaxKBResults:        5,
		AnalyzedBy:          "zen-brain",
		AnalyzerVersion:     "1.0",
	}
}
