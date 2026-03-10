// Package funding provides the Funding Evidence Aggregator for Block 5.4.
// It produces SR&ED T661 narratives and IRAP technical reports from accumulated evidence.
package funding

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/evidence"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// T661Narrative holds the SR&ED T661 technical narrative (Part 2, Section B).
// Line 242: uncertainties; Line 244: work performed; Line 246: advancements.
type T661Narrative struct {
	// ProjectTitle is the project or claim title.
	ProjectTitle string `json:"project_title"`
	// PeriodStart and PeriodEnd define the claim period.
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	// ScientificTechnologicalUncertainties (Line 242, ~350 words).
	ScientificTechnologicalUncertainties string `json:"scientific_technological_uncertainties"`
	// WorkPerformedToOvercome (Line 244, ~700 words).
	WorkPerformedToOvercome string `json:"work_performed_to_overcome"`
	// AdvancementsAchieved (Line 246, ~350 words).
	AdvancementsAchieved string `json:"advancements_achieved"`
	// EvidenceSummary lists evidence IDs and types used.
	EvidenceSummary []EvidenceRef `json:"evidence_summary"`
}

// EvidenceRef references a single evidence item in the narrative.
type EvidenceRef struct {
	ID       string               `json:"id"`
	Type     contracts.EvidenceType `json:"type"`
	SessionID string              `json:"session_id"`
	CollectedAt time.Time         `json:"collected_at"`
}

// IRAPReport is a technical report structure (e.g. for IRAP or internal use).
type IRAPReport struct {
	Title    string    `json:"title"`
	Date     time.Time `json:"date"`
	Summary  string    `json:"summary"`
	Sections []Section `json:"sections"`
	// EvidenceRefs lists all evidence referenced.
	EvidenceRefs []EvidenceRef `json:"evidence_refs"`
}

// Section is a named section of a report.
type Section struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// FundingReport combines T661 narrative and IRAP report for a set of sessions.
type FundingReport struct {
	T661   *T661Narrative `json:"t661,omitempty"`
	IRAP   *IRAPReport    `json:"irap,omitempty"`
	SessionIDs []string   `json:"session_ids"`
	GeneratedAt time.Time `json:"generated_at"`
}

// Aggregator produces funding reports from evidence in the vault.
type Aggregator struct {
	Vault evidence.Vault
}

// NewAggregator returns an aggregator that reads from the given vault.
func NewAggregator(v evidence.Vault) *Aggregator {
	return &Aggregator{Vault: v}
}

// AggregateForSessions builds a FundingReport from evidence for the given session IDs.
func (a *Aggregator) AggregateForSessions(ctx context.Context, sessionIDs []string, projectTitle string) (*FundingReport, error) {
	if a.Vault == nil {
		return nil, fmt.Errorf("vault is nil")
	}
	var all []contracts.EvidenceItem
	for _, sid := range sessionIDs {
		items, err := a.Vault.GetBySession(ctx, sid)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
	}
	return a.buildReport(all, sessionIDs, projectTitle), nil
}

// AggregateForSession is a convenience for a single session.
func (a *Aggregator) AggregateForSession(ctx context.Context, sessionID string, projectTitle string) (*FundingReport, error) {
	return a.AggregateForSessions(ctx, []string{sessionID}, projectTitle)
}

