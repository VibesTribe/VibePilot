# VibePilot Current State

**Last Updated:** 2026-03-06 Session 55
**Status:** GO CODE REWRITE REQUIRED

---

## ⚠️ CRITICAL: Go Code Is Broken

**Symptom:** Tasks show "active", "pending", "assigned" all at once. Never completes.

**Root Cause:** Go handlers have 4 critical bugs:
1. RPC allowlist check failing (even though RPCs are in list)
2. Git commit failing ("files must be an array")
3. Status logic wrong (sets "review" even on failure)
4. No error recovery (continues on errors)

**Time Analysis:**
- Plan creation: ~25 seconds ✅
- Supervisor review: ~22 seconds ✅
- Task creation: instant ✅
- Task execution: ~26 seconds ✅
- **Total: 73 seconds for hello world** (but never actually completes)

---

## 📋 The Fix: Complete Rewrite Spec

**READ THIS FIRST:** [`docs/GO_REWRITE_SPEC.md`](docs/GO_REWRITE_SPEC.md)

This is the **single source of truth** for the rewrite. Contains:
- Files to KEEP (15 files, no modifications)
- Files to REWRITE (8 files, ~1,600 lines)
- Complete database schema
- Exact event flows (5 events, step-by-step)
- Router logic (models vs destinations)
- Error handling rules
- Testing plan
- Success criteria

---

## 🎯 Next Action

**Start Rewrite Session 1:** validation.go + router.go

This fixes the foundation before touching handlers.

**Say:** "START REWRITE SESSION 1"

---

## 📊 Rewrite Sessions

| Session | Files | Lines | Time | Status |
|---------|-------|-------|------|--------|
| **1** | validation.go, router.go | ~350 | 1-2h | ⏳ READY |
| **2** | handlers_plan.go | ~400 | 1-2h | 🔒 Blocked on 1 |
| **3** | handlers_task.go | ~500 | 2-3h | 🔒 Blocked on 2 |
| **4** | handlers_testing, council, research | ~350 | 1-2h | 🔒 Blocked on 3 |

---

## 📚 Documentation Reference

### Primary Docs (Read These)

| Doc | Purpose |
|-----|---------|
| [`docs/GO_REWRITE_SPEC.md`](docs/GO_REWRITE_SPEC.md) | **REWRITE SPEC - Single source of truth** |
| [`docs/GO_CODE_LOGIC_MAP.md`](docs/GO_CODE_LOGIC_MAP.md) | Complete analysis of current broken code |
| [`VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md`](VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md) | Everything about VibePilot design |

### Supporting Docs

| Doc | Purpose |
|-----|---------|
| [`docs/DATA_FLOW_MAPPING.md`](docs/DATA_FLOW_MAPPING.md) | Dashboard → Supabase → Go mapping |
| [`docs/HOW_DASHBOARD_WORKS.md`](docs/HOW_DASHBOARD_WORKS.md) | Dashboard data flow |
| [`docs/SUPABASE_ACTUAL_STATE.md`](docs/SUPABASE_ACTUAL_STATE.md) | Current Supabase schema state |

---

## 🔍 What's Working

| Component | Status | Notes |
|-----------|--------|-------|
| GitHub webhooks | ✅ | Detects PRD pushes |
| Supabase realtime | ✅ | Receives INSERT events |
| Plan creation | ✅ | Planner works |
| Supervisor review | ✅ | Reads PRD + plan |
| Task creation | ✅ | Creates in available status |
| Model routing | ✅ | Selects glm-5 |
| CLI execution | ✅ | kilo works |
| Token extraction | ✅ | Gets tokens from output |

---

## ❌ What's Broken

| Component | Issue | Impact |
|-----------|-------|--------|
| Task assignment | RPC fails | Task never gets assigned_to |
| Git commit | Files parsing fails | Output not committed |
| Status updates | Wrong logic | Shows review even on failure |
| Error recovery | None | Task stuck forever |
| Learning system | Not wired | No self-improvement |

---

## 📈 Success Criteria (After Rewrite)

ALL of these must work:

1. ✅ PRD pushed → Plan created (< 30s)
2. ✅ Plan reviewed → Tasks created (< 30s)
3. ✅ Task available → Task assigned (< 5s)
4. ✅ Task assigned → Task in_progress (< 5s)
5. ✅ Task executed → Output committed (< 60s for hello world)
6. ✅ task_runs created with tokens
7. ✅ Dashboard shows correct status at each step
8. ✅ Errors trigger retry (status=available)
9. ✅ Learning system records success/failure
10. ✅ Dependencies block until parent completes

**Total time for hello world:** < 2 minutes end-to-end

---

## 🕐 Session History

### Session 55 (2026-03-06)
- Created complete rewrite specification
- Mapped all Go code logic
- Identified 4 critical bugs
- Decision: Full rewrite (Option C)

### Session 54 (2026-03-06)
- Documentation cleanup
- Dashboard status mapping
- Found task_runs broken

### Session 53 (2026-03-06)
- Routing fixes
- Migration 064

### Session 52 (2026-03-06)
- E2E flow verification

### Session 51 (2026-03-05)
- Database cleanup
- Connector fixes
