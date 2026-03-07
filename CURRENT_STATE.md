# VibePilot Current State
**Last Updated:** 2026-03-07 Session 62
**Status:** FLOW FIXED - All blocking bugs resolved, ready for testing

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**

Dashboard cannot use anon key for reads. Options:
  1. Embed dashboard in Go binary and serve from governor
  2. Use service role key with RLS policies
  3. Implement proper authentication

**Action required before April 6th.**

---

## ✅ Current Status: FLOW WORKING

**Plan creation works** - Planner creates plan file
**Plan review works** - Supervisor reviews plans  
**Task creation works** - Tasks created from approved plans (migration 068 applied)
**Task execution works** - Tasks run via CLI/API connectors
**Dashboard data flow** - Token counts and costs being recorded
**Realtime subscriptions** - Listening to all events (INSERT, UPDATE, DELETE)
**Git push** - Plan files now committed and pushed
**Router simplified** - Planner flags internal, else check courier then platform

---

## 🎯 What's Ready to Test

**Internal Flow (Current):**
1. Create PRD in `docs/prd/`
2. Push to GitHub
3. Governor detects via Supabase realtime
4. Planner creates plan (committed to GitHub)
5. Supervisor reviews plan
6. If approved, tasks created
7. Tasks routed to internal (kilo or gemini-api)
8. Tasks executed
9. Results committed

**Available Internal Models:**
- glm-5 via kilo CLI (subscription)
- gemini-2.5-flash via gemini-api (free tier with rate limits)

**Web/Courier Flow (Future):**
- No courier agent configured yet
- No browser automation yet
- All tasks route to internal for now

---

## ✅ Fixed Issues (Session 62)

### 1. Realtime now subscribes to all events ✅
**Location:** `governor/internal/realtime/client.go:187-191`
**Fix:** `SubscribeToTable` now calls `SubscribeToTableWithFilter(table, "*", "")`

### 2. Processing lock race condition ✅
**Location:** `governor/cmd/governor/handlers_plan.go:169-175`
**Fix:** Clear processing lock before status update, rely on realtime UPDATE event to trigger next step

### 3. Git push for plan files ✅
**Location:** `governor/cmd/governor/handlers_plan.go:135-152`
**Fix:** Added `git.CommitAndPush()` call after writing plan file
**New method:** `governor/internal/gitree/gitree.go:CommitAndPush()`

### 4. Task creation RPC ✅
**Location:** `docs/supabase-schema/068_fix_task_creation_rpc.sql`
**Fix:** Changed dependencies from UUID[] to JSONB, added max_attempts, fixed parameter order

### 5. Router simplified ✅
**Location:** `governor/internal/runtime/router.go:39-57`
**Logic:** Planner flags internal → internal, else check courier → platform → internal

---

## 📁 Recent Migrations

| Migration | Purpose | Status |
|-----------|---------|--------|
| 067 | Fix task type constraint | ✅ Applied |
| 066 | Fix RPC signatures | ✅ Applied |
| 064 | Update task assignment | ✅ Applied |
| 063 | Enable realtime | ✅ Applied |

---

## 🔄 Session History
- **Session 62:** Fixed all 3 blocking bugs (realtime, processing lock, git push)
- **Session 61:** Comprehensive audit, blocking bugs identified
- **Session 59:** Flow working, realtime issue identified
- **Session 58:** Flow working, schema issue identified
- **Session 57:** Realtime integration complete
- **Session 56:** Task execution working
