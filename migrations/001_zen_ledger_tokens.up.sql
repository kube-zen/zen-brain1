-- ZenLedger token_records table (Block 3.6)
-- Tracks LLM token usage and cost per task/session for model efficiency and SR&ED.
CREATE TABLE IF NOT EXISTS token_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id STRING NOT NULL,
    task_id STRING NOT NULL,
    agent_role STRING NOT NULL,
    model_id STRING NOT NULL,
    inference_type STRING NOT NULL,
    source STRING NOT NULL,
    tokens_input INT8 NOT NULL DEFAULT 0,
    tokens_output INT8 NOT NULL DEFAULT 0,
    tokens_cached INT8 NOT NULL DEFAULT 0,
    cost_usd FLOAT NOT NULL DEFAULT 0,
    latency_ms INT8 NOT NULL DEFAULT 0,
    outcome STRING NOT NULL,
    evidence_class STRING,
    human_corrections INT NOT NULL DEFAULT 0,
    sred_eligible BOOL NOT NULL DEFAULT false,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    cluster_id STRING,
    project_id STRING
);

CREATE INDEX IF NOT EXISTS idx_token_records_session ON token_records (session_id);
CREATE INDEX IF NOT EXISTS idx_token_records_task ON token_records (task_id);
CREATE INDEX IF NOT EXISTS idx_token_records_model ON token_records (model_id);
CREATE INDEX IF NOT EXISTS idx_token_records_recorded ON token_records (recorded_at);
CREATE INDEX IF NOT EXISTS idx_token_records_project ON token_records (project_id);
