# VibePilot Changelog

**Purpose:** Full audit trail of all changes. Anyone/any agent can see what, where, when, why.

**Update Frequency:** After EVERY change (file add, update, remove, merge, branch delete)

---

# 2026-02-17 (Session 8)

## Session Summary

### Major Work

1. **ROI Calculator v1** — Full ROI tracking in dashboard
   - Enhanced RoiPanel with real data from Supabase
   - USD/CAD toggle with live exchange rate fetch
   - Slice-level ROI breakdown (clickable to show tasks)
   - Model-level ROI breakdown (clickable to show tasks per model)
   - Subscription tracking with renewal recommendations

2. **Schema v1.4** — Enhanced ROI fields
   - `tokens_in` / `tokens_out` (split for accurate costing)
   - `courier_model_id`, `courier_tokens`, `courier_cost_usd`
   - `platform_theoretical_cost_usd`, `total_actual_cost_usd`, `total_savings_usd`
   - Subscription fields on models (cost, start/end dates, status)
   - `slice_roi` view for slice-level rollup
   - `get_subscription_roi()` and `get_full_roi_report()` functions
   - `exchange_rates` table for CAD conversion

3. **Model ROI Calculation** — Calculate ROI per model
   - Models track: executor runs, courier runs, or both
   - Each model shows: total tokens, theoretical cost, actual cost, savings
   - Click to expand: all task runs with that model's contribution

### Cost Model

| Access Type | Actual Cost | Theoretical Cost |
|-------------|-------------|------------------|
| Free API | $0 | tokens × API rate |
| Pay-per-use API | tokens × rate | tokens × rate (same) |
| CLI Subscription | prorated (sub_cost / days × days_used) | tokens × equivalent API rate |

### Files Created/Updated

```
vibepilot/
├── docs/schema_v1.4_roi_enhanced.sql (NEW)
│   - task_runs: tokens_in/out, courier fields, cost fields
│   - models: subscription tracking, input/output cost split
│   - platforms: theoretical cost fields
│   - slice_roi view
│   - calculate_enhanced_task_roi(), get_subscription_roi()
│   - exchange_rates table

vibeflow/apps/dashboard/
├── lib/roiCalculator.ts (NEW)
│   - Exchange rate fetch (Supabase → exchangerate-api fallback)
│   - Currency formatting utilities
│   - ROI aggregation helpers
│
├── lib/vibepilotAdapter.ts (UPDATED)
│   - VibePilotTaskRun: added ROI fields
│   - VibePilotModel: added subscription fields
│   - VibePilotPlatform: added cost fields
│   - ROI types: SliceROI, SubscriptionROI, ProjectROI, ROITotals, ModelROI, TaskRunROI
│   - calculateROI(), calculateSliceROI(), calculateSubscriptionROI(), calculateModelROI()
│
├── hooks/useMissionData.ts (UPDATED)
│   - Exposes roi data from adapter (includes ModelROI)
│
└── components/modals/MissionModals.tsx (UPDATED)
    - RoiPanel: enhanced with real data
    - USD/CAD toggle
    - By Slice breakdown (clickable to expand)
    - By Model breakdown (clickable to show tasks)
    - Subscription tracking with recommendations
    - Removed 404 link to roi-calculator.html

├── styles.css (UPDATED)
│   - New styles for model list, task list, expand icons
```

### Remaining Work

- Add cost/savings to model cards in main dashboard
- Admin Panel forms → Supabase
- Vibes → Maintenance handoff
- Test ROI with real task runs

---

# 2026-02-16/17 (Session 7)

## Session Summary

### Major Architectural Decisions

1. **Slices First** — Planner now outputs modular vertical slices (not just phases)
2. **Routing Flags** — Tasks get Q/W/M based on complexity, not destination
3. **Dashboard Contract** — Mock data shape is sacred, adapter transforms VibePilot → Dashboard
4. **Lamp Metaphor** — Agent = lamp (swappable shade/bulb/base/outlet)
5. **Supabase is Runtime Truth** — JSON files are backup/seed, Supabase is live source
6. **80% Cooldown** — Models/platforms pause at 80% usage with countdown timer

### Routing Flag Thresholds

| Condition | Flag | Why |
|-----------|------|-----|
| 0-1 dependencies | W | Safe for web free tier |
| 2+ dependencies | Q | Internal required |
| Touches existing file | Q | Needs codebase |
| Touches 2+ existing files | RED FLAG | Council audit |

### Files Created/Updated

```
docs/
├── schema_v1.1_routing.sql (NEW)
│   - Adds: slice_id, phase, task_number, routing_flag, routing_flag_reason
│   - Functions: get_tasks_by_slice, get_slice_summary, get_available_for_routing
│
├── schema_v1.2_platforms.sql (NEW)
│   - platforms table + display columns
│   - get_dashboard_agents() function
│
├── schema_v1.2.1_platforms_fix.sql (NEW)
│   - Adds missing columns to existing platforms table
│
└── schema_v1.3_config_jsonb.sql (NEW)
    - config JSONB column on models/platforms (stores full config)
    - Live stats: tokens_used, success_rate, last_run_at
    - cooldown_expires_at for 80% tracking
    - skills, prompts, tools tables
    - Functions: update_model_stats, update_platform_stats

config/prompts/
└── planner.md (MAJOR UPDATE)
    - Isolation-first principle
    - Slice-based output structure
    - Routing flags with thresholds (2+ deps = Q)
    - Multi-file red flag escalation
    - Dependency types with slice boundary rules

core/
└── orchestrator.py (MAJOR UPDATE)
    - CooldownManager: tracks cooldowns per runner
    - UsageTracker: monitors requests/tokens, triggers cooldown at 80%
    - RunnerPool checks cooldown before assigning tasks
    - routing_capability per runner (CLI/API = internal+web+mcp)

scripts/
└── sync_config_to_supabase.py (REWRITTEN)
    - Bidirectional: import (JSON→DB) and export (DB→JSON)
    - Stores full config in JSONB column
    - Handles skills, tools, prompts tables

tests/
└── test_routing_logic.py (NEW)
    - Verifies routing flag thresholds
    - Tests orchestrator task filtering
    - Tests slice grouping

CURRENT_STATE.md (MAJOR UPDATE)
    - Full documentation of new architecture
    - Vibeflow dashboard section
    - Config sync workflow
    - New key decisions (DEC-017 to DEC-021)
```

