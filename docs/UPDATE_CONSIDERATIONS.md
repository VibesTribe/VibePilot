# VibePilot Update Considerations

**Purpose:** Daily input of potential improvements from research, videos, AI analysis

**Workflow:**
1. Research agent (or human) adds considerations here
2. GLM-5 / Council vets against VibePilot's specific needs
3. Decision logged in DECISION_LOG.md (Accepted/Rejected + reasoning)
4. This file cleared for next day's considerations

**Archive:** Previous considerations stored in `.context/DECISION_LOG.md`

---

# 2026-02-15 Considerations (New)

**Source 1:** Vibeflow prototype review (github.com/VibesTribe/vibeflow)
**Source 2:** Surya's "Agent Engineering 2026" video via Gemini analysis
**Source 3:** Session discussion on context/memory efficiency

---

## Consideration 16: Pluggable Memory Architecture

**From:** Session discussion - 100k context load concern

**Problem:**
- Currently: Agents read full documents for context
- Future: Projects with months of history may exceed document approach
- Risk: Retrofitting memory systems = painful rewrite

**Proposal:**
Design memory interface now, implement with documents, swap later.

```
Memory Interface:
├── store(project_id, key, content, metadata)
├── retrieve(project_id, query, limit, filters)
└── search(project_id, embedding, threshold)

Current Implementation: File-based (read docs)
Future Implementations: Vector DB, Graph RAG, Whatever Comes Next
```

**Where it plugs in:**
- Consultant stores research findings
- Planner retrieves relevant context for planning
- Council retrieves relevant past decisions
- Supervisor retrieves similar past approvals/rejections
- All agents: semantic search of project history

**VibePilot Fit:** ✅ Essential
- Core principle: Build for change, not for now
- Unknown future tech → design interface, swap implementation
- Zero pain to add vector/graph/superseding-rag when ready

**Decision:** Accepted - Design memory interface into agent architecture

---

## Consideration 8: Vibeflow Dashboard Reuse

**From:** Vibeflow `mission-control-mockup.tsx` review

**Proposal:**
- Reuse Vibeflow's dashboard mockup for VibePilot frontend
- Already built: slice progress rings, agent hangar, task cards, ROI modal
- Just needs Supabase connection instead of static data

**VibePilot Fit:** ✅ Excellent fit
- Don't rebuild what exists
- Same team, same vision
- React + TypeScript already matches our tech stack

**Decision:** Accepted - Use Vibeflow dashboard as frontend starting point

---

## Consideration 9: Skills Manifest Pattern

**From:** Vibeflow `skills/` directory structure

**Proposal:**
- Declarative skill manifests (`skill.json`) + tiny runners (`skill.runner.mjs`)
- Skills defined outside agent code
- Swapping behavior = edit manifest, not rewrite agent

**VibePilot Fit:** ✅ Good fit
- Matches our "zero code changes for swaps" principle
- Current approach: skills defined in agent prompts
- Could enhance: external skill registry

**Decision:** Pending - Consider for future, current prompt-based approach works

---

## Consideration 10: Event Log Pattern

**From:** Vibeflow `data/state/events.log.jsonl`

**Proposal:**
- Single append-only log of all events
- State derived from events (not mutated directly)
- Full audit trail, replayable

**VibePilot Fit:** ⚠️ Partial fit
- Good for audit, but adds complexity
- We already have task_runs table for execution history
- CURRENT_STATE.md + CHANGELOG.md provide context restoration

**Decision:** Pending - Evaluate if we need event sourcing or if current approach sufficient

---

## Consideration 11: CI Gate Structure

**From:** Vibeflow `.github/workflows/` (5 CI gates)

**Proposal:**
```
ci-contracts.yml    → Schema validation
ci-diff-scope.yml   → Region-scoped patches only
ci-merge-gate.yml   → Final alignment check
ci-tests.yml        → Test suite
ci-backup.yml       → Auto-backup on merge
```

