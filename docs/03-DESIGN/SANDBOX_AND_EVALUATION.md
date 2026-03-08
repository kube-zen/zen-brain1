# Agent Sandbox and Evaluation

## Status

**Draft** (2026-03-08) – **1.1 Radar Item**

## Context

As Zen-Brain becomes more capable, the risk of **uncontrolled autonomous behavior** increases. Direct AI agents with external write access can cause production incidents, data corruption, or security issues even when well-intentioned.

The **agent sandbox** is a safety feature that provides:
1. A **non-destructive evaluation lane** for testing agent behavior
2. A controlled environment where agents can plan, explore, and simulate without making real changes
3. Metrics and observability for comparing behavior, effectiveness, and safety

This is inspired by experience with "uncontrolled dogfooding" in earlier systems, where agents created cascading failures through recursive, unbounded action.

## Decision

**Agent Sandbox is a 1.1 radar item – a non-destructive evaluation lane for safe agent experimentation.**

### Sandbox Purpose

The sandbox is **not a production tool** – it is a testing and evaluation environment where:

- **Planning is allowed** – Agents can think, reason, and generate plans
- **Exploration is allowed** – Agents can simulate workflows, test hypotheses, and explore options
- **Execution is simulated** – Changes are recorded but not applied to real systems
- **Behavior is measured** – Metrics capture how agents operate, what tools they use, how they respond to constraints

### What Agents Can Do in Sandbox

#### Allowed Operations
- **Read operations** – Fetch data, read code, analyze logs, search KB
- **Planning** – Generate multi-step plans, design solutions, create runbook variants
- **Simulation** – Run what-if scenarios, predict outcomes, test logic
- **Metrics collection** – Log decisions, tool calls, reasoning steps, time spent

#### Non-Destructive Execution
- **Drafting** – Create change plans, incident reports, code changes (as drafts)
- **Mock execution** – Simulate deployment steps without applying changes
- **Dry-run validation** – Test procedures against constraints and policies
- **Evidence generation** – Create proof-of-work bundles without external writes

#### Promotions and Escalation
- **Review workflow** – Sandbox output is reviewed by human before promotion
- **Selective promotion** – Only approved actions are promoted to production
- **Escalation on failure** – If agent cannot complete task sandbox, escalate to human
- **Audit trail** – All sandbox actions are logged for post-mortem analysis

### Sandbox Architecture

#### Environment Isolation
- **Isolated workspace** – Separate worktree or directory, not shared with production
- **No external write access** – Jira, Git, databases, APIs are mocked or read-only
- **Resource limits** – Memory, CPU, and token quotas to prevent runaway
- **Time limits** – Maximum execution time before automatic stop

#### Mocked External Systems
- **Jira sandbox** – Mock Jira API for creating tickets, comments, status updates
- **Git sandbox** – Mock Git operations (clone, branch, commit) without real repository
- **DB sandbox** – In-memory or ephemeral database for testing queries and migrations
- **API mocks** – Mock external service responses for testing integration logic

#### Evaluation and Metrics
- **Behavior tracking** – What tools are called, in what order, with what parameters
- **Effectiveness metrics** – Task completion rate, time to completion, quality of output
- **Policy adherence** – How often agents violate constraints or exceed authority
- **Escalation rate** – How often agents give up and require human help

### Use Cases

#### 1. Testing New Roles and Prompts
- **Scenario:** Engineer wants to test a new "Database Migration" role
- **Sandbox workflow:**
  - Agent receives sample migration task
  - Agent plans migration steps (read schema, generate migration script, test queries)
  - Sandbox validates plan against policies (no data loss, rollback available)
  - Metrics captured: planning quality, tool selection, time spent
- **Outcome:** Engineer reviews plan, approves role, promotes to production

#### 2. Evaluating Model Choices
- **Scenario:** Ops team comparing Qwen 0.8B vs GPT-4 for incident response
- **Sandbox workflow:**
  - Same incident task run with both models
  - Sandbox tracks: time to resolve, accuracy, tool usage, reasoning depth
  - Comparison report generated (model A vs model B)
- **Outcome:** Data-driven decision on which model to use for production

#### 3. Validating Policy Constraints
- **Scenario:** Security team testing if "delete data" is properly blocked
- **Sandbox workflow:**
  - Agent receives "delete customer data" task
  - Agent attempts deletion (blocked by sandbox policy)
  - Agent escalates to human or requests alternative (anonymization instead)
  - Metrics: policy violations, escalation behavior
- **Outcome:** Confirmation that safety gates work; policy refined if needed

#### 4. Experimenting with Tools and Workflows
- **Scenario:** Engineering team testing new "GitHub PR automation" tool
- **Sandbox workflow:**
  - Agent uses mock GitHub API to create PR, run CI, merge
  - Sandbox validates: no force-pushes, CI passes, review completed
  - Metrics: tool usage, success rate, error handling
- **Outcome:** Tool refined, approved, promoted to production use

### Behavior and Effectiveness Metrics

