# VibePilot System Reference

**Purpose:** Single source of truth for "what we have and how it works." Read this instead of exploring 35+ files.

**Last Updated:** 2026-02-22
**Updated By:** GLM-5 - Created for Go Iron Stack transition

---

## 1. INFRASTRUCTURE

| Component | Current | Target | Notes |
|-----------|---------|--------|-------|
| GCE Instance | e2-standard-2 ($64/mo) | e2-micro ($0/free tier) | **COST PROBLEM** |
| RAM | 8GB (wasted) | 1GB (constraint) | Current runner uses 1.4GB alone |
| Supabase | Free tier | Same | 500MB, 50K requests/day |
| GitHub | Free tier | Same | Actions, storage |
| Vercel | Free tier | Same | Dashboard hosting |

**The Problem:** $34 in 6 days is not sustainable. Must fit on e2-micro (1GB RAM).

---

## 2. SUPABASE SCHEMA (Quick Reference)

### Core Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `tasks` | Work items | id, title, status, dependencies (JSONB), routing_flag, slice_id, assigned_to |
| `task_runs` | Execution history | task_id, model_id, platform, status, tokens_in/out, costs |
| `models` | AI model registry | id, platform, context_limit, status, tokens_used, cooldown_expires_at |
| `platforms` | Web AI platforms | id, url, daily_limit, daily_used, status |
| `projects` | Project tracking | id, name, total_tasks, ROI fields |
| `task_packets` | Versioned work orders | task_id, prompt, tech_spec, context |
| `maintenance_commands` | Git command queue | command_type, payload, status, approved_by |
| `council_reviews` | Multi-model reviews | plan_id, round, model_id, lens, vote |
| `agent_messages` | Inter-agent comms | from_agent, to_agent, content |

### Key RPC Functions

| Function | Purpose |
|----------|---------|
| `claim_next_task(courier, platform, model_id)` | Atomically claim available task |
| `get_available_for_routing(can_web, can_internal, can_mcp)` | Get tasks filtered by capability |
| `unlock_dependent_tasks(completed_task_id)` | Unlock tasks waiting on completed task |
| `check_dependencies_complete(task_id)` | Check if all deps are merged |
| `update_model_stats(model_id, success, tokens)` | Update model performance |
| `get_full_roi_report()` | Comprehensive ROI |

### Task States

```
pending → locked → available → in_progress → review → testing → approval → merged
         (has deps) (unlocked)  (dispatched)  (done)   (tests)  (waiting)  (complete)
                              ↘ escalated (3 failures)
```

### Full Schema Location
`docs/supabase-schema/` - 35 SQL migration files

---

## 3. CODEBASE STRUCTURE

```
vibepilot/
├── core/
│   ├── orchestrator.py      # Main orchestration loop (TO BE REPLACED)
│   ├── memory.py            # Pluggable context storage
│   └── telemetry.py         # OpenTelemetry + fallback
├── runners/
│   ├── base_runner.py       # Abstract contract (RUNNER_INTERFACE.md)
│   └── contract_runners.py  # Kimi, DeepSeek, Gemini, Courier implementations
├── agents/
│   ├── supervisor.py        # Review queue, quality gate
│   └── maintenance.py       # Git operations (ONLY agent with write)
├── task_manager.py          # Supabase operations
├── config/
│   ├── models.json          # WHO provides intelligence
│   ├── platforms.json       # WHERE to run (web platforms)
│   ├── destinations.json    # WHERE to run (cli/api/web)
│   ├── roles.json           # WHAT job to do
│   ├── routing.json         # WHY this path chosen
│   ├── skills.json          # Capabilities
│   ├── tools.json           # Primitives
│   └── agents.json          # Actor definitions
├── config/prompts/          # Agent behavior definitions (PRESERVE)
│   ├── vibes.md, orchestrator.md, researcher.md
│   ├── consultant.md, planner.md, council.md
│   ├── supervisor.md, courier.md, maintenance.md
│   └── internal_cli.md, internal_api.md, tester_code.md
├── docs/
│   ├── prd_v1.4.md          # Full system specification (PRESERVE)
│   ├── core_philosophy.md   # Strategic mindset (PRESERVE)
│   ├── UPDATE_CONSIDERATIONS.md  # Research findings (PRESERVE)
│   └── supabase-schema/     # SQL migrations
└── vault_manager.py         # Encrypted secrets in Supabase
```

---

## 4. AGENT ROLES (12 Agents)

### Decision Agents (can approve/reject)
| Agent | Role | Model | Can Decide |
|-------|------|-------|------------|
| orchestrator | Routing | gemini-2.0-flash | Yes |
| council | Review | gemini-2.0-flash | Yes |
| supervisor | Quality gate | gemini-2.0-flash | Yes |

### Execution Agents (produce output)
| Agent | Role | Model | Can Execute | Git Write |
|-------|------|-------|-------------|-----------|
| courier | Web execution | browser-use-gemini | Yes | No |
| internal_cli | CLI execution | kimi-cli/opencode | Yes | No |
| internal_api | API execution | gemini-api | Yes | No |
| tester_code | Code testing | opencode | Yes | No |
| **maintenance** | Git operator | opencode | Yes | **Yes (ONLY)** |

