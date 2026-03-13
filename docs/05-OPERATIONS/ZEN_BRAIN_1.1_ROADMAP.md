# Zen-Brain 1.1 Roadmap

## Executive Summary

Zen-Brain 1.0 is now good enough to be used in a controlled way. Zen-Brain 1.1 should not be "more features everywhere."

**1.1 Theme:** Turn Zen-Brain from a working single-lane operator into a multi-worker, RBAC-aware, secret-safe, night-shift factory.

## Guiding Principles

- Safer
- More visible
- Better coordinated
- Better calibrated
- Better at boring nightly work

## Priority Order

### Tier 1 — Must Wire Early

1. **Secret Plane with Zen-Lock**
   - Store Jira / Redis / Cockroach / cloud creds as ZenLock resources
   - Bind access by service account using AllowedSubjects and RBAC
   - Keep etcd encryption-at-rest enabled
   - No broad shared credential access

2. **Worker Identity and Claim Model**
   - Stable `worker_id`
   - Task claim / lease / expiration
   - Ownership visible in Jira, artifacts, and logs
   - Prepare for future same-project multi-instance work

3. **Night-Shift Task Taxonomy**
   - Create recurring safe task categories
   - Start with at least 30 low-risk tasks
   - Keep night work non-critical and measurable

4. **Factory Specialization**
   - Separate planner, triager, hunter, reporter, executor
   - Do not let every worker do every task type

5. **Skills / Calibration / Training**
   - Start with skills + evals + confidence scoring
   - Use 0.8b MoE for specialist lanes first
   - Postpone broader training until evaluation and routing are solid

6. **Governance / Compliance Lane** (NEW)
   - Compliance Reporter: Generate SR&ED/IRAP/ISO/SOC evidence reports
   - Compliance Gap Hunter: Detect missing controls, weak evidence, documentation gaps
   - Read-only, summarization-heavy, artifact/report producing
   - Safe night-shift workers with useful morning outputs

### Tier 2 — High ROI Next

- Real KB/qmd
- Queue scoring/prioritization
- Confidence and calibration layer
- Nightly report synthesizer
- Worker health/heartbeat metrics
- Auto-stop / safety brakes on noisy behavior

### Tier 3 — After That

- Real ledger
- Deploy credentials with strict gating
- Cloud-ops lanes
- Controlled deployment workflows
- Dual-instance same-project coordination

---

## 1. Secret and Access Plane

### Design Rule
No generic "brain can read all creds." Instead:

- `zen-brain-planner` SA → Jira read/write, maybe no cloud creds
- `zen-brain-reporter` SA → Jira comment/artifact only
- `zen-brain-operator` SA → selective infra read
- `zen-brain-deployer` SA → deploy-only creds, tightly gated
- `zen-brain-nightshift` SA → safe non-critical access only

This makes operations visible and auditable.

### What 1.1 Should Do

- Use Zen-Lock for static operational credentials:
  - Jira
  - CockroachDB
  - Redis
  - GitHub/Git provider
  - AWS
  - GCP
  - Any deploy/API tokens
- Target model:
  - Secrets stored encrypted in Git as ZenLock resources
  - Decrypted only at runtime
  - Ephemeral Kubernetes Secrets only when needed
  - Access restricted by service account identity
  - etcd encryption-at-rest enabled
  - Namespace and role boundaries enforced with RBAC

---

## 2. Worker Identity and Claim/Lease Model

If you want multiple AIs later, this becomes mandatory.

### Each Worker Should Have

- Stable instance identity
- Service account identity
- Explicit task claim
- Heartbeat / lease ownership
- Visible in Jira/comments/artifacts/logs

### Minimum Objects to Add

- `worker_id`
- `role`
- `claimed_at`
- `lease_expires_at`
- `approved_by`
- `source_project`
- `action_class`

Without that, two instances on the same project will collide.

---

## 3. Night-Shift Task System

1.1 should formalize a recurring low-risk task catalog so workers stay useful overnight without improvising dangerous work.

### Good Nightly Categories

- Stub hunting
- Bug hunting
- Hardcoded hunting
- Weak-test hunting
- Flaky-path hunting
- Report generation
- Doc/runbook gap detection
- Proof quality review
- Config drift detection
- Action-policy drift detection

---

## 4. Factory Specialization

Do not make every worker do everything. Your future factory should prefer narrow specialists over generalist chaos.

### Suggested Worker Roles

1. **Triage**: selects and classifies tasks
2. **Planner**: turns findings into ordered work
3. **Stub Hunter**: finds TODO/FIXME/placeholder stubs
4. **Bug Hunter**: finds common bug patterns
5. **Hardcode Hunter**: finds hardcoded paths/URLs
6. **Test Hardener**: finds weak/flaky tests
7. **Metrics Gardener**: collects and reports metrics
8. **Proof Formatter**: formats and validates proof artifacts
9. **Runbook Curator**: updates and validates runbooks
10. **Execution Worker**: executes pre-approved safe tasks
11. **Approval Coordinator**: manages approval workflow
12. **Auditor**: reviews action logs and compliance
13. **Compliance Reporter**: generates SR&ED/IRAP/ISO/SOC evidence reports
14. **Compliance Gap Hunter**: detects missing controls, weak evidence, documentation gaps

This is much more efficient than one agent trying to reason and execute across all task types.

---

## 5. Skills, Calibration, and Training

For 1.1, I would not start with "train everything." I would do this order:

### First

- Task-specific skills
- Evaluation harness
- Confidence scoring
- Routing rules
- Failure taxonomy

### Then

- Lightweight finetuning / adapters / prompt packs

### Only Later

