# VibePilot Current State

**Required reading: SEVEN files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`AUDIT_REPORT.md`** - FULL CODE AUDIT - What works, what doesn't, what's missing
3. **`docs/vibepilot_process.md`** - COMPLETE process flow, all roles, failure handling, learning
4. **`docs/learning_system.md`** - Learning system design, review flow, thresholds
5. **`docs/SYSTEM_REFERENCE.md`** - What we have and how it works
6. **`docs/SESSION_35_HANDOFF.md`** - Dynamic routing implementation details
7. **`docs/core_philosophy.md`** - Strategic mindset and principles

**Read all six → Know everything → Do anything**

---

**Last Updated:** 2026-03-02
**Updated By:** GLM-5 - Session 42 (Phase 3 In Progress)
**Branch:** `main`
**Status:** BUILDING - Implementing state-based recovery architecture

---

## Architecture Improvements (Session 42)

### Analysis Complete
- ✅ Full architecture analysis: `ARCHITECTURE_ANALYSIS.md`
- ✅ Identified broken processing claims (timeout-based)
- ✅ Identified fragile revision flow
- ✅ Designed state-based recovery system

### Phase 1: Config (DONE)
- ✅ Reduce processing timeout to 5 min (from 10)
- ✅ Reduce recovery interval to 10s (from 30s)
- ✅ Add state-based recovery config
- ✅ Add revision tracking config (max_rounds, on_max_rounds)
- ✅ Add performance targets
- ✅ Add logging config for state transitions

### Phase 2: Database (DONE - Migrations Ready)
- ✅ Migration 050: State tracking and revision history
  - plans: revision_round, revision_history, latest_feedback
  - tasks: retry_count, last_error, last_error_at
  - state_transitions table
  - performance_metrics table
  - Helper functions for recording
  
- ✅ Migration 051: Fix task_packets relationship

### Phase 3: Code Refactor (✅ DONE)
- ✅ State tracking helpers (`governor/internal/db/state.go`)
  - RecordStateTransition
  - RecordPerformanceMetric  
  - GetLatestState
  - ClearProcessingAndRecordTransition
- ✅ RPC allowlist updated
- ✅ Event handlers updated with state tracking
  - EventPlanCreated
  - EventPRDReady
- ✅ Full reset SQL created (`scripts/sql/full_reset.sql`)
- ✅ Test PRD created (`docs/prd/test-autonomous-flow.md`)

### Phase 4: Testing (NEXT)
1. ⬜ Run `scripts/sql/full_reset.sql` in Supabase SQL Editor
2. ⬜ Start governor with new architecture
3. ⬜ Create fresh test PRD
4. ⬜ Verify autonomous flow works end-to-end
5. ⬜ Check state transitions are being recorded
6. ⬜ Check performance metrics are being tracked

### What This Fixes:
- ❌ 10-minute timeout → ✅ 5-minute timeout + state-based recovery
- ❌ Lost revision state → ✅ Tracked in database
- ❌ Permanent error states → ✅ Resettable to draft
- ❌ No performance visibility → ✅ Full metrics tracking
- ❌ Fragile processing claims → ✅ Immediate clear on success/failure

### Next Actions:
1. **Run full_reset.sql** in Supabase (clears all error states and processing claims)
2. **Start governor** and watch logs
3. **Test with new PRD** and verify full flow works

---

## Session 42 Earlier (Prompt Packet Fixes)

**Issues Found:**
1. **Planner prompt too complex** (693 lines) - confusing for model
2. **Prompt packets empty** - Dashboard expects `task.result.prompt_packet` but planner was outputting complex nested JSON
3. **Parsing fragile** - Expected code blocks around prompt_packet, but format was inconsistent

**Fixes Applied:**
1. ✅ **Simplified planner prompt** (693 → 112 lines)
   - Follows Vibeflow's clean task structure
   - `task_id`, `title`, `context`, `files`, `acceptance_criteria`
   - Self-contained `prompt_packet` (copy-paste ready)
   - Clear `expected_output` format
   - No complex nested examples
   - Explicit JSON-only output requirement

2. ✅ **Updated task parsing** in governor
   - Supports both code block and plain text formats
   - Falls back to raw content if no code blocks
   - Handles prompt_packet and expected_output extraction

**Still TODO:**
- [ ] Apply migration 049 to Supabase (if not already applied)
- [ ] Test with actual PRD to verify prompt packets are created correctly
- [ ] Verify dashboard can display prompt packets from `task.result.prompt_packet`

---

### DONE - Session 42 (Prompt Packet Quality)

...

---

# CURRENT ARCHITECTURE

## What's Running

