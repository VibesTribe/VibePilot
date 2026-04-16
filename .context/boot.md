# VibePilot Bootstrap
# Generated: 2026-04-16T00:22:54Z | Commit: d109fb1d | Branch: main
# AUTO-GENERATED. DO NOT EDIT. Run .context/build.sh to regenerate.
# Recovery: clone repo, bash .context/tools/install.sh, bash .context/build.sh

# TIER 0: NON-NEGOTIABLE RULES
# Hand-crafted. These exist because agents keep making these same mistakes.
# Edit this file directly. Not auto-generated.
# Located at: .context/tools/tier0-static.md
# build.sh copies this into boot.md verbatim.

## Why These Rules Exist

Every rule below exists because an agent has repeatedly caused real problems
by violating it. These are not theoretical -- each one cost significant time
and cleanup work.

## Core Principles (the WHY)

- **Modular and agnostic** -- swap any component in a day. No vendor dependency.
- **Config-driven** -- everything configurable via JSON in config/. Nothing baked in.
- **Reversible** -- every change can be undone. No one-way doors.
- **Recoverable** -- GitHub + Supabase = full rebuild anywhere, any device.

## Absolute Rules (the DO NOT)

1. **NEVER hardcode anything.** Every hardcoded value eventually requires undoing a mess.
   Models, connectors, routing, agents -- all in config JSON files.

2. **NEVER hunt for .env files.** Credentials live in Supabase vault. Period.
   Agents have burned 30+ minutes searching for .env files that don't exist,
   then another 30 figuring out vault access. Use vault. First time. Every time.

3. **NEVER guess -- check first.** Guessing creates cleanup work.
   Read the existing code, query knowledge.db, check what's there before inventing.

4. **NEVER apply migrations directly.** Always go through GitHub first.
   Supabase REST API cannot run DDL (CREATE TABLE, ALTER TABLE, etc).
   There is no programmatic way to apply migrations from this machine.
   The ONLY path: write the SQL file, push to GitHub, human applies via Dashboard.
   
   When creating a schema migration:
   - Check existing files: ls ~/VibePilot/docs/supabase-schema/ to find the next number
   - Write the file to ~/VibePilot/docs/supabase-schema/NNN_name.sql (dev repo)
   - Commit and push to GitHub main (VibesTribe/VibePilot repo, main branch only)
   - Then pull into ~/vibepilot/ (running copy): cd ~/vibepilot && git pull
   - Tell the human explicitly: "Apply migration NNN via Supabase SQL Editor"
   - Provide the direct GitHub link: https://github.com/VibesTribe/VibePilot/blob/main/docs/supabase-schema/NNN_name.sql
   - Human clicks link, copies SQL, pastes into Supabase Dashboard > SQL Editor, runs it
   - Do NOT skip this. Do NOT apply via REST. Do NOT assume it's done.
   - Every time an agent got this wrong, the human had to redo it manually.

5. **ALWAYS push to GitHub.** Local-only work gets lost. Commit and push.
   This has caused more lost work than anything else.

6. **NEVER take shortcuts or "do it later."** Every shortcut becomes a bigger problem later.
   "I'll just hardcode this for now" = days of cleanup later.
   "I'll skip the config and bake it in" = can't swap without rewrite.
   "I'll just put this here temporarily" = permanent technical debt.
   "I'll wire it later" = the wiring never happens and the next agent is blind.
   If you can't do it properly right now, stop and say so. Don't half-do it.
   The right amount of work is the full amount. Not a TODO comment that becomes someone else's crisis.

7. **Go SERVES VibePilot's design. It does NOT invent new processes.**
   Go is the plumbing that makes VibePilot's actual processes run.
   It writes to Supabase in the format the dashboard expects.
   Do NOT rewrite Go with hallucinated processes that aren't in the spec.
   Do NOT modify Supabase schema to accommodate hallucinated Go code.
   Supabase is the contract. Dashboard shows what VibePilot state IS.
   Go conforms to both, not the other way around.

## Operational Rules (the DO)

8. **Governor is a systemd user service.**
   Use: systemctl --user (not sudo systemctl)
   Logs: journalctl --user -u vibepilot-governor (not journalctl -u governor)
   Getting stuck on this wastes entire sessions.

9. **Roles are defined in config, not guessed.** Look them up, don't invent.

   **Supervisor (3 responsibilities):**
   - Reviews task plans against PRD for alignment + plan quality. Calls Council for complex plans.
   - Reviews task output against task prompt + expected output + quality/security. Approved -> testing -> auto-merge if pass.
   - Approves basic researcher suggestions (new platform/model). Complex architecture -> Council -> Human.

   **Council (2 responsibilities):**
   - Reviews complex task plans escalated by Supervisor.
   - Reviews complex architecture changes escalated by Supervisor.

   **Human (Vibes) -- NOT technical. Does NOT code, merge, debug, or review code:**
   - Reviews visual UI/UX after visual testing. Requests changes or approves.
   - Receives Council-reviewed architecture suggestions. Asks questions. Final yes/no.

   **Merge system -- fully automated, zero human involvement:**
   - Supervisor approves output -> testing phase -> auto-merge on pass
   - Task branch created, merged to module branch, task branch deleted
   - Module branch merged after further Supervisor + Tester review
   - Merge problems are solved by agents, not human

   **Maintenance -- ONLY agent with git write access. No approval = no change.**

