# VibePilot Agent Protocol

This document defines how multiple AI agents coordinate, communicate, and resolve conflicts. Without this, agents can overwrite each other's work, miss context, or create inconsistent state.

---

## Agent Roles

| Agent | Role | Capabilities | When Used |
|-------|------|--------------|-----------|
| GLM-5 | Architect/Coder | Planning, coding, supervision | Primary for all tasks |
| Kimi K2.5 | Executor/Attacker | Parallel execution, visual debugging, attack review | Parallel tasks, security review |
| Gemini | Researcher/Reviewer | Research, architecture review | Council review, research |
| DeepSeek | Backup | Same as GLM-5 | When GLM-5 unavailable |

---

## Task Lifecycle

```
1. CREATE     Human creates task with requirements
2. PLAN       GLM-5 proposes approach, writes to DECISION_LOG
3. REVIEW     Council reviews (Gemini: architecture, Kimi: attack)
4. APPROVE    Human or Council majority approves
5. EXECUTE    Assigned agent implements
6. VERIFY     Kimi or Gemini verifies implementation
7. COMPLETE   Task marked complete, handoff written
```

### State Transitions

```
pending -> planning -> reviewing -> approved -> executing -> verifying -> complete
                           |                          |
                           v                          v
                        blocked                    failed
```

### State Rules
- `planning`: Only GLM-5 can be in this state
- `reviewing`: At least 2 Council members must review
- `approved`: Requires human approval OR 2/3 Council approval
- `failed`: After 3 failed execution attempts, escalate to human
- `blocked`: Agent cannot proceed, needs human input

---

## Handoff Protocol

### When Agent Completes Work

1. **Update Supabase** (atomic transaction):
   ```sql
   UPDATE tasks 
   SET status = 'verifying',
       handoff = '{
         "from_agent": "glm-5",
         "to_agent": "kimi",
         "summary": "Implemented X, needs verification",
         "files_changed": ["src/foo.py", "src/bar.py"],
         "concerns": ["Not tested under load"]
       }',
       updated_at = NOW()
   WHERE task_id = $1;
   ```

2. **Write Handoff Summary** (human-readable):
   ```
   Task #123: Implement search debouncing
   Completed by: GLM-5
   Status: Ready for verification
   
   What was done:
   - Added 300ms debounce to search input
   - Implemented in src/components/Search.tsx
   
   Concerns:
   - Not tested with rapid consecutive searches
   - May need adjustment for mobile (touch vs click)
   
   Next agent: Please verify debouncing works correctly
   ```

3. **Next Agent Reads Before Starting**:
   - Read task record from Supabase
   - Read relevant `DECISION_LOG.md` entries
   - Read `SESSION_LOG.md` for context
   - Review `handoff` field

### Handoff Rules
- No two agents work on same task simultaneously
- Handoff must include: summary, files changed, concerns, next steps
- Receiving agent must acknowledge before starting
- If handoff incomplete, reject and request clarification

---

## Conflict Resolution

### File Conflicts
**Scenario:** Two agents modify same file

**Detection:** Before any write, check if file changed since last read:
```python
# Pseudocode
current_hash = file_hash(path)
if current_hash != expected_hash:
    raise ConflictError("File modified by another agent")
```

**Resolution:**
1. Agent with conflict must re-read file
2. Merge changes manually (no auto-merge)
3. If merge unclear, ESCALATE to human
4. Update expected_hash after successful write

### Council Deadlocks
**Scenario:** 1-2 vote where minority raised critical concern

**Rules:**
- If minority raised **security** concern: PAUSE, require human review
- If minority raised **scale** concern: PAUSE, require load testing
- If minority raised **architecture** concern: Continue with 2/3, document minority view

### Task Dependencies
**Scenario:** Task B depends on Task A, but A not complete

**Rules:**
- Task B cannot start until Task A is `complete`
- If Task A is `failed` or `blocked`, Task B is also `blocked`
- Circular dependencies: Reject at planning stage

---

## Communication Channels

