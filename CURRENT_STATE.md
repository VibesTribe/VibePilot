# VibePilot Current State

**Required reading: FIVE files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`docs/SYSTEM_REFERENCE.md`** ← **WHAT WE HAVE AND HOW IT WORKS** (start here!)
3. **`docs/GOVERNOR_REBUILD_PLAN.md`** ← **REBUILD PLAN** (read this before touching Go code!)
4. **`docs/core_philosophy.md`** - Strategic mindset and inviolable principles
5. **`docs/prd_v1.4.md`** - Complete system specification

**Read all five → Know everything → Do anything**

---

**Last Updated:** 2026-02-25
**Updated By:** GLM-5 - Session 30
**Session Focus:** GOVERNOR REBUILD Phase 1 COMPLETE
**Direction:** Agent logic moves to prompts, Go becomes tools + runtime only

**Schema Location:** `docs/supabase-schema/` (all SQL files)
**Progress:** Phase 1 COMPLETE - runtime package built, config files created

---

# CRITICAL: GOVERNOR REBUILD IN PROGRESS

## The Problem

Current Go codebase: **8,287 lines**
- 70% is "agent logic" that should be in LLM prompts
- Hardcoded decisions prevent VibePilot from functioning as designed
- Council, Planner, Supervisor, Orchestrator - all need LLMs to think, but Go tries to think for them

## The Solution

**Read `docs/GOVERNOR_REBUILD_PLAN.md` for full details.**

Summary:
- DELETE ~5,800 lines of "agent logic" modules
- KEEP ~2,500 lines of tools (gitree, vault, maintenance, db, runtime)
- All intelligence moves to `config/prompts/*.md`
- All config stays in JSON files (agents.json, tools.json, models.json, etc.)

## What's Done (Phase 1)

### New runtime/ package (~1,150 lines)
- `config.go` - Load JSON config files (system, agents, tools, destinations)
- `events.go` - EventWatcher interface + PollingWatcher implementation
- `parallel.go` - AgentPool with per-module limits, SessionManager
- `session.go` - LLM session with tool calling loop
- `tools.go` - Tool registry with security validation, tool call parsing

### New db/rpc.go (~130 lines)
- Generic `CallRPC(name, params)` with allowlist security
- `CallRPCInto` for typed results
- Default allowlist with common RPCs

### New config files
- `config/system.json` - Database, vault, git, runtime, courier settings
- `config/tools.json` - Tool definitions with parameters, security levels, allowed agents

### Line count progress
```
runtime/       ~1,150 lines (NEW)
gitree/          252 lines (KEPT)
vault/           253 lines (KEPT)
maintenance/     746 lines (KEPT)
security/        130 lines (KEPT)
db/rpc.go        130 lines (NEW)
-------------------------------
Total:        ~2,660 lines (close to 2,500 target!)
```

## What's Next (Phase 2-5)

1. **Phase 2:** Wire up tool implementations (git, db, vault, maintenance)
2. **Phase 3:** Create destination runners (CLI, API, Courier)
3. **Phase 4:** Delete old agent modules (orchestrator, dispatcher, council, planner, etc.)
4. **Phase 5:** Update main.go to use new runtime, verify dashboard works

## Key Decisions Made

1. **Tool security:** 2-3 tools per agent, validated at runtime
2. **RPC:** Generic `CallRPC(name, params)` with allowlist
3. **Events:** Abstraction layer (PollingWatcher today, can add Realtime later)
4. **Parallel:** Goroutines with config limits (8 per module, 160 total)
5. **Everything swappable:** Models, platforms, database, git host, language

## What NOT To Do

- DO NOT add more "agent logic" to Go files
- DO NOT build prompts in Go code
- DO NOT make decisions in Go that LLMs should make
- DO NOT hardcode anything

## What TO Do

- Read the rebuild plan (`docs/GOVERNOR_REBUILD_PLAN.md`)
- Implement remaining phases
- Keep prompts in markdown files
- Keep config in JSON files
- Go = tools + runtime only

---

# PREVIOUS SESSIONS (Historical)

# SESSION 29: CONFIG-DRIVEN DESTINATIONS (2026-02-25)

## What Was Actually Done

### 1. Config-Driven Destinations (Zero Hardcoded Tools)