10. **Read before you code.**
   Query knowledge.db for existing rules and patterns before starting any task.
   sqlite3 .context/knowledge.db "SELECT title,content FROM rules WHERE priority='high'"
   5 minutes of reading saves hours of rewriting.

## Human Boundaries

- Human is NOT technical. Never assume technical knowledge.
- Human sees visual outputs and makes aesthetic/UX decisions.
- Human makes strategic architecture decisions after Council review.
- Human does NOT code, merge, debug, review code, or fix merge conflicts.
- Respect human time: batch questions together, don't ask one at a time.

## Going Deeper

When you need more than these basics:
- Architecture details: sqlite3 .context/knowledge.db "SELECT title,content FROM rules WHERE priority='high'"
- All prompts: sqlite3 .context/knowledge.db "SELECT name,role FROM prompts"
- Search docs: sqlite3 .context/knowledge.db "SELECT title,file_path FROM docs WHERE title LIKE '%<topic>%'"
- Full code map: cat .context/map.md
- Context layer explained: docs/CONTEXT_KNOWLEDGE_LAYER.md

## Post-Task Update Discipline (NON-OPTIONAL)

After completing any significant task (new feature, bug fix, config change, research, doc update),
you MUST update the relevant docs to prevent the next agent from flying blind:

1. **CURRENT_STATE.md** -- Update the section(s) affected by your work.
   If you changed anything real, the state doc must reflect it.

2. **WYNTK (VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md)** -- If your change affects:
   - Architecture (new packages, moved files, new config files)
   - File paths (anything moved or renamed)
   - How something works (new procedures, changed commands)
   - Technical constraints (new hardware info, changed limits)
   Then update the relevant section. Don't rewrite the whole file -- patch what changed.

3. **tier0-static.md** -- If you discovered a new rule that agents keep violating,
   add it here. This is the single source of truth for rules.

4. **This TODO list** -- Mark items done, add new items discovered during work.

5. **Commit and push** -- Local-only work gets lost. Always push.

The .context/ layer (boot.md, knowledge.db, map.md) auto-rebuilds on commit.
But the SOURCE docs it reads from (tier0-static.md, CURRENT_STATE.md, WYNTK, etc.)
only get updated if you update them. The auto-build is useless if sources are stale.

**Why this matters:** Agents swap frequently. Each new agent starts from these docs.
A gap here means the next agent repeats mistakes, wastes time, or breaks things.
The prevention system only works if we maintain the prevention system.

## What Is VibePilot
Sovereign AI execution engine. Transforms PRDs -> production code via multi-agent orchestration.
Runtime: Go binary (governor). Event-driven via Supabase.

## Codebase Structure (auto-discovered)
- governor/cmd/cleanup/ (1 files, 1 funcs, 0 types)
- governor/cmd/encrypt_secret/ (1 files, 1 funcs, 0 types)
- governor/cmd/governor/ (12 files, 94 funcs, 12 types)
- governor/cmd/migrate_vault/ (1 files, 5 funcs, 1 types)
- governor/internal/connectors/ (2 files, 22 funcs, 8 types)
- governor/internal/core/ (4 files, 35 funcs, 27 types)
- governor/internal/dag/ (3 files, 18 funcs, 13 types)
- governor/internal/db/ (3 files, 27 funcs, 7 types)
- governor/internal/gitree/ (2 files, 27 funcs, 5 types)
- governor/internal/maintenance/ (3 files, 31 funcs, 7 types)
- governor/internal/mcp/ (3 files, 23 funcs, 4 types)
- governor/internal/memory/ (2 files, 19 funcs, 5 types)
- governor/internal/realtime/ (1 files, 23 funcs, 8 types)
- governor/internal/runtime/ (10 files, 164 funcs, 92 types)
- governor/internal/security/ (1 files, 3 funcs, 3 types)
- governor/internal/tools/ (7 files, 50 funcs, 22 types)
- governor/internal/vault/ (1 files, 15 funcs, 4 types)
- governor/internal/webhooks/ (2 files, 20 funcs, 7 types)
- governor/pkg/types/ (1 files, 0 funcs, 9 types)
## Config: JSON (auto-discovered)
  config/agents.json - Agent definitions with capability declarations. Roles separated: decide vs execute. Only Maintenance has git write.
  config/connectors.json - Destination configurations with native tool capabilities. CLI destinations provide tools, API destinations do other.
  config/destinations.json - WHERE tasks execute. CLI, Web platforms, API endpoints. All swappable.
  config/kilo-session.json - keys: max_sessions, max_concurrent_tasks_per_session, notes, memory_per_session_mb, reason
  config/maintenance_commands.json - Maintenance command configuration. Defines allowed git operations and validation rules.
  config/models.json - Model profiles with rate limits, costs, and recovery config. Source of truth synced to Supabase.
  config/plan_lifecycle.json - Plan lifecycle configuration - states, transitions, revision rules, complexity detection, consensus rules. All configurable.
  config/platforms.json - Web platforms and API models for VibePilot routing. Updated April 8, 2026 with verified OpenRouter data.
  config/roles.json - WHAT job is being done. Roles are job definitions. Model and destination assigned by orchestrator at runtime.
  config/routing_contract.json - Routing decision contract - what orchestrator returns when routing a task
  config/routing.json - Routing strategy. How VibePilot decides WHERE to send tasks. The free_cascade strategy uses UsageTracker to pick only currently-active providers.
  config/skills.json - All available skills in VibePilot. Add/remove skills here. Agents reference these by ID.
  config/system.json - System configuration - database, vault, git, runtime settings. All swappable.
  config/tools.json - Tool definitions for VibePilot runtime. Parameters + security + implementation.
