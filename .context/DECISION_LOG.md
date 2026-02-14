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

### DEC-004: Council Iterative Consensus Process

**Date:** 2026-02-14
**Status:** Accepted
**Context:** Council review is not a one-shot vote. Real experience shows 3 different models (GPT, Gemini, GLM-5) start with very different approaches and need 4 rounds to reach consensus.

### Decision
Council uses iterative consensus process:
- Round 1: Each model reviews independently with their lens
- Round 2+: Each model sees others' feedback, adjusts position
- Continue until all 3 models agree
- Max 5 rounds, then human arbitration

### Model Lenses
| Model | Lens | Strength |
|-------|------|----------|
| GPT | User Alignment | True to user intent, catches drift |
| Gemini | Ideal/Vision | Explores best possible, may drift from practical |
| GLM-5 | Technical/Security | Finds vulnerabilities, suggests better options |

### Trade-offs
- We gain: Multiple perspectives converge on best approach
- We lose: Time (3-4 rounds typical)
- We accept: Complex decisions need more rounds

### Review Notes
- [x] Human validated: 2026-02-14, "This is exactly how it worked in practice"
- [ ] Kimi attacked: pending

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
**Status:** Pending
**Context:** Hussein Younes video showed 75% cost savings via prompt caching. Council reviews repeat same context 3x.

### Decision
[Pending implementation]

### Implementation Notes
- Add `cache_control: { type: "ephemeral" }` to API runners
- Cache system prompts, PRD, plan context
- Measure actual savings

---

### DEC-008: Kimi K2.5 Swarm Trigger

**Date:** 2026-02-14
**Status:** Pending
**Context:** Kimi can spawn up to 100 parallel agents. Useful for massive refactoring, audits, parallel work.

### Decision
[Pending implementation]

### Implementation Notes
- Define "wide task" type in orchestrator
- Trigger Kimi swarm for: repo-wide audits, parallel refactoring, large-scale testing
- GLM-5 remains architect, Kimi is construction crew

---

## Decision Log Index

| ID | Title | Status | Date |
|----|-------|--------|------|
| DEC-001 | Dual Orchestrator Architecture | Accepted | 2026-02-13 |
| DEC-002 | State in Supabase, Code in GitHub | Accepted | 2026-02-13 |
| DEC-003 | Role System with Bounded Skills | Accepted | 2026-02-13 |
| DEC-004 | Council Iterative Consensus Process | Accepted | 2026-02-14 |
| DEC-005 | Context Isolation by Agent Role | Accepted | 2026-02-14 |
| DEC-006 | TypeScript Migration | Proposed | 2026-02-14 |
| DEC-007 | Prompt Caching Implementation | Pending | 2026-02-14 |
| DEC-008 | Kimi K2.5 Swarm Trigger | Pending | 2026-02-14 |

---

*Last updated: 2026-02-14*
*Next decision number: DEC-009*
