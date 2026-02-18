# Session Notes - 2026-02-18

## What We Were Testing

**Goal:** See orchestrator pick up research tasks, assign to Kimi, execute, and show on dashboard

**What We Did:**
1. Activated research task (DAILY-P1-T001) to `status = "available"`
2. Started orchestrator
3. Watched dashboard for live task execution

---

## What Worked ✅

| Component | Status |
|-----------|--------|
| Orchestrator finds available tasks | ✅ Working |
| Orchestrator dispatches to runners | ✅ Working |
| Runner selection (gemini, deepseek, glm-5) | ✅ Working |
| Dashboard shows task assignment | ✅ Working |
| Task status changes (available → in_progress) | ✅ Working |
| Planner creates tasks with prompt_packets | ✅ Working |
| Dashboard shows prompt_packet content | ✅ Working |
| Research slice shows on dashboard | ✅ Working |

**Evidence from logs:**
```
Dispatched a895cf51... to gemini-2.0-flash
Dispatched 22cd2ec2... to gemini-2.5-flash
Dispatched 20bd4715... to deepseek-chat
Dispatched e90d1ec8... to glm-5
```

**Dashboard shows:** Tasks moving from pending → in_progress with assigned model

---

## Issues Found and Fixed ✅

| Issue | Fix | File |
|-------|-----|------|
| `self.runners` undefined | Changed to `self.runner_pool.runners` | orchestrator.py line 669 |
| `test_type` column missing | Removed from route_to_testing | supervisor.py |
| `duration_seconds` column missing | Removed from task_runs insert | orchestrator.py line 791 |

**Commit:** `bb159240 Fix orchestrator and supervisor column errors`

---

## Issues Still To Fix ❌

| Issue | What's Wrong | Fix Needed |
|-------|--------------|------------|
| `tokens_total` column missing | Orchestrator tries to insert `tokens_total` but column doesn't exist | Remove line 790: `"tokens_total": result.get("tokens", 0)` from orchestrator.py |

**task_runs actual columns in Supabase:**
- actual_cost
- chat_url
- completed_at
- courier
- courier_cost_usd
- courier_model_id
- courier_tokens
- created_at
- error
- id
- model_id
- platform
- platform_theoretical_cost_usd
- result
- savings
- started_at
- status
- task_id
- theoretical_api_cost
- tokens_in ✅
- tokens_out ✅
- tokens_used ✅
- total_actual_cost_usd
- total_savings_usd
- updated_at

**NO tokens_total column exists**

---

## Token Counting Question

**API Models (DeepSeek, Gemini API):**
- API returns `usage.prompt_tokens` and `usage.completion_tokens`
- These ARE tracked in contract_runners.py
- Gets written to `tokens_in`, `tokens_out`, `tokens_used`

**Web Platforms (Courier):**
- Web platforms (ChatGPT web, Claude web) don't report tokens
- Options:
  - A) Estimate tokens from input/output text length (chars ÷ 4)
  - B) Leave blank (null) for web runs
  - C) Track courier_tokens separately (column exists: `courier_tokens`)

**Recommendation:** Leave blank for web, track for API. Dashboard can show "N/A" for web runs.

---

## Next Session Action Items

1. **Remove `tokens_total` from orchestrator.py line 790** - causes task_runs insert to fail
   ```python
   # FIND AND REMOVE THIS LINE:
   "tokens_total": result.get("tokens", 0),
   ```

2. **Commit and push to GitHub**

3. **Re-test orchestrator** - should complete full task execution

4. **Verify on dashboard** - task goes available → in_progress → review

5. **Kimi execution** - Test with kimi-internal runner specifically

---

## Dashboard Status

- Shows "Daily Research" slice ✅
- Shows prompt_packet content ✅
- Shows tasks being assigned to models ✅
- Waiting on: Full task completion (blocked by tokens_total error)

---

## Commits This Session

| Commit | Description |
|--------|-------------|
| `bb159240` | Fix orchestrator and supervisor column errors |
| `f772b28` | Add prompt_packet validation and auto-generation |
| `decede19` | Rewrite Planner agent to full design specification |
| (vibeflow) | Research slices + prompt_packet mapping to dashboard |

---

## Files Still Need Fixing

```
vibepilot/core/orchestrator.py
  Line ~790: Remove "tokens_total" from task_runs insert
```