```
vibepilot-governor.service (Go binary)
├── Polls Supabase every 1s
├── Max 8 concurrent per module, 160 total
├── Dynamic routing via config (NO hardcoded destinations)
├── Branch creation when Orchestrator assigns task
├── Reads secrets from vault at runtime
├── Startup recovery: finds and recovers orphaned sessions
├── Usage tracking: multi-window rate limit enforcement
├── Learning: model scoring RPC (ready to deploy)
├── Revision loop: max rounds configurable (default: 6)
├── Council execution: 3 members, parallel or sequential
├── Plan lifecycle: all states configurable via JSON
└── Processing state: prevents duplicate event firing (migration 042)
```

**Status:** `systemctl status vibepilot-governor`

## Architecture Principle

```
Model = Intelligence (thinks, outputs)
Transport/CLI = Provides tools natively (read/write/bash)
Destination = Where/how access happens (has capabilities)
Agent = Role with capabilities needed (for routing)
Prompt packet = Task + expected output format
Hat = Prompt/role a model wears for a specific task

Routing = config/routing.json (strategies, restrictions, categories)
Destinations = config/destinations.json (status, type, provides_tools)
Models = config/models.json (availability, access_via)
Plan Lifecycle = config/plan_lifecycle.json (states, transitions, rules)

NO HARDCODED DESTINATIONS. Everything configurable.
ALL CHANGES GO THROUGH TASK SYSTEM. Nothing implemented directly.
```

## Plan Lifecycle (NEW)

```
config/plan_lifecycle.json controls:
├── states: draft → review → [approved | revision_needed | council_review]
├── revision_rules: max_rounds (default: 6), on_max_rounds action
├── complexity_rules: simple vs complex detection thresholds
├── consensus_rules: unanimous_approval | majority | weighted
└── council_rules: member_count, lenses, parallel vs sequential strategy
```

## Event System (UPDATED)

| Event | Fires When | Handler Action |
|-------|------------|----------------|
| `EventPRDReady` | status = `draft` | Planner creates plan |
| `EventPlanReview` | status = `review` | Supervisor reviews |
| `EventRevisionNeeded` | status = `revision_needed` | Planner revises with feedback |
| `EventCouncilReview` | status = `council_review` | 3 council members review |
| `EventCouncilComplete` | council done | Consensus calculated |
| `EventPlanApproved` | status = `approved` (direct) | Tasks created |
| `EventPlanBlocked` | status = `blocked` | Awaits human |
| `EventPlanError` | status = `error` | Logged for recovery |

## Dynamic Routing

```
Event fires
    ↓
selectDestination(agentID, taskID, taskType)
    ↓
Get strategy for agent (internal_only for governance agents)
    ↓
Get priority order from routing.json
    ↓
For each category: find active destination
    ↓
Get model score from RPC (success_rate from task_runs)
    ↓
Return destination ID or "" if none available
```

**Internal agents (planner, supervisor, council, etc.)** → internal_only strategy → never external
**Task execution** → default strategy → external first, then internal

## "Hats" Concept

Models don't have fixed roles. Orchestrator assigns any available model to wear the appropriate "hat" (use the right prompt) for each task.

Example:
- Task needs maintenance work → Orchestrator picks available model → Model wears "maintenance hat"
- Task needs planning → Orchestrator picks available model → Model wears "planner hat"

## Codebase Structure (Clean)

```
vibepilot/
├── governor/              # ACTIVE - Go binary (everything)
│   ├── cmd/governor/      # Main entry point + event handlers + routing
│   ├── internal/
│   │   ├── db/            # Supabase client + RPC allowlist
│   │   ├── vault/         # Secret decryption
│   │   ├── runtime/       # Events, sessions, router, usage_tracker, config
│   │   ├── gitree/        # Git operations (branch, commit, merge, delete)
│   │   ├── destinations/  # CLI/API runners
│   │   └── tools/         # Tool registry
│   └── config/            # JSON configs (routing.json, destinations.json, etc.)
├── config/                # Root config files
│   ├── plan_lifecycle.json  # NEW - Plan states, revision rules, council rules
│   ├── routing.json        # Routing strategies
│   ├── destinations.json   # Execution destinations
│   └── ...
├── prompts/               # Agent behavior definitions (.md)
├── docs/                  # Documentation
│   └── supabase-schema/   # SQL migrations (034, 035, 036)
├── scripts/               # Deploy scripts
│   └── opencode-count.sh  # Check opencode session count
└── legacy/                # DEAD CODE - kept for reference
```

---

## Bootstrap Keys (Secure)