**Removed hardcoded switch statement** in dispatcher.go that would break if opencode disappeared.

**New schema:** `docs/supabase-schema/026_destinations.sql`
- `destinations` table with id, type, status, command/endpoint
- Updated `get_best_runner` to JOIN destinations WHERE status='active'

**Sync script updated:** `import_destinations()` added

**Go changes:**
- `Destination` struct + `GetDestination()` in db/supabase.go
- `executeCLI()`, `executeAPI()` in dispatcher.go
- Same fixes in agent/executor.go and analyst/analyst.go

**Result:** Change destinations.json → sync → system uses what's available. Zero code changes.

### 2. Planner Learning Schema (Phase 2)

**Schema:** `docs/supabase-schema/025_planner_learning.sql`

**Table:** `planner_learned_rules` - Rules learned from rejections

**RPCs:** create_planner_rule, get_planner_rules, record_planner_rule_applied, create_rule_from_rejection

## What Was Created Without Discussion (Needs Review)

### Vibes Agent (vibes/vibes.go)

**Created:** Commit b82d18dd - WITHOUT discussion with human
**Status:** NOT validated, NOT discussed, may need to be reworked or removed
**What Vibes should be:** Unknown - was not discussed this session

## Pre-Existing Agents (Not Created This Session)

| Agent | File | Notes |
|-------|------|-------|
| council | council/council.go (565 lines) | Pre-existing from Feb 23 |
| consultant | consultant/consultant.go | Created earlier |
| planner | planner/planner.go | Created earlier |

## What Was Rolled Back

### Maintenance Agent - WRONG IMPLEMENTATION

**What I wrongly built:**
- Polling loop in `maintenance.go`
- `Run()`, `pollAndExecute()`, `executeCommand()` methods
- Bypassed agent architecture entirely

**Why it was wrong:**
- Maintenance should be an AGENT like all others
- Receives tasks via Orchestrator → Dispatcher → pool.SelectBest()
- NOT a separate polling process

**What was rolled back (commit `90b22984`):**
- Removed polling loop from maintenance.go
- Removed merge special case from dispatcher.go
- Removed executeMerge() and handleMergeFailure()

**What remains (correct):**
- maintenance.go = git utility functions only

## What's NOT Done

### Maintenance Agent - Needs Correct Implementation

**Current gaps:**
- Branch creation happens AFTER task completes (should be at ASSIGNMENT)
- No `target_branch` in merge tasks (module vs main)
- No task→module merge logic
- No module completion detection
- No module→main merge logic
- maintenance.md still describes wrong architecture

**Merge flow needed:**
```
Task approved+tested → merge task/T001 → module/{slice}
All tasks in slice complete → merge module/{slice} → main
```

## Commits This Session

```
6aff001b docs: update for Session 29
90b22984 fix: revert maintenance polling, route merge through agent flow
f6cc19b9 feat: add maintenance polling loop (ROLLED BACK - wrong)
8f3c2529 fix: remove remaining hardcoded tool references
85c63da9 feat: config-driven destinations (zero hardcoded tools)
690bbf9c feat: add planner learning (Phase 2) schema and RPCs
b82d18dd feat: add Vibes agent (CREATED WITHOUT DISCUSSION - needs review)
```

## Code Stats

```
Total Go files: 28
Total lines:   ~6,000
Build:         ✅ Clean
Vet:           ✅ No issues
```

---

# SESSION 28: LEARNING SYSTEM (2026-02-24)

## What We Did

### 1. Security Audit + Fixes (8 commits)

**7 security hardening fixes:**
- Vault key caching at init (not per-decrypt)
- WebSocket origin check fix (prevent `evil-vercel.app` bypass)
- Config validation (fail fast on missing Supabase creds)
- Error body truncation (prevent secret leaks in logs)
- Courier result timeout (30s context)
- Orchestrator goroutine lifecycle (clean shutdown)
- Git add error checking (no silent failures)

**1 race condition fix:**
- Hub RLock → Lock when modifying clients map

### 2. Learning System Phase 1 COMPLETE

**Schema:** `docs/supabase-schema/024_learning_system.sql` (applied)

**New tables:**
| Table | Purpose |
|-------|---------|
| `learned_heuristics` | Model preferences per task type |
| `failure_records` | Structured failure logging |
| `problem_solutions` | What fixed what |

