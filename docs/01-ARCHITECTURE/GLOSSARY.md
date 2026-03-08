# Glossary

## Core Concepts

### Zen‑Brain
The overall system that orchestrates AI‑assisted software development. Combines **Office** (planning) and **Factory** (execution) with a shared **Nervous System** (connectivity).

### Office
The planning layer where human intent is captured. External systems (Jira, Linear, Slack) connect via **ZenOffice** adapters. The Office analyzes intent, creates plans, and manages human‑in‑the‑loop approvals.

### Factory
The execution layer where AI agents perform work. Runs on Kubernetes, uses warm worker pools, session affinity, and isolated git worktrees. Operates exclusively on canonical `WorkItem` types.

### Nervous System
The connectivity layer that links Office and Factory. Includes message bus, ZenJournal (immutable event ledger), ZenLedger (cost accounting), and knowledge‑base integration.

## Components

### ZenOffice
Interface for work ingress from external systems. Implementations: Jira connector, Linear connector, Slack connector. Maps external tickets to canonical `WorkItem`s.

### ZenContext
Tiered memory system for agents:
- **Tier 1 (Hot)** – Redis/tmpfs for sub‑millisecond session context.
- **Tier 2 (Warm)** – Vector database (QMD) for fast knowledge retrieval.
- **Tier 3 (Cold)** – Object storage for archival logs.

### ZenJournal
Immutable event ledger with cryptographic chain hashes. Records all significant actions for auditability and SR&ED evidence collection. Built on `zen‑sdk/pkg/receiptlog`.

### ZenLedger
Token and cost accounting system. Tracks input/output tokens, cost (real for API, estimated for local), latency, and task outcome per model per task. Provides yield metrics (value‑per‑token) for optimization.

### ZenGate
Admission controller that validates and authorizes requests before they enter the Factory. Enforces input validation, authorization checks, and policy rules.

### ZenPolicy
Declarative YAML‑based rules defining what actions are allowed, required, or forbidden. Evaluated by ZenGate and other components.

### ZenGuardian
Proactive monitoring system that watches running agents and intervenes when necessary. Implements circuit breaking, anomaly detection, PII filtering, and safety boundaries.

### QMD (Question‑Answer Memory Database)
Vector‑optimized store for semantic search over the knowledge base (`zen‑docs` repository). Used by agents to retrieve relevant documentation and procedures.

## Data Types

### WorkItem
Canonical work representation that all Office connectors map to, and the Factory operates on exclusively. Includes fields for identity, classification, lifecycle, tags, evidence requirements, and source metadata.

### WorkTags
Structured tag model that replaces flat labels. Categories:
- **HumanOrg** – Epics, teams, quarters.
- **Routing** – System routing decisions.
- **Policy** – ZenGate policy evaluation.
- **Analytics** – Dashboards and reporting.
- **SRED** – SR&ED uncertainty categories.

### SREDTag
Typed enum for SR&ED uncertainty categories:
- `u1_dynamic_provisioning`
- `u2_security_gates`
- `u3_deterministic_delivery`
- `u4_backpressure`
- `experimental_general`

### AIAttribution
Structured attribution for AI‑generated content. Includes agent role, model used, session ID, task ID, and timestamp. Injected as a header in Jira comments and descriptions.

## Kubernetes Resources

### ZenProject
Custom Resource Definition (CRD) representing a logical project (e.g., zen‑brain, zen‑mesh). Contains project‑level configuration: cluster reference, repository URLs, KB scopes, SR&ED tags, funding programs, and budget.

### ZenCluster
CRD representing a physical or virtual Kubernetes cluster that can execute work. Includes endpoint, authentication reference, capacity, status, and location.

## Processes

### ReMe (Recursive Memory)
Protocol for agents to reconstruct their state when waking up. Reads ZenJournal entries, reconstructs causal chain, verifies current state, then continues execution.

### Session Affinity
Routing multi‑step tasks to the same worker to preserve context in `/dev/shm`. Enabled by warm worker pools and cluster‑aware dispatcher.

### Warm Worker Pool
Long‑running worker pods that keep models loaded, eliminating cold‑start overhead. Use git worktrees for isolation and shared `/factory` volume for repositories.

## Funding & Compliance

### SR&ED (Scientific Research and Experimental Development)
Canadian tax‑credit program for experimental development work. Zen‑Brain collects SR&ED evidence by default for all eligible tasks.

### IRAP (Industrial Research Assistance Program)
Canadian funding program for technology innovation. Zen‑Brain can generate IRAP technical reports from accumulated evidence.

### ZenFunding
Interface for SR&ED/IRAP alignment. Records evidence, generates T661 narratives and technical reports.

## Development

### k3d
Lightweight Kubernetes distribution used for local development. Zen‑Brain includes `make dev‑up`/`dev‑down` targets for managing a local k3d cluster.

### zen‑sdk
Shared Go library with reusable packages for cross‑cutting concerns: receiptlog, scheduler, dedup, dlq, observability, retry, crypto, etc. **Rule:** If zen‑sdk has it, use it.

### Contracts Package (`pkg/contracts`)
Neutral package containing all canonical data types used across Zen‑Brain. Ensures Office and Factory agree on data shape without creating circular dependencies.