# Ops Department

## Status

**Draft** (2026-03-08)

## Context

Operations teams (Ops) are critical to reducing toil around incidents, changes, deploys, and launches. In modern infrastructure, Ops work involves:

- **Incident response** – diagnosing, mitigating, and resolving production issues
- **Change management** – planning, approving, and executing deployments and config changes
- **Runbook execution** – following documented procedures for routine and emergency tasks
- **Approvals and gates** – ensuring risky actions are reviewed before execution

Zen-Brain 1.0 aims to become a **trusted internal operator** that accelerates Ops work without introducing uncontrolled risk. The system must support Jira-centric Ops workflows with appropriate safety boundaries.

## Decision

**Ops Department is a first-class concept in Zen-Brain, with Jira as the human operating console.**

### Ops Operating Model

#### Jira-Centric Work
- **Primary interface:** Jira as human front door (issues, projects, statuses)
- **Work item types:**
  - **Incident:** Production issue requiring diagnosis and resolution
  - **Problem:** Known issue or bug requiring investigation
  - **Change:** Planned deployment, configuration change, or feature rollout
  - **Task:** Routine maintenance, runbook execution, or follow-up
- **Lifecycle:** Create → Triage → Plan → Execute → Verify → Close

#### Initial Scope: Narrow, Focused
For 1.0, Ops Department operates within:
- **Single Jira space/project** – "Operations" or "Ops" space for initial narrow lane
- **Defined workflows:** Incident, Problem, Change, Task flows
- **Runbook linkage:** Connection to KB (zen-docs + qmd) for documented procedures

#### What Can Be Automated Safely in 1.0
- **Low-risk tasks:**
  - Runbook lookup and retrieval
  - Status updates and progress tracking
  - Log aggregation and summarization
  - Knowledge base search for similar incidents/problems
- **Medium-risk tasks:**
  - Drafting change plans or incident reports
  - Suggesting diagnostic commands or rollback steps
  - Generating runbook variants for testing
- **Requires approval:**
  - Executing production changes
  - Restarting services or modifying config
  - Deleting data or rolling back deployments

### Incident/Problem Workflows

#### Incident Flow
1. **Detection** – Alert or user report creates Jira Incident ticket
2. **Triage** – Zen-Brain assists with:
   - Similar incident search (KB + qmd)
   - Suggested severity and priority
   - Recommended assignee based on expertise tags
3. **Planning** – Zen-Brain assists with:
   - Breakdown into diagnostic steps
   - Suggested runbooks to follow
   - Potential impact assessment
4. **Execution** – Ops executes steps; Zen-Brain tracks progress via Jira comments
5. **Verification** – Zen-Brain monitors for:
   - Error logs or continued alerts
   - Rollback success metrics
6. **Close** – Ops confirms resolution; Zen-Brain generates:
   - Post-incident summary
   - Knowledge update proposal (if new issue type)
   - Follow-up tasks if needed

#### Problem Flow
1. **Investigation** – Ops investigates root cause (RCA)
2. **Analysis** – Zen-Brain assists with:
   - Log aggregation and pattern detection
   - Correlation with similar problems
   - Suggested fixes or workarounds
3. **Solution** – Ops implements fix; Zen-Brain generates:
   - Draft problem statement
   - RCA template
   - Test cases for regression prevention
4. **Closure** – Problem marked resolved; Zen-Brain updates KB

### Change Management Workflows

#### Change Flow
1. **Request** – Ops or engineering submits Change ticket
2. **Risk Assessment** – Zen-Brain assists with:
   - Impact analysis (affected services, data, users)
   - Risk category (low/medium/high/critical)
   - Suggested approval gates based on risk
3. **Approval** – Change approved based on risk level:
   - **Low-risk:** Single approver, can be automated
   - **Medium-risk:** Ops manager approval
   - **High-risk:** Ops director + engineering approval
4. **Planning** – Zen-Brain assists with:
   - Deployment checklist generation
   - Rollback plan drafting
   - Test plan suggestions
5. **Execution** – Ops executes deployment; Zen-Brain:
   - Tracks progress via Jira comments
   - Monitors health checks and logs
   - Triggers alerts if deployment fails
6. **Verification** – Zen-Brain verifies:
   - Services are healthy
   - No new errors in logs
   - Rollback plan not needed
7. **Close** – Change marked successful; Zen-Brain generates:
   - Change summary
   - Knowledge update (deployment runbook variant)
   - Follow-up monitoring tasks

### Runbooks and Knowledge Linkage

#### Runbook Structure
Runbooks are documented procedures in KB (zen-docs + qmd):
- **Title:** Clear, searchable (e.g., "Restart Payment Service Safely")
- **Prerequisites:** Required access, tools, permissions
- **Steps:** Numbered, verified instructions
- **Rollback:** Clear rollback procedure
- **Validation:** How to verify success

#### KB Integration
- **Storage:** zen-docs git repo as source of truth
- **Search:** qmd index for fast retrieval
- **Linkage:** Jira tickets link to relevant runbooks (automatic suggestion)
- **Feedback:** Ops can update runbooks from execution (Zen-Brain drafts suggestions)

### Approval and Gate Hooks

