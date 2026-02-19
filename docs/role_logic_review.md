# Role Logic Review - File Access & Git Operations

**Created:** 2026-02-19
**Status:** Council Review Required
**Author:** GLM-5 (Session 16)

---

## The Question

Who touches files? Who touches git? Who decides vs. who executes?

This document proposes a cleaner separation of concerns for Council review.

---

## Core Principle

> **Decision and Execution are separate.**
> 
> One agent decides (Supervisor). One agent executes (Maintenance).
> 
> Runners return code. They don't commit it.

---

## Current State (Problems)

| Agent | Has Git Access | Has File Access | Problem |
|-------|----------------|-----------------|---------|
| Planner | Yes (read/write) | Yes | Why write? Planner reads to understand, shouldn't modify |
| Supervisor | Yes | Yes | Does both decision AND execution - conflates roles |
| Internal CLI | Yes | Yes | Creates branches, commits - should just return code |
| Maintenance | Yes | Yes | Only does system patches, not task execution |
| Courier | No | No | Correct |
| Council | No | Read only | Correct |
| Tester | No | Read only (test files) | Correct |
| Orchestrator | No | No | Correct |
| Researcher | No | No | Correct |

**Key Issues:**

1. **Supervisor does too much** - decides AND executes git operations
2. **Internal CLI touches git** - should return code only
3. **Maintenance scope too narrow** - only system patches, not task git operations
4. **Planner has unnecessary write access** - only needs to read codebase for planning

---

## Proposed State

### File Access Matrix

| Agent | Read Files | Write Files | Git Operations | Notes |
|-------|------------|-------------|----------------|-------|
| **Planner** | ✅ | ❌ | ❌ | Read codebase for context only |
| **Council** | ✅ | ❌ | ❌ | Read-only review |
| **Supervisor** | ✅ | ❌ | ❌ | Read for review, decide actions |
| **Orchestrator** | ❌ | ❌ | ❌ | Routing only |
| **Researcher** | ❌ | ❌ | ❌ | Web research only |
| **Internal CLI Runner** | ✅ | ❌* | ❌ | *Returns code, doesn't write it |
| **Courier Runner** | ❌ | ❌ | ❌ | No codebase access |
| **Code Tester** | ✅ | ❌ | ❌ | Read code to test |
| **Visual Tester** | ❌ | ❌ | ❌ | Browser only |
| **Maintenance** | ✅ | ✅ | ✅ | ONLY agent with write/git |
| **Watcher** | ❌ | ❌ | ❌ | Monitor only |

### Git Operations Matrix

| Git Action | Who Decides | Who Executes |
|------------|-------------|--------------|
| Create task branch | Supervisor | Maintenance |
| Commit runner output | Supervisor | Maintenance |
| Merge task → module | Supervisor | Maintenance |
| Create module branch | Supervisor | Maintenance |
| Merge module → main | Supervisor | Maintenance |
| Tag release | Supervisor | Maintenance |
| Delete task branch | Supervisor | Maintenance |
| Delete module branch | Supervisor | Maintenance |
| System patches | Maintenance (self) | Maintenance |

---

## Detailed Role Definitions

### Supervisor (Quality Gatekeeper)

**Decides:**
- Plan approval (after Council)
- Task output approval/rejection
- Test pass/fail acceptance
- When to merge
- When to delete branches
- When to tag

**Does NOT:**
- Touch git
- Write files
- Execute code

**Triggers Maintenance with:**
```
{
  "action": "create_branch",
  "branch_type": "task",
  "branch_name": "task/T001-user-auth",
  "approved_by": "supervisor",
  "timestamp": "..."
}
```

```
{
  "action": "commit_code",
  "branch": "task/T001-user-auth",
  "code_output": { ... from runner ... },
  "task_id": "uuid",
  "approved_by": "supervisor"
}
```

```
{
  "action": "merge_branch",
  "source": "task/T001-user-auth",
  "target": "module/user-auth",
  "create_target_if_missing": true,
  "approved_by": "supervisor",
  "after_merge": "delete_source"
}
```

---

### Maintenance (File & Git Operator)

**Receives commands from:**
- Supervisor (task execution)
- Council (system improvements)
- Research findings (approved changes)

**Executes:**
- All git operations
- All file writes
- All branch management

**Never acts without approval:**
- Supervisor approval for task-related
- Council approval for system changes
- Human approval for high-risk

**Reports back:**
```
{
  "action": "merge_branch",
  "status": "success",
  "source": "task/T001-user-auth",
  "target": "module/user-auth",
  "source_deleted": true,
  "commit_hash": "abc123",
  "timestamp": "..."
}
```

