# VibePilot Governor Intelligence Fix -- Design Document

**Date:** 2026-04-21
**Status:** PENDING REVIEW (rev 2 -- incorporates user feedback)
**Scope:** 4 interdependent root causes, 1 coherent fix unit

---

## Problem Statement

The governor pipeline has never completed an E2E run successfully. The root causes form a cascade:

```
Stale code map → Planner invents files → Supervisor can't verify → Executor produces garbage
```

Each failure amplifies the next. Fixing any one in isolation leaves the cascade broken.

---

## Root Causes & Fixes

### ROOT CAUSE 1: Stale Code Map
**Problem:** `.context/map.md` is only as fresh as the last manual jcodemunch run. If the repo changes, every agent works from outdated information.
**Impact:** Planner hallucinates file paths. Supervisor can't verify references. Executor can't find files.

**Fix A: Startup code map refresh**
- On governor boot, before event processing starts, run jcodemunch `index_folder` via the MCP registry
- Configurable: `system.json` → `code_map.refresh_on_startup: true` (default: true)
- Graceful: if jcodemunch fails, log warning and proceed with existing map

**Fix B: Configurable cache TTL**
- Replace `sync.Once` in context_builder.go with a TTL-based cache
- Configurable: `system.json` → `code_map.cache_ttl_seconds: 3600` (default: 3600 = 1 hour)
- After TTL expires, next access re-reads map.md from disk (no jcodemunch call, just file read)
- jcodemunch full re-index on a separate configurable interval or manual trigger only

**Fix C: Configurable paths**
- Code map path: `system.json` → `code_map.path: ".context/map.md"` (default)
- No hardcoded paths in Go code

### ROOT CAUSE 2: Hardcoded Agent Context
**Problem:** Agent context strategy is hardcoded in Go switch blocks using string literals for agent IDs. Adding a new agent requires a code change.
**Impact:** Not modular. Not config-driven. Violates VibePilot's own philosophy.

**Fix: context_policy on AgentConfig**
- Add `context_policy` field to `AgentConfig` struct in config.go
- Add `context_policy` to each agent in `governor/config/agents.json`
- Session factory reads policy from config, not from hardcoded switch
- Eliminates duplicated switch block in session.go

