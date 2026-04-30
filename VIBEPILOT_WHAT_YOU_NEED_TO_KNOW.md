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
- **Cost Tracking System:** Every model touch recorded in task_runs with exact/estimated tokens and cost data. 4-phase system deployed (see Section 6d). `record_internal_run` RPC for planner/supervisor/analyst calls. `aggregate_task_costs` on completion. Dashboard ROI panel with 10 fixes (Apr 29). Analyst agent handles diagnostic ceiling (see Section 6c). Decision needed: Z.AI ($200/3mo) vs DeepSeek V4 Flash (~$51/mo) vs DeepSeek V4 Pro (~$160/mo discounted).
- **Config/DB Sync:** Research findings write to config/models.json AND PostgreSQL atomically via `ResearchActionApplier`. No LLM middleman. Both sources stay in sync automatically.
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

## 3. How to Access the Database and Vault

### ⛔ STOP. READ THIS. ⛔

**The database is LOCAL PostgreSQL. Supabase was cut off April 24 (egress killed by polling). Here's EXACTLY how to do it.**

### Quick Database Access

```bash
# Connect directly (peer auth, no password)
psql -d vibepilot

# Connection string for apps/scripts
DATABASE_URL="postgres://vibes@/vibepilot?host=/var/run/postgresql"

# Query examples
psql -d vibepilot -c "SELECT id, title, status FROM tasks LIMIT 10"
psql -d vibepilot -c "\df"                    # list all functions
psql -d vibepilot -c "\dt"                    # list all tables
psql -d vibepilot -c "\df unlock_*"           # find specific functions
```

### Vault (Encrypted Secrets)

All API keys stored in `secrets_vault` table, AES-256-GCM encrypted.

```bash
# Get VAULT_KEY from systemd override
export VAULT_KEY="$(grep VAULT_KEY ~/.config/systemd/user/vibepilot-governor.service.d/override.conf | cut -d'"' -f2)"

# Use governor CLI to manage secrets
cd ~/vibepilot/governor
./governor vault list                    # List all key names
./governor vault set KEY_NAME "value"    # Add or update a key
./governor vault get KEY_NAME            # Decrypt and show a key
./governor vault delete KEY_NAME         # Remove a key
```

15 keys stored: GROQ, OPENROUTER, GEMINI (4 projects), NVIDIA, DEEPSEEK, GITHUB_TOKEN, ZAI_API_KEY, webhook_secret, Gmail SSO.

### ⛔ What NOT to Do

❌ **DON'T look for .env files** - They don't exist
❌ **DON'T curl Supabase REST API** - It's gone. Local PG only.
❌ **DON'T use `sudo systemctl`** - It's a user service, use `systemctl --user`
❌ **DON'T use `journalctl -u governor`** - Use `journalctl --user -u vibepilot-governor`
❌ **DON'T hardcode credentials** - Use the vault
❌ **DON'T guess** - Check what exists first

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
- Numbered migrations: `132_description.sql`
- Applied directly via `psql -d vibepilot -f path/to/migration.sql`
- Always commit to GitHub first, then apply locally

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

Fix the Go code that writes to PostgreSQL.

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

**This is the canonical pipeline. Every handler, every agent, every test must align with this.**