func (a *Aggregator) buildReport(items []contracts.EvidenceItem, sessionIDs []string, projectTitle string) *FundingReport {
	now := time.Now()
	var periodStart, periodEnd time.Time
	for i, e := range items {
		if i == 0 || e.CollectedAt.Before(periodStart) {
			periodStart = e.CollectedAt
		}
		if e.CollectedAt.After(periodEnd) {
			periodEnd = e.CollectedAt
		}
	}
	if periodEnd.IsZero() {
		periodEnd = now
	}
	if periodStart.IsZero() {
		periodStart = periodEnd
	}

	refs := make([]EvidenceRef, 0, len(items))
	for _, e := range items {
		refs = append(refs, EvidenceRef{ID: e.ID, Type: e.Type, SessionID: e.SessionID, CollectedAt: e.CollectedAt})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].CollectedAt.Before(refs[j].CollectedAt) })

	// Build T661 sections from evidence by type
	var uncertainties, workPerformed, advancements []string
	for _, e := range items {
		excerpt := truncate(e.Content, 500)
		switch e.Type {
		case contracts.EvidenceTypeHypothesis, contracts.EvidenceTypeExperiment, contracts.EvidenceTypeObservation:
			uncertainties = append(uncertainties, excerpt)
		case contracts.EvidenceTypeAnalysis, contracts.EvidenceTypeMeasurement, contracts.EvidenceTypeExecutionLog, contracts.EvidenceTypeProofOfWork:
			workPerformed = append(workPerformed, excerpt)
		case contracts.EvidenceTypeConclusion:
			advancements = append(advancements, excerpt)
		default:
			workPerformed = append(workPerformed, excerpt)
		}
	}

	t661 := &T661Narrative{
		ProjectTitle:                        projectTitle,
		PeriodStart:                         periodStart,
		PeriodEnd:                           periodEnd,
		ScientificTechnologicalUncertainties: strings.Join(uncertainties, "\n\n"),
		WorkPerformedToOvercome:             strings.Join(workPerformed, "\n\n"),
		AdvancementsAchieved:                strings.Join(advancements, "\n\n"),
		EvidenceSummary:                     refs,
	}

	// IRAP report: summary + sections from evidence
	sections := []Section{
		{Title: "Uncertainties", Content: t661.ScientificTechnologicalUncertainties},
		{Title: "Work Performed", Content: t661.WorkPerformedToOvercome},
		{Title: "Advancements", Content: t661.AdvancementsAchieved},
	}
	irap := &IRAPReport{
		Title:        projectTitle + " – Technical Report",
		Date:         now,
		Summary:      fmt.Sprintf("Report generated from %d evidence items across %d session(s).", len(items), len(sessionIDs)),
		Sections:     sections,
		EvidenceRefs: refs,
	}

	return &FundingReport{
		T661:        t661,
		IRAP:        irap,
		SessionIDs:  sessionIDs,
		GeneratedAt: now,
	}
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// T661Text returns the T661 narrative as plain text (e.g. for form paste).
func (t *T661Narrative) T661Text() string {
	var b strings.Builder
	b.WriteString("Project: " + t.ProjectTitle + "\n")
	b.WriteString("Period: " + t.PeriodStart.Format("2006-01-02") + " to " + t.PeriodEnd.Format("2006-01-02") + "\n\n")
	b.WriteString("Scientific or technological uncertainties (Line 242):\n")
	b.WriteString(t.ScientificTechnologicalUncertainties + "\n\n")
	b.WriteString("Work performed to overcome uncertainties (Line 244):\n")
	b.WriteString(t.WorkPerformedToOvercome + "\n\n")
	b.WriteString("Advancements achieved (Line 246):\n")
	b.WriteString(t.AdvancementsAchieved + "\n")
	return b.String()
}

// IRAPMarkdown returns the IRAP report as markdown.
func (r *IRAPReport) IRAPMarkdown() string {
	var b strings.Builder
	b.WriteString("# " + r.Title + "\n\n")
	b.WriteString("**Date:** " + r.Date.Format("2006-01-02") + "\n\n")
	b.WriteString("## Summary\n\n" + r.Summary + "\n\n")
	for _, s := range r.Sections {
		b.WriteString("## " + s.Title + "\n\n" + s.Content + "\n\n")
	}
	b.WriteString("## Evidence references\n\n")
	for _, e := range r.EvidenceRefs {
		b.WriteString("- " + e.ID + " (" + string(e.Type) + ", " + e.SessionID + ", " + e.CollectedAt.Format("2006-01-02") + ")\n")
	}
	return b.String()
}
