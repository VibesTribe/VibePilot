# VibePilot Architecture

**THE SINGLE SOURCE OF TRUTH. Read this FIRST.**

**Last Updated:** 2026-03-04 (Session 49)
**Updated By:** GLM-5

---

## Table of Contents

1. [What Is VibePilot](#1-what-is-vibepilot)
2. [Core Principles](#2-core-principles)
3. [Coding Rules](#3-coding-rules)
4. [Architecture Overview](#4-architecture-overview)
5. [Complete Flow](#5-complete-flow)
6. [Components & Files](#6-components--files)
7. [Configuration System](#7-configuration-system)
8. [Security & Vault](#8-security--vault)
9. [Webhooks](#9-webhooks)
10. [Quick Reference](#10-quick-reference)

---

## 1. What Is VibePilot

**VibePilot is an AI execution engine that autonomously builds production systems.**

### Target System
**Webs of Wisdom** - A global, massively scalable, multilingual, multimedia social media platform where people pass down stories and wisdom to descendants and anyone globally, across languages and time.

### What Makes VibePilot Different

| Vibe Coding (Gemini in 30 seconds) | VibePilot |
|------------------------------------|-----------|
| Todo app, hardcoded | Global scale, configurable |
| Breaks on changes | Survives updates |
| No quality control | Supervisor + Council review |
| Single model, single pass | Multi-agent, revision loops |
| Static | Self-learning, self-improving |
| Dead on launch | Evolves with new models/strategies |
| Toy apps | Production-grade systems |

### Constraints

| Constraint | Value | Why |
|------------|-------|-----|
| Codebase size | 4,000-8,000 lines Go | Fits in LLM context |
| RAM footprint | 10-20MB | e2-micro free tier (1GB total) |
| Deployment | Single binary | Portable, deployable anywhere |
| Architecture | Plug-and-play modules | Swap models, tools, agents without code changes |

---

## 2. Core Principles

### The Inviolable Rules

| Principle | The Test |
|-----------|----------|
| **Zero Vendor Lock-In** | Can we replace [X] in one day with zero code changes? |
| **Modular & Swappable** | Change one thing. Did anything else break? |
| **Exit Ready** | Can someone else take over tomorrow with zero friction? |
| **Reversible** | Can you revert in 5 minutes? If not, don't do it. |
| **Always Improving** | Did we consider a better way? |

### Swappability Matrix

| Component | Can Swap To | How |
|-----------|-------------|-----|
| **Database** | Supabase → PostgreSQL → MySQL → SQLite | JSONB everywhere |
| **Code Host** | GitHub → GitLab → Bitbucket | Git-based, no API lock-in |
| **AI CLI** | OpenCode → Claude CLI → Gemini CLI → Anything | Config-driven connectors |
| **Hosting** | GCP → AWS → Azure → Local | Single binary, config files |
| **Models** | Any LLM with any provider | Routing config |

### Prevention Over Cure

- **Type 1 errors** (fundamental design mistakes) cost 100x more to fix later
- **Prevention = 1% of cure cost**
- Always think ahead, design for change
- If it can't be undone, it can't be done

---

## 3. Coding Rules

### From GO_IRON_STACK.md (Claw Patterns)

| Rule | Source | Why |
|------|--------|-----|
| **1 file = 1 concern** | NanoClaw | Changes touch one file max |
| **No ORM** | NanoClaw | Direct SQL, no abstraction bloat |
| **<500 lines per file** | NanoClaw | main.go is exception, being split |
| **Config-driven** | ZeroClaw | Behavior = config edit, not code change |
| **No vendor-specific features** | All | TEXT[] is PostgreSQL-only → use JSONB |
| **Leak detection on outputs** | IronClaw | Scan for secrets before committing |
| **Credential injection at boundary** | IronClaw | Secrets from vault, never in context |
| **10-20MB footprint** | ZeroClaw | Single binary, minimal deps |

### JSONB Rules

1. **JSONB for arrays/objects** - Works everywhere, LLM-friendly
2. **No TEXT[] or UUID[]** - PostgreSQL-only, not portable
3. **Pass slices directly to RPCs** - No pre-marshaling
4. **All schema in `docs/supabase-schema/`** - GitHub is source of truth

### File Naming Conventions

| Pattern | Example | Purpose |
|---------|---------|---------|
| `handlers_*.go` | `handlers_task.go` | Event handlers by domain |
| `0XX_*.sql` | `061_webhook_secret.sql` | Schema migrations in order |
| `*_test.go` | `main_test.go` | Go test files |
| `*.json` | `connectors.json` | Configuration files |

### What NOT To Do

| Don't | Why |
|-------|-----|
| Hardcode anything | "Temporary" becomes permanent |
| Use pkg/types.Task | Use map[string]any for flexibility |
| Delete without understanding | Could be "duplicate" CSS all over again |
| Assume code is dead | Could be planned functionality |
| Push dashboard/UI to main | Vercel auto-deploys, breaks production |
| Use multiple choice forms | User hates them |

---

## 4. Architecture Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         EXTERNAL WORLD                               │
│  GitHub (PRDs, Code)    Supabase (DB)     Web Platforms (AI Chat)   │
└─────────────┬─────────────────┬──────────────────┬─────────────────┘
              │                 │                  │
              │ Webhooks        │ Webhooks         │ Browser Automation
              ▼                 ▼                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         GOVERNOR (Go Binary)                         │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ Webhook      │  │ Event        │  │ Session Factory          │  │
│  │ Server:8080  │→ │ Router       │→ │ (Creates AI sessions)    │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                            │                                         │
│                            ▼                                         │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                    EVENT HANDLERS                             │  │
│  │  EventPlanCreated → EventTaskAvailable → EventTaskReview     │  │
│  │  EventPRDReady → EventRevisionNeeded → EventCouncilReview    │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                            │                                         │
│                            ▼                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ Router       │  │ Agent Pool   │  │ Connectors               │  │
│  │ (Routing)    │→ │ (Concurrency)│→ │ (CLI, API, Courier)      │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ Gitree       │  │ Vault        │  │ Leak Detector            │  │
│  │ (Git Ops)    │  │ (Secrets)    │  │ (Security)               │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
              │                 │                  │
              ▼                 ▼                  ▼
        Git Branches      Supabase RPCs      Task Outputs
        (task/feature)    (status updates)   (committed to branches)
```

### Key Concepts

| Concept | Definition | Examples |
|----------|------------|----------|
| **Connector** | HOW we connect to a model | CLIRunner, APIRunner, CourierRunner |
| **Destination** | WHERE couriers go (passed as parameter) | chatgpt.com, claude.ai |
| **Agent** | WHAT job is done | planner, supervisor, task_runner |
| **Model** | WHO provides intelligence | gemini-2.0-flash, deepseek-chat |
| **Strategy** | WHY/selection priority | internal_only, external_first |

### Routing Architecture

```
Task needs execution
    ↓
Router.SelectDestination(agentID, taskType)
    ↓
Get strategy for agent (from routing.json)
    ↓
Get category priority (e.g., ["internal"])
    ↓
Get connectors in category (from connectors.json, filtered by type)
    ↓
Filter by status="active"
    ↓
Return first available connector
    ↓
SessionFactory.Create(agentID)
    ↓
ConnectorRunner.Run(prompt)
```

---

## 5. Complete Flow

### PRD to Task Execution Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│ PHASE 1: PRD CREATION                                               │
├─────────────────────────────────────────────────────────────────────┤
│ 1. Human creates PRD in docs/prd/*.md                               │
│ 2. Git push to GitHub                                               │
│ 3. GitHub webhook → Governor :8080/webhooks                         │
│ 4. github.go detects PRD file                                       │
│ 5. Calls create_plan RPC                                            │
│ 6. Plan inserted (status='draft')                                   │
└─────────────────────────────────────────────────────────────────────┘
                                    ↓
┌─────────────────────────────────────────────────────────────────────┐
│ PHASE 2: PLAN CREATION                                              │
├─────────────────────────────────────────────────────────────────────┤
│ 7. Supabase webhook fires on plans INSERT                           │
│ 8. Governor receives webhook                                        │
│ 9. EventPlanCreated → Planner                                       │
│ 10. Planner creates tasks (status='pending')                        │
│ 11. Tasks with no dependencies → status='available'                 │
└─────────────────────────────────────────────────────────────────────┘
                                    ↓
┌─────────────────────────────────────────────────────────────────────┐
│ PHASE 3: TASK EXECUTION                                             │
├─────────────────────────────────────────────────────────────────────┤
│ 12. Supabase webhook fires on tasks INSERT/UPDATE                   │
│ 13. Governor receives webhook                                       │
│ 14. EventTaskAvailable → Task Runner                                │
│ 15. Router selects connector (opencode, deepseek-api, etc.)         │
│ 16. Session created, prompt sent to AI                              │
│ 17. AI produces output                                              │
│ 18. Leak detection scans output                                     │
│ 19. Output committed to task branch                                 │
│ 20. Task status → 'review'                                          │
└─────────────────────────────────────────────────────────────────────┘
                                    ↓
┌─────────────────────────────────────────────────────────────────────┐
│ PHASE 4: REVIEW                                                      │
├─────────────────────────────────────────────────────────────────────┤
│ 21. EventTaskReview → Supervisor                                    │
│ 22. Supervisor reviews output                                       │
│ 23. Decision: pass/fail/revise                                      │
│     - pass → status='testing'                                       │
│     - fail → status='available' (retry) or 'failed' (exhausted)     │
│     - revise → EventRevisionNeeded                                  │
└─────────────────────────────────────────────────────────────────────┘
                                    ↓
┌─────────────────────────────────────────────────────────────────────┐
│ PHASE 5: TESTING & MERGE                                            │
├─────────────────────────────────────────────────────────────────────┤
│ 24. EventTestResults → Test Runner                                  │
│ 25. Tests run (if configured)                                       │
│ 26. Pass → Merge to module branch → Merge to main                   │
│ 27. Fail → Back to revision                                         │
│ 28. Cleanup: Delete task/module branches                            │
└─────────────────────────────────────────────────────────────────────┘
```

### Task States

```
pending → locked → available → in_progress → review → testing → approval → merged
          (has deps) (unlocked)  (dispatched)  (done)   (tests)  (waiting)  (complete)
                               ↘ escalated (3 failures)
```

### Plan States

```
draft → review → revision_needed → council_review → approved
          ↓            ↓                ↓
      blocked      incomplete      (complex plans)
```

---

## 6. Components & Files

### Directory Structure

```
vibepilot/
├── ARCHITECTURE.md              ← YOU ARE HERE (read first!)
├── CURRENT_STATE.md             ← Current status (what's done/in progress)
├── CHANGELOG.md                 ← Full history of all changes
├── AGENTS.md                    ← Agent workflow guide (references this file)
├── SESSION_HANDOFF.md           ← Session-to-session continuity
│
├── governor/                    ← ACTIVE - Go binary
│   ├── cmd/governor/            ← Main entry + event handlers
│   │   ├── main.go              ← Entry point, wireup (752 lines)
│   │   ├── handlers_task.go     ← Task event handlers (531 lines)
│   │   ├── handlers_plan.go     ← Plan event handlers (576 lines)
│   │   ├── handlers_council.go  ← Council handlers (469 lines)
│   │   ├── handlers_research.go ← Research handlers (397 lines)
│   │   ├── handlers_testing.go  ← Test handlers (157 lines)
│   │   ├── handlers_maint.go    ← Maintenance handler (92 lines)
│   │   ├── recovery.go          ← Checkpoint recovery (255 lines)
│   │   ├── validation.go        ← Validation logic (276 lines)
│   │   └── main_test.go         ← Integration tests (405 lines)
│   │
│   ├── internal/
│   │   ├── connectors/          ← HOW we connect to models
│   │   │   ├── runners.go       ← CLIRunner, APIRunner (393 lines)
│   │   │   └── courier.go       ← CourierRunner for web (239 lines)
│   │   │
│   │   ├── runtime/             ← Core runtime
│   │   │   ├── events.go        ← Event types, EventRouter (631 lines)
│   │   │   ├── router.go        ← Routing logic (163 lines)
│   │   │   ├── session.go       ← Session management (185 lines)
│   │   │   ├── config.go        ← Config loading (1021 lines)
│   │   │   └── parallel.go      ← AgentPool for concurrency (210 lines)
│   │   │
│   │   ├── webhooks/            ← Webhook infrastructure
│   │   │   ├── server.go        ← Webhook server (291 lines)
│   │   │   └── github.go        ← GitHub webhook handler (129 lines)
│   │   │
│   │   ├── core/                ← State machine, checkpoints (857 lines)
│   │   ├── db/                  ← Supabase client (569 lines)
│   │   ├── vault/               ← Secret decryption (337 lines)
│   │   ├── gitree/              ← Git operations (379 lines)
│   │   ├── security/            ← Leak detection (69 lines)
│   │   └── tools/               ← Tool registry (~1,228 lines)
│   │
│   └── config/                  ← JSON configurations
│       ├── connectors.json      ← All connectors (cli, api, web)
│       ├── routing.json         ← Routing strategies
│       ├── models.json          ← AI model definitions
│       ├── agents.json          ← Agent definitions
│       ├── system.json          ← System configuration
│       └── plan_lifecycle.json  ← Plan state rules
│
├── prompts/                     ← Agent behavior (.md files)
│   ├── planner.md               ← Planner agent prompt
│   ├── supervisor.md            ← Supervisor agent prompt
│   ├── council.md               ← Council member prompt
│   └── ...                      ← Other agent prompts
│
├── docs/
│   ├── supabase-schema/         ← SQL migrations (source of truth)
│   │   ├── 057_task_checkpoints.sql
│   │   ├── 058_jsonb_parameters.sql
│   │   ├── 060_rls_dashboard_safe.sql
│   │   ├── 061_webhook_secret.sql
│   │   └── ...                  ← Other migrations
│   │
│   ├── prd/                     ← Product Requirements Documents
│   ├── core_philosophy.md       ← Strategic mindset
│   └── ...                      ← Other documentation
│
├── scripts/                     ← Utility scripts
│   ├── e2e-checkpoint-test.sh   ← E2E test
│   ├── deploy-governor.sh       ← Deploy script
│   └── ...                      ← Other scripts
│
└── legacy/                      ← DEAD - Python (kept for reference)
```

### Component Responsibilities

| Package | Purpose | Lines | Status |
|---------|---------|-------|--------|
| `cmd/governor/` | Entry point + handlers | 4,072 | Active |
| `internal/runtime/` | Events, sessions, config | 3,545 | Active |
| `internal/connectors/` | CLI, API, Courier runners | 632 | Active |
| `internal/webhooks/` | Webhook server | 420 | Active |
| `internal/core/` | State machine, checkpoints | 857 | Active + Planned |
| `internal/db/` | Supabase operations | 569 | Active |
| `internal/gitree/` | Git operations | 379 | Active |
| `internal/vault/` | Secret decryption | 337 | Active |
| `internal/security/` | Leak detection | 69 | Active |
| `internal/tools/` | Tool registry | 1,228 | Active |

### Event Handlers (17 Total)

| Handler | File | Triggers |
|---------|------|----------|
| EventPlanCreated | handlers_plan.go | Plan inserted |
| EventPRDReady | handlers_plan.go | Plan has prd_path |
| EventPlanReview | handlers_plan.go | Plan needs review |
| EventRevisionNeeded | handlers_plan.go | Plan needs revision |
| EventPlanApproved | handlers_plan.go | Plan approved |
| EventPlanBlocked | handlers_plan.go | Plan blocked |
| EventPlanError | handlers_plan.go | Plan error |
| EventPRDIncomplete | handlers_plan.go | PRD incomplete |
| EventTaskAvailable | handlers_task.go | Task ready |
| EventTaskReview | handlers_task.go | Task output ready |
| EventTaskCompleted | handlers_task.go | Task done |
| EventCouncilReview | handlers_council.go | Council needed |
| EventCouncilDone | handlers_council.go | Council complete |
| EventResearchReady | handlers_research.go | Research ready |
| EventResearchCouncil | handlers_research.go | Research council |
| EventMaintenanceCmd | handlers_maint.go | Maintenance command |
| EventTestResults | handlers_testing.go | Test results ready |

---

## 7. Configuration System

### Config Files (governor/config/)

| File | Purpose | When to Edit |
|------|---------|--------------|
| `connectors.json` | WHERE tasks execute (cli/api/web) | Add/remove/modify runners |
| `routing.json` | WHY/strategy (priorities, restrictions) | Change routing logic |
| `models.json` | WHO provides intelligence (LLMs) | Add/remove models |
| `agents.json` | WHAT job is done (agent definitions) | Add/remove agents |
| `system.json` | HOW system behaves (timeouts, limits) | Tune performance |
| `plan_lifecycle.json` | Plan state rules | Change plan flow |

### connectors.json Structure

```json
{
  "destinations": [
    {
      "id": "opencode",
      "type": "cli",
      "status": "active",
      "command": "opencode",
      "cli_args": ["run", "--format", "json"],
      "timeout_seconds": 300,
      "provides_tools": ["read", "write", "bash", "webfetch", "edit", "glob", "grep"]
    },
    {
      "id": "deepseek-api",
      "type": "api",
      "status": "active",
      "endpoint": "https://api.deepseek.com",
      "api_key_ref": "DEEPSEEK_API_KEY",
      "models_available": ["deepseek-chat"]
    },
    {
      "id": "chatgpt-web",
      "type": "web",
      "status": "active",
      "url": "https://chatgpt.com",
      "requires_auth": "browser_login"
    }
  ]
}
```

**Types:**
- `cli` - Command-line tools (opencode, kimi)
- `api` - HTTP API endpoints (deepseek-api, gemini-api)
- `web` - Browser platforms (chatgpt-web, claude-web) - handled by CourierRunner

### routing.json Structure

```json
{
  "default_strategy": "default",
  "strategies": {
    "default": {"priority": ["external", "internal"]},
    "internal_only": {"priority": ["internal"]}
  },
  "agent_restrictions": {
    "internal_only": ["planner", "supervisor", "council", "orchestrator"],
    "default": ["consultant", "researcher", "courier", "task_runner"]
  },
  "destination_categories": {
    "external": {"check_field": "type", "check_values": ["web"]},
    "internal": {"check_field": "type", "check_values": ["cli", "api"]}
  }
}
```

### How Routing Works

1. Agent needs to execute → `Router.SelectDestination(agentID, taskType)`
2. Get strategy for agent (e.g., "internal_only" for planner)
3. Get priority order (e.g., `["internal"]`)
4. For each category in priority:
   - Get connectors matching category (type=cli or type=api)
   - Filter by status="active"
   - Return first available
5. If nothing available → log error, task skipped

---

## 8. Security & Vault

### Bootstrap Credentials (Environment)

These are injected by systemd, NOT in code or config:

| Variable | Purpose | Where Stored |
|----------|---------|--------------|
| `SUPABASE_URL` | Database endpoint | systemd override |
| `SUPABASE_SERVICE_KEY` | Admin access (bypasses RLS) | systemd override |
| `VAULT_KEY` | Master decryption key | systemd override |
| `GITHUB_TOKEN` | Git operations | systemd override |
| `DEEPSEEK_API_KEY` | API key for DeepSeek | systemd override |

**Location:** `/etc/systemd/system/vibepilot-governor.service.d/override.conf`

### Vault Architecture

```
Bootstrap (environment, systemd injects):
├── SUPABASE_URL
├── SUPABASE_SERVICE_KEY (NOT anon - bypasses RLS)
└── VAULT_KEY (master decryption)

All other secrets:
├── Encrypted in secrets_vault table
├── RLS: service_role full access, authenticated read-one-at-a-time
└── Decrypted at runtime with VAULT_KEY
```

### secrets_vault Table

```sql
CREATE TABLE secrets_vault (
  key_name TEXT PRIMARY KEY,
  encrypted_value TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Adding a Secret

1. Generate plaintext value
2. Encrypt: `python3 scripts/vault_manager.py encrypt "secret_value"`
3. Store: `INSERT INTO secrets_vault (key_name, encrypted_value) VALUES ('new_secret', '...')`
4. Reference in config: `"api_key_ref": "NEW_SECRET"`

### Leak Detection

All task outputs are scanned for:
- API keys
- Tokens
- Passwords
- URLs with credentials

If leaks found, they're redacted before committing.

---

## 9. Webhooks

### Webhook Architecture

```
GitHub Push                    Supabase Table Change
    ↓                                 ↓
GitHub Webhook                  Supabase Webhook
    ↓                                 ↓
POST :8080/webhooks             POST :8080/webhooks
(X-GitHub-Event header)         (Supabase payload)
    ↓                                 ↓
github.go handler               server.go maps to event
    ↓                                 ↓
Detects docs/prd/*.md           mapToEventType()
    ↓                                 ↓
Creates plan via RPC            EventRouter.Route(event)
    ↓                                 ↓
Plan inserted                   Handler executes
    ↓
[Supabase webhook fires]
    ↓
Full flow continues...
```

### GitHub Webhooks

**Purpose:** Detect PRD file changes, create plans

**File:** `governor/internal/webhooks/github.go`

**Trigger:** Push events to any branch

**Logic:**
1. Parse push event
2. Check added/modified files in `docs/prd/*.md`
3. For each PRD file:
   - Call `create_plan` RPC
   - Plan inserted with status='draft'

### Supabase Webhooks

**Purpose:** Fire events on table changes

**Tables Monitored:**
| Table | Events | Fires When |
|-------|--------|------------|
| `plans` | INSERT, UPDATE | Plan created or status changed |
| `tasks` | INSERT, UPDATE | Task created or status changed |
| `research_suggestions` | INSERT, UPDATE | Research item added |
| `maintenance_commands` | INSERT | Maintenance command queued |
| `test_results` | INSERT | Test results available |

**Event Mapping (in server.go):**
```go
case table == "plans" && payload.Record["prd_path"] != nil:
    return string(runtime.EventPRDReady)
case table == "plans":
    return string(runtime.EventPlanCreated)
case table == "tasks":
    switch record["status"] {
    case "available": return EventTaskAvailable
    case "review": return EventTaskReview
    // ... etc
    }
```

### Webhook Configuration

**In Supabase Dashboard:**
1. Database → Webhooks → Create
2. Name: `governor-events`
3. URL: `http://<GCE-IP>:8080/webhooks`
4. Secret: From vault (`webhook_secret`)
5. Tables: Select from list above
6. Events: INSERT, UPDATE

**In system.json:**
```json
{
  "webhooks": {
    "enabled": true,
    "port": 8080,
    "path": "/webhooks",
    "secret_vault_key": "webhook_secret"
  }
}
```

---

## 10. Audit Findings (Session 49)

### What's Working vs Not Wired

| Component | Status | Lines | Notes |
|-----------|--------|-------|-------|
| **Connectors** | ✅ Active | 632 | CLIRunner, APIRunner working |
| **Router** | ✅ Active | 163 | Routes by strategy + category |
| **Webhooks** | ✅ Active | 420 | GitHub + Supabase |
| **Event Handlers** | ✅ Active | 4,072 | All 17 handlers wired |
| **Checkpoint Recovery** | ✅ Active | 255 | Works on startup |
| **Leak Detection** | ✅ Active | 69 | Scans all outputs |
| **StateMachine** | ⚠️ Created, not used | 302 | Passed to handlers but they call RPCs directly |
| **CheckpointManager** | ⚠️ Created, not used | 143 | Same - handlers use direct RPC |
| **TestRunner** | ⚠️ Not wired | 296 | Created but never invoked |
| **Analyst** | ⚠️ Not wired | 116 | Created but never scheduled |
| **CourierRunner** | ⚠️ Not wired | 239 | Web platforms not yet automated |
| **Maintenance** | ⚠️ Needs refactor | 759 | Uses pkg/types.Task, should use map[string]any |
| **PollingWatcher** | ❌ Obsolete | ~400 | Replaced by webhooks, can delete |
| **PRDWatcher** | ❌ Obsolete | 164 | Replaced by GitHub webhooks, can delete |

### Code That Can Be Removed

| Component | Lines | Why | Status |
|-----------|-------|-----|--------|
| PollingWatcher | ~400 | Replaced by Supabase webhooks | ✅ REMOVED Session 49 |
| PRDWatcher | 164 | Replaced by GitHub webhooks | ✅ REMOVED Session 49 |
| **Total** | ~564 | | |

### Code That Looks Unused But ISN'T

| Component | Lines | Why It Exists |
|-----------|-------|---------------|
| core/state.go | 302 | Planned - cleaner state abstraction |
| core/checkpoint.go | 143 | Planned - cleaner checkpoint API |
| core/test_runner.go | 296 | Planned - automated testing |
| core/analyst.go | 116 | Planned - daily self-improvement |
| connectors/courier.go | 239 | Planned - web platform execution |
| maintenance/*.go | 759 | Needs refactoring, not deletion |
| pkg/types/types.go | 122 | Dependency of maintenance |

**These are NOT dead code.** They are planned infrastructure awaiting wiring.

### Supabase Gaps

| Gap | Status | Impact |
|-----|--------|--------|
| `prd_files` table | Doesn't exist | Not needed - GitHub webhooks create plans directly |
| 6th webhook | Not configured | Only 5 webhooks needed (no prd_files) |
| E2E flow verified | ❓ Unknown | Need to run DIAGNOSTIC_WEBHOOK_CHECK.sql |

### Connector Types

| Type | Executable? | Handler | Status |
|------|-------------|---------|--------|
| `cli` | ✅ Yes | CLIRunner | Active |
| `api` | ✅ Yes | APIRunner | Active |
| `web` | ❌ No | CourierRunner | Not wired |

The Router intentionally skips `web` type connectors (see router.go:84-86). They're for future browser automation.

### Current Connector Status

| ID | Type | Status | RAM |
|----|------|--------|-----|
| opencode | cli | ✅ Active | ~700MB |
| kilo | cli | Inactive | ~350MB (50% less) |
| kimi | cli | Inactive | Unknown |
| deepseek-api | api | ✅ Active | Minimal |
| gemini-api | api | Inactive | Minimal |
| groq-api | api | Pending key | Minimal |
| openrouter-api | api | Emergency fallback | Minimal |
| chatgpt-web | web | Active (not directly executable) | N/A |
| claude-web | web | Active (not directly executable) | N/A |
| gemini-web | web | Active (not directly executable) | N/A |
| + 5 more web | web | Various | N/A |

**For e2-micro (1GB RAM):** Consider switching from opencode to kilo for 50% RAM savings.

---

## 11. Quick Reference

### Key Files to Read (In Order)

1. **ARCHITECTURE.md** (this file) - Single source of truth
2. **CURRENT_STATE.md** - What's done, what's in progress
3. **CHANGELOG.md** - Full history of changes
4. **SESSION_HANDOFF.md** - Session continuity

### Common Commands

| Command | Action |
|---------|--------|
| `systemctl status vibepilot-governor` | Check if running |
| `journalctl -u vibepilot-governor -f` | Live logs |
| `journalctl -u vibepilot-governor -n 50 \| grep -i connector` | Check connector registration |
| `cd ~/vibepilot/governor && go build -o governor ./cmd/governor` | Build binary |
| `cd ~/vibepilot/governor && go test ./cmd/governor/...` | Run tests |
| `sudo systemctl restart vibepilot-governor` | Restart service |
| `~/vibepilot/scripts/e2e-checkpoint-test.sh` | E2E checkpoint test |

### CLI Tools (OpenCode vs Kilo)

| Command | OpenCode | Kilo |
|---------|----------|------|
| Interactive mode | `opencode` | `kilo` |
| One-shot task | `opencode run "task"` | `kilo run "task"` |
| Autonomous mode | Limited | `kilo run --auto "task"` |
| Check auth | `opencode auth list` | `kilo auth list` |
| List models | `opencode models` | `kilo models` |
| RAM usage | ~700MB | ~350MB (50% less) |

**Auth location:**
- OpenCode: `~/.local/share/opencode/auth.json`
- Kilo: `~/.local/share/kilo/auth.json` (same format)

### GCE Maintenance

**Check RAM usage:**
```bash
free -h
ps aux --sort=-%mem | head -10
```

**Check disk usage:**
```bash
df -h
du -sh ~/.local ~/.cache
```

**Clean up space (saves ~1.6GB):**
```bash
rm -rf ~/.local/share/opencode ~/.cache/go-build
```

**Current RAM hogs:**
| Process | RAM | Notes |
|---------|-----|-------|
| opencode/kilo | 350-700MB | AI CLI session |
| gopls | ~350MB | Go language server |
| governor | ~35MB | VibePilot |
| systemd-journald | ~100MB | Logging |

### File Locations Quick Reference

| What | Where |
|------|-------|
| Main entry point | `governor/cmd/governor/main.go` |
| Event handlers | `governor/cmd/governor/handlers_*.go` |
| Connectors | `governor/internal/connectors/` |
| Webhooks | `governor/internal/webhooks/` |
| Config files | `governor/config/*.json` |
| Agent prompts | `prompts/*.md` |
| Schema migrations | `docs/supabase-schema/*.sql` |
| Bootstrap secrets | `/etc/systemd/system/vibepilot-governor.service.d/override.conf` |

### Database Tables

| Table | Purpose |
|-------|---------|
| `tasks` | Work items |
| `plans` | Plans from PRDs |
| `task_runs` | Execution history |
| `task_checkpoints` | Crash recovery |
| `models` | AI model registry |
| `secrets_vault` | Encrypted secrets |
| `council_reviews` | Multi-model reviews |
| `maintenance_commands` | Git command queue |

### Status Values

**Tasks:**
- `pending` → Has dependencies
- `available` → Ready to execute
- `in_progress` → Currently running
- `review` → Output ready for review
- `testing` → Tests running
- `merged` → Complete
- `failed` → Exhausted retries

**Plans:**
- `draft` → Just created
- `review` → Needs review
- `revision_needed` → Needs changes
- `council_review` → Complex, needs council
- `approved` → Ready for task creation

---

## After Every Session, Update

1. **ARCHITECTURE.md** - If architecture changed
2. **CURRENT_STATE.md** - What was done, what's next
3. **CHANGELOG.md** - Full audit trail
4. **SESSION_HANDOFF.md** - If critical context for next session

---

**This document is the single source of truth. When in doubt, read this first.**
