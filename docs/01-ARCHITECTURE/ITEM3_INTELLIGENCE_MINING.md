# Item #3: Intelligence / ReMe / Memory - Proof-of-Work Mining

## Status: ✅ WIRED INTO RUNTIME (2026-03-10)

## Overview

This document describes the proof-of-work mining system implemented as the first component of Item #3 (Intelligence/ReMe/Memory).

## Architecture

### Components

1. **Miner** (`internal/intelligence/miner.go`)
   - Scans proof-of-work directories for JSON artifacts
   - Extracts patterns from successful and failed executions
   - Aggregates statistics by work type, template, and duration
   - Stores learned patterns in the pattern store

2. **Pattern Store** (`internal/intelligence/pattern_store.go`)
   - `JSONPatternStore`: File-based persistence using JSON
   - `InMemoryPatternStore`: In-memory storage for testing
   - Stores work type statistics, template statistics, and duration percentiles
   - Provides query interface for retrieving patterns

3. **Recommender** (`internal/intelligence/recommender.go`)
   - Uses learned patterns to recommend templates and configurations
   - Calculates confidence scores based on sample count
   - Provides timeout and retry recommendations based on historical performance
   - Offers pattern analysis for insights

## Data Models

### WorkTypeStatistics
```go
type WorkTypeStatistics struct {
    WorkType           string
    WorkDomain         string
    TotalRuns          int
    SuccessfulRuns     int
    SuccessRate        float64
    AverageDuration    time.Duration
    TotalDuration      time.Duration
    TotalFilesChanged  int
    FilesChangedPerRun float64
}
```

### TemplateStatistics
```go
type TemplateStatistics struct {
    TemplateName     string
    TotalRuns       int
    SuccessfulRuns  int
    SuccessRate     float64
    AverageDuration time.Duration
    TotalDuration   time.Duration
}
```

### DurationStatistics
```go
type DurationStatistics struct {
    WorkType       string
    WorkDomain     string
    Samples        []time.Duration
    MinDuration    time.Duration
    MaxDuration    time.Duration
    MedianDuration time.Duration
    P95Duration    time.Duration
    P99Duration    time.Duration
}
```

## Mining Process

1. **Discovery**: Scan `runtimeDir/proof-of-work/` for artifact directories
2. **Extraction**: Parse `proof-of-work.json` files
3. **Aggregation**: Compute statistics by work type and template
4. **Storage**: Persist aggregated patterns to pattern store
5. **Analysis**: Compute duration percentiles (min, max, median, P95, P99)

## Recommendation Engine

### Template Recommendation
- Selects templates based on historical success rate
- Confidence calculated from sample count (0.0 to 1.0)
- Returns `default` template for low-confidence situations

### Configuration Recommendation
- Timeout: P95 duration × 2 (1 min minimum, 1 hour maximum)
- Retries: 3 (default) or 5 (for low-success-rate work types)
- Confidence based on duration sample count

### Pattern Analysis
- Aggregate statistics across all work types
- Top 5 work types by execution volume
- Top 5 templates by execution volume
- Overall success rate and execution count

## Integration Points

### Factory Integration (Wired)
- **Intelligence is wired into runtime**: vertical-slice initializes pattern store at `<ZEN_BRAIN_RUNTIME_DIR>/patterns` (default `/tmp/zen-brain-factory/patterns`), creates MiningIntegration, and sets Factory recommender via `SetRecommender(mining.GetFactoryRecommender())`.
- **Factory consumes recommendations**: When a recommender is configured, `createExecutionPlan()` uses `chooseTemplateAndConfig()` to call `RecommendTemplateWithMetadata` and `RecommendConfiguration`; selected template identity, timeout/retry overrides, and metadata (source, confidence, reasoning) are persisted on the task spec and in proof-of-work.
- **Proof-of-work records actual template**: Artifacts include `template_used`, `selection_source`, `selection_confidence`, `selection_reasoning`; `model_used` is kept for backward compatibility.
- **Mining after execution**: After Factory execution completes, vertical-slice invokes `MineProofOfWorks(ctx)` (warning only on failure), so the loop learns from each run.
- Proof-of-work artifacts include `WorkType` and `WorkDomain`; miner prefers `template_used` when present, with `model_used` fallback for older artifacts.

### KB Integration (Future)
- Pattern store can be integrated with KB stub
- Learned patterns can be stored as documents
- Patterns can be queried alongside KB documents

### Planner Integration (Future)
- Recommender can provide input to template selection
- Configuration recommendations can be injected into task specs
- Historical success rates can inform priority decisions

## Test Coverage

All tests pass (7/7):
- ✅ `TestInMemoryPatternStore` - In-memory store operations
- ✅ `TestRecommender` - Recommendation generation
- ✅ `TestConfidenceCalculation` - Confidence scoring
- ✅ `TestDurationStatistics` - Percentile calculations
- ✅ `TestJSONPatternStore` - File-based persistence
- ✅ `TestMinerWithRealProofOfWorks` - Mining real artifacts
- ✅ `TestFullWorkflow` - End-to-end mining and recommendation

## Usage Example

