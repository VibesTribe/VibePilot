# VibePilot Comprehensive Audit

**Date:** 2026-03-03  
**Auditor:** GLM-5  
**Purpose:** Full codebase audit with Webs of Wisdom vision in mind

---

## EXECUTIVE SUMMARY

**Total Codebase:** 11,125 lines of Go code  
**Target:** ~4,000 lines (to fit in LLM context, run on e2-micro)  
**Gap:** 7,125 lines of excess (64% bloat)

### Critical Issues

| Issue | Severity | Lines Affected | Impact |
|-------|----------|----------------|--------|
| **Orphaned core package** | CRITICAL | 908 | Built but never wired - complete waste |
| **Monolithic main.go** | HIGH | 2,676 | 24% of codebase in one file - unmaintainable |
| **Dead Python code** | MEDIUM | 61 files | Cluttering repo, confusing agents |
| **Unclear execution flow** | HIGH | All | Can't verify basic task completion works |

---

## DETAILED FINDINGS

### 1. Orphaned Core Package (908 lines - COMPLETE WASTE)
**Location:** `governor/internal/core/`
**Files:**
- `state.go` (302 lines) - State machine implementation
- `checkpoint.go` (143 lines) - Checkpoint manager  
- `test_runner.go` (296 lines) - Sandboxed test runner
- `analyst.go` (123 lines) - Analysis agent
- `db_storage.go` (44 lines) - DB checkpoint storage
**Status:** 
- ❌ NOT imported in main.go
- ❌ Not imported in any other file
- ❌ Types not used outside core/ directory
- ✅ DB migration created (043_checkpoint.sql)
- ❌ Migration NOT deployed to Supabase
**Recommendation:**
- **Option A:** Delete entire package (save 908 lines)
- **Option B:** Wire it into main.go (but adds complexity)
- **Option C:** Salvage useful concepts, delete implementation

### 2. Monolithic main.go (2,676 lines - 24% of codebase)
**Functions in main.go:**
1. `main()` - Entry point
2. `getConfigDir()` - Config loading
3. `getEnvOrDefault()` - Env helper
4. `registerDestinations()` - Destination registration
5. `setupEventHandlers()` - **MASSIVE** - 1,900+ lines
6. `recordModelSuccess()` - Metrics
7. `recordModelFailure()` - Metrics
8. `truncateID()` - Helper
9. `truncateOutput()` - Helper
10. `extractCouncilReviews()` - Helper
11. `getRecoveryConfig()` - Recovery config
12. `runStartupRecovery()` - Recovery logic
13. `runProcessingRecovery()` - Recovery goroutine
14. `recoverStaleProcessing()` - Recovery helper
15. `ValidationError.Error()` - Error type
16. `ValidationFailedError.Error()` - Error type
17. `validateTasks()` - Task validation
18. `createTasksFromApprovedPlan()` - Task creation
19. `parseTasksFromPlanMarkdown()` - Parser
20. `parseTaskSection()` - Parser
**Problems:**
- `setupEventHandlers()` is 1,900+ lines - should be split into separate files
- Event handlers for: TaskAvailable, TaskReview, TaskCompleted, PlanReview, PlanCreated, CouncilDone, PRDReady, RevisionNeeded, CouncilReview, CouncilComplete, PlanApproved, PlanBlocked, PlanError, PRDIncomplete, TestResults, HumanQuery, MaintenanceCommand, ResearchReady, ResearchCouncil
- Recovery, validation, parsing all mixed together
**Recommendation:**
Split into modules:
- `cmd/governor/main.go` - Entry only (~100 lines)
- `internal/handlers/task_handlers.go` - Task events
- `internal/handlers/plan_handlers.go` - Plan events
- `internal/handlers/council_handlers.go` - Council events
- `internal/recovery/recovery.go` - Recovery logic
- `internal/validation/validator.go` - Validation logic
### 3. Dead Python Code (61 files)
**Location:** `legacy/python/`
**Status:** Not used, not maintained
**Recommendation:** Delete or move to separate archive repo

### 4. Event Detection Analysis (events.go - 621 lines)
**Current Flow:**
```
PollingWatcher.poll()
  ├── detectTaskEvents()     → filters by status + processing_by IS NULL
  ├── detectPlanEvents()       → filters by status + processing_by IS NULL
  ├── detectTestResults()      → filters by status
  ├── detectMaintenanceEvents() → filters by status + processing_by IS NULL
  ├── detectResearchEvents()    → filters by status
  ├── detectPRDReady()         → checks for new PRDs
```
**Processing Claim Logic:**
- Each event handler calls `set_processing` RPC before spawning agent
- Prevents duplicate spawns
- Recovery clears stale processing after timeout
**This appears correct.**

### 5. File-by-File Analysis
| Package | Files | Lines | Status |
|---------|-------|-------|--------|
| `cmd/governor` | 1 | 2,676 | Monolithic, needs split |
| `internal/runtime` | 12 | 3,518 | Core runtime, large but functional |
| `internal/destinations` | 2 | 632 | CLI/API runners, functional |
| `internal/tools` | 7 | 1,428 | Tool implementations, functional |
| `internal/maintenance` | 3 | 759 | Maintenance logic, functional |
| `internal/db` | 3 | 777 | Database layer, functional |
| `internal/vault` | 1 | 337 | Secret management, functional |
| `internal/gitree` | 1 | 379 | Git operations, functional |
| `internal/security` | 1 | 94 | Leak detection, functional |
| `internal/core` | 5 | 908 | **ORPHANED** - not wired |
| `pkg/types` | 1 | 38 | Type definitions, functional |

---

## THE REAL PROBLEM

**User reported:**
> Couldn't even manage a single simple task without spawning 8 duplicate opencode sessions

**Root Causes Identified:**

1. **Processing claims ARE implemented** - events.go filters `processing_by IS NULL`
2. **Each handler claims processing before spawning** - main.go lines 227-241, 385-399, etc.
3. **But:**
   - What happens when polling interval is 1s?
   - What happens when startup before processing claims are set?
   - Are there race conditions in pool submission?
   - Is opencode session count checked?

**Questions to**
1. Has a basic task flow ever been tested end-to-end?
2. Are processing claims actually persisting in DB?
3. Is there a concurrency limit being exceeded
4. What's the actual opencode session count during execution?

---

## RECOMMENDATIONS

### Immediate Actions (High Priority)

1. **Delete orphaned core package** (save 908 lines)
   - Or wire it if actually needed
2. **Test basic flow manually** - Submit one simple task, verify:
3. **Add session count monitoring** - Prevent > 2 opencode sessions
### Medium Priority

4. **Refactor main.go** - Split into modules
5. **Delete legacy Python** - Clean up repository
6. **Consolidate tools** - Check for duplicates
### Long Term

7. **Target 4,000 lines** - Need to cut 7,125 more lines
8. **Design for e2-micro** - Must fit in 1GB RAM
9. **Remove opencode dependency** - Build custom CLI

---

## ARCHITECTURE VS REALITY
**Design Goal:** Clean, lean, plug-and-play, 4k lines, fits in LLM context
**Reality:** 11,125 lines, monolithic, orphaned code  64% bloat

**We Gap:** 7,125 lines of excess code
