# VibePilot Decision Log

Every significant technical decision must be documented here. This is not optional - it's how we avoid "vibe coding" (making decisions without realizing we're making them).

---

## Template

For each decision, create an entry following this format:

```markdown
## DEC-XXX: [Short Title]

**Date:** YYYY-MM-DD
**Status:** Proposed | Accepted | Deprecated | Superseded by DEC-YYY
**Context:** What is the problem we're solving?

### Decision
[One paragraph: what are we doing?]

### Alternatives Considered
1. **Alternative A:** [description]
   - Pros: [list]
   - Cons: [list]
   - Why rejected: [reason]

2. **Alternative B:** [description]
   - Pros: [list]
   - Cons: [list]
   - Why rejected: [reason]

### Trade-offs
- We gain: [what we get]
- We lose: [what we sacrifice]
- We accept: [what we tolerate]

### Failure Modes
- **At 10x scale:** What breaks?
- **At 100x scale:** What breaks?
- **If [dependency] fails:** What happens?

### Dependencies
- Requires: [other decisions, packages, services]
- Blocks: [decisions that depend on this]

### Rollback Plan
If this decision is wrong, how do we undo it?

### Review Notes
- [ ] Gemini reviewed: [date, verdict]
- [ ] Kimi attacked: [date, vulnerabilities found]
- [ ] Human approved: [date, notes]

---

*Last updated: [date]*
```

---

## Active Decisions

### DEC-001: Dual Orchestrator Architecture

**Date:** 2026-02-13
**Status:** Accepted
**Context:** Need to route tasks between multiple AI models (GLM-5, Kimi, Gemini) based on task type.

### Decision
Implement a `dual_orchestrator.py` that routes tasks based on type: planning/coding to GLM-5, research to Gemini, parallel execution to Kimi.

### Alternatives Considered
1. **Single model (GLM-5 only)**
   - Pros: Simpler, no routing logic
   - Cons: Misses Kimi's parallel strengths, Gemini's research
   - Why rejected: Wastes available model capabilities

2. **All models review everything**
   - Pros: Maximum redundancy
   - Cons: 3x cost, slower, overkill for simple tasks
   - Why rejected: Inefficient resource use

### Trade-offs
- We gain: Best model for each task type
- We lose: Simplicity, single model to debug
- We accept: More complex routing logic

### Failure Modes
- **At 10x scale:** Routing becomes bottleneck - need queue
- **At 100x scale:** Need distributed orchestrator
- **If GLM-5 unavailable:** Kimi can take coding tasks as backup

### Dependencies
- Requires: Model registry in Supabase
- Blocks: Council activation, swarm triggers

### Rollback Plan
Replace `dual_orchestrator.py` with simple `glm_runner.py` - all tasks go to GLM-5.

### Review Notes
- [x] Gemini reviewed: 2026-02-13, approved
- [ ] Kimi attacked: pending
- [ ] Human approved: 2026-02-13, "let's try it"

---

### DEC-002: State in Supabase, Code in GitHub

**Date:** 2026-02-13
**Status:** Accepted
**Context:** Need reliable state management and version control for multi-agent system.

### Decision
All task state, runs, and configuration stored in Supabase. All code and documentation in GitHub. Local machine has only ephemeral files (venv, logs, cache).

### Alternatives Considered
1. **Local SQLite database**
   - Pros: No network dependency, faster
   - Cons: Lost on machine failure, hard to share
   - Why rejected: Multi-agent needs shared state

2. **Files in GitHub only**
   - Pros: Version control for everything
   - Cons: Merge conflicts on concurrent updates, no atomicity
   - Why rejected: Not designed for high-frequency state updates

### Trade-offs
- We gain: Durable state, multi-agent access, queryability
- We lose: Some latency, external dependency
- We accept: Supabase as critical dependency

### Failure Modes
- **At 10x scale:** Supabase connection pooling needed
- **At 100x scale:** May need read replicas
- **If Supabase down:** Queue operations locally, sync when up

### Dependencies
- Requires: Supabase account, connection strings
- Blocks: Everything - this is foundational

### Rollback Plan
Export Supabase data, migrate to local SQLite + file-based queue.