**VibePilot Fit:** ✅ Good fit for later
- We have no CI yet
- These patterns are solid
- Implement when we have tests to run

**Decision:** Pending - Implement in Phase 2 (after core system working)

---

## Consideration 12: Router Scoring Formula

**From:** Vibeflow system_plan_v5.md

**Proposal:**
```
score = w1*priority + w2*confidence + w3*provider_success_rate
        - w4*expected_latency - w5*token_over_budget_penalty
```

**VibePilot Fit:** ✅ Good fit
- Orchestrator already routes based on multiple factors
- Formalized scoring is cleaner than heuristics
- Tunable weights

**Decision:** Accepted - Implement in Orchestrator routing logic

---

## Consideration 13: OpenTelemetry Tracing

**From:** Surya "Agent Engineering" video (via Gemini)

**Proposal:**
- Every LLM call: timestamp, tokens, cost, duration
- Every tool invocation: what, when, result
- Detect loops, token waste, stuck states
- Essential for "invisible token burn" problem

**VibePilot Fit:** ✅ Excellent fit
- Watcher agent already designed for loop detection
- Tracing provides the DATA Watcher needs
- Hard to retrofit - add early
- We already track tokens in task_runs, expand to all calls

**Decision:** Accepted - Add OpenTelemetry to runners early (before we scale)

---

## Consideration 14: Agent Engineering Principles

**From:** Surya video (via Gemini)

**Principles:**
1. **Observability is non-negotiable** - See what agents are actually doing
2. **Product thinking** - Define what "good" looks like
3. **Engineering** - Durable execution, error handling
4. **Evaluation** - Test cases before users see output

**VibePilot Fit:** ✅ Already aligned
- We have Watcher for observability
- PRD-first approach for product thinking
- Error handling built into agents
- Code Tester exists, need evaluation harness

**Decision:** Confirmed - We're on the right track, add evaluation harness

---

## Consideration 15: Skills for Latest SDK Context

**From:** Surya video on "legacy SDK problem"

**Problem:** Agents default to outdated SDKs/models because training data is stale

**Proposal:**
- Use Gemini API Dev Skill or similar
- Live index of latest models, SDKs, documentation
- Agents always write current code, not legacy

**VibePilot Fit:** ⚠️ Consider later
- Our System Research agent already finds new models/tools
- Could enhance with live documentation fetching
- Not urgent for current phase

**Decision:** Pending - Consider when we have more API integrations

---

# Summary of This Session

| Consideration | Decision | Priority |
|---------------|----------|----------|
| Vibeflow Dashboard | Accepted | High - immediate reuse |
| Skills Manifest | Pending | Medium - current approach works |
| Event Log Pattern | Pending | Low - evaluate need |
| CI Gates | Pending | Phase 2 - after tests |
| Router Scoring | Accepted | Medium - implement in orchestrator |
| OpenTelemetry | Accepted | High - add early |
| Agent Engineering | Confirmed | N/A - already aligned |
| SDK Skills | Pending | Low - future consideration |

---

# 2026-02-14 Considerations (Processed)

**Source:** Gemini analysis of 3 videos on database design, AI memory, context management

## Consideration 1: Senior Engineer Schema Rules

**From:** Database design best practices video

**Proposal:**
- All tables: `id` (UUID), `created_at`, `updated_at`
- All FKs indexed and marked
- 1:1 relationships: UNIQUE constraint on FK
- N:N relationships: junction tables
- All names: lowercase_snake_case
- No business logic in primary keys

**VibePilot Fit:** ✅ Good fit
- Swappability: Standard SQL = portable
- Auditability: Timestamps on everything
- Stability: UUID IDs don't break

**Decision:** DEC-011 (Pending) - Schema audit + validation script

---

## Consideration 2: Noiseless Compression

**From:** Manolo Remiddi video on AI memory

**Proposal:**
- Compress verbose logs to shorthand signals
- 80% token reduction
- Hash-tag blocks for fetch-on-demand