## Config: Prompt Templates (auto-discovered)
  config/prompts/consultant.md - Consultant Agent
  config/prompts/council.md - Council Agent
  config/prompts/courier.md - Courier Agent
  config/prompts/internal_api.md - Internal API Agent
  config/prompts/internal_cli.md - Internal CLI Agent
  config/prompts/maintenance.md - Maintenance Agent
  config/prompts/orchestrator.md - Orchestrator Agent
  config/prompts/planner.md - Planner Agent
  config/prompts/researcher.md - Researcher Agent
  config/prompts/supervisor.md - Supervisor Agent
  config/prompts/tester_code.md - Code Tester Agent
  config/prompts/vibes.md - Vibes Agent

## Service Info
- Service: vibepilot-governor (systemd --user)
- Logs: journalctl --user -u vibepilot-governor
- Branch: main
- Commit: d109fb1d

## How To Use .context/
1. boot.md (this file) = orientation + Tier 0 rules (~2K tokens)
2. map.md = all function signatures, compressed (~12K tokens)
3. index.db = jCodeMunch SQLite: code symbols, imports, call graph
   sqlite3 .context/index.db ".tables"  (see what's indexed)
4. knowledge.db = THE key file. Unified SQLite with:
   - rules table: 36 rules, priority-ranked (critical/high/medium), deduplicated
   - prompts table: 30 prompt templates across all directories
   - configs table: 15 config files with purpose + key fields
   - docs table: ~3000 documentation sections, full-text searchable
   Quick queries:
   sqlite3 .context/knowledge.db "SELECT title FROM rules WHERE priority='critical'"
   sqlite3 .context/knowledge.db "SELECT name,role FROM prompts WHERE name LIKE '%supervisor%'"
   sqlite3 .context/knowledge.db "SELECT title,file_path FROM docs WHERE title LIKE '%vault%'"
5. Raw source = for implementation details only

## Current Status (from CURRENT_STATE.md)
# VibePilot Current State - 2026-04-15 (night)
## Status: Fully operational. Schema deployed, vault stocked, connectors wired, worktrees live.
### The Repo Situation
Two copies on disk, both synced to main:
| Location | Purpose | State |
|---|---|---|
| `~/vibepilot/` | RUNNING copy. Compiled binary + systemd service. | Current (main). Binary rebuilt Apr 15 20:19. |
| `~/VibePilot/` | DEVELOPMENT copy. Primary working directory. | Current (main). |
**GitHub main is current** -- 12 commits pushed April 15 evening session.
---
### What's Running
- **Governor:** systemd user service, active (running since Apr 15 20:19)
  - Binary: `~/vibepilot/governor/governor` (compiled Apr 15, includes worktree wiring + MCP + memory)
  - Service: `systemctl --user status vibepilot-governor`
  - Logs: `journalctl --user -u vibepilot-governor -f`
  - MCP servers: jcodemunch (52 tools) + jdocmunch (15 tools) = 67 tools connected
  - Governor MCP server: disabled in config (ready to enable for SSE port 8081)
  - **Worktrees: ENABLED** -- base path `/home/vibes/VibePilot-work/`, auto-cleanup on shutdown
  - **Connectors registered:** claude-code (cli), gemini-api (api), groq-api (api), nvidia-api (api)
- **Cloudflared tunnel:** live at vibestribe.rocks, sacred (don't touch)
- **Hermes agent:** accessible via dashboard chat through tunnel
- **Chrome CDP:** port 9222, bind mount active, user auto-logged into Gmail/Gemini/Sheets
- **TTS:** edge-tts (fast, free)
### Connectors (4 API + 7 web couriers)
| ID | Type | Status | Key Vault |
|---|---|---|---|
| claude-code | cli | active | none (local) |
| gemini-api | api | active | GEMINI_API_KEY |
| groq-api | api | active | GROQ_API_KEY |
| nvidia-api | api | active | NVIDIA_API_KEY |
