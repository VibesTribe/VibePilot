# VibePilot Quick Reference

One-page cheat sheet for daily use.

---

## Before Coding (The 8 Gates)

```
☐ P1: Problem defined in 1 sentence? Failure modes identified?
☐ P2: 2+ alternatives considered? Trade-offs documented?
☐ P3: Dependencies audited? (maintained, secure, light)
☐ P4: State source identified? Race conditions prevented?
☐ P5: All error paths handled? (fail, garbage, rate-limit)
☐ P6: Security reviewed? (inputs, secrets, auth)
☐ P7: Observability planned? (logs, metrics, alerts)
☐ P8: Cost estimated? Caching opportunities identified?
```

## The P-R-E-V-C Workflow

```
Plan     → GLM-5 proposes, writes to DECISION_LOG
Review   → Council: Gemini (architecture), Kimi (attack)
Execute  → Approved agent implements
Validate → Different agent verifies
Confirm  → Human or Council majority confirms
```

## Council Review Modes

| Agent | Mode | Question |
|-------|------|----------|
| Gemini | Architecture | "Does this solve the problem correctly?" |
| Kimi | Attack | "How does this break?" |

## Kimi Attack Checklist

```
☐ Race conditions?
☐ Missing rate limiting/debouncing?
☐ Memory leaks? TTL gaps?
☐ N+1 queries?
☐ Missing indexes?
☐ SQL injection / XSS?
☐ Hardcoded values?
☐ Missing timeouts?
☐ Unhandled errors?
☐ What breaks at 100x scale?
```

## Common Vibe Coding Traps

| Trap | Fix |
|------|-----|
| No debouncing | Rate limit all user inputs |
| No timeout | All external calls have timeout |
| N+1 queries | Review JOIN patterns |
| Magic numbers | Named constants |
| Hardcoded paths | Environment variables |
| Silent failures | Observable outcomes |
| No error context | Log full context |

## Agent Handoff Protocol

```
1. Update Supabase (status + handoff JSON)
2. Write summary: done, changed, concerns, next
3. Next agent reads: task, DECISION_LOG, SESSION_LOG
4. Acknowledge before starting
```

## Conflict Resolution

```
File conflict → Re-read, merge manually, escalate if unclear
Council 2-1 + security concern → PAUSE, human review
Task blocked > 3 attempts → ESCALATE to human
```

## State Transitions

```
pending → planning → reviewing → approved → executing → verifying → complete
                          ↓                         ↓
                       blocked                    failed (→ escalate after 3x)
```

## Migration Essentials

```
☐ All env vars in .env.example
☐ setup.sh works on fresh machine
☐ Supabase data exported
☐ GitHub fully synced
☐ SESSION_LOG updated
```

## Daily Checklist

```
Start:
☐ Read SESSION_LOG.md
☐ Check task_packets
☐ Read relevant DECISION_LOG entries

End:
☐ Update task status
☐ Write handoff if needed
☐ Update SESSION_LOG.md
☐ Commit to GitHub
☐ Create DECISION_LOG entry if significant
```

## Cost Awareness

```
Council review = 3x model calls (use prompt caching!)
Kimi swarm = parallel but expensive
GLM-5 primary = cheapest for most tasks
```

## Emergency Contacts

```
Human escalation: task_packets with message_type: 'escalation'
System down: Check Supabase status, GitHub status
Context lost: Re-read SESSION_LOG.md + DECISION_LOG.md
```

---

*Print this. Keep it visible. No excuses.*
