# VibePilot Core Rebuild Analysis

**Date:** 2026-03-03
**Session:** Architecture Audit & Rebuild Planning
**Status:** Pre-rebuild documentation

---

## Executive Summary

After 2 weeks of Go development (7000+ lines, 55+ migrations), VibePilot has solid infrastructure but lacks the core state machine needed for VibeFlow 2.0.

**Decision:** Rebuild core with proper state machine, event sourcing, recovery logic, test execution, and dashboard feed. Keep existing infrastructure.

---

## Sources of Truth (Already Working)

| Source | Purpose | Status |
|--------|---------|--------|
| **Supabase** | State persistence, ROI, dashboard data | ✅ Working |
| **GitHub** | Commits, merges, branches | ✅ Working |
| **Vercel** | Dashboard deployment | ✅ Working |
| **Dashboard** | Reads from Supabase (via RPCs) | ✅ Working |

**The dashboard already shows live data from VibePilot operations.**

---

## What We Can Salvage (Keep As-Is)

### Infrastructure (All Good)

| Component | Location | Status | Why Keep |
|-----------|----------|--------|----------|
| Config System | `governor/internal/runtime/config.go` | ✅ Working | Swappable, JSON-driven, no hardcoding |
| Routing | `governor/internal/runtime/router.go` | ✅ Working | Model scoring, destination selection |
| Decision Parsing | `governor/internal/runtime/decision.go` | ✅ Working | Parses all agent outputs |
| Event Types | `governor/internal/runtime/events.go` | ✅ Working | 12+ event types defined |
| RPC Layer | `governor/internal/db/rpc.go` | ✅ Working | Allowlist, error handling |
| Database Layer | `governor/internal/db/supabase.go` | ✅ Working | Connection pooling, queries |
| Vault Security | `governor/internal/tools/vault_tools.go` | ✅ Working | Encrypted secrets |
| gitree Library | `governor/internal/gitree/gitree.go` | ✅ Working | Branch, commit, merge operations |
| Processing Claims | `governor/cmd/governor/main.go` | ✅ Working | Prevents duplicate processing |
| PRD Watcher | `governor/internal/runtime/prd_watcher.go` | ✅ Working | Detects new PRDs |

### Database Schema (Keep, May Simplify)

| Component | Status | Notes |
|-----------|--------|-------|
| Core tables (tasks, plans, task_runs) | ✅ Keep | Essential |
| ROI calculator views | ✅ Keep | Working, enhanced |
| Slice rollups | ✅ Keep | Dashboard uses these |
| Learning tables | ✅ Keep | Model scoring works |
| 55+ migrations | ✅ Keep | Already applied |
| State tracking (migrations 050-056) | ⚠️ Keep but verify | Added but not tested |

### Event Handlers (Logic Good, Structure Needs Work)

| Handler | Logic | Structure | Action |
|---------|-------|-----------|--------|
| EventPRDReady | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventPlanCreated | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventPlanReview | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventTaskAvailable | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventTaskCompleted | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventRevisionNeeded | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventCouncilReview | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventCouncilDone | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventTestResults | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventResearchReady | ✅ Good | ⚠️ Needs state machine | Rebuild with state |
| EventResearchCouncil | ✅ Good | ⚠️ Needs state machine | Rebuild with state |

**All handler LOGIC is correct. They parse decisions, call RPCs, update statuses. The problem is they don't checkpoint or enable recovery.**

---

## What We Need to Rebuild

### 1. State Machine (Core)

**Current:** Status fields updated, no state tracking, no recovery
**Needed:** Every transition recorded, resumable, checkpointed

```go
type StateMachine struct {
    state      *SystemState
    eventLog   *EventLog
    db         *DB
    subscribers []chan StateChange
}

func (sm *StateMachine) Apply(event Event) error {
    // 1. Append to event log (never fails)
    sm.eventLog.Append(event)
    
    // 2. Update state (atomic)
    sm.state = sm.reduce(sm.state, event)
    
    // 3. Persist to Supabase
    sm.persist()
    
    // 4. Notify subscribers
    sm.notify(event)
    
    return nil
}
```

### 2. Event Sourcing (Audit Trail)

**Current:** Events detected but not persisted properly
**Needed:** Every action appends to log, replayable

```go
type Event struct {
    ID         string
    Type       EventType
    EntityType string    // "task", "plan", "agent"
    EntityID   string
    Timestamp  time.Time
    Details    map[string]any
    Checkpoint *Checkpoint // Optional: for long-running operations
}
```

### 3. Checkpointing (Work Preservation)

**Current:** Work lives in memory until completion → LOST on crash
**Needed:** Checkpoint at 25%, 50%, 75% progress