| Key | Where It Lives | Who Can Read |
|-----|----------------|--------------|
| `SUPABASE_URL` | `/etc/systemd/.../override.conf` | root only |
| `SUPABASE_SERVICE_KEY` | `/etc/systemd/.../override.conf` | root only |
| `VAULT_KEY` | `/etc/systemd/.../override.conf` | root only |

**All other secrets** → Encrypted in Supabase `secrets_vault` table

**Read `docs/SECURITY_BOOTSTRAP.md` before touching credentials.**

---

## Quick Commands

| Command | Action |
|---------|--------|
| `systemctl status vibepilot-governor` | Check if running |
| `journalctl -u vibepilot-governor -f` | Live logs |
| `sudo systemctl restart vibepilot-governor` | Restart |
| `cd ~/vibepilot/governor && go build -o governor ./cmd/governor` | Build |
| `sudo scripts/deploy-governor.sh` | Full deploy |
| `~/vibepilot/scripts/opencode-count.sh` | Check opencode session count |

---

## Session Progress

### DONE - Session 35
- ✅ Dynamic routing (router.go, routing.json)
- ✅ Python moved to legacy/
- ✅ TOOL: format removed

### DONE - Session 36
- ✅ Full documentation update (vibepilot_process.md)
- ✅ Failure handling flow documented
- ✅ Learning system documented (docs/learning_system.md)
- ✅ Branch creation on orchestrator assignment (main.go)
- ✅ Research branch created for review docs
- ✅ "What I've Learned" sections added to agent prompts
- ✅ Supervisor decision matrix (simple/complex/human)
- ✅ System Researcher flow documented
- ✅ Branch lifecycle documented
- ✅ Courier vs Internal clarified
- ✅ "Hats" concept documented
- ✅ All changes go through task system documented
- ✅ Model scoring RPC created (033_model_scoring_rpc.sql)
- ✅ Model scoring added to RPC allowlist
- ✅ FULL CODE AUDIT COMPLETE

### DONE - Session 37
- ✅ decision.go - Parse agent outputs + extractJSON for markdown blocks
- ✅ context_builder.go - Build context from existing RPCs
- ✅ prd_watcher.go - Detect new PRDs
- ✅ task_runner.md - New agent for executing tasks
- ✅ EventTaskAvailable - task_runner executes, commits to GitHub, sets status
- ✅ EventTaskReview - Parse decision, call record_failure, update status
- ✅ EventPRDReady - Parse planner output, commit to GitHub
- ✅ EventPlanReview - Parse initial review, set status
- ✅ EventCouncilDone - Parse votes, set consensus, create planner rules
- ✅ EventTestResults - Parse outcome, merge/reset/await based on result
- ✅ Context builder wired to planner and supervisor sessions
- ✅ PRD watcher wired to main.go

### DONE - Session 38 (Phase 1 & 2)

**Phase 1 - Critical Bug Fixes:**
- ✅ Migration 034: confidence/category columns, create_task_with_packet RPC
- ✅ Migration 035: update_plan_status sets plan_path
- ✅ Migration 036: revision_round, revision_history, council tracking
- ✅ config/plan_lifecycle.json: All plan rules configurable
- ✅ BUG FIX: Task creation failure → status = "error" (not "approved")
- ✅ BUG FIX: Council check before processing consensus
- ✅ Config loader: GetMaxRevisionRounds(), GetCouncilMemberCount(), etc.

**Phase 2 - Event Renaming & Revision Loop:**
- ✅ New events: EventRevisionNeeded, EventCouncilReview, EventCouncilComplete, EventPlanApproved, EventPlanBlocked, EventPlanError
- ✅ detectPlanEvents: Correct event firing based on status
- ✅ EventRevisionNeeded handler: Planner gets feedback, round limit enforced
- ✅ EventCouncilReview handler: 3 members, parallel/sequential, configurable
- ✅ EventPlanApproved handler: Direct approval creates tasks
- ✅ Consensus calculation: Uses config (unanimous_approval, majority, weighted)
- ✅ Council loads PRD for comparison (configurable)

### DONE - Session 39 (Bug Fixes)

**Critical Bug Fixes:**
- ✅ Fixed infinite task loop: EventTaskCompleted now properly handles supervisor decision
- ✅ Fixed branch checkout: Fetches from remote if branch not found locally
- ✅ Fixed JSON parsing: Handles both object arrays and string arrays for files_created
- ✅ Removed poe-web destination (web courier not implemented)
- ✅ Set stuck task T001 to 'escalated' status

