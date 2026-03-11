# BLOCK 4 & 5 ROADMAP TO 100%

**Goal**: Reach 100% completeness in Block 4 (Factory) and Block 5 (Intelligence)

**Current Status**:
- Block 4: 95% (after structured proof artifacts)
- Block 5: 97% (after real inference validation)

---

## BLOCK 4: FACTORY (95% → 100%)

### Missing 5% - Concrete Work Items

#### 1. **Static Analysis Integration** (1.5%)
**Status**: Not implemented
**File**: `internal/factory/bounded_executor.go`

**What**:
- Add `staticcheck` step for Go workspaces
- Add `golangci-lint` integration
- Add Python `pylint`/`black` for Python workspaces

**Implementation**:
```go
// In bounded_executor.go, add to step name overrides:
case "staticcheck", "static analysis", "analyze code":
    if hasGoMod(workspacePath) {
        return "staticcheck ./..."
    }
    return "echo 'No go.mod, skipping staticcheck'"
```

**Test**:
```go
func TestBoundedExecutor_StaticCheck(t *testing.T) {
    // Create workspace with go.mod
    // Execute "staticcheck" step
    // Verify real staticcheck runs
}
```

**Effort**: 2-3 hours

---

#### 2. **Cryptographic Proof Signing** (1.5%)
**Status**: Hash-only signing implemented
**File**: `internal/factory/proof.go`

**What**:
- Upgrade from SHA256 hash to RSA/ECDSA signatures
- Add key management (generate, store, rotate)
- Add signature verification

**Implementation**:
```go
// Add to proof.go
type ProofSigner interface {
    Sign(data []byte) ([]byte, error)
    Verify(data, signature []byte) bool
    PublicKey() []byte
}

type RSASigner struct {
    privateKey *rsa.PrivateKey
}

func (s *RSASigner) Sign(data []byte) ([]byte, error) {
    hashed := sha256.Sum256(data)
    return rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hashed[:])
}
```

**Test**:
```go
func TestProofSigning_RSA(t *testing.T) {
    signer := NewRSASigner(2048)
    data := []byte("test proof data")

    sig, err := signer.Sign(data)
    require.NoError(t, err)

    assert.True(t, signer.Verify(data, sig))
}
```

**Effort**: 4-6 hours

---

#### 3. **Multi-Language Support** (1%)
**Status**: Go-only real steps
**Files**: `internal/factory/bounded_executor.go`, `work_templates.go`

**What**:
- Add Python test/build/lint steps
- Add Node.js test/build/lint steps
- Add Rust test/build/lint steps

**Implementation**:
```go
// In bounded_executor.go
case "pytest", "python test":
    if hasRequirementsTxt(workspacePath) || hasPyprojectToml(workspacePath) {
        return "pytest"
    }
    return "echo 'No Python project, skipping pytest'"

case "npm test", "yarn test":
    if hasPackageJson(workspacePath) {
        return "npm test"
    }
    return "echo 'No package.json, skipping npm test'"
```

**Test**:
```go
func TestBoundedExecutor_PythonTest(t *testing.T) {
    // Create workspace with requirements.txt
    // Execute "pytest" step
    // Verify real pytest runs
}
```

**Effort**: 3-4 hours

---

#### 4. **Execution Verification** (1%)
**Status**: Partial
**Files**: `internal/factory/postflight.go`

**What**:
- Add post-execution verification for all work types
- Verify files were actually created/modified
- Verify tests actually ran
- Verify builds succeeded

**Implementation**:
```go
// Add to postflight.go
func (p *PostflightChecker) VerifyExecution(result *ExecutionResult) error {
    // Verify files changed
    for _, file := range result.FilesChanged {
        if _, err := os.Stat(file); os.IsNotExist(err) {
            return fmt.Errorf("file %s was not created", file)
        }
    }

    // Verify tests ran (check for test output)
    if len(result.TestsRun) > 0 && !result.TestsPassed {
        return fmt.Errorf("tests were run but failed")
    }

    // Verify build artifacts exist (for build steps)
    // ...

    return nil
}
```