```
┌─────────────────────────────────────────────────────────────┐
│ 1. PRD PUSHED TO GITHUB                                     │
│    docs/prd/[feature-name].md                               │
│    GitHub webhook → governor detects docs/prd/ path         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. PLANNER CREATES PLAN + TASKS                             │
│    - Reads PRD from GitHub                                  │
│    - Creates plan record (status: draft)                    │
│    - Breaks into atomic tasks with prompt packets           │
│    - Sets dependencies between tasks                        │
│    - Writes plan to docs/plans/[name]-plan.md               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. SUPERVISOR REVIEWS PLAN                                  │
│    - Validates: confidence scores, dependencies, prompt     │
│      packets, internal markers, alignment with PRD          │
│    - Approve → orchestrator                                 │
│    - Reject → planner revises                               │
│                                                              │
│    For complex plans: Council also reviews                   │
│    - Council checks architecture and design decisions        │
│    - Council ONLY reviews PLANS, never outputs               │
│    - Supervisor triggers council when needed                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. ORCHESTRATOR ASSIGNS TASKS                               │
│    - Selects best active model per task                      │
│    - Assigns no-dependency tasks first                       │
│    - Dependency tasks locked until deps complete             │
│    - Sets routing_flag: "internal", "web", or "mcp"         │
│    - "web" = courier agent (external LLM via free API)       │
│    - "internal" = local worktree execution                   │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │ routing_flag?                  │
              ▼                               ▼
┌──────────────────────────┐  ┌─────────────────────────────────────────────────┐
│ 5a. INTERNAL EXECUTION   │  │ 5b. COURIER AGENT (routing_flag = "web")        │
│ (isolated worktree)      │  │                                                 │
│ - git worktree per task  │  │ Dispatches to GitHub Actions for browser-use:   │
│ - prompt packet sent     │  │ - Zero local weight (runs in cloud)             │
│ - model generates code   │  │ - Zero polling (realtime results)               │
│ - commit to task branch  │  │                                                 │
│ - task_runs record       │  │ CourierRunner.dispatch() → GitHub Actions       │
│                          │  │ courier_run.py: browser-use + playwright        │
│                          │  │ Result → task_runs → realtime → review          │
└──────────────────────────┘  └─────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. SUPERVISOR REVIEWS OUTPUT                                │
│    - Sees EVERYTHING on the task branch (git diff main)     │
│    - Compares original prompt vs actual deliverables        │
│    - Handles any output type: code, video, images, docs     │
│    - Binary files shown as [binary file, N bytes]           │
│    - Model judges quality, not regex                        │
│    - Approve → testing                                      │
│    - Reject → back to task runner with feedback             │
│    - Supervisor owns ALL output review                      │
│    - Council NEVER reviews outputs                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 7. TESTING HANDLER (three layers)                           │
│                                                              │
│    Layer 1: Output Artifact Validation                      │
│    - Verify files in expected_output exist                  │
│    - Validate format (JSON parses, HTML has structure)      │
│                                                              │
│    Layer 2: Semgrep Static Analysis                         │
│    - semgrep --config auto on worktree                      │
│    - ERROR severity findings = test failure                 │
│    - WARNING severity = logged but passes                   │
│                                                              │
│    Layer 3: Native Test Suite (if project has one)          │
│    - go.mod → go build + go test                            │
│    - package.json with test script → npm test               │
│    - pyproject.toml → pytest                                │
│    - No project file → skip (pure output task)              │
│                                                              │
│    Pass → merge to module branch                            │
│    Fail → back to task runner                               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 8. TASK MERGE TO MODULE BRANCH                              │
│    - Shadow merge: test for conflicts before real merge     │
│    - Clean merge: task branch → module branch               │
│    - Delete task branch, remove worktree                    │
│    - Unlock dependent tasks                                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 9. MODULE INTEGRATION TEST (when all module tasks merged)   │
│    - Full module tested as integrated unit                  │
│    - Semgrep on complete module                             │
│    - Native test suite on module branch                     │
│    - Pass → merge module to project main                    │
│    - Fail → identify failing task, back to runner           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 10. MODULE MERGE TO PROJECT MAIN                            │
│     - Module branch → project's main branch                │
│     - Delete module branch                                 │
│     - Dashboard shows completed module                      │
│     - No human involvement for code                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 11. DASHBOARD SHOWS PROGRESS                                │
│     https://vibeflow-dashboard.vercel.app/                  │
│     - Task status, model assignment, token counts, ROI     │
│     - Real-time via SSE from governor /api/dashboard/stream │
└─────────────────────────────────────────────────────────────┘

KEY DISTINCTIONS:
- SUPERVISOR reviews: plan quality (step 3) + output alignment (step 6)
- COUNCIL reviews: complex plan architecture (step 3 only, supervisor triggers)
- TESTING HANDLER: code correctness (step 7, never reviews alignment)
- HUMAN reviews: visual UI/UX only, never code/merge/debug
```

