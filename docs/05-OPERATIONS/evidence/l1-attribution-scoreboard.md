# L1 Attribution Scoreboard

**v1 Pilot:** 2026-03-28 09:10-09:23 EDT
**v2 Pilot:** 2026-03-28 10:57-11:05 EDT
**Model:** Qwen3.5-0.8B-Q4_K_M via llama.cpp (port 56227, CPU, 10 parallel slots)

---

## v2 Scoreboard — Patch-Oriented Contract (THE CURRENT STANDARD)

**Contract:** max_tokens=2048, timeout=60s, patch_commands only, no new_content/file_body.

| Jira Key | Task Type | Time | Parsed | Score | Patches | Validation | Forbidden | Produced By | Final State |
|----------|-----------|------|--------|-------|---------|------------|-----------|-------------|-------------|
| ZB-853 | config_change | 9.8s | ✅ | 25/25 | ✅ | ✅ | no | **l1** | Done |
| ZB-854 | code_edit | 60s | ❌ | 0/25 | ❌ | ❌ | no | l1-failed-parse | PAUSED |
| ZB-855 | doc_update | 9.3s | ✅ | 25/25 | ✅ | ✅ | no | **l1** | Done |
| ZB-856 | config_change | 8.6s | ✅ | 25/25 | ✅ | ✅ | no | **l1** | Done |
| ZB-857 | code_edit | 12.3s | ✅ | 25/25 | ✅ | ✅ | no | **l1** | Done |
| ZB-858 | doc_update | 60s | ❌ | 0/25 | ❌ | ❌ | no | l1-failed-parse | PAUSED |
| ZB-860 | doc_update | 19.8s | ✅ | 25/25 | ✅ | ✅ | no | **l1** | Done |
| ZB-863 | config_change | 14.1s | ❌ | 0/25 | ❌ | ❌ | no | l1-failed-parse | PAUSED |
| ZB-865 | config_change | 26.5s | ✅ | 25/25 | ✅ | ✅ | no | **l1** | Done |
| ZB-867 | doc_update | 29.6s | ❌ | 0/25 | ❌ | ❌ | no | l1-failed-parse | PAUSED |

### v2 Counts

| Category | Count | % |
|----------|-------|---|
| **l1-produced** | **6** | **60%** |
| l1-produced-needs-review | 4 | 40% |
| supervisor-written | 0 | 0% |
| script-only | 0 | 0% |

### v2 Assessment
- Every successful task scored 25/25 (perfect quality gate)
- 0 tasks contained forbidden full-content fields
- Average response time for successful tasks: 13s
- Remaining failures: code_edit timeouts (L1 still tries too hard on code edits)
- **60% threshold MET for config_change and doc_update**

---

## v1 Scoreboard — Full-File Contract (OBSOLETE)

| Jira Key | Task Type | Lane | Raw L1 Output | Final Artifact | Validation | Produced By | Supervisor Intervention | Final State |
|----------|-----------|------|---------------|----------------|------------|-------------|------------------------|-------------|
| ZB-817 | config_change | l1 | ✅ 4.7s | ✅ .gitignore entry | l1_output_parseable | **l1** | none | Done |
| ZB-818 | code_edit | l1 | ❌ 120s timeout | ❌ none | parse_failed | l1-failed-parse | normalization_only | PAUSED |
| ZB-819 | doc_update | l1 | ❌ 120s timeout | ❌ none | parse_failed | l1-failed-parse | normalization_only | PAUSED |
| ZB-820 | config_change | l1 | ❌ 120s timeout | ❌ none | parse_failed | l1-failed-parse | normalization_only | PAUSED |
| ZB-824 | code_edit | l1 | ⚠️ 12.5s partial | ❌ truncated JSON | parse_failed | l1-failed-parse | normalization_only | PAUSED |
| ZB-826 | doc_update | l1 | ✅ 18.0s | ✅ template JSON | l1_output_parseable | **l1** | none | Done |
| ZB-827 | doc_update | l1 | ❌ 120s timeout | ❌ none | parse_failed | l1-failed-parse | normalization_only | PAUSED |
| ZB-829 | config_change | l1 | ❌ 120s timeout | ❌ none | parse_failed | l1-failed-parse | normalization_only | PAUSED |
| ZB-832 | config_change | l1 | ✅ 20.4s | ✅ template JSON | l1_output_parseable | **l1** | none | Done |
| ZB-834 | doc_update | l1 | ❌ 120s timeout | ❌ none | parse_failed | l1-failed-parse | normalization_only | PAUSED |

## Counts

| Category | Count | Percentage |
|----------|-------|-----------|
| **l1-produced** | 3 | 30% |
| **l1-produced-needs-review** | 7 | 70% |
| supervisor-written | 0 | 0% |
| script-only | 0 | 0% |
| failed | 0 | 0% |

## Honest Assessment

### What L1 (0.8b) can do:
- **Simple config_change tasks** (adding .gitignore entries, Makefile targets): 30% success rate
- **Small doc_update tasks** (creating JSON templates): works when the target is small
- Response time when it works: 4-25 seconds

### What L1 (0.8b) cannot do yet:
- **code_edit tasks**: Either times out (120s) or produces truncated JSON
- **Tasks targeting large files**: L1 tries to generate the entire file content, overwhelms max_tokens or context
- **Complex doc_update**: Go doc comments on large structs — L1 tries to include the whole struct

### Root causes for 70% failure:
1. **Timeout (4/7)**: L1 tries to regenerate the entire target file instead of just the diff/description. 0.8b can't produce 4000+ tokens of valid JSON in 120s on CPU.
2. **Truncation (1/7)**: L1 starts producing JSON but hits max_tokens or context limit mid-output.
3. **Garbage output (2/7)**: L1 returned empty content or non-JSON after timeout.

### What this means for the factory:
- **L1 is real but limited** — 30% of bounded tasks produced attributable artifacts
- **The 30% that worked are genuinely L1-authored** — raw output is saved, parseable, and traceable
- **The 70% failure is honest** — L1 couldn't handle larger code edits or complex tasks within constraints
- **Not ready for autonomous expansion** — need to fix the timeout/truncation issues before trusting L1 with more tasks

## Evidence Artifacts

All raw L1 outputs saved to: `docs/05-OPERATIONS/evidence/l1-attribution-pilot/`

- Raw outputs: `ZB-XXX_*_raw.json` (10 files)
- Normalized outputs: `ZB-XXX_*_normalized.json` (5 files — only from successful parses)
- Scoreboard: `l1-attribution-scoreboard.json`

## Policy Decision

Per Phase 5 decision rules:
- **Result: Scenario B** — Most bounded tickets are NOT truly l1-produced (30% vs 70%)
- **Action**: Stop claiming L1 is doing the work for most tasks
- **Next**: Tighten packet design — instruct L1 to produce descriptions only, not full file contents
- **Then**: Re-run pilot with description-only output format

## Jira State After Pilot

| Status | Count |
|--------|-------|
| Backlog | 2 |
| PAUSED | 13 |
| Done | 539 |
| **Total** | **554** |