**CRITICAL FIX - Prompt Packet Delivery:**
- ✅ Added `GetTaskPacket()` to DB package - fetches from `task_packets` table
- ✅ EventTaskAvailable now fetches prompt packet BEFORE execution
- ✅ Task runner receives full context: `prompt_packet`, `expected_output`, `context`
- ✅ Error handling for missing/empty packets (sets task to error)
- ✅ Category passed for routing consideration
- ✅ Agent hat now works - model receives instructions it can follow

**Task Validation + Feedback Loop:**
- ✅ Tasks validated at creation: confidence >= 0.95, non-empty prompt, category, expected output
- ✅ Validation failure → revision_needed (not error) with specific feedback
- ✅ Planner receives validation feedback via revision loop
- ✅ Supervisor rule recorded for learning (safety net catches missed issues)
- ✅ All validation thresholds configurable via system.json (not hardcoded)

**Council Integration for Complex Plans:**
- ✅ Supervisor can route complex plans to council_review
- ✅ Council members review in parallel or sequential (configurable)
- ✅ Consensus calculated (unanimous or majority, configurable)
- ✅ Council-approved plans now create tasks with validation
- ✅ Robust JSONB handling for council_reviews field

**System Research Flow (Self-Improvement):**
- ✅ research_suggestions table with type-based complexity routing
- ✅ Simple items (new_model, pricing_change): Supervisor approves → maintenance command
- ✅ Complex items (architecture, security): Council reviews → consensus
- ✅ Human items (api_credit_exhausted, ui_ux): Flagged for human immediately
- ✅ EventResearchReady: Routes based on complexity
- ✅ EventResearchCouncil: Full council review for research
- ✅ Maintenance commands created for approved research

**Security Audit Fixes:**
- ✅ No hardcoded paths - all paths from config
- ✅ Branch name validation - prevents command injection
- ✅ Table name validation - prevents SQL injection
- ✅ URL encoding in query builder - safe filter values
- ✅ Path traversal protection - symlinks and absolute paths blocked
- ✅ Error logging - no silently ignored errors

### DONE - Session 40 (Infinite Event Loop Fix)

**Root Cause:**
- Events fired every poll (1s) while agent worked on plan/task
- Status didn't change until work completed (minutes)
- Same event fired hundreds of times, spawning duplicate agents
- Capacity exhausted, all sessions killed

**Solution - Processing State:**
- ✅ Migration 042: `processing_by` and `processing_at` columns on plans and tasks
- ✅ Event detection filters `processing_by IS NULL` - only fire for idle items
- ✅ Handlers claim processing atomically before spawning agent
- ✅ Clear processing on completion, error, or pool submission failure
- ✅ Recovery goroutine: clears stale processing (configurable timeout)
- ✅ Fixed `record_planner_revision` RPC parameter format (TEXT[] not JSONB)
- ✅ Added `record_supervisor_rule` RPC to allowlist

**New Config Options:**
- `recovery.processing_timeout_seconds`: 300 (default)
- `recovery.processing_recovery_interval_seconds`: 60 (default)

### DONE - Session 40 (Full Code Audit)

**Three Parallel Audits:**
1. Governor code (main.go, events.go, config.go, rpc.go)
2. Prompts and configs
3. Schema and RPCs

**Critical Fixes:**
- ✅ Migration 043: `test_results` table created
- ✅ Migration 043: `record_supervisor_rule` uses correct table
- ✅ courier.md copied to `prompts/`

**Non-Critical Fixes:**
- ✅ RPC allowlist reorganized with categories
- ✅ Hardcoded "main" → `cfg.GetDefaultMergeTarget()`
- ✅ Hardcoded "origin" → configurable `git.remote_name`
- ✅ `plan_lifecycle.json` copied to governor/config/
- ✅ `config/prompts/` marked deprecated

### DONE - Session 42 (Prompt Packet Quality)

