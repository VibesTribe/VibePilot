# Agent Prompt Review - Discrepancies and Fixes

**Date:** 2026-02-20  
**Reviewer:** Kimi (Internal Coding Agent)  
**Scope:** All prompts in `config/prompts/` vs role definitions in `docs/vibepilot_process.md` and `config/agents.json` v1.1

---

## Executive Summary

**Overall Assessment:** Prompts are well-written and capture the spirit of roles, BUT several have **critical discrepancies** with the newly established infrastructure (Session 16). Most issues are around git capabilities and the Supervisor/Maintenance command flow.

| Status | Count | Files |
|--------|-------|-------|
| ✅ Aligned | 6 | vibes, researcher, consultant, council, courier, tester_code |
| ⚠️ Minor Issues | 2 | orchestrator, internal_api |
| ❌ Critical Issues | 3 | supervisor, maintenance, internal_cli, planner |

---

## Critical Issues (Must Fix)

### 1. `prompts/supervisor.md` - OUTDATED (Pre-Session 16)

**Problem:** Still describes old flow where Supervisor "commands Maintenance" but describes it as if Supervisor has git access.

**Current (Wrong):**
```markdown
### Passed
1. Command Maintenance: "Merge task/T001 → module/feature"
2. Wait for merge confirmation
3. Command Maintenance: "Merge module/feature → main" (if module complete)
4. Mark task complete in Supabase
```

**Missing:**
- No mention of `maintenance_commands` table
- No mention of command queue pattern
- No mention of git_read capability for reviewing branches
- "Command Maintenance" is vague - should be "Insert command to queue"

**Fix Required:**
```markdown
## Git Operations (You Do NOT Execute)

You have **git_read** access to review branches, but you CANNOT write.

All git operations go through the `maintenance_commands` table:

### To Create Task Branch
```python
# You insert command, Maintenance executes
command_id = command_create_branch(
    task_id="T001",
    branch_name="task/T001-auth",
    base_branch="module/auth"
)
```

### To Merge (After Tests Pass)
```python
# Task → Module
command_merge_branch(
    task_id="T001",
    source="task/T001-auth",
    target="module/auth",
    delete_source=True
)

# Module → Main (requires human approval)
command_merge_branch(
    task_id="T001",
    source="module/auth",
    target="main",
    delete_source=True
)
```

### Command Status Tracking
- `wait_for_command(command_id)` - Block until done
- `get_command_status(command_id)` - Check status
- All commands logged in Supabase

## You Never
- Touch git directly (always use command queue)
- Merge to main without human approval
```

---

### 2. `prompts/maintenance.md` - NEEDS GITHUB SECTION

**Problem:** Describes git operations conceptually but not the actual implementation from Session 16.

**Current:** Good description of polling and commands, but missing:
- Actual GitHub integration details
- How `agents/maintenance.py` works
- The command execution flow
- Error handling for git failures

**Fix Required - Add Section:**
```markdown
## GitHub Integration

You execute commands via the `MaintenanceAgent` class in `agents/maintenance.py`:

### Command Execution Flow
1. Poll `maintenance_commands` table for `status='pending'`
2. Claim command atomically using `claim_next_command(agent_id)`
3. Validate command against `config/maintenance_commands.json` allowlist
4. Execute git operation
5. Report result back to table

### Commands You Handle
- `create_branch`: `git checkout -b {branch} && git push -u origin {branch}`
- `commit_code`: Write files → `git add . && git commit -m "{message}" && git push`
- `merge_branch`: `git checkout {target} && git merge {source}`
- `delete_branch`: `git branch -d {branch} && git push origin --delete {branch}`
- `tag_release`: Requires human approval

### Security Enforcement
```python
# Before ANY command:
- Validate branch name (no special chars)
- Check protected branches (main/master)
- Check forbidden patterns (force, rm -rf)
- Verify human approval for merge_to_main
```

### Error Handling
- Git conflict → Report failure, DON'T force
- Network error → Retry with backoff (max 3)
- Invalid command → Report failure with reason
```

---

### 3. `prompts/internal_cli.md` - WRONG GIT CAPABILITIES

**Problem:** Says "Git operations" in tools and describes creating branches/commits. This is WRONG per Session 16 architecture.

**Current (WRONG):**
```markdown
## Your Tools
- CLI execution (Kimi, OpenCode)
- File read/write
- Git operations
...
## Git Operations
After completing a task:
1. Create branch: `task/P1-T001-auth-module`
2. Commit changes with task_id in message
3. DO NOT push or merge (Supervisor handles that)
```

**This is completely wrong.** Internal CLI runners should:
1. Return code in response
2. NEVER touch git
3. NEVER create branches
4. NEVER commit

**Fix Required:**
```markdown
## Your Tools
- CLI execution (Kimi, OpenCode)
- File read (codebase context)

## What You Return
You return code ONLY. You do NOT:
- Create branches
- Commit code
- Push to git
- Touch filesystem directly

## Output Format
```json
{
  "task_id": "P1-T001",
  "status": "success|failed",
  "output": "Summary of what was done",
  "artifacts": {
    "files_created": [{"path": "src/auth.py", "content": "..."}],
    "files_modified": [{"path": "src/config.py", "content": "..."}]
  },
  "metadata": {...}
}
```

## You Never
- Create branches
- Commit code
- Push to git
- Modify files directly (return content instead)
```

