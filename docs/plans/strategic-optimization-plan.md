# VibePilot Strategic Optimization Plan

**Date:** 2026-04-20  
**Status:** Draft, Ready for Review  
**Scope:** Orchestrator logic, dashboard alignment, courier completion, system researcher

---

## Current State Summary (Verified in Code)

### WORKING (confirmed in Go + Supabase)

| Component | Status | Evidence |
|-----------|--------|----------|
| PRD detection | DONE | Supabase Live triggers |
| Plan creation | DONE | Planner creates plan + tasks + packets |
| Supervisor review | DONE | Reads PRD + plan, validates, approves/escalates |
| Task claiming | DONE | Atomic claim_task RPC with model + routing_flag + reason |
| Model routing | DONE | SelectDestination with cascade, dual-envelope for couriers |
| task_runs | DONE | create_task_run writes all 15+ columns including costs |
| routing_flag | DONE | Written via claim_task, dashboard reads it |
| Learning system | DONE | record_model_success/failure + update_model_learning wired |
| Cost tracking | DONE | calculateCosts() + deduct_model_credit per task |
| Token counting | DONE | tokens_in + tokens_out per run, our-side counting |
| Rate limiting | DONE | Usage windows, connector shared limits, platform limits |
| Cooldown tracking | DONE | Persistent, survives restarts via LoadFromDatabase |
| Dashboard status model | DONE | Ready/Active/Cooldown/Credit/Issue rendering |
| Courier dispatch payload | DONE | Passes llm_provider, llm_model, api_key, web_platform_url |
| Courier dual-envelope | DONE | Router checks fueling model + web platform limits |
| Startup validation | DONE | Cascade model IDs validated against models.json |

### NOT YET DONE

| Component | Gap | Impact |
|-----------|-----|--------|
| E2E pipeline test | Never tested full chain PRD→merge | Can't trust the system until proven |
| Courier workflow (courier.yml) | Hardcoded gemini-2.0-flash, doesn't read dispatch payload | Couriers can't actually run |
| System researcher | Prompt exists, not wired to cron or Supabase | No automated landscape monitoring |
| Visual QA agent | Concept only, no implementation | No automated screenshot testing |
| Multi-key Gemini strategy | Research done, needs Google Cloud project creation | Single point of failure for couriers |
| SiliconFlow connector | Research done, no account or connector entry | Missing cheaper alternative |
| Gap analysis doc | Stale -- lists 3 "broken" things that are all fixed | Misleading documentation |

---

## Workstreams

### WORKSTREAM 1: E2E Pipeline Validation

**Why first:** Everything else is built but unproven. A clean E2E test validates all the working components together and reveals what we missed.

**Steps:**
1. Push a simple PRD (hello-world or status-endpoint level)
2. Governor detects it, planner creates plan
3. Supervisor approves plan
4. Orchestrator picks up task, routes to model
5. Model executes, output committed to branch
6. Supervisor reviews output
7. Tester validates
8. Human approves merge
9. Verify task_runs row has all fields populated
10. Verify dashboard reflects every state transition in real time

**Success criteria:** One complete PRD→merge cycle with visible dashboard updates at every phase.

**Risk:** Likely to find integration bugs. That's the point. Fix them, re-test.

---

### WORKSTREAM 2: Courier Agent Activation

**Dependencies:** Workstream 1 (need working pipeline first)

**2a. Fix courier.yml (GitHub Actions workflow)**

Current state: Hardcodes `gemini-2.0-flash` via `langchain_google_genai`, uses fake token counting, doesn't read dispatch payload.

Fix to:
- Read `llm_provider`, `llm_model`, `llm_api_key`, `web_platform_url` from `client_payload`
- Use langchain provider-agnostic: langchain-google-genai for Gemini, langchain-openai for OpenRouter/SiliconFlow
- Navigate to `web_platform_url` before starting task
- Extract chat_url from browser address bar after response
- Report tokens via tiktoken or langchain built-in
- Write output + chat_url to result file for governor to collect

**2b. Wire courier fueling cascade**