---

### Internal CLI Runner (Code Producer)

**Receives:**
- Task packet
- Relevant codebase files (read-only context)
- Output format requirements

**Returns:**
```json
{
  "task_id": "T001",
  "status": "success",
  "output": {
    "files": [
      {"path": "auth.py", "content": "..."},
      {"path": "test_auth.py", "content": "..."}
    ],
    "summary": "Created auth module with login/logout",
    "notes": "Used bcrypt for passwords as specified"
  },
  "metadata": {
    "model": "kimi-k2.5",
    "tokens_in": 2000,
    "tokens_out": 1500,
    "files_read": 5,
    "duration_seconds": 120
  }
}
```

**Does NOT:**
- Create branches
- Commit code
- Push to remote
- Modify files directly

---

### Courier Runner (Code Producer)

**Same as Internal CLI - returns code, doesn't touch git.**

Plus: Returns `chat_url` for future reference.

---

## Task Flow with New Roles

```
1. Supervisor approves plan
   → Tasks created in Supabase (no git yet)

2. Orchestrator assigns task to Runner
   → No git yet

3. Supervisor tells Maintenance: "create branch task/T001"
   → Maintenance creates branch

4. Runner executes, returns code
   → No git access

5. Supervisor reviews output
   → If FAIL: Return to runner with notes
   → If PASS: Continue

6. Supervisor tells Maintenance: "commit this code to task/T001"
   → Maintenance commits

7. Supervisor triggers Tester
   → Tester returns pass/fail

8. If tests pass, Supervisor tells Maintenance:
   "merge task/T001 → module/user-auth, create module if needed, delete task branch"
   → Maintenance executes

9. When all tasks in module complete:
   Supervisor tells Maintenance: "merge module/user-auth → main, tag, delete module branch"
   → Maintenance executes

10. Maintenance reports completion to Supervisor
    → Supervisor updates Supabase, unlocks dependents
```

---

## Why Task → Module → Main Branch Structure

### The Structure

```
task/T001     task/T002     task/T003     task/T004
    │             │             │             │
    │ [pass]      │ [pass]      │ [pass]      │ [pass]
    │             │             │             │
    └─────────────┴──────┬──────┴─────────────┘
                         │
                         ▼
                  module/user-auth
                         │
                         │ [all tasks complete, module tests pass]
                         │
                         ▼
                       main
                         │
                         │ [tag: module-user-auth-v1]
                         │
                       DONE
```

### Why Three Levels? Why Not Just task → main?

---

### 1. Token Tracking Per Task

**Each task branch = clean accounting unit.**

```
task/T001
├── Commit 1: Initial implementation (tokens_in: 500, tokens_out: 800)
├── Commit 2: Fix after review (tokens_in: 200, tokens_out: 300)
└── Final: merged to module

Total: tokens_in: 700, tokens_out: 1100, total: 1800
Model: kimi-k2.5
Attempts: 2
Success: Yes (after 1 revision)
```

**Without branch-per-task:**
- Tokens get mixed across multiple tasks on same branch
- Hard to attribute costs to specific work
- Retry costs get lost in noise

**With branch-per-task:**
- Every task has exact token count
- Retry costs visible (multiple commits = multiple attempts)
- Per-task ROI calculation is trivial
- Model performance per task type is trackable

---

### 2. Model Performance Tracking

**We learn which models excel at which task types.**

```sql
-- With branch-per-task, this query is trivial:
SELECT 
  model_id,
  task_type,
  COUNT(*) as tasks_completed,
  AVG(tokens_total) as avg_tokens,
  AVG(attempts) as avg_attempts,
  SUM(CASE WHEN success THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as success_rate
FROM task_runs
GROUP BY model_id, task_type;
```

**Result:**
| Model | Task Type | Success Rate | Avg Tokens | Avg Attempts |
|-------|-----------|--------------|------------|--------------|
| kimi-k2.5 | code_generation | 95% | 12,000 | 1.1 |
| kimi-k2.5 | refactoring | 88% | 18,000 | 1.4 |
| deepseek-chat | code_generation | 75% | 15,000 | 1.8 |
| gemini-api | research | 92% | 8,000 | 1.0 |

**Without this data:**
- Can't make informed routing decisions
- Can't justify subscription renewals
- Flying blind on model selection

---

### 3. Security & Attack Surface

**Runner isolation prevents cascade damage.**

