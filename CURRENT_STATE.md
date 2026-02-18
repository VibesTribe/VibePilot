# VibePilot Current State

**Required reading: FOUR files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/prd_v1.4.md`** - Complete system specification
3. **`docs/core_philosophy.md`** - Strategic mindset and inviolable principles
4. **`CHANGELOG.md`** - History, changes, rollback commands

**Read all four → Know everything → Do anything**

---

**Last Updated:** 2026-02-18 06:22 UTC
**Updated By:** GLM-5 (Session 13: Smart routing, ROI calculation)
**Known Good Commit:** `891aa2d7`
**Kimi Subscription:** $0.99/mo expires Feb 27 → $19/mo (9 DAYS LEFT - MAXIMIZE USAGE)

---

# WHAT IS VIBEPILOT

Sovereign AI execution engine. Human provides idea → VibePilot executes with zero drift.

**Core Principles (see docs/core_philosophy.md):**
- Zero vendor lock-in - everything swappable
- Modular & swappable - change one thing, nothing else breaks
- Exit ready - pack up, hand over to anyone
- Reversible - if it can't be undone, it can't be done
- Always improving - new ideas evaluated daily

**Core Rules:**
- All state in Supabase
- All code in GitHub
- All changes via config (zero code edits for swaps)
- Context isolation per agent (task agents see ONLY their task)
- Council for PRDs/plans (iterative), system updates (one-shot vote)
- Maintenance is ONLY agent that touches system files

---

# CURRENT STATUS

## What's Working ✅

| Component | Status | Notes |
|-----------|--------|-------|
| Orchestrator dispatch | ✅ Working | Dispatches to available runners based on DB status |
| Task status flow | ✅ Working | pending → available → in_progress → review |
| Planner | ✅ Working | Creates tasks with full prompt_packets |
| Dashboard research slice | ✅ Deployed | Daily Research, Inquiry Research, etc. |
| Dashboard prompt_packet | ✅ Deployed | Shows full task content |
| Runner selection | ✅ Working | Subscription > Free API > Paid API scoring |
| Task assignment display | ✅ Working | Dashboard shows model assigned |
| ROI calculation | ✅ Working | Automatic after task_run insert |
| Token tracking | ✅ Working | tokens_in, tokens_out, tokens_used |
| Cost tracking | ✅ Working | theoretical vs actual, savings calculated |

## What's Blocked ❌

None currently.

---

# ORCHESTRATOR TESTING STATUS

## Live Test Results (2026-02-18)

**What works:**
```
✓ Orchestrator finds available tasks
✓ Runner selection with database status (paused/active)
✓ Subscription priority (Kimi > Free API > Paid API)
✓ Web → Internal fallback when no couriers available
✓ Task dispatch to kimi-internal for web research
✓ task_runs insert with courier column
✓ ROI calculation triggered automatically
✓ Token tracking: tokens_in, tokens_out, tokens_used
```

## ROI Calculation

**Formula:**
- `theoretical` = (tokens_in/1000 × platform_input_rate) + (tokens_out/1000 × platform_output_rate)
- `actual` = $0 for subscriptions, API cost for paid
- `savings` = theoretical - actual

**Platform Rates (in platforms table):**
| Platform | Input/1K | Output/1K |
|----------|----------|-----------|
| chatgpt | $0.00015 | $0.00060 |
| claude | $0.00100 | $0.00500 |
| gemini | $0.00030 | $0.00250 |
| deepseek-api | $0.00028 | $0.00042 |
| moonshot (Kimi) | $0.00060 | $0.00250 |

## Column Mismatch Fixes Applied

| Column | Status | Action |
|--------|--------|--------|
| `test_type` | ✅ Fixed | Removed from supervisor.py |
| `duration_seconds` | ✅ Fixed | Removed from orchestrator.py |
| `tokens_total` → `tokens_used` | ✅ Fixed | Corrected column name |

---

# NEXT SESSION ACTION ITEMS

1. **Re-test orchestrator** - full task execution should now work
2. **Test Kimi as executor** - kimi-internal runner
3. **Verify dashboard** shows completed task with token counts
4. **Monitor ROI calculator** - confirm tokens_used populates correctly

---

# VIBEFLOW DASHBOARD (Reference)

**Live Dashboard (Supabase):** https://vibeflow-dashboard.vercel.app/

**GitHub Repo:** https://github.com/VibesTribe/vibeflow

**Note:** These URLs have been provided multiple times. Check here first before asking.

---

# SESSION NOTES

See `SESSION_NOTES_2026-02-18.md` for detailed testing notes.

---

# FILE STRUCTURE

```
vibepilot/
├── agents/
│   ├── planner.py (REWRITTEN - full implementation)
│   ├── supervisor.py (FIXED - removed test_type)
│   └── ...
├── core/
│   ├── orchestrator.py (FIXED - self.runners, duration_seconds; NEEDS - tokens_total)
│   └── ...
├── config/
│   ├── prompts/
│   │   └── planner.md (full planner spec)
│   ├── researcher_context.md (NEW - compressed context for researcher)
│   └── templates/
│       └── research_packet.json (NEW - task packet template)
├── runners/
│   ├── contract_runners.py (has token counting)
│   └── ...
├── docs/
│   ├── supabase-schema/ (NEW - all schemas in one place)
│   └── UPDATE_CONSIDERATIONS.md (research findings)
├── kimi_usage_log.md (NEW - track all Kimi usage)
├── SESSION_NOTES_2026-02-18.md (NEW - this session's notes)
└── ...
```

---

# GIT BRANCHING RULES (CRITICAL)

**Vercel auto-deploys from main. Breaking main = Breaking production.**

- Dashboard/UI changes → Feature branch first, human approves → merge to main
- Backend changes → Less risky, can go direct but prefer branches
- **The Rule:** "If it's the dashboard, never goes to main until approved by me."

---

# QUICK COMMANDS

| Command | Action |
|---------|--------|
| `cat SESSION_NOTES_2026-02-18.md` | This session's detailed notes |
| `git log --oneline -5` | Recent commits |
| `git diff origin/main` | Unpushed changes |
| `source venv/bin/activate && python -c "from core.orchestrator import ConcurrentOrchestrator"` | Test orchestrator import |

