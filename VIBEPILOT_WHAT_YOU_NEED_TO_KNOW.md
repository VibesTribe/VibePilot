# VibePilot: What You Need to Know

**Read this FIRST. Every session. 5 minutes here saves hours of confusion.**

---

## 1. What is VibePilot?

VibePilot is a **sovereign AI execution engine** that transforms PRDs into production code using multiple specialized AI agents working in parallel.

**Not a chatbot. Not a toy app generator.** A production-grade system for building complete, secure, end-to-end full-stack applications.

### Target: Webs of Wisdom
A global, massively scalable, multilingual, multimedia social media platform where people pass down stories and wisdom across languages and time.

### Why VibePilot Exists
| Vibe Coding (Gemini in 30s) | VibePilot (Legacy Projects) |
|-----------------------------|------------------------------|
| Todo app, hardcoded | Global scale, configurable |
| Breaks on changes | Survives updates |
| No quality control | Supervisor + Council review |
| Single model, single pass | Multi-agent, revision loops |
| Static | Self-learning, self-improving |
| Dead on launch | Evolves with new models |

### Technical Constraints
- **Codebase:** ~14k lines Go + config layer (14,419 lines across 13 packages)
- **Hosting:** ThinkPad X220 (16GB RAM, i5-2520M Sandy Bridge, no AVX2, no GPU, 781GB disk free)
- **Runtime:** Single Go binary (~12MB), systemd user service
- **Network:** Phone USB tethered (wifi chip died during repaste), planning ethernet + headless
- **Architecture:** Config-driven plug-and-play modules, YAML DAG pipeline
- **Model Strategy:** Cloud free tier cascade across multiple providers. Counts change frequently — see `CURRENT_STATE.md` for verified numbers. Providers include Groq, OpenRouter (free + low-cost), Gemini (4 independent Google Cloud projects = 60 RPM free), NVIDIA NIM. Local inference too slow on this hardware (tested, 2 tok/s).
- **Config/DB Sync:** Research findings write to config/models.json AND Supabase atomically via `ResearchActionApplier`. No LLM middleman. Both sources stay in sync automatically.
- **Dashboard:** https://vibeflow-dashboard.vercel.app/ (sacred, Vercel auto-deploy)
- **Tunnel:** vibestribe.rocks via cloudflared (sacred, don't touch)
- **TTS:** edge-tts (fast, free, no API key)
- **Two-laptop setup:** x220 = dedicated VibePilot server, other laptop (mjlockboxsocial) = main machine with Claude Code CLI

---

## 2. Core Principles

### ⛔ NEVER Hardcode Anything
Everything is configurable via JSON files in `governor/config/`:
- Models → `models.json`
- Connectors → `connectors.json`
- Agents → `agents.json`
- Routing → `routing.json`

### ⛔ NO Type 1 Errors
Fundamental design mistakes ruin everything downstream. Think ahead. Design for change.

**Examples:**
- Hardcoding model name → Can't swap later
- Tight coupling → Changes cascade
- Skipping interface design → Can't plug in future tech

### ⛔ NO Multiple Choice Forms
The user hates restrictive form-style questions. Ask open questions naturally.

### Clean, Lean, Optimized
- No duplicate code
- No dead code
- Every line earns its place
- 4k lines achievable (NanoClaw proved it)

### Config Over Code
Behavior changes = config edit, not code change.

### The Dashboard is SACRED
VibePilot was designed from the dashboard backwards. **If the dashboard isn't showing what it should, the problem is in the Go code, NOT the dashboard.**

---

## 3. How the Vault Works

### ⛔ STOP. READ THIS. ⛔

**You WILL need to access Supabase or GitHub during this session. Here's EXACTLY how to do it without wasting 30% of your context window.**

### The Vault System

VibePilot uses a **two-tier credential system**:

| Credential Type | Location | Who Can Access |
|-----------------|----------|----------------|
| **Bootstrap Keys** | `~/.config/systemd/user/vibepilot-governor.service.d/override.conf` | User service (no sudo needed) |
| **All Other Keys** | Supabase Vault (encrypted) | Via vault_manager.py |

### Bootstrap Keys (User Service)
```bash
# These are in the systemd user service override file
cat ~/.config/systemd/user/vibepilot-governor.service.d/override.conf
```

Contains:
- `SUPABASE_URL` - Your Supabase project URL
- `SUPABASE_SERVICE_KEY` - Service role key (admin access)
- `VAULT_KEY` - Decrypts the Supabase vault

### ⛔ How to Access Supabase (Read This Carefully)

**Method 1: Extract from User Service Environment**

```bash
# Get env vars from the user systemd service
source <(systemctl --user show vibepilot-governor -p Environment | sed "s/Environment=//" | tr " " "\n" | grep -E "^(SUPABASE_URL|SUPABASE_SERVICE_KEY)=")

# Query tasks
curl -s "${SUPABASE_URL}/rest/v1/tasks?select=id,title,status" \
  -H "apikey: ${SUPABASE_SERVICE_KEY}" \
  -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}"
```

**Method 2: Use the Running Governor (RECOMMENDED)**

The governor binary has access to the environment. Use it to run database operations via Go code.
The Go vault package (`governor/internal/vault/vault.go`) handles all vault access.

**Method 3: Environment Variables from ~/.governor_env**
```bash
# Source the environment file
source ~/.governor_env
echo "URL: $SUPABASE_URL"

# Or extract from systemd user service
source <(systemctl --user show vibepilot-governor -p Environment | sed "s/Environment=//" | tr " " "\n")
```

### ⛔ What NOT to Do (Wastes Time)

❌ **DON'T look for .env files** - They don't exist
❌ **DON'T use `sudo systemctl`** - It's a user service, use `systemctl --user`
❌ **DON'T use `journalctl -u governor`** - Use `journalctl --user -u vibepilot-governor`
❌ **DON'T hardcode credentials** - Use the vault
❌ **DON'T guess** - Check what exists first

### ✅ Quick Reference: Database Operations

```bash
# Check for query tools
ls ~/VibePilot/governor/cmd/tools/

# Use the systemd user environment
source <(systemctl --user show vibepilot-governor -p Environment | sed "s/Environment=//" | tr " " "\n")
echo "URL: $SUPABASE_URL"

# Query Supabase via curl
source <(systemctl --user show vibepilot-governor -p Environment | sed "s/Environment=//" | tr " " "\n")
curl -s "${SUPABASE_URL}/rest/v1/tasks?select=id,title,status" \
  -H "apikey: ${SUPABASE_SERVICE_KEY}" \
  -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}"
```

### GitHub Access

```bash
# GitHub CLI is available
gh auth status
gh api repos/VibesTribe/VibePilot

# Or use git directly
cd ~/VibePilot
git status
git log --oneline -5
```

---

## 4. Coding Rules

### JSONB Everywhere
- No TEXT[] or UUID[] (PostgreSQL-specific)
- Use JSONB for all arrays/objects
- Works in any database, understood by any LLM

### No Vendor Lock-In
- Can swap: Database, Code Host, AI CLI, Hosting, Models
- Test: Can we swap [X] by changing config only? If no, refactor.

### Pass Slices Directly to RPCs
```go
// ✅ CORRECT
database.RPC(ctx, "update_task", map[string]any{
    "p_dependencies": []string{"T001", "T002"},
})

// ❌ WRONG (pre-marshaling)
jsonDeps, _ := json.Marshal([]string{"T001", "T002"})
database.RPC(ctx, "update_task", map[string]any{
    "p_dependencies": jsonDeps,
})
```

### All Schema Changes in `docs/supabase-schema/`
- Human applies from GitHub (source of truth)
- Numbered migrations: `064_update_task_assignment.sql`
- Never apply directly - commit to GitHub first

### File Organization
- 1 file = 1 concern
- Changes should touch one file max
- Code should fit in LLM context (~4-8k lines)

---

## 5. How the Dashboard Works

**Location:** `~/vibeflow/` (separate repo)

### ⛔ CRITICAL: Dashboard is READ-ONLY

The dashboard is a **view** of VibePilot state. It does NOT:
- Make decisions
- Route tasks
- Execute code

It ONLY displays what VibePilot has already done.

### If Dashboard Shows Wrong Data

**The problem is in the Go code, NOT the dashboard.**

Fix the Go code that writes to Supabase.

### What Dashboard Displays

| Section | Data Source | Update Method |
|---------|-------------|---------------|
| Status Pills | `tasks.status` | Realtime (instant) |
| Slice Hub | `tasks.slice_id` | Realtime (instant) |
| Task Cards | `tasks.*`, `task_runs.*` | Realtime (instant) |
| Agent Hangar | `models.*`, `platforms.*` | Realtime (instant) |
| ROI Panel | `task_runs.tokens_*`, `task_runs.*_cost_usd` | Realtime (instant) |
| Event Timeline | `orchestrator_events.*` | Realtime (instant) |

### Critical Fields Dashboard Expects

**tasks table:**
- `assigned_to` (text) - Model ID (e.g., "glm-5")
- `slice_id` (text) - Slice grouping
- `routing_flag` (text) - "internal", "mcp", or "web"
- `status` (text) - Task status
- `result` (jsonb) - Contains `prompt_packet`

**task_runs table:**
- `model_id` (text) - Which model executed
- `tokens_in`, `tokens_out` (int) - Token counts
- `platform_theoretical_cost_usd` (decimal) - Theoretical cost
- `total_actual_cost_usd` (decimal) - Actual cost
- `total_savings_usd` (decimal) - Savings

**Full details:** See `docs/HOW_DASHBOARD_WORKS.md`

---

## 6. Flow and Architecture

### Complete Flow: PRD → Completion

```
┌─────────────────────────────────────────────────────────────┐
│ 1. HUMAN PUSHES PRD TO GITHUB                               │
│    docs/prd/my-feature.md                                   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. GOVERNOR DETECTS PUSH (Supabase Live)                    │
│    - Monitors Supabase for new PRD records                  │
│    - Triggers Planner agent                                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. PLANNER CREATES PLAN + TASKS                             │
│    - Analyzes PRD                                           │
│    - Creates plan record in Supabase                        │
│    - Breaks into atomic tasks                               │
│    - Writes prompt packets                                   │
│    - Sets dependencies                                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. SUPERVISOR REVIEWS PLAN                                  │
│    - Validates task breakdown                               │
│    - Checks dependencies                                    │
│    - Approves or requests changes                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. TASKS BECOME AVAILABLE                                   │
│    - Tasks with met dependencies → status: available        │
│    - Governor emits EventTaskAvailable                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. ROUTER SELECTS MODEL AND ROUTING PATH                    │
│    - Checks task type, category, requirements               │
│    - Selects from active models/connectors                   │
│    - Writes to tasks.assigned_to                             │
│    - Sets routing_flag: "internal", "web", or "mcp"         │
│    - "web" = courier agent (external LLM via free API)       │
│    - "internal" = local worktree execution                   │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │ routing_flag?                  │
              ▼                               ▼
┌──────────────────────────┐  ┌─────────────────────────────────────────────────┐
│ 7a. INTERNAL EXECUTION   │  │ 7b. COURIER AGENT (routing_flag = "web")        │
│ (isolated worktree)      │  │                                                 │
│ - git worktree           │  │ Dispatches to GitHub Actions for browser-use:   │
│ - prompt packet sent     │  │ - Zero local weight (runs in cloud)             │
│ - model generates code   │  │ - Zero polling (Supabase Realtime results)      │
│ - commit to task branch  │  │                                                 │
│ - task_runs record       │  │ 1. executeCourierTask() builds packet           │
│                          │  │ 2. Vault decrypts key via deriveLLMKeyRef       │
│                          │  │ 3. CourierRunner.dispatch() → GitHub Actions    │
│                          │  │ 4. courier_run.py: browser-use + playwright     │
│                          │  │ 5. Result → task_runs via Supabase REST         │
│                          │  │ 6. Realtime fires EventCourierResult            │
│                          │  │ 7. NotifyResult() → waiting goroutine           │
│                          │  │ 8. Task → "review"                              │
└──────────────────────────┘  └─────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 8. SUPERVISOR REVIEWS OUTPUT                                │
│    - Supervisor reviews ALL outputs                         │
│    - Check against task prompt + expected output            │
│    - Approve → testing                                      │
│    - Reject → back to task runner                           │
│    (Council reviews PLANS, not outputs. Supervisor owns all │
│     output review. Council is only for plan/architecture.)  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 9. TESTER VALIDATES                                         │
│    - Runs tests                                             │
│    - Runs lint/typecheck                                    │
│    - Pass → ready for merge                                 │
│    - Fail → back to task runner                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 10. SYSTEM AUTO-MERGES (with shadow merge safety)           │
│     - Shadow merge: test for conflicts before real merge    │
│     - If conflicts: merge_pending, agents resolve           │
│     - If clean: task branch → Module branch (auto-merge)    │
│     - Worktree removed, task branch deleted                 │
│     - No human involvement for code                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 11. DASHBOARD SHOWS PROGRESS                                │
│     - Task status: complete                                 │
│     - Model assignment: which model did the work            │
│     - Token counts: how many tokens used                    │
│     - ROI: cost savings                                     │
└─────────────────────────────────────────────────────────────┘

---

## ⚠️ HUMAN ROLE (VERY LIMITED)

**Human ONLY does these 3 things:**

1. **Visual UI/UX review** - Looks at visual tester output. Requests changes or approves.
2. **Paid API benched** - When paid models run out of credit, human decides: add credits or keep benched.
3. **Research after council** - Receives Council-reviewed suggestions. Final yes/no.

**Human NEVER:**
- Reviews code
- Merges code
- Maintains the system
- Writes code
- Debugs code
- Fixes merge conflicts
- Does anything technical

**Merge system: fully automated, zero human involvement.**
Supervisor approves output -> testing -> auto-merge if tests pass.
Merge problems are solved by agents, not human.
```

### Architecture Components

```
~/VibePilot/                  # Main repo (also at ~/vibepilot/ -- running copy)
├── governor/                 # Go backend (THE BRAIN)
│   ├── cmd/governor/        # Main entry point + handlers
│   ├── internal/            # Core logic
│   │   ├── core/           # State machine, checkpointing, test runner
│   │   ├── db/             # Supabase client, RPCs
│   │   ├── runtime/        # Router, agents, sessions, context builder
│   │   ├── dag/            # DAG engine, YAML workflow execution
│   │   ├── gitree/         # Git worktree management (isolated worktrees per parallel agent)
│   │   ├── mcp/            # MCP protocol support
│   │   ├── connectors/     # Courier agents, runner connectors
│   │   ├── security/       # Leak detection
│   │   ├── vault/          # Supabase vault access
│   │   ├── webhooks/       # GitHub webhook handling
│   │   ├── realtime/       # Supabase realtime subscriptions
│   │   ├── maintenance/    # System maintenance operations
│   │   └── tools/          # Tool registry (db, file, git, sandbox, vault, web)
│   └── config/              # JSON configs + YAML pipelines
│       ├── pipelines/      # DAG pipeline YAML (code-pipeline.yaml)
│       ├── models.json     # Model definitions
│       ├── connectors.json # Connector definitions
│       ├── routing.json    # Routing strategy
│       └── *.json          # agents, platforms, roles, skills, tools, etc.
│
├── .context/                 # Knowledge layer (auto-generated, agent boot material)
│   ├── boot.md             # Agent orientation (~3764 tokens, Tier 0 first)
│   ├── knowledge.db        # SQLite: 30 rules, 30 prompts, 15 configs, 3337 docs, 364 schema, 17 pipelines
│   │                       # + schema_current (49 tables, 668 cols with Go/Dash cross-ref)
│   │                       # + schema_functions (148 RPCs, 49 traced to Go callers)
│   ├── map.md              # Full code map (functions, types, imports)
│   ├── index.db            # Code index (functions, dependencies)
│   └── tools/              # Build tools
│       ├── tier0-static.md # Single source of truth for all rules
│       └── build-knowledge-db.py  # Builds knowledge.db from tier0 + sources
│
├── prompts/                  # Agent system prompts (synced to Supabase on startup)
│
├── docs/
│   ├── prd/                 # Product Requirements (INPUT)
│   ├── plans/               # Generated plans (OUTPUT)
│   ├── supabase-schema/     # Database migrations (111 files)
│   ├── rate_limits/         # Multi-tier rate limit data (Gemini, DeepSeek, Kimi)
│   └── research/            # Historical research reports
│
├── research/                 # Current research (model rolodex, landscape analysis)
│
├── scripts/                  # Utility scripts (all portable, no hardcoded paths)
│
├── agents/                   # Agent definitions, flow diagrams
│
├── config/                   # Top-level configs + prompts
│   ├── prompts/             # Per-role prompt templates
│   ├── roles.json           # 13 role definitions
│   └── platforms.json       # Web AI platform configs
│
└── inbox/                    # Incoming research/ideas (from Kimi, etc.)

~/vibeflow/                   # Dashboard (SEPARATE REPO, Vercel auto-deploy)
├── apps/dashboard/          # React dashboard
└── src/                     # Core types, agents
```

### Worktree Strategy (Agent Isolation)

**The Problem:** Multiple agents sharing one directory with `git checkout -b` caused constant branch confusion. Agent A checks out task/T001, Agent B checks out task/T002, and now Agent A is on the wrong branch.

**The Solution:** One git worktree per task. Each agent works in a physically separate directory.

```
~/VibePilot/                 # Main repo (never checked out to a task branch)
~/VibePilot-work/{task1}/    # Agent 1's isolated checkout on task/T001
~/VibePilot-work/{task2}/    # Agent 2's isolated checkout on task/T002
~/VibePilot-work/{task3}/    # Agent 3's isolated checkout on task/T003
```

**Bootstrap:** Each worktree gets symlinks to shared resources:
- `governor/config/*.json` -- model definitions, routing, connectors
- `governor/config/prompts/` -- agent role templates
- `governor/config/pipelines/` -- pipeline definitions
- `.hermes.md` -- enforcement rules
- `.context/` -- knowledge layer (index.db, knowledge.db, map.md)

**Shadow Merge:** Before real merge, the system does a test merge to detect conflicts. If conflicts found, task goes to `merge_pending` for agent resolution instead of blowing up.

**Lifecycle:**
1. Task claimed → `CreateWorktree()` + `BootstrapWorktree()`
2. Agent executes in worktree (isolated directory)
3. Review/testing → `ShadowMerge()` before real merge
4. Merge success → `RemoveWorktree()` + delete branch
5. Task fail → `RemoveWorktree()` cleanup

**Fallback:** If worktree creation fails, falls back to legacy single-dir branch mode.

### Courier Agent System

Courier agents are the primary execution path for tasks routed to external free-tier LLMs. When `routing_flag = "web"`, the governor dispatches a courier instead of running code locally.

**How it works:**
```
Governor receives task → router selects routing_flag="web"
        │
        ▼
TaskHandler.executeCourierTask() builds courier packet
        │
        ▼
Vault.GetSecret(deriveLLMKeyRef(connectorID)) → decrypted API key
        │
        ▼
CourierRunner.dispatch() → repository_dispatch to GitHub Actions
        │
        ▼
GitHub Actions: ubuntu-latest + browser-use + playwright
        │
        ▼
courier_run.py: navigate platform URL → paste prompt → extract response
        │
        ▼
courier_run.py writes to task_runs via Supabase REST
        │
        ▼
Supabase Realtime fires UPDATE → EventCourierResult
        │
        ▼
CourierRunner.NotifyResult() → channel delivery to waiting goroutine
        │
        ▼
Tokens counted client-side → Cost calculated → transition_task → review
```

**Key files:**
| File | Role |
|------|------|
| `governor/internal/connectors/courier.go` | CourierRunner, GitHub Actions dispatch |
| `governor/cmd/governor/handlers_task.go` | Vault threading, courier packet assembly, deriveLLMKeyRef |
| `scripts/courier_run.py` | Python courier runner (GitHub Actions path) |

**Vault keys (courier-relevant):**
| Vault Key | Provider | Used By |
|-----------|----------|---------|
| `GROQ_API_KEY` | Groq | 7 free models |
| `OPENROUTER_API_KEY` | OpenRouter | 19 free models ($0 credit, max spend limit set) |
| `GEMINI_COURIER_KEY` | Google AI | Courier tasks (gemini-2.5-flash-lite) |
| `GEMINI_RESEARCHER_KEY` | Google AI | Research tasks (gemini-3.1-flash-lite-preview) |
| `GEMINI_VISUAL_TESTER_KEY` | Google AI | Visual QA (gemini-3-flash-preview) |
| `GEMINI_GENERAL_KEY` | Google AI | General/fallback (gemini-2.5-flash-lite) |
| `NVIDIA_API_KEY` | NVIDIA NIM | 3 free models |

**Implementation status:** Fully built and wired. All components exist: CourierRunner (courier.go), browser-use script (courier_run.py), GitHub Actions workflow (courier.yml), Supabase realtime result delivery. Not yet E2E tested (governor stopped).

---

## 6b. Self-Learning Feedback Loops

Every agent receives feedback from downstream outcomes. The system learns which models work best for which roles.

### Handler Learning Coverage (all verified by grep)

| Handler | Tracking Calls | Coverage |
|---------|---------------|----------|
| plan | 10 | 95% |
| council | 6 | 95% |
| task | 21 | 98% |
| testing | 3 | 90% |
| research | 4 | 90% |
| maint | 7 | 95% |

### What Gets Tracked
- **Supervisor model performance:** Tracked in plan review, task review, and research review. Correct rejections = success signal.
- **Per-model vote alignment:** Council members get success/failure based on consensus alignment.
- **Per-role learning:** Router learns which models make good supervisors, council members, planners.
- **Rule creation:** Supervisor AND council create planner rules on rejection. Approved plans reinforce active rules.
- **Failed model exclusion:** Test failures store failed model, excluded from next routing via ExcludeModels.

### Config ↔ DB Sync

When supervisor approves research findings (new_model, pricing_change, config_tweak, new_platform):
1. `ResearchActionApplier` writes config file (models.json or connectors.json) **and** upserts Supabase DB
2. Both sources update atomically — no LLM middleman, no drift
3. Fallback to maintenance command if direct apply fails
4. Thread-safe with mutex-protected config writes

---

## 7. GitHub and Supabase as Sources of Truth

### ⚠️ CRITICAL: Supabase Realtime (NEVER Poll)

**VibePilot uses Supabase Realtime subscriptions, NOT polling.** Polling nearly killed the project (Supabase egress costs). All dashboard updates, task state changes, and event tracking use realtime subscriptions.

GitHub push events are received via webhooks (webhooks.vibestribe.rocks) and written to Supabase. Governor watches Supabase via realtime -- never polls GitHub either.

### GitHub: Code & Schema Source of Truth

| What | Where | Why |
|------|-------|-----|
| PRDs | `docs/prd/*.md` | Human creates, Governor reads |
| Plans | `docs/plans/*.md` | Governor creates, tracks progress |
| Schema Migrations | `docs/supabase-schema/*.sql` | Human applies from GitHub |
| Agent Prompts | `prompts/*.md` | Configurable agent behavior |
| Config Files | `governor/config/*.json` | Models, connectors, routing |

### Supabase: State & Realtime Source of Truth

| What | Table | Why |
|------|-------|-----|
| Tasks | `tasks` | Task records, status, assignment |
| Task Runs | `task_runs` | Execution history, tokens, costs |
| Plans | `plans` | Plan records, metadata |
| Models | `models` | Model profiles, status, limits |
| Platforms | `platforms` | Web platforms (courier destinations) |
| Events | `orchestrator_events` | Event log for timeline |
| Review Queue | `review_queue` | Tasks awaiting human review |

### How They Work Together

```
┌─────────────────────────────────────────────────────────────┐
│ GITHUB (Source of Truth for Code)                           │
│                                                              │
│  - PRDs pushed here                                          │
│  - Migrations stored here                                    │
│  - Agent prompts stored here                                 │
│  - Config files stored here                                  │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Governor reads from here
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ GOVERNOR (Go Backend)                                        │
│                                                              │
│  - Watches Supabase for changes (Live)                      │
│  - Reads PRDs from GitHub                                    │
│  - Writes tasks to Supabase                                  │
│  - Updates task_runs with tokens/costs                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Writes state here
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ SUPABASE (Source of Truth for State)                         │
│                                                              │
│  - Realtime subscriptions (no webhooks)                     │
│  - Task records                                              │
│  - Execution history                                         │
│  - Model/platform status                                     │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Realtime subscriptions (NEVER poll)
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ DASHBOARD (React Frontend)                                   │
│                                                              │
│  - READ-ONLY view of Supabase                               │
│  - Displays task status, model assignment, ROI              │
│  - Human reviews visual UI/UX changes here ONLY             │
│    (NOT code, NOT merge, NOT general approval)              │
└─────────────────────────────────────────────────────────────┘
```

### Applying Schema Changes

1. Create migration in `docs/supabase-schema/XXX_description.sql`
2. Commit to GitHub main
3. Human copies SQL from GitHub
4. Human applies in Supabase SQL Editor
5. Governor restarts and uses new schema

**NEVER apply migrations directly. Always go through GitHub first.**

---

## 8. What is Where

### Quick Reference

| Need to... | Look in... | File/Command |
|------------|------------|--------------|
| Create a PRD | `docs/prd/` | `my-feature.md` |
| Check current state | Root | `CURRENT_STATE.md` |
| Understand architecture | Root | `ARCHITECTURE.md` |
| See recent changes | Root | `CHANGELOG.md` |
|| Find agent prompts | `prompts/` | `planner.md`, `supervisor.md`, etc. |
| Check model config | `governor/config/` | `models.json` |
| Check connector config | `governor/config/` | `connectors.json` |
| Check routing config | `governor/config/` | `routing.json` |
| Find database schema | `docs/supabase-schema/` | `*.sql` files |
| Understand dashboard | `docs/` | `HOW_DASHBOARD_WORKS.md` |
| Query Supabase | Use systemctl --user | See Section 3 |
| Check governor logs | System | `journalctl --user -u vibepilot-governor` |
| Restart governor | System | `systemctl --user restart vibepilot-governor` |

### Governor Internal Structure

```
governor/
├── cmd/
│   ├── governor/           # Main entry point
│   │   ├── main.go        # Starts everything
│   │   ├── adapters.go    # Agent adapters
│   │   ├── handlers_*.go  # Event handlers (plan, task, council, testing, research, maint)
│   │   ├── helpers.go     # Shared helpers
│   │   ├── recovery.go    # Crash recovery
│   │   ├── types.go       # Handler types
│   │   └── validation.go  # Input validation
│   ├── cleanup/            # Cleanup utility
│   ├── encrypt_secret/     # Vault encryption
│   └── migrate_vault/      # Vault migration
│
├── internal/
│   ├── core/              # Core orchestration
│   │   ├── checkpoint.go  # Checkpoint management
│   │   ├── state.go       # State definitions
│   │   ├── analyst.go     # Analysis logic
│   │   └── test_runner.go # Test execution
│   │
│   ├── db/                # Database layer
│   │   ├── supabase.go    # Client
│   │   ├── rpc.go         # RPC allowlist
│   │   └── state.go       # DB state tracking
│   │
│   ├── runtime/           # Agent runtime
│   │   ├── router.go      # Model/connector routing
│   │   ├── session.go     # Agent sessions
│   │   ├── parallel.go    # Parallel execution
│   │   ├── config.go      # Runtime config loading
│   │   ├── context_builder.go  # Context assembly
│   │   ├── model_loader.go     # Model config loading
│   │   ├── research_action.go  # Config↔DB sync on research approval
│   │   ├── usage_tracker.go    # Token/cost tracking
│   │   └── tools.go       # Runtime tools
│   │
│   ├── dag/                # DAG engine
│   │   ├── engine.go      # DAG execution engine
│   │   ├── workflow.go    # YAML workflow structs
│   │   └── registry.go    # DAG node registry
│   │
│   ├── gitree/            # Git branch management
│   │   └── gitree.go      # Orphan branches, commit, merge, clear
│   │
│   ├── mcp/               # MCP protocol
│   │   ├── executor.go    # MCP execution
│   │   └── registry.go    # MCP server registry
│   │
│   ├── connectors/        # External connectors
│   │   ├── courier.go     # Courier agent connector
│   │   └── runners.go     # Runner connectors
│   │
│   ├── security/          # Security
│   │   └── leak_detector.go # Secret scanning
│   │
│   ├── vault/             # Vault
│   │   └── vault.go       # Supabase vault access
│   │
│   ├── webhooks/          # Webhooks
│   │   ├── github.go      # GitHub webhook handler
│   │   └── server.go      # Webhook HTTP server
│   │
│   ├── realtime/          # Realtime
│   │   └── client.go      # Supabase realtime subscriptions
│   │
│   ├── maintenance/       # Maintenance
│   │   ├── maintenance.go  # Maintenance operations
│   │   ├── sandbox.go     # Sandbox management
│   │   └── validation.go  # Maintenance validation
│   │
│   └── tools/             # Tools
│       ├── registry.go    # Tool registry
│       ├── db_tools.go    # Database tools
│       ├── file_tools.go  # File tools
│       ├── git_tools.go   # Git tools
│       ├── sandbox_tools.go # Sandbox tools
│       ├── vault_tools.go # Vault tools
│       └── web_tools.go   # Web tools
│
├── config/                # JSON configs + YAML pipelines
│   ├── pipelines/         # DAG pipeline YAML
│   │   └── code-pipeline.yaml  # Main code pipeline
│   ├── models.json        # Model definitions
│   ├── connectors.json    # Connector definitions
│   ├── agents.json        # Agent definitions
│   ├── routing.json       # Routing strategy
│   ├── roles.json         # Role definitions
│   ├── skills.json        # Skill definitions
│   ├── tools.json         # Tool definitions
│   ├── platforms.json     # Platform definitions
│   ├── destinations.json  # Destination definitions
│   ├── system.json        # Runtime settings
│   └── plan_lifecycle.json # Plan state machine
│
└── pkg/types/             # Shared types
    └── types.go           # Common type definitions
```

### Key Files to Know

| File | Purpose |
|------|---------|
| `.context/boot.md` | Agent orientation (~3764 tokens). Tier 0 rules first, then codebase map. Auto-generated. |
| `.context/knowledge.db` | SQLite: 30 rules, 30 prompts, 15 configs, 3337 docs, 364 SQL schema, 17 pipeline stages. Queryable by agents. |
| `.context/tools/tier0-static.md` | Single source of truth for all principles, rules, roles. Hand-crafted. |
| `governor/cmd/governor/main.go` | Entry point, starts all services |
| `governor/cmd/governor/handlers_task.go` | Task execution logic |
| `governor/cmd/governor/handlers_plan.go` | Plan creation logic |
| `governor/internal/runtime/router.go` | Routing logic (SelectRouting) |
| `governor/internal/dag/engine.go` | DAG execution engine |
| `governor/internal/gitree/gitree.go` | Git branch + worktree management (isolated worktrees per parallel agent) |
| `governor/internal/db/supabase.go` | Database client |
| `governor/config/pipelines/code-pipeline.yaml` | YAML DAG pipeline definition |
| `governor/config/models.json` | Model definitions |
| `governor/config/connectors.json` | Connector definitions |
| `research/2026-04-14-free-model-rolodex.md` | Verified free provider cascade |
| `docs/rate_limits/` | Multi-tier rate limit data (RPM/TPM/RPD by tier) |
| `TODO.md` | Current TODO list with priorities |
| `CURRENT_STATE.md` | Honest current state assessment |

---

## 9. Deep Dive References

**When you need more detail than this file provides:**

| Document | When to Read | What It Contains |
|----------|--------------|------------------|
| [docs/HOW_DASHBOARD_WORKS.md](docs/HOW_DASHBOARD_WORKS.md) | Fixing dashboard display issues | Full dashboard data flow, all sections, field mappings |
| [docs/DASHBOARD_AUDIT.md](docs/DASHBOARD_AUDIT.md) | **CONTRACT: what dashboard needs from Supabase** | Exact columns, types, status maps, computed fields, mismatches, Go↔Dash contract |
| [docs/DATA_FLOW_MAPPING.md](docs/DATA_FLOW_MAPPING.md) | Understanding what Go code writes where | Dashboard → Supabase → Go code mapping, current gaps |
| [docs/supabase-schema/](docs/supabase-schema/) | Making schema changes | All database migrations, numbered SQL files |
| [docs/core_philosophy.md](docs/core_philosophy.md) | Understanding the "why" | Strategic mindset, principles, decision framework |

### Quick Schema Reference

**Core Tables:**
| Table | Purpose | Key Fields |
|-------|---------|------------|
| `tasks` | Work items | id, title, status, assigned_to, slice_id, routing_flag, dependencies |
| `task_runs` | Execution history | task_id, model_id, tokens_in, tokens_out, *_cost_usd |
| `models` | AI model registry | id, name, status, context_limit, subscription_* |
| `platforms` | Web platforms | id, name, status, config (free_tier, capabilities) |
| `plans` | Plan records | id, prd_id, status, plan_path |
| `orchestrator_events` | Event log | event_type, task_id, model_id, reason |

**Key RPCs:**
- `claim_next_task(courier, platform, model_id)` - Atomically claim task
- `update_task_status(task_id, status)` - Update task status
- `unlock_dependent_tasks(completed_task_id)` - Unlock waiting tasks

---

## Start of Session Checklist

**Every session, do this:**

1. ✅ Read this file (VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md)
2. ✅ Read CURRENT_STATE.md
3. ✅ Check git branch: `cd ~/VibePilot && git branch --show-current`
4. ✅ Check governor status: `systemctl --user status vibepilot-governor`
5. ✅ Check recent logs: `journalctl --user -u vibepilot-governor -n 50`
6. ✅ Verify you can access Supabase (see Section 3)

---

## Common Tasks Quick Reference

### Query Supabase
```bash
source <(systemctl --user show vibepilot-governor -p Environment | sed "s/Environment=//" | tr " " "\n")
curl -s "${SUPABASE_URL}/rest/v1/tasks?select=id,title,status" \
  -H "apikey: ${SUPABASE_SERVICE_KEY}" \
  -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}"
```

### Clean Up Test Data
```bash
source <(systemctl --user show vibepilot-governor -p Environment | sed "s/Environment=//" | tr " " "\n")
curl -X DELETE "${SUPABASE_URL}/rest/v1/tasks?id=not.is.null" \
  -H "apikey: ${SUPABASE_SERVICE_KEY}" \
  -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}"
curl -X DELETE "${SUPABASE_URL}/rest/v1/task_runs?id=not.is.null" \
  -H "apikey: ${SUPABASE_SERVICE_KEY}" \
  -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}"
curl -X DELETE "${SUPABASE_URL}/rest/v1/plans?id=not.is.null" \
  -H "apikey: ${SUPABASE_SERVICE_KEY}" \
  -H "Authorization: Bearer ${SUPABASE_SERVICE_KEY}"
```

### Rebuild and Restart Governor
```bash
cd ~/VibePilot/governor && go build -o governor ./cmd/governor && systemctl --user restart vibepilot-governor
```

### Check Governor Logs
```bash
journalctl --user -u vibepilot-governor --since "5 minutes ago" | tail -50
```

### Create a Test PRD
```bash
cat > ~/VibePilot/docs/prd/test-feature.md << 'EOF'
# PRD: Test Feature

Priority: Low
Complexity: Simple
Category: coding

## Context
Test description.

## What to Build
- Item 1
- Item 2

## Files
- file1.go
- file2.go

## Expected Output
- Expected result
EOF

cd ~/VibePilot && git add docs/prd/test-feature.md && git commit -m "test: add test PRD" && git push origin main
```

---

## Remember

- **Dashboard is READ-ONLY** - Fix Go code, not dashboard
- **NEVER poll Supabase** - Realtime subscriptions only (polling nearly killed the project)
- **No hardcoding** - Everything in config files
- **GitHub = Code source of truth**
- **Supabase = State source of truth**
- **User service, not system** - `systemctl --user`, not `sudo systemctl`
- **Cloud free tiers = primary** — Multiple providers, free and near-free. Counts change, see CURRENT_STATE.md.
- **Token counting = client-side** - Never trust external counts
- **4 Gemini projects = 4x free quota** - Independent keys, 60 RPM combined

**Need more detail?** See Section 9 for deep dive references.

**Questions?** Ask the human.

---

## Local PostgreSQL + Vault (April 23, 2026)

Supabase replaced with local PostgreSQL 16. Peer auth for user `vibes` (no password).

```bash
# Connect to database
psql -d vibepilot

# Connection string for apps
DATABASE_URL="postgres://vibes@/vibepilot?host=/var/run/postgresql"
```

### Vault Master Key

**VAULT_KEY:** Check `~/.config/systemd/user/vibepilot-governor.service.d/override.conf` or GitHub Secrets. Not stored in public repos.

```bash
# Vault CLI
export DATABASE_URL="postgres://vibes@/vibepilot?host=/var/run/postgresql"
export VAULT_KEY="$(grep VAULT_KEY ~/.config/systemd/user/vibepilot-governor.service.d/override.conf | cut -d'"' -f2)"
cd ~/vibepilot/governor

./governor vault list                    # List all key names
./governor vault set KEY_NAME "value"    # Add or update a key (copy paste done)
./governor vault get KEY_NAME            # Decrypt and show a key
./governor vault delete KEY_NAME         # Remove a key
```

### Governor Startup
```bash
# Rebuild after code changes
cd ~/vibepilot/governor && go build -o governor ./cmd/governor/

# Restart (systemd has all env vars)
systemctl --user daemon-reload
systemctl --user restart vibepilot-governor

# Verify
curl http://localhost:8080/status
```

### Dashboard API
```bash
# Full dashboard data (~185KB JSON)
curl http://localhost:8080/api/dashboard

# SSE live stream (for browser EventSource)
curl http://localhost:8080/api/dashboard/stream
```

### Backup
```bash
# PG dump + push to knowledgebase repo (runs daily at 3am via cron)
~/vibepilot/scripts/pg-dump-and-push.sh
```