```sql
CREATE TABLE task_checkpoints (
  id UUID PRIMARY KEY,
  task_id UUID REFERENCES tasks(id),
  progress_pct INT,
  output_so_far TEXT,
  files_created JSONB,
  state JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 4. Recovery Logic (Resume from Crash)

**Current:** Timeout-based, clears claim, work lost
**Needed:** State-based, read checkpoint, continue

```go
func (sm *StateMachine) Recover() {
    // 1. Load last state from Supabase
    sm.state = sm.load()
    
    // 2. Find anything in_progress
    for _, task := range sm.state.Tasks {
        if task.Status == "in_progress" {
            // 3. Get checkpoint
            checkpoint := sm.getCheckpoint(task.ID)
            
            // 4. Resume from checkpoint
            if checkpoint.Progress > 0 {
                sm.emit(Event{Type: EventResume, EntityID: task.ID})
            }
        }
    }
}
```

### 5. Test Execution (Missing Entirely)

**Current:** Handler exists, table exists, NO execution code
**Needed:** Sandbox → run tests → capture results

```go
type TestRunner struct {
    sandbox    *Sandbox
    timeout    time.Duration
}

func (tr *TestRunner) Execute(ctx context.Context, taskID string) (*TestResult, error) {
    // 1. Create sandbox
    sandbox := tr.sandbox.Create(taskID)
    defer sandbox.Cleanup()
    
    // 2. Checkout task branch
    sandbox.Checkout(fmt.Sprintf("task/%s", taskID))
    
    // 3. Find and run tests
    result := sandbox.Run("npm test") // or make test, etc.
    
    // 4. Capture results
    return &TestResult{
        Passed:  result.ExitCode == 0,
        Output:  result.Output,
        Coverage: result.Coverage,
    }, nil
}
```

### 6. Self-Improvement Loop (Not Wired)

**Current:** Learning RPCs exist, data collected, NOT analyzed
**Needed:** Daily analysis agent, pattern detection, prompt updates

```go
type SelfImprovement struct {
    db         *DB
    researcher *Agent
    scheduler  *Scheduler
}

func (si *SelfImprovement) RunDaily() {
    // 1. Analyze failures
    patterns := si.detectPatterns()
    
    // 2. Generate improvement suggestions
    suggestions := si.researcher.Analyze(patterns)
    
    // 3. Route to council or apply directly
    for _, s := range suggestions {
        if s.Complexity == "simple" {
            si.applyDirectly(s)
        } else {
            si.routeToCouncil(s)
        }
    }
}
```

### 7. Error State Recovery (Dead Ends)

**Current:** error → stuck forever
**Needed:** error → retry → escalate → human → recover

```go
type ErrorRecovery struct {
    maxRetries    int
    backoff       time.Duration
    escalateAfter int
}

func (er *ErrorRecovery) Handle(err error, entity Entity) {
    attempts := entity.Attempts()
    
    if attempts < er.maxRetries {
        // Retry with backoff
        time.Sleep(er.backoff * time.Duration(attempts))
        er.retry(entity)
    } else if attempts < er.escalateAfter {
        // Escalate to human
        er.escalate(entity, err)
    } else {
        // Human intervention required
        er.pauseForHuman(entity)
    }
}
```

---

## Architecture Comparison

### VibeFlow (Working Pattern)

```
┌─────────────────────────────────────────┐
│         task.state.json                 │
│    (Single source of truth)             │
├─────────────────────────────────────────┤
│  - tasks[]                              │
│  - agents[]                             │
│  - metrics{}                            │
│  - failures[]                           │
│  - updated_at                           │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│         events.log.jsonl                │
│    (Append-only history)                │
├─────────────────────────────────────────┤
│  {"type":"task_started",...}            │
│  {"type":"task_checkpoint",...}         │
│  {"type":"task_completed",...}          │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│           Dashboard                     │
│    (Reads ONE source)                   │
└─────────────────────────────────────────┘
```

### VibePilot Current (Broken Pattern)

```
┌────────────────┐  ┌────────────────┐  ┌────────────────┐
│    tasks       │  │     plans      │  │   task_runs    │
└────────────────┘  └────────────────┘  └────────────────┘
         │                  │                    │
         └──────────────────┼────────────────────┘
                            │
                   ┌────────▼────────┐
                   │   Scattered     │
                   │    State        │
                   │  (No recovery)  │
                   └─────────────────┘
