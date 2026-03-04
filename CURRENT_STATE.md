# VibePilot Current State

**Required reading:**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how
2. **`docs/SYSTEM_REFERENCE.md`** - What we have and how it works
3. **`docs/core_philosophy.md`** - Strategic mindset
4. **`docs/CODEBASE_ANALYSIS_20260303.md`** - Code audit results

---

**Last Updated:** 2026-03-03 23:40 UTC
**Updated By:** GLM-5
**Branch:** `main`
**Status:** ARCHITECTURE REFACTORING IN PROGRESS - handlers being extracted

---

## Session Summary (2026-03-03 - Session 46)

### What We Did:

**Phase 1: Documentation:**
1. ✅ Created SESSION_HANDOFF.md - Comprehensive handoff document for future sessions
2. ✅ Created AUDIT_SESSION_46.md - Full codebase audit findings

**Phase 2: Handler Extraction:**
1. ✅ Created `handlers_task.go` (531 lines) - Task event handlers
   - EventTaskAvailable
   - EventTaskReview
   - EventTaskCompleted
2. ✅ main.go reduced from 2197 → 1713 lines (484 lines, 22% reduction)

### Key Accomplishments:
- **main.go reduction:** 2197 → 1713 lines (-484 lines, 22% reduction)
- **Clean commit:** 1 atomic commit with clear message
- **Tests:** All 12 integration tests passing

### Files Changed This Session:
- `governor/cmd/governor/main.go` - Reduced from 2197 to 1713 lines
- `governor/cmd/governor/handlers_task.go` - NEW (531 lines)
- `SESSION_HANDOFF.md` - NEW (comprehensive handoff document)
- `docs/AUDIT_SESSION_46.md` - NEW (audit findings)

### Commits This Session:
1. `refactor: extract handlers_task.go from main.go` - Task handler extraction

---

## Session Summary (2026-03-03 - Session 45)

### What We Did:

**Phase 1: Rollback Broken Refactoring:**
1. ✅ Deleted all broken handler files from previous session
2. ✅ Restored main.go to working state (2,834 lines)
3. ✅ Verified all tests passing

**Phase 2: Package Rename (destinations → connectors):**
1. ✅ Renamed `internal/destinations/` to `internal/connectors/`
2. ✅ Renamed `DestinationConfig` → `ConnectorConfig`
3. ✅ Renamed `DestinationRunner` → `ConnectorRunner`
4. ✅ Updated all references across codebase
5. ✅ Renamed `registerDestinations` → `registerConnectors`

**Phase 3: Handler Extraction (IN PROGRESS):**
1. ✅ Created `types.go` (41 lines) - RecoveryConfig, TaskData, ValidationError types
2. ✅ Created `adapters.go` (36 lines) - dbCheckpointAdapter
3. ✅ Created `recovery.go` (255 lines) - All recovery functions
4. ✅ Created `validation.go` (276 lines) - Task validation, plan parsing
5. ⬜ Handler files (task, plan, council, etc.) - NOT STARTED

### Key Accomplishments:
- **Architecture clarity**: Connectors = HOW to connect to models, Destinations = WHERE couriers go
- **main.go reduction**: 2,834 → 2,261 lines (-573 lines, 20% reduction)
- **Clean commits**: 4 atomic commits with clear messages
- **Tests**: All passing after each change

### Files Changed This Session:
- `governor/internal/destinations/` → `governor/internal/connectors/` (package renamed)
- `governor/internal/runtime/config.go` - ConnectorConfig, GetConnector methods
- `governor/internal/runtime/session.go` - ConnectorRunner, RegisterConnector
- `governor/internal/runtime/router.go` - GetAvailableConnectors, GetConnectorCategory
- `governor/cmd/governor/main.go` - Reduced from 2,834 to 2,261 lines
- `governor/cmd/governor/types.go` - NEW (41 lines)
- `governor/cmd/governor/adapters.go` - NEW (36 lines)
- `governor/cmd/governor/recovery.go` - NEW (255 lines)
- `governor/cmd/governor/validation.go` - NEW (276 lines)

### Commits This Session:
1. `refactor: rename destinations package to connectors` - Package rename
2. `refactor: extract types.go and adapters.go from main.go` - First extraction
3. `refactor: extract recovery.go from main.go` - Recovery functions
4. `refactor: extract validation.go from main.go` - Validation functions

---

## Architecture Clarification (This Session)

### Connector vs Destination

| Term | Definition | Example |
|------|------------|---------|
| **Connector** | HOW we connect to a model | CLI (opencode), API (OpenAI, Anthropic) |
| **Destination** | WHERE couriers go (web AI platforms) | deepseek.com, chatgpt.com, claude.ai |

**Internal agents** (planner, supervisor, task_runner, council):
- Need: Model + Connector
- No "destination" concept

**Courier agents**:
- Need: Model + Connector + Destination (web platform URL)
- Destination is passed as parameter to courier connector

---

## Current Codebase State

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| **Total Go lines** | ~11,000 | 4,000-8,000 | ⚠️ Over target |
| **main.go lines** | 1,179 | <500 | ⚠️ In progress |
| **Handler files** | 7 created | 10 planned | ⬜ 3 remaining |

### File Structure (Current)

```
cmd/governor/
├── main.go              (1,179 lines) - Event handlers, wireup - IN PROGRESS
├── handlers_task.go     (531 lines)  - Task event handlers ✅ NEW
├── handlers_plan.go     (576 lines)  - Plan event handlers ✅ NEW
├── types.go             (41 lines)   - Shared types ✅
├── adapters.go          (36 lines)   - DB adapters ✅
├── recovery.go          (255 lines)  - Recovery functions ✅
├── validation.go        (276 lines)  - Validation logic ✅
├── helpers.go           (72 lines)   - Shared helpers ✅
└── main_test.go         (405 lines)  - Integration tests

Total: 3,371 lines (was 2,834 monolithic, now modular)
```

