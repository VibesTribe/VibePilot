# VibePilot Learning System

**Created:** 2026-02-28
**Session:** 36

---

## PRINCIPLES

- Human only reviews: complex system suggestions, API credit issues, UI/UX
- Everything else: AI self-maintains, self-optimizes
- GitHub = source of truth
- Research branch for findings and review docs
- Threshold: 2-3 failures → learning triggered

---

## LEARNING FLOW

```
Task completes (success or fail)
        ↓
Supervisor notes outcome in task_runs (Supabase)
        ↓
Supabase computes fresh scores via RPC (on-the-fly)
        ↓
Pattern detection (2-3 same issue):
  - Trigger: Update "What I've Learned" in agent prompt
  - Trigger: Adjust routing/model selection
  - Trigger: Flag for system optimization if complex
        ↓
Daily: Maintenance agent updates agent prompts with new learnings
```

---

## DATA STORAGE

| Data | Location | Why |
|------|----------|-----|
| Task runs | Supabase `task_runs` | Queryable, computed on-the-fly |
| Model scores | Supabase RPC `get_model_score_for_task` | Computed from task_runs |
| Failure patterns | Supabase RPC `get_failure_patterns` | Computed from task_runs |
| Agent prompts | Git `prompts/*.md` | Versioned, source of truth |
| Research findings | Git `research/` branch | Versioned, linkable |
| Review docs | Git `research/` branch | Human sees full history |

---

## SIMPLE vs COMPLEX (Council Routing)

| Change Type | Who Decides | Example |
|-------------|-------------|---------|
| **New model add** | Supervisor (simple) | Add Grok to registry |
| **New platform add** | Supervisor (simple) | Add Mistral web |
| **Pricing update** | Supervisor (simple) | DeepSeek price change |
| **Config tweak** | Supervisor (simple) | Adjust timeout values |
| **Minor prompt improvement** | Supervisor (simple) | Clarify instruction |
| **New RAG system** | Council (complex) | Add vector DB |
| **Architecture change** | Council (complex) | New agent type |
| **Workflow modification** | Council (complex) | Change task flow |
| **Security-related** | Council (complex) | Auth changes |
| **API credit exhausted** | Human (always) | Paid tier empty |
| **UI/UX changes** | Human (always) | Dashboard modification |
| **Council blocked** | Human (always) | Can't reach consensus |

---

## HUMAN REVIEW FLOW

```
Complex change proposed
        ↓
Council reviews (3 lenses, independent)
        ↓
System Researcher updates doc with all council feedback
        ↓
Doc saved: research/YYYY-MM-DD-suggestion-name.md
        ↓
Flag in dashboard: "Review Needed"
        ↓
Human clicks "Review Now" → Sees complete doc:
  - Original suggestion
  - How things work now
  - Council feedback (all 3 lenses)
  - Recommendation
  - Alternative options (if any)
        ↓
Human decides:
  - Approve → Maintenance implements
  - Ask Questions → Clarification added to doc
  - Reject → Closed with reason
```

---

## MAINTENANCE DAILY TASK

Runs daily at scheduled time:

```
Maintenance agent:
  1. Query Supabase for new learnings (patterns detected today)
  2. For each agent prompt with new learnings:
     - Read current "What I've Learned" section
     - Append new learnings with date
     - Prune learnings older than 30 days
  3. Commit to main: "learn: Update agent learnings YYYY-MM-DD"
  4. Report to Supervisor
```

---

## "WHAT I'VE LEARNED" FORMAT

Each agent prompt includes:

```markdown
## What I've Learned

### YYYY-MM-DD
- [Learning 1] (X successes/failures)
- [Learning 2] (Council feedback)

### YYYY-MM-DD
- [Learning 3] (pattern detected)

### Patterns to Avoid
- [Pattern 1]
- [Pattern 2]

### Strengths Discovered
- [Strength 1]
```

**Rules:**
- Keep last 30 days of dated learnings
- "Patterns to Avoid" and "Strengths Discovered" persist
- Each learning includes source (failures/Council/pattern)

---

## FAILURE PATTERN THRESHOLDS

| Pattern Count | Action |
|---------------|--------|
| 1 failure | Log, no action |
| 2 failures | Add to agent learning, adjust routing |
| 3 failures | Flag for strategic review if high impact |
| 5+ failures | Escalate to Planner for task redesign |

---

## SUPERVISOR DECISION MATRIX

### Simple → Approve Directly (No Council)
- New model in registry
- New platform destination
- Pricing update
- Config tweak
- Minor prompt improvement
- Dependency version bump
- New tool registration

### Complex → Call Council
- New architecture component
- New data store (vector DB, cache layer)
- Agent role changes
- Workflow modifications
- Security-related changes
- Schema changes
- Breaking changes

### Always → Human Review
- API credit exhausted
- UI/UX changes (visual tester requires human)
- Council blocked (can't reach consensus after 6 rounds)
- Legal/ethical concerns
- Cost impact >$X threshold

---

## REVIEW DOC TEMPLATE

Location: `research/YYYY-MM-DD-short-name.md`

```markdown
# [Suggestion Title]

**Date:** YYYY-MM-DD
**Status:** Pending Review
**Source:** System Researcher / Pattern Detection / Council Escalation

---

## Suggestion

[What is being proposed]

## How Things Work Now

[Current state, what we use, how it functions]

## Council Feedback

### Lens 1: [User Alignment / Architecture / Feasibility]
- **Vote:** APPROVED / REVISION_NEEDED / BLOCKED
- **Notes:** [Specific feedback]

### Lens 2: [Lens Name]
- **Vote:** APPROVED / REVISION_NEEDED / BLOCKED
- **Notes:** [Specific feedback]

### Lens 3: [Lens Name]
- **Vote:** APPROVED / REVISION_NEEDED / BLOCKED
- **Notes:** [Specific feedback]

## Consensus

[Summary of council decision: approved / revision needed / blocked]

## Recommendation

[System Researcher's recommendation based on council feedback]

## Alternative Options (if any)

1. [Alternative 1 with pros/cons]
2. [Alternative 2 with pros/cons]

## Impact Assessment

- **Complexity:** Low / Medium / High
- **Risk:** Low / Medium / High
- **Dependencies:** [List any]
- **Rollback:** Easy / Moderate / Difficult

## Actions

- [ ] Approve
- [ ] Request Changes
- [ ] Ask Questions
- [ ] Reject

---

## Human Decision

**Decision:** [Pending / Approved / Rejected / Questions Asked]
**Date:** [When decided]
**Notes:** [Any notes from human]
