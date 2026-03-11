# VibePilot Current State
**Last Updated:** 2026-03-11 Session 81 End (21:50 UTC)
**Status:** BROKEN - Prompts missing from task branches

---

## ROOT CAUSE FOUND

**The Problem:**
1. Governor binary runs from current git working directory
2. When task executes, git operations switch to `task/T001` branch
3. `task/T001` branch does NOT have `prompts/` directory
4. Supervisor fails: "supervisor.md: no such file or directory"
5. Task stuck in `review` forever

**Evidence:**
```
Mar 11 21:46:45 governor[1853708]: [TaskReview] Session error for 3eacc447: load prompt: read prompt /home/mjlockboxsocial/vibepilot/prompts/supervisor.md: open /home/mjlockboxsocial/vibepilot/prompts/supervisor.md: no such file or directory
```

**Why prompts disappear:**
- `prompts/` exists on `main` branch
- When task branch is created/checked out, prompts/ is not included
- Governor looks for prompts in working directory which is now on task branch

---

## FIX NEEDED

Either:
1. Prompts should be embedded in governor binary (not read from filesystem)
2. Or prompts path should be absolute and outside git repo
3. Or task branches should include prompts directory

---

## CURRENT STUCK STATE

- Task T001: `status=review`, `processing_by=supervisor:...` (stale)
- Tokens used: Yes (task executed successfully)
- File created: Yes (`governor/cmd/tools/hello.go` exists)
- Stuck because: Supervisor can't run without prompt file

---

## NEXT SESSION

1. Decide on fix approach (embed prompts, absolute path, or include in branches)
2. Implement fix
3. Clean everything
4. Test fresh

---

## TIMELINE FROM CLEAN TEST

| Time | Event | Duration |
|------|-------|----------|
| 21:42:38 | PRD detected, plan created | 0s |
| 21:43:00 | Plan written to file | 22s |
| 21:43:30 | Supervisor approved plan | 30s |
| 21:43:30 | Task T001 created | 0s |
| 21:43:30 | Task claimed by glm-5 | 0s |
| 21:46:44 | Task → review | ~3min execution |
| 21:46:45 | **FAIL: supervisor.md not found** | - |

The flow WORKS until supervisor review. The only issue is the missing prompt file.

---

## DO NOT DO NEXT SESSION

- DO NOT manually call RPCs
- DO NOT manually PATCH database
- DO NOT rebuild without understanding why
- DO NOT add more complexity
- DO NOT "test" individual parts

## DO NEXT SESSION

1. Fix the prompts path issue (pick one approach)
2. Clean everything
3. One test
4. Watch logs
5. Verify completion
