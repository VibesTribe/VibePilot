# VibePilot Update Considerations

**Purpose:** Daily input of potential improvements from research, videos, AI analysis

**Workflow:**
1. Research agent (or human) adds considerations here
2. GLM-5 / Council vets against VibePilot's specific needs
3. Decision logged in DECISION_LOG.md (Accepted/Rejected + reasoning)
4. This file cleared for next day's considerations

**Archive:** Previous considerations stored in `.context/DECISION_LOG.md`

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
