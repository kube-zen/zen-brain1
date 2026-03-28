# L1 Attribution Policy

**Version:** 1.0
**Created:** 2026-03-28
**Status:** Active

## Core Principle

**A task counts as "L1 did useful work" ONLY if it leaves behind an artifact traceable to L1 output.**

No artifact = no attribution claim. This is non-negotiable.

## Attribution Categories

| Category | Definition | Counts as L1 Work? |
|----------|-----------|-------------------|
| `l1-produced` | L1 output parsed correctly, produced usable content, validated | **Yes** |
| `l1-produced-needs-review` | L1 output parsed but needs human review before application | **Partial** — L1 drafted, not verified |
| `supervisor-written` | GLM-5 or human wrote the actual output | **No** |
| `script-only` | Automated script did the work, no LLM involved | **No** |
| `infra-only` | Infrastructure change with no LLM authorship | **No** |

## Required Attribution Fields

Every task in telemetry, ledger, and evidence must record:

```json
{
  "produced_by": "l1 | l2 | supervisor | script | none",
  "first_pass_model": "qwen3.5:0.8b-q4 | qwen3.5:2b-q4 | none",
  "supervisor_intervention": "none | normalization_only | prompt_fix | manual_rewrite | script_override",
  "artifact_authorship": "l1 | mixed | supervisor | none",
  "final_disposition": "l1-produced | l1-produced-needs-review | supervisor-written | script-only | failed"
}
```

## What Counts as L1 Artifact Evidence

A task claiming L1 authorship must have at least one saved artifact:

- **Raw L1 output** — the complete response from the model
- **Normalized output** — parsed, cleaned JSON after normalization
- **Final accepted payload** — the version that was actually used

Storage path: `docs/05-OPERATIONS/evidence/l1-attribution-pilot/{KEY}_{target}_raw.json`

## What Does NOT Count as L1 Work

- GLM-5 writing a Python script that then operates on Jira
- Bulk Jira transitions with no per-ticket L1 output
- Supervisor writing remediation patches manually
- Config changes made by automation scripts
- Discovery/report generation (these are scanner output, not L1 remediation)

## Measurement Honesty Rules

1. **Commit authorship alone is not the metric.** A commit by GLM-5 that used a script is not L1 work.
2. **Jira state movement alone is not the metric.** Moving tickets to Done via script is ops cleanup, not factory output.
3. **Ticket count alone is not the metric.** Creating tickets is not the same as closing them via L1.
4. **Bulk cleanup is useful but separate.** Ops cleanup (stale ticket closing, deduplication) goes in a separate category from L1 production.

## L1 Capability Assessment (Updated)

### v1 Pilot (full-file contract) — 2026-03-28
- **Success rate:** 30% (3/10 tasks produced parseable, usable output)
- **Timeout rate:** 40% (4/10 hit the 120s timeout)
- **Truncation rate:** 30% (3/10 produced partial/unparseable output)

### v2 Pilot (patch-oriented contract) — 2026-03-28
- **Success rate:** 60% (6/10 tasks scored 25/25 on quality gate)
- **Failure rate:** 40% (4/10 — 3 code_edit timeouts, 1 parse failure)
- **Average L1 time for successful tasks:** 13s (range: 8.6–26.5s)
- **Forbidden fields:** 0 (no tasks included new_content/file_body)
- **Patch concreteness:** 100% of successful tasks produced actionable sed/echo commands

### Key Insight
Switching from full-file generation to patch-oriented output **doubled** the L1 success rate (30% → 60%).
The remaining 40% failures are code_edit tasks where L1 still times out.

### Timeout Policy (Corrected)

**Healthy-but-slow is not the same as hung.** CPU-only 0.8b is allowed to be slow.

| Task Shape | Adaptive Timeout |
|-----------|-----------------|
| Small config_change (<100 char desc) | 90s |
| Normal config_change / code_edit / doc_update | 120s |
| Complex code_edit / large doc_update | 180s |

**Global timeout reduction is REJECTED.** The prior 60s blanket timeout killed productive slow generations.

**Blanket fail-fast is the wrong fix.** Output-contract reduction is preferred over patience reduction.

### Truncation Repair

L1 sometimes produces correct JSON that gets cut off mid-field (common in sed commands with escape sequences). A JSON truncation repair step recovers these by:
1. Finding the last complete key-value pair before truncation
2. Closing open braces/brackets
3. Parsing the repaired JSON

This recovered 5/10 corrective retry tickets that would have been classified as failures under the old policy.

### Output Classification

Every L1 call is classified as one of:
- **fast-productive** — completed quickly with parseable output
- **slow-but-productive** — took 60-80s but produced usable output (recovered by truncation repair)
- **no-output** — timed out with empty response
- **truncated-output** — got substantial output but couldn't parse even after repair
- **repetitive-degenerate** — L1 produced repetitive/garbage output
- **infra-fail** — infrastructure error (network, server down)

Only the last three justify aggressive fail-fast behavior.

## Decision Framework

### Before Expanding L1 Workload:
- [x] L1 success rate ≥ 60% on bounded tasks — **MET (v2 patch contract)**
- [x] Timeout rate < 20% — **NOT MET for code_edit** (60% failure on code_edit tasks)
- [x] All output saved as attributable artifacts — **YES**
- [x] Scoreboard shows honest l1-produced majority — **YES (6/10 v2)**

### Safe to Expand:
- config_change tasks (3/3 success in v2)
- doc_update tasks (2/4 success in v2 — 50%, mixed)
- simple code_edit with small files

### NOT Safe to Expand:
- code_edit on large Python files (>500 lines)
- Multi-file editing tasks
- Tasks requiring deep context understanding

## Separation of Categories

| Category | What It Measures | Where It Goes |
|----------|-----------------|---------------|
| Ops Cleanup | Bulk Jira state changes, stale ticket closure | ops-cleanup log |
| L1 Production | Tasks where L1 produced attributable artifacts | l1-attribution-scoreboard |
| Supervisor Work | Tasks where GLM-5/human did the actual work | supervisor-intervention log |
| Discovery Output | Scanner reports, findings generation | finding-to-ticket flow |

**These are separate. Do not mix them in reporting.**

## Mandatory Statements

- Usefulness claims require attributable L1 artifacts
- GLM-5/script work must not be mislabeled as L1 work
- The factory must be measured honestly before expanding claims
- Backlog cleanup is useful operations work, but it is not factory production