Config in models.json (no code changes):
- gemini-2.5-flash (primary, via Google API key)
- google/gemma-4-31b-it:free (fallback 1, via OpenRouter)
- google/gemma-4-26b-a4b-it:free (fallback 2, via OpenRouter)
- bytedance/ui-tars-1.5-7b (fallback 3, ultra-cheap, GUI specialist)

All marked `courier: true` + `vision` capability. Router's selectCourierModel() already filters for these.

**2c. Multi-project Gemini keys (user action)**

Create 4 Google Cloud projects for independent quota:
- vibepilot-couriers (courier fueling)
- vibepilot-researcher (daily research)
- vibepilot-visual-tester (QA screenshots)
- vibepilot-backup (spare)

Each gets its own API key → 60 RPM total, all free.

**2d. Add SiliconFlow connector (optional)**

New connector in connectors.json. GLM-4.1V-9B-Thinking free, Qwen-VL cheaper than OpenRouter. Account setup required ($1 min top-up).

---

### WORKSTREAM 3: System Researcher Automation

**Dependencies:** None (independent of pipeline)

**Current state:** Prompt exists at `prompts/daily_landscape_researcher.md`. Not wired.

**Steps:**
1. Create cron job (daily, off-peak hours)
2. Researcher runs via Hermes or direct API call
3. Checks: OpenRouter free model rotation, Groq model changes, NVIDIA NIM updates, Gemini model status, new free platforms, deprecated models
4. Posts findings to Supabase (new table: `research_findings` or reuse `orchestrator_events`)
5. Supervisor reviews findings:
   - Simple (new free model, deprecated model): supervisor handles directly, updates models.json
   - Complex (new platform, architecture change): supervisor escalates to council → human
6. Dashboard shows research feed in LearningFeed component

**Output format per finding:**
```
type: new_model | deprecated_model | new_platform | rate_limit_change | pricing_change
model_id: ...
provider: ...
details: ...
action_suggested: add_to_models | remove_from_models | update_limits | escalate
confidence: 0.0-1.0
```

---

### WORKSTREAM 4: Documentation Cleanup

**Why:** Stale docs are dangerous. The gap analysis says 3 things are broken that are all fixed. Anyone reading it would waste time re-fixing.

**Steps:**
1. Update ARCHITECTURE_GAP_ANALYSIS.md -- mark all 3 gaps as RESOLVED with commit refs
2. Update CURRENT_STATE.md -- reflect verified state from this session
3. Verify contract_registry.json matches actual RPC signatures
4. Archive completed plans (orchestrator-rate-limit-enhancement-plan.md, courier-fueling-strategy.md)

---

## What's NOT in This Plan (future)

These are tracked in TODO.md for after the above is done:

- Visual QA agent (uses same browser-use pattern as couriers)
- LogAct patterns (intent logging, safety voter, append-only events)
- JourneyKits patterns (95 kits, 20 mapped to gaps)
- Pre-execution design preview (UI mockup before coding)
- MCP server activation (no consumer yet)
- Headless + ethernet setup (infrastructure)
- .context/ async hooks (performance)

---

## Implementation Order

```
Workstream 4 (docs) ──────────── concurrent ──────────→ 
Workstream 1 (E2E test) ──→ Workstream 2 (couriers) ──→ Visual QA (future)
Workstream 3 (researcher) ──── concurrent ──────────────→ Council escalation
```

Workstream 1 and 3 can run in parallel. Workstream 2 depends on 1. Workstream 4 is always concurrent.

---

## Principles Maintained

1. **No hardcoding** -- Models, providers, platforms all from config files
2. **Dashboard is sacred** -- Every state change visible in real time
3. **Supabase + GitHub = truth** -- Go code serves both, doesn't fight either
4. **Free first, paid fallback** -- Cascade starts free, escalates to cheap
5. **Token counting is ours** -- Never trust external counts
6. **Config-driven swaps** -- New model = config edit, zero code changes
7. **Governor subservient to VibePilot** -- It executes, doesn't decide strategy
