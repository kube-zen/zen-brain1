-- ZenLedger planned_model_selections table (Block 3.6)
-- Stores Planner's model selection for analysis and budget tracking.
CREATE TABLE IF NOT EXISTS planned_model_selections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id STRING NOT NULL,
    task_id STRING NOT NULL,
    model_id STRING NOT NULL,
    reason STRING NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_planned_model_selections_session ON planned_model_selections (session_id);
CREATE INDEX IF NOT EXISTS idx_planned_model_selections_recorded ON planned_model_selections (recorded_at);
