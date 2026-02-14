# VibePilot Guardrails

> "The difference between a senior dev and a vibe coder is that seniors make decisions explicitly, even if they're wrong. Vibe coders never make decisions at all."

This document defines the mandatory checks that must pass before any code is written. No exceptions.

---

## Pre-Code Gates (Must Pass Before Coding)

### P1: Problem Definition
- [ ] Is the problem clearly stated in one sentence?
- [ ] What is the **failure mode** if this feature doesn't exist?
- [ ] What is the **failure mode** if this feature is implemented poorly?

### P2: Trade-off Analysis
- [ ] List at least 2 alternative approaches
- [ ] Explain why the chosen approach is preferred
- [ ] Document what we're giving up by this choice
- [ ] What breaks at 10x scale? 100x scale?

### P3: Dependency Audit
- [ ] Every new package: is it actively maintained? (last commit < 6 months)
- [ ] Every new package: any known security vulnerabilities?
- [ ] Every new package: is there a lighter alternative?
- [ ] Does this add lock-in to a specific platform/provider?

### P4: State & Concurrency
- [ ] Where is the source of truth for this feature's state?
- [ ] What happens if two agents update this simultaneously?
- [ ] Is there a race condition risk? How is it prevented?
- [ ] What happens if the state store (Supabase) is unavailable?

### P5: Error Handling
- [ ] What if external API fails? (retry? queue? graceful degradation?)
- [ ] What if external API returns garbage?
- [ ] What if external API is rate-limited?
- [ ] Are all error paths logged with context?

### P6: Security Review
- [ ] Are there any user inputs? How are they sanitized?
- [ ] Are there any secrets/credentials? Where are they stored?
- [ ] Is there auth/authz required? Implemented?
- [ ] Could this be exploited for unauthorized access?

### P7: Observability
- [ ] How will we know this is working in production?
- [ ] How will we know this is broken in production?
- [ ] What metrics should be tracked?
- [ ] What logs will help debug issues?

### P8: Cost Impact
- [ ] Estimated tokens per operation?
- [ ] Estimated API calls per operation?
- [ ] Any caching opportunities?
- [ ] What happens if this is called 1000x in parallel?

---

## Production Readiness Checklist (Before Merge)

### Performance
- [ ] Rate limiting / debouncing implemented where needed?
- [ ] Database queries optimized? (N+1 problem avoided?)
- [ ] Memory leaks checked? (long-running processes)
- [ ] Timeouts implemented for all external calls?

### Reliability
- [ ] Graceful degradation for external failures?
- [ ] Retry logic with exponential backoff?
- [ ] Circuit breaker for flaky dependencies?
- [ ] Dead letter queue for failed operations?

### Testing
- [ ] Happy path tested
- [ ] Error paths tested
- [ ] Edge cases tested
- [ ] Load/stress tested (if applicable)

### Migration Safety
- [ ] Schema changes backward compatible?
- [ ] Rollback plan documented?
- [ ] Data migration tested?
- [ ] Feature flags for gradual rollout?

---

## Agent Coordination Rules

### Handoff Protocol
When one agent completes work and hands to another:
1. Update task status in Supabase (atomic transaction)
2. Write handoff summary to task record
3. Next agent reads handoff before starting
4. No two agents work on same task simultaneously

### Conflict Resolution
- If Council vote is 2-1 where the "1" raised a security/scale concern: **PAUSE**
- If two agents modify same file: **ESCALATE** to human
- If task blocked > 3 attempts: **ESCALATE** to human

### Context Preservation
- Every agent must read `SESSION_LOG.md` before starting
- Every agent must read relevant `DECISION_LOG.md` entries
- Every agent must update `SESSION_LOG.md` with progress

---

## Platform Migration Safety

### Must Be Portable
- No hardcoded paths (use environment variables)
- No GCE-specific APIs
- All state in Supabase (not local files)
- All code in GitHub (not local)
- Setup must be one command: `./setup.sh`

### Migration Checklist (Before Any Move)
- [ ] All environment variables documented in `.env.example`
- [ ] `setup.sh` tested on fresh machine
- [ ] Supabase data exported
- [ ] GitHub fully synced
- [ ] No uncommitted changes
- [ ] Session log updated with migration state

---

## Common "Vibe Coding" Traps to Avoid

| Trap | Symptoms | Prevention |
|------|----------|------------|
| Missing debouncing | Works in dev, crashes at scale | Rate limit all user-triggered operations |
| No timeout | Hangs forever on slow API | All external calls have timeout + retry |
| N+1 queries | Fast locally, slow in prod | Review all database query patterns |
| Missing indexes | Works with 10 rows, slow with 10k | Index columns used in WHERE/JOIN |
| Hardcoded config | Works on current machine only | All config in env vars |
| No error context | "Something went wrong" | Log full context on errors |
| Silent failures | Task "completes" but nothing happened | All operations have observable outcomes |
| Magic numbers | `if x > 100` - why 100? | All thresholds are named constants |

---

## Review Mode Reference

When Council reviews code, use these lenses:

### Architecture Review (Gemini)
- "Does this solve the stated problem?"
- "Is the approach idiomatic?"
- "Are there simpler alternatives?"

### Attack Review (Kimi)
- "How does this break under load?"
- "What race conditions exist?"
- "What security vulnerabilities exist?"
- "What happens if [dependency] fails?"

---

*Last updated: 2026-02-14*
*This document is mandatory. Skipping checks = skipping production safety.*
