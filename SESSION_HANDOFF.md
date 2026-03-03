# VibePilot Session Handoff

**STOP. READ THIS FIRST. THEN ACT.**

This document exists because 40%+ of context was wasted every session re-reading the same docs. Read this, then you know everything.

---

## WHAT WE ARE BUILDING

**VibePilot** = AI execution engine that builds production systems autonomously.

**Target System:** VibeFlow 2.0 - "Webs of Wisdom" - global multilingual multimedia legacy platform

**Constraints:**
- Codebase: 4,000-8,000 lines Go (fits in LLM context)
- RAM: 10-20MB (e2-micro free tier)
- Deployment: Single binary
- 100% swappable (models, DBs, hosting, CLI)

---

## THE INVOLIABLE PRINCIPLES

**Every decision must align with these. No exceptions.**

| Principle | The Test |
|-----------|----------|
| **Zero Vendor Lock-In** | Can we replace [X] in one day with zero code changes? |
| **Modular & Swappable** | Change one thing. Did anything else break? |
| **Exit Ready** | Can someone else take over tomorrow with zero friction? |
| **Reversible** | Can you revert in 5 minutes? If not, don't do it. |
| **Always Improving** | Did we consider a better way? |

---

## CODING RULES

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
| **HTTP allowlist** | IronClaw | Validate URLs before requests |
| **10-20MB footprint** | ZeroClaw | Single binary, minimal deps |

### From core_philosophy.md

| Rule | Why |
|------|-----|
| **Prevention over cure** | 1% cost of fixing |
| **Homework before action** | Never paint into a corner |
| **Design interfaces for things we don't need yet** | Future-proof |
| **Never break the principles** | If it can't be undone, it can't be done |

### From AGENTS.md

| Rule | Why |
|------|-----|
| **⛔ STOP. READ. SUMMARIZE. THEN ACT.** | Prevents reactive fixes |
| **⛔ NO MULTIPLE CHOICE FORMS** | User hates them |
| **⛔ NO TYPE 1 ERRORS** | Fundamental mistakes ruin everything downstream |
| **⛔ NEVER push dashboard/UI to main** | Vercel auto-deploys, breaks production |
| **⛔ NEVER act before human confirms** | One wrong move undoes months |

---

## WHAT NOT TO DO

| ❌ DON'T | What Happened |
|----------|---------------|
| **Test tasks with broken code** | Whack-a-mole fixing surface symptoms |
| **Add new code during cleanup** | Bloat increases, never decreases |
| **Fix bugs reactively** | Root cause persists, bug returns |
| **Skip reading docs** | Session 9 pushed CSS directly, broke dashboard |
| **Assume code is dead** | "Duplicate" CSS deleted, cascade failure |
| **Assume code is working** | Session 41 built 908 lines never wired in |
| **Spawn multiple agents for one task** | 8 duplicate opencode sessions |
| **Hardcode anything** | "Temporary" becomes permanent |
| **Use pkg/types.Task** | Wrong type system, use map[string]any |
| **Delete without understanding** | Could be "duplicate" CSS all over again |

---

## CURRENT STATE (Session 46)

### What's Done

| Component | Status | Notes |
|-----------|--------|-------|
| **Core state machine** | ✅ Created | internal/core/state.go (302 lines) |
| **Checkpoint manager** | ✅ Created | internal/core/checkpoint.go (143 lines) |
| **Test runner** | ✅ Created | internal/core/test_runner.go (296 lines) |
| **Analyst** | ✅ Created | internal/core/analyst.go (116 lines) |
| **Security leak detector** | ✅ Wired | Scans task outputs |
| **Processing claims** | ✅ Working | Prevents duplicate sessions |
| **Package rename** | ✅ Done | destinations → connectors |
| **types.go** | ✅ Extracted | 41 lines |
| **adapters.go** | ✅ Extracted | 36 lines (dbCheckpointAdapter) |
| **recovery.go** | ✅ Extracted | 255 lines |
| **validation.go** | ✅ Extracted | 276 lines |
| **helpers.go** | ✅ Extracted | 72 lines |

### What's In Progress

| Component | Status | Next |
|-----------|--------|------|
| **main.go split** | ⬜ 50% | 2,261 lines, target <200 |
| **Core wiring** | ⬜ Partial | StateMachine created but handlers use direct RPC |
| **TestRunner wiring** | ⬜ Not started | Created but never invoked |
| **Analyst wiring** | ⬜ Not started | Created but never scheduled |

### What's Dead (To Delete)

| Package | Lines | Why Dead |
|---------|-------|----------|
| internal/maintenance/ | 759 | Uses pkg/types.Task instead of map[string]any |
| pkg/types/ | 122 | Only used by dead maintenance package |

---

## ARCHITECTURE DECISIONS

### Connector vs Destination

| Term | Definition | Example |
|------|------------|---------|
| **Connector** | HOW we connect to a model | CLI (opencode), API (OpenAI) |
| **Destination** | WHERE couriers go | deepseek.com, chatgpt.com |

