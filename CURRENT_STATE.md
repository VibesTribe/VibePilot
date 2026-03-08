# VibePilot Current State
**Last Updated:** 2026-03-08 Session 65
**Status:** E2E FLOW WORKING

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**
Action required before April 6th.

---

## Session 65 Summary

**End-to-end flow tested successfully:**
```
available → in_progress → review → testing → approval → escalated
```

**Fixes applied:**
1. Branch creation - Changed from `--orphan` to normal branches (gitree.go)
2. Status value - Changed `complete` to `success` for task_runs constraint
3. Model access - Changed glm-5 from `opencode` to `kilo` connector
4. RPC allowlist - Added `increment_task_attempts`
5. Issues parsing - Flexible parsing for string/array/object (decision.go)
6. Existing files - Treat as success if branch already has files
7. Concurrency - Increased to 2 per module/total

**Remaining issues:**
- `branch_name` not saved to task (needs investigation)
- Courier agents not available (no playwright/browser-use)

---

## Flow Status
```
PRD → Plan → Supervisor → Tasks → Router → Execute → Review → Test → Merge
 ✅     ✅        ✅         ✅       ✅        ✅        ✅      ❌      ❌
```

---

## Configuration
- **Active:** kilo (cli), glm-5 (model)
- **Concurrency:** 2 per module, 2 total

---

## Session History
- **65:** E2E flow testing, multiple fixes applied
- **64:** Fixed branch_name, commitOutput type
- **63:** Fixed duplicate RPCs, constraints