**New RPCs (7):**
- `record_failure` - Log structured failure
- `get_heuristic` - Get routing preference
- `get_problem_solution` - Find proven fix
- `record_heuristic_result` - Track heuristic success
- `record_solution_result` - Track solution success
- `get_recent_failures` - For routing exclusions
- `upsert_heuristic` - LLM updates heuristics

**Go implementation:**

| File | Changes |
|------|---------|
| `db/supabase.go` | 7 new methods for learning (RecordFailure, GetHeuristic, etc.) |
| `pool/model_pool.go` | Heuristic-aware routing, fallback logic, failure exclusion |
| `orchestrator/orchestrator.go` | Structured failure recording, failure classification |

### 3. Routing Flow (Enhanced)

```
Task needs routing:
1. Check learned_heuristics (preferred model for this task type?)
2. If found and model available → use preferred
3. Get recent failures (any models failing this type?)
4. Exclude failed models from selection
5. Get best runner from remaining
6. On failure: record to failure_records with type/category
```

### 4. Failure Classification

| Failure Type | Category | Detected By |
|--------------|----------|-------------|
| `timeout` | model_issue | "timeout", "timed out" |
| `rate_limited` | platform_issue | "rate limit", "429" |
| `context_exceeded` | model_issue | "context", "token limit" |
| `platform_down` | platform_issue | "platform down" |
| `test_failed` | quality_issue | "test fail" |
| `empty_output` | model_issue | "empty", "no output" |
| `quality_rejected` | quality_issue | "deliverable", "missing" |
| `latency_high` | platform_issue | "latency", "slow" |

## Commits This Session

```
35a7e8a3 feat: Learning system Phase 1 - Go implementation
8589deba fix: add defaults to all function parameters in learning schema
5cd99d73 docs: Learning system implementation plan + Phase 1 schema
5f537459 fix: use Lock instead of RLock when modifying hub clients map
0b30e3bc Security hardening and defensive fixes
```

## Code Stats

```
Total Go files: 24
Total lines:   5,303 (+354 from Session 27)
Build:         ✅ Clean
Vet:           ✅ No issues
```

## Reassignment Tracking

Task reassignments ARE tracked in `tasks.routing_history` JSONB:
```json
[
  {"from": "glm-4", "to": "kimi", "reason": "timeout", "at": "2026-02-24T..."},
  {"from": "kimi", "to": "deepseek", "reason": "rate limited", "at": "..."}
]
```

Dashboard can display this in task details.

---

# SESSION 27: STATELESS ORCHESTRATOR (2026-02-24)

## What We Did

### 1. Schema Migration (023_orchestrator_state.sql)

**New columns:**
- `runners.max_concurrent` - Maximum concurrent tasks per runner
- `runners.current_in_flight` - Current tasks in progress
- `tasks.routing_history` - JSONB array of routing decisions

**New tables:**
- `orchestrator_events` - Audit trail of all orchestrator decisions
- `security_audit` - Sensitive operation tracking

**New RPCs:**
- `log_orchestrator_event` - Record decision to event log
- `append_routing_history` - Add routing step to task
- `increment_in_flight` - Atomic concurrent capacity check
- `decrement_in_flight` - Release concurrent slot
- `get_system_state` - Full snapshot for orchestrator
- `log_security_audit` - Track vault access

**Security:**
- Vault RLS hardened (no bulk export/delete for authenticated)

### 2. Vault Module (internal/vault/vault.go)

Go Governor can now access encrypted secrets:
- Fernet decryption (matches Python vault_manager.py)
- In-memory caching (5 min TTL)
- Audit logging to security_audit table

### 3. Orchestrator Event Logging

Every decision logged:
- task_dispatched, runner_selected, task_complete
- supervisor_approve, supervisor_reject, awaiting_human
- visual_test_passed, visual_test_failed
- escalated, analysis_complete, rerouted

### 4. Concurrent Tracking

Dispatcher uses atomic RPCs:
- `increment_in_flight(runner_id)` before task
- `decrement_in_flight(runner_id)` after completion

### 5. Systemd Service

Created `scripts/governor.service` for production deployment.

## Files Changed

