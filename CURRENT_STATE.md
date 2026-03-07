# VibePilot Current State

**Last Updated:** 2026-03-07 Session 58
**Status:** GO REWRITE IN PROGRESS - STILL BROKEN

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**

This means:
- Dashboard cannot use anon key for reads
- Options:
  1. Embed dashboard in Go binary and serve from governor
  2. Use service role key with RLS policies
  3. Implement proper authentication

**Action required before April 6th.**

---

## 🔴 Current Status: BROKEN

The rewrite is incomplete and the flow is broken:

1. **Plan creation works** - Planner creates plan file
2. **Plan review fails** - "Plan already being processed" error
3. **No tasks created** - Flow stops at plan review
4. **Root cause unclear** - Processing lock not being cleared properly OR duplicate events

---

## What Was Done This Session

### Session 58 (2026-03-07)
- Rewrote `handlers_testing.go` - task testing flow
- Rewrote `handlers_council.go` - council review with parallel voting
- Rewrote `handlers_research.go` - research suggestion review
- Rewrote `handlers_maint.go` - maintenance command execution
- Changed realtime from INSERT-only to all events (then reverted)
- Changed plan flow to call runPlanReview directly after plan creation
- Build passes but flow is broken

### Issues Found
1. Realtime only subscribed to INSERT events (UPDATEs not triggering handlers)
2. Attempted fix: Call runPlanReview() directly from handlePlanCreated()
3. New issue: "Plan already being processed" - processing lock conflict
4. The defer clear_processing may not run before runPlanReview tries to claim

---

## 🔧 What Needs to Happen Next

### Option A: Fix the Processing Lock Issue
The problem: handlePlanCreated holds a processing lock, then calls runPlanReview which tries to get the same lock.

Fix: Clear the processing lock BEFORE calling runPlanReview, or don't use processing locks for the same-plan chain.

### Option B: Use UPDATE Events Properly
1. Subscribe to UPDATE events on plans table
2. When plan status changes to "review", trigger handlePlanReview
3. Keep processing locks separate for each handler

### Option C: Single Handler Chain
1. handlePlanCreated does everything: plan creation + review + task creation
2. No separate runPlanReview function
3. Single processing lock for entire flow

---

## 📊 Rewrite Status

| File | Status | Notes |
|------|--------|-------|
| validation.go | ✅ Rewritten | Task validation |
| router.go | ✅ Rewritten | Model routing |
| handlers_plan.go | ⚠️ Broken | Plan creation works, review broken |
| handlers_task.go | ✅ Rewritten | Task execution (untested) |
| handlers_testing.go | ✅ Rewritten | Testing flow (untested) |
| handlers_council.go | ✅ Rewritten | Council review (untested) |
| handlers_research.go | ✅ Rewritten | Research flow (untested) |
| handlers_maint.go | ✅ Rewritten | Maintenance (untested) |

**Build:** ✅ Passes
**Flow:** ❌ Broken at plan review

---

## 📚 Key Documentation

| Doc | Purpose |
|-----|---------|
| `VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md` | Read this FIRST every session |
| `docs/GO_REWRITE_SPEC.md` | Rewrite specification |
| `docs/GO_CODE_LOGIC_MAP.md` | Current code analysis |
| `docs/HOW_DASHBOARD_WORKS.md` | Dashboard data flow |

---

## 🔑 Supabase Access

```bash
# Credentials are in systemd override
sudo cat /etc/systemd/system/governor.service.d/override.conf

# Query Supabase
curl -s "https://qtpdzsinvifkgpxyxlaz.supabase.co/rest/v1/TABLE?select=*" \
  -H "apikey: SERVICE_KEY" \
  -H "Authorization: Bearer SERVICE_KEY"
```

---

## 🕐 Session History

### Session 58 (2026-03-07) - CURRENT
- Rewrote handlers_testing, council, research, maint
- Flow still broken - processing lock issue
- Need to fix plan → review → tasks chain

### Session 57 (2026-03-07)
- Rewrote handlers_task.go
- Build passes

### Session 56 (2026-03-07)
- Rewrote handlers_plan.go, router.go, validation.go
- Build passes

---

## Next Session Tasks

1. **Fix the plan review flow** - Processing lock conflict
2. **Get end-to-end test working** - PRD → Plan → Tasks → Execution
3. **Address Supabase anon key deprecation** - Before April 6th
4. **Test all handlers** - Currently only plan creation tested

---

## Remember

- **Read VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md first**
- **Dashboard is READ-ONLY** - Fix Go code, not dashboard
- **No webhooks** - Using Supabase Realtime
- **No hardcoding** - Everything in config files
- **GitHub = Code source of truth**
- **Supabase = State source of truth**
