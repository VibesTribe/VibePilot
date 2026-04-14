# VibePilot Bootstrap
# Generated: 2026-04-14T22:39:49Z | Commit: 00ce9b7f | Branch: research-update-april2026
# AUTO-GENERATED. DO NOT EDIT. Run .context/build.sh to regenerate.
# Recovery: clone repo, bash .context/tools/install.sh, bash .context/build.sh

## What Is VibePilot
Sovereign AI execution engine. Transforms PRDs -> production code via multi-agent orchestration.
Runtime: Go binary (governor). Event-driven via Supabase.

## Codebase Structure (auto-discovered)
- governor/cmd/cleanup/ (1 files, 1 funcs, 0 types)
- governor/cmd/encrypt_secret/ (1 files, 1 funcs, 0 types)
- governor/cmd/governor/ (12 files, 93 funcs, 12 types)
- governor/cmd/migrate_vault/ (1 files, 5 funcs, 1 types)
- governor/internal/connectors/ (2 files, 22 funcs, 8 types)
- governor/internal/core/ (4 files, 35 funcs, 27 types)
- governor/internal/dag/ (3 files, 18 funcs, 13 types)
- governor/internal/db/ (3 files, 27 funcs, 7 types)
- governor/internal/gitree/ (1 files, 14 funcs, 2 types)
- governor/internal/maintenance/ (3 files, 31 funcs, 7 types)
- governor/internal/mcp/ (2 files, 13 funcs, 3 types)
- governor/internal/realtime/ (1 files, 23 funcs, 8 types)
- governor/internal/runtime/ (10 files, 160 funcs, 89 types)
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
  config/models.json - WHO provides intelligence. Actual LLM models with their capabilities and access points.
  config/plan_lifecycle.json - Plan lifecycle configuration - states, transitions, revision rules, complexity detection, consensus rules. All configurable.
  config/platforms.json - Web platforms and API models for VibePilot routing. Updated April 8, 2026 with verified OpenRouter data.
  config/roles.json - WHAT job is being done. Roles are job definitions. Model and destination assigned by orchestrator at runtime.
  config/routing_contract.json - Routing decision contract - what orchestrator returns when routing a task
  config/routing.json - Routing strategy configuration. How VibePilot decides WHERE to send tasks. Fully configurable per user preference.
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
## Constraints (auto-extracted)
## TIER 0: NON-NEGOTIABLE RULES (READ FIRST, NO EXCEPTIONS)

### Absolute Rules
These apply to EVERY task, EVERY agent, EVERY session:
- **NEVER Hardcode Anything**: Everything is configurable via JSON files in governor/config/:
- Models → models.json
- Connectors → connectors.json
- Agents → agents.json
- Routing 
- **NO Type 1 Errors**: Fundamental design mistakes ruin everything downstream. Think ahead. Design for change.
- **The Dashboard is SACRED**: VibePilot was designed from the dashboard backwards. If the dashboard isn't showing what it should, the problem is in the Go code, NOT the dashboard.
- **CRITICAL: Dashboard is READ-ONLY**: The dashboard is a **view** of VibePilot state. It does NOT:
- Make decisions
- Route tasks
- Execute code
- **CRITICAL: No Webhooks**: **VibePilot uses Supabase Live (realtime subscriptions), NOT webhooks.**
- **Never apply migrations directly**: Always go through GitHub first.

### Design Philosophy (violating these = rewrite)
- **Zero Vendor Lock-In**: - No dependency on any single provider
- Model, platform, database, hosting - all swappable
- Switching costs = near zero
- **Exit Ready**: - Pack up and hand over to anyone - new host, new owner, anyone
- Export everything, import anywhere
- VibePilot could shut down tomorrow and projects
- **If It Can't Be Undone, It Can't Be Done**: - Every change is reversible
- Rollback plan before implementation
- No one-way doors
- **THE CORE RULE**: NOT: Hardcoded → Specific Vendor → Broken when vendor changes

### Before You Code (mandatory checklist)
- [ ] Read relevant docs in knowledge.db FIRST, not after
- [ ] Search knowledge.db for existing rules/patterns before inventing new ones
- [ ] No hardcoding - use config JSON in governor/config/
- [ ] Credentials: vault ONLY (sqlite3 .context/knowledge.db "SELECT content FROM rules WHERE title LIKE '%Supabase%'")
- [ ] No .env files, no sudo systemctl, no journalctl -u governor
- [ ] If dashboard looks wrong, fix the Go code, not the dashboard

### Time Wasters (don't do these)
- Looking for .env files (don't exist)
- Using sudo systemctl (it's a user service: systemctl --user)
- Wrong log command (use: journalctl --user -u vibepilot-governor)
- Guessing instead of checking what exists first
- Touching dashboard code (problem is in Go, not dashboard)
- Using webhooks (we use Supabase Live realtime)

### Human Role
- Human reviews code, merges code, maintains the system
- Human does NOT debug agent code or write agent code directly
- Respect human time: batch questions, not one-at-a-time

### Going Deeper
When you need more than Tier 0:
- Architecture rules: sqlite3 .context/knowledge.db "SELECT title,content FROM rules WHERE priority='high'"
- All prompts: sqlite3 .context/knowledge.db "SELECT name,role FROM prompts"
- Search docs: sqlite3 .context/knowledge.db "SELECT title,file_path FROM docs WHERE title LIKE '%<topic>%'"
- Full code map: cat .context/map.md

## Service Info
- Service: vibepilot-governor (systemd --user)
- Logs: journalctl --user -u vibepilot-governor
- Branch: research-update-april2026
- Commit: 00ce9b7f

## How To Use .context/
1. boot.md (this file) = orientation (~1.5K tokens)
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
# VibePilot Current State - 2026-04-14
## Status: Infrastructure Optimized, Research Phase
### What's Running
- **Governator:** systemd user service, running since April 7, active
- **Cloudflared tunnel:** live at vibestribe.rocks, sacred (don't touch)
- **Hermes agent:** accessible via dashboard chat through tunnel
- **Chrome CDP:** port 9222 for browser automation
- **TTS:** edge-tts (fast, free, no changes needed)
### Hardware: ThinkPad X220
- Intel i5-2520M (no AVX2, no GPU)
- 16GB RAM (~10GB available)
- 781GB disk free
- Phone WiFi tethered
### What Changed This Session (April 14)
- **Ollama:** installed v0.20.4, daemon stopped/disabled. Tested qwen3:4b and qwen3-vl:4b -- too slow (2 tok/s) for real work. Cleaned out. Ready to pull models when landscape shifts.
- **Kokoro TTS:** removed (9GB freed). Edge-tts is better for this hardware.
- **Free model research:** verified 7 free API providers. Full rolodex in `research/2026-04-14-free-model-rolodex.md`.
- **GitHub PAT:** rotated (done in earlier session).
### Key Decisions
1. **No local models** -- x220 can't run useful inference. Cloud free tiers are the path.
2. **Edge-tts only** -- fastest free option, no reason to change.
3. **RAM for agents, not models** -- parallel agent sessions are the priority.
4. **Multiple free providers** -- cascade of Groq/Google/OpenRouter/SambaNova, never single-vendor dependency.
5. **Real usage decides spending** -- run tasks on free tiers first, data tells where $10 credit is worth it.
---
## Verified Free API Providers (April 2026)
| Provider | Card Needed | Best Free Models | Rate Limits |
|---|---|---|---|
| OpenRouter | NO | 24 free models, $0 cap | 50-1000 RPD |
| Groq | NO | qwen3-32b, llama-4-scout, gpt-oss | 30 RPM, 100-500K TPD |