| Attack Scenario | With Branch-Per-Task | Without |
|-----------------|----------------------|---------|
| Runner goes rogue, deletes files | Damage isolated to task branch | Damage to main, all work affected |
| Runner injects malicious code | Caught at task review, never reaches main | Directly in main, harder to detect |
| Runner modifies unrelated files | Supervisor sees diff, rejects | Files already modified in main |
| Prompt injection tries to read secrets | Runner has no vault access, no git access | If runner has git, could exfiltrate |

**Principle:** Every agent should have minimum necessary access. Runners produce code. That's it. They don't commit, merge, or access secrets.

---

### 4. Drift Prevention

**Each level catches what the level below missed.**

```
Task Level Review:
  - Did runner output match task packet?
  - Any extra files touched?
  - Any spaghetti code?
  - Any security issues?
  → Catches: Wrong output, scope creep, obvious bugs

Task Level Tests:
  - Do unit tests pass?
  - Is coverage adequate?
  - Edge cases handled?
  → Catches: Logic errors, missing edge cases

Module Level Review:
  - Do all tasks integrate correctly?
  - Any conflicts between tasks?
  - Module-level tests pass?
  → Catches: Integration issues, architectural drift

Main Level:
  - Does module fit with rest of system?
  - Any regressions in existing code?
  - Full test suite passes?
  → Catches: System-level issues, regressions
```

**Each level is a checkpoint.** Issues caught early cost 1% of what they cost in production.

---

### 5. Rollback Capability

**Every action is reversible.**

| Level | Rollback Method |
|-------|-----------------|
| Task branch | Delete branch, reassign task |
| Module branch | Reset to before merge, rework |
| Main | `git revert` merge commit, or reset |
| Tagged release | Checkout tag, redeploy |

**Without branches:**
- Bad code in main requires careful surgery
- Hard to isolate which commit caused issue
- Multiple tasks' work intertwined

**With branches:**
- Bad task? Delete branch, nothing affected
- Bad module? Reset module branch, tasks preserved
- Clear blame for every change

---

### 6. ROI Calculation

**Precise cost attribution per task, module, project.**

```
Task Level:
  T001: 1,800 tokens, kimi-k2.5, $0.00 (subscription)
  T002: 2,400 tokens, kimi-k2.5, $0.00 (subscription)
  T003: 1,200 tokens, deepseek-api, $0.001 (API)

Module Level (user-auth):
  Total: 5,400 tokens
  Actual cost: $0.001
  Theoretical cost (if all API): $0.008
  Savings: 87.5%

Project Level:
  Sum of all modules
  Subscription allocation
  Total ROI
```

**This enables:**
- Subscription renewal decisions backed by data
- Routing optimization based on actual costs
- Project budget tracking
- Model cost comparison

---

### 7. Audit Trail

**Every decision recorded, every action traceable.**

```
Task T001:
├── Created by: Planner
├── Approved by: Council (3/3)
├── Assigned to: kimi-k2.5 by Orchestrator
├── Branch created by: Maintenance (per Supervisor)
├── Code committed by: Maintenance (per Supervisor approval)
├── Tests run by: Code Tester
├── Tests passed: 15/15
├── Merged to module by: Maintenance (per Supervisor)
├── Branch deleted by: Maintenance
└── Tokens: 1,800 | Duration: 120s | Cost: $0.00
```

**Without branches:**
- Commits mixed together
- Hard to trace which model did what
- Approval chain unclear

**With branches:**
- Each task has complete history
- Model attribution is exact
- Approval chain is clear

---

### 8. Observability & Debugging

**When things go wrong, we know exactly where.**

```
Scenario: Bug found in production

Step 1: Find which module
  → git log --oneline --grep="user-auth"

Step 2: Find which task introduced bug
  → Module branch shows all task merges
  → Each task merge has commit hash

Step 3: Find which model produced buggy code
  → task_runs table, filter by task_id

Step 4: Find why it passed review
  → Supervisor logs, what checks passed

Step 5: Find why tests didn't catch it
  → Tester logs, which tests ran

Step 6: Fix and prevent
  → Add test case, update task packet, improve review
```

**All traceable because each task is isolated.**

---

### 9. Parallel Development

**Multiple tasks can run simultaneously without conflict.**

```
Time →

Orchestrator assigns:
  T001 → Runner A (branch: task/T001)
  T002 → Runner B (branch: task/T002)
  T003 → Runner C (branch: task/T003)

All execute in parallel, isolated branches.

When done:
  T001 passes → merge to module
  T002 passes → merge to module
  T003 fails → stays in branch, retry

Module not affected by T003 failure.
Other tasks continue unimpeded.
```