**Root Cause Analysis:**
- Agents outputting markdown code blocks with language specifiers (\```json go)
- Agents outputting conversational text before JSON
- Prompt packets could be empty or placeholder text
- Expected output missing task_number for supervisor reference

**Critical Fixes:**
- ✅ Planner: prompt_packet must be non-empty, complete, self-contained
- ✅ Planner: expected_output must include task_number for supervisor reference
- ✅ Planner: Added clear task structure showing prompt_packet vs metadata separation
- ✅ Supervisor: Stricter validation of prompt_packet (no empty, no placeholders)
- ✅ Supervisor: Added validation failure examples with specific guidance
- ✅ Task Runner: task_number added to output format
- ✅ All agents: Stronger warnings about NO markdown code blocks, NO conversational text
- ✅ All agents: Added WRONG/CORRECT examples for output format

**Prompt Architecture Clarified:**
```
Task record in DB:
├── task_id, task_number, title (metadata)
├── category (routing metadata)
├── dependencies (planning metadata)
├── prompt_packet (EXECUTOR RECEIVES THIS ONLY)
├── expected_output (SUPERVISOR CHECKS THIS)
└── confidence (planner quality metric)

Orchestrator:
├── Reads task record
├── Strips metadata
├── Passes ONLY prompt_packet to executor (internal or web)
└── Executor outputs JSON with task_number for reference
```

---

### NOW - Ready for Full Autonomous Test

System has:
- ✅ Duplicate event prevention (all 5 tables)
- ✅ Processing state with recovery
- ✅ Revision loop with round limits
- ✅ Council execution for complex plans
- ✅ Prompt packet quality requirements
- ✅ JSON-only output enforcement
- ✅ Supervisor validation of prompt packets

Test PRD ready: `docs/prd/governor-startup-message.md`

---

## Migrations Applied

| # | File | Status |
|---|------|--------|
| 034 | task_improvements.sql | ✅ Applied |
| 035 | fix_plan_path.sql | ✅ Applied |
| 036 | revision_loop.sql | ✅ Applied |
| 040 | update_task_status.sql | ✅ Applied |
| 041 | research_suggestions.sql | ✅ Applied |
| 042 | processing_state.sql | ✅ Applied |
| 043 | fix_schema_gaps.sql | ✅ Applied |
| 044 | processing_state_all_tables.sql | ✅ Applied |
| 045 | fix_processing_timestamp.sql | ✅ Applied |
| 046 | add_blocked_error_status.sql | ✅ Applied |
| 047 | fix_revision_history.sql | ✅ Applied |
| 048 | add_prd_incomplete_status.sql | ✅ Applied |

---

### DONE - Session 41 (Duplicate Event Prevention)

**Root Cause Analysis:**
- Processing claims existed for plans/tasks but NOT for test_results, research_suggestions, maintenance_commands
- 8 event handlers lacked processing claims, causing duplicate firing when handlers took >1s

**Critical Fixes:**
- ✅ Migration 044: Added processing_by columns to test_results, research_suggestions, maintenance_commands
- ✅ Updated set_processing, clear_processing, find_stale_processing, recover_stale_processing RPCs for all 5 tables
- ✅ EventTaskCompleted: Added processing claim
- ✅ EventCouncilReview: Added processing claim
- ✅ EventCouncilDone: Added processing claim
- ✅ EventTestResults: Added processing claim (test_results table)
- ✅ EventResearchReady: Added processing claim (research_suggestions table)
- ✅ EventResearchCouncil: Added processing claim (research_suggestions table)
- ✅ EventPlanCreated: Added processing claim
- ✅ EventMaintenanceCmd: Added processing claim (maintenance_commands table)

**Recovery Updates:**
- ✅ runProcessingRecovery now recovers all 5 tables (plans, tasks, test_results, research_suggestions, maintenance_commands)

**Event Detection Updates:**
- ✅ detectTestResults: Added processing_by IS NULL filter
- ✅ detectResearchSuggestions: Added processing_by IS NULL filter for both pending and council_review
- ✅ detectMaintenanceEvents: Added processing_by IS NULL filter

---

## Audit Findings (Session 40) - All Fixed

### Critical Issues Fixed
1. **record_supervisor_rule** - Now uses `supervisor_learned_rules` table
2. **test_results table** - Created with full schema
3. **courier.md** - Copied to correct `prompts/` directory

### Non-Critical Issues Fixed
1. **RPC allowlist** - Reorganized with categories, removed unused entries
2. **Hardcoded "main" branch** - Now uses `cfg.GetDefaultMergeTarget()`
3. **Hardcoded "origin" remote** - Now configurable via `git.remote_name`
4. **plan_lifecycle.json** - Copied to `governor/config/`
5. **config/prompts/** - Marked as deprecated with README

---

## Key Principles

- **All changes go through task system** - Nothing implemented directly
- **Hats, not fixed roles** - Any model can wear any hat
- **Everything configurable** - Nothing hardcoded (max_rounds, member_count, etc.)
- **Everything learns** - All agents improve over time
- **Human reviews only** complex suggestions, API credit, UI/UX

---

## What NOT to Do

- Don't look for keys in `.env` (it's empty)
- Don't use Python code (it's in legacy/)
- Don't hardcode keys anywhere
- Don't hardcode destination IDs in code
- Don't hardcode routing logic in code
- Don't add TOOL: format back - it's gone for good
- Don't modify cleanup script without understanding cgroup logic
- Don't hardcode any defaults - everything in config files
- Don't implement anything directly - all changes through task system
- Don't hardcode revision rounds or council member counts - use config