**Internal agents** (planner, supervisor, task_runner): Model + Connector
**Courier agents**: Model + Connector + Destination (URL as parameter)

### Core Package Purpose

The core package was built because the old system was broken:

| Old (Broken) | New (Core) |
|--------------|------------|
| Direct RPC scattered | StateMachine abstraction |
| No audit trail | Event log via Emit() |
| Crash = lost work | CheckpointManager recovery |
| No test execution | TestRunner sandboxed |
| No pattern learning | Analyst daily analysis |

**Current Gap:** Handlers receive stateMachine/checkpointMgr but use direct RPC instead.

### Vault Architecture

```
Bootstrap (environment, systemd injects):
├── SUPABASE_URL
├── SUPABASE_SERVICE_KEY (NOT anon - bypasses RLS)
└── VAULT_KEY (master decryption)

All other secrets:
├── Encrypted in secrets_vault table
├── RLS: service_role full, authenticated read-one-at-a-time
└── Decrypted at runtime with VAULT_KEY

Why you won't find keys:
- They're encrypted
- Only service_role can read
- Need VAULT_KEY to decrypt
```

---

## FILE STRUCTURE (Current)

```
governor/cmd/governor/
├── main.go           (2,261 lines) - Event handlers, wireup - SPLIT IN PROGRESS
├── types.go          (41 lines)   - RecoveryConfig, TaskData, ValidationError ✅
├── adapters.go       (36 lines)   - dbCheckpointAdapter ✅
├── recovery.go       (255 lines)  - All recovery functions ✅
├── validation.go     (276 lines)  - Task validation, plan parsing ✅
├── helpers.go        (72 lines)   - truncateID, recordModelSuccess, etc. ✅
└── main_test.go      (405 lines)  - Integration tests

governor/internal/
├── core/             (857 lines)  - State machine, checkpoint, test_runner, analyst
├── db/               (483 lines)  - Supabase operations
├── vault/            (337 lines)  - Encrypted secrets
├── gitree/           (379 lines)  - Git operations
├── connectors/       (632 lines)  - CLI/API runners (renamed from destinations)
├── security/         (69 lines)   - Leak detection
├── runtime/          (~3,500)     - Event polling, routing, sessions
├── tools/            (~1,000)     - Tool implementations
└── maintenance/      (759 lines)  - ❌ DEAD - uses wrong types
```

---

## EVENT HANDLERS (In main.go)

17 handlers, grouped by extraction target:

| Handler | Lines | Extract To |
|---------|-------|------------|
| EventTaskAvailable | ~200 | handlers_task.go |
| EventTaskReview | ~150 | handlers_task.go |
| EventTaskCompleted | ~150 | handlers_task.go |
| EventPRDReady | ~120 | handlers_plan.go |
| EventPlanReview | ~150 | handlers_plan.go |
| EventPlanCreated | ~70 | handlers_plan.go |
| EventPlanApproved | ~15 | handlers_plan.go |
| EventPlanBlocked | ~15 | handlers_plan.go |
| EventPlanError | ~15 | handlers_plan.go |
| EventRevisionNeeded | ~170 | handlers_plan.go |
| EventCouncilReview | ~250 | handlers_council.go |
| EventCouncilDone | ~200 | handlers_council.go |
| EventResearchReady | ~160 | handlers_research.go |
| EventResearchCouncil | ~160 | handlers_research.go |
| EventMaintenanceCmd | ~60 | handlers_maint.go |
| EventTestResults | ~160 | handlers_testing.go |
| EventPRDIncomplete | ~30 | handlers_plan.go |

---

## REMAINING WORK

### Phase 1: Continue main.go Split (NEXT)

Extract handlers to files:

1. **handlers_task.go** (~350 lines)
   - EventTaskAvailable, EventTaskReview, EventTaskCompleted
   - Requires: database, factory, pool, cfg, git, leakDetector, checkpointMgr

2. **handlers_plan.go** (~400 lines)
   - EventPRDReady, EventPlanReview, EventPlanCreated, EventPlanApproved
   - EventPlanBlocked, EventPlanError, EventRevisionNeeded, EventPRDIncomplete

3. **handlers_council.go** (~250 lines)
   - EventCouncilReview, EventCouncilDone

4. **handlers_research.go** (~200 lines)
   - EventResearchReady, EventResearchCouncil

5. **handlers_maint.go** (~80 lines)
   - EventMaintenanceCmd

6. **handlers_testing.go** (~100 lines)
   - EventTestResults

**Target:** main.go < 200 lines (entry point + wireup only)

### Phase 2: Wire Core Infrastructure

Replace direct RPC with core abstractions:

```go
// Current (handlers do this):
database.RPC(ctx, "save_checkpoint", map[string]any{...})
database.RPC(ctx, "update_task_status", map[string]any{...})

// Should be:
checkpointMgr.SaveProgress(ctx, taskID, step, progress, output, files)
stateMachine.Emit(ctx, core.Event{Type: core.EventTaskStarted, TaskID: taskID})
```