### Support Agents (no decision, no execution)
| Agent | Role | Model |
|-------|------|-------|
| vibes | Human interface | gemini-2.0-flash |
| researcher | Intelligence | gemini-2.0-flash |
| consultant | PRD generation | gemini-2.0-flash |
| planner | Task planning | kimi-cli |

### Critical Rules
- **ONLY `maintenance` agent has git write access**
- Other agents command maintenance via `maintenance_commands` table
- Courier has NO codebase access, NO git access

---

## 5. CONFIGURATION SYSTEM

### Three-Layer Separation

```
ROLES (WHAT job) → MODELS (WHO provides intelligence) → DESTINATIONS (WHERE it runs)
```

### Key Config Files

| File | Purpose | Swappable? |
|------|---------|------------|
| models.json | AI model definitions | Yes - add/remove/modify |
| platforms.json | Web platform definitions | Yes |
| destinations.json | All execution destinations | Yes |
| roles.json | Job definitions | Yes |
| routing.json | Routing strategy selection | Yes |
| skills.json | Capability definitions | Yes |
| agents.json | Actor definitions | Yes |

### Routing Strategies

| Strategy | Priority Order |
|----------|---------------|
| web_first | Web → CLI → API Free → API Credit |
| subscription_first | CLI → Web → API |
| kimi_priority | CLI first (current until Feb 27) |
| cost_optimize | Free → Subscription → Credit |

### Credit Protection

- Warning at $5.00 remaining
- Pause at $1.00 remaining
- Skip credit APIs if alternatives available

---

## 6. RUNNER CONTRACT

### Input: task_packet
```json
{
  "task_id": "uuid",
  "prompt": "the task to execute",
  "title": "Human readable title",
  "constraints": { "max_tokens": 4000, "timeout_seconds": 300 },
  "runner_context": { "platform": "...", "model": "...", "work_dir": "..." }
}
```

### Output: result
```json
{
  "success": true,
  "output": "the actual output",
  "error": null,
  "error_code": null,
  "tokens": 1500,
  "prompt_tokens": 500,
  "completion_tokens": 1000,
  "model_id": "gemini-2.0-flash",
  "chat_url": "https://..."  // For web platforms only
}
```

### Exit Codes
- 0 = Success
- 1 = Failure

---

## 7. CURRENT RUNNERS

| Runner | Type | RAM | Status |
|--------|------|-----|--------|
| glm-5 (opencode) | CLI | 1.4GB | ACTIVE |
| kimi-cli | CLI | - | BENCHED (subscription cancelled) |
| gemini-api | API | - | PAUSED (quota) |
| deepseek-chat | API | - | PAUSED (needs credit) |

**RAM Problem:** OpenCode alone uses 1.4GB. e2-micro has 1GB total.

---

## 8. DASHBOARD (vibeflow repo)

**Location:** `~/vibeflow/apps/dashboard/`
**Live at:** https://vibeflow-dashboard.vercel.app/
**DO NOT TOUCH** - User is very attached to current design.

### Dashboard Features
- Real-time task status
- Agent activity monitoring
- ROI tracking (USD/CAD toggle)
- Model/platform health
- Collapsible sections by slice/model

---

## 9. KEY CONSTRAINTS

| Constraint | Value | Impact |
|------------|-------|--------|
| e2-micro RAM | 1GB | Cannot run current architecture |
| OpenCode RAM | 1.4GB | Exceeds e2-micro alone |
| Supabase free tier | 500MB, 50K req/day | Ample headroom |
| GitHub Actions free | 2000 min/month, 7GB/runner | For offloaded browsers |
| Web platform limits | ChatGPT 40/day, Claude 10/day | 80% throttle rule |

---

## 10. PRINCIPLES (from core_philosophy.md)

1. **Zero Vendor Lock-In** - Everything swappable in one day
2. **Modular & Swappable** - Change one thing, nothing else breaks
3. **Exit Ready** - Pack up, hand over to anyone
4. **Reversible** - If it can't be undone, it can't be done
5. **Always Improving** - Daily research, weekly evaluation

---

## 11. TRANSITION TO GO IRON STACK

### What Changes
- Python orchestrator → Go "Governor" binary
- Local browsers → GitHub Actions (7GB runners)
- Deployment → Single binary with embedded UI

### What Stays
- Supabase schema (no changes)
- Config files (models.json, platforms.json, etc.)
- Agent prompts (config/prompts/*.md)
- Architecture docs (prd, philosophy, considerations)
- Dashboard (vibeflow repo - untouched)

### Architecture Target
```
GOVERNOR (Go binary, 10-20MB)
├── Polls Supabase every 15s
├── Max 3 concurrent tasks
├── Couriers → GitHub Actions (free 7GB)
├── Internal → Local CLI tools
└── Embedded React UI
```

---

## 12. QUICK COMMANDS

| Command | Action |
|---------|--------|
| `cat CURRENT_STATE.md` | Current project status |
| `cat AGENTS.md` | Agent workflow guide |
| `cat docs/SYSTEM_REFERENCE.md` | This file |
| `sudo journalctl -u vibepilot-orchestrator -f` | Orchestrator logs |
| `cd ~/vibepilot && source venv/bin/activate` | Activate venv |

---

## UPDATE LOG

| Date | Change |
|------|--------|
| 2026-02-22 | Created for Go Iron Stack transition planning |