---

## Multi-Project Architecture

**VibePilot is an orchestrator. It can build ANY project, not just itself.**

### Three Tiers of Usage

| Tier | Example | Task Scope | Target Repo |
|------|---------|-----------|-------------|
| Focused change | "Fix the docs button on dashboard" | 1-3 tasks, existing files | vibeflow (or any repo) |
| Cross-repo project | "Build knowledgebase with graph view" | 10+ tasks, multiple repos | knowledgebase + vibeflow + vibepilot |
| Full greenfield | "Build Extra Yum recipe app" | Entire project, new repo | brand new repo |

### How It Works

```
VibePilot (orchestrator repo)         Target Project (any repo)
┌──────────────────────┐              ┌──────────────────────┐
│ docs/prd/            │              │ src/                 │
│ docs/plans/          │              │ tests/               │
│ governor/            │              │ package.json         │
│ config/              │              │ ...                  │
└──────────────────────┘              └──────────────────────┘
         │                                     │
         │ PRDs live here                       │ Worktrees clone this
         │ Plans live here                      │ Code changes go here
         │ Pipeline runs here                   │ Tests run here
```

### What's Needed (not yet implemented)

- `projects` table needs: `repo_owner`, `repo_name`, `base_branch`
- `create_plan` RPC takes `project_id` which determines target repo
- Handlers read project config instead of hardcoded repo references
- Worktrees clone the TARGET project repo, not always vibepilot
- Testing handler detects project type from the TARGET repo

### Current State

