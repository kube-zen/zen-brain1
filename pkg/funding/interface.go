// Package funding provides the ZenFunding interface for SR&ED/IRAP alignment.
// ZenFunding collects evidence and generates reports for funding compliance.
//
// SR&ED evidence collection is default ON for all sessions unless explicitly disabled.
// This package defines the interfaces for evidence recording and report generation.
package funding

import (
	"context"
	"time"
)

// Program represents a funding program.
type Program string

const (
	ProgramSRED   Program = "sred"
	ProgramIRAP   Program = "irap"
	ProgramMITACS Program = "mitacs"
	ProgramOther  Program = "other"
)

// EvidenceRequirement defines what evidence is required.
type EvidenceRequirement struct {
	// Type is the evidence type (hypothesis_documentation, approach_attempts, time_tracking, outcome_documentation)
	Type string `json:"type"`

	// Frequency indicates how often evidence is required (per_task, continuous, daily, weekly, monthly)
	Frequency string `json:"frequency"`

	// Description explains the requirement
	Description string `json:"description,omitempty"`

	// RequiredFields lists fields that must be present
	RequiredFields []string `json:"required_fields,omitempty"`
}

// Evidence represents a piece of evidence for funding compliance.
type Evidence struct {
	// ID uniquely identifies this evidence
	ID string `json:"id"`

	// TaskID is the task this evidence relates to
	TaskID string `json:"task_id"`

	// SessionID is the session this evidence relates to
	SessionID string `json:"session_id"`

	// ClusterID for multi-cluster context
	ClusterID string `json:"cluster_id,omitempty"`

	// ProjectID for project context
	ProjectID string `json:"project_id"`

	// Program is the funding program (SR&ED, IRAP, etc.)
	Program Program `json:"program"`

	// Type is the evidence type (hypothesis_documentation, approach_attempts, time_tracking, outcome_documentation, etc.)
	Type string `json:"type"`

	// SubType is the evidence subtype (benchmark_run, failure_case, iteration_record, experiment_card)
	SubType string `json:"sub_type,omitempty"`

	// Content contains the evidence data
	Content map[string]interface{} `json:"content"`

	// SREDTags are the SR&ED uncertainty tags
	SREDTags []string `json:"sred_tags,omitempty"`

	// Eligible indicates whether this evidence is eligible for funding
	Eligible bool `json:"eligible"`

	// CostUSD is the associated cost (if applicable)
	CostUSD float64 `json:"cost_usd,omitempty"`

	// TimeInvestedMinutes is the time invested (if applicable)
	TimeInvestedMinutes float64 `json:"time_invested_minutes,omitempty"`

	// CreatedAt is when the evidence was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the evidence was last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// FundingReport represents a funding report.
type FundingReport struct {
	// ID uniquely identifies this report
	ID string `json:"id"`

	// ProjectID is the project this report covers
	ProjectID string `json:"project_id"`

	// Program is the funding program
	Program Program `json:"program"`

	// Type is the report type (t661_technical_narrative, quarterly_progress, irap_technical_report)
	Type string `json:"type"`

	// PeriodStart is the start of the reporting period
	PeriodStart time.Time `json:"period_start"`

	// PeriodEnd is the end of the reporting period
	PeriodEnd time.Time `json:"period_end"`

	// Content contains the report data
	Content map[string]interface{} `json:"content"`

	// GeneratedAt is when the report was generated
	GeneratedAt time.Time `json:"generated_at"`

	// GeneratedBy is who/what generated the report
	GeneratedBy string `json:"generated_by"`

	// Status is the report status (draft, final, submitted)
	Status string `json:"status"`
}

// FundingConfig contains configuration for funding programs.
type FundingConfig struct {
	// ProjectRef is the project reference
	ProjectRef string `json:"project_ref"`

	// Program is the funding program
	Program Program `json:"program"`

	// TaxYear is the tax year
	TaxYear int `json:"tax_year"`

	// EvidenceRequirements are the evidence requirements
	EvidenceRequirements []EvidenceRequirement `json:"evidence_requirements"`

	// Reporting defines reporting schedules
	Reporting []ReportingSchedule `json:"reporting"`

	// SREDDisabled indicates whether SR&ED evidence collection is disabled
	SREDDisabled bool `json:"sred_disabled"`

	// AutoGenerateReports indicates whether reports should be auto-generated
	AutoGenerateReports bool `json:"auto_generate_reports"`
}

// ReportingSchedule defines a reporting schedule.
type ReportingSchedule struct {
	// Type is the report type (t661_technical_narrative, quarterly_progress, etc.)
	Type string `json:"type"`

	// Schedule is the cron expression or frequency
	Schedule string `json:"schedule"`

	// Timezone is the IANA timezone
	Timezone string `json:"timezone,omitempty"`
}

// ReportRequest is a request to generate a funding report.
type ReportRequest struct {
	// ProjectID is the project to report on
	ProjectID string `json:"project_id"`

	// Program is the funding program
	Program Program `json:"program"`

	// Type is the report type
	Type string `json:"type"`

	// PeriodStart is the start of the reporting period
	PeriodStart time.Time `json:"period_start"`

	// PeriodEnd is the end of the reporting period
	PeriodEnd time.Time `json:"period_end"`

	// Format is the output format (json, pdf, markdown, confluence)
	Format string `json:"format,omitempty"`
}

// ZenFunding is the interface for funding evidence collection and reporting.
type ZenFunding interface {
	// GetEvidenceRequirements returns evidence requirements for a program.
	GetEvidenceRequirements(ctx context.Context, program Program) ([]EvidenceRequirement, error)

	// RecordEvidence records evidence for funding compliance.
	RecordEvidence(ctx context.Context, evidence Evidence) error

	// GetEvidence retrieves evidence by ID.
	GetEvidence(ctx context.Context, evidenceID string) (*Evidence, error)

	// QueryEvidence queries evidence by criteria.
	QueryEvidence(ctx context.Context, projectID string, program Program, start, end time.Time) ([]Evidence, error)

	// GenerateReport generates a funding report.
	GenerateReport(ctx context.Context, req ReportRequest) (*FundingReport, error)

	// GetReport retrieves a report by ID.
	GetReport(ctx context.Context, reportID string) (*FundingReport, error)

	// ListReports lists reports for a project and program.
	ListReports(ctx context.Context, projectID string, program Program) ([]FundingReport, error)

	// SubmitReport marks a report as submitted.
	SubmitReport(ctx context.Context, reportID string) error

	// GetConfig returns funding configuration for a project.
	GetConfig(ctx context.Context, projectID string) (*FundingConfig, error)

	// UpdateConfig updates funding configuration for a project.
	UpdateConfig(ctx context.Context, config FundingConfig) error

	// Stats returns funding statistics.
	Stats(ctx context.Context) (map[string]interface{}, error)

	// Close closes the funding service.
	Close() error
}