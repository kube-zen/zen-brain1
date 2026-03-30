-- Rollback token_records enhancements
DROP INDEX IF EXISTS idx_token_records_project_recorded;
DROP INDEX IF EXISTS idx_token_records_model_project;
DROP INDEX IF EXISTS idx_token_records_outcome;
DROP INDEX IF EXISTS idx_token_records_sred_eligible;
DROP INDEX IF EXISTS idx_token_records_evidence_class;
ALTER TABLE token_records DROP COLUMN IF EXISTS evidence_class;
