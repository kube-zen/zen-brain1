// Package session provides work session management for zen-brain.
package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-sdk/pkg/store"
)

// SQLiteStore is a SQLite implementation of Store.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	ctx := context.Background()
	db, err := store.OpenSQLiteSimple(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}
	return s, nil
}

// migrate creates the sessions table if it does not exist.
func (s *SQLiteStore) migrate() error {
	query := `
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	work_item_id TEXT NOT NULL,
	source_key TEXT NOT NULL,
	state TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	started_at TIMESTAMP,
	completed_at TIMESTAMP,
	assigned_agent TEXT,
	assigned_model TEXT,
	error TEXT,
	work_item_json TEXT,
	analysis_result_json TEXT,
	brain_task_specs_json TEXT,
	state_history_json TEXT,
	evidence_items_json TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_work_item_id ON sessions(work_item_id);
CREATE INDEX IF NOT EXISTS idx_sessions_state ON sessions(state);
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_updated_at ON sessions(updated_at);
CREATE INDEX IF NOT EXISTS idx_sessions_source_key ON sessions(source_key);
CREATE INDEX IF NOT EXISTS idx_sessions_assigned_agent ON sessions(assigned_agent);
`
	_, err := s.db.Exec(query)
	return err
}

// Create creates a new session.
func (s *SQLiteStore) Create(ctx context.Context, session *contracts.Session) error {
	workItemJSON, err := marshalOptional(session.WorkItem)
	if err != nil {
		return err
	}
	analysisResultJSON, err := marshalOptional(session.AnalysisResult)
	if err != nil {
		return err
	}
	brainTaskSpecsJSON, err := json.Marshal(session.BrainTaskSpecs)
	if err != nil {
		return err
	}
	stateHistoryJSON, err := json.Marshal(session.StateHistory)
	if err != nil {
		return err
	}
	evidenceItemsJSON, err := json.Marshal(session.EvidenceItems)
	if err != nil {
		return err
	}

	query := `
INSERT INTO sessions (
	id, work_item_id, source_key, state,
	created_at, updated_at, started_at, completed_at,
	assigned_agent, assigned_model, error,
	work_item_json, analysis_result_json,
	brain_task_specs_json, state_history_json, evidence_items_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.ExecContext(ctx, query,
		session.ID,
		session.WorkItemID,
		session.SourceKey,
		string(session.State),
		session.CreatedAt,
		session.UpdatedAt,
		nullableTime(session.StartedAt),
		nullableTime(session.CompletedAt),
		nullableString(session.AssignedAgent),
		nullableString(session.AssignedModel),
		nullableString(session.Error),
		workItemJSON,
		analysisResultJSON,
		brainTaskSpecsJSON,
		stateHistoryJSON,
		evidenceItemsJSON,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrSessionExists
		}
		return fmt.Errorf("failed to insert session: %w", err)
	}
	return nil
}

// Get retrieves a session by ID.
func (s *SQLiteStore) Get(ctx context.Context, sessionID string) (*contracts.Session, error) {
	query := `
SELECT
	id, work_item_id, source_key, state,
	created_at, updated_at, started_at, completed_at,
	assigned_agent, assigned_model, error,
	work_item_json, analysis_result_json,
	brain_task_specs_json, state_history_json, evidence_items_json
FROM sessions WHERE id = ?`

	row := s.db.QueryRowContext(ctx, query, sessionID)
	return s.scanSession(row)
}

// GetByWorkItem retrieves the active session for a work item.
func (s *SQLiteStore) GetByWorkItem(ctx context.Context, workItemID string) (*contracts.Session, error) {
	// We need to find the most recent active session for this work item.
	// Active means not completed/failed/canceled.
	query := `
SELECT
	id, work_item_id, source_key, state,
	created_at, updated_at, started_at, completed_at,
	assigned_agent, assigned_model, error,
	work_item_json, analysis_result_json,
	brain_task_specs_json, state_history_json, evidence_items_json
FROM sessions
WHERE work_item_id = ?
AND state NOT IN (?, ?, ?)
ORDER BY created_at DESC
LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, workItemID,
		string(contracts.SessionStateCompleted),
		string(contracts.SessionStateFailed),
		string(contracts.SessionStateCanceled))
	return s.scanSession(row)
}

