# VibePilot Bootstrap
# Generated: 2026-04-14T21:07:54Z | Commit: ce326fe5
# Read THIS FIRST. ~2K tokens. Everything else is lazy-loaded from map.md or index.db

## What Is VibePilot
Sovereign AI execution engine on ThinkPad X220 (i5-2520M, 16GB RAM, no AVX2, no GPU).
Transforms PRDs -> production code via multi-agent orchestration.
Target app: Webs of Wisdom (global multilingual social media platform).
Runtime: Single Go binary (governor). Event-driven via Supabase state transitions + realtime.

## Stack
- Language: Go (governor binary)
- Database: Supabase (Postgres + RPC + Realtime + Vault)
- Config: JSON/YAML in governor/config/
- Agent connectors: CLI runners (codex, opencode) + API runners
- Webhooks: GitHub + Supabase Realtime for event triggers
- Tunnel: Cloudflared at vibestribe.rocks (DO NOT TOUCH)
- TTS: edge-tts only
- Frontend: VibeDashboard (Supabase + chat panel)

## Architecture (packages)
- cmd/governor/ - entry point, event handlers, adapters
- internal/core/ - state machine (task lifecycle), checkpoint, analyst
- internal/runtime/ - session factory, agent pool, context builder, router, tool registry
- internal/connectors/ - CLI and API agent runners
- internal/dag/ - DAG pipeline engine (YAML-defined workflows)
- internal/db/ - Supabase client, RPC calls, state queries
- internal/gitree/ - git operations (branch, commit, PR)
- internal/vault/ - secrets via Supabase vault
- internal/webhooks/ - GitHub webhook server
- internal/realtime/ - Supabase Realtime subscription client
- internal/mcp/ - external MCP server registry + tool bridge
- internal/security/ - secret leak detection
- internal/tools/ - tool implementations (db, file, git, vault, web, sandbox)
- pkg/types/ - shared type definitions

## Constraints
- NO local LLM inference (too slow on x220). Cloud free tiers only.
- NO hardcoded values. Everything in config/ JSON files.
- NO .env files. Secrets in Supabase vault (get_vault_secret RPC).
- RAM is for agent sessions, not model inference.
- Agent swap velocity: works with Hermes/Claude/Codex/OpenCode/Kimi/Kilo. Must be agent-agnostic.
- Branch: research-update-april2026
- Service: vibepilot-governor (systemd --user)
- Logs: journalctl --user -u vibepilot-governor

## How To Use This .context/ Directory
1. Read boot.md (this file) for orientation (~2K tokens)
2. Read map.md for code structure (~12K tokens, all function signatures)
3. Query index.db with sqlite3 for targeted searches:
   sqlite3 .context/index.db "SELECT * FROM symbols WHERE name LIKE '%vault%'"
4. Read raw source files only when you need implementation details

## Event Flow
GitHub webhook / Dashboard action -> Supabase DB insert -> Realtime event ->
EventRouter -> Handler (task/plan/council/research/maint) -> SessionFactory ->
Agent connector (CLI/API) -> Result -> DB update -> Next state

## Current Status
See CURRENT_STATE.md for full details. Summary:
- Governor running as systemd user service
- Free model cascade: Google AI Studio -> Groq -> SambaNova -> OpenRouter
- .context/ knowledge layer: THIS DIRECTORY (new!)