Right now all tasks target vibepilot itself (hardcoded VibesTribe/VibePilot in 4 handler locations).
This is acceptable for initial E2E testing. Multi-project routing is the next architectural milestone.

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
│   │   ├── db/             # PostgreSQL client, RPCs (postgres.go, NOT supabase.go — legacy dead code)
│   │   ├── runtime/        # Router, agents, sessions, context builder
│   │   ├── dag/            # DAG engine, YAML workflow execution
│   │   ├── gitree/         # Git worktree management (isolated worktrees per parallel agent)
│   │   ├── mcp/            # MCP protocol support
│   │   ├── connectors/     # Courier agents, runner connectors
│   │   ├── security/       # Leak detection
│   │   ├── vault/          # PostgreSQL vault access (encrypted secrets)
│   │   ├── webhooks/       # GitHub webhook handling
│   │   ├── realtime/       # pg_notify listener (replaced Supabase Realtime)
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
├── prompts/                  # Agent system prompts (synced to DB on startup for legacy compat)
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
courier_run.py POSTs result to governor /api/courier/result
(via cloudflare tunnel: https://webhooks.vibestribe.rocks)
        │
        ▼
Governor handleCourierResult → record_courier_result RPC → local PG task_runs
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

**Implementation status:** Fully built and wired. All components exist: CourierRunner (courier.go), browser-use script (courier_run.py), GitHub Actions workflow (courier_dispatch.yml), governor callback endpoint (/api/courier/result via cloudflare tunnel). Result delivery: local PG task_runs + channel notification. Not yet E2E tested.

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
1. `ResearchActionApplier` writes config file (models.json or connectors.json) **and** upserts PostgreSQL DB
2. Both sources update atomically — no LLM middleman, no drift
3. Fallback to maintenance command if direct apply fails
4. Thread-safe with mutex-protected config writes

---

## 6c. Analyst Agent (Diagnostic Ceiling)

When a task fails repeatedly, the system doesn't loop forever. After `diagnostic_trigger_attempts` failures (default 2, configured in `system.json`), the analyst agent kicks in:

1. **Trigger**: `handleTaskReview` checks attempt count against ceiling before retrying
2. **Analyst runs**: `runAnalystDiagnosis()` gathers all failure feedback, sends to analyst model with role `"analyst"`
3. **Parser**: `ParseAnalystDecision()` extracts structured diagnosis (root_cause, action, excluded_models, guidance)
4. **Routing**: `routeAnalystDecision()` takes action based on diagnosis:
   - `re_prompt`: reset to pending with new guidance from analyst
   - `exclude_and_retry`: exclude failed model(s), reset to pending
   - `split_task`: flag for planner to break into subtasks
   - `escalate`: surface to human as genuinely blocked
5. **Cost tracking**: Analyst model call recorded via `record_internal_run` with role `"analyst"`
6. **Pipeline event**: `analyst_diagnosis` event logged to orchestrator_events

**Key files**: `handlers_task.go` (runAnalystDiagnosis, routeAnalystDecision), `prompts/analyst.md`, `runtime/` (ParseAnalystDecision, AnalystDecision type)

**Config**: `system.json` → `execution.diagnostic_trigger_attempts` (default 2)

---

## 6d. Cost Tracking System (deployed 2026-04-29)

Every model touch is now tracked in `task_runs` with token counts and cost data.

### Data Flow

```
Model call completes → record_internal_run(task_id, role, model_id, tokens, cost)
  → INSERT INTO task_runs (role, token_source, tokens_in, tokens_out, costs)

Task completes testing → aggregate_task_costs(task_id)
  → UPDATE tasks SET total_tokens_in/out, total_cost_usd, model_count FROM task_runs

Dashboard ROI panel → reads tasks + task_runs + subscription_history
```

### RPCs
- `record_internal_run`: Creates task_run rows for planner, supervisor, analyst model calls
- `aggregate_task_costs`: Sums all task_runs into task-level totals (called on task completion)
- `calc_run_costs`: Calculates costs for a single run based on model pricing

### Tables
- `task_runs`: per-invocation records with `role`, `token_source`, `total_actual_cost_usd`
- `tasks`: aggregated totals `total_tokens_in`, `total_tokens_out`, `total_cost_usd`, `model_count`
- `subscription_history`: tracks all subscriptions over time, persists when archived
- `project_snapshots`: archive/clear functionality for project totals

### Token Estimation
- API models: use reported token counts
- Web/courier models: ~4 chars/token estimated from output length
- Dashboard shows both live (session) and project (cumulative) modes via header pill toggle

### Dashboard ROI (vibeflow, Apr 29)
- 10 fixes: subscription_history wired, token display, formatting, visibility
- USD/CAD converter (1.36 default, live rate from DB or exchangerate-api.com)
- Header alerts banner polls `/api/project/alerts` every 60s for subscription/credit warnings

---

## 7. GitHub and PostgreSQL as Sources of Truth

### Data Flow: Who Talks to What

```
┌─────────────────────────────────────────────────────────────┐
│ GITHUB (Source of Truth for Code)                           │
│  - PRDs, plans, migrations, prompts, configs               │
│  - Push events trigger pipeline via webhook                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ GOVERNOR (Go Backend, reads both)                           │
│  - Reads PRDs from GitHub                                   │
│  - Reads/writes state to local PostgreSQL                   │
│  - pg_notify events drive the pipeline                      │
│  - SSE broker pushes live updates to dashboard              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ POSTGRESQL (Source of Truth for State, LOCAL)               │
│  - 66 tables, 152 functions                                 │
│  - Tasks, task_runs, plans, models, platforms, events       │
│  - pg_notify on vp_changes channel                          │
│  - Encrypted vault (secrets_vault table)                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ DASHBOARD (React, READ-ONLY view)                           │
│  - Reads from governor /api/dashboard (REST + SSE)          │
│  - Does NOT connect to PostgreSQL directly                  │
│  - Human reviews visual UI/UX changes here ONLY             │
└─────────────────────────────────────────────────────────────┘
```

### GitHub: Code Source of Truth

| What | Where | Why |
|------|-------|-----|
| PRDs | `docs/prd/*.md` | Human creates, Governor reads |
| Plans | `docs/plans/*.md` | Governor creates, tracks progress |
| Schema Migrations | `docs/supabase-schema/*.sql` | Numbered, applied via psql |
| Agent Prompts | `prompts/*.md` | Configurable agent behavior |
| Config Files | `governor/config/*.json` | Models, connectors, routing |

### PostgreSQL: State Source of Truth (LOCAL)

| What | Table | Why |
|------|-------|-----|
| Tasks | `tasks` | Task records, status, assignment |
| Task Runs | `task_runs` | Execution history, tokens, costs |
| Plans | `plans` | Plan records, metadata |
| Models | `models` | Model profiles, status, limits |
| Platforms | `platforms` | Web platforms (courier destinations) |
| Events | `orchestrator_events` | Event log for timeline |
| Secrets | `secrets_vault` | Encrypted API keys (AES-256-GCM) |

### Applying Schema Changes

1. Create migration in `docs/supabase-schema/NNN_description.sql` (find next number)
2. Include `DROP IF EXISTS` for idempotency
3. Commit and push to GitHub main
4. `cd ~/vibepilot && git pull`
5. `psql -d vibepilot -f /path/to/migration.sql`
6. Verify: `psql -d vibepilot -c "\df function_name"`

**Always commit to GitHub first. Local-only work gets lost.**

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
|| Check model config | `governor/config/` | `models.json` |
|| Check connector config | `governor/config/` | `connectors.json` |
|| Check routing config | `governor/config/` | `routing.json` |
|| Find database schema | `docs/supabase-schema/` | `*.sql` files |
|| Understand dashboard | `docs/` | `HOW_DASHBOARD_WORKS.md` |
|| Query database | Use psql | See Section 3 |
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
│   │   ├── postgres.go    # PostgreSQL client (local PG, not Supabase)
│   │   ├── rpc.go         # RPC allowlist (101 entries)
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
| `governor/internal/db/postgres.go` | PostgreSQL client (local PG) |
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
| [docs/DASHBOARD_AUDIT.md](docs/DASHBOARD_AUDIT.md) | **CONTRACT: what dashboard needs from PostgreSQL** | Exact columns, types, status maps, computed fields, mismatches, Go↔Dash contract |
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
- `unlock_dependent_tasks(completed_task_id)` - Unlock waiting tasks (sets locked/pending → available)
- `set_processing(p_table, p_id, p_processing_by)` - Claim any record atomically
- `transition_task(p_task_id, p_new_status)` - Status transition with JSONB merge

**RPC Allowlist:** 101 entries in `internal/db/rpc.go`. Any RPC not in this list is silently rejected at runtime. If adding a new RPC call, MUST add to allowlist first.

---

## Start of Session Checklist

**Every session, do this:**

1. ✅ Read this file (VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md)
2. ✅ Read CURRENT_STATE.md
3. ✅ Check git branch: `cd ~/VibePilot && git branch --show-current`
4. ✅ Check governor status: `systemctl --user status vibepilot-governor`
5. ✅ Check recent logs: `journalctl --user -u vibepilot-governor -n 50`
6. ✅ Verify database access: `psql -d vibepilot -c "SELECT count(*) FROM tasks"`

---

## Common Tasks Quick Reference

### Query PostgreSQL
```bash
psql -d vibepilot -c "SELECT id, title, status FROM tasks LIMIT 10"
psql -d vibepilot -c "SELECT task_id, model_id, tokens_in, tokens_out FROM task_runs ORDER BY created_at DESC LIMIT 10"
```

### Clean Up Test Data
```bash
psql -d vibepilot -c "DELETE FROM task_runs WHERE task_id IN (SELECT id FROM tasks WHERE title LIKE 'test%')"
psql -d vibepilot -c "DELETE FROM tasks WHERE title LIKE 'test%'"
psql -d vibepilot -c "DELETE FROM plans WHERE title LIKE 'test%'"
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
# PRD: [Feature Name]

Priority: Low|Medium|High
Complexity: Simple|Moderate|Complex
Category: coding|configuration|documentation

## Context
Why this feature exists. What problem it solves.

## What to Build
- Specific deliverable 1
- Specific deliverable 2

## Files
- path/to/file1.ext - purpose of this file
- path/to/file2.ext - purpose of this file

## Expected Output
- What success looks like (specific, measurable)

## Constraints
- Do NOT modify [specific files or systems]
- Must work with [specific technology or framework]
EOF

cd ~/VibePilot && git add docs/prd/test-feature.md && git commit -m "prd: [feature name]" && git push origin main
```

---

## Remember

- **Dashboard is READ-ONLY** - Fix Go code, not dashboard
- **NEVER poll** - pg_notify events drive everything (polling killed Supabase)
- **No hardcoding** - Everything in config files
- **GitHub = Code source of truth**
- **PostgreSQL = State source of truth** (local, peer auth)
- **User service, not system** - `systemctl --user`, not `sudo systemctl`
- **Cloud free tiers = primary** — Multiple providers, free and near-free. Counts change, see CURRENT_STATE.md.
- **Token counting = client-side** - Never trust external counts
- **4 Gemini projects = 4x free quota** - Independent keys, 60 RPM combined

**Need more detail?** See Section 9 for deep dive references.

**Questions?** Ask the human.

---

## Local PostgreSQL + Vault (April 23, 2026)

Supabase replaced with local PostgreSQL 16. Peer auth for user `vibes` (no password).
Supabase project cut off April 24 due to excessive egress from redundant polling (never poll again).

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

---

## DASHBOARD REFERENCE AUDIT (Validated April 25, 2026)

**Read this instead of re-reading dashboard source code. Every field, status, and data flow traced from actual code.**

### Task Statuses (THESE ARE THE ONLY VALID TASK STATUSES)

```
available      Task created with no dependencies, ready to claim immediately
pending        Task created with dependencies, waiting for deps to complete
locked         Task claimed/processing by an agent
in_progress    Orchestrator assigned to model, executing
received       Courier/web agent returned results
review         Supervisor reviewing output quality
testing        Testing handler running (3 layers)
complete       Passed all gates, ready for merge
merge_pending  Shadow merge detected conflicts, awaiting resolution
merged         Merged to module/project branch
failed         Failed at some stage, has notes, awaiting intelligent reassignment
human_review   Awaiting human decision (visual UI/UX, researcher suggestions after council, API credit exhaustion)
```

**NO OTHER STATUSES ARE VALID FOR TASKS.** No `assigned`, no `blocked`, no `council_review` for tasks. Council reviews PLANS only. Tasks only go to `human_review` for the three mandatory human triggers: visual UI/UX review, researcher suggestions after council, and API credit exhaustion.

### Task Status Flow

```
available → in_progress → received → review → testing → complete → merge_pending → merged
pending → (dep completes → available) → in_progress → ...
                ↘ locked (claimed by agent)
                ↘ failed (with notes → back to pending for intelligent reassignment)

Visual/UI/UX tasks take a different path after testing:
testing → human_review (human must approve before merge) → complete → merge_pending → merged
```

- `canTransition()`: forward-only, any status can transition to `failed`
- `failed` tasks get notes, go back to `pending` for intelligent reassignment
- System learns from every failure at every stage

### What Dashboard Displays (per component)

**TaskSnapshot (core/types.ts):**
- `id` - Task ID
- `title` - Task title
- `status` - One of the 9 valid TaskStatus values
- `confidence` - Confidence score (0-1)
- `updatedAt` - ISO timestamp
- `owner` - Model/provider assigned (e.g. "groq:gemma-2-9b-it")
- `lessons` - Array of {title, summary} from learning
- `sliceId` - Slice grouping
- `taskNumber` - Task number within slice
- `location` - TaskLocation object: {kind: "platform"|"mcp"|"internal", label, link?/endpoint?}
- `dependencies` - Array of task IDs this depends on
- `packet` - {prompt: string, attachments?: {label, href}[]}
- `summary` - Task summary
- `mergePending` - Boolean
- `metrics` - {tokensUsed?, runtimeSeconds?, costUsd?}

**AgentSnapshot (core/types.ts):**
- `id`, `name`, `status`, `summary`, `updatedAt`
- `logo?`, `tier?`, `cooldownReason?`, `costPerRunUsd?`
- `vendor?`, `capability?`, `contextWindowTokens?`, `effectiveContextWindowTokens?`
- `cooldownExpiresAt?`, `creditStatus?` ("available"|"low"|"depleted"|"unknown")
- `rateLimitWindowSeconds?`, `costPer1kTokensUsd?`, `warnings?`

**FailureSnapshot:** `id`, `title`, `summary`, `reasonCode`

**MergeCandidate:** `branch`, `title`, `summary`, `checklist` (boolean[])

**ModelAnalyticsView:** `id`, `started_at`, `status` (completed|failed|pending), `notes`

### How Dashboard Gets Data

1. **Governor SSE stream** at `http://localhost:8080/api/dashboard/stream`
2. **Governor REST API** at `http://localhost:8080/api/dashboard` (~185KB JSON)
3. Dashboard does NOT connect to PostgreSQL directly
4. Dashboard does NOT connect to Supabase (was cut off April 24)
5. Governor reads PostgreSQL, serves to dashboard

### Dashboard Internal Architecture

**Orchestrator (core/orchestrator.ts):**
- `plan(objectives)` → builds TaskPacket[] via Planner
- `dispatch(packet)` → Router selects provider, emits "in_progress" event, saves task state
- Creates snapshots with status "in_progress" and owner = provider

**Router (core/router.ts):**
- Scores providers by: priority(0.28), confidence(0.26), successRate(0.24), latency(0.14), penalty(0.08)
- Returns `{skillId, provider, confidence}`

**Planner (core/planner.ts):**
- Maps objectives → TaskPacket[] with auto-generated IDs
- Each has taskId, title, objectives, deliverables, confidence, editScope

**MCP Server (mcp/server.ts):**
- Tools: runSkill, getTaskState, emitNote, queryEvents
- `/run-task` endpoint: validates TaskPacket, dispatches via orchestrator, queues to disk
- Port 3030 by default

**Agents (all in src/agents/):**
- All take TaskPacket, return {summary, confidence, deliverables}
- PrdAgent, PlannerAgent, DevAgent, DesignAgent, SupervisorAgent, TesterAgent, WatcherAgent, AnalystAgent, ResearchAgent, MaintenanceAgent
- SupervisorAgent: validates task state against JSON schema, runs TesterAgent
- TesterAgent: spawns skill runners (validate_output, run_visual_tests), returns passed/failed/error
- WatcherAgent: drift detection stub

**TaskState (core/taskState.ts):**
- Persisted to `data/state/task.state.json`
- Shape: {tasks: TaskSnapshot[], agents: AgentSnapshot[], failures: FailureSnapshot[], merge_candidates: MergeCandidate[], metrics: Record<string,unknown>, updated_at: string}

**Events (utils/events.ts):**
- Stored in `data/state/events.log.jsonl` (JSONL)
- Quality derived from events: "pass" if positive status/event, "fail" if negative or error
- POSITIVE_STATUSES: complete, merged, merge_pending
- NEGATIVE_STATUSES: failed

### What Dashboard Needs from Go Governor API

The `/api/dashboard` endpoint must return JSON containing:
1. `tasks` - Array matching TaskSnapshot shape (with all fields above)
2. `agents` - Array matching AgentSnapshot shape
3. `failures` - Array matching FailureSnapshot shape
4. `merge_candidates` - Array matching MergeCandidate shape
5. `metrics` - Key-value metrics
6. `updated_at` - ISO timestamp

Each task must have:
- `status` as one of the 9 valid TaskStatus values (no others)
- `owner` showing which model it was assigned to
- `location` with kind (platform/mcp/internal), label, and link/endpoint
- `packet` with the prompt and any attachments
- `metrics` with token usage, runtime, cost

### Plan Statuses (separate from task statuses)

Plans have their own lifecycle (see `plan_lifecycle.json`). Council reviews PLANS only, never tasks.

### Council Triggers

Council can be called for:
1. Complex plan architecture review (supervisor triggers)
2. System researcher suggested updates
3. Architecture improvements

### Human Role (VERY LIMITED)

1. Visual UI/UX review only
2. Paid API benched decisions (credit depleted)
3. Research suggestions after council review

Human NEVER: reviews code, merges code, debugs, writes code, does anything technical.
