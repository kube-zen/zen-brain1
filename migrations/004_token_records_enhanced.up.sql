-- Add evidence_class column to token_records (V6 Section 3.14)
-- EvidenceClass indicates the type of outcome produced:
-- pr_merged, test_passed, doc_updated, plan_approved, code_reviewed, etc.

ALTER TABLE token_records ADD COLUMN IF NOT EXISTS evidence_class STRING;

-- Index for querying by evidence class (task type cost profile)
CREATE INDEX IF NOT EXISTS idx_token_records_evidence_class
    ON token_records (evidence_class) WHERE evidence_class IS NOT NULL;

-- Index for SR&ED eligibility queries
CREATE INDEX IF NOT EXISTS idx_token_records_sred_eligible
    ON token_records (sred_eligible) WHERE sred_eligible = true;

-- Index for outcome analysis
CREATE INDEX IF NOT EXISTS idx_token_records_outcome
    ON token_records (outcome);

-- Composite index for model efficiency report queries
CREATE INDEX IF NOT EXISTS idx_token_records_model_project
    ON token_records (model_id, project_id);

-- Composite index for cost breakdown by project/time
CREATE INDEX IF NOT EXISTS idx_token_records_project_recorded
    ON token_records (project_id, recorded_at DESC);

COMMENT ON COLUMN token_records.evidence_class IS
    'Type of outcome: pr_merged, test_passed, doc_updated, plan_approved, code_reviewed, etc.';
