# Zen-Brain 1.1 Action Plan

## This Week (Mar 13-17)

### Keep Night Shift Running
- [ ] Continue running Night Shift on safe Zen-Brain tasks
- [ ] Monitor action policy enforcement
- [ ] Track proof-of-work generation
- [ ] Validate Jira write-back (Class B actions)

### Define Worker Roles and Task Taxonomy
- [ ] Create worker role definitions (5 initial roles)
- [ ] Map 30 night-shift tasks to appropriate worker roles
- [ ] Define task priority system (Class A/B/C)
- [ ] Create task execution templates

### Wire Zen-Lock Integration
- [ ] Audit current credential usage (Jira, Redis, Cockroach)
- [ ] Create ZenLock resource manifests for each credential type
- [ ] Define secret groups (A/B/C)
- [ ] Create service account definitions
- [ ] Design RBAC role bindings

### Define Service Accounts and RBAC Boundaries
- [ ] `zb-planner-sa` - read/write Jira, no cloud creds
- [ ] `zb-nightshift-sa` - safe non-critical access only
- [ ] `zb-reporter-sa` - Jira comment/artifact only
- [ ] `zb-operator-sa` - selective infra read
- [ ] `zb-deployer-sa` - deploy-only creds, tightly gated

### Add Worker Identity + Claim/Lease Model
- [ ] Design worker identity structure (worker_id, role, SA)
- [ ] Implement task claim mechanism
- [ ] Add lease expiration logic
- [ ] Update Jira integration to include worker identity
- [ ] Add worker identity to proof artifacts

---

## Next Week (Mar 18-22)

### Add Real KB/qmd
- [ ] Wire qmd integration for Zen-Mesh context
- [ ] Create KB ingestion pipeline
- [ ] Test KB retrieval in analysis
- [ ] Validate KB stub fallback

### Improve Planner from Overnight Outputs
- [ ] Create output synthesis from multiple workers
- [ ] Implement priority scoring
- [ ] Add conflict resolution (task collisions)
- [ ] Create daily summary report format

### Add Calibration/Confidence Tracking
- [ ] Implement confidence scoring per task type
- [ ] Create evaluation harness
- [ ] Track worker success rates
- [ ] Build confidence calibration dashboard

### Start Second Instance on Another Project
- [ ] Deploy second zen-brain instance
- [ ] Configure separate service account
- [ ] Test multi-instance coordination
- [ ] Validate claim/lease model with 2 workers

---

## Tier 1 Status Tracker

### 1. Secret Plane with Zen-Lock
- [ ] Audit existing credential usage
- [ ] Create ZenLock manifests for:
  - [ ] Jira (Group A)
  - [ ] Redis (Group B)
  - [ ] CockroachDB (Group B)
  - [ ] GitHub/Git provider (Group A)
  - [ ] AWS (Group C - later)
  - [ ] GCP (Group C - later)
- [ ] Implement runtime secret decryption
- [ ] Configure etcd encryption-at-rest
- [ ] Define namespace boundaries

### 2. Worker Identity and Claim Model
- [ ] Design worker identity data structure
- [ ] Implement task claim API
- [ ] Add lease expiration logic
- [ ] Update Jira integration with worker metadata
- [ ] Add worker ID to proof artifacts
- [ ] Implement collision detection

### 3. Night-Shift Task Taxonomy
- [ ] Create task type definitions (30 tasks)
- [ ] Map tasks to worker roles
- [ ] Define action class per task type
- [ ] Create task execution templates
- [ ] Implement task queue

### 4. Factory Specialization
- [ ] Define worker role responsibilities
- [ ] Create role-based routing logic
- [ ] Implement task assignment rules
- [ ] Add worker health monitoring
- [ ] Create worker capability registry

### 5. Skills / Calibration / Training
- [ ] Define evaluation metrics
- [ ] Create confidence scoring system
- [ ] Implement failure taxonomy
- [ ] Build routing rule engine
- [ ] Create 0.8b MoE specialist lanes

---

## Night-Shift Task Catalog (30 Tasks)