---

### 4. `prompts/planner.md` - GIT_ACCESS UNCLEAR

**Problem:** Says "Git history" in "What You Have Access To" but agents.json says `git_read: true`. Need to clarify scope.

**Current:**
```markdown
## What You Have Access To
- The PRD (zero-ambiguity, fully specified)
- Codebase (read-only, to understand existing structure)
- Git history (to understand patterns)
- Tech stack specifications (from PRD)
```

**Issue:** "Git history" is vague. Per agents.json, Planner has `git_read: true` but not `git_write`.

**Fix Required:**
```markdown
## What You Have Access To
- The PRD (zero-ambiguity, fully specified)
- Codebase (read-only, to understand existing structure)
- Git read access (to review existing code patterns, NOT to modify)
- Tech stack specifications (from PRD)

## You Never
- Write to git (no branch creation, no commits)
- Modify codebase directly
- Execute code
```

---

## Minor Issues (Should Fix)

### 5. `prompts/orchestrator.md` - MISSING COUNCIL ROUTING

**Problem:** No mention of Council routing functionality added in Session 16.

**Fix Required - Add Section:**
```markdown
## Council Routing

When Supervisor requests Council review:

1. Check available models (need 1-3)
2. Assign lenses based on review type:
   - Project review: User Alignment, Architecture, Feasibility
   - System improvement: Architecture, Security, Integration, Reversibility
3. Route to models (parallel if 3 available, sequential if 1)
4. Aggregate votes and return to Supervisor

### Rate Limit Countdown
Show human when platforms refresh:
- "Gemini available in 4h 23m"
- "ChatGPT resets at 12:00 UTC"

Format: `_format_duration(seconds)` → "4h 23m"
```

---

### 6. `prompts/internal_api.md` - COST TRACKING OUTDATED

**Problem:** Mentions specific costs that will get stale. Should reference config instead.

**Current:**
```markdown
| API | Cost | Credit Status |
|-----|------|---------------|
| DeepSeek | $0.00014/1k in, $0.00028/1k out | $2.00 remaining |
| Gemini API | FREE | Unlimited (free tier) |
```

**Fix Required:**
```markdown
| API | Cost Source | Check Status Via |
|-----|-------------|------------------|
| DeepSeek | `config/models.json` | Supabase models table |
| Gemini API | `config/models.json` | Supabase models table |

## Cost Awareness
Check `models.credit_available` before executing. If insufficient:
1. Return status: "failed"
2. Include reason: "insufficient_credit"
3. Suggest alternative from orchestrator
```

---

## Files That Are ✅ Aligned

These prompts correctly match their role definitions:

| File | Why It Works |
|------|--------------|
| `vibes.md` | Correctly describes read-only access, consulting role, no execution |
| `researcher.md` | Correctly states "find only, don't implement" |
| `consultant.md` | Correctly focused on PRD generation, no system access |
| `council.md` | Correctly describes multi-model review, approval only |
| `courier.md` | Correctly states no codebase access, returns chat_url |
| `tester_code.md` | Correctly focused on validation, no fixes |

---

## Recommended Fix Priority

### Phase 1 (Critical - Blocks Testing)
1. `supervisor.md` - Must match new command queue pattern
2. `internal_cli.md` - Remove all git references (runners don't touch git)

### Phase 2 (Important - Clarity)
3. `maintenance.md` - Add GitHub integration specifics
4. `planner.md` - Clarify git_read scope

### Phase 3 (Nice to Have)
5. `orchestrator.md` - Add Council routing section
6. `internal_api.md` - Remove hardcoded costs

---

## Cross-File Consistency Check

| Concept | vibes | supervisor | maintenance | planner | council | Result |
|---------|-------|------------|-------------|---------|---------|--------|
| Git write | ❌ | ❌ | ✅ | ❌ | ❌ | ✅ Correct |
| Git read | ❌ | ✅ | ✅ | ✅ | ❌ | ✅ Correct |
| Decide | ❌ | ✅ | ❌ | ❌ | ✅ | ✅ Correct |
| Execute | ❌ | ❌ | ✅ | ❌ | ❌ | ✅ Correct |
| Command queue | ❌ | ✅ (inserts) | ✅ (polls) | ❌ | ❌ | ⚠️ Vague in supervisor.md |

**Issue:** supervisor.md describes "commanding Maintenance" but doesn't explain the queue mechanism.

---

## Summary of Required Changes

| File | Lines to Change | Type |
|------|-----------------|------|
| `supervisor.md` | ~40 lines | Major rewrite of git/command section |
| `internal_cli.md` | ~20 lines | Remove git operations section |
| `maintenance.md` | ~30 lines | Add GitHub integration details |
| `planner.md` | ~5 lines | Clarify git access scope |
| `orchestrator.md` | ~15 lines | Add Council routing section |
| `internal_api.md` | ~5 lines | Remove hardcoded costs |

**Total:** ~115 lines across 6 files

---

## Action Items

1. **Human Review:** Approve these changes?
2. **Implementation:** I can make these changes once approved
3. **Verification:** Run `tests/test_full_flow.py` after prompt updates

---

**End of Review**
