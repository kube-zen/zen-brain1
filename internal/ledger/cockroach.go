// Package ledger provides CockroachDB-backed ZenLedger implementation (Block 3.6).
package ledger

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

// CockroachLedger implements ZenLedgerClient and TokenRecorder using CockroachDB.
// Use NewCockroachLedger with a postgres-compatible DSN (e.g. postgres://root@localhost:26257/defaultdb?sslmode=disable).
type CockroachLedger struct {
	db *sql.DB
}

// NewCockroachLedger opens a connection to CockroachDB and returns a ledger that implements
// both ZenLedgerClient and TokenRecorder. dsn must be a postgres-compatible DSN.
// If dsn is empty, returns nil, nil (caller can fall back to stub).
func NewCockroachLedger(dsn string) (*CockroachLedger, error) {
	if dsn == "" {
		return nil, nil
	}
	// CockroachDB accepts postgres://; migrate uses cockroachdb:// - normalize for lib/pq
	dsn = strings.Replace(dsn, "cockroachdb://", "postgres://", 1)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &CockroachLedger{db: db}, nil
}

// Close closes the database connection.
func (c *CockroachLedger) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// Record implements TokenRecorder.Record.
func (c *CockroachLedger) Record(ctx context.Context, record ledger.TokenRecord) error {
	evidenceClass := ""
	if record.EvidenceClass != "" {
		evidenceClass = string(record.EvidenceClass)
	}
	ts := record.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := c.db.ExecContext(ctx, `INSERT INTO token_records (
		session_id, task_id, agent_role, model_id, inference_type, source,
		tokens_input, tokens_output, tokens_cached, cost_usd, latency_ms,
		outcome, evidence_class, human_corrections, sred_eligible, recorded_at, cluster_id, project_id
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		record.SessionID, record.TaskID, record.AgentRole, record.ModelID, string(record.InferenceType), string(record.Source),
		record.TokensInput, record.TokensOutput, record.TokensCached, record.CostUSD, record.LatencyMs,
		string(record.Outcome), evidenceClass, record.HumanCorrections, record.SREDEligible, ts, nullStr(record.ClusterID), nullStr(record.ProjectID),
	)
	return err
}

// RecordBatch implements TokenRecorder.RecordBatch.
func (c *CockroachLedger) RecordBatch(ctx context.Context, records []ledger.TokenRecord) error {
	for _, r := range records {
		if err := c.Record(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// GetModelEfficiency implements ZenLedgerClient.GetModelEfficiency.
func (c *CockroachLedger) GetModelEfficiency(ctx context.Context, projectID string, taskType string) ([]ledger.ModelEfficiency, error) {
	query := `
		SELECT model_id,
			COALESCE(AVG(cost_usd),0), COALESCE(SUM(tokens_input+tokens_output),0)::INT8,
			COALESCE(SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END)::FLOAT / NULLIF(COUNT(*),0), 0),
			COALESCE(AVG(human_corrections),0), COALESCE(AVG(latency_ms),0)::INT8, COUNT(*)
		FROM token_records
		WHERE (CAST($1 AS STRING) = '' OR project_id = $1)
		GROUP BY model_id
	`
	rows, err := c.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ledger.ModelEfficiency
	for rows.Next() {
		var m ledger.ModelEfficiency
		var cnt int
		if err := rows.Scan(&m.ModelID, &m.AvgCostPerTask, &m.AvgTokensPerTask, &m.SuccessRate, &m.AvgCorrections, &m.AvgLatencyMs, &cnt); err != nil {
			return nil, err
		}
		m.SampleSize = cnt
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetCostBudgetStatus implements ZenLedgerClient.GetCostBudgetStatus.
func (c *CockroachLedger) GetCostBudgetStatus(ctx context.Context, projectID string) (*ledger.BudgetStatus, error) {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0)
	var spent float64
	err := c.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0) FROM token_records
		WHERE recorded_at >= $1 AND recorded_at < $2 AND (CAST($3 AS STRING) = '' OR project_id = $3)
	`, periodStart, periodEnd, projectID).Scan(&spent)
	if err != nil {
		return nil, err
	}
	budgetLimit := 1000.0
	remaining := budgetLimit - spent
	if remaining < 0 {
		remaining = 0
	}
	percentUsed := 0.0
	if budgetLimit > 0 {
		percentUsed = spent / budgetLimit * 100
	}
	return &ledger.BudgetStatus{
		ProjectID:      projectID,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		SpentUSD:       spent,
		BudgetLimitUSD: budgetLimit,
		RemainingUSD:   remaining,
		PercentUsed:    percentUsed,
	}, nil
}

// RecordPlannedModelSelection implements ZenLedgerClient.RecordPlannedModelSelection.
func (c *CockroachLedger) RecordPlannedModelSelection(ctx context.Context, sessionID, taskID, modelID, reason string) error {
	_, err := c.db.ExecContext(ctx, `INSERT INTO planned_model_selections (session_id, task_id, model_id, reason) VALUES ($1,$2,$3,$4)`,
		sessionID, taskID, modelID, reason)
	if err != nil {
		log.Printf("[CockroachLedger] RecordPlannedModelSelection failed: %v", err)
		return err
	}
	return nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// Ensure CockroachLedger implements both interfaces.
var (
	_ ledger.ZenLedgerClient = (*CockroachLedger)(nil)
	_ ledger.TokenRecorder   = (*CockroachLedger)(nil)
)
