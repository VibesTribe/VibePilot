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
- **Codebase:** ~4-8k lines (fits in LLM context)
- **Hosting:** ThinkPad X220 (16GB RAM, i5-2520M, 781GB disk free)
- **Runtime:** Single Go binary (10-20MB)
- **Architecture:** Plug-and-play modules
- **Model Strategy:** Cloud free tier cascade (Groq, Google AI Studio, OpenRouter, SambaNova). Local inference too slow on this hardware.

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

**Method 3: Python with Vault Manager (IF IT EXISTS)**

```bash
# Check if vault_manager.py exists FIRST
ls ~/VibePilot/scripts/vault_manager.py

# If it exists, use it
python3 ~/VibePilot/scripts/vault_manager.py --action get --key SOME_API_KEY
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
│ 6. ROUTER SELECTS MODEL                                     │
│    - Checks task type, category, requirements               │
│    - Selects from active models/connectors                  │
│    - Writes to tasks.assigned_to                            │
│    - Writes to tasks.routing_flag                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 7. TASK RUNNER EXECUTES                                     │
│    - Creates task branch                                    │
│    - Sends prompt packet to model                           │
│    - Model generates code                                   │
│    - Commits to task branch                                 │
│    - Creates task_runs record with tokens/costs             │
└─────────────────────────────────────────────────────────────┘
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
│ 10. SYSTEM AUTO-MERGES                                      │
│     - Task branch → Module branch (auto-merge)              │
│     - Task branch deleted                                   │
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

**Human ONLY does these 2 things:**

1. **Visual UI/UX review** - Looks at visual tester output. Requests changes or approves.
2. **Architecture decisions** - Receives Council-reviewed suggestions. Asks questions. Final yes/no.

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
~/VibePilot/
├── governor/                 # Go backend (THE BRAIN)
│   ├── cmd/governor/        # Main entry point
│   ├── internal/            # Core logic
│   │   ├── core/           # State machine, checkpointing
│   │   ├── db/             # Supabase client, RPCs
│   │   ├── runtime/        # Router, agents, sessions
│   │   ├── dag/            # DAG engine, YAML workflow
│   │   ├── gitree/         # Git operations
│   │   └── security/       # Leak detection
│   └── config/              # JSON configs (models, connectors, etc.)
│
├── docs/
│   ├── prd/                 # Product Requirements (INPUT)
│   ├── plans/               # Generated plans (OUTPUT)
│   ├── supabase-schema/     # Database migrations
│   ├── prompts/             # Agent system prompts
│   └── *.md                 # Documentation
│
├── research/                # Model/provider research, landscape analysis
│
└── scripts/                 # Utility scripts

~/vibeflow/                   # Dashboard (SEPARATE REPO, Vercel auto-deploy)
├── apps/dashboard/          # React dashboard
└── src/                     # Core types, agents
```

---

## 7. GitHub and Supabase as Sources of Truth

### ⛔ CRITICAL: No Webhooks

**VibePilot uses Supabase Live (realtime subscriptions), NOT webhooks.**

Webhooks failed. We switched to Supabase Live.

### GitHub: Code & Schema Source of Truth

| What | Where | Why |
|------|-------|-----|
| PRDs | `docs/prd/*.md` | Human creates, Governor reads |
| Plans | `docs/plans/*.md` | Governor creates, tracks progress |
| Schema Migrations | `docs/supabase-schema/*.sql` | Human applies from GitHub |
| Agent Prompts | `docs/prompts/*.md` | Configurable agent behavior |
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
                              │ Dashboard polls every 5s
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
| Find agent prompts | `docs/prompts/` | `planner.md`, `supervisor.md`, etc. |
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
│   │   ├── handlers_*.go  # Event handlers
│   │   └── state.go       # State machine
│   └── tools/             # Utility commands (if any)
│
├── internal/
│   ├── core/              # Core orchestration
│   │   ├── checkpoint.go  # Checkpoint management
│   │   └── state.go       # State definitions
│   │
│   ├── db/                # Database layer
│   │   ├── supabase.go    # Client
│   │   └── rpc.go         # RPC allowlist
│   │
│   ├── runtime/           # Agent runtime
│   │   ├── router.go      # Model/connector routing
│   │   ├── session.go     # Agent sessions
│   │   ├── factory.go     # Session factory
│   │   └── pool.go        # Agent pool
│   │
│   ├── dag/                # DAG engine
│   │   ├── engine.go      # DAG execution engine
│   │   └── workflow.go    # YAML workflow structs
│   │
│   ├── gitree/            # Git operations
│   │   └── gitree.go      # Branch, commit, merge
│   │
│   └── security/          # Security
│       └── leak_detector.go # Secret scanning
│
└── config/                # JSON configs
    ├── models.json        # Model profiles
    ├── connectors.json    # CLI/API/Web destinations
    ├── agents.json        # Agent definitions
    └── routing.json       # Routing strategy
```

### Key Files to Know

| File | Purpose |
|------|---------|
| `governor/cmd/governor/main.go` | Entry point, starts all services |
| `governor/cmd/governor/handlers_task.go` | Task execution logic |
| `governor/cmd/governor/handlers_plan.go` | Plan creation logic |
| `governor/internal/runtime/router.go` | Routing logic (SelectDestination) |
| `governor/internal/dag/engine.go` | DAG execution engine |
| `governor/internal/db/supabase.go` | Database client |
| `governor/config/models.json` | Model definitions |
| `governor/config/connectors.json` | Connector definitions |
| `research/2026-04-14-free-model-rolodex.md` | Verified free provider cascade |
| `docs/HOW_DASHBOARD_WORKS.md` | Dashboard data flow |

---

## 9. Deep Dive References

**When you need more detail than this file provides:**

| Document | When to Read | What It Contains |
|----------|--------------|------------------|
| [docs/HOW_DASHBOARD_WORKS.md](docs/HOW_DASHBOARD_WORKS.md) | Fixing dashboard display issues | Full dashboard data flow, all sections, field mappings |
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
- **No webhooks** - We use Supabase Live
- **No hardcoding** - Everything in config files
- **GitHub = Code source of truth**
- **Supabase = State source of truth**
- **User service, not system** - `systemctl --user`, not `sudo systemctl`
- **Cloud free tiers = primary** - Local inference too slow on x220

**Need more detail?** See Section 9 for deep dive references.

**Questions?** Ask the human.
