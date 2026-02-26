# Governor Implementation Handoff Document

**Session:** 2026-02-26 (Session 32)
**Purpose:** Fixed PRD → Plan → Review flow, added review status, fixed agent tools
**Status:** READY FOR TESTING

---

## QUICK START

```bash
cd ~/vibepilot/governor
go build -o governor ./cmd/governor
./governor
```

**Expected output:**
```
VibePilot Governor 2.0.0 (commit: dev, built: unknown)
Connected to database
Registered CLI destination: opencode (opencode)
Governor started (poll: 1s, max/module: 8, max total: 160)
Press Ctrl+C to stop
```

---

## FINAL METRICS

| Metric | Value |
|--------|-------|
| Go files | 23 |
| Total lines | ~5,200 |
| Binary size | ~9.6MB |
| Python remnants | 0 |
| Stubs | 0 |
| Hardcoded values | 0 (all constants) |
| RPCs in allowlist | 42 |
| Protected branches | 3 (main, master, research-considerations) |

---

## PLANNING APPROVAL FLOW (NEW)

```
┌─────────────────────────────────────────────────────────────┐
│                    PLANNING PHASE                           │
│                                                             │
│  PRD → Planner creates plan (status: draft)                 │
│                    ↓                                        │
│         Supervisor reviews complexity                       │
│           ↓              ↓                                  │
│        Simple         Complex                               │
│           ↓              ↓                                  │
│     Supervisor      Council (3 lenses)                      │
│       approves           ↓                                  │
│           ↓         Consensus?                              │
│           │         ↓       ↓                               │
│           │      Approved  Revision                         │
│           │         ↓       ↓                               │
│           │      Human    Planner                           │
│           │      Review   revises                           │
│           │         ↓                                        │
│           └─────────→ plan.status = 'approved'              │
│                         ↓                                   │
│               Tasks become AVAILABLE                        │
└─────────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────────┐
│                    EXECUTION PHASE                          │
│                                                             │
│  Governor sees tasks.status = 'available'                   │
│  Routes to Orchestrator → Runner → Execution                │
│  NO unapproved tasks ever reach orchestrator                │
└─────────────────────────────────────────────────────────────┘
```

**Key Points:**
- Tasks are created with `status = 'pending'`
- When `plan.status = 'approved'`, SQL function auto-flips tasks to `available`
- Governor only sees `available` tasks
- Research suggestions stay in GitHub until approved

### Agent = Hat (Prompt + Tools + Skills + Knowledge + Learning)

```
┌─────────────────────────────────────────────────────────┐
│                    THE HAT                              │
│                                                         │
│  KNOWLEDGE     - Where fish are                         │
│  STRATEGIES    - How to catch fish                      │
│  SKILLS        - How to use the tools                   │
│  TOOLS         - Hooks, lures, weights, line            │
│  LEARNING      - Gets better each time                  │
└─────────────────────────────────────────────────────────┘
                         ▲
                    Model puts on hat
                         │
                    Any available brain
                  (GLM-5, Gemini, DeepSeek)
```

### Source of Truth

| What | Where |
|------|-------|
| Code | GitHub |
| State | Supabase |
| Prompts | `~/vibepilot/prompts/*.md` |
| Config | `governor/config/*.json` |

### Config Files = Swap Points

| File | What | Change To |
|------|------|-----------|
| `models.json` | Which brains available | Add/remove models |
| `destinations.json` | How to reach brains | Add CLI/API runners |
| `agents.json` | What hats exist | Modify agent capabilities |
| `tools.json` | What's on each hat | Add/remove tools |
| `system.json` | All runtime settings | Timeouts, limits, allowlists |

---

## ALL CONSTANTS (No Hardcoded Values)