#### Risk-Based Gates
| Risk Level | Approval Required | Automation Level |
|-------------|------------------|------------------|
| **Low** | None or single peer | Can be automated |
| **Medium** | Ops manager | Semi-automated (draft, human approves) |
| **High** | Ops director + engineering | Manual (Zen-Brain suggests, human decides) |
| **Critical** | Executive approval | Manual (no automation) |

#### Gate Triggers
- **Production changes** – deployment, config modification, service restart
- **Data operations** – delete, migrate, export customer data
- **Security actions** – access changes, key rotation, vulnerability patching
- **Network changes** – firewall rules, DNS updates, routing changes

### Safety Boundaries

#### Protected Actions
Zen-Brain cannot execute the following without explicit human approval:
- **Production deployments** – can draft, cannot push
- **Data deletion** – can suggest, cannot execute
- **Service restarts** – can prepare commands, cannot execute
- **Config changes** – can review, cannot apply

#### Read-Only Operations
Zen-Brain can safely perform:
- **Monitoring and alerting** – read logs, check health, detect anomalies
- **Analysis and summarization** – aggregate logs, summarize incidents, detect patterns
- **Knowledge retrieval** – search KB, suggest runbooks, find similar issues
- **Drafting assistance** – create change plans, incident reports, RCAs

#### Sandbox for Testing
For 1.1, consider:
- **Ops Sandbox** – non-destructive environment for testing procedures
- **Runbook validation** – simulate runbook execution before production use
- **Change dry-runs** – practice deployments in sandbox environment

### Integration with Zen-Mesh

#### Launch Support
- Zen-Brain assists with launch monitoring and incident response
- Tracks launch checklist completion
- Monitors for post-launch issues (errors, latency, user reports)

#### Production Support
- Zen-Brain is first responder for common issues (restart, rollback, config change)
- Escalates to human for complex or novel issues
- Learns from resolved issues to improve suggestions

## Consequences

### Positive
- **Reduced toil** – Automated assistance speeds up routine Ops work
- **Consistent processes** – KB-linked runbooks ensure standard procedures
- **Better incident response** – Suggested diagnostics and similar incidents reduce time to resolution
- **Safer changes** – Risk-based gates prevent uncontrolled production changes
- **Knowledge growth** – Every resolved issue/incident can update KB

### Negative
- **Requires Jira integration** – Ops team must use Jira for workflow
- **Initial scope narrow** – Single Jira space/project limits initial usefulness
- **Approval bottleneck** – Risk-based gates may slow down rapid changes
- **KB quality dependency** – Bad runbooks lead to bad suggestions

### Neutral
- **Design remains Jira-centric** – Can add other systems (PagerDuty, Slack) later
- **Safety boundaries explicit** – Clear what Zen-Brain can/cannot do without approval
- **Sandbox path exists** – Ops Sandbox for 1.1 allows testing without production risk

## Alternatives Considered

### Alternative 1: Broad multi-system Ops integration
- **Pros:** More flexible, not Jira-dependent
- **Cons:** Higher complexity, harder to maintain consistency
- **Rejected:** Start narrow (Jira) and expand based on 1.0 experience

### Alternative 2: No approval gates, fully automated
- **Pros:** Faster execution, less toil
- **Cons:** Uncontrolled risk, human in loop still needed for safety
- **Rejected:** Trusted operator requires safety boundaries; Ops work has high risk

### Alternative 3: KB in database instead of Git
- **Pros:** Faster updates, easier search integration
- **Cons:** Requires database infrastructure, not simple for 1.0
- **Rejected:** Git + qmd is simpler, already architectural decision (ADR-0007)

## Related Decisions

- [ADR-0005](../01-ARCHITECTURE/ADR/0005_AI_ATTRIBUTION_JIRA.md) – AI attribution on all Jira content
- [ADR-0007](../01-ARCHITECTURE/ADR/0007_QMD_FOR_KNOWLEDGE_BASE.md) – QMD for KB search
- [Block 2 Office](BLOCK2_OFFICE.md) – Jira connector design
- [Bounded Orchestrator Loop](../../03-DESIGN/BOUNDED_ORCHESTRATOR_LOOP.md) – Approval gates integrate with orchestrator stop conditions

## Future Work

### 1.1+
- **Ops Sandbox** – Non-destructive environment for testing procedures
- **PagerDuty/Slack integration** – Additional alerting and notification channels
- **Auto-healing** – Common restarts and rollbacks automated
- **Predictive alerts** – ML-based anomaly detection for preemptive issue detection
- **Ops metrics dashboard** – Real-time view of Ops health and Zen-Brain contribution

### Multi-Department Expansion
- **Engineering Ops** – CI/CD workflows, build failures, deployment automation
- **Security Ops** – Vulnerability response, access management, compliance audits
- **Platform Ops** – Infrastructure scaling, cost optimization, capacity planning

## References

- Jira ITSM Practices: [https://www.atlassian.com/itsm](https://www.atlassian.com/itsm)
- Incident Management (ITIL): [https://www.axelos.com/itil-incident-management](https://www.axelos.com/itil-incident-management)
- Change Management (ITIL): [https://www.axelos.com/itil-change-management](https://www.axelos.com/itil-change-management)
- KB/QMD Strategy: [../../01-ARCHITECTURE/KB_QMD_STRATEGY.md](../01-ARCHITECTURE/KB_QMD_STRATEGY.md)
- Zen-Brain ROADMAP: [ROADMAP.md](../01-ARCHITECTURE/ROADMAP.md)