# Configuration Reference

This document describes all configuration options available in Zen‑Brain. Configuration is stored in YAML files, with environment variable overrides supported for sensitive values.

## Configuration File Location

**Canonical runtime config:** `$ZEN_BRAIN_HOME/config.yaml` (where `ZEN_BRAIN_HOME` defaults to `~/.zen-brain`). No search of current directory or repo paths at runtime.

- Override: command‑line `--config /path/to/config.yaml` or set config path explicitly when calling `LoadConfig(path)`.
- **Repo `config/`**: deployment env contract only (e.g. `config/clusters.yaml` for Helmfile/zen.py). Not used as application runtime config.
- **Repo `configs/`**: app config templates/examples only. Copy to `$ZEN_BRAIN_HOME/config.yaml` for runtime use.

**Runtime state (no repo-local paths):** Session persistence uses `$ZEN_BRAIN_HOME/sessions` (or `ZEN_BRAIN_DATA_DIR`). Coverage and other build artifacts use `.artifacts/` (e.g. `.artifacts/coverage/coverage.out`). No runtime DB or coverage files are written into the repo tree.

## Top‑Level Structure

```yaml
# config.yaml
version: v1

# Core settings
home: ~/.zen‑brain  # Overridden by ZEN_BRAIN_HOME env var
cluster_id: local‑machine‑1
project_id: zen‑brain

# Component configurations
context: ...
zen_context: ...   # Block 3: tier1_redis, tier2_qmd, tier3_s3, journal, cluster_id, required
message_bus: ...   # Block 3: enabled, kind, redis_url, stream, required
ledger: ...
gate: ...
policy: ...
llm: ...
jira: ...
kb: ...
```

## Block 3 runtime and message bus

Block 3 uses a **canonical bootstrap** (`internal/runtime.Bootstrap`) driven by config and env. The following apply to both `zen-brain` and `apiserver` when they load config.

**Message bus** (optional):

```yaml
message_bus:
  enabled: false
  kind: redis
  redis_url: ""    # or set REDIS_URL env
  stream: zen-brain.events
  required: false
```

When `ZEN_BRAIN_MESSAGE_BUS=redis` is set, the bus is enabled and uses `REDIS_URL` (default `redis://localhost:6379`). Session lifecycle events (session.created, session.transitioned, etc.) are published to the configured stream when `session.Config.EventBus` is set.

**Proof-of-work signing (env):** When `ZEN_PROOF_SIGNING_KEY` is set, proof-of-work artifacts are signed with HMAC-SHA256 (digest of proof data). Verification uses the same key; set the env at verify time to verify. Algorithm: `HMAC-SHA256`; KeyID is a short hash of the key.

**Strictness (env):** To require capabilities and fail startup when unavailable, set:

- `ZEN_RUNTIME_PROFILE=prod` or `ZEN_BRAIN_STRICT_RUNTIME=1` — **fail-closed**: require all of ZenContext, QMD, Ledger, MessageBus; startup fails if any required capability is missing (no mock/stub fallback).
- `ZEN_BRAIN_REQUIRE_ZENCONTEXT=1`, `ZEN_BRAIN_REQUIRE_QMD=1`, `ZEN_BRAIN_REQUIRE_LEDGER=1`, `ZEN_BRAIN_REQUIRE_MESSAGEBUS=1` — require the corresponding capability.

Config can also set `zen_context.required`, `ledger.required`, `message_bus.required` in YAML.

**Runtime commands:** `zen-brain runtime doctor` (readable summary), `zen-brain runtime report` (JSON), `zen-brain runtime ping` (exit non-zero if required capability unhealthy).

**LLM local worker (Ollama):** When **`OLLAMA_BASE_URL`** is set (e.g. `http://localhost:11434` or in-cluster `http://ollama:11434`), the **local-worker** lane uses the real Ollama provider (`internal/llm/ollama_provider.go`) and calls Ollama’s `/api/chat` endpoint. Model and timeout come from gateway config (`local_worker_model`, `local_worker_timeout`). When `OLLAMA_BASE_URL` is unset, the gateway falls back to the simulated local worker (`internal/llm/local_worker.go`). **In-cluster Ollama (Block 5):** Deployment is **Helm/Helmfile-based and declarative**. Set `deploy.use_ollama: true` and `deploy.ollama.models: ["qwen3.5:0.8b"]` (or other models) in `config/clusters.yaml`. Run `make dev-up` or `python3 scripts/zen.py env redeploy --env <env>`; values are generated from clusters.yaml and Helmfile deploys the Ollama chart with a **model preload Job** (no manual `kubectl exec ... ollama pull` in the standard path). For emergency or one-off model pulls, use the StatefulSet pod: `kubectl exec -it ollama-0 -n zen-brain -- ollama pull <model>` (or `statefulset/ollama`). See `deploy/README.md` and `charts/zen-brain-ollama/`.

**Mock/degraded paths (Block 3/5):** By default, capabilities are optional. **QMD** can fall back to mock when the `qmd` CLI is missing or unavailable (Tier 2 warm store then disabled or mock). **Ledger** falls back to stub when no DSN is set or Cockroach is unreachable; the runtime report shows `ledger.mode: stub`. To require a capability and fail startup instead of using a fallback, set the corresponding `ZEN_BRAIN_REQUIRE_*` env or `required: true` in config.