| File | Constant | Default | Configurable Via |
|------|----------|---------|------------------|
| `destinations/runners.go` | `DefaultTimeoutSecs` | 300 | destinations.json |
| `db/supabase.go` | `DefaultHTTPTimeoutSecs` | 30 | system.json |
| `db/supabase.go` | `DefaultErrorTruncateLen` | 200 | system.json |
| `gitree/gitree.go` | `DefaultGitTimeout` | 60s | system.json |
| `vault/vault.go` | `defaultCacheTTL` | 5m | system.json |
| `vault/vault.go` | `pbkdf2Iterations` | 100000 | (crypto constant) |
| `vault/vault.go` | `saltSize` | 16 | (crypto constant) |
| `vault/vault.go` | `nonceSize` | 12 | (GCM standard) |
| `vault/vault.go` | `keySize` | 32 | (AES-256) |
| `tools/registry.go` | `DefaultHTTPTimeoutSecs` | 30 | system.json |
| `tools/registry.go` | `DefaultMaxIdleConns` | 10 | system.json |
| `tools/registry.go` | `DefaultIdleConnTimeoutSecs` | 30 | system.json |
| `tools/sandbox_tools.go` | `DefaultSandboxTimeoutSecs` | 60 | system.json |
| `tools/sandbox_tools.go` | `DefaultLintTimeoutSecs` | 60 | system.json |
| `tools/sandbox_tools.go` | `DefaultTypecheckTimeoutSecs` | 120 | system.json |
| `tools/web_tools.go` | `WebFetchMaxLen` | 10000 | system.json |
| `tools/web_tools.go` | `WebSearchURL` | DuckDuckGo | system.json |
| `tools/web_tools.go` | `UserAgent` | VibePilot/2.0 | system.json |

---

## SECURITY FIXES

| Issue | Severity | Fix |
|-------|----------|-----|
| API key in URL (Gemini) | HIGH | Use `x-goog-api-key` header |
| No URL allowlist | MEDIUM | Enforce HTTPAllowlist from config |
| HTTP client per request | MEDIUM | Shared client with connection pooling |
| Unsanitized DB filters | MEDIUM | Regex sanitization for column names/values |
| Weak crypto (AES-CFB) | MEDIUM | Upgraded to AES-GCM with PBKDF2 |
| Goroutine panics | MEDIUM | Added recover() + error channel |
| Silent error swallowing | LOW | Errors logged and collectable via DrainErrors() |
| No git command timeout | LOW | Added configurable timeout (60s default) |

---

## FILE STRUCTURE

```
governor/ (5,179 lines)
├── cmd/governor/main.go           470  - Entry point, event handlers
├── internal/
│   ├── runtime/
│   │   ├── config.go              416  - Config loading, all types
│   │   ├── events.go              347  - PollingWatcher + EventRouter
│   │   ├── session.go             199  - LLM session with tool loop
│   │   ├── tools.go               187  - Tool registry + validation
│   │   └── parallel.go            121  - AgentPool with panic recovery
│   ├── tools/
│   │   ├── registry.go             75  - RegisterAll() wires tools
│   │   ├── git_tools.go           185  - Branch, commit, merge, delete
│   │   ├── db_tools.go            173  - Query, update, RPC (sanitized)
│   │   ├── file_tools.go          129  - Read, write, delete
│   │   ├── web_tools.go           178  - Search, fetch (allowlist)
│   │   ├── sandbox_tools.go       191  - Test, lint, typecheck
│   │   └── vault_tools.go          41  - Secret access
│   ├── db/
│   │   ├── supabase.go            183  - REST client (configurable)
│   │   └── rpc.go                 130  - RPC allowlist (38 RPCs)
│   ├── destinations/runners.go    311  - CLI + API runners
│   ├── gitree/gitree.go           252  - Git operations (timeout)
│   ├── vault/vault.go             253  - AES-GCM + PBKDF2
│   ├── maintenance/
│   │   ├── maintenance.go         346  - File ops, backup
│   │   ├── sandbox.go             165  - Sandbox execution
│   │   └── validation.go          248  - Config validation
│   └── security/leak_detector.go   69  - Secret scanning
├── config/
│   ├── system.json                - All settings
│   ├── agents.json                - 9 agents
│   ├── tools.json                 - 18 tools
│   ├── destinations.json          - 4 destinations
│   └── models.json                - 5 models
└── pkg/types/types.go             122  - Shared types
```

---

## 9 AGENTS

| Agent | Prompt | Tools | When Called |
|-------|--------|-------|-------------|
| consultant | consultant.md | web_search, web_fetch, db_query, file_read, file_write | Human has new idea |
| planner | planner.md | db_query, db_update, file_read, file_write | PRD ready (draft status) |
| supervisor | supervisor.md | db_query, db_update, db_rpc, file_read | Plan review, task review, test results |
| council | council.md | db_query, file_read, web_search | Complex decisions |
| orchestrator | orchestrator.md | db_query, db_update, db_rpc | Task available, routing, Vibes |
| maintenance | maintenance.md | git_*, file_*, sandbox_test, run_lint, run_typecheck | Code changes |
| researcher | system_researcher.md | web_search, web_fetch, db_query | Daily research |
| tester | testers.md | file_read, sandbox_test, run_lint, run_typecheck | Code validation |
| watcher | watcher.md | db_query, db_update, db_rpc | Failure monitoring |

