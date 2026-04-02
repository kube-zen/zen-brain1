# L1/L2 Lane Runbook — Updated for PHASE 34/35

**Last updated:** 2026-04-02

## L1 Lane (qwen3.5:0.8B — llama.cpp, port 56227)

### What It Does
L1 produces useful analysis reports for the zen-brain1 codebase using deterministic evidence extraction and bounded reasoning.

### How It Works (PHASE 34/35 rewrite)

1. **Code does discovery** — deterministic shell commands extract real repo evidence
2. **Code shapes evidence** — evidence is trimmed, structured, labeled, and bounded (≤80 lines)
3. **Code pre-extracts candidates** — for defects/bug-hunting, candidates are extracted, deduplicated, and diversity-capped
4. **Model gets bounded task packet** — Evidence Bundle → Scope → Output Spec → Task → Constraints
5. **Model ranks/summarizes** — NOT discovering from scratch; using what code already found
6. **Code validates output** — fail-closed checks: size, structure, required sections, repetition, file grounding, degenerate tables
7. **Code enforces MaxFindings** — trimmed to configured limit after generation

### Task Classes (10 report types)

| Task | Evidence Strategy | Max Findings | Status |
|------|------------------|-------------|--------|
| defects | Pre-extracted candidates (4 scanners) | 8 | ✅ Grounded |
| bug_hunting | Pre-extracted candidates (4 scanners) | 8 | ✅ Grounded |
| tech_debt | Structured evidence bundle | 10 | ✅ Grounded |
| dead_code | Structured evidence bundle | 10 | ✅ Grounded |
| stub_hunting | Structured evidence bundle | 10 | ✅ Grounded |
| roadmap | Structured evidence bundle | 5 | ✅ Grounded |
| executive_summary | Structured evidence bundle | 5 | ✅ Grounded |
| package_hotspots | Structured evidence bundle | 10 | ✅ Grounded |
| test_gaps | Structured evidence bundle | 10 | ✅ Grounded |
| config_drift | Structured evidence bundle | 8 | ✅ Grounded |

### Evidence Strategies

**Structured Evidence Bundle** (tech_debt, roadmap, etc.):
- Multiple labeled shell commands per task
- Each command has Label, Cmd, Lines limit
- Total capped at 80 lines
- Code runs commands, model summarizes

**Pre-Extracted Candidates** (defects, bug_hunting):
- Deterministic scanners with category-specific patterns
- Deduplication by (file, category)
- Diversity cap: max 3 per category
- Model ranks and prioritizes from candidates
- 0.1 pattern: code discovers, AI reasons

### Validation Classification

| Status | Meaning | Jira Label |
|--------|---------|------------|
| `success` | All checks passed | `ai:completed` |
| `success-needs-review` | Passes but file grounding < 30% | `ai:needs-review` |
| `artifact-fail` | Too short, no headings, degenerate table | `ai:blocked` |
| `context-fail` | Repetition pattern detected | `ai:blocked` |

### No-Think
All L1 useful-task requests use `enable_thinking: false` via `chat_template_kwargs`.
This is proven from PHASE 18 and consistently applied.

## L2 Lane (2B — llama.cpp, port 60509)

Currently used as fallback/escalation for bounded code tasks.
Not used for report tasks by default.