### Review Notes
- [x] Gemini reviewed: 2026-02-13, approved
- [ ] Kimi attacked: pending
- [ ] Human approved: 2026-02-13, "correct approach"

---

### DEC-003: Role System with Bounded Skills

**Date:** 2026-02-13
**Status:** Accepted
**Context:** Agents with too many skills drift and hallucinate. Need focused roles.

### Decision
Each role has maximum 2-3 skills. Roles are defined in `core/roles.py` with bounded prompts. Models "wear hats" rather than being single-purpose agents.

### Alternatives Considered
1. **Single generalist agent**
   - Pros: No context switching
   - Cons: No specialization, more drift
   - Why rejected: Observed hallucination issues

2. **Many single-skill micro-agents**
   - Pros: Maximum focus
   - Cons: Coordination overhead, handoff complexity
   - Why rejected: Too much overhead for current scale

### Trade-offs
- We gain: Focused prompts, less drift
- We lose: Some flexibility
- We accept: May need new roles for edge cases

### Failure Modes
- **At 10x scale:** Role definitions become stale
- **At 100x scale:** Need dynamic role generation
- **If role doesn't fit task:** Escalate to human or generalist mode

### Dependencies
- Requires: Dual orchestrator to route by role
- Blocks: Council activation

### Rollback Plan
Replace roles with single generalist prompt.

### Review Notes
- [x] Gemini reviewed: 2026-02-13, approved
- [ ] Kimi attacked: pending
- [ ] Human approved: 2026-02-13

---

## Pending Decisions

### DEC-004: Council Two-Process Model

**Date:** 2026-02-14
**Status:** Accepted
**Context:** Council review isn't one-size-fits-all. PRDs and plans need iterative consensus. System updates need quick validation.

### Decision
Two Council processes:

**Process A - One-Shot Vote:** System updates, maintenance, new features
- 1 round, vote-based
- 3 APPROVED = proceed
- Fast, clear-cut decisions

**Process B - Iterative Consensus:** PRDs, full vertical slice plans
- 3-4 rounds typical
- Each model sees others' feedback
- Ensures: user intent, system goals, no tech drift, clear dependencies, preventative medicine

### Model Lenses
| Model | Lens | Catches |
|-------|------|---------|
| GPT | User Alignment | Drift from user intent |
| Gemini | Ideal/Vision | Opportunities (may need reining) |
| GLM-5 | Technical/Security | Build issues, vulnerabilities |

### Trade-offs
- We gain: Right process for right decision type
- We lose: None (more efficient)
- We accept: PRDs take longer, but worth it

### Review Notes
- [x] Human validated: 2026-02-14, "One-shot for updates, iterative for PRDs/plans"

---

### DEC-005: Context Isolation by Agent Role

**Date:** 2026-02-14
**Status:** Accepted
**Context:** Task agents seeing full system causes drift and hallucination. Each agent should see only what they need.

### Decision
Enforce context isolation by agent role:
- Task Agent: ONLY their task (zero drift risk)
- Planner: Full PRD, system overview
- Council: Everything (for thorough review)
- Supervisor: Plan + task (for validation)
- Maintenance: Everything (sandbox tested first)
- Researcher: System overview (to find improvements)
- Tester: Only code (objective testing)

### Trade-offs
- We gain: Zero drift, no hallucination from context overflow
- We lose: Some context that might be helpful
- We accept: Explicit handoffs required

### Review Notes
- [x] Human approved: 2026-02-14

---

### DEC-006: TypeScript Migration

**Date:** 2026-02-14
**Status:** Proposed
**Context:** Video evidence suggests AI agents perform better with TypeScript due to type inference and self-documentation.

### Decision
[Pending discussion]

### Open Questions
- When to migrate? (now vs. later vs. never)
- What's the migration cost?
- Does current Python stack have issues that TS would solve?

---

### DEC-007: Prompt Caching Implementation

**Date:** 2026-02-14
**Status:** Accepted
**Context:** Council reviews repeat same context 3x. Hussein Younes video showed 75% cost savings via prompt caching.

### Decision
Implement `cache_control: { type: "ephemeral" }` in API runners.

### Implementation
- Created `runners/api_runner.py` - Base class with caching support
- Supports: DeepSeek, GLM API, Gemini API
- Cached context: System prompts, PRD, plans
- Only pay for new tokens