```go
// Create pattern store
patternStore, _ := NewJSONPatternStore("/var/run/zen-brain/patterns")

// Create miner
miner := NewMiner("/var/run/zen-brain", patternStore)

// Mine proof-of-works
result, err := miner.MineProofOfWorks(ctx)

// Create recommender
recommender := NewRecommender(patternStore, 3) // min 3 samples

// Get recommendations
templateRec, configRec, err := recommender.RecommendAll(
    ctx,
    "implementation",
    "backend",
)

// Get pattern analysis
analysis, err := recommender.PatternAnalysis(ctx)
fmt.Println(analysis.FormatAnalysis())
```

## Example Output

```
Pattern Analysis:
  Work Types: 1
  Templates: 1
  Total Executions: 5
  Success Rate: 80.0%

Top Work Types (by volume):
  1. implementation/backend: 5 runs, 80.0% success, avg 1m0s

Top Templates (by volume):
  1. implementation:real: 5 runs, 80.0% success, avg 1m0s
```

## Next Steps

### Immediate (Priority)
- [x] Integrate mining into Factory's proof-of-work generation pipeline (done: vertical-slice calls MineProofOfWorks after execution)
- [ ] Add scheduled mining (e.g., after every N executions)
- [ ] Add pattern cleanup (remove old/dated patterns)

### Medium Term
- [ ] Enhance pattern extraction (e.g., failure mode analysis)
- [ ] Add pattern versioning and migration
- [x] Implement pattern-based template auto-selection (done: Factory uses recommender when set; compatibility filter ensures only workType/workDomain-matching templates are recommended)

### Long Term
- [ ] Integrate with KB for pattern search and retrieval
- [ ] Build ReMe (Remember Me) for user preference learning
- [ ] Implement intent tracking and improvement loop
- [ ] Add ML-based pattern discovery and prediction

## File Structure

```
internal/intelligence/
├── miner.go           # Proof-of-work mining and pattern extraction
├── pattern_store.go   # Pattern persistence (JSON and in-memory)
├── recommender.go     # Recommendation engine
└── intelligence_test.go # Comprehensive test suite
```

## Performance Considerations

- Mining is fast: < 100μs for 5 artifacts
- Pattern aggregation is O(n) where n is number of artifacts
- Duration percentile calculation uses simple sort (acceptable for small datasets)
- Pattern store operations are thread-safe with mutex locks

## CLI Commands (Block 5)

- `zen-brain intelligence mine` — Load runtime dir + pattern store, run miner; print artifacts found/mined, patterns extracted, failure stats, errors.
- `zen-brain intelligence analyze` — Load pattern store, run `PatternAnalysis()`, print formatted summary.
- `zen-brain intelligence recommend <workType> <workDomain>` — Load pattern store, run `RecommendAll`, print template, confidence, reasoning, timeout, retries.
- `zen-brain intelligence diagnose <workType> <workDomain>` — Print failure statistics summary (total failures, top failure mode, recommended actions, last failure at).
- `zen-brain intelligence checkpoint <sessionID>` — Load structured execution checkpoint via session manager, print summary and details.

Runtime dir from `ZEN_BRAIN_RUNTIME_DIR` (default `/tmp/zen-brain-factory`); pattern store at `<runtimeDir>/patterns`. Exit non-zero on hard failures. Existing commands (`test`, `vertical-slice`, `version`) unchanged.

## Limitations (Current)

1. **Template Tracking**: ✅ Resolved — proof-of-work now includes `template_used`; miner uses it first with `model_used` fallback for backward compatibility.

2. **Failure Analysis**: ✅ Addressed — deterministic `classifyFailure` (test/timeout/workspace/policy/infra/runtime/validation); `FailureStatistics` persisted; `GetFailureStats`/`GetAllFailureStats`; recommender applies failure-aware confidence downgrade when recent failures ≥ 3.

3. **Pattern Versioning**: No schema versioning for stored patterns
   - Future: Add migration support for pattern schema changes

4. **Temporal Decay**: No decay of older patterns
   - Future: Weight recent executions more heavily

5. **Cross-workspace Learning**: Patterns not shared across workspaces
   - Future: Global pattern store with workspace-specific overrides

## Success Criteria

✅ **MVP Complete**: Can mine, store, and recommend based on proof-of-work
✅ **Wired into runtime**: vertical-slice enables recommender, mines after execution; proof-of-work includes template_used
✅ **Recommender compatibility**: RecommendTemplate only returns templates compatible with requested workType/workDomain
✅ **Tests Pass**: Intelligence, factory, and session tests cover miner template preference, recommender filter, proof metadata, execution checkpoint
✅ **Performance**: Mining completes in < 1ms for typical workloads
✅ **Extensible**: Architecture supports future enhancements

**Still missing (out of scope for this patch):** no ML; no temporal decay/recency weighting; no deep causal failure classifier; no cross-cluster/global intelligence service.

## Related Items

- **Item #1**: Vertical slice - provides proof-of-work artifacts to mine
- **Item #2**: More useful templates - provides more diverse patterns to learn
- **Item #4**: Controlled rescue - can use learned patterns for recovery