// Update updates an existing session.
func (s *SQLiteStore) Update(ctx context.Context, session *contracts.Session) error {
	workItemJSON, err := marshalOptional(session.WorkItem)
	if err != nil {
		return err
	}
	analysisResultJSON, err := marshalOptional(session.AnalysisResult)
	if err != nil {
		return err
	}
	brainTaskSpecsJSON, err := json.Marshal(session.BrainTaskSpecs)
	if err != nil {
		return err
	}
	stateHistoryJSON, err := json.Marshal(session.StateHistory)
	if err != nil {
		return err
	}
	evidenceItemsJSON, err := json.Marshal(session.EvidenceItems)
	if err != nil {
		return err
	}

	query := `
UPDATE sessions SET
	work_item_id = ?, source_key = ?, state = ?,
	created_at = ?, updated_at = ?, started_at = ?, completed_at = ?,
	assigned_agent = ?, assigned_model = ?, error = ?,
	work_item_json = ?, analysis_result_json = ?,
	brain_task_specs_json = ?, state_history_json = ?, evidence_items_json = ?
WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query,
		session.WorkItemID,
		session.SourceKey,
		string(session.State),
		session.CreatedAt,
		session.UpdatedAt,
		nullableTime(session.StartedAt),
		nullableTime(session.CompletedAt),
		nullableString(session.AssignedAgent),
		nullableString(session.AssignedModel),
		nullableString(session.Error),
		workItemJSON,
		analysisResultJSON,
		brainTaskSpecsJSON,
		stateHistoryJSON,
		evidenceItemsJSON,
		session.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// List returns sessions matching the filter.
func (s *SQLiteStore) List(ctx context.Context, filter SessionFilter) ([]*contracts.Session, error) {
	var whereClauses []string
	var args []interface{}

	if filter.State != nil {
		whereClauses = append(whereClauses, "state = ?")
		args = append(args, string(*filter.State))
	}
	if filter.WorkItemID != nil {
		whereClauses = append(whereClauses, "work_item_id = ?")
		args = append(args, *filter.WorkItemID)
	}
	if filter.SourceKey != nil {
		whereClauses = append(whereClauses, "source_key = ?")
		args = append(args, *filter.SourceKey)
	}
	if filter.AssignedAgent != nil {
		whereClauses = append(whereClauses, "assigned_agent = ?")
		args = append(args, *filter.AssignedAgent)
	}
	if filter.CreatedAfter != nil {
		whereClauses = append(whereClauses, "created_at >= ?")
		args = append(args, *filter.CreatedAfter)
	}
	if filter.CreatedBefore != nil {
		whereClauses = append(whereClauses, "created_at <= ?")
		args = append(args, *filter.CreatedBefore)
	}
	if filter.UpdatedAfter != nil {
		whereClauses = append(whereClauses, "updated_at >= ?")
		args = append(args, *filter.UpdatedAfter)
	}
	if filter.UpdatedBefore != nil {
		whereClauses = append(whereClauses, "updated_at <= ?")
		args = append(args, *filter.UpdatedBefore)
	}

	query := `
SELECT
	id, work_item_id, source_key, state,
	created_at, updated_at, started_at, completed_at,
	assigned_agent, assigned_model, error,
	work_item_json, analysis_result_json,
	brain_task_specs_json, state_history_json, evidence_items_json
FROM sessions`
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*contracts.Session
	for rows.Next() {
		session, err := s.scanSessionFromRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return sessions, nil
}

// Delete deletes a session.
func (s *SQLiteStore) Delete(ctx context.Context, sessionID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// Close closes the store.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// scanSession scans a single row from a *sql.Row.
func (s *SQLiteStore) scanSession(row *sql.Row) (*contracts.Session, error) {
	var session contracts.Session
	var state string
	var workItemJSON, analysisResultJSON sql.NullString
	var brainTaskSpecsJSON, stateHistoryJSON, evidenceItemsJSON string
	var startedAt, completedAt sql.NullTime
	var assignedAgent, assignedModel, errMsg sql.NullString

	err := row.Scan(
		&session.ID,
		&session.WorkItemID,
		&session.SourceKey,
		&state,
		&session.CreatedAt,
		&session.UpdatedAt,
		&startedAt,
		&completedAt,
		&assignedAgent,
		&assignedModel,
		&errMsg,
		&workItemJSON,
		&analysisResultJSON,
		&brainTaskSpecsJSON,
		&stateHistoryJSON,
		&evidenceItemsJSON,
	)
	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan session: %w", err)
	}

	session.State = contracts.SessionState(state)
	if startedAt.Valid {
		session.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}
	if assignedAgent.Valid {
		session.AssignedAgent = assignedAgent.String
	}
	if assignedModel.Valid {
		session.AssignedModel = assignedModel.String
	}
	if errMsg.Valid {
		session.Error = errMsg.String
	}

	// Deserialize JSON fields
	if workItemJSON.Valid && workItemJSON.String != "" {
		var workItem contracts.WorkItem
		if err := json.Unmarshal([]byte(workItemJSON.String), &workItem); err != nil {
			return nil, fmt.Errorf("failed to unmarshal work item: %w", err)
		}
		session.WorkItem = &workItem
	}
	if analysisResultJSON.Valid && analysisResultJSON.String != "" {
		var analysisResult contracts.AnalysisResult
		if err := json.Unmarshal([]byte(analysisResultJSON.String), &analysisResult); err != nil {
			return nil, fmt.Errorf("failed to unmarshal analysis result: %w", err)
		}
		session.AnalysisResult = &analysisResult
	}
	if err := json.Unmarshal([]byte(brainTaskSpecsJSON), &session.BrainTaskSpecs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal brain task specs: %w", err)
	}
	if err := json.Unmarshal([]byte(stateHistoryJSON), &session.StateHistory); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state history: %w", err)
	}
	if err := json.Unmarshal([]byte(evidenceItemsJSON), &session.EvidenceItems); err != nil {
		return nil, fmt.Errorf("failed to unmarshal evidence items: %w", err)
	}

	return &session, nil
}

// scanSessionFromRows scans a single row from *sql.Rows (same schema as scanSession).
func (s *SQLiteStore) scanSessionFromRows(rows *sql.Rows) (*contracts.Session, error) {
	var session contracts.Session
	var state string
	var workItemJSON, analysisResultJSON sql.NullString
	var brainTaskSpecsJSON, stateHistoryJSON, evidenceItemsJSON string
	var startedAt, completedAt sql.NullTime
	var assignedAgent, assignedModel, errMsg sql.NullString

	err := rows.Scan(
		&session.ID,
		&session.WorkItemID,
		&session.SourceKey,
		&state,
		&session.CreatedAt,
		&session.UpdatedAt,
		&startedAt,
		&completedAt,
		&assignedAgent,
		&assignedModel,
		&errMsg,
		&workItemJSON,
		&analysisResultJSON,
		&brainTaskSpecsJSON,
		&stateHistoryJSON,
		&evidenceItemsJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan session row: %w", err)
	}

	session.State = contracts.SessionState(state)
	if startedAt.Valid {
		session.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}
	if assignedAgent.Valid {
		session.AssignedAgent = assignedAgent.String
	}
	if assignedModel.Valid {
		session.AssignedModel = assignedModel.String
	}
	if errMsg.Valid {
		session.Error = errMsg.String
	}

	// Deserialize JSON fields
	if workItemJSON.Valid && workItemJSON.String != "" {
		var workItem contracts.WorkItem
		if err := json.Unmarshal([]byte(workItemJSON.String), &workItem); err != nil {
			return nil, fmt.Errorf("failed to unmarshal work item: %w", err)
		}
		session.WorkItem = &workItem
	}
	if analysisResultJSON.Valid && analysisResultJSON.String != "" {
		var analysisResult contracts.AnalysisResult
		if err := json.Unmarshal([]byte(analysisResultJSON.String), &analysisResult); err != nil {
			return nil, fmt.Errorf("failed to unmarshal analysis result: %w", err)
		}
		session.AnalysisResult = &analysisResult
	}
	if err := json.Unmarshal([]byte(brainTaskSpecsJSON), &session.BrainTaskSpecs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal brain task specs: %w", err)
	}
	if err := json.Unmarshal([]byte(stateHistoryJSON), &session.StateHistory); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state history: %w", err)
	}
	if err := json.Unmarshal([]byte(evidenceItemsJSON), &session.EvidenceItems); err != nil {
		return nil, fmt.Errorf("failed to unmarshal evidence items: %w", err)
	}

	return &session, nil
}

// Helper functions

func marshalOptional(v interface{}) (sql.NullString, error) {
	if v == nil {
		return sql.NullString{Valid: false}, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(data), Valid: true}, nil
}

func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}