### Code Quality / Risk Hunting (10 tasks)
- [ ] Stub hunting
- [ ] Mock fallback hunting
- [ ] Hardcoded path hunting
- [ ] Localhost/default URL hunting
- [ ] Secret-in-code hunting
- [ ] Nil-safety hunting
- [ ] Panic-path hunting
- [ ] TODO/FIXME hunting
- [ ] Untested switch/default hunting
- [ ] Dead-code hunting

### Runtime / Ops Quality (10 tasks)
- [ ] Runtime doctor clarity improvements
- [ ] Runtime report noise reduction
- [ ] Runtime ping failure classification
- [ ] Config validation gap detection
- [ ] Environment-variable drift detection
- [ ] Action-policy drift detection
- [ ] Worker lease expiry validation
- [ ] Retry-loop abuse detection
- [ ] Long-running command timeout review
- [ ] Proof artifact completeness review

### Factory / Execution Quality (8 tasks)
- [ ] Template placeholder hunting
- [ ] Unsafe shell command hunting
- [ ] Repo-write guardrail verification
- [ ] Proof-of-work formatting improvements
- [ ] Postflight warning classification
- [ ] Task-claim collision detection
- [ ] Failed-task deduplication logic review
- [ ] Action-class routing verification

### Testing / Validation (2 tasks)
- [ ] Missing regression test suggestions
- [ ] Flaky-test candidate detection

---

## Worker Role Definitions

### 1. Planner (zb-planner-sa)
**Purpose:** Reviews overnight outputs, chooses priorities, does not execute risky work

**Capabilities:**
- Read Jira (all projects)
- Write Jira comments (status updates only)
- Read KB/qmd
- No cloud credentials
- No deploy access

**Action Classes:** A, B (limited)

### 2. Night-Shift Triager (zb-nightshift-sa)
**Purpose:** Pulls safe tasks, classifies and routes them, never deploys

**Capabilities:**
- Read Jira (unassigned tickets only)
- Read KB/qmd
- Write Jira comments (findings only)
- No deploy access

**Action Classes:** A, B (findings only)

### 3. Hunter Worker (zb-operator-sa)
**Purpose:** Stub/bug/hardcode/mock/default/TODO hunting, produces findings only

**Capabilities:**
- Read codebase
- Read KB/qmd
- Write Jira comments (bug reports only)
- Read Redis (cache)
- Read CockroachDB (audit only)

**Action Classes:** A, B (bug reports only)

### 4. Reporter Worker (zb-reporter-sa)
**Purpose:** Nightly reports, proof formatting, Jira safe comments/artifacts

**Capabilities:**
- Read KB/qmd
- Read proof artifacts
- Write Jira comments/attachments (reports only)
- Read Redis (metrics)

**Action Classes:** A, B (reports only)

### 5. Executor Worker (zb-deployer-sa)
**Purpose:** Only for pre-approved low-risk work classes, can create patch plans or safe edits, no deploys initially

**Capabilities:**
- Read codebase
- Write code (Class B only)
- Read KB/qmd
- No deploy credentials (Tier 3)

**Action Classes:** A, B (patch plans, safe edits only)

---

## Secret Groups Definition

### Group A — Low-Risk External Integration
- Jira (read/write for planner, read-only for others)
- Git provider (read-only)
- Reporting APIs (read-only)

**Allowed Roles:** planner, reporter, triager

### Group B — Internal State Systems
- Redis (cache/metrics)
- CockroachDB (audit logs)
- qmd (KB access)

**Allowed Roles:** operator, executor

### Group C — High-Risk Infra (Tier 3)
- AWS (deploy credentials)
- GCP (deploy credentials)
- Cluster-admin tokens

**Allowed Roles:** deployer (tightly approved only)

---

## Next Immediate Steps (This Week Priority)

1. **Document current credential usage**
   - Where is Jira token used?
   - Where is Redis connection?
   - Where is CockroachDB connection?

2. **Create worker identity structure**
   - Define Go struct for worker metadata
   - Add to action policy
   - Add to proof artifacts

3. **Implement task claim mechanism**
   - Claim endpoint/API
   - Lease expiration logic
   - Collision detection

4. **Start Night Shift on concrete tasks**
   - Pick 5 tasks from the 30-task catalog
   - Run them overnight
   - Collect results and metrics
