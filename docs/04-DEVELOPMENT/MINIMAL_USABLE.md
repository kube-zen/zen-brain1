> **NOTE:** This document references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

# Minimal Usable Runbook

This runbook is the fastest honest path to getting zen-brain1 working as a developer tool.

## What "minimal usable" means here

The system is considered minimally usable when all of the following work on one machine:

1. `zen-brain runtime doctor`
2. `zen-brain office doctor`
3. `zen-brain vertical-slice --mock`

This proves:
- the CLI builds
- Block 3 runtime inspection works in dev mode
- Office bootstraps without real Jira
- the end-to-end pipeline runs with a mock work item
- proof-of-work artifacts are generated locally

It does **not** yet prove:
- real Jira integration
- real Redis/S3-backed ZenContext
- real message bus
- k3d/Helmfile deployment
- production readiness

## Step 1: prepare config

```bash
make minimal-config
```

This creates `~/.zen-brain/config.yaml` from `configs/config.minimal.yaml` if missing.

## Step 2: build

```bash
make build
```

## Step 3: run the smoke path

```bash
make smoke
```

Equivalent commands:

```bash
./bin/zen-brain runtime doctor
./bin/zen-brain office doctor
./bin/zen-brain vertical-slice --mock
```

## Optional real dependency upgrades

### Real Ollama

```bash
export OLLAMA_BASE_URL=http://localhost:11434
./bin/zen-brain test
./bin/zen-brain vertical-slice --mock
```

### Real Jira

```bash
export JIRA_URL=https://your-domain.atlassian.net
export JIRA_EMAIL=you@example.com
export JIRA_API_TOKEN=...
export JIRA_PROJECT_KEY=ZEN
./bin/zen-brain office doctor
./bin/zen-brain vertical-slice ZEN-123
```

## Recommended adoption order

1. mock vertical slice
2. real Ollama
3. real Jira
4. real Redis/S3 ZenContext
5. message bus
6. k3d + Helmfile deployment

## Notes

- `make run` only launches the CLI binary and is not the best first-run path.
- `vertical-slice --mock` is the correct first end-to-end proof.
- In dev mode, simulated LLM providers are acceptable for basic smoke validation.