### Phase 3: Wire TestRunner

- Create TestRunner instance in main.go
- When task status = "testing", call testRunner.RunTests()
- Emit events via state machine

### Phase 4: Wire Analyst

- Create Analyst instance in main.go
- Schedule daily analysis
- Store results in DB

### Phase 5: Delete Dead Code

- internal/maintenance/ (759 lines)
- pkg/types/ (122 lines)

---

## HOW TO EXTRACT HANDLERS

### Pattern (From Session 45)

1. Create new file (e.g., handlers_task.go)
2. Add package main
3. Add necessary imports
4. Copy handler function from main.go
5. Create wrapper function that takes all dependencies:

```go
// handlers_task.go
package main

import (
    "context"
    "encoding/json"
    "log"
    // ... other imports
)

func setupTaskHandlers(
    ctx context.Context,
    router *runtime.EventRouter,
    factory *runtime.SessionFactory,
    pool *runtime.AgentPool,
    database *db.DB,
    cfg *runtime.Config,
    connRouter *runtime.Router,
    git *gitree.Gitree,
    checkpointMgr *core.CheckpointManager,
    leakDetector *security.LeakDetector,
) {
    selectDestination := func(agentID, taskID, taskType string) string {
        // ... routing logic
    }

    router.On(runtime.EventTaskAvailable, func(event runtime.Event) {
        // ... handler body
    })

    router.On(runtime.EventTaskReview, func(event runtime.Event) {
        // ... handler body
    })

    router.On(runtime.EventTaskCompleted, func(event runtime.Event) {
        // ... handler body
    })
}
```

6. In main.go, replace handler code with call:

```go
// main.go
func setupEventHandlers(...) {
    setupTaskHandlers(ctx, router, factory, pool, database, cfg, connRouter, git, checkpointMgr, leakDetector)
    setupPlanHandlers(ctx, router, ...)
    // etc.
}
```

7. Run tests: `go test ./cmd/governor/...`
8. Commit: `git commit -am "refactor: extract handlers_task.go from main.go"`

---

## TESTING

| Command | What It Does |
|---------|--------------|
| `cd ~/vibepilot/governor && go test ./cmd/governor/...` | Run integration tests |
| `cd ~/vibepilot/governor && go build -o governor ./cmd/governor` | Build binary |
| `systemctl status vibepilot-governor` | Check if running |
| `journalctl -u vibepilot-governor -f` | Live logs |

**All 12 tests must pass before committing.**

---

## MIGRATIONS

Applied:
- 057: task_checkpoints.sql ✅
- 058: jsonb_parameters.sql ✅

---

## QUICK REFERENCE

### What VibePilot Process Expects

1. **Idea → PRD** (Consultant/Researcher)
2. **PRD → Plan** (Planner, 95%+ confidence tasks)
3. **Plan Review** (Supervisor → Council if complex)
4. **Tasks Created** (After plan approved)
5. **Task Execution** (Orchestrator routes to runner)
6. **Output Review** (Supervisor checks quality)
7. **Tests Run** (TestRunner validates)
8. **Merge** (To module, then main)

### Status Values

- pending → available → in_progress → review → testing → complete
- failed (exhausted retries)
- revision_needed (for plans)

### Branch Lifecycle

- Task assigned → Create `task/{task_number}`
- Output committed → task branch
- Passes review → Merge to `module/{slice_id}`
- All module tasks complete → Merge to main
- Cleanup → Delete task/module branches

---

## GIT RULES

| Rule | Why |
|------|-----|
| **Dashboard/UI changes → feature branch** | Vercel auto-deploys from main |
| **Never push to main without approval** | Breaks production |
| **Backend changes → main OK** | Code is rollbackable |
| **Commit often, small units** | Easy to revert |
| **Stay on your branch** | Multi-agent coordination |

---

## WHAT TO READ NEXT

After this document, if you need more context:

1. `docs/SYSTEM_REFERENCE.md` - What we have and how it works
2. `docs/vibepilot_process.md` - Full system flow
3. `docs/CORE_STATE_MACHINE_DESIGN.md` - Why core exists
4. `docs/CORE_REBUILD_ANALYSIS.md` - What was broken
5. `docs/GO_IRON_STACK.md` - Claw patterns

---

## SESSION LOG

| Session | What | Key Changes |
|---------|------|-------------|
| 40 | Fix infinite event loop | Processing state (042) |
| 41 | Core package phase 1-4 | state.go, checkpoint.go, test_runner.go, analyst.go |
| 42-43 | Wire checkpointing | Migration 057, 058 |
| 44 | Security wiring | Leak detector wired |
| 45 | Architecture refactoring | destinations→connectors, extract types/adapters/recovery/validation/helpers |
| 46 | This handoff doc | SESSION_HANDOFF.md created |

---

**Last Updated:** 2026-03-03 (Session 46)
**Next Priority:** Extract handlers_task.go from main.go
