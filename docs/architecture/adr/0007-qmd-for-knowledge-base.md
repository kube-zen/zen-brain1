# ADR 0007: Use qmd for knowledge base search with git as source of truth

## Status

**Accepted** (2026‑03‑07)

## Context

Zen‑Brain agents need fast access to relevant documentation and procedures. We need a knowledge base (KB) system that:

1. **Integrates with Git** – documentation is written in markdown and stored in the `zen‑docs` repository.
2. **Supports semantic search** – agents can ask natural‑language questions and get relevant excerpts.
3. **Scales to thousands of documents** – with low latency (<100ms) for agent queries.
4. **Respects scope isolation** – company‑wide vs project‑specific vs general documentation.
5. **Simple to operate** – no separate vector database cluster for 1.0 if possible.

Alternatives considered: building a custom vector store in CockroachDB, using Pinecone/Weaviate, using Elasticsearch.

## Decision

Use **qmd (Question‑Answer Memory Database)** as the search/index layer over the `zen‑docs` Git repository, with the following principles:

1. **Git repo is the source of truth** – `zen‑docs` repository holds all canonical documentation.
2. **qmd is search/index over git** – qmd indexes the `zen‑docs` repo; agents query via CLI with JSON output.
3. **Confluence is a published mirror** – one‑way sync from `zen‑docs` → Confluence for human‑friendly browsing.
4. **Jira is the human entry point** – tickets link to KB scopes/docs; planner uses scopes to narrow retrieval.
5. **No CockroachDB for KB/QMD in 1.0** – CockroachDB holds structured runtime data (ZenLedger, session state, policies), not the document corpus.

**Implementation details:**

- **qmd CLI** – invoked as a subprocess: `qmd search --repo /path/to/zen‑docs --query "dynamic provisioning" --json --limit 5`.
- **Embedding model** – `nomic‑embed‑text` (768 dimensions) for local inference, or `text‑embedding‑3‑small` (1536d) for API‑based.
- **Chunking strategy** – 512‑token chunks with 50‑token overlap, respecting document structure.
- **Scope isolation** – each chunk tagged with scope (`company`, `general`, `zen‑brain`, `zen‑lock`, etc.).
- **Confluence sync** – one‑way automated publishing; edits in Confluence are overwritten on next sync.

## Consequences

### Positive

- **Simple architecture** – no separate vector database cluster to manage in 1.0.
- **Git‑centric workflow** – documentation changes follow normal PR/merge process.
- **Fast iteration** – qmd indexes incrementally; new commits trigger re‑indexing of changed files.
- **Cost effective** – local embedding model eliminates API costs for KB queries.
- **Portable** – qmd can run anywhere Git is available (local, CI, Kubernetes).

### Negative

- **qmd is external tool** – not a Go library; requires shelling out and parsing JSON.
- **Limited scalability** – qmd may not scale to millions of vectors as well as dedicated vector DBs.
- **No advanced features** – lacks built‑in hybrid search, reranking, metadata filtering.
- **Operational overhead** – need to monitor qmd process, handle failures, manage embedding model updates.

### Neutral

- The decision is **for 1.0 only** – we can replace qmd with CockroachDB + C‑SPANN vector index in a future version without changing the `pkg/qmd` interface.
- **CockroachDB is still used** for structured runtime data; only the document corpus is excluded.

## Alternatives Considered

### 1. CockroachDB with C‑SPANN vector index

- **Pros**: Single database for all structured and vector data, advanced query capabilities, scalability.
- **Cons**: Adds operational complexity for 1.0, requires vector dimension decisions upfront, larger footprint.

### 2. Dedicated vector database (Pinecone, Weaviate, Qdrant)

- **Pros**: Best‑in‑class vector search, managed service available, rich features.
- **Cons**: Additional external dependency, cost, network latency, vendor lock‑in.

### 3. Elasticsearch with dense vectors

- **Pros**: Mature, supports hybrid search, already used elsewhere in the ecosystem.
- **Cons**: Heavyweight, operational burden, not optimized for pure vector search.

### 4. Build custom vector store in Go

- **Pros**: Full control, no external dependencies.
- **Cons**: Significant development effort, likely inferior performance to specialized solutions.

Choosing qmd allows us to **ship 1.0 faster** while keeping the door open to upgrade later via the `pkg/qmd` abstraction.

## Related Decisions

- [ADR‑0004](0004‑multi‑cluster‑crds.md) – Multi‑cluster topology (KB scopes align with projects).
- [KB/QMD Strategy](../kb‑qmd.md) – detailed implementation plan.
- Construction Plan V6.0, Section 3.6 – KB Ingestion Service Architecture.

## References

- qmd documentation – https://github.com/kube‑zen/qmd
- `nomic‑embed‑text` model card – https://ollama.com/library/nomic‑embed‑text
- Construction Plan V6.0, Section 3.2 – Knowledge Base with QMD.