**VibePilot Fit:** ❌ Not right for us
- Loses WHY (reasoning), not just WHAT
- Already solved by CURRENT_STATE.md (summary) + DECISION_LOG.md (detail)

**Decision:** DEC-013 (Rejected) - Over-compression loses context

---

## Consideration 3: Navigation-Based Context

**From:** Adam Lucek video on RAG alternatives

**Proposal:**
- Give agents terminal tools: `ls`, `cat`, `grep`, `find`
- Agents explore like human devs
- Recursive sub-LLM for large tasks

**VibePilot Fit:** ❌ Not right for us
- Security risk (execution capability)
- Implementation complexity
- Directory index in CURRENT_STATE.md already solves navigation

**Decision:** DEC-014 (Rejected) - Complexity, security risk

---

## Consideration 4: Awareness Agent

**From:** Both videos mentioned

**Proposal:**
- Tiny watcher that auto-injects context based on keywords
- "schema" → inject schema docs, "model" → inject model config

**VibePilot Fit:** ❌ Not right for us
- Heuristic risk ("model" = AI model vs data model)
- Over-engineering
- Explicit structure in CURRENT_STATE.md is clearer

**Decision:** DEC-015 (Rejected) - Over-engineering

---

## Consideration 5: Self-Awareness Document

**From:** SSOT concept in Manolo Remiddi video

**Proposal:**
- Create document explaining VibePilot to AI
- Prevents assumptions and bugs

**VibePilot Fit:** ⚠️ Partial fit
- Good idea, but don't create new file
- Add "Must Preserve / Never Do" sections to CURRENT_STATE.md instead

**Decision:** DEC-012 (Rejected as separate file, incorporated into CURRENT_STATE.md)

---

## Consideration 6: Prompt Caching

**From:** Hussein Younes video

**Proposal:**
- Implement `cache_control: { type: "ephemeral" }` in API runners
- 75% cost savings on Council reviews (same plan, 3 models)

**VibePilot Fit:** ✅ Excellent fit
- Council reviews repeat same context 3x
- Direct cost savings
- No architecture changes needed

**Decision:** DEC-007 (Pending) - Add to runners

---

## Consideration 7: Kimi K2.5 Swarm

**From:** Moonshot/Together video

**Proposal:**
- Kimi can spawn up to 100 parallel agents
- Use for "wide" tasks (repo-wide audits, parallel refactoring)
- GLM-5 remains architect, Kimi is construction crew

**VibePilot Fit:** ✅ Good fit
- Matches our multi-model architecture
- GLM-5 plans, Kimi executes in parallel
- Useful for massive legacy projects

**Decision:** DEC-008 (Pending) - Add swarm trigger to orchestrator

---

# Summary of Session

| Consideration | Decision | Why |
|---------------|----------|-----|
| Senior Schema Rules | Accepted (DEC-011) | Standard practices, portable, auditable |
| Noiseless Compression | Rejected | Loses reasoning, already solved |
| Navigation Tools | Rejected | Security/complexity, solved by index |
| Awareness Agent | Rejected | Heuristic risk, over-engineering |
| Self-Awareness Doc | Merged | Added to CURRENT_STATE.md instead |
| Prompt Caching | Accepted (DEC-007) | 75% cost savings, direct value |
| Kimi Swarm | Accepted (DEC-008) | Fits architecture, enables scale |

---

# Template for Future Considerations

```markdown
## Consideration X: [Title]

**From:** [Source - video, article, research agent]

**Proposal:**
[What the suggestion is]

**VibePilot Fit:** ✅/❌/⚠️ [Good fit / Not right / Partial fit]

**Reasoning:**
[Why it does or doesn't fit our specific system]

**Decision:** DEC-XXX (Accepted/Rejected/Pending) - [One line summary]
```

---

*File cleared after processing. Archive in DECISION_LOG.md*
*Next update: Research agent or human input*
