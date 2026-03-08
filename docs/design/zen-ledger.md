# ZenLedger Design

## Overview

ZenLedger is the token and cost accounting system for Zen‑Brain. It tracks **the yield (value produced) per token spent**, not just raw cost.

**Key insight:** A task that costs $0.40 in API tokens but produces a merged PR is cheaper than a task that costs $0.02 but produces a comment that needs three human corrections.

ZenLedger records every LLM call with details on tokens, cost, latency, outcome, and evidence class. This data enables:

- **Model efficiency ranking** – which models deliver the best results per dollar.
- **Project cost breakdown** – track spending per project, cluster, task type.
- **SR&ED‑eligible cost export** – filter costs for funding claims.
- **Budget enforcement** – prevent overspending.
- **Planner optimization** – cost‑aware model selection.

## Interface

ZenLedger provides two interfaces for different consumers:

### ZenLedgerClient (used by Planner Agent)

Defined in `pkg/ledger/interface.go`:

```go
type ZenLedgerClient interface {
    // GetModelEfficiency returns historical efficiency data for models on a task type.
    GetModelEfficiency(ctx context.Context, projectID, taskType string) ([]ModelEfficiency, error)

    // GetCostBudgetStatus returns current spending against budget limits.
    GetCostBudgetStatus(ctx context.Context, projectID string) (*BudgetStatus, error)

    // RecordPlannedModelSelection logs the Planner's model choice for later analysis.
    RecordPlannedModelSelection(ctx context.Context, sessionID, taskID, modelID, reason string) error
}
```

This interface is **defined early (Block 1.7.1)** so the Planner (Block 2.5) can depend on it without circular dependencies.

### TokenRecorder (used by Worker Agents)

```go
type TokenRecorder interface {
    // Record records a token usage event.
    Record(ctx context.Context, record TokenRecord) error

    // RecordBatch records multiple token usage events.
    RecordBatch(ctx context.Context, records []TokenRecord) error
}
```

Worker agents call `Record` after each LLM call (chat, embedding, rerank). The recorder can batch writes for efficiency.

## Data Structures

### TokenRecord

Core record of an LLM usage:

**Identity & Context:**
- `SessionID`, `TaskID`, `AgentRole`
- `ModelID` – e.g., `"glm‑4.7"`, `"claude‑sonnet‑4‑6"`, `"nomic‑embed‑text"`
- `InferenceType` – `chat`, `embedding`, `rerank`
- `Source` – `local` or `api`

**Cost Side:**
- `TokensInput`, `TokensOutput`, `TokensCached`
- `CostUSD` – real cost for API, estimated for local (see Local Cost Model)
- `LatencyMs`

**Yield Side:**
- `Outcome` – `completed`, `failed`, `human_corrected`, `abandoned`
- `EvidenceClass` – `pr_merged`, `test_passed`, `doc_updated`, `plan_approved`, `summary`
- `HumanCorrections` – count of times human had to fix the output
- `SREDEligible` – whether this usage contributes to SR&ED‑eligible costs

**Metadata:**
- `Timestamp`, `ClusterID`, `ProjectID`

### ModelEfficiency

Aggregated view for Planner decision‑making:

- `ModelID`, `AvgCostPerTask`, `AvgTokensPerTask`
- `SuccessRate`, `AvgCorrections`, `AvgLatencyMs`
- `SampleSize`

### BudgetStatus

Current spending vs. budget:

- `ProjectID`, `PeriodStart`, `PeriodEnd`
- `SpentUSD`, `BudgetLimitUSD`, `RemainingUSD`, `PercentUsed`

## Local Cost Model

Local inference is not free—it has a cost in time and hardware:

```yaml
local_cost_model:
  cpu_inference_rate: 0.001    # $/min of CPU time
  gpu_inference_rate: 0.02     # $/min of GPU time (if available)
  memory_overhead_rate: 0.0001 # $/GB/min
```

