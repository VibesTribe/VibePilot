# VibePilot Current State
**Last Updated:** 2026-03-08 Session 68
**Status:** FIXING MERGE FLOW

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**
Action required before April 6th.

---

## Session 68 Summary

**Fixes applied:**
1. Branch creation - now creates from source branch (main/module) instead of orphan
2. Merge - added `--allow-unrelated-histories` flag
3. Merge failure - now sets status to `merge_pending` (system retries, no human)
4. Task retry - deletes old branch and recreates clean
5. Prompt packet - falls back to task.result.prompt_packet if task_packets table empty
6. Disabled gemini-api connector (broken vault key)

**Human only reviews 3 things:**
1. Visual UI/UX changes
2. System researcher suggestions (after council review)
3. Paid API key out of credit

**All technical issues handled by system:**
- Merge failures → auto retry
- Parse errors → auto retry
- Model failures → auto retry with different model
- Branch issues → auto recreate

---

## Flow Status
```
PRD → Plan → Supervisor → Tasks → Router → Execute → Review → Test → Approval → Merge
 ✅     ✅        ✅         ✅       ✅        ✅        ✅      ✅       ✅        🔄
```

---

## Configuration
- **Active connectors:** kilo (cli)
- **Active models:** glm-5 (via kilo)
- **Concurrency:** 2 per module, 2 total

---

## Task Status Flow
```
pending → available → in_progress → review → testing → approval → merged
   ↑         ↑            ↓           ↓         ↓          ↓           ↓
   └─────────┴────────────┴───────────┴─────────┴──────────┘     merge_pending
                     (on failure, retry from available)              ↓
                                                               (system retries)
```

---

## Session History
- **68:** Branch creation from source, merge fixes, disabled gemini-api
- **67:** Gemini API activated, status mapping fixed, removed escalated status
- **66:** Fixed RPC allowlist for update_task_branch
- **65:** E2E flow testing, multiple fixes applied
- **64:** Fixed branch_name, commitOutput type
- **63:** Fixed duplicate RPCs, constraints