## ZenContext Configuration

```yaml
context:
  # Tier 1 (Hot) – Redis + tmpfs
  tier1:
    redis:
      address: redis‑service:6379
      password: "${REDIS_PASSWORD}"  # env var
      db: 0
      pool_size: 10
      timeout_seconds: 5
    tmpfs:
      size_limit_mb: 512
      sync_interval_seconds: 5

  # Tier 2 (Warm) – Vector database (CockroachDB with C‑SPANN)
  tier2:
    cockroachdb:
      uri: "postgresql://root@cockroachdb‑public:26257/zen_brain?sslmode=disable"
      embedding_model: "nomic‑embed‑text"  # or "text‑embedding‑3‑small"
      embedding_dimension: 768
      max_connections: 20

  # Tier 3 (Cold) – Object storage
  tier3:
    object_store:
      provider: "minio"  # minio, s3, gcs
      endpoint: "minio‑service:9000"
      bucket: "zen‑brain‑archives"
      access_key: "${MINIO_ACCESS_KEY}"
      secret_key: "${MINIO_SECRET_KEY}"
      region: ""  # optional

  # Session management
  session_ttl_minutes: 30
  archive_after_days: 1
```

## ZenJournal Configuration

```yaml
journal:
  # Active storage
  database:
    path: "~/.zen‑brain/journal.db"  # SQLite with receiptlog
    max_size_mb: 1024
    backup_interval_seconds: 300

  # Backup to S3‑compatible storage
  backup:
    enabled: true
    interval_seconds: 300
    s3:
      bucket: "zen‑brain‑journals"
      prefix: "{clusterID}/"
      endpoint: "minio‑service:9000"
      access_key: "${MINIO_ACCESS_KEY}"
      secret_key: "${MINIO_SECRET_KEY}"

  # Archival (move old receipts to cold storage)
  archival:
    enabled: true
    after_days: 30
    s3:
      bucket: "zen‑brain‑archives"
      prefix: "journals/{year}/{month}/"

  # Multi‑cluster aggregation
  aggregation:
    enabled: true
    interval_seconds: 300
    control_plane_endpoint: "https://control‑plane:8080"
```

## ZenLedger Configuration

```yaml
ledger:
  # Storage
  cockroachdb:
    uri: "postgresql://root@cockroachdb‑public:26257/zen_brain?sslmode=disable"

  # Local inference cost model (for cost estimation)
  local_cost_model:
    cpu_inference_rate: 0.001    # $/min of CPU time
    gpu_inference_rate: 0.02     # $/min of GPU time
    memory_overhead_rate: 0.0001 # $/GB/min

  # Budgets per project
  budgets:
    - project_id: "zen‑brain"
      budget_limit_usd: 1000.0
      period: "monthly"  # monthly, quarterly, yearly
    - project_id: "zen‑mesh"
      budget_limit_usd: 500.0
      period: "monthly"

  # Materialized view refresh interval
  refresh_interval_seconds: 300
```

## ZenGate & ZenPolicy Configuration

```yaml
gate:
  # Validation
  validators:
    - name: cost_limit
      enabled: true
    - name: resource_quota
      enabled: true

  # Policy engine
  policy_engine:
    provider: "certo"  # or "opa", "builtin"
    default_effect: "deny"
    audit_log_enabled: true

  # Database for policy rules
  database:
    uri: "postgresql://root@cockroachdb‑public:26257/zen_brain?sslmode=disable"

policy:
  # Rule storage
  rule_refresh_interval_seconds: 30

  # Default policies (built‑in)
  default_policies:
    - name: deny_all
      effect: deny
      priority: 0
```

## LLM Gateway Configuration

```yaml
llm:
  # Provider configurations
  providers:
    openai:
      enabled: true
      api_key: "${OPENAI_API_KEY}"
      default_model: "gpt‑4‑turbo‑preview"
      timeout_seconds: 30
      max_retries: 3
    anthropic:
      enabled: true
      api_key: "${ANTHROPIC_API_KEY}"
      default_model: "claude‑sonnet‑4‑6"
      timeout_seconds: 30
    ollama:
      enabled: true
      base_url: "http://ollama‑service:11434"
      default_model: "glm‑4.7"
      timeout_seconds: 120
    mock:
      enabled: false

  # Routing
  routing:
    default_strategy: "cost_optimized"  # cost_optimized | quality_optimized | balanced
    prefer_local: true
    fallback_to_api: true
    max_retries: 3
    retry_delay_ms: 1000
    task_overrides:
      - task_type: "debug"
        preferred_models: ["glm‑4.7‑local", "claude‑sonnet‑4‑6‑api"]
        max_cost_usd: 0.30
      - task_type: "documentation"
        preferred_models: ["glm‑4.7‑local"]
        max_cost_usd: 0.10

  # Token recording
  token_recorder:
    enabled: true
    batch_size: 10
    flush_interval_seconds: 5

  # Embedding model selection
  embedding:
    default_model: "nomic‑embed‑text"
    dimension: 768
    provider: "ollama"
```