### Vibeflow Dashboard (Connected)

```
~/vibeflow/apps/dashboard/
├── lib/
│   ├── supabase.ts (NEW) — Supabase client with config check
│   └── vibepilotAdapter.ts (NEW) — Transforms Supabase → Dashboard shape
│       - Reads config JSONB for full model/platform details
│       - Shows cooldown countdown from cooldown_expires_at
│       - Completed tasks have no owner (vanish from orbit)
│
├── hooks/
│   └── useMissionData.ts (UPDATED)
│       - Queries tasks, task_runs, models, platforms from Supabase
│       - Falls back to mock data if not configured
│
└── .env.example (NEW)
    - VITE_SUPABASE_URL, VITE_SUPABASE_ANON_KEY
```

**Live at:** https://vibeflow-dashboard.vercel.app/

### What Was Done Manually

1. Run schema v1.1, v1.2.1, v1.3 on Supabase
2. Add VITE_SUPABASE_URL and VITE_SUPABASE_ANON_KEY to Vercel environment

### Completed

- [x] Run schema migrations on Supabase (v1.1, v1.2.1, v1.3)
- [x] E2E test with new routing logic
- [x] Vibeflow adapter (Supabase → Dashboard shape)
- [x] Dashboard connected to live Supabase
- [x] Completed tasks vanish from orbit
- [x] Full config in JSONB (rate limits, context limits, notes)
- [x] 80% usage tracking with cooldown
- [x] Cooldown countdown in dashboard

### Remaining Work

- [ ] Wire Admin Panel forms to Supabase
- [ ] Wire Vibes → Maintenance for "add X" requests
- [ ] Test cooldown with real usage

---

# 2026-02-16 (Session 6)

## Session Summary

### What Was Fixed
- Removed langchain-google-genai bloat (installed without asking)
- Courier runner now model-agnostic (LLM from config, swap = one param)

### What Was Researched
- 10 web AI platforms researched for rate limits, context limits, auth, API pricing
- 6 usable (Gmail auth): Gemini, Claude, ChatGPT, Copilot, DeepSeek, HuggingChat
- 4 blocked (Chinese phone/Alipay): Kimi web, GLM, Qwen, Minimax

### What Was Clarified
- **Q** = Quality internal (supervisor, testing, review)
- **W** = Web courier (browser automation)
- **M** = MCP-connected (future, for external IDE/CLI)
- Kimi CLI and OpenCode are **internal**, not M-tier

### Files Updated
```
config/
├── models.json (v1.1) - 4 models we HAVE (2 API, 2 CLI)
└── platforms.json (v2.0) - 6 usable platforms with full limits/pricing

runners/
└── contract_runners.py - Courier is model-agnostic

docs/
└── vibeflow_dashboard_analysis.md - Dashboard structure documented
```

### Vibeflow Work Started
- Cloned ~/vibeflow from github.com/VibesTribe/vibeflow
- Created branch `feature/vibepilot-supabase` from `feature/admin-control-center-ui`
- Installed @supabase/supabase-js
- Created apps/dashboard/lib/supabase.ts
- Started modifying useMissionData.ts (in progress, NOT complete)

**Confirmed baseline:** https://vibeflow-dashboard-git-feature-admi-1e8c37-vibestribes-projects.vercel.app/

### Visual Testing Workflow (Discussed, Not Built)
- Visual test agent visits Vercel preview
- Tests layout, style, functionality against task context
- Pass → Review queue for human
- Fail → Back to dev agent

### Next Session
1. Complete Supabase connection in Vibeflow useMissionData.ts
2. Push branch to GitHub → Vercel preview
3. Human reviews before any merge
4. DO NOT BREAK FRONTEND

---

# 2026-02-16 (Session 5)

## Session Summary

### What Was Fixed (Vendor Lock-in)
**Problem:** Courier runner had Gemini hardcoded, langchain bloat installed without asking.

**Solution:**
1. Removed langchain-google-genai, langchain-core, langsmith (bloat)
2. Rewrote courier runner to be model-agnostic
3. LLM for browser-use now comes from config/models.json

**Swap Example:**
```python
# Before: hardcoded Gemini
# After: read from config
CourierContractRunner(platform='gemini', llm_model_id='gemini-api')  # or 'deepseek-chat'
```

### What Was Researched
**Platform Registry - 10 platforms researched for:**
- Rate limits (per minute, hour, day)
- Context limits (free tier, not paid)
- Attachment penalties (ChatGPT = 1/10th limits!)
- Auth methods (Gmail OK? Chinese phone = out)
- API pricing (for virtual ROI calculator)

**Results:**
- 6 USABLE: Gemini, Claude, ChatGPT, Copilot, DeepSeek, HuggingChat
- 4 NOT USABLE: Kimi web, GLM, Qwen, Minimax (require Chinese phone/Alipay)

### Files Updated
```
config/
├── models.json (v1.1) - Only what we HAVE (4 models: 2 API, 2 CLI)
└── platforms.json (v2.0) - Where we GO (6 usable, 4 blocked, full limits/pricing)

runners/
└── contract_runners.py - Courier now model-agnostic, factory pattern

CURRENT_STATE.md - Added Models vs Platforms section
CHANGELOG.md - This entry
```

### Dependencies Removed
- langchain-google-genai (bloat)
- langchain-core (bloat)
- langsmith (bloat)

### Key Decisions
1. **Models ≠ Platforms** - models.json is what we have, platforms.json is where we go
2. **Gmail-only auth** - Platforms requiring Chinese phone/Alipay are blocked
3. **80% limit policy** - Orchestrator will pause platforms at 80% of limits
4. **Cheapest API = DeepSeek** - $0.28/1M input, $0.42/1M output
5. **Best free tier = Gemini** - 1M tokens/day, 1M context

### Platform Quick Reference
| Platform | Context | Free Limits | API $/1M | Auth |
|----------|---------|-------------|----------|------|
| Gemini | 1M | 1500/day, 1M tok/day | $0.30/$2.50 | Gmail |
| Claude | 200K | ~10-20/day | $1.00/$5.00 | Gmail |
| ChatGPT | 128K | 10/3hr ⚠️attach=1/10 | $0.15/$0.60 | Gmail |
| Copilot | 128K | 30/sess unlimited | $2.50/$10.00 | Gmail |
| DeepSeek | 64K | Generous | $0.28/$0.42 | Gmail |
| HuggingChat | Varies | Varies | Free | Optional |