**Without branch-per-task:**
- Runners conflict on same branch
- Merge conflicts constantly
- One failure blocks everything

---

### 10. Quality Gates Enforced

**No code reaches main without passing ALL gates.**

```
Gate 1: Task Packet Match
  → Supervisor compares output to expected
  → Fail: Return to runner

Gate 2: Code Quality
  → No hardcoded secrets
  → Pattern consistency
  → Error handling present
  → Fail: Return to runner

Gate 3: Unit Tests
  → Tester runs tests
  → Coverage meets minimum
  → Fail: Return to runner

Gate 4: Module Integration
  → All tasks present
  → Integration tests pass
  → Fail: Fix specific task

Gate 5: System Tests
  → Full test suite on main
  → No regressions
  → Fail: Rollback module, fix

Gate 6: Human Approval (visual)
  → If UI task
  → Human reviews preview
  → Fail: Return to runner with feedback
```

**Each gate catches different issues. Together, comprehensive coverage.**

---

## Summary: Why Branch-Per-Task Matters

| Benefit | Without | With |
|---------|---------|------|
| Token tracking | Approximate, mixed | Exact per task |
| Model performance | Guesswork | Data-driven |
| Security | High attack surface | Minimal exposure |
| Drift prevention | Catches late | Catches at every level |
| Rollback | Complex, risky | Trivial, safe |
| ROI calculation | Estimated | Precise |
| Audit trail | Fragmented | Complete |
| Debugging | Needle in haystack | Direct path to cause |
| Parallel work | Blocked by conflicts | Independent streams |
| Quality enforcement | Easy to skip | Impossible to bypass |

**The overhead of branch-per-task is minimal. The protection is massive.**

This is not bureaucracy. This is engineering for a complex, secure, production system.

---

## Why This Separation Matters

### Security
- One agent with write access = smaller attack surface
- Maintenance has vault access, others don't
- Runner compromise can't touch git directly
- Each task isolated, damage contained

### Auditability
- Every git action has a clear approval chain
- Supervisor decision → Maintenance execution → logged result
- No agent doing both decide and execute
- Complete history per task

### Debugging
- When something goes wrong, clear who did what
- Supervisor said X, Maintenance did Y, result was Z
- No "Supervisor merged and deleted, not sure which step failed"
- Task isolation means problem isolation

### Token/ROI Tracking
- Each branch = one task = one accounting unit
- Clean commit history per task
- Easy to attribute tokens to specific work
- Model performance data accumulates

---

## Questions for Council

1. **Is the Supervisor → Maintenance command flow too chatty?**
   - Every git operation is a separate command
   - Could batch some operations?
   - Or is explicit better?

2. **Should Maintenance have ANY self-initiated actions?**
   - Current: Can do low-risk updates without approval
   - Proposed: Even low-risk goes through Supervisor?
   - Balance between speed and safety?

3. **What if Maintenance is down?**
   - Supervisor queues commands?
   - Tasks pile up until Maintenance returns?
   - Human intervention path?

4. **Runner isolation verification:**
   - How do we ensure runners CAN'T write files?
   - Sandbox enforcement?
   - Contract validation?

5. **Planner's git access:**
   - Currently has git in agents.json
   - Should it have read-only git (for history)?
   - Or just file_read for current state?

---

## Implementation Checklist (If Approved)

- [ ] Update `config/agents.json` - remove git from non-Maintenance agents
- [ ] Update `prompts/supervisor.md` - remove git operations, add Maintenance commands
- [ ] Update `prompts/maintenance.md` - add task execution duties
- [ ] Update `prompts/internal_cli.md` - remove git, return code only
- [ ] Update `prompts/courier.md` - ensure no git
- [ ] Update `prompts/planner.md` - clarify read-only
- [ ] Create Maintenance command schema
- [ ] Create Supervisor → Maintenance communication layer
- [ ] Test full flow with new roles

---

## Alternative Considered: Supervisor Keeps Git

**Argument:** Supervisor is already trusted, why add Maintenance in the middle?

**Counter-argument:**
- Supervisor's job is QUALITY, not execution
- Adding git operations splits focus
- Maintenance is designed for careful execution
- Separation of concerns is cleaner
- If Supervisor is compromised, limited blast radius

**Decision:** Prefer separation. Council to confirm.

---

## Document Status

- [ ] GLM-5 review complete
- [ ] Kimi review
- [ ] Kimi subagent review (optional, for thoroughness)
- [ ] Human approval
- [ ] Implementation

---

**Council Members: Please review and discuss in AGENT_CHAT.md**