## Jira Connector Configuration

```yaml
jira:
  enabled: true
  base_url: "https://your‑domain.atlassian.net"
  project: "ZEN"

  # Authentication
  auth:
    type: "pat"  # pat, oauth2, basic
    pat: "${JIRA_PAT}"  # Personal Access Token
    # oauth2:
    #   client_id: "..."
    #   client_secret: "..."
    #   token_url: "..."

  # Webhook handling
  webhook:
    enabled: true
    path: "/webhooks/jira"
    secret: "${JIRA_WEBHOOK_SECRET}"
    port: 8081

  # Field mapping
  field_mapping:
    kb_scope_custom_field: "customfield_10010"
    sred_custom_field: "customfield_10011"

  # Status mapping (Jira → canonical WorkStatus)
  status_mapping:
    "To Do": "requested"
    "In Progress": "running"
    "Done": "completed"
    "Backlog": "requested"

  # WorkType mapping (Jira issue type → WorkType)
  worktype_mapping:
    "Bug": "debug"
    "Task": "implementation"
    "Story": "design"
    "Epic": "research"
```

## Knowledge Base (QMD) Configuration

```yaml
kb:
  # Source repositories
  repos:
    - name: "zen‑docs"
      url: "git@github.com:kube‑zen/zen‑docs.git"
      branch: "main"
      local_path: "/factory/repos/zen‑docs.git"

  # QMD settings
  qmd:
    binary_path: "/usr/local/bin/qmd"
    embedding_model: "nomic‑embed‑text"
    chunk_size_tokens: 512
    chunk_overlap_tokens: 50

  # Scopes
  scopes:
    company:
      sources:
        - repo: "zen‑docs"
          paths: ["/company/*", "/policies/*"]
    general:
      sources:
        - repo: "zen‑docs"
          paths: ["/guides/*", "/reference/*"]
    zen‑brain:
      sources:
        - repo: "zen‑brain‑1.0"
          paths: ["/docs/*", "/adr/*"]
        - repo: "zen‑docs"
          paths: ["/projects/zen‑brain/*"]

  # Confluence sync
  confluence:
    enabled: true
    base_url: "https://your‑domain.atlassian.net/wiki"
    username: "${CONFLUENCE_USERNAME}"
    api_token: "${CONFLUENCE_API_TOKEN}"
    spaces:
      company: "COMPANY"
      general: "ENG"
      zen‑brain: "ZENBRAIN"
    sync_interval_minutes: 60
```

## Factory (Execution) Configuration

```yaml
factory:
  # Worker pool
  worker_pool:
    replicas: 3
    image: "zen‑brain‑worker:latest"
    resources:
      requests:
        cpu: "1"
        memory: "2Gi"
      limits:
        cpu: "2"
        memory: "4Gi"
    tmpfs_size_mb: 512

  # Shared volume
  shared_volume:
    type: "hostPath"  # hostPath, pvc
    host_path: "/factory"
    size_gb: 50

  # Session affinity
  session_affinity:
    enabled: true
    session_ttl_minutes: 30
    mapping_store: "cockroachdb"  # cockroachdb, redis

  # Worktree management
  worktree:
    base_path: "/factory/worktrees"
    cleanup_after_seconds: 86400  # 24 hours
```

## Multi‑Cluster Configuration

```yaml
multicluster:
  # Control plane
  control_plane:
    enabled: true
    endpoint: "https://control‑plane:8080"
    auth_token: "${CONTROL_PLANE_TOKEN}"

  # Data plane agent
  data_plane:
    cluster_id: "local‑machine‑1"
    cluster_name: "Local Development"
    location: "local"  # local, cloud, edge
    capacity:
      cpu_cores: 8
      memory_gb: 32
    labels:
      environment: "dev"
      team: "platform"
```

## Observability Configuration

```yaml
observability:
  # Metrics (Prometheus)
  metrics:
    enabled: true
    port: 9090
    path: "/metrics"

  # Tracing (OpenTelemetry)
  tracing:
    enabled: true
    endpoint: "jaeger‑collector:4317"
    sampler: "parent‑based‑always‑on"

  # Logging
  logging:
    level: "info"  # debug, info, warn, error
    format: "json"
    output: "stdout"
```

## Environment Variable Overrides

Any configuration value can be overridden by an environment variable using the pattern `ZEN_BRAIN_<SECTION>_<KEY>` with nested keys joined by underscores.

Example:

```yaml
context:
  tier1:
    redis:
      address: "redis:6379"
```

Can be overridden by:

```bash
export ZEN_BRAIN_CONTEXT_TIER1_REDIS_ADDRESS="redis‑prod:6379"
```

## Validation

Configuration is validated on startup. Invalid configuration will cause Zen‑Brain to exit with an error message indicating the problem.

## Configuration Generation

A configuration template can be generated with:

```bash
zen‑brain config generate > config.yaml
```

Then edit the generated file to fill in your values.

---

*Configuration is evolving; refer to `configs/config.dev.yaml` for the latest template.*