**Policy values (simplified to 4):**
| Policy | What the agent gets | Who uses it |
|--------|-------------------|-------------|
| `full_map` | Complete code map + rules + failures + MCP tools | planner |
| `file_tree` | File tree (## headers) from code map | supervisor, council, consultant, researcher, maintenance, vibes |
| `targeted` | Only specific files from task's target_files list | task_runner |
| `none` | Nothing beyond prompt | (reserved) |

**Default:** If agent has no context_policy or value is empty, defaults to `file_tree`. Safe fallback.

### ROOT CAUSE 3: Task Executor Has No File Context
**Problem:** Task agents receive only `prompt_packet` and `expected_output` from the task's result JSONB. They have no idea what files exist or what code to modify.
**Impact:** Executors produce generic tutorial text instead of project-specific code.

**Fix: Complete task packet enrichment chain**

The chain has 4 links. All must work or none should ship:

**Link 1: Planner output includes deliverables (already works)**
- Planner JSON output already has `deliverables` and `expected_output.files_created/files_modified`
- No change needed to planner output format

**Link 2: Parser extracts target files from plan**
- Add `TargetFiles []string` to `TaskData` in validation.go
- Parse from plan markdown: extract from `**Target Files:**` section or `**Deliverables:**` section
- Planner prompt update: explicitly instruct planner to list target files in each task section

**Link 3: Target files stored with task**
- In `createTasksFromApprovedPlan()`, store target_files in `tasks.result` JSONB:
  ```json
  {
    "prompt_packet": "...",
    "expected_output": "...",
    "target_files": ["path/to/file1.go", "path/to/file2.go"]
  }
  ```
- This uses existing JSONB field. No schema change. No migration.
- Everything in Supabase + GitHub = fully recoverable. Git clone + migration replay + governor restart = full system.

**Link 4: Executor reads target files at execution time**
- In `handleTaskAvailable()`, after building the full prompt (prompt_packet + target file contents):
  1. Read `target_files` from task result JSONB
  2. For each file, read contents from disk (repo)
  3. Count total tokens (prompt + file contents)
  4. Append file contents to executor prompt
  5. Router receives token count and uses model context_limit from models.json to filter candidates
  6. Only models with `context_limit > prompt_tokens + output_buffer` are eligible
  7. If no model fits → task confidence should be low, planner should decompose

- File presentation format:
  ```
  ## Files You Will Modify

  ### path/to/file1.go
  [file contents]

  ### path/to/file2.go
  [file contents]
  ```

- If a file doesn't exist on disk: "[FILE DOES NOT YET EXIST - CREATE IT]"
- No arbitrary byte cap. The constraint comes from model context limits, which are already in models.json.
- If total prompt exceeds all available model context windows, the task is genuinely too large and needs decomposition. The system handles this naturally through confidence scoring.

### ROOT CAUSE 4: Supervisor Prompt Doesn't Verify
**Problem:** Supervisor prompt has no explicit instructions to verify file references or check plan alignment. Currently only checks task alignment, dependencies, and prompt specificity in vague terms.
**Impact:** Supervisor rubber-stamps bad plans. The entire quality gate is bypassed.

**Fix: Supervisor prompt enhancement -- objective checks only, no subjective limits**

1. **File Reference Verification:**
   - "For every file path mentioned in the plan, verify it exists in the Codebase File Tree"
   - "New files to be created are acceptable. Existing files referenced must match exactly"
   - "If any EXISTING file reference is wrong, reject with needs_revision"
   - This is an objective, binary check. File exists or it doesn't.

2. **Requirement Traceability:**
   - "Every task must trace to a specific PRD requirement"
   - "If a task does not clearly map to something the PRD asks for, flag it"
   - "Tasks that introduce infrastructure, abstractions, or utilities not requested by the PRD should be questioned"
   - This is objective: does the PRD mention it or not?

3. **Dependency Validation:**
   - "Check for circular dependencies between tasks"
   - "Verify dependency ordering is logical (foundation before features)"
   - Objective graph check.

**Explicitly NOT in the supervisor's job:**
- Task count limits (a 200-task PRD is fine if every task maps to a requirement)
- Over-engineering judgment (subjective -- let requirement traceability handle it)
- Task sizing (the planner's confidence scoring handles this -- 95% one-shot or decompose)
- Confidence score override (the planner owns confidence, supervisor just verifies it's justified)

The supervisor is a quality gate, not a redesign engine. Its job is: "does this plan faithfully and correctly implement the PRD?" Not "is this plan the way I would have designed it?"

---

## Files Changed (Complete Unit)

| File | Change | Size |
|------|--------|------|
| `governor/config/system.json` | Add `code_map` section (path, cache_ttl, refresh_on_startup) | ~8 lines |
| `governor/config/agents.json` | Add `context_policy` to each agent | ~12 lines |
| `governor/internal/runtime/config.go` | Add `ContextPolicy` to AgentConfig struct + `CodeMapConfig` to runtime config | ~15 lines |
| `governor/internal/runtime/context_builder.go` | Replace sync.Once with TTL cache, read config for paths, keep context methods | ~30 lines changed |
| `governor/internal/runtime/session.go` | Replace hardcoded switch with config-driven policy lookup, deduplicate | ~40 lines changed |
| `governor/cmd/governor/main.go` | Add jcodemunch startup refresh call before event loop | ~15 lines |
| `governor/cmd/governor/validation.go` | Add TargetFiles to TaskData, parse from plan markdown | ~20 lines |
| `governor/cmd/governor/handlers_task.go` | Read target files from task result, inject into executor prompt, pass token count to router | ~30 lines |
| `config/prompts/supervisor.md` | Add 3 objective verification checks | ~25 lines |
| `config/prompts/planner.md` | Add explicit "Target Files" instruction to task format | ~5 lines |

**Total: ~10 files, ~200 lines of meaningful change**

---

## What We're NOT Changing (and why)

1. **task_packets table** -- Currently orphaned (never written). The enrichment chain uses `tasks.result` JSONB instead. Rewiring to task_packets is a separate coherent unit. Not touching it avoids schema risk.

2. **Planner output JSON format** -- Already includes deliverables. We're just adding a markdown section for target files that the parser can extract.

3. **Dashboard** -- All changes are compatible. `routing_flag_reason` formats are passthrough strings. `tasks.result` JSONB gets a new `target_files` key but dashboard doesn't parse it. Status values unchanged.

4. **Database migrations** -- Zero migrations needed. Everything uses existing columns and JSONB flexibility.

5. **RPC functions** -- No new RPCs. `createTasksFromApprovedPlan` writes to existing `tasks.result`.

6. **models.json** -- Already has context_limit per model. No changes needed there -- router just needs to use it during routing.

---

## Configurability Checklist

| What | Config Key | Default | Location |
|------|-----------|---------|----------|
| Code map file path | `code_map.path` | `.context/map.md` | system.json |
| Code map cache TTL | `code_map.cache_ttl_seconds` | 3600 | system.json |
| Refresh on startup | `code_map.refresh_on_startup` | true | system.json |
| Agent context level | `context_policy` per agent | `file_tree` (if empty) | agents.json |
| Model context limit | `context_limit` per model | (already exists) | models.json |

No arbitrary caps. Model context limits drive the constraint naturally.

---

## Graceful Degradation

- **Code map missing:** Agents still function. Context builder returns empty string with HTML comment. No crash.
- **jcodemunch fails on startup:** Log warning, proceed with existing map. No crash.
- **Target file not found on disk:** Include "[FILE DOES NOT YET EXIST - CREATE IT]" note. Executor proceeds.
- **Target files not in task result:** Old tasks without target_files work fine -- no file injection, just prompt as before.
- **Agent has no context_policy:** Defaults to `file_tree`, gets file tree. Safe fallback.
- **Prompt exceeds all model context windows:** Task gets low confidence, planner should decompose. Natural system behavior.

---

## Dashboard Compatibility (Verified)

| Change | Dashboard Impact | Safe? |
|--------|-----------------|-------|
| tasks.result gets target_files key | Dashboard doesn't parse result JSONB | YES |
| routing_flag_reason exec_failed_by format | Dashboard passes through as summary text | YES |
| agents.json gets context_policy field | Dashboard doesn't read agents.json | YES |
| system.json gets code_map section | Dashboard doesn't read system.json | YES |
| Supervisor output format unchanged | Dashboard reads plan status, not review text | YES |

---

## Recoverability

Everything needed to rebuild from scratch:
- **GitHub:** PRD files, plan files, config JSONs, Go source, migrations
- **Supabase:** All task/plan/run data in tables with JSONB
- **.context/:** Auto-regenerated by jcodemunch (triggered on startup)

Fresh machine recovery: `git clone` → Supabase migration replay → `bash .context/build.sh` → governor starts → jcodemunch reindexes → pipeline resumes.

No data loss. No manual intervention.

---

## Test Plan

After implementation, verify with a simple E2E test:

1. **PRD:** "Hello World" -- a single endpoint that returns a greeting
2. **Expected:** 1-2 tasks, planner lists target files, supervisor approves, executor produces real code
3. **Verify:**
   - Code map was refreshed on startup (check logs)
   - Planner received full code map (check session prompt in logs)
   - Supervisor received file tree and verified references (check review output)
   - Executor received specific target file contents (check executor prompt in logs)
   - Executor prompt token count was checked against model context limits
   - Task completed with usable output
   - Dashboard shows correct status transitions

---

## Implementation Order (within the unit)

All changes are committed together, but implementation order matters for incremental build verification:

1. config.go -- Add ContextPolicy to AgentConfig, CodeMapConfig to runtime config
2. system.json -- Add code_map config section
3. agents.json -- Add context_policy to each agent
4. context_builder.go -- TTL cache, config-driven paths
5. session.go -- Config-driven policy lookup, deduplicate
6. main.go -- jcodemunch startup refresh
7. validation.go -- TargetFiles parsing
8. handlers_task.go -- Target file injection + token count to router
9. planner.md -- Target files instruction
10. supervisor.md -- Verification steps (file refs, traceability, deps)
11. Build + verify
12. Single commit

---

## APPROVAL REQUIRED

This design must be reviewed and approved before any implementation begins.

Questions resolved from user feedback:
- ~~7 context policy levels~~ → Simplified to 4 (full_map, file_tree, targeted, none)
- ~~50KB max file content cap~~ → No arbitrary cap. Model context_limit from models.json drives the constraint naturally through routing.
- ~~Task count limits~~ → Removed. Objective requirement traceability only.
- ~~Over-engineering detection~~ → Removed. Requirement traceability covers it objectively.
- ~~Task sizing~~ → Not supervisor's job. Planner confidence scoring handles this (95% one-shot or decompose).