**Example calculation:**
- 50ms latency on CPU
- 4GB memory allocated
- Equivalent cost: `(50/1000/60 * 0.001) + (4 * 0.0001 * 50/1000/60) ≈ $0.000001`

These costs are **estimates** for comparison with API costs, not actual monetary outlays.

## Storage Schema (CockroachDB)

ZenLedger stores `TokenRecord`s in CockroachDB with indexes optimized for the required query patterns.

**Table definition:**

```sql
CREATE TABLE token_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Identity & Context
    session_id STRING NOT NULL,
    task_id STRING NOT NULL,
    agent_role STRING NOT NULL,
    model_id STRING NOT NULL,
    inference_type STRING NOT NULL,  -- 'chat', 'embedding', 'rerank'
    source STRING NOT NULL,          -- 'local', 'api'
    
    -- Cost side
    tokens_input INT8 NOT NULL,
    tokens_output INT8 NOT NULL,
    tokens_cached INT8 DEFAULT 0,
    cost_usd DECIMAL(10,6) NOT NULL,
    latency_ms INT8 NOT NULL,
    
    -- Yield side
    outcome STRING NOT NULL,         -- 'completed', 'failed', 'human_corrected', 'abandoned'
    evidence_class STRING NOT NULL,  -- 'pr_merged', 'test_passed', etc.
    human_corrections INT DEFAULT 0,
    sred_eligible BOOL DEFAULT false,
    
    -- Metadata
    timestamp TIMESTAMPTZ NOT NULL,
    cluster_id STRING NOT NULL,
    project_id STRING NOT NULL,
    
    -- Indexes for common queries
    INDEX (project_id, timestamp),
    INDEX (model_id, outcome),
    INDEX (project_id, source),
    INDEX (project_id, sred_eligible, timestamp),
    INDEX (session_id, task_id)
);
```

**Materialized views** for frequent aggregations:

```sql
-- Model efficiency per project/task type (refreshed every 5 minutes)
CREATE MATERIALIZED VIEW model_efficiency AS
SELECT 
    project_id,
    model_id,
    outcome,
    COUNT(*) as total_tasks,
    AVG(cost_usd) as avg_cost_per_task,
    AVG(tokens_input + tokens_output) as avg_tokens_per_task,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) as success_rate,
    AVG(human_corrections) as avg_corrections,
    AVG(latency_ms) as avg_latency_ms
FROM token_records
WHERE timestamp > NOW() - INTERVAL '30 days'
GROUP BY project_id, model_id, outcome;
```

## Query Patterns

### 1. Model Efficiency Report (Planner)

```sql
SELECT 
    model_id,
    COUNT(*) as total_tasks,
    AVG(tokens_input + tokens_output) as avg_tokens_per_task,
    AVG(cost_usd) as avg_cost_per_task,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) as success_rate,
    AVG(human_corrections) as avg_corrections
FROM token_records
WHERE project_id = $1
GROUP BY model_id
ORDER BY avg_cost_per_task ASC;
```

### 2. Task Type Cost Profile

```sql
SELECT 
    evidence_class,
    COUNT(*) as total_tasks,
    AVG(cost_usd) as avg_cost,
    AVG(latency_ms) as avg_latency,
    AVG(human_corrections) as avg_corrections
FROM token_records
WHERE project_id = $1
GROUP BY evidence_class
ORDER BY avg_cost ASC;
```

### 3. Local vs API Comparison

```sql
SELECT 
    source,
    evidence_class,
    AVG(cost_usd) as avg_cost,
    AVG(latency_ms) as avg_latency,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) as success_rate
FROM token_records
WHERE project_id = $1 AND model_id LIKE '%glm%'
GROUP BY source, evidence_class
ORDER BY evidence_class, source;
```

### 4. Project Cost Breakdown

```sql
SELECT 
    date_trunc('week', timestamp) as week,
    project_id,
    SUM(cost_usd) as total_cost,
    SUM(tokens_input + tokens_output) as total_tokens,
    SUM(CASE WHEN outcome = 'completed' THEN 1 ELSE 0 END) as completed_tasks
FROM token_records
GROUP BY week, project_id
ORDER BY week DESC, project_id;
```