### Next Session
1. Wire orchestrator to use platforms.json for routing
2. Implement 80% limit tracking with auto-pause/resume
3. Test courier with headless browser

---

# 2026-02-16 (Session 4)

## Session Summary

### What Was Built
**Dashboard + Pipeline Test:**
- `dashboard/terminal_dashboard.py` - Terminal-based real-time dashboard
- `tests/test_pipeline.py` - End-to-end pipeline test

**Courier Runner Enhanced:**
- `runners/contract_runners.py` - Added browser-use integration with Gemini adapter

### What Was Tested
```
Pipeline Test:
1. Created task in Supabase
2. Created task packet
3. Dispatched to Kimi runner
4. Result: success in 15.31s, 49 tokens
5. Task marked merged
6. Task run logged

✓ PIPELINE TEST PASSED
```

### Dashboard Features
- Task status counts
- Active tasks list
- Recent runs with success/fail icons
- Available models from config
- `--watch` mode for auto-refresh

### Files Created
```
dashboard/
└── terminal_dashboard.py

tests/
└── test_pipeline.py
```

### Files Updated
- `runners/contract_runners.py` - Courier runner with browser-use
- `core/orchestrator.py` - Uses contract runners, loads from config
- `config/models.json` - Added browser-use-gemini model
- `CURRENT_STATE.md` - Updated components

### Dependencies Added
- browser-use (browser automation)
- google-genai (Google AI SDK)

**NOTE:** langchain-google-genai was added here but removed in Session 5 (bloat).

### Next Session
1. Web-based dashboard (React/Vibeflow)
2. Courier runner browser test (needs display)
3. Voice interface (Deepgram + Kokoro)

---

# 2026-02-16 (Session 3)

## Session Summary

### What Was Built
**Contract Runners (Complete):**
- `runners/base_runner.py` - Abstract base class enforcing RUNNER_INTERFACE contract
- `runners/contract_runners.py` - Kimi, DeepSeek, Gemini, Courier runners following interface
- `core/config_loader.py` - Central module for loading all JSON configs
- `tests/test_contract_e2e.py` - End-to-end test for contract layer

**Orchestrator Integration:**
- RunnerPool now loads from config/models.json (not database)
- _call_runner uses contract runners with proper task packet format
- All 9 runners loaded and validated

### What Was Tested
```
=== Testing Config Loader ===
✓ Config valid (13 skills, 10 tools, 9 models, 12 agents, 4 platforms)

=== Testing Runner Probes ===
✓ kimi: OK
✓ gemini: OK  
✓ deepseek: OK

=== Testing Contract Runner Execution ===
✓ Task completed successfully (9.75s, 18 tokens)

=== Testing Result Schema ===
✓ All required fields present

=== Testing Invalid Input Handling ===
✓ Invalid input rejected correctly

RESULTS: 5 passed, 0 failed
```

### Files Created
```
runners/
├── base_runner.py (abstract base class)
└── contract_runners.py (Kimi, DeepSeek, Gemini, Courier)

core/
└── config_loader.py (JSON config loader)

tests/
└── test_contract_e2e.py (E2E test)
```

### Files Updated
- `config/models.json` - Added browser-use-gemini model
- `core/orchestrator.py` - Wired to config loader, uses contract runners
- `CURRENT_STATE.md` - Updated with new components

### Key Features Implemented
1. **Contract Enforcement** - stdin JSON → stdout JSON with exact schema
2. **Health Checks** - `--probe` flag for all runners
3. **Config Caching** - Single load, cached access
4. **Agent Resolution** - Skills/tools expanded when loading agents
5. **Validation** - Config consistency checked at startup

### Next Session
1. Dashboard connection (Vibeflow mockup → Supabase)
2. Courier runner implementation (browser-use integration)
3. First real task through full pipeline

---

# 2026-02-16 (Session 2)

## Session Summary

### What Was Built
**Contract Layer (Complete):**
- 4 JSON schemas (task_packet, result, event, run_feedback)
- 1 runner interface document (RUNNER_INTERFACE.md)
- 5 config files (skills.json, tools.json, models.json, platforms.json, agents.json)
- 12 agent prompts in config/prompts/

**Key Architectural Decisions:**
- Researcher suggests only, does NOT implement
- Maintenance is ONLY agent that touches system
- Orchestrator + Researcher = learning brain
- 80% limit rule (pause before cutoff)
- If it can't be undone, it can't be done

### What Was Caught (Type 1 Errors Prevented)
- "We're invested in Python" circular reasoning
- Supervisor doing routing (that's orchestrator's job)
- Pausing for human after 3 failures (lazy - should diagnose)
- Researcher implementing things (suggests only)
- Exit ready meaning just "reversible" not "portable to anyone"

### Files Created
```
config/
├── schemas/
│   ├── task_packet.schema.json
│   ├── result.schema.json
│   ├── event.schema.json
│   └── run_feedback.schema.json
├── skills.json (13 skills)
├── tools.json (10 tools)
├── models.json (8 models)
├── platforms.json (4 platforms)
├── agents.json (12 agents)
├── prompts/
│   ├── vibes.md
│   ├── orchestrator.md
│   ├── researcher.md
│   ├── consultant.md
│   ├── planner.md
│   ├── council.md
│   ├── supervisor.md
│   ├── courier.md
│   ├── internal_cli.md
│   ├── internal_api.md
│   ├── tester_code.md
│   └── maintenance.md
└── RUNNER_INTERFACE.md
```

### Files Updated
- `docs/core_philosophy.md` - Added "If it can't be undone", clarified exit ready
- `CURRENT_STATE.md` - Complete rewrite for new structure

### Key Principles Clarified
1. Zero vendor lock-in - everything swappable
2. Modular & swappable - change one, nothing else breaks
3. Exit ready - pack up, hand over to ANYONE
4. Reversible - if it can't be undone, it can't be done
5. Always improving - daily research, weekly evaluation

### Next Session
1. Create runner skeletons (follow RUNNER_INTERFACE.md)
2. Test config loading
3. Wire orchestrator to new config files
4. First end-to-end test

---

# 2026-02-16 (Session 1)

## Session Summary

