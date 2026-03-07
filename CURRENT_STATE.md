# VibePilot Current State
**Last Updated:** 2026-03-07 Session 63
**Status:** BLOCKED - Migration 069 needs to be applied to Supabase

---

## ⚠️ CRITICAL: Supabase Anon Key Deprecation

**Supabase will disable all anon keys by April 6th, 2026.**

Dashboard cannot use anon key for reads. Options:
  1. Embed dashboard in Go binary and serve from governor
  2. Use service role key with RLS policies
  3. Implement proper authentication

**Action required before April 6th.**

---

## 🚫 Current Blocker: Duplicate RPC Function

**Problem:** Two versions of `create_task_with_packet` exist in Supabase:
- Old version (pre-068) with wrong parameter order
- New version (068) with correct parameter order

Supabase returns error: "Could not choose the best candidate function"

**Result:** Tasks are NOT being created from approved plans.

**Fix:** Migration 069 on GitHub - needs to be applied in Supabase SQL Editor
https://github.com/VibesTribe/VibePilot/blob/main/docs/supabase-schema/069_fix_duplicate_functions_and_missing_rpcs.sql

---

## 🔧 After Applying Migration 069

The following will be fixed:
1. ✅ Duplicate `create_task_with_packet` dropped
2. ✅ `check_platform_availability` RPC added (router needs this)
3. ✅ `get_vault_secret` RPC added (for GEMINI_API_KEY access)

---

## 📋 Configuration Status

**Concurrency (already correct):**
- `max_concurrent_per_module: 1`
- `max_concurrent_total: 1`
- `kilo limit: 1`

This prevents multiple kilo sessions from crashing.

---

## 🎯 What Should Work After Migration 069

**Internal Flow:**
1. Create PRD in `docs/prd/`
2. Push to GitHub
3. Governor detects via Supabase realtime
4. Planner creates plan (committed to GitHub)
5. Supervisor reviews plan
6. If approved, tasks created ← **THIS IS BROKEN NOW**
7. Tasks routed to internal (kilo only - gemini-api needs vault fix)
8. Tasks executed
9. Results committed

**Available Internal Models:**
- glm-5 via kilo CLI (subscription) ← **WORKING**
- gemini-2.5-flash via gemini-api ← **NEEDS GEMINI_API_KEY IN VAULT**

---

## 📁 Recent Migrations

| Migration | Purpose | Status |
|-----------|---------|--------|
| 069 | Fix duplicate function, add missing RPCs | ⏳ **NEEDS TO BE APPLIED** |
| 068 | Fix task creation RPC | ✅ Applied (but duplicate exists) |
| 067 | Fix task type constraint | ✅ Applied |
| 066 | Fix RPC signatures | ✅ Applied |
| 063 | Enable realtime | ✅ Applied |

---

## 🔄 Session History
- **Session 63:** Identified duplicate RPC function blocker, created migration 069
- **Session 62:** Fixed realtime, processing lock, git push (but task creation still broken)
- **Session 61:** Comprehensive audit, blocking bugs identified
- **Session 59:** Flow working, realtime issue identified