- Broader training

### 0.8b MoE Usage

For your 0.8b MoE idea, 1.1 should first make it a specialist lane, not a general-purpose brain.

**Best first uses:**
- Classifier
- Triager
- Report formatter
- Simple detector lanes
- Low-cost overnight workers

Not top-level planner yet.

---

## 30 Concrete Night-Shift Tasks

### Compliance / Funding (NEW - 15 tasks)

**Evidence Production (5 tasks):**
1. Generate weekly SR&ED readiness summary
2. Generate IRAP-ready milestone evidence
3. Map controls to SOC 2 categories
4. Map controls to ISO 27001 domains
5. Generate R&D activity log summary

**Gap Detection (10 tasks):**
6. Detect missing control evidence
7. Detect evidence freshness issues
8. Detect undocumented operational practices
9. Detect missing change-management artifacts
10. Detect missing incident/problem-management evidence
11. Detect weak design/experiment/proof artifacts
12. Detect stale control evidence
13. Detect missing control ownership
14. Detect undocumented operational practices
15. Detect missing change/incident documentation

### Code Quality / Risk Hunting (existing tasks)

**Runtime / Ops Quality**

1. Stub hunting
2. Mock fallback hunting
3. Hardcoded path hunting
4. Localhost/default URL hunting
5. Secret-in-code hunting
6. Nil-safety hunting
7. Panic-path hunting
8. TODO/FIXME hunting
9. Untested switch/default hunting
10. Dead-code hunting

### Runtime / Ops Quality

11. Runtime doctor clarity improvements
12. Runtime report noise reduction
13. Runtime ping failure classification
14. Config validation gap detection
15. Environment-variable drift detection
16. Action-policy drift detection
17. Worker lease expiry validation
18. Retry-loop abuse detection
19. Long-running command timeout review
20. Proof artifact completeness review

### Factory / Execution Quality

21. Template placeholder hunting
22. Unsafe shell command hunting
23. Repo-write guardrail verification
24. Proof-of-work formatting improvements
25. Postflight warning classification
26. Task-claim collision detection
27. Failed-task deduplication logic review
28. Action-class routing verification

### Testing / Validation

29. Missing regression test suggestions
30. Flaky-test candidate detection

---

## Recommended 1.1 Worker Structure

Minimal efficient factory - start with just 5 roles:

1. **Planner**
   - Reviews overnight outputs
   - Chooses priorities
   - Does not execute risky work

2. **Night-shift Triager**
   - Pulls safe tasks
   - Classifies and routes them
   - Never deploys

3. **Hunter Worker**
   - Stub/bug/hardcode/mock/default/TODO hunting
   - Produces findings only

4. **Reporter Worker**
   - Nightly reports
   - Proof formatting
   - Jira safe comments/artifacts

5. **Executor Worker**
   - Only for pre-approved low-risk work classes
   - Can create patch plans or safe edits
   - No deploys initially

That is enough to create a real overnight software factory.

---

## Secret/RBAC Model for 1.1

### Secret Groups

**Group A — Low-Risk External Integration**
- Jira
- Git provider read-only
- Reporting APIs

**Group B — Internal State Systems**
- Redis
- CockroachDB
- qmd access

**Group C — High-Risk Infra**
- AWS
- GCP
- Deploy credentials
- Cluster-admin-like tokens

### Policy

- Group A can be given to reporter/triager/planner roles as needed
- Group B only to operator/executor roles
- Group C only to tightly approved deployer roles

### Service Accounts

At minimum:
- `zb-planner-sa`
- `zb-nightshift-sa`
- `zb-reporter-sa`
- `zb-operator-sa`
- `zb-deployer-sa`

### Visibility

Every Jira comment / attachment / status change should include:
- Worker identity
- Action class
- Whether approval was required
- Evidence/proof reference

That will make AI activity visible "over there," exactly as you want.

---

## What I Would Do Next, Concretely

### This Week

- Keep running Night Shift on safe Zen-Brain tasks
- Define worker roles and task taxonomy
- Wire Zen-Lock for Jira + Redis + Cockroach credentials
- Define service accounts and RBAC boundaries
- Add worker identity + claim/lease model

### Next Week

- Add real KB/qmd
- Improve planner from overnight outputs
- Add calibration/confidence tracking
- Start second instance on another project

### Only Later

- Test same-project multi-instance mode

---

## The Most Important 1.1 Rule

More autonomy without better identity, claims, RBAC, and secrets will create confusion faster than value.

So 1.1 should first improve:
- Who can do what
- Which worker owns what
- Which secrets each worker can use
- How safe work gets selected overnight
- How confidence is measured

---

## Recommended Next Handoff to AI

Zen-Brain 1.1 should focus on five things in this order:

1. **Secret plane with Zen-Lock**
   - Store Jira / Redis / Cockroach / cloud creds as ZenLock resources
   - Bind access by service account using AllowedSubjects and RBAC
   - Keep etcd encryption-at-rest enabled
   - No broad shared credential access

2. **Worker identity and claim model**
   - Stable worker_id
   - Task claim / lease / expiration
   - Ownership visible in Jira, artifacts, and logs
   - Prepare for future same-project multi-instance work

3. **Night-shift task taxonomy**
   - Create recurring safe task categories
   - Start with at least 30 low-risk tasks
   - Keep night work non-critical and measurable

4. **Factory specialization**
   - Separate planner, triager, hunter, reporter, executor
   - Do not let every worker do every task type

5. **Skills / calibration / training**
   - Start with skills + evals + confidence scoring
   - Use 0.8b MoE for specialist lanes first
   - Postpone broader training until evaluation and routing are solid