### Remaining Extractions

| File | Est. Lines | Contains | Status |
|------|------------|----------|--------|
| `handlers_task.go` | ~530 | EventTaskAvailable, EventTaskReview, EventTaskCompleted | ✅ DONE |
| `handlers_plan.go` | ~575 | EventPRDReady, EventPlanReview, EventRevisionNeeded, etc. | ✅ DONE |
| `handlers_council.go` | ~425 | EventCouncilReview, EventCouncilDone | ⬜ Next |
| `handlers_research.go` | ~310 | EventResearchReady, EventResearchCouncil | ⬜ |
| `handlers_maint.go` | ~55 | EventMaintenanceCmd | ⬜ |
| `handlers_testing.go` | ~120 | EventTestResults | ⬜ |

**Expected final main.go:** ~150 lines (entry point + wireup only)

---

## Next Priorities

### HIGH PRIORITY - Continue main.go Split

**Current:** 2,261 lines in main.go
**Target:** <500 lines per file, main.go ~150 lines

**Next files to create:**
1. `handlers_task.go` - Extract task event handlers
2. `handlers_plan.go` - Extract plan lifecycle handlers
3. `handlers_council.go` - Extract council handlers
4. `handlers_research.go` - Extract research handlers
5. `handlers_maint.go` - Extract maintenance handler
6. `handlers_testing.go` - Extract test results handler

### MEDIUM PRIORITY - Maintenance Package

Refactor to use `map[string]any`:
1. Change `pkg/types.Task` → `map[string]any`
2. Update all type assertions
3. Wire into EventMaintenanceCmd handler

---

## Architecture Principles (From AGENTS.md)

**VibePilot is 100% swappable, portable, and vendor-agnostic:**

| Component | Can Swap To | How |
|-----------|-------------|-----|
| **Database** | Supabase → PostgreSQL → MySQL → SQLite → MongoDB | JSONB everywhere |
| **Code Host** | GitHub → GitLab → Bitbucket | Git-based |
| **AI CLI** | OpenCode → Claude CLI → Gemini CLI → Anything | Config-driven |
| **Hosting** | GCP → AWS → Azure → Local | Single binary |
| **Models** | Any LLM with any provider | Routing config |

**Rules:**
1. JSONB for arrays/objects (no TEXT[], no UUID[])
2. Config over code
3. No vendor-specific features
4. Pass slices directly to RPCs (no pre-marshaling)
5. All schema in `docs/supabase-schema/`

---

## Migrations Applied

| # | File | Status |
|---|------|--------|
| 057 | task_checkpoints.sql | ✅ Applied |
| 058 | jsonb_parameters.sql | ✅ Applied |

---

## Quick Commands

| Command | Action |
|---------|--------|
| `systemctl status vibepilot-governor` | Check if running |
| `journalctl -u vibepilot-governor -f` | Live logs |
| `cd ~/vibepilot/governor && go build -o governor ./cmd/governor` | Build |
| `cd ~/vibepilot/governor && go test ./cmd/governor/...` | Run tests |
| `~/vibepilot/scripts/e2e-checkpoint-test.sh` | E2E checkpoint test |
| `~/vibepilot/scripts/opencode-count.sh` | Count opencode sessions |

---

## What's Running

```
vibepilot-governor.service (Go binary)
├── Polls Supabase every 1s
├── Event-driven task execution
├── Checkpoint recovery on startup
├── Security leak detection on task output
├── 17 event handlers (main.go - being extracted)
└── 12 integration tests passing
```

---

## Tests

**Integration Tests:** `governor/cmd/governor/main_test.go`
- TestCheckpointRecovery_NoCheckpoints
- TestCheckpointRecovery_ExecutionStep
- TestCheckpointRecovery_ReviewStep
- TestCheckpointRecovery_TestingStep
- TestCheckpointRecovery_MultipleTasks
- TestCheckpointRecovery_JSONBParameter
- TestCheckpointRecovery_ProgressValues (5 subtests)
- TestCheckpointRecovery_TaskNumberParsing (4 subtests)
- TestCheckpointRecovery_StatusFilter (5 subtests)

**E2E Test:** `scripts/e2e-checkpoint-test.sh`
- Tests real Supabase RPCs
- Verifies create/load/delete checkpoint lifecycle

---

## Session History

### Session 45 (2026-03-03)
- Rolled back broken refactoring
- Renamed destinations → connectors (architecture clarity)
- Extracted types.go, adapters.go, recovery.go, validation.go
- main.go: 2,834 → 2,261 lines (-573)
- 4 clean atomic commits
- All tests passing

### Session 44 (2026-03-03)
- Database agnosticism (TEXT[] → JSONB)
- Checkpoint recovery working
- 12 integration tests
- Security leak detection wired
- Codebase audit complete

### Session 43 (2026-03-03)
- Core state machine wired
- Checkpoint system created
- Session limiter fixed

---

## Important Notes for Next Session

1. **Continue main.go split** - Extract handlers_task.go next
2. **One file at a time** - Build and test after each extraction
3. **Keep under 8k lines** - Currently ~11.5k, target 4-8k
4. **Security is active** - LeakDetector scans all outputs
5. **Tests are passing** - Run `go test ./cmd/governor/...` before changes
6. **Clean commits** - Atomic commits after each file extraction