#### Core Metrics
| Metric | Description | Target |
|--------|-------------|--------|
| **Completion rate** | % of tasks completed without escalation | >80% for bounded tasks |
| **Time to completion** | Average time from task start to completion | Within 2x human baseline |
| **Tool accuracy** | % of tool calls that are correct/relevant | >90% |
| **Policy adherence** | % of actions within policy constraints | >95% |
| **Escalation rate** | % of tasks that require human intervention | <20% for simple tasks |

#### Advanced Metrics
- **Reasoning depth** – How many recursive steps does agent take?
- **Self-correction rate** – How often does agent fix its own mistakes?
- **Hallucination detection** – Rate of fabricated information or tool results
- **Context utilization** – How effectively does agent use provided context?
- **Determinism** – How often does agent produce different results for same input?

### Comparison and A/B Testing

#### Model Comparison
- Run same task with multiple models (e.g., Qwen 0.8B vs Llama 3.2B vs GPT-4)
- Compare: time, cost, quality, policy adherence
- Data-driven decision on which model to use for production

#### Prompt A/B Testing
- Test different prompt styles (step-by-step vs freeform)
- Compare: completion rate, time, user satisfaction
- Optimize prompts for specific roles or task types

#### Workflow Comparison
- Compare different execution strategies (bounded orchestrator vs recursive planner)
- Measure: completion rate, resource usage, safety violations
- Choose workflow with best trade-off for use case

### Integration with Production

#### Promotion Workflow
1. **Sandbox execution** – Agent completes task in sandbox
2. **Review and approval** – Human reviews sandbox output
3. **Selective promotion** – Approved actions promoted to production
4. **Audit logging** – All sandbox actions and promotions logged
5. **Feedback loop** – Production results inform sandbox improvements

#### Continuous Evaluation
- **Periodic sandbox runs** – Regular evaluation of agents/models against baseline tasks
- **Regression detection** – Alert if new version performs worse than baseline
- **Trend analysis** – Track improvement or degradation over time

## Consequences

### Positive
- **Safe experimentation** – No production risk from trying new agents/models
- **Data-driven decisions** – Metrics inform model, prompt, and tool choices
- **Rapid iteration** – Can test many variants in parallel without blocking production
- **Behavioral insight** – Understand how agents think and act, not just results
- **Training data generation** – Sandbox runs create labeled examples for fine-tuning

### Negative
- **Requires infrastructure** – Sandbox environment, mocks, and metrics pipeline
- **Not production execution** – Still need to promote approved actions manually
- **Mocking overhead** – Realistic mocks require maintenance
- **False confidence** – Sandbox success does not guarantee production success

### Neutral
- **1.1 timeframe** – Not blocking 1.0 delivery; can be built incrementally
- **Optional for simple agents** – If agents are highly constrained, sandbox may be overkill
- **Complements production** – Sandbox and production can run in parallel (sandbox for testing, production for work)

## Alternatives Considered

### Alternative 1: No sandbox, trust agents in production
- **Pros:** Simpler, faster execution, no sandbox overhead
- **Cons:** High risk of production incidents, uncontrolled behavior
- **Rejected:** Experience shows uncontrolled dogfooding causes real damage; safety first

### Alternative 2: Sandbox as mandatory gate for all work
- **Pros:** Maximum safety
- **Cons:** High toil, blocks simple tasks, inefficient
- **Rejected:** Sandbox is for testing and evaluation, not all work; use for risky/novel tasks

### Alternative 3: Automated promotion without human review
- **Pros:** Faster, more autonomous
- **Cons:** Still risk of bad actions reaching production
- **Rejected:** Human in loop for production safety is non-negotiable

## Related Decisions

- [Bounded Orchestrator Loop](../../03-DESIGN/BOUNDED_ORCHESTRATOR_LOOP.md) – Sandbox integrates with orchestrator stop conditions
- [Small-Model Strategy](../../03-DESIGN/SMALL_MODEL_STRATEGY.md) – Sandbox used for model evaluation and calibration
- [Proof of Work](../../03-DESIGN/PROOF_OF_WORK.md) – Sandbox generates proof-of-work bundles without external writes
- [ZenRoleProfile](../01-ARCHITECTURE/CONTROL_PLANE_VOCABULARY.md) – Policy profiles define sandbox constraints

## Future Work

### 1.1 Implementation
- **Sandbox infrastructure** – Isolated workspaces, mocked external systems, resource limits
- **Metrics pipeline** – Collect, store, and visualize sandbox metrics
- **Evaluation suite** – Baseline tasks for regression testing and model comparison
- **Promotion workflow** – Human review interface for approving sandbox output

### 2.0+
- **Self-healing sandbox** – Sandbox detects and recovers from its own failures
- **Multi-agent simulation** – Test agent-to-agent interactions in sandbox
- **Production-like testing** – Sandbox with realistic data and latency
- **Automated promotion** – For low-risk, high-confidence actions (with safeguards)

## References

- Symphony Bounded Orchestrator: [https://github.com/getsymphony/symphony](https://github.com/getsymphony/symphony) (reference inspiration)
- Zen-Brain ROADMAP: [ROADMAP.md](../01-ARCHITECTURE/ROADMAP.md)
- Control Plane Vocabulary: [../01-ARCHITECTURE/CONTROL_PLANE_VOCABULARY.md](../01-ARCHITECTURE/CONTROL_PLANE_VOCABULARY.md) (pending)