### What Was Built
- **Kimi Swarm Test (DEC-008)** - ✅ SUCCESS - 3 tasks in 12.53s, parallel execution confirmed
- **Task Packet Schema** - Created `contracts/task_packet.schema.json`
- **Vibeflow Deep Review** - Documented in `docs/vibeflow_review.md`

### What Was Caught (Type 1 Errors Prevented)
- Hardcoding agent counts, worker counts, phase counts
- Task packet schema without templates
- Courier confusion (thought fallback, actually primary for web)
- Planning before understanding

### Key Learnings from Vibeflow
1. Task packets need TEMPLATES, not just schemas
2. Skills loaded from registry.json (no hardcoded agents)
3. Visual execution = couriers (browser-use to AI studios)
4. Each task self-verifiable with acceptance criteria
5. OK probes verify skills work

### Files Created
- `SESSION_NOTES.md` - Mistakes documented
- `docs/vibeflow_review.md` - Full Vibeflow analysis
- `contracts/task_packet.schema.json` - Schema (templates TODO)
- `plans/vibepilot_prd.json` - Draft (needs revision)
- `plans/vibepilot_plan.json` - Draft (needs revision)

### Next Session
1. Create task packet TEMPLATES
2. Create skills registry
3. Map existing code to skills
4. THEN create plan with zero ambiguity

---

# 2026-02-15

## 15:30 UTC - GLM-5 (Autonomous)
**Commit:** (pending)
**Type:** Infrastructure (Major)
**Summary:** Execution Backbone complete - concurrent orchestration, supervisor, telemetry, memory interface

**Files Created:**
- `core/memory.py` — Pluggable memory interface (FileBackend, SupabaseBackend) for future RAG/Vector
- `core/telemetry.py` — OpenTelemetry observability with fallback logging, LoopDetector for Watcher
- `core/orchestrator.py` — ConcurrentOrchestrator with RunnerPool, DependencyManager, ThreadPoolExecutor
- `agents/supervisor.py` — SupervisorAgent implementation (review, approve, reject, coordinate testing)
- `docs/schema_dependency_rpc.sql` — Supabase RPC functions for dependency unlock

**Files Changed:**
- `core/__init__.py` — Added imports for new modules
- `CURRENT_STATE.md` — Updated built components list

**Execution Backbone Components:**
| Component | Purpose | Status |
|-----------|---------|--------|
| Supervisor | Reviews outputs, approves/rejects, coordinates testing | ✅ Complete |
| Dependency Manager | Checks deps, unlocks ready tasks | ✅ Complete |
| Runner Pool | Tracks available runners, prevents double-assign | ✅ Complete |
| Concurrent Orchestrator | ThreadPoolExecutor for parallel execution | ✅ Complete |
| Telemetry | OpenTelemetry tracing + fallback logging | ✅ Complete |
| Loop Detector | Analyzes telemetry for stuck patterns | ✅ Complete |
| Memory Interface | Pluggable storage for context (future RAG) | ✅ Complete |
| Dependency RPC | Supabase functions for atomic task unlock | ✅ Complete |

**Key Features:**
- Router scoring formula from Vibeflow (w1*priority + w2*success_rate + w3*strengths)
- Automatic dependency unlock when tasks complete
- Model performance ratings updated after each task
- ROI report generation
- Loop detection: repeated tool calls, repeated errors, long-running, token waste

**Why:**
- Infrastructure needed before Planner can create execution tasks
- Parallel agents require: supervisor, dependency unlock, runner pool
- Prevention = 1% of cure cost - telemetry catches issues early
- Memory interface designed for future swap to vector/graph RAG

**Next:**
- Dashboard connection (Vibeflow mockup → Supabase)
- Run schema RPC in Supabase
- Test concurrent execution

**Rollback:**
```bash
git revert HEAD
```

---

## [Earlier Today]

## [Same Session - Update 3] - GLM-5 + Human
**Type:** Documentation (Philosophy + Prevention)
**Summary:** Add prevention principle, Type 1 errors, pluggable memory consideration, NO FORMS rule

**Files Changed:**
- `docs/core_philosophy.md` — Added "Prevention over cure" with futurologist mindset
- `docs/UPDATE_CONSIDERATIONS.md` — Added Consideration 16: Pluggable Memory Architecture
- `~/AGENTS.md` — Added NO MULTIPLE CHOICE FORMS rule, Type 1 Error awareness, core_philosophy to required reading

**Key Additions:**
- Prevention = 1% of cure cost
- Type 1 Error: Fundamental mistake that ruins everything downstream
- Futurologist glasses: What WILL go wrong eventually?
- Pluggable memory interface: Design now, implement later, swap when better tech emerges
- NO FORMS: Two sessions tried restrictive multiple-choice, user hated it, never again

**Why:**
- User emphasized prevention and foresight
- Memory systems may be needed later - design interface now
- Forms create friction, humans hate filling them
- Type 1 errors must be prevented, not fixed

**Rollback:**
```bash
git checkout HEAD~1 -- docs/core_philosophy.md docs/UPDATE_CONSIDERATIONS.md ~/AGENTS.md
```

---

## [Same Session - Update 2] - GLM-5 + Human
**Type:** Documentation (Philosophy)
**Summary:** Add core philosophy document - strategic mindset for all agents

**Files Created:**
- `docs/core_philosophy.md` — Strategic mindset and inviolable principles

**Files Changed:**
- `CURRENT_STATE.md` — Added to required reading (now 4 files)

**Philosophy Captured:**
1. **Backwards Planning** — Dream → enables that → enables that → first step
2. **Options Thinking** — Many paths, always have alternatives, door closed = find or build another
3. **Preparation Over Hope** — Every scenario considered, resources created not just found
4. **Inviolable Principles** — Zero lock-in, modular, exit-ready, always improving

**Why:**
- User articulated strategic thinking approach
- Applies to ALL agents: Consultant, Planner, Council, Supervisor, Maintenance, Research
- Not just rules, but how VibePilot thinks
- Must be referenced in every decision

**Rollback:**
```bash
git checkout HEAD~1 -- docs/core_philosophy.md CURRENT_STATE.md
```

---

## [Same Session - Update] - GLM-5
**Type:** Documentation
**Summary:** Add Vibeflow and Gemini video considerations to UPDATE_CONSIDERATIONS.md

**Files Changed:**
- `docs/UPDATE_CONSIDERATIONS.md` — Added 8 new considerations (8-15)

