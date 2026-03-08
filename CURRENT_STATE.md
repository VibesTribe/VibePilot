# VibePilot Current State
**Last Updated:** 2026-03-08 Session 64
**Status:** IN PROGRESS - Testing end-to-end flow

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**

Dashboard cannot use anon key for reads. Options:
  1. Embed dashboard in Go binary and serve from governor
  2. Use service role key with RLS policies
  3. Implement proper authentication

**Action required before April 6th.**

---

## Current State

**Task T001 (Hello World):**
- Status: `available` (should be `review` or `testing`)
- Branch: `task/T001` EXISTS with correct `hello.go` file
- `tasks.branch_name` is NULL (migration 074 will fix)
- `tasks.result` has `prompt_packet` ✓

**What's Working:**
1. ✅ Plan creation from PRD
2. ✅ Supervisor approval
3. ✅ Task creation with prompt_packet
4. ✅ Router selecting kilo/glm-5
5. ✅ Branch creation
6. ✅ kilo executing and creating files
7. ❌ Status not progressing to review/testing
8. ❌ branch_name not saved to task

---

## Pending Migrations

| Migration | Purpose | Status |
|-----------|---------|--------|
| 074 | Save branch_name to task | ⏳ **NEEDS TO BE APPLIED** |
| 073 | Write prompt_packet to tasks.result | ✅ Applied |
| 071 | Fix task constraints | ✅ Applied |
| 070 | Fix vault column name | ✅ Applied |

**Apply 074:**
```
https://github.com/VibesTribe/VibePilot/blob/main/docs/supabase-schema/074_update_task_branch.sql
```

---

## Known Issues

1. **gemini-api disabled** - Vault uses Fernet encryption (Python), Go expects AES-GCM
   - Workaround: Using kilo/glm-5 only
   - Fix: Re-encrypt vault secrets with Go's AES-GCM format

2. **Infinite retry loop** - Error handling resets status to `available`
   - Fixed: Added attempt tracking, escalate after max attempts

3. **branch_name not saved** - Created branch but didn't update task
   - Fixed: Migration 074 adds `update_task_branch` RPC

---

## Configuration

**Concurrency:**
- `max_concurrent_per_module: 1`
- `max_concurrent_total: 1`
- Prevents kilo crashes

**Active Connectors:**
- kilo (CLI) - glm-5 via Z.ai subscription

**Disabled:**
- gemini-api (vault encryption mismatch)

---

## Flow Status

```
PRD → Plan → Supervisor → Tasks → Router → Execute → Review → Test → Merge
 ✅     ✅        ✅         ✅       ✅        ✅        ❌      ❌      ❌
```

**Broken:**
- Execute completes but status doesn't progress
- No supervisor review of output
- No testing phase
- No merge

---

## Session History
- **Session 64:** Fixed branch_name saving, commitOutput type, error handling
- **Session 63:** Fixed duplicate RPCs, constraints, dashboard data
- **Session 62:** Fixed realtime, processing lock
- **Session 61:** Comprehensive audit
