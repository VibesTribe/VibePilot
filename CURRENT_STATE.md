# VibePilot Current State

**Required reading: FOUR files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/prd_v1.4.md`** - Complete system specification
3. **`docs/core_philosophy.md`** - Strategic mindset and inviolable principles
4. **`CHANGELOG.md`** - History, changes, rollback commands

**Read all four → Know everything → Do anything**

---

**Last Updated:** 2026-02-17 22:30 UTC
**Updated By:** GLM-5 (Session 10: Config refactor, courier works, Kimi team activated)
**Known Good Commit:** `cf2cbfcb`
**Kimi Subscription:** $0.99/mo expires Feb 27 → $19/mo (MAXIMIZE USAGE UNTIL THEN)

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

# VIBEFLOW DASHBOARD (Reference)

**Live Dashboard (Mock Data):** https://vibestribe.github.io/vibeflow/

**GitHub Repo:** https://github.com/VibesTribe/vibeflow

**Note:** These URLs have been provided multiple times. Check here first before asking.

---

# KNOWN GOOD STATE

| Commit | Date | Status | Notes |
|--------|------|--------|-------|
| `aaabc5c5` | 2026-02-15 | ✅ Verified | PRD v1.4 complete (operational details) |
| `46423d69` | 2026-02-14 | ✅ Verified | Vault-based secrets, migration ready |
| `c5c5b143` | 2026-02-14 | ✅ Verified | Schema audit, caching, Council RPC |

**If everything breaks:**
```bash
git checkout aaabc5c5
```

---

# ACTIVE WORK

## GLM-5 + Kimi Team (Feb 17-27, 2025)

**Kimi subscription jumps from $0.99 → $19 on Feb 27. MAXIMIZE USAGE.**

| Role | Agent | Job |
|------|-------|-----|
| Architect/Vetter | GLM-5 | Think, analyze gaps, ensure principles, ask before acting |
| Executor | Kimi | Research, code, parallel tasks, post to branches |
| Maintenance | GLM-5 | ONLY agent that touches system files (after full understanding) |
| Supervisor | GLM-5 | Quality gate, nothing to main without human approval |
| Final Gate | Human | Approve before anything hits main |

**Workflow:**
```
Human → GLM-5 (analyze/plan) → Kimi (execute to branch) → GLM-5 (review) → Human (approve) → Main
```

**Kimi Tasks (until Feb 27):**
- Research repos → post to UPDATE_CONSIDERATIONS.md (on branch)
- Parallel courier tasks
- Browser automation with screenshots
- Any heavy lifting

| Task | Status | Agent | Started | Notes |
|------|--------|-------|---------|-------|
| Orchestrator update | pending | GLM-5 | - | Read new config structure |
| Kimi research review | pending | GLM-5 | - | Branch: kimi-research-2026-02-17 |

---

# CURRENT ISSUES

| Issue | Status | Impact | Notes |
|-------|--------|--------|-------|
| Kimi subscription ending | 🟡 Soon | Low | CLI auth transfers with ~/.kimi/ |
| GCE cost ($24/2wks) | 🟡 Action needed | High | Ready for Hetzner move (~€4/mo) |
| Secrets in .env | ✅ FIXED | - | Now in vault |

---

# WHERE WE ARE

## Built & Working

| Component | What It Does | Location |
|-----------|--------------|----------|
| Core Schema | Stores all state | Supabase: tasks, task_packets, models, platforms, projects, task_runs, skills, prompts, tools |
| Model Registry | Tracks available AI models with full config | Supabase `models` table (config JSONB) |
| Platform Registry | Tracks web AI platforms with rate limits | Supabase `platforms` table (config JSONB) |
| **Config Sync** | Bidirectional sync JSON ↔ Supabase | `scripts/sync_config_to_supabase.py` |
| **Routing Flags** | Q/W/M task routing | `tasks.routing_flag` + orchestrator respects it |
| **Slice Architecture** | Modular vertical slices | `tasks.slice_id` + Planner outputs slices |
| **Usage Tracking** | Track requests/tokens, 80% cooldown | `core/orchestrator.py` (CooldownManager, UsageTracker) |
| **Dashboard Adapter** | Supabase → Dashboard shape | `~/vibeflow/apps/dashboard/lib/vibepilotAdapter.ts` |
| Kimi CLI Runner | Executes tasks via Kimi | `runners/kimi_runner.py` |
| API Runner with Caching | DeepSeek, Gemini, OpenRouter | `runners/api_runner.py` |
| Dual Orchestrator | Routes tasks to right model | `orchestrator.py` (legacy), `core/orchestrator.py` (NEW concurrent) |
| Role System | Defines agent capabilities | `core/roles.py` + `config/vibepilot.yaml` |
| **Vault** | Encrypted secret storage | `vault_manager.py` + Supabase `secrets_vault` |
| **Telemetry** | OpenTelemetry observability | `core/telemetry.py` |
| **Loop Detector** | Detect stuck agents | `core/telemetry.py` |
| **Memory Interface** | Pluggable context storage | `core/memory.py` |
| **Dependency RPC** | Supabase functions for dep unlock | `docs/schema_dependency_rpc.sql` |
| **Base Runner** | Abstract class for contract enforcement | `runners/base_runner.py` |
| **Contract Runners** | Kimi, DeepSeek, Gemini, Courier following interface | `runners/contract_runners.py` |
| **Config Loader** | Central JSON config access | `core/config_loader.py` |
| **Terminal Dashboard** | Real-time task/runner monitoring | `dashboard/terminal_dashboard.py` |
| **Pipeline Test** | End-to-end task execution test | `tests/test_pipeline.py` |
| **Routing Tests** | Verify routing logic | `tests/test_routing_logic.py` |

## NEW: Contract Layer (Previous Session)

| Component | What It Is | Location |
|-----------|------------|----------|
| **Task Packet Schema** | JSON schema for task packets | `config/schemas/task_packet.schema.json` |
| **Result Schema** | JSON schema for runner output | `config/schemas/result.schema.json` |
| **Event Schema** | JSON schema for lifecycle events | `config/schemas/event.schema.json` |
| **Run Feedback Schema** | JSON schema for task feedback + learning | `config/schemas/run_feedback.schema.json` |
| **Runner Interface** | Contract every runner must follow | `config/RUNNER_INTERFACE.md` |
| **Skills Registry** | 13 skills, declarative | `config/skills.json` |
| **Tools Registry** | 10 tools | `config/tools.json` |
| **Models Registry** | 9 models with costs/limits | `config/models.json` |
| **Platforms Registry** | 4 web platforms | `config/platforms.json` |
| **Agents Registry** | 12 agents with scoped skills/tools | `config/agents.json` |
| **Agent Prompts** | 12 complete agent prompts | `config/prompts/*.md` |

## Contract Runners (This Session)

| Component | What It Is | Location |
|-----------|------------|----------|
| **Base Runner** | Abstract class enforcing RUNNER_INTERFACE | `runners/base_runner.py` |
| **Contract Runners** | Kimi, DeepSeek, Gemini, Courier following interface | `runners/contract_runners.py` |

## NEW: ROI Calculator (Session 8)

| Component | What It Is | Location |
|-----------|------------|----------|
| **ROI Schema** | Enhanced cost tracking fields | `docs/schema_v1.4_roi_enhanced.sql` |
| **ROI Adapter** | Calculate ROI from task_runs | `~/vibeflow/apps/dashboard/lib/vibepilotAdapter.ts` |
| **ROI Calculator** | Currency formatting, exchange rates | `~/vibeflow/apps/dashboard/lib/roiCalculator.ts` |
| **ROI Panel** | Dashboard panel with live data | `~/vibeflow/apps/dashboard/components/modals/MissionModals.tsx` |

**ROI Features:**
- Tokens split into in/out (output costs more)
- Courier cost tracking
- Theoretical vs actual cost
- Slice-level rollup (clickable to show tasks)
- Model-level rollup (clickable to show tasks per model)
- Model cards: cost/savings under "Tokens used"
- Agent details: "ROI Summary" section with total runs, role, costs
- Subscription tracking with renewal recommendations
- USD/CAD toggle with live exchange rate
| **Config Loader** | Central module for all JSON configs | `core/config_loader.py` |
| **E2E Test** | Full contract layer test | `tests/test_contract_e2e.py` |
| **Terminal Dashboard** | Real-time monitoring | `dashboard/terminal_dashboard.py` |
| **Pipeline Test** | End-to-end task execution | `tests/test_pipeline.py` |

**All runners now follow the contract:**
- Input: JSON via stdin or --task flag
- Output: JSON via stdout with exact schema
- Exit codes: 0=success, 1=failure
- `--probe` for health check

**Config Loader provides:**
- Cached access to skills, tools, models, platforms, agents
- Agent resolution with skills/tools expanded
- Config validation
| **Agent Prompts** | 12 complete agent prompts | `config/prompts/*.md` |

## Config Architecture (Session 10 - NEW)

| File | Purpose | Version |
|------|---------|---------|
| `destinations.json` | WHERE tasks execute (cli/web/api) | v1.1 |
| `roles.json` | WHAT job is done (13 roles) | v2.0 |
| `models.json` | WHO provides intelligence (10 LLMs) | v2.0 |
| `routing.json` | WHY/strategy (web_first, throttle, credit protection) | v1.0 |
| `tools.json` | HOW execution happens (browser-use, etc) | v2.0 |
| `WHAT_WHERE.md` | Project navigation guide | NEW |

**Three LEGO pieces (swappable independently):**
- DESTINATION (where) + ROLE (what) + MODEL (who)

**Old files (deprecated but kept for transition):**
- `agents.json` → replaced by `roles.json`
- `platforms.json` → replaced by `destinations.json`

## Courier Runner (Session 10 - In Progress)

| Component | Status | Notes |
|-----------|--------|-------|
| Playwright/Chromium | ✅ Installed | Browser automation ready |
| Browser navigation | ✅ Works | Tested chatgate.ai, deepseek.com |
| No-auth platforms | ✅ Identified | chatgate.ai (ChatGPT), deepseek.com |
| LLM adapter | ⚠️ Needs fix | browser-use interface compatibility |

## Agent Prompts (Complete)

| Agent | Role | Prompt File |
|-------|------|-------------|
| Vibes | Human interface, daily briefings | `config/prompts/vibes.md` |
| Orchestrator | Strategic routing, learning | `config/prompts/orchestrator.md` |
| Researcher | Daily research, suggests only | `config/prompts/researcher.md` |
| Consultant | Zero ambiguity PRD | `config/prompts/consultant.md` |
| Planner | Task breakdown from PRD | `config/prompts/planner.md` |
| Council | Multi-model review | `config/prompts/council.md` |
| Supervisor | Quality gate, merge control | `config/prompts/supervisor.md` |
| Courier | Web platform execution | `config/prompts/courier.md` |
| Internal CLI | CLI with codebase access | `config/prompts/internal_cli.md` |
| Internal API | API execution (emergency) | `config/prompts/internal_api.md` |
| Code Tester | Code validation | `config/prompts/tester_code.md` |
| Maintenance | ONLY system updater | `config/prompts/maintenance.md` |

## Vault (Secret Management)

**Purpose:** API keys encrypted in Supabase, never in .env files.

**Bootstrap Keys (must be set manually):**
- `SUPABASE_URL`
- `SUPABASE_KEY`
- `VAULT_KEY` (Fernet key: `LgbwdSxxDwTaeCN5Ed2J6ETrLhtIFhf2tfeEO0bVABg=`)

**Secrets in Vault:**
- `DEEPSEEK_API_KEY` ✅
- `GITHUB_TOKEN` ✅
- `GEMINI_API_KEY` ✅
- `OPENROUTER_API_KEY` ✅

**To add a secret:**
```python
from vault_manager import VaultManager
vault = VaultManager()
vault.ingest_secret('KEY_NAME', 'key_value')
```

**Runners use vault automatically:** `get_api_key('DEEPSEEK_API_KEY')`

## Models vs Platforms (Critical Distinction)

**MODELS** (`config/models.json`) = What we HAVE direct access to:
| Model | Type | Use |
|-------|------|-----|
| gemini-api | API | Drive browser-use, direct API calls |
| deepseek-chat | API | Drive browser-use, direct API calls |
| kimi-cli | CLI | Execute tasks directly |
| opencode | CLI | Execute tasks directly |

**PLATFORMS** (`config/platforms.json`) = Where couriers GO via browser:
| Platform | Context | Free Limits | Auth | Best For |
|----------|---------|-------------|------|----------|
| Gemini | 1M | 1500/day, 1M tokens/day | Gmail | Long docs, best free tier |
| Claude | 200K | ~10-20/day | Gmail | Coding, reasoning |
| ChatGPT | 128K | 10/3hr ⚠️ attach=1/10th | Gmail | General |
| Copilot | 128K | 30/session unlimited | Gmail | Free GPT-4 |
| DeepSeek | 64K | Generous | Gmail | Cheapest API |
| HuggingChat | Varies | Varies | Optional | Open source |

**NOT USABLE (Chinese phone/Alipay required):** Kimi web, GLM, Qwen, Minimax

## Vibeflow Dashboard (Connected to Supabase)

**Live Dashboard:** https://vibeflow-dashboard.vercel.app/

**GitHub Repo:** https://github.com/VibesTribe/vibeflow

**Adapter:** `~/vibeflow/apps/dashboard/lib/vibepilotAdapter.ts`

**How it works:**
1. Dashboard queries Supabase (tasks, task_runs, models, platforms)
2. Adapter transforms to dashboard shape (TaskSnapshot, AgentSnapshot, SliceCatalog)
3. Shows live data: progress, status, tokens, cooldown countdown

**What shows:**
- Slices with task counts (from tasks.slice_id or keyword inference)
- Agents with Q/W/M tier badges (Q=internal, W=web, M=MCP)
- Cooldown countdown when model hits 80% usage
- Completed tasks vanish from orbit

## Config Sync Workflow

**Import (JSON → Supabase):**
```bash
cd ~/vibepilot
source venv/bin/activate
python scripts/sync_config_to_supabase.py
```

**Export (Supabase → JSON):**
```bash
python scripts/sync_config_to_supabase.py --export
```

**When to use:**
- Import: Initial setup, recovery, after editing JSON files
- Export: Backup, before major changes, periodic

**What syncs:**
- models.json → models table (full config in JSONB)
- platforms.json → platforms table (full config in JSONB)
- skills.json → skills table (if exists)
- tools.json → tools table (if exists)
- prompts/*.md → prompts table (if exists)

## Not Yet Built

| Component | What It Will Do | Notes |
|-----------|-----------------|-------|
| Admin Panel forms | Add/edit models, platforms, skills via UI | UI exists, not wired to Supabase |
| Vibes → Maintenance | "Add X" requests flow to Maintenance agent | Handoff not wired |
| Voice Interface | Talk to Vibes | Designed in `docs/voice_interface.md` |
| Hetzner migration | Cost reduction | Ready, just needs execution |

---

# WHERE WE'RE GOING

## Done This Session (2026-02-17 Session 8)

1. ✅ **ROI Schema v1.4** — Enhanced cost tracking (tokens_in/out, courier costs, subscription fields)
2. ✅ **ROI Calculator** — Full dashboard panel with real data
3. ✅ **USD/CAD Toggle** — Live exchange rate fetch
4. ✅ **Slice ROI** — Per-slice cost breakdown
5. ✅ **Subscription Tracking** — Renewal recommendations based on metrics

## Done This Session (2026-02-17 Session 8)

1. ✅ **ROI Schema v1.4** — Enhanced cost tracking (tokens_in/out, courier costs, subscriptions)
2. ✅ **ROI Calculator** — Full dashboard panel with real data
3. ✅ **USD/CAD Toggle** — Live exchange rate fetch
4. ✅ **Slice ROI** — Per-slice cost breakdown (clickable)
5. ✅ **Model ROI** — Per-model cost breakdown (clickable to show tasks)
6. ✅ **Model cards** — Cost/savings shown under "Tokens used"
7. ✅ **Agent details** — "ROI Summary" section with total runs, role, costs
8. ✅ **Subscription Tracking** — Renewal recommendations

## Immediate (Next Session)

1. **Wire Admin Panel** — Forms to add/edit models, platforms, skills
2. **Wire Vibes → Maintenance** — "Add X" requests flow to Maintenance
3. **Test ROI with real courier runs** — Once courier is working

## Done Previous Session (2026-02-16/17 Session 7)

1. ✅ **Slice-based planning** — Planner outputs modular vertical slices
2. ✅ **Routing flags** — Q/W/M for task routing constraints
3. ✅ **Orchestrator respects routing** — Internal tasks never go to web
4. ✅ **Full config in Supabase** — JSONB stores complete model/platform config
5. ✅ **Bidirectional sync** — Import (JSON→DB) and Export (DB→JSON)
6. ✅ **Dashboard connected** — Live Supabase data, no hardcoding
7. ✅ **Completed tasks vanish** — No agent shown on merged tasks
8. ✅ **80% usage tracking** — CooldownManager + UsageTracker in orchestrator
9. ✅ **Cooldown countdown** — Dashboard shows cooldown with time remaining

## Near Term

1. Full PRD → Plan → Execute → Merge workflow
2. Voice Interface (Deepgram + Kokoro)
3. Hetzner migration (cost reduction)

---

# 30-SECOND SWAPS (Zero Code Changes)

| What to Swap | How | Time |
|--------------|-----|------|
| Add model | Edit `config/models.json` → add to `models` array | 30s |
| Remove model | Edit `config/models.json` → remove from array | 30s |
| Add platform | Edit `config/platforms.json` | 30s |
| Add skill | Edit `config/skills.json` → add to `skills` array | 30s |
| Add tool | Edit `config/tools.json` → add to `tools` array | 30s |
| Add agent | Edit `config/agents.json` → add to `agents` array | 30s |
| Change agent's model | Edit `config/agents.json` → change `model` field | 30s |
| Scope agent skills | Edit `config/agents.json` → change `skills` array | 30s |
| Update prompt | Edit `config/prompts/[agent].md` | 30s |
| Update limits | Edit `config/models.json` → change limit fields | 30s |

**All swaps:**
1. Edit the JSON file
2. Save
3. Update CHANGELOG.md
4. Done

---

# UPDATE RESPONSIBILITY MATRIX

| If You Change... | Update These Files |
|------------------|-------------------|
| Config (model, platform, role, prompt, threshold) | `config/vibepilot.yaml` + `CHANGELOG.md` |
| Add new file | `CURRENT_STATE.md` (directory index) + `CHANGELOG.md` |
| Remove file | `CURRENT_STATE.md` (directory index) + `CHANGELOG.md` |
| Architecture | `docs/MASTER_PLAN.md` + `CHANGELOG.md` + `CURRENT_STATE.md` |
| Decision made | `.context/DECISION_LOG.md` + `CURRENT_STATE.md` (key decisions) |
| Component built | `CURRENT_STATE.md` (where we are) + `CHANGELOG.md` |
| Priority changed | `CURRENT_STATE.md` (where we're going) + `CHANGELOG.md` |
| Issue found/resolved | `CURRENT_STATE.md` (current issues) + `CHANGELOG.md` |
| Anything | `CHANGELOG.md` (always) |

**Rule: Every change → CHANGELOG.md. Every structural change → CURRENT_STATE.md.**

---

# QUICK FIX GUIDE

## "Something broke, I don't know what"

```bash
# 1. Check if it worked before
git log --oneline -10

# 2. Rollback to last known good
git checkout 5a2f118b

# 3. If that fixes it, bisect to find bad commit
git bisect start
git bisect bad HEAD
git bisect good 5a2f118b
# Git will guide you through finding the bad commit

# 4. Once found, rollback just that commit
git bisect reset
git revert <bad_commit_hash>
```

## "Config change broke something"

```bash
# 1. Check what changed
git diff HEAD~1 config/vibepilot.yaml

# 2. Revert the config file
git checkout HEAD~1 -- config/vibepilot.yaml

# 3. Test

# 4. If fixed, commit the revert
git add config/vibepilot.yaml
git commit -m "Revert config change that broke X"
```

## "Database issue"

```bash
# 1. Check Supabase status
# Go to supabase.com dashboard

# 2. If table corrupted, restore from backup
# Supabase → Database → Backups → Restore

# 3. Check schema
psql $SUPABASE_URL -c "\dt"

# 4. Check recent changes
psql $SUPABASE_URL -c "SELECT * FROM task_runs ORDER BY created_at DESC LIMIT 10"
```

## "Agent stuck in loop"

```bash
# 1. Check task status in Supabase
# tasks table → find stuck task → check attempts

# 2. If > 3 attempts, escalate
UPDATE tasks SET status = 'escalated' WHERE id = '<task_id>';

# 3. Check CHANGELOG.md for recent changes that might have caused it

# 4. Rollback if needed
git revert <recent_commit>
```

## "Need to move to new server"

```bash
# NEW SERVER (30 minutes max):
# 1. Store these 3 in GitHub Secrets:
#    - SUPABASE_URL
#    - SUPABASE_KEY
#    - VAULT_KEY

# 2. On new server:
git clone git@github.com:VibesTribe/VibePilot.git
cd VibePilot

# 3. Set bootstrap keys (one of these methods):
#    Option A: GitHub CLI
gh secret set SUPABASE_URL --body "https://..."
gh secret set SUPABASE_KEY --body "eyJ..."
gh secret set VAULT_KEY --body "LgbwdSxx..."

#    Option B: Create .env manually
cat > .env << EOF
SUPABASE_URL=https://qtpdzsinvifkgpxyxlaz.supabase.co
SUPABASE_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
VAULT_KEY=LgbwdSxxDwTaeCN5Ed2J6ETrLhtIFhf2tfeEO0bVABg=
EOF

# 4. Run setup
./setup.sh

# 5. Verify vault works
source venv/bin/activate
python -c "from vault_manager import VaultManager; v=VaultManager(); print(v.get_secret('DEEPSEEK_API_KEY')[:8])"
# Should print: sk-dba57

# Done - all other secrets come from vault automatically
```

---

# MIGRATION CHECKLIST

**Before Move:**
- [x] All changes committed to GitHub ✅
- [x] Vault stores all API keys ✅
- [x] `.env` reduced to 3 bootstrap keys ✅
- [x] `setup.sh` exists and works ✅
- [ ] Store 3 keys in GitHub Secrets (or secure location)
- [x] CHANGELOG.md up to date ✅
- [x] CURRENT_STATE.md up to date ✅

**After Move:**
- [ ] `git clone` works
- [ ] Set 3 bootstrap env vars
- [ ] `./setup.sh` runs without errors
- [ ] Vault retrieval works (test with DeepSeek key)
- [ ] Kimi CLI auth transferred (`~/.kimi/` folder)
- [ ] Read CURRENT_STATE.md
- [ ] Continue from "WHERE WE'RE GOING"

---

# SOURCE OF TRUTH INDEX

## Changelog (Audit Trail)

| File | Purpose | Update Frequency |
|------|---------|------------------|
| `CHANGELOG.md` | Full audit trail of all changes | After EVERY change |

**Contains:** What changed, when, why, commit hash, rollback command

## Supabase (All State)

| Table | What It Stores | Key Fields |
|-------|----------------|------------|
| `tasks` | Task queue + state | id, status, priority, dependencies, project_id |
| `task_packets` | Versioned prompts | task_id, prompt, tech_spec, version |
| `models` | AI model registry | id, platform, status, limits, usage |
| `platforms` | Web AI platforms | id, type, capabilities, limits, status |
| `projects` | Multi-project tracking | id, name, status, roi_cumulative |
| `task_runs` | Execution history | task_id, model_id, platform, tokens, duration |

**Connection:** `.env` → `SUPABASE_URL`, `SUPABASE_KEY`

## Supabase Schema Updates (IMPORTANT)

**AI sessions CANNOT directly modify Supabase.** Schema changes require manual execution.

**Workflow:**
1. AI creates/updates schema file in `docs/schema_*.sql`
2. Commit to GitHub
3. Human copies file content from GitHub
4. Human pastes into Supabase SQL Editor and runs
5. Verify with the SELECT statements at end of file

**Current schema files:**
- `docs/schema_v1_core.sql` — Base tables (tasks, models, task_runs, etc.)
- `docs/schema_v1.1_routing.sql` — slice_id, routing_flag, phase, task_number on tasks
- `docs/schema_v1.2_platforms.sql` — platforms table + display columns
- `docs/schema_v1.2.1_platforms_fix.sql` — Adds missing columns to existing platforms
- `docs/schema_v1.3_config_jsonb.sql` — config JSONB, live stats, skills/prompts/tools tables, cooldown_expires_at
- `docs/schema_v1.4_roi_enhanced.sql` — ROI tracking (tokens_in/out, courier costs, subscriptions, exchange rates)

**All v1.x schemas should be applied to Supabase.**

## GitHub (All Code + Docs)

| Path | What It Does | Update When |
|------|--------------|-------------|
| `CURRENT_STATE.md` | Context restoration | Every session |
| `CHANGELOG.md` | Audit trail | Every change |
| `setup.sh` | One-command setup | Rare (new machine) |
| `.env.example` | Environment template | New variable needed |
| `config/vibepilot.yaml` | ALL runtime config | Config change |
| `.context/DECISION_LOG.md` | Full decision details | New decision |
| `.context/guardrails.md` | Pre-code gates | Rare |
| `docs/MASTER_PLAN.md` | Full specification | Architecture change |
| `core/roles.py` | Role definitions | Role change |
| `runners/kimi_runner.py` | Kimi CLI integration | Runner change |
| `orchestrator.py` | Multi-model routing | Orchestrator change |
| `task_manager.py` | Task CRUD | Task logic change |
| `agents/` | Agent implementations | Agent change |

**Repo:** `git@github.com:VibesTribe/VibePilot.git`

## GCE/Terminal (Ephemeral - NOT Source of Truth)

| Path | Purpose | Persistent? |
|------|---------|-------------|
| `~/vibepilot/` | Project clone | No (re-clone) |
| `~/vibepilot/venv/` | Python venv | No (recreate) |
| `~/vibepilot/.env` | Secrets | No (recreate from .env.example) |
| `~/.local/bin/kimi` | Kimi CLI | No (reinstall) |

**Server:** `ssh mjlockboxsocial@vibestribe`
**Nothing on GCE is source of truth.**

---

# DIRECTORY INDEX

```
~/vibepilot/
│
├── CURRENT_STATE.md          # THIS FILE - start here
├── CHANGELOG.md              # Audit trail - rollback info
├── README.md                 # GitHub landing page
├── setup.sh                  # One-command setup for fresh machine
├── .env                      # Secrets (NOT in git)
├── .env.example              # Secret template (in git)
├── requirements.txt          # Python dependencies
│
├── config/                   # ALL configuration (NEW)
│   ├── schemas/              # JSON schemas (contracts)
│   │   ├── task_packet.schema.json
│   │   ├── result.schema.json
│   │   ├── event.schema.json
│   │   └── run_feedback.schema.json
│   ├── skills.json           # 13 skills, declarative
│   ├── tools.json            # 10 tools
│   ├── models.json           # 8 models with costs/limits
│   ├── platforms.json        # 4 web platforms
│   ├── agents.json           # 12 agents with scoped skills/tools
│   ├── prompts/              # 12 agent prompts
│   │   ├── vibes.md
│   │   ├── orchestrator.md
│   │   ├── researcher.md
│   │   ├── consultant.md
│   │   ├── planner.md
│   │   ├── council.md
│   │   ├── supervisor.md
│   │   ├── courier.md
│   │   ├── internal_cli.md
│   │   ├── internal_api.md
│   │   ├── tester_code.md
│   │   └── maintenance.md
│   └── RUNNER_INTERFACE.md   # Contract every runner must follow
│
├── docs/
│   ├── core_philosophy.md    # Strategic mindset + principles
│   ├── prd_v1.4.md           # Product requirements
│   ├── tech_stack.md         # Technology decisions
│   ├── vibeflow_review.md    # Vibeflow patterns
│   ├── vibeflow_adoption.md  # What we kept/discarded
│   └── schema_*.sql          # Database schemas
│
├── core/
│   ├── orchestrator.py       # Concurrent orchestrator
│   ├── telemetry.py          # OpenTelemetry
│   ├── memory.py             # Memory interface
│   └── roles.py              # Role definitions
│
├── runners/
│   ├── kimi_runner.py        # Kimi CLI integration
│   └── api_runner.py         # API runner with caching
│
├── agents/                   # Agent implementations
│
├── scripts/
│   ├── backup_supabase.sh    # Daily backup
│   └── prep_migration.sh     # Migration prep
│
└── venv/                     # Python venv (ignored)
```

---

# KEY DECISIONS

| ID | Decision | Status | Summary |
|----|----------|--------|---------|
| DEC-001 | Dual Orchestrator | ✅ | GLM-5 primary, Kimi parallel, Gemini research |
| DEC-002 | State in Supabase | ✅ | All state in DB, code in GitHub |
| DEC-003 | Bounded Roles | ✅ | 2-3 skills max per role |
| DEC-004 | Council Two-Process | ✅ | One-shot for updates, iterative for PRDs |
| DEC-005 | Context Isolation | ✅ | Task agents see ONLY their task |
| DEC-006 | Python for Now | ✅ | Stay Python (can't afford rewrite), but design for swap |
| DEC-007 | Contract Layer | ✅ | JSON schemas + config files = zero code for swaps |
| DEC-008 | Runner Interface | ✅ | stdin JSON → stdout JSON, language agnostic |
| DEC-009 | Skills Registry | ✅ | Declarative skills.json, not hardcoded agents |
| DEC-010 | Maintenance is ONLY System Updater | ✅ | No other agent touches system files |
| DEC-011 | Researcher Suggests Only | ✅ | Finds, suggests. Does NOT implement. |
| DEC-012 | Orchestrator + Researcher = Learning | ✅ | Orchestrator learns from feedback, Researcher finds improvements |
| DEC-013 | 80% Limit Rule | ✅ | Pause platforms at 80% to prevent mid-task cutoff |
| DEC-014 | Human Consulted For | ✅ | Credit/subscription, visual UI/UX, daily briefings |
| DEC-015 | Exit Ready | ✅ | Pack up, hand over to anyone. All portable. |
| DEC-016 | If It Can't Be Undone | ✅ | It can't be done. Every change reversible. |
| DEC-017 | Supabase is Runtime Truth | ✅ | JSON files are backup/seed, Supabase is live source |
| DEC-018 | Routing Flags | ✅ | Q=internal only, W=web capable, M=MCP only |
| DEC-019 | Slices Before Phases | ✅ | Planner outputs vertical slices first, phases within slices |
| DEC-020 | 2+ Dependencies = Q | ✅ | Tasks with 2+ deps cannot go to web (guaranteed failure) |
| DEC-021 | Lamp Metaphor | ✅ | Agent = lamp (swappable shade/bulb/base/outlet) |

---

# ARCHITECTURE QUICK REF

## Agent Context Levels

| Agent | Sees | Why |
|-------|------|-----|
| Task Agent | ONLY their task | Zero drift |
| Planner | PRD, system overview | Create atomic slices |
| Council | Everything | Thorough review |
| Supervisor | Plan + task | Validate output |
| Maintenance | Everything (sandbox) | System changes |
| Researcher | System overview | Find improvements |
| Tester | Only code | Objective testing |

## Council Process

| Type | Rounds | For |
|------|--------|-----|
| One-shot vote | 1 | System updates, maintenance, features |
| Iterative consensus | 3-4 | PRDs, full vertical slice plans |

## Model Lenses

| Model | Catches |
|-------|---------|
| GPT | Drift from user intent |
| Gemini | Opportunities |
| GLM-5 | Build issues, vulnerabilities |

---

# QUICK START

```
NEW SESSION:
1. Read CURRENT_STATE.md (this file)
2. Read docs/core_philosophy.md (principles)
3. Read CHANGELOG.md (recent changes)
4. Check "WHERE WE'RE GOING" for priorities
5. Check "CURRENT ISSUES" for blockers
6. Start work
7. Update this file and CHANGELOG.md when done

MAKING A CHANGE:
1. Can it be done via config edit? (config/*.json)
   - Add model → models.json
   - Add skill → skills.json
   - Swap agent model → agents.json
   - Edit prompt → prompts/*.md
2. If yes → edit config, done
3. If no → Check UPDATE RESPONSIBILITY MATRIX
4. Make change
5. Update required files
6. Always update CHANGELOG.md

SOMETHING BROKE:
1. Check "KNOWN GOOD STATE" for rollback target
2. Check CHANGELOG.md for recent changes
3. Check "QUICK FIX GUIDE" for common issues
4. Rollback if debugging takes > 10 min
```

---

# HOW TO UPDATE THIS FILE

**After every session:**
1. Update "Last Updated" timestamp
2. Update "WHERE WE ARE" if components changed
3. Update "WHERE WE'RE GOING" if priorities changed
4. Add new decisions to "KEY DECISIONS"
5. Update "CURRENT ISSUES" as they arise/resolve
6. **Always update CHANGELOG.md**