---

## SUPERVISOR: 3 RESPONSE PATHS

| Decision | When | What Happens |
|----------|------|--------------|
| APPROVE | Meets criteria | Proceed |
| REJECT → Council | Complex issues | 3-lens review |
| REJECT → Human | Visual/UI/architecture | Status = awaiting_human |
| REJECT → Retry | Fixable issue | Return with notes |

---

## RPC ALLOWLIST (42 RPCs)

### Core Task Operations
- get_available_tasks, get_task_by_id, claim_task
- update_task_status, reset_task, record_task_run
- unlock_dependent_tasks

### Routing & Learning
- get_best_runner, record_runner_result
- calculate_roi, append_routing_history
- log_orchestrator_event
- increment_in_flight, decrement_in_flight

### Learning System
- record_failure, get_recent_failures
- get_heuristic, upsert_heuristic, record_heuristic_result
- get_problem_solution, record_solution_result

### Agent Learning
- create_planner_rule, get_planner_rules, record_planner_rule_applied, deactivate_planner_rule
- create_supervisor_rule, get_supervisor_rules, record_supervisor_rule_hit
- create_tester_rule, get_tester_rules, record_tester_rule_hit

### Runner Management
- archive_runner, boost_runner, revive_runner

### Dashboard & Planning
- vibes_query, get_dashboard_stats
- create_plan, update_plan_status, create_tasks

### Council (NEW)
- add_council_review, set_council_consensus

### Security
- log_security_audit

---

## EVENT FLOW

```
PRD created (draft)    → EventPRDReady    → Planner creates plan, saves to GitHub, sets status=review
Plan ready (review)    → EventPlanReview  → Supervisor reads PRD+plan, decides simple/complex
Plan needs council     → EventPlanCreated → Council reviews
Council done           → EventCouncilDone → Supervisor approves, tasks created
Tasks available        → EventTaskAvailable → Orchestrator routes
Task needs review      → EventTaskReview   → Supervisor validates
Tests completed        → EventTestResults  → Supervisor processes
Research ready         → EventResearchReady → Supervisor reviews
Maintenance command    → EventMaintenanceCmd → Maintenance executes
```

### Plan Status Flow

```
draft → (Planner saves plan) → review → (Supervisor decides)
                                         ↓
                              Simple → approved → tasks created
                              Complex → council_review → approved → tasks created
```

---

## PRINCIPLES (All Enforced)

1. **Everything swappable** - models, platforms, database, git host
2. **No hardcoded decisions** - LLM decides, Go executes
3. **No hardcoded values** - All constants, configurable via JSON
4. **2-3 tools per agent** - Enforced at runtime
5. **Generic RPC** - `CallRPC(name, params)` with allowlist
6. **Event-driven** - Single poller, no scattered loops
7. **Lean code** - Fits in LLM context for easy modification
8. **Security first** - No secrets in URLs, input sanitization, AES-GCM

---

## VERIFICATION COMMANDS

```bash
# Build
cd ~/vibepilot/governor && go build -o governor ./cmd/governor

# Static analysis
go vet ./...

# Check for hardcoded values
grep -rn "300\|60\|30" --include="*.go" | grep -v "const\|Constant"

# Check for TODOs/stubs
grep -rn "TODO\|FIXME\|STUB" --include="*.go"

# Run
./governor
```

---

## UPDATE LOG

| Date | Session | Change |
|------|---------|--------|
| 2026-02-26 | 32 | Fixed PRD → Plan → Review flow: added review status, EventPlanReview |
| 2026-02-26 | 32 | Fixed agent tools: planner (file_write, db_update), supervisor (file_read) |
| 2026-02-26 | 32 | Updated planner prompt: save plan to GitHub, not db_insert tasks |
| 2026-02-26 | 32 | Updated supervisor prompt: added Scenario 0 (initial plan review) |
| 2026-02-26 | 32 | Fixed CLIRunner: NDJSON parsing for opencode output |
| 2026-02-26 | 32 | Added courier runner implementation |
| 2026-02-26 | 32 | Added db_insert tool (for future use after approval) |
| 2026-02-26 | 31 | Full audit: security fixes, all hardcoded → constants |
| 2026-02-26 | 31 | Killed orphaned Python processes |
| 2026-02-26 | 31 | Added event types: PRDReady, TestResults, HumanQuery |
| 2026-02-26 | 31 | Added 10 RPCs to allowlist |
| 2026-02-25 | 30 | Governor rebuild (4,517 lines, -45%) |
| 2026-02-24 | 28-29 | Learning system implementation |