```

### VibePilot Target (Proper Pattern)

```
┌─────────────────────────────────────────┐
│         Supabase (Source of Truth)      │
├─────────────────────────────────────────┤
│  system_state (JSONB document)          │
│  - tasks[], agents[], metrics{}         │
│  - updated_at, version                  │
├─────────────────────────────────────────┤
│  event_log (table)                      │
│  - id, type, entity_id, details         │
│  - created_at                           │
├─────────────────────────────────────────┤
│  task_checkpoints (table)               │
│  - task_id, progress_pct, state         │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│         State Machine (Go)              │
├─────────────────────────────────────────┤
│  - Load state from Supabase             │
│  - Apply events (append + reduce)       │
│  - Checkpoint long operations           │
│  - Recover from crashes                 │
│  - Notify subscribers                   │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│         Dashboard (VibeFlow/VibePilot)  │
├─────────────────────────────────────────┤
│  - Subscribes to state changes          │
│  - Renders from ONE query               │
│  - Real-time updates                    │
└─────────────────────────────────────────┘
```

---

## Implementation Plan

### Phase 1: Core State Machine (2-3 days)

1. Define `SystemState` struct
2. Build `StateMachine` with:
   - Load/Save to Supabase
   - Apply event (append + reduce)
   - Recover from crash
3. Add checkpointing RPC
4. Test: crash → recover → continue

### Phase 2: Event Sourcing (1-2 days)

1. Define all event types
2. Build event log table
3. Wire all handlers to emit events
4. Test: replay events → same state

### Phase 3: Test Execution (1-2 days)

1. Build test runner
2. Create sandbox
3. Wire to EventTaskCompleted
4. Test: task → tests → pass/fail

### Phase 4: Self-Improvement (1-2 days)

1. Wire daily analysis agent
2. Build pattern detection
3. Connect to research suggestions
4. Test: failures → patterns → suggestions

### Phase 5: Error Recovery (1 day)

1. Add retry logic with backoff
2. Add escalation paths
3. Add circuit breakers
4. Test: error → retry → escalate → human

### Total: 6-10 days

---

## File Structure (Post-Rebuild)

```
governor/
├── cmd/governor/
│   └── main.go              # Entry point (rebuilt to use state machine)
│
├── internal/
│   ├── core/                # NEW: Core state machine
│   │   ├── state.go         # SystemState, StateMachine
│   │   ├── events.go        # Event types, event log
│   │   ├── recovery.go      # Crash recovery logic
│   │   └── checkpoint.go    # Checkpoint management
│   │
│   ├── execution/           # NEW: Execution layer
│   │   ├── test_runner.go   # Test execution
│   │   ├── sandbox.go       # Isolated execution
│   │   └── parallel.go      # Parallel task execution
│   │
│   ├── improvement/         # NEW: Self-improvement
│   │   ├── analyzer.go      # Pattern detection
│   │   ├── researcher.go    # Daily analysis agent
│   │   └── updater.go       # Apply improvements
│   │
│   ├── runtime/             # KEEP: Existing infrastructure
│   │   ├── config.go        # ✅ Keep
│   │   ├── router.go        # ✅ Keep
│   │   ├── decision.go      # ✅ Keep
│   │   ├── events.go        # ⚠️ Merge with core/events.go
│   │   └── prd_watcher.go   # ✅ Keep
│   │
│   ├── db/                  # KEEP: Database layer
│   │   ├── supabase.go      # ✅ Keep
│   │   ├── rpc.go           # ✅ Keep
│   │   └── state.go         # ⚠️ Merge with core/state.go
│   │
│   ├── gitree/              # KEEP: Git operations
│   │   └── gitree.go        # ✅ Keep
│   │
│   └── tools/               # KEEP: Tool registry
│       ├── vault_tools.go   # ✅ Keep
│       └── registry.go      # ✅ Keep
```

---

## Migration Strategy

### What Gets Migrated

| From | To | Action |
|------|-----|--------|
| main.go (event handlers) | core/state.go | Rebuild with state machine |
| db/state.go | core/state.go | Merge into state machine |
| runtime/events.go | core/events.go | Merge with event sourcing |
| (new) | core/recovery.go | Add checkpoint + resume |
| (new) | execution/test_runner.go | Add test execution |
| (new) | improvement/analyzer.go | Add self-improvement |

### What Stays the Same

- All config files (models.json, destinations.json, etc.)
- All prompts (planner.md, supervisor.md, etc.)
- All database schema (55+ migrations)
- All RPC functions in Supabase
- gitree library
- Vault security
- Routing logic

---

## Success Criteria

After rebuild, VibePilot must:

1. **Never lose work** - Crash → recover → continue from checkpoint
2. **Run tests** - Every task gets tested before merge
3. **Self-improve** - Daily analysis → patterns → improvements
4. **Handle errors** - Retry → escalate → human intervention
5. **Scale to 50+ agents** - Parallel execution without confusion
6. **Show everything live** - Dashboard sees all state in real-time
7. **Be lean** - Fits on e2-micro (1GB RAM)
8. **Be robust** - No infinite loops, no dead ends, no stuck states

---

## Rollback Plan

If rebuild fails:

1. All existing code is in git (rollbackable)
2. All database schema is in migrations (rollbackable)
3. This document captures what to salvage
4. VibeFlow dashboard still works (independent)

---

## Related Documents

- `CURRENT_STATE.md` - Current project status
- `SYSTEM_REFERENCE.md` - What we have and how it works
- `ARCHITECTURE_ANALYSIS.md` - Previous architecture analysis
- `AUDIT_REPORT.md` - Previous audit
- `prd_v1.4.md` - Full system specification
- `core_philosophy.md` - Strategic mindset

---

## Conclusion

We have solid infrastructure (config, routing, gitree, vault). We need to rebuild the core state machine with proper event sourcing, checkpointing, and recovery. This is not a patch - it's a proper foundation.

**Estimated time:** 6-10 days
**Risk:** Low (existing code is rollbackable)
**Benefit:** VibeFlow 2.0 that actually works

---

*Document created: 2026-03-03*
*Last updated: 2026-03-03*