**Test**:
```go
func TestPostflight_VerifyExecution(t *testing.T) {
    result := &ExecutionResult{
        FilesChanged: []string{"/tmp/test.go"},
        TestsRun: []string{"TestExample"},
        TestsPassed: true,
    }

    err := checker.VerifyExecution(result)
    assert.NoError(t, err)
}
```

**Effort**: 2-3 hours

---

### BLOCK 4 TOTAL EFFORT: **11-16 hours (1.5-2 days)**

---

## BLOCK 5: INTELLIGENCE (97% → 100%)

### Missing 3% - Concrete Work Items

#### 1. **VPA Path Validation** (1%)
**Status**: VPA disabled in sandbox (CRD not available)
**Files**: `deployments/k3d/dependencies.yaml`, test file

**What**:
- Enable VPA in sandbox cluster
- Validate auto-scaling behavior
- Document resource optimization

**Implementation**:
```yaml
# In deployments/k3d/dependencies.yaml, add VPA CRD
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: verticalpodautoscalers.autoscaling.k8s.io
spec:
  group: autoscaling.k8s.io
  # ... VPA CRD spec
```

**Test**:
```go
func TestVPA_AutoScaling(t *testing.T) {
    // Deploy workload with high memory usage
    // Wait for VPA recommendation
    // Verify pod resources were adjusted
    // Verify application remains healthy
}
```

**Effort**: 3-4 hours

---

#### 2. **Intelligence Mining Depth** (1%)
**Status**: Basic mining implemented
**Files**: `internal/intelligence/miner.go`, `recommender.go`

**What**:
- Deeper failure pattern detection
- Root cause analysis from failures
- Correlation between failures and system state
- Predictive failure modeling

**Implementation**:
```go
// Add to miner.go
type FailurePattern struct {
    WorkType       string
    FailureMode    string      // "timeout", "oom", "syntax_error", "test_failure"
    RootCause      string
    Frequency      int
    LastOccurrence time.Time
    Mitigation     string
}

func (m *Miner) AnalyzeFailurePatterns() ([]FailurePattern, error) {
    // Scan failed executions
    // Group by error type
    // Extract root causes
    // Generate mitigation recommendations
}
```

**Test**:
```go
func TestMiner_FailurePatterns(t *testing.T) {
    miner := NewMiner(runtimeDir)

    patterns, err := miner.AnalyzeFailurePatterns()
    require.NoError(t, err)

    assert.NotEmpty(t, patterns)
    for _, p := range patterns {
        assert.NotEmpty(t, p.RootCause)
        assert.NotEmpty(t, p.Mitigation)
    }
}
```

**Effort**: 4-5 hours

---

#### 3. **Model Selection Optimization** (1%)
**Status**: Basic selection with confidence
**Files**: `internal/intelligence/recommender.go`, `internal/planner/model_router.go`

**What**:
- Cost-aware model selection
- Latency-aware model selection
- Quality-aware model selection
- Multi-objective optimization (cost vs quality)

**Implementation**:
```go
// Add to recommender.go
type ModelOptimization struct {
    ModelID          string
    EstimatedCost    float64
    EstimatedLatency time.Duration
    QualityScore     float64
    OverallScore     float64  // Weighted combination
}

func (r *Recommender) OptimizeModelSelection(
    ctx context.Context,
    constraints OptimizationConstraints,
) (*ModelOptimization, error) {
    // Get candidate models
    // Estimate cost/latency/quality for each
    // Compute weighted score
    // Return optimal selection
}

type OptimizationConstraints struct {
    MaxCostUSD     float64
    MaxLatency     time.Duration
    MinQuality     float64
    CostWeight     float64  // 0.0-1.0
    LatencyWeight  float64
    QualityWeight  float64
}
```

