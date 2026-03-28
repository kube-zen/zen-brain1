package metrics

import "time"

// ClassifyCompletion determines the completion class for a task based on
// timing and outcome signals. This is the canonical classification logic.
func ClassifyCompletion(
	wallTimeMs int64,
	outputSizeChars int,
	parseError bool,
	qualityScore float64,
	qualityThreshold float64,
) CompletionClass {
	if parseError {
		return ClassParseFail
	}
	if outputSizeChars == 0 {
		return ClassTimeout
	}
	if qualityScore > 0 && qualityScore < qualityThreshold {
		return ClassValidationFail
	}
	// productive: check speed
	if wallTimeMs < 30000 {
		return ClassFastProductive
	}
	return ClassSlowButProductive
}

// ClassifyProducedBy determines who produced the final artifact.
func ClassifyProducedBy(
	l1OutputSize int,
	parseSucceeded bool,
	qualityScore float64,
	qualityThreshold float64,
	supervisorIntervention string,
) ProducedBy {
	if supervisorIntervention == "manual_rewrite" || supervisorIntervention == "script_override" {
		return ProducedBySupervisor
	}
	if !parseSucceeded || l1OutputSize == 0 {
		return ProducedByL1Failed
	}
	if qualityScore >= qualityThreshold {
		return ProducedByL1
	}
	return ProducedByL1Partial
}

// NewTaskRecord is a convenience builder for creating telemetry records
// from the remediation-worker's execution path.
type TaskRecordBuilder struct {
	rec TaskTelemetryRecord
}

// NewTaskRecord starts building a telemetry record.
func NewTaskRecord(runID, taskID string) *TaskRecordBuilder {
	return &TaskRecordBuilder{
		rec: TaskTelemetryRecord{
			Timestamp: time.Now(),
			RunID:     runID,
			TaskID:    taskID,
		},
	}
}

func (b *TaskRecordBuilder) JiraKey(key string) *TaskRecordBuilder          { b.rec.JiraKey = key; return b }
func (b *TaskRecordBuilder) Schedule(name string) *TaskRecordBuilder         { b.rec.ScheduleName = name; return b }
func (b *TaskRecordBuilder) Model(model string) *TaskRecordBuilder           { b.rec.Model = model; return b }
func (b *TaskRecordBuilder) Lane(lane string) *TaskRecordBuilder             { b.rec.Lane = lane; return b }
func (b *TaskRecordBuilder) Provider(provider string) *TaskRecordBuilder     { b.rec.Provider = provider; return b }
func (b *TaskRecordBuilder) PromptSize(chars int) *TaskRecordBuilder         { b.rec.PromptSizeChars = chars; return b }
func (b *TaskRecordBuilder) OutputSize(chars int) *TaskRecordBuilder         { b.rec.OutputSizeChars = chars; return b }
func (b *TaskRecordBuilder) Tokens(in, out int) *TaskRecordBuilder           { b.rec.InputTokens = in; b.rec.OutputTokens = out; return b }
func (b *TaskRecordBuilder) Timing(start time.Time, wallMs int64) *TaskRecordBuilder {
	b.rec.StartTime = start
	b.rec.EndTime = start.Add(time.Duration(wallMs) * time.Millisecond)
	b.rec.WallTimeMs = wallMs
	return b
}
func (b *TaskRecordBuilder) FirstTokenMs(ms int64) *TaskRecordBuilder        { b.rec.FirstTokenMs = ms; return b }
func (b *TaskRecordBuilder) LoadDurationMs(ms int64) *TaskRecordBuilder      { b.rec.LoadDurationMs = ms; return b }
func (b *TaskRecordBuilder) CompletionClass(c CompletionClass) *TaskRecordBuilder { b.rec.CompletionClass = c; return b }
func (b *TaskRecordBuilder) ProducedBy(p ProducedBy) *TaskRecordBuilder      { b.rec.ProducedBy = p; return b }
func (b *TaskRecordBuilder) Attempt(n int) *TaskRecordBuilder                { b.rec.AttemptNumber = n; return b }
func (b *TaskRecordBuilder) QualityScore(score float64) *TaskRecordBuilder   { b.rec.QualityScore = score; return b }
func (b *TaskRecordBuilder) Repair(used, succeeded bool) *TaskRecordBuilder  { b.rec.RepairUsed = used; b.rec.RepairSucceeded = succeeded; return b }
func (b *TaskRecordBuilder) TaskClass(class string) *TaskRecordBuilder       { b.rec.TaskClass = class; return b }
func (b *TaskRecordBuilder) RemediationType(rt string) *TaskRecordBuilder    { b.rec.RemediationType = rt; return b }
func (b *TaskRecordBuilder) FinalStatus(s string) *TaskRecordBuilder         { b.rec.FinalStatus = s; return b }
func (b *TaskRecordBuilder) JiraTransition(t string) *TaskRecordBuilder      { b.rec.JiraTransition = t; return b }
func (b *TaskRecordBuilder) JiraUpdated(ok bool) *TaskRecordBuilder          { b.rec.JiraUpdated = ok; return b }
func (b *TaskRecordBuilder) EvidencePath(p string) *TaskRecordBuilder        { b.rec.EvidencePackPath = p; return b }

// Build returns the completed telemetry record.
func (b *TaskRecordBuilder) Build() TaskTelemetryRecord {
	return b.rec
}
