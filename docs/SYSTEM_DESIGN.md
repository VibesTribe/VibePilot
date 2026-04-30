# VibePilot System Design

**Last Updated:** 2026-03-09 Session 74

---

## Core Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     GITHUB (Code Source)                     │
│  - PRDs: docs/prd/*.md                                      │
│  - Plans: docs/plans/*.md                                    │
│  - Migrations: docs/supabase-schema/*.sql                   │
│  - Config: governor/config/*.json                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Push triggers
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    GOVERNOR (Go Backend)                     │
│                                                              │
│  Components:                                                 │
│  - Realtime Client: Listens to Supabase changes             │
│  - Event Router: Dispatches events to handlers              │
│  - Session Factory: Creates AI agent sessions               │
│  - Pool: Manages concurrent execution                        │
│  - Router: Selects model/connector for tasks                │
│  - Gitree: Git operations (branch, merge, commit)           │
│  - Vault: Retrieves encrypted secrets                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Writes state
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   SUPABASE (State Source)                    │
│                                                              │
│  Core Tables:                                                │
│  - tasks: Work items with status, assignment, slice_id      │
│  - task_runs: Execution history, tokens, costs              │
│  - plans: PRD plans with status                             │
│  - models: AI model registry (active/paused)                │
│  - platforms: Web platforms for courier routing             │
│                                                              │
│  Learning Tables:                                            │
│  - supervisor_learned_rules: Patterns from rejections       │
│  - tester_learned_rules: Patterns from test failures        │
│  - learned_heuristics: Model preferences per task type      │
│  - failure_records: Failure history for analysis            │
│  - problem_solutions: What fixes work (TODO)                │
│                                                              │
│  Infrastructure Tables:                                      │
│  - secrets_vault: Encrypted API keys                        │
│  - exchange_rates: Currency rates for ROI                   │
│  - task_history: Audit trail                                │
│  - orchestrator_events: Event log for timeline              │
│  - maintenance_commands: Admin commands                     │
│  - research_suggestions: Research queue                     │
│  - test_results: Test outcomes                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Realtime subscriptions
                              ▼
┌─────────────────────────────────────────────────────────────┐
│               VIBEFLOW DASHBOARD (React Frontend)            │
│                                                              │
│  Uses Supabase Realtime (NOT polling):                      │
│  - Channel: dashboard-tasks (tasks, runs, models, platforms)│
│  - Channel: dashboard-events (orchestrator_events)          │
│                                                              │
│  Displays:                                                   │
│  - Status Pills: Task counts by status                      │
│  - Slice Hub: Tasks grouped by slice_id                     │
│  - Task Cards: Details, assignment, tokens                  │
│  - Agent Hangar: Model/platform status                      │
│  - ROI Panel: Cost savings, subscriptions                   │
│  - Event Timeline: Orchestrator decisions                   │
└─────────────────────────────────────────────────────────────┘
```

---

## Tables Actually Used

### Core Tables (8)
| Table | Rows | Used By | Purpose |
|-------|------|---------|---------|
| `tasks` | 0 | Governor, Dashboard | Work items |
| `task_runs` | 0 | Governor, Dashboard | Execution records |
| `plans` | 0 | Governor | PRD plans |
| `models` | 15 | Governor, Dashboard | Model registry |
| `platforms` | 17 | Governor, Dashboard | Web platforms |
| `test_results` | 0 | Governor | Test outcomes |
| `research_suggestions` | 0 | Governor | Research queue |
| `maintenance_commands` | 12 | Governor | Admin commands |

### Learning Tables (6)
| Table | Rows | Status | Purpose |
|-------|------|--------|---------|
| `supervisor_learned_rules` | 42 | ✅ Active | Learning from rejections |
| `failure_records` | 332 | ✅ Active | Failure history |
| `tester_learned_rules` | 0 | ✅ Fixed S74 | Learning from test failures |
| `learned_heuristics` | 0 | ✅ Fixed S74 | Model preferences |
| `problem_solutions` | 0 | ⚠️ TODO | What fixes work |
| `lessons_learned` | 0 | ⚠️ TODO | General lessons |

### Infrastructure Tables (5)
| Table | Rows | Used By | Purpose |
|-------|------|---------|---------|
| `secrets_vault` | 8 | Governor | Encrypted API keys |
| `exchange_rates` | 1 | Dashboard | ROI currency conversion |
| `task_history` | 96 | Audit | Execution audit trail |
| `orchestrator_events` | 0 | Dashboard | Event timeline |

---

## Data Flow

### Task Execution Flow
```
1. PRD pushed to GitHub
2. Governor detects via Supabase (plan record created)
3. Planner creates tasks → writes to tasks table
4. Module branches created for each slice_id
5. Router selects model → writes to tasks.assigned_to
6. Task runner executes → writes to task_runs
7. Supervisor reviews → creates supervisor_learned_rules on rejection
8. Tester validates → creates tester_learned_rules on failure
9. On success → records heuristic for future routing
10. Dashboard shows progress via Realtime
```

### Learning Flow
```
1. Task fails/rejected
2. Failure recorded in failure_records
3. Rule created in appropriate learning table
4. Next task: Context builder fetches rules
5. Rules injected into agent prompt
6. Agent learns from past mistakes
```

---

## What NOT to Delete

### Critical Infrastructure
- `secrets_vault` - Encrypted API keys (would break all connectors)
- `exchange_rates` - ROI calculation (used by dashboard)
- `task_history` - Audit trail (has 96 records)

### Active Learning
- `supervisor_learned_rules` - 42 rules, actively used
- `failure_records` - 332 records, actively used
- `tester_learned_rules` - Will populate with fixes
- `learned_heuristics` - Will populate with fixes

### Core Operations
- `models` - 15 models, required for routing
- `platforms` - 17 platforms, required for web routing
- `maintenance_commands` - 12 commands, admin interface

---

## Legacy Tables (Needs Investigation)

These tables exist but may not be actively used. **DO NOT DELETE** without verification:

- `access`, `agent_messages`, `agent_tasks`
- `chat_queue`, `council_reviews`, `event_queue`
- `hardened_prd`, `lane_locks`, `platform_health`
- `project_drafts`, `project_structure`, `projects`
- `security_audit`, `skills`, `system_config`
- `task_backlog`, `task_checkpoints`, `task_packets`
- `tools`, `vibes_conversations`, `vibes_ideas`, `vibes_preferences`

**Before any cleanup:**
1. Check if referenced by any RPCs
2. Check if referenced by dashboard
3. Check if referenced by future feature plans
4. Verify no data loss risk
