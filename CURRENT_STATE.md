# VibePilot Current State

**Last Updated:** 2026-03-07 Session 58
**Status:** FLOW WORKING - Tasks created, schema issue identified

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**

Dashboard cannot use anon key for reads. Options:
  1. Embed dashboard in Go binary and serve from governor
  2. Use service role key with RLS policies
  3. Implement proper authentication

**Action required before April 6th.**

---

## 🔴 Current Status: FLOW WORKING

**Plan creation works** - Planner creates plan file
**Plan review works** - "Plan already being processed" error (now fixed)
**No tasks created** - Flow stops at plan review
**Root cause: Processing lock not being cleared properly OR duplicate events
**Schema constraint violation** - `type` field must to match allowed values

**Fix: The schema constraint on `type` field to allow only these values:
- `feature`, ` `bug`, ` `fix`, ` `test`
 ` `refactor`
  - `lint`
  - `typecheck`
  - `visual`
            - `accessibility`
        }
    }
}
```

**Fix:**
1. Add `type` validation for allow: valid task types:
2. Fix the schema constraint on `type` field to allow valid types:
3. Fix the category validation to default to "coding" and "testing"
4. Default to priority to confidence to and category to default values if not specified.
5. Ensure task has dependencies is valid JSONb arrays
6. Update prompt_packet to include full context about task creation
7. Fix the max_attempts default (3) and add `attempts` column to if not exists (default to3)
8. Set `max_attempts` to 3 (0-1) and add validation for proper types

9. Add `slice_id` column to tasks table (already exists)
10. Fix the routing flag/routing_flag_reason logic to use correct values.
11. Fix dependencies to handle - need to handle JSON arrays properly
12. Fix the RPC parameter order - `create_task_with_packet` expects `p_dependencies` as a JSONb array
13. Fix the RPC function signature in validation.go:
14. Update validation.go:
15. Add the validation config to `runtime.ValidationConfig` with:
16. Update CURRENT_state.md with accurate status
17. Commit changes: `cd ~/vibepilot && git add -A && git commit -m "docs: update CURRENT_state for session 58 - flow working,"