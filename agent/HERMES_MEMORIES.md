# Hermes Agent Memories
# Auto-backed up from Hermes memory store. Do not edit manually.
# Last updated: 2026-04-15

## MEMORY

VP: Gov v2.0.0, main branch only. TWO copies: ~/VibePilot/ (dev) + ~/vibepilot/ (running). x220=server (wifi dead, USB tethered). 15,000+ lines Go, 60 files, 16 pkgs. WYNTK updated Apr 15. MCP LIVE: jcodemunch(52 tools)+jdocmunch(15 tools)=67 tools in governor. jdatamunch failed(transport error), disabled. MCP Server Phase 2 DONE: governor_server.go exposes tools via stdio/SSE. 3-Layer Memory DONE: service.go + migration 110 applied. Context Compaction DONE: compactor.go auto-summarizes sessions. Git Worktrees DONE: worktree.go for parallel agent isolation. tier0 rules: rule4(explicit migration workflow+GitHub link), rule6(no shortcuts). tier0 rules now 10 items incl post-task discipline.

Chrome CDP port 9222. BROWSER-FIRST for email/Docs/web AI. Google blocks fresh sign-in on CDP Chrome. Manual first-login needed in chrome-debug window, cookies persist after. Same pattern for courier agents -- manual first-login each platform, then Browser Use takes over.

TTS: edge-tts only (fast, free). Kokoro removed (too slow). Cache: ~/.hermes/audio_cache/

OLLAMA v0.20.4, AVX-only(no AVX2). llama3.2:1b(1.3GB,no vision,no thinking) WORKS at ~6tok/s via /v1 endpoint. Enabled on boot. qwen3.5:0.8b FAILS via /v1 (thinking eats all tokens, only works with native API think:false). Hermes fallback updated to llama3.2:1b. No vision locally -- use cloud APIs for vision.

EMAIL: himalaya reads OK (app password "vibes"), use Python smtplib fallback for sending (himalaya raw send crashes on MIME parsing). Gmail browser needs manual login once, cookies to chrome-debug profile.

CONTEXT LAYER: lean-ctx (map.md) + jCodeMunch (index.db) + build-knowledge-db.py (knowledge.db). MCP: jcodemunch 52 + jdocmunch 15 = 67 tools live. tier0 = rules source of truth. Post-task: update CURRENT_STATE+WYNTK+TODO.

§MEMORY-BACKUP: Hermes memories are backed up to GitHub (VibePilot repo, agent/HERMES_MEMORIES.md). Update that file whenever memories change.

## USER PROFILE

PET PEEVE: Stop overcomplicating. SIMPLE DIRECT ACTION. Don't spawn extra models/processes when I can just DO it myself. Never dismiss local path fixes or config tweaks as "minor" -- if they break on another machine, they're not minor.

User vision: n8n-like visual config-driven orchestration (draw pipelines, not code them). Courier agents to free web AI tiers via Browser Use (self-hosted, open source). Chat URLs stored in task_runs for revision context. Visual QA agent checks apps before human review. MIT/Apache only. No Apple. Conservative subagent usage with GLM. May 1 = budget cliff.

WORKFLOW: User researches on personal laptop (emails, raindrop.io, Gemini chats, newsletters). Forwards links/articles to vibepilot email. Phone secondary. Dashboard primary, Telegram backup. No .env files, keys in Supabase vault.
AGENT SWAP VELOCITY: Uses Codex, OpenCode, Kimi, Kilo, Claude, Hermes -- swaps at lightspeed. Everything MUST be agent-independent. No Hermes-only dependencies. Knowledge must survive agent swap. Prefers building own tools over external deps. No multi-MCP chains.
BURNING CONSTRAINT: 20-46K tokens to boot up on repo files out of 70K context. Need compressed knowledge layer (queryable index, not raw files). Think before build -- analyze carefully first.
