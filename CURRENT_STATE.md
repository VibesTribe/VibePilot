# VibePilot Current State
**Last Updated:** 2026-03-08 Session 67
**Status:** E2E FLOW WORKING

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**
Action required before April 6th.

---

## Session 67 Summary

**Fixes applied:**
1. Gemini API activated in connectors.json - now registered and available
2. Fixed dashboard status mapping:
   - `pending`/`available` → pending (awaiting deps or resources)
   - `in_progress`/`review`/`testing` → in_progress (actively working)
   - `approval` → supervisor_approval (human review for UI/UX, research, credit)
   - `failed`/`escalated` → pending (system retries, no human needed)
3. Fixed Go code to retry instead of escalate on parse errors
4. Fixed SupervisorDecision.issues JSON tag (`issues` not `issues_raw`)

**Human only reviews 3 things:**
1. Visual UI/UX changes
2. System researcher suggestions (after council review)
3. Paid API key out of credit

---

## Flow Status
```
PRD → Plan → Supervisor → Tasks → Router → Execute → Review → Test → Approval → Merge
 ✅     ✅        ✅         ✅       ✅        ✅        ✅      ✅       ✅        ✅
```

---

## Configuration
- **Active connectors:** kilo (cli), gemini-api (api)
- **Active models:** glm-5 (via kilo), gemini-2.5-flash (via gemini-api)
- **Concurrency:** 2 per module, 2 total

---

## Task Status Flow
```
pending → available → in_progress → review → testing → approval → merged
   ↑         ↑            ↓           ↓         ↓          ↓
   └─────────┴────────────┴───────────┴─────────┴──────────┘
                     (on failure, retry from available)
```

---

## Session History
- **67:** Gemini API activated, status mapping fixed, removed escalated status
- **66:** Fixed RPC allowlist for update_task_branch
- **65:** E2E flow testing, multiple fixes applied
- **64:** Fixed branch_name, commitOutput type
- **63:** Fixed duplicate RPCs, constraints
