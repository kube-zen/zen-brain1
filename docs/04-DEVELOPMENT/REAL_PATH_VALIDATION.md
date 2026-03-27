> **NOTE:** This document references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

# Real-Path Validation for Intelligence Block

**Date**: 2026-03-12
**Status**: Operational proof pending (code paths exist)

## Overview

This document provides a clean proof story for validating real dependency paths in the Intelligence block (Block 5). It covers:

1. **Real Ollama inference** – LLM Gateway → local-worker → Ollama
2. **Real QMD knowledge base** – QMD CLI → zen-docs repository
3. **Real evidence/mining** – Evidence vault, mining pipeline

Each validation can be run independently when dependencies are available.

## 1. Real Ollama Inference

### Prerequisites
- Ollama service running (local or remote)
- `OLLAMA_BASE_URL` environment variable set
- Model `qwen3.5:0.8b` pulled (`ollama pull qwen3.5:0.8b`)

### Validation Test

The system includes an integration test that validates the complete inference chain:

```bash
# Run the real inference test
cd /path/to/zen-brain1
OLLAMA_BASE_URL=http://localhost:11434 go test ./internal/integration -run TestRealInferencePath -v
```

### Expected Output
```
=== RUN   TestRealInferencePath
    real_inference_test.go:53: Testing real inference path with Ollama at: http://localhost:11434
    real_inference_test.go:54: This test validates: Client → Gateway → Local-Worker → Ollama → Response
    real_inference_test.go:100: ✅ REAL INFERENCE SUCCESSFUL!
    real_inference_test.go:101: ============================================================
    real_inference_test.go:102:    Model: qwen3.5:0.8b
    real_inference_test.go:103:    Response: "hello world"
    real_inference_test.go:104:    Latency: 1.234s
    real_inference_test.go:105: ============================================================
--- PASS: TestRealInferencePath (2.00s)
```

### Manual Validation

You can also validate via the CLI:

```bash
# Set environment
export OLLAMA_BASE_URL=http://localhost:11434

# Run vertical slice with mock work item (will use real Ollama)
./zen-brain vertical-slice --mock
```

Check logs for:
```
[LLM Gateway] local-worker lane: connected to Ollama at http://localhost:11434
[Analyzer] Using real Ollama inference
```

## 2. Real QMD Knowledge Base

### Prerequisites
- `qmd` CLI installed and in PATH
- Zen-docs repository cloned locally (`~/.zen/zen-brain1/zen-docs` or configured path)
- QMD index built (`qmd index`)

### Validation Test

The QMD adapter includes real-path logic that fails closed when `qmd` is not available:

```go
// internal/qmd/adapter.go
func NewRealQMDAdapter(repoPath string) (*RealQMDAdapter, error) {
    if !qmdAvailable() {
        return nil, fmt.Errorf("qmd CLI not available in PATH")
    }
    // ... real implementation
}
```

To validate:

```bash
# Check qmd availability
which qmd

# Run QMD population test
go test ./internal/qmd -v
```

### Manual Validation

```bash
# Set up zen-docs repository
git clone https://github.com/kube-zen/zen-docs ~/.zen/zen-brain1/zen-docs

# Build index
cd ~/.zen/zen-brain1/zen-docs
qmd index

# Verify QMD adapter works
cat > /tmp/test_qmd.go << 'EOF'
package main

import (
    "fmt"
    "github.com/kube-zen/zen-brain1/internal/qmd"
)

func main() {
    adapter, err := qmd.NewRealQMDAdapter("/home/user/.zen/zen-brain1/zen-docs")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Println("✅ QMD adapter created successfully")
}
EOF
cd /path/to/zen-brain1
go run /tmp/test_qmd.go
```

## 3. Real Evidence Mining Pipeline

### Prerequisites
- ZenContext configured with real Tier 1 (Redis) and Tier 3 (S3)
- Evidence vault directory writable

### Validation Test

The evidence vault includes real-path storage:

```go
// internal/evidence/vault.go
func NewEvidenceVault(baseDir string) (*EvidenceVault, error) {
    // Creates real directory structure
}
```

To validate the complete mining pipeline:

```bash
# Run intelligence mining tests
go test ./internal/intelligence -v

# Run session checkpoint tests (includes evidence storage)
go test ./internal/session -v
```

### Manual Validation with Real Dependencies

```bash
# Set up real dependencies
export REDIS_URL=redis://localhost:6379
export S3_ENDPOINT=http://localhost:9000
export S3_BUCKET=zen-brain
export S3_ACCESS_KEY_ID=minioadmin
export S3_SECRET_ACCESS_KEY=minioadmin

# Run a complete session with evidence collection
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
./zen-brain vertical-slice --mock

# Check evidence was stored
ls -la ~/.zen/zen-brain1/evidence/
```

## Canonical Workflow: End-to-End Real Path

For a complete real-path validation, run this workflow:

### Step 1: Start dependencies
```bash
# Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# Start MinIO (S3 compatible)
docker run -d -p 9000:9000 -p 9001:9001 \
  minio/minio server /data --console-address ":9001"

# Start Ollama
ollama serve &
```

### Step 2: Configure environment
```bash
export REDIS_URL=redis://localhost:6379
export S3_ENDPOINT=http://localhost:9000
export S3_BUCKET=zen-brain
export S3_ACCESS_KEY_ID=minioadmin
export S3_SECRET_ACCESS_KEY=minioadmin
export OLLAMA_BASE_URL=http://localhost:11434
export ZEN_DOCS_PATH=~/.zen/zen-brain1/zen-docs
```

### Step 3: Run validation script
```bash
# Build zen-brain
go build -o zen-brain cmd/zen-brain/main.go

# Run full pipeline with real dependencies
./zen-brain vertical-slice --mock
```

### Step 4: Verify real paths were used
Check logs for:
- `[LLM Gateway] local-worker lane: connected to Ollama`
- `[ZenContext] Using real Redis storage`
- `[EvidenceVault] Storing evidence at`
- `[QMD] Real adapter created`

## Fallback Behavior

When real dependencies are unavailable, the system fails closed:

1. **Ollama unavailable**: `OLLAMA_BASE_URL` not set → LLM Gateway uses simulated local-worker
2. **QMD unavailable**: `qmd` not in PATH → QMD adapter returns error (fail closed)
3. **Redis unavailable**: `REDIS_URL` not set → ZenContext returns nil (optional component)
4. **S3 unavailable**: Missing S3 env vars → Tier 3 storage disabled

This fail-closed posture ensures production reliability while allowing development with explicit stub opt-in.

## Next Steps

- [ ] Create automated validation script that runs the canonical workflow
- [ ] Add CI job that runs real-path tests when dependencies are available
- [ ] Document operational deployment checklist for production
- [ ] Enhance evidence mining with real S3 storage validation

## References

- `internal/integration/real_inference_test.go` – Ollama real-path test
- `internal/qmd/adapter.go` – QMD real adapter
- `internal/evidence/vault.go` – Evidence vault implementation
- `docs/04-DEVELOPMENT/CONFIGURATION.md` – Environment variable configuration