**New Considerations Added:**
| ID | Topic | Decision |
|----|-------|----------|
| DEC-008 | Vibeflow Dashboard | Accepted - reuse for frontend |
| DEC-009 | Skills Manifest | Pending - current approach works |
| DEC-010 | Event Log Pattern | Pending - evaluate need |
| DEC-011 | CI Gates | Pending - Phase 2 |
| DEC-012 | Router Scoring Formula | Accepted - add to orchestrator |
| DEC-013 | OpenTelemetry Tracing | Accepted - add early |
| DEC-014 | Agent Engineering Principles | Confirmed - already aligned |
| DEC-015 | SDK Skills | Pending - future consideration |

**Why:**
- Session produced actionable research findings
- Needed proper documentation, not just CHANGELOG mention
- UPDATE_CONSIDERATIONS.md is the canonical place for vetted improvements

**Rollback:**
```bash
git checkout HEAD~1 -- docs/UPDATE_CONSIDERATIONS.md
```

## 06:30 UTC - GLM-5
**Commit:** `6ccdeb5a`
**Type:** Documentation (Major Session)
**Summary:** Complete agent definitions, prompts, tech stack decisions

**Files Created:**
- `agents/agent_definitions.md` — Complete specs for 11 agents
- `prompts/planner.md` — Full prompt (400+ lines)
- `prompts/supervisor.md` — Full prompt (400+ lines)
- `prompts/council.md` — Full prompt (400+ lines)
- `prompts/orchestrator.md` — Full prompt (400+ lines)
- `prompts/testers.md` — Code + Visual tester prompts
- `prompts/system_researcher.md` — Full prompt (400+ lines)
- `prompts/watcher.md` — Full prompt (400+ lines)
- `prompts/maintenance.md` — Full prompt (400+ lines)
- `prompts/consultant.md` — Stub (awaiting user notes)
- `docs/tech_stack.md` — Technology decisions documented

**Key Decisions:**
- Python backend, React/TS/Vite frontend
- pytest (Python), Vitest (TS) for testing
- browser-use for browser automation (Gemini primary, ChatBrowserUse backup)
- GitHub Actions for CI/CD
- Gmail via browser-use for notifications
- Hetzner VPS target (€4/mo vs GCE $24/2wks)
- OpenRouter marked DANGEROUS (last resort only)
- Runner variants: Kimi CLI, OpenCode, DeepSeek API, Gemini API

**Agent Definitions Include:**
- Agent 0: Orchestrator (Vibes)
- Agent 1: Consultant Research
- Agent 2: Planner
- Agent 3: Council Member (3 lenses)
- Agent 4: Supervisor
- Agent 5: Watcher (redesigned - proactive prevention)
- Agent 6: Code Tester
- Agent 7: Visual Tester
- Agent 8: System Research (enhanced with comprehensive data collection)
- Agent 9: Task Runners (4 variants)
- Agent 10: Courier (Phase 3)
- Agent 11: Maintenance

**Why:**
- PRD was descriptive but not plan-ready
- Agents needed full specs before Planner can create build tasks
- Tech stack decisions needed documentation for consistency

**Rollback:**
```bash
git revert 6ccdeb5a
```

---

## 02:00 UTC - GLM-5
**Commit:** `967dee2e`
**Type:** Documentation
**Files Changed:**
- `docs/prd_v1.4.md` — Complete ROI calculator with full task cost tracking

**Key Additions:**
- Philosophy: Real world testing, continuous evaluation, best/cheapest wins
- Full task cost: ALL attempts counted (failed attempt 1 + failed attempt 2 + split + passed attempts)
- Live ROI calculator dashboard mockup
- Courier cost attribution per task
- Model performance with cost per SUCCESSFUL task

**Why:**
- ROI wasn't tracking failed attempts
- Need to see total cost of task including all retries/reassignments
- Dashboard must show live, always-current ROI
- Model routing decisions need real data

**Rollback:**
```bash
git revert 967dee2e
```

---

## 01:30 UTC - GLM-5
**Commit:** `aaabc5c5`
**Type:** Documentation (Major)
**Files Changed:**
- `docs/prd_v1.4.md` — Comprehensive operational details added

**New Sections (10):**
- 3.6 Council Process Detail — Iterative rounds, feedback consolidation
- 3.7 Security: Vault Access Control — Who can access vault
- 3.8 Tester Isolation — Only code, nothing else
- 3.9 Credit & Rate Limit Tracking — Tokens in/out, cost calc
- 3.10 Task Failure & Branch Lifecycle — Handling, branch states
- 3.11 PRD Changes Mid-Project — Version control process
- 3.12 Human Notification — Dashboard, daily email, alerts
- 3.13 Multi-Project Handling — Separate repos, shared models
- 3.14 Prompt Storage — YAML in GitHub, human editable
- 3.15 Deployment Flow — Merge to deploy process
- 3.16 Data Retention — Lifecycle, archive rules

**Orchestrator Enhanced:**
- Learning mechanism detailed
- Platform exhaustion handling
- Only agent user communicates with

**Data Model:**
- models: credit tracking, tokens in/out costs, recommendation_score
- task_runs: separate tokens_in/out, failure_reason/code

**Why:**
- Gaps identified after thorough review
- Clarifies security (vault access control)
- Operational details for every edge case
- No ambiguity for future sessions

**Rollback:**
```bash
git revert aaabc5c5
```

---

## 00:45 UTC - GLM-5
**Commit:** `910a2918`
**Type:** Documentation
**Files Changed:**
- `docs/prd_v1.4.md` - Added research agents, watcher, clarified runners vs couriers

**Additions:**
- **Consultant Research Agent** — Deep market/competition research, works with user until PRD approved
- **System Research Agent** — Daily web scouring, findings → UPDATE_CONSIDERATIONS.md
- **Watcher Agent** — Prevents loops of doom, kills stuck tasks, detects drift
- **Runners vs Couriers clarification:**
  - Runners: May see codebase (dependencies), NO chat URL
  - Couriers: Browser delivery, chat URL captured, NO codebase
- **Dashboard watcher alerts** section

**Why:**
- Task agents were conflated with couriers
- Research agents were missing from role definitions
- Watcher prevents the "fix it" loop we experienced this session

**Rollback:**
```bash
git revert 910a2918
```

---

## 00:15 UTC - GLM-5
**Commit:** `f3feb88c`
**Type:** Documentation (Critical)
**Files Added:**
- `docs/prd_v1.4.md` - Comprehensive system specification

**Files Changed:**
- `CURRENT_STATE.md` - Updated required reading to 3 files

**Why:**
- Previous PRD missing key concepts from this session
- New sessions need complete context without re-explaining everything
- Captures: full pipeline, planner spec, runners vs couriers, vault, GitHub flow, dashboard, ROI

**Key Sections:**
- Section 2: Complete pipeline diagram
- Section 5: Planner specification with confidence calculation
- Section 6: Runners vs Couriers distinction
- Section 8: Vault (secret management)
- Section 9: Dashboard features

**Rollback:**
```bash
git revert f3feb88c
```

---

# 2026-02-14

## 21:30 UTC - GLM-5
**Commit:** `46423d69`
**Type:** Security + Migration
**Files Changed:**
- `vault_manager.py` - Fixed schema column names, added get_api_key() helper
- `runners/api_runner.py` - Runners now use vault, added OpenRouter runner
- `.env.example` - Reduced to 3 bootstrap keys, vault instructions

**Vault Secrets Added:**
- DEEPSEEK_API_KEY ✅
- GITHUB_TOKEN ✅
- GEMINI_API_KEY ✅
- OPENROUTER_API_KEY ✅

**Why:**
- Secrets in .env file = prompt injection risk (any agent could read them)
- Vault approach: keys encrypted in Supabase, retrieved on demand
- Migration now needs only 3 keys (SUPABASE_URL, SUPABASE_KEY, VAULT_KEY)
- Store those 3 in GitHub Secrets → instant setup on new machine

**Migration Path:**
```
git clone → set 3 env vars → ./setup.sh → done
```

**Rollback:**
```bash
git revert 46423d69
```

---

## 21:00 UTC - GLM-5
**Commit:** N/A (no code change)
**Type:** Documentation
**Files Changed:**
- `~/AGENTS.md` - Added "READ BEFORE ANY TOOL USE" warning, philosophy preamble

**Why:**
- Session started with reactive "fix it" behavior (reinstalled Kimi without reading context)
- AGENTS.md now enforces reading CURRENT_STATE.md BEFORE any tool use
- Prevents context window waste from fixing things that aren't broken

**Rollback:**
```bash
git checkout HEAD~1 -- ~/AGENTS.md
```

---

## 20:25 UTC - GLM-5
**Commit:** `eb3a85e3`
**Type:** Setup
**Actions:**
- Created logs/ directory
- Added cron job for daily backup (2 AM)
- Removed TEMP_CRON_COMMANDS.md
- Verified schema changes applied in Supabase:
  - task_packets: created_at, updated_at ✅
  - models: created_at, updated_at ✅
  - task_runs: created_at, updated_at ✅
  - council_reviews table ✅

**Status:** 1-3 complete, cron set up, all verified

---

## 19:55 UTC - GLM-5
**Commit:** `c5c5b143`
**Type:** Refactor + Philosophy
**Files Renamed:**
- `dual_orchestrator.py` → `orchestrator.py` (main orchestrator now)
- `orchestrator.py` → `docs/orchestrator_v1_reference.py` (kept for reference)

**Files Changed:**
- `README.md` - Updated command to `python orchestrator.py`
- `CURRENT_STATE.md` - Updated file references
- `.context/guardrails.md` - Added core philosophy

**Why:** 
- "dual" naming was about current state (GLM + Kimi), not architecture
- Architecture already handles unlimited models via config
- Drop Kimi, add Gemini CLI, swap OpenCode for Codex - no problem
- One orchestrator, reads config, routes dynamically

**Philosophy Added:**
```
Core Philosophy:
- World-class engineering - design for change
- Permaculture principles - sustainable, self-evolving
- Prevent slop at source - bad design compounds
- Modular & swappable - no cascade failures

We avoid:
- Monolithic anything
- Tightly coupled dependencies
- Changes requiring full rewrites
```

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 19:40 UTC - GLM-5
**Commit:** `3382449f`
**Type:** Add + Cleanup
**Files Added:**
- `README.md` - GitHub landing page, quick start
- `TEMP_CRON_COMMANDS.md` - Cron setup (DELETE AFTER USE)
- `.env.example` - Environment template

**Files Removed (Obsolete):**
- `STATUS.md` - Superseded by CURRENT_STATE.md
- `archive/` - Old unused files
- `docs/scripts/kimi_setup.sh` - Kimi already set up
- `docs/scripts/sync_structure.py` - References non-existent table
- `docs/scripts/ingest_keys.py` - We use .env now

**Why:** Lean and clean. Removed obsolete files, added GitHub discoverability.

**README.md features:**
- Quick start (4 commands)
- Documentation map
- Architecture overview
- Maintenance commands

**Cleanup:**
- Removed 5 obsolete files
- Removed archive folder
- Updated directory index in CURRENT_STATE.md

**TEMP_CRON_COMMANDS.md:**
- Cron job for daily backup
- Instructions for setup
- DELETE THIS FILE after setting up cron

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 19:35 UTC - GLM-5
**Commit:** `af237421`
**Type:** Add
**Files Added:**
- `setup.sh` - One-command setup for fresh machine
- `.env.example` - Environment template with documentation
- `scripts/backup_supabase.sh` - Daily backup automation

**Files Changed:**
- `CURRENT_STATE.md` - Updated directory index, migration checklist, removed TODOs

**Why:** Critical gaps that could block migration:
- setup.sh: Can't spin up on new server without it
- .env.example: Incomplete = silent failures
- backup script: Data loss = total loss

**setup.sh features:**
- Checks prerequisites (python3, pip, git, curl)
- Verifies .env exists and has required variables
- Creates venv and installs dependencies
- Tests Supabase connection
- Tests GitHub access
- Clear next steps output

**.env.example features:**
- All 6 required variables documented
- Instructions on where to get each key
- Notes on model priority
- Security reminders

**backup_supabase.sh features:**
- 30-day retention
- Timestamped backups
- Cleanup of old backups

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 19:15 UTC - GLM-5
**Commit:** `992ba26a`
**Type:** Rename + Restructure
**Files Renamed:**
- `docs/video summary ideas` → `docs/UPDATE_CONSIDERATIONS.md`

**Files Removed:**
- `docs/video_insights_2026-02-14.md` (content merged into UPDATE_CONSIDERATIONS.md)

**Files Changed:**
- `CURRENT_STATE.md` - Updated references, directory index

**Why:** 
- Set up daily workflow for research agent input
- File will be cleared after each day's considerations are processed
- Archive of decisions kept in DECISION_LOG.md
- Future: Research agent finds improvements, adds here, Council/GLM-5 vets

**Structure:**
- Daily considerations → UPDATE_CONSIDERATIONS.md
- Vetting → GLM-5 / Council
- Decisions → DECISION_LOG.md
- Clear file → Ready for next day

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 19:00 UTC - GLM-5
**Commit:** `872b6e21`
**Type:** Update
**Files Changed:**
- `CURRENT_STATE.md` - Added Must Preserve/Never Do sections, simplified priorities
- `.context/DECISION_LOG.md` - Marked DEC-012 to DEC-015 as rejected with reasoning
- `docs/video_insights_2026-02-14.md` - Added what was rejected and why

**Why:** Vetted research suggestions against VibePilot's specific needs. Rejected over-engineering in favor of simpler approach.

**Decisions:**
- DEC-012, 013, 014, 015: Rejected (over-engineering, duplicates, complexity)
- Solution: Add Must Preserve/Never Do to CURRENT_STATE.md instead

**Priorities Updated:**
1. Schema Audit + Validation Script (DEC-011)
2. Prompt Caching (DEC-007)
3. Council RPC

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 18:35 UTC - GLM-5
**Commit:** `98668742`
**Type:** Add
**Files Added:**
- `docs/video_insights_2026-02-14.md` - Senior engineer rules, noiseless memory, navigation context

**Files Changed:**
- `CURRENT_STATE.md` - Updated decisions, priorities, directory index
- `.context/DECISION_LOG.md` - Added DEC-011 through DEC-015

**Why:** Capture video insights for next session:
- Senior engineer schema rules (portability, auditability)
- Noiseless compression (80% token reduction)
- Navigation-based context (terminal tools vs RAG)
- Awareness agents (auto-inject by keyword)

**New Proposed Decisions:**
- DEC-011: Schema Senior Rules Audit
- DEC-012: Self-Awareness SSOT Document
- DEC-013: Noiseless Compression Protocol
- DEC-014: Navigation-Based Context
- DEC-015: Awareness Agent

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 18:10 UTC - GLM-5
**Commit:** `8df8c51e`
**Type:** Update
**Files Changed:**
- `CURRENT_STATE.md` - Updated known good commit
- `CHANGELOG.md` - Added this entry

**Why:** Update known good commit after restructure

**Rollback:**
```bash
git revert 8df8c51e
```

---

## 18:05 UTC - GLM-5
**Commit:** `5719ea0f`
**Type:** Update
**Files Changed:**
- `CURRENT_STATE.md` - Major restructure for comprehensive clarity