**Test**:
```go
func TestRecommender_ModelOptimization(t *testing.T) {
    rec := NewRecommender(patternStore)

    opt, err := rec.OptimizeModelSelection(ctx, OptimizationConstraints{
        MaxCostUSD: 0.05,
        MaxLatency: 30 * time.Second,
        CostWeight: 0.6,
        QualityWeight: 0.4,
    })

    require.NoError(t, err)
    assert.LessOrEqual(t, opt.EstimatedCost, 0.05)
    assert.NotEmpty(t, opt.ModelID)
}
```

**Effort**: 3-4 hours

---

### BLOCK 5 TOTAL EFFORT: **10-13 hours (1.5 days)**

---

## EXECUTION PLAN

### **Day 1 (8 hours)** - Block 4 Focus
1. **Morning (4 hours)**:
   - Static analysis integration (staticcheck, golangci-lint)
   - Multi-language support (Python, Node.js)

2. **Afternoon (4 hours)**:
   - Execution verification
   - Start cryptographic signing

### **Day 2 (8 hours)** - Mixed Focus
1. **Morning (4 hours)**:
   - Finish cryptographic signing
   - VPA path validation

2. **Afternoon (4 hours)**:
   - Intelligence mining depth
   - Model selection optimization

### **Day 3 (4-6 hours)** - Polish & Documentation
1. **Test coverage** (all new features)
2. **Documentation updates**
3. **Completeness matrix update**

---

## TOTAL EFFORT SUMMARY

| Block | Missing % | Work Items | Effort | Priority |
|-------|-----------|------------|--------|----------|
| **4** | 5% | 4 items | 11-16h | High |
| **5** | 3% | 3 items | 10-13h | High |
| **Total** | **8%** | **7 items** | **21-29h** | - |

**Timeline**: **3 days** (with testing and documentation)

---

## SUCCESS CRITERIA

### Block 4 → 100%
- ✅ Static analysis runs for all Go workspaces
- ✅ Multi-language support (Python, Node.js)
- ✅ Cryptographic proof signing (RSA/ECDSA)
- ✅ Execution verification passes for all work types

### Block 5 → 100%
- ✅ VPA enabled and validated in sandbox
- ✅ Failure pattern analysis extracts root causes
- ✅ Model selection optimizes for cost/latency/quality

### Overall
- ✅ All 7 work items completed
- ✅ Test coverage >80% for new features
- ✅ Documentation updated
- ✅ COMPLETENESS_MATRIX.md shows 100%

---

## ALTERNATIVE: MINIMUM VIABLE 100%

If time-constrained, achieve **99%** in 1-2 days by prioritizing:

### Block 4 (95% → 99%)
1. **Static analysis** (1.5%) - HIGH impact
2. **Execution verification** (1%) - HIGH impact
3. **Multi-language support** (1.5%) - MEDIUM impact
4. ~~Cryptographic signing~~ - DEFER to post-1.0

### Block 5 (97% → 99%)
1. **Intelligence mining depth** (1%) - HIGH impact
2. **Model selection optimization** (1%) - MEDIUM impact
3. ~~VPA validation~~ - DEFER to post-1.0

**Effort**: **12-16 hours (1.5-2 days)**
**Result**: Block 4 @ 99%, Block 5 @ 99%, Overall @ 98%

---

## DECISION POINT

**Option A**: Full 100% (21-29 hours, 3 days)
- Complete all features
- Cryptographic signing
- VPA validation
- Maximum production confidence

**Option B**: Pragmatic 99% (12-16 hours, 1.5-2 days)
- Core features only
- Defer crypto signing and VPA
- Faster to completion
- Still production-ready

**Option C**: Ship Now @ 96%
- Current state is production-ready
- Real inference validated
- Structured proofs complete
- Defer all to post-1.0

**Recommendation**: **Option B (Pragmatic 99%)**
- Best ROI on effort
- Core features that matter
- Can reach in 2 days
- Remaining 1% can be post-1.0 polish
