-- ZenLedger Reporting Views (V6 Section 3.14)
-- Pre-built views for common token/cost analysis queries.
-- These views support dashboards, SR&ED reporting, and efficiency analysis.

-- ═══════════════════════════════════════════════════════════════════════════════
-- MODEL EFFICIENCY REPORT
-- Analyzes model performance by success rate, cost, and human corrections.
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE OR REPLACE VIEW model_efficiency_report AS
SELECT
    model_id,
    COUNT(*) AS total_tasks,
    AVG(tokens_input + tokens_output) AS avg_tokens_per_task,
    AVG(cost_usd) AS avg_cost_per_task,
    AVG(latency_ms) AS avg_latency_ms,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) AS success_rate,
    AVG(human_corrections) AS avg_corrections,
    SUM(CASE WHEN sred_eligible THEN 1 ELSE 0 END) AS sred_tasks
FROM token_records
GROUP BY model_id
ORDER BY avg_cost_per_task ASC;

COMMENT ON VIEW model_efficiency_report IS
    'Model efficiency: success rate, cost, tokens, latency by model_id';

-- ═══════════════════════════════════════════════════════════════════════════════
-- TASK TYPE COST PROFILE
-- Analyzes cost and quality by evidence_class (task type).
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE OR REPLACE VIEW task_type_cost_profile AS
SELECT
    evidence_class,
    COUNT(*) AS total_tasks,
    AVG(cost_usd) AS avg_cost,
    AVG(latency_ms) AS avg_latency,
    AVG(tokens_input + tokens_output) AS avg_tokens,
    AVG(human_corrections) AS avg_corrections,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) AS success_rate
FROM token_records
WHERE evidence_class IS NOT NULL
GROUP BY evidence_class
ORDER BY total_tasks DESC;

COMMENT ON VIEW task_type_cost_profile IS
    'Task type analysis: cost, latency, success rate by evidence_class';

-- ═══════════════════════════════════════════════════════════════════════════════
-- LOCAL VS API COMPARISON
-- Compares local inference vs API providers.
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE OR REPLACE VIEW local_vs_api_comparison AS
SELECT
    source,
    model_id,
    COUNT(*) AS total_tasks,
    SUM(cost_usd) AS total_cost,
    AVG(latency_ms) AS avg_latency_ms,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) AS success_rate,
    AVG(human_corrections) AS avg_corrections
FROM token_records
GROUP BY source, model_id
ORDER BY source, total_tasks DESC;

COMMENT ON VIEW local_vs_api_comparison IS
    'Compare local inference (source=local) vs API providers';

-- ═══════════════════════════════════════════════════════════════════════════════
-- PROJECT COST BREAKDOWN
-- Cost and task analysis by project.
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE OR REPLACE VIEW project_cost_breakdown AS
SELECT
    project_id,
    COUNT(*) AS total_tasks,
    SUM(cost_usd) AS total_cost,
    AVG(cost_usd) AS avg_cost_per_task,
    SUM(tokens_input + tokens_output) AS total_tokens,
    SUM(CASE WHEN sred_eligible THEN cost_usd ELSE 0 END) AS sred_cost,
    MIN(recorded_at) AS first_task,
    MAX(recorded_at) AS last_task
FROM token_records
WHERE project_id IS NOT NULL
GROUP BY project_id
ORDER BY total_cost DESC;

COMMENT ON VIEW project_cost_breakdown IS
    'Project cost breakdown: total/avg cost, tokens, SR&ED-eligible costs';

-- ═══════════════════════════════════════════════════════════════════════════════
-- SR&ED COST EXPORT
-- Export-ready view for SR&ED tax credit claims.
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE OR REPLACE VIEW sred_cost_export AS
SELECT
    recorded_at::DATE AS date,
    project_id,
    model_id,
    agent_role,
    task_id,
    session_id,
    tokens_input,
    tokens_output,
    tokens_cached,
    cost_usd,
    latency_ms,
    evidence_class,
    human_corrections,
    cluster_id
FROM token_records
WHERE sred_eligible = true
ORDER BY recorded_at DESC;

COMMENT ON VIEW sred_cost_export IS
    'SR&ED-eligible token records for tax credit claims';

-- ═══════════════════════════════════════════════════════════════════════════════
-- DAILY COST SUMMARY
-- Aggregated daily costs for dashboards.
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE OR REPLACE VIEW daily_cost_summary AS
SELECT
    recorded_at::DATE AS date,
    source,
    model_id,
    COUNT(*) AS total_tasks,
    SUM(cost_usd) AS total_cost,
    SUM(tokens_input) AS total_input_tokens,
    SUM(tokens_output) AS total_output_tokens,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END) AS completed_tasks,
    SUM(CASE WHEN outcome = 'failed' THEN 1 ELSE 0 END) AS failed_tasks
FROM token_records
GROUP BY recorded_at::DATE, source, model_id
ORDER BY date DESC, total_cost DESC;

COMMENT ON VIEW daily_cost_summary IS
    'Daily aggregated costs and task counts by source/model';