### Usage
```python
runner = DeepSeekRunner()
result = runner.execute(
    prompt="What is VibePilot?",
    cached_context=["PRD content", "Architecture overview"],
    system_prompt="You are a helpful assistant."
)
```

---

### Proposed Rules
- All tables: `id` (UUID), `created_at`, `updated_at`
- All FKs indexed and marked
- 1:1 relationships: UNIQUE constraint on FK
- N:N relationships: junction tables
- All names: lowercase_snake_case
- No business logic in primary keys

### Why
- Swappability: Standard SQL = portable
- Auditability: Timestamps on everything
- Stability: UUID IDs don't break

### Implementation
1. Audit existing schema against rules
2. Create validation script (`scripts/validate_schema.sh`)
3. Fix any violations

---

### DEC-012: Self-Awareness SSOT Document

**Date:** 2026-02-14
**Status:** Rejected
**Context:** Proposed separate document explaining system to AI.

### Why Rejected
- Duplicates CURRENT_STATE.md (two sources of truth)
- Better: Add "Must Preserve / Never Do" sections to existing file
- Simpler is better

### Resolution
Added sections to CURRENT_STATE.md instead of new file.

---

### DEC-013: Noiseless Compression Protocol

**Date:** 2026-02-14
**Status:** Rejected
**Context:** Proposed compressing logs to shorthand signals for 80% token reduction.

### Why Rejected
- "Noise" often contains WHY, not just WHAT
- Over-compression loses reasoning
- Already solved: CURRENT_STATE.md (summary) + DECISION_LOG.md (detail)

### Resolution
Keep existing summary/detail pattern. Don't compress further.

---

### DEC-014: Navigation-Based Context

**Date:** 2026-02-14
**Status:** Rejected
**Context:** Proposed giving agents terminal tools (ls, cat, grep) instead of feeding files.

### Why Rejected
- Requires execution capability → security risk
- Implementation complexity
- Context index in CURRENT_STATE.md solves the same problem

### Resolution
Directory index + Source of Truth index already tells agents where to look.

---

### DEC-015: Awareness Agent

**Date:** 2026-02-14
**Status:** Rejected
**Context:** Proposed auto-injecting context based on keywords.

### Why Rejected
- Heuristic risk ("model" = AI model vs data model)
- Over-engineering
- CURRENT_STATE.md structure already provides clear navigation

### Resolution
Explicit structure in CURRENT_STATE.md is sufficient.

---

## Decision Log Index

| ID | Title | Status | Date |
|----|-------|--------|------|
| DEC-001 | Dual Orchestrator Architecture | Accepted | 2026-02-13 |
| DEC-002 | State in Supabase, Code in GitHub | Accepted | 2026-02-13 |
| DEC-003 | Role System with Bounded Skills | Accepted | 2026-02-13 |
| DEC-004 | Council Two-Process Model | Accepted | 2026-02-14 |
| DEC-005 | Context Isolation by Agent Role | Accepted | 2026-02-14 |
| DEC-006 | TypeScript Migration | Proposed | 2026-02-14 |
| DEC-007 | Prompt Caching Implementation | Accepted | 2026-02-14 |
| DEC-008 | Kimi K2.5 Swarm Trigger | Pending | 2026-02-14 |
| DEC-009 | Council Feedback Summarization | Accepted | 2026-02-14 |
| DEC-010 | Single Source of Truth (CURRENT_STATE.md) | Accepted | 2026-02-14 |
| DEC-011 | Schema Senior Rules Audit | Accepted | 2026-02-14 |
| DEC-012 | Self-Awareness SSOT Document | Rejected | 2026-02-14 |
| DEC-013 | Noiseless Compression Protocol | Rejected | 2026-02-14 |
| DEC-014 | Navigation-Based Context | Rejected | 2026-02-14 |
| DEC-015 | Awareness Agent | Rejected | 2026-02-14 |

**Why DEC-012 to DEC-015 rejected:** Over-engineering. Solved by adding Must Preserve/Never Do sections to CURRENT_STATE.md.

---

*Last updated: 2026-02-14*
*Next decision number: DEC-016*