**Added:**
- KNOWN GOOD STATE section (verified working commit for rollback)
- ACTIVE WORK section (what's in progress)
- 30-SECOND SWAPS section (zero code change swaps)
- UPDATE RESPONSIBILITY MATRIX (if X changes, update Y)
- QUICK FIX GUIDE (common issues and fixes)
- MIGRATION CHECKLIST (pack up and move)
- Required reading clarification (TWO files: this + CHANGELOG)

**Why:** Any agent/human reads TWO files and knows everything. No debugging loops of doom. Stress-free architecture.

**Rollback:**
```bash
git revert <commit_hash>
```

---

## 17:50 UTC - GLM-5
**Commit:** `4ad011e3`
**Type:** Update
**Files Changed:**
- `CHANGELOG.md` - Added entry for commit 8b104062

**Why:** Changelog must track itself

**Rollback:**
```bash
git revert 4ad011e3
```

---

## 17:45 UTC - GLM-5
**Commit:** `8b104062`
**Type:** Add
**Files Added:**
- `CHANGELOG.md` - Full audit trail for easy rollback

**Files Changed:**
- `CURRENT_STATE.md` - Added CHANGELOG references

**Why:** Track every change with timestamps for easy rollback. Prevent debugging when rollback is faster.

**Rollback:**
```bash
git revert 8b104062
```

---

## 17:35 UTC - GLM-5
**Commit:** `0715bfae`
**Type:** Update
**Files Changed:**
- `CURRENT_STATE.md` - Added source of truth index, directory index

**Why:** Prevent Supabase queries and ls commands just to see structure

**Rollback:**
```bash
git revert 0715bfae
```

---

## 16:50 UTC - GLM-5
**Commit:** `a8c7d17b`
**Type:** Add
**Files Added:**
- `CURRENT_STATE.md` - Single source of truth for context restoration

**Why:** 77k tokens to understand state was unsustainable. Now one file.

**Decisions:** DEC-009 (Council feedback summary), DEC-010 (Single source of truth)

**Rollback:**
```bash
git revert a8c7d17b
```

---

## 16:15 UTC - GLM-5
**Commit:** `b41a98b6`
**Type:** Update
**Files Changed:**
- `docs/MASTER_PLAN.md` - Clarified Council two-process model
- `.context/DECISION_LOG.md` - Updated DEC-004

**Why:** Council isn't one-size-fits-all. PRDs need iterative, updates need one-shot.

**Rollback:**
```bash
git revert b41a98b6
```

---

## 15:50 UTC - GLM-5
**Commit:** `8eec28b1`
**Type:** Update
**Files Changed:**
- `docs/MASTER_PLAN.md` - Refined Council process based on real experience
- `.context/DECISION_LOG.md` - Added DEC-004, DEC-005

**Why:** Real experience showed 3 models need 4 rounds for consensus on PRDs

**Rollback:**
```bash
git revert 8eec28b1
```

---

## 15:20 UTC - GLM-5
**Commit:** `b8c4ee32`
**Type:** Add
**Files Added:**
- `docs/MASTER_PLAN.md` - 858-line zero-ambiguity specification

**Files Changed:**
- `STATUS.md` - Updated structure
- `docs/SESSION_LOG.md` - Added Phase 5, Phase 6

**Why:** Unified specification for all agents, context isolation by role

**Rollback:**
```bash
git revert b8c4ee32
```

---

## 14:30 UTC - GLM-5
**Commit:** `ed2e425d`
**Type:** Add
**Files Added:**
- `.context/guardrails.md` - 8 pre-code gates, P-R-E-V-C workflow
- `.context/DECISION_LOG.md` - Template + 3 documented decisions
- `.context/agent_protocol.md` - Handoff rules, conflict resolution
- `.context/quick_reference.md` - One-page cheat sheet
- `.context/ops_handbook.md` - Disaster recovery, monitoring
- `scripts/prep_migration.sh` - Migration prep automation

**Why:** Strategic safeguards to prevent "vibe coding" traps

**Rollback:**
```bash
git revert ed2e425d
```

---

# 2026-02-13

## 23:50 UTC - Human
**Commit:** `6a97eaaa`
**Type:** Add
**Files Added:**
- `docs/video summary ideas` - Video insights (prompt caching, context standard, Kimi swarm)

**Why:** Capture video learnings for future implementation

**Rollback:**
```bash
git revert 6a97eaaa
```

---

## 22:30 UTC - GLM-5
**Commit:** `26502559`
**Type:** Update
**Files Changed:**
- `docs/SESSION_LOG.md` - Added multi-project support to roadmap

**Why:** Support recipe app, finance app, VibePilot, legacy project simultaneously

**Rollback:**
```bash
git revert 26502559
```

---

## 21:50 UTC - GLM-5
**Commit:** `eded835c`
**Type:** Add
**Files Added:**
- `STATUS.md` - Root-level status and recovery

**Why:** Quick recovery after terminal crash

**Rollback:**
```bash
git revert eded835c
```

---

## 21:00 UTC - GLM-5
**Commit:** `4141f826`
**Type:** Add
**Files Added:**
- `docs/SESSION_LOG.md` - Session history
- `config/vibepilot.yaml` - Config-driven architecture

**Why:** Single config file for all runtime changes

**Rollback:**
```bash
git revert 4141f826
```

---

## 20:00 UTC - GLM-5
**Commit:** `6cb215c0`
**Type:** Add
**Files Changed:**
- `core/roles.py` - Role system
- `dual_orchestrator.py` - Gemini orchestrator option

**Why:** 2-3 skills max per role, models wear hats

**Rollback:**
```bash
git revert 6cb215c0
```

---

## 19:00 UTC - GLM-5
**Commit:** `b51acf8d`
**Type:** Add
**Files Added:**
- `docs/scripts/kimi_dispatch_demo.py` - Kimi dispatch demo

**Why:** Test Kimi CLI integration

**Rollback:**
```bash
git revert b51acf8d
```

---

## 18:00 UTC - GLM-5
**Commit:** `fc145ea2`
**Type:** Add
**Files Added:**
- `runners/kimi_runner.py` - Kimi runner for automatic dispatch

**Why:** Integrate Kimi CLI as parallel executor

**Rollback:**
```bash
git revert fc145ea2
```

---

## 17:00 UTC - GLM-5
**Commit:** `9f0fbac1`
**Type:** Add
**Files Added:**
- `docs/scripts/kimi_setup.sh` - Kimi CLI setup commands

**Why:** Document Kimi installation

**Rollback:**
```bash
git revert 9f0fbac1
```

---

## 16:00 UTC - GLM-5
**Commit:** `c425b24a`
**Type:** Add
**Files Added:**
- `docs/scripts/pipeline_test.py` - Pipeline test script

**Why:** Test full 12-stage task flow

**Rollback:**
```bash
git revert c425b24a
```

---

## 15:00 UTC - GLM-5
**Commit:** `8c5d6111`
**Type:** Update
**Files Changed:**
- `docs/schema_rls_fix.sql` - RLS fix for backend access

**Why:** Allow backend to query without RLS blocking

**Rollback:**
```bash
git revert 8c5d6111
```

---

## Earlier (see git log)
- `52ae359f` - Fix ROUND() function
- `8867b16e` - Add voice interface + project tracking
- `3527f775` - Add VibePilot v1.3 PRD + Platform Registry
- `170b3fdf` - Add VibePilot v1.2 architecture diagram
- `d3086fc5` - Add Vibeflow v5 adoption analysis
- `3d4d40de` - Add safety patches + escalation logic
- `b7966925` - Add TaskManager for new schema
- `62f816fd` - Add VibePilot Core Schema v1.0
- `052aa579` - Phase 2: Core Agent Implementation
- `c888a932` - Add Supabase schema reset SQL

---

# ROLLBACK PROCEDURES

## Single Commit Rollback

```bash
# See what commit did
git show <commit_hash>

# Rollback (creates new commit that undoes changes)
git revert <commit_hash>

# Push rollback
git push origin main
```

## Multiple Commits Rollback

```bash
# Rollback to specific point (DESTRUCTIVE - use carefully)
git reset --hard <commit_hash>

# Force push (only if you're sure)
git push origin main --force
```

## File-Level Rollback

```bash
# Restore specific file to specific commit
git checkout <commit_hash> -- path/to/file

# Commit the restoration
git add path/to/file
git commit -m "Rollback path/to/file to <commit_hash>"
git push origin main
```

## Full System Rollback (Nuclear Option)

```bash
# 1. Clone fresh
git clone git@github.com:VibesTribe/VibePilot.git vibepilot-rollback
cd vibepilot-rollback

# 2. Checkout specific commit
git checkout <commit_hash>

# 3. Update remote
git push origin main --force

# 4. On GCE, re-clone
cd ~
rm -rf vibepilot
git clone git@github.com:VibesTribe/VibePilot.git
cd vibepilot
source venv/bin/activate  # If venv exists, or recreate
```

---

# BRANCH TRACKING

## Active Branches

| Branch | Purpose | Status |
|--------|---------|--------|
| `main` | Production | Active |

## Merged & Deleted Branches

| Branch | Merged | Deleted | Commit | Notes |
|--------|--------|---------|--------|-------|
| (none yet) | - | - | - | - |

---

# HOW TO UPDATE THIS FILE

**After EVERY change:**

```markdown
## HH:MM UTC - <Agent/Human>
**Commit:** `<hash>`
**Type:** Add | Update | Remove | Merge
**Files Added:** (if any)
**Files Changed:** (if any)
**Files Removed:** (if any)
**Why:** <one line reason>
**Decisions:** DEC-XXX (if applicable)
**Rollback:** `git revert <hash>`
---
```

**After EVERY merge:**
1. Update "Merged & Deleted Branches" table
2. Include branch name, merge commit, deletion timestamp

**This file is the audit trail. Keep it accurate.**

---

*Last updated: 2026-02-14 17:35 UTC*
*Next entry: After next change*