| File | Change |
|------|--------|
| `docs/supabase-schema/023_orchestrator_state.sql` | NEW - Schema migration |
| `internal/vault/vault.go` | NEW - Vault access module |
| `internal/orchestrator/orchestrator.go` | Event logging added |
| `internal/dispatcher/dispatcher.go` | In-flight tracking + event logging |
| `internal/db/supabase.go` | New RPCs for events/concurrent |
| `scripts/governor.service` | NEW - Systemd service |

## Final Code Stats

```
Total Go files: 24
Total lines:   4,901
Build:         ✅ Clean
Vet:           ✅ No issues
```

## Branch Status

| Repo | Branch | Status |
|------|--------|--------|
| vibepilot | `go-governor` | Phase 6 complete, ready to push |
| vibeflow | `main` | Production with merge pending UI |
| vibeflow | `vibeflow-test` | Staging (can merge to main) |

## Go Governor Status

See `docs/GOVERNOR_HANDOFF.md` for full details.

**Done:**
- Supervisor 3 actions (APPROVE, REJECT, HUMAN) for task outputs
- Supervisor 3 actions (APPROVE, REJECT, COUNCIL) for plans/research
- Council reviews PLANS and RESEARCH SUGGESTIONS
- Visual testing agent (stub) before human review
- System Researcher for escalated task analysis
- All hardcoded values configurable
- Event logging to orchestrator_events
- Concurrent capacity tracking
- Vault access from Go
- Security audit trail
- Systemd service

**Stub Remaining:**
- `visual/visual.go` - `TestVisual()` passes by default (needs real implementation)
- `maintenance.go` - No command queue polling yet

## Config Options

```yaml
governor:
  poll_interval: 15s
  max_concurrent: 3
  stuck_timeout: 10m
  max_per_module: 8
  task_timeout_sec: 300
  council_max_rounds: 4
```

## Dashboard Status

**Live at Vercel** - auto-deploys from `main` branch.

**Event log now available:** `orchestrator_events` table feeds dashboard logs modal.

**Key status mappings:**
| DB Status | Dashboard Status | Flags? |
|-----------|------------------|--------|
| `awaiting_human` | `supervisor_review` | YES - needs review |
| `approval` | `supervisor_approval` | YES - merge pending badge |
| `testing` | `testing` | NO |
| `merged` | `complete` | NO |
| `failed`/`escalated` | `blocked` | NO |

## Active Models

| Model ID | Status | Notes |
|----------|--------|-------|
| glm-5 (opencode) | ✅ ACTIVE | Only working runner |
| kimi-cli | BENCHED | Subscription cancelled |
| gemini-api | PAUSED | Quota exhausted |
| deepseek-chat | PAUSED | Credit needed |

---

# NEXT SESSION

**Phase 1 COMPLETE.** Remaining phases from `docs/LEARNING_SYSTEM_PLAN.md`:

1. **Phase 2:** Planner learning (from council/supervisor feedback)
2. **Phase 3:** Tester/Supervisor learning
3. **Phase 4:** Daily LLM analysis (self-optimization)
4. **Phase 5:** Deprecation/Revival system

**Other pending:**
- Visual testing implementation
- Maintenance command polling
- Dashboard: Display `routing_history` in task details

---

# QUICK COMMANDS

| Command | Action |
|---------|--------|
| `cat CURRENT_STATE.md` | This file |
| `cat docs/GOVERNOR_HANDOFF.md` | Go Governor status |
| `cat AGENTS.md` | Mental model + workflow |
| `cd ~/vibepilot/governor && go build ./...` | Build Go Governor |
| `cd ~/vibeflow && npm run typecheck` | Check vibeflow types |

---

# FILES MODIFIED THIS SESSION

| File | Change |
|------|--------|
| `docs/supabase-schema/023_orchestrator_state.sql` | NEW - Schema migration |
| `governor/internal/vault/vault.go` | NEW - Vault access module |
| `governor/internal/orchestrator/orchestrator.go` | Event logging |
| `governor/internal/dispatcher/dispatcher.go` | In-flight tracking + event logging |
| `governor/internal/db/supabase.go` | New RPCs |
| `scripts/governor.service` | NEW - Systemd service |
| `docs/GOVERNOR_HANDOFF.md` | Updated for Session 27 |