### Primary: Supabase `task_packets` Table
```sql
CREATE TABLE task_packets (
  packet_id UUID PRIMARY KEY,
  task_id UUID REFERENCES tasks,
  from_agent TEXT,
  to_agent TEXT,
  message_type TEXT, -- 'request', 'response', 'escalation', 'handoff'
  content JSONB,
  created_at TIMESTAMPTZ
);
```

### Message Types
- `request`: Agent asks another agent for help
- `response`: Agent responds to request
- `escalation`: Agent cannot proceed, needs human
- `handoff`: Agent passing task to next agent

### Protocol
1. Sender writes to `task_packets`
2. Receiver polls for messages where `to_agent = self`
3. Receiver processes and writes response
4. Sender marks original as acknowledged

### Escalation Protocol
When agent cannot proceed:
```json
{
  "message_type": "escalation",
  "from_agent": "glm-5",
  "to_agent": "human",
  "content": {
    "reason": "Council vote deadlock",
    "context": "Kimi raised security concern about SQL injection",
    "requested_action": "Please review query sanitization approach",
    "blocking": true
  }
}
```

---

## Error Handling

### Agent Failure Modes
| Failure | Detection | Recovery |
|---------|-----------|----------|
| Timeout | No response in N seconds | Retry with exponential backoff, max 3 attempts |
| Garbage output | Response fails validation | Request regeneration, log for analysis |
| Rate limited | 429 from API | Queue task, retry after cooldown |
| Context lost | Forgets earlier constraints | Re-inject from SESSION_LOG.md |

### Retry Rules
- Max 3 attempts per task
- After 3 failures: status = `failed`, escalate to human
- Backoff: 1s, 5s, 30s

### Graceful Degradation
If primary model (GLM-5) unavailable:
1. Route coding tasks to DeepSeek (backup)
2. Log degradation in `task_runs`
3. Alert human
4. Continue with degraded capability

---

## Context Preservation

### Session Start Checklist
Every agent must do this before working:
- [ ] Read `SESSION_LOG.md` - what happened before?
- [ ] Read relevant `DECISION_LOG.md` entries
- [ ] Read task record from Supabase
- [ ] Check `task_packets` for messages to self
- [ ] Verify no conflicts with in-progress tasks

### Session End Checklist
Every agent must do this before stopping:
- [ ] Update task status in Supabase
- [ ] Write handoff if passing to another agent
- [ ] Update `SESSION_LOG.md` with progress
- [ ] Create `DECISION_LOG.md` entry if significant decision made
- [ ] Ensure all code committed to GitHub

### Context Window Safety
- If approaching 80% context (160k tokens):
  1. Say "Compress session"
  2. Summarize to `SESSION_LOG.md`
  3. Start fresh session
  4. Read `SESSION_LOG.md` to restore

---

## Testing Protocol

### Before Marking Task Complete
- [ ] Happy path tested
- [ ] Error paths tested
- [ ] Edge cases considered
- [ ] No regressions in existing tests
- [ ] Code reviewed by another agent or human

### Verification Mode
After implementation, different agent verifies:
```
IMPLEMENTER: GLM-5
VERIFIER: Kimi (attack mode) or Gemini (architecture mode)

Verification includes:
- Does it solve the stated problem?
- Are there security vulnerabilities?
- Are there race conditions?
- Does it handle errors gracefully?
- Is it tested?
```

---

## Migration Safety

### Pre-Migration Checklist
- [ ] All tasks complete or properly blocked
- [ ] All state synced to Supabase
- [ ] All code committed to GitHub
- [ ] `SESSION_LOG.md` updated with migration state
- [ ] `.env.example` has all required variables
- [ ] `setup.sh` works on fresh machine

### Post-Migration Checklist
- [ ] Read `SESSION_LOG.md`
- [ ] Run `setup.sh`
- [ ] Verify Supabase connection
- [ ] Verify GitHub sync
- [ ] Resume from "Next Steps" in SESSION_LOG.md

---

*Last updated: 2026-02-14*
*This document is the contract between agents. Violations = system instability.*