### 5. SR&ED Cost Export (for T661 Schedule)

```sql
SELECT 
    project_id,
    sred_tag,
    SUM(cost_usd) as eligible_cost,
    SUM(tokens_input + tokens_output) as total_tokens,
    COUNT(*) as experimental_tasks
FROM token_records tr
JOIN task_sred_tags tst ON tr.task_id = tst.task_id
WHERE tr.sred_eligible = true
  AND tr.timestamp >= '2026‑01‑01'
  AND tr.timestamp < '2027‑01‑01'
GROUP BY project_id, sred_tag
ORDER BY project_id, sred_tag;
```

## Integration Points

### Planner Agent (Block 2.5)

- Calls `GetModelEfficiency` to choose optimal model for each task type.
- Calls `GetCostBudgetStatus` to enforce budget limits.
- Calls `RecordPlannedModelSelection` to log its decision for later analysis.

### Worker Agents (Block 4.3)

- After each LLM call, call `TokenRecorder.Record` with the `TokenRecord`.
- Batch records when possible (e.g., multiple tool calls in one agent step).

### Funding Evidence Aggregator (Block 5.4)

- Queries SR&ED‑eligible costs for reporting.
- Uses `task_sred_tags` join to map tasks to SR&ED uncertainty categories.

### Observability Stack

- ZenLedger metrics exposed via Prometheus:
  - `zen_ledger_records_total` – total records.
  - `zen_ledger_cost_usd_total` – cumulative cost.
  - `zen_ledger_tokens_total` – cumulative tokens.
- Grafana dashboards for model efficiency, project spending, etc.

## Configuration

Example `config.yaml` snippet:

```yaml
ledger:
  # Storage
  cockroachdb:
    uri: "postgresql://root@cockroachdb‑public:26257/zen_brain?sslmode=disable"

  # Local cost model
  local_cost_model:
    cpu_inference_rate: 0.001
    gpu_inference_rate: 0.02
    memory_overhead_rate: 0.0001

  # Budgets
  budgets:
    - project_id: "zen‑brain"
      budget_limit_usd: 1000.0
      period: "monthly"
    - project_id: "zen‑mesh"
      budget_limit_usd: 500.0
      period: "monthly"

  # Materialized view refresh interval
  refresh_interval_seconds: 300
```

## Monitoring

**Metrics (Prometheus):**

- `zen_ledger_records_total` – counter of recorded token records.
- `zen_ledger_cost_usd_total` – cumulative cost (by project, model, source).
- `zen_ledger_tokens_total` – cumulative tokens (input, output, cached).
- `zen_ledger_latency_ms` – histogram of LLM call latency.
- `zen_ledger_budget_remaining_usd` – gauge of remaining budget per project.

**Dashboards (Grafana):**

- Model efficiency ranking (cost per completed task, success rate).
- Project cost breakdown (weekly/monthly).
- Local vs API comparison (cost, latency, success rate).
- SR&ED‑eligible cost accumulator (running total for tax year).

## Open Questions

1. **How to handle rate‑limited API providers?** – Could add `rate_limit_delay_ms` field to separate network latency from inference latency.
2. **Should we track token usage per tool call?** – Possibly, but adds complexity; maybe aggregate per agent step.
3. **How to handle currency fluctuations?** – Store cost in USD using exchange rate at time of record; maybe add `currency` field.
4. **Should we anonymize data for sharing?** – Could hash `session_id` and `task_id` before exporting for analysis.

## Next Steps

1. Implement `internal/ledger/cockroach.go` – CockroachDB storage for `TokenRecord`.
2. Implement `internal/ledger/local_cost.go` – local cost estimation.
3. Create materialized views for efficient queries.
4. Write unit and integration tests.
5. Integrate with Planner and Worker agents.

---

*This document is a living design spec; update as implementation progresses.*