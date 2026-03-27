> **NOTE:** This document references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only.

# MLQ Lane Routing Matrix

**Version:** 1.0
**Status:** Production
**Applies to:** zen-brain1 factory execution via LLM

## Routing Decision Table

| Criterion | Level 1 (0.8B) | Level 2 (2B) | Escalate to L3+ |
|-----------|---------------|--------------|-----------------|
| Files touched | 1 file + 1 test | 1–3 files | >3 files |
| Architecture change | None | Minor extension | New hierarchy/protocol |
| New types | None | Extend existing only | Invent new |
| Cross-package work | No | Limited | Yes |
| Context budget | Target + 1 adjacent file | Target + 2–3 files | Broad synthesis |
| Prompt budget | <2000 chars | <5000 chars | Unbounded |
| Target file | Explicit and known | Explicit and known | Unclear/multiple |
| Verification | build/test/lint | build/test/lint | May require manual review |

## L1 Admission Criteria (0.8B workhorse)

A task routes to L1 when ALL of these are true:
- Single file edit-in-place, or file + one test file
- No new architecture, no new type hierarchy
- Target file, package name, and existing symbols are explicit
- Minimal context: target file contents + one adjacent dependency
- Direct verification path exists (build, test, lint)
- Tools available and explicitly mentioned in packet

**Examples:**
- Add validation method to existing struct
- Fix a bounded bug in a single function
- Add/update a unit test
- Small parsing/formatting fix
- Narrow config validation guard

## L2 Admission Criteria (2B stronger lane)

A task routes to L2 when it exceeds L1 but meets ALL of these:
- 1–3 files to touch
- Moderate adaptation or refactor using existing abstractions
- Can tolerate more context than L1
- Still bounded, grounded, and explicit
- Verification path exists

**Examples:**
- Adapt one subsystem across 2–3 existing files
- Medium bug fix with test coverage
- Bounded refactor preserving existing interfaces
- Implement missing check/guard across one package
- Small feature extension using existing interfaces

## Immediate Escalation to L3+

Do NOT route to L1 or L2 if ANY of these are true:
- Architecture invention required
- Many-file refactor (>3 files)
- Target file/package is unclear
- Complex protocol or state-machine change
- Broad repo search/synthesis required
- Prior L1/L2 failure under correct operating conditions

## MLQ Level Mapping

| MLQ Level | Model | Backend | Use Case |
|-----------|-------|---------|----------|
| 0 | qwen3.5:0.8b | Ollama | Fallback (certified) |
| 1 | qwen3.5:0.8b-q4 | llama.cpp | L1 workhorse |
| 2 | 2b-q4 | llama.cpp | L2 bounded implementation |
| 3+ | External model | Cloud/external | Escalation |

## Task Class → Default Level

| Task Class | Default Level | Escalation Trigger |
|-----------|---------------|--------------------|
| `implementation` (bounded, single-file) | L1 | >1 file or architecture needed |
| `implementation` (multi-file, moderate) | L2 | Architecture invention needed |
| `bugfix` (bounded) | L1 | Cross-package or unclear scope |
| `refactor` (single file) | L1 | Multi-file or interface change |
| `refactor` (multi-file) | L2 | Architecture redesign |
| `test` | L1 | Test infrastructure creation |
| `migration` | L2 | Scope >3 files → L3 |
| `documentation` | L1 | — |
