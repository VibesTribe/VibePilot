# Hermes Agent Memories
# Auto-backed up from Hermes memory store. Do not edit manually.
# Last updated: 2026-04-15

## MEMORY

VP: Gov v2.0.0, main branch only. TWO copies: ~/VibePilot/ (dev) + ~/vibepilot/ (running). x220=server (wifi dead, USB tethered). 15,000+ lines Go, 60 files, 16 pkgs. WYNTK updated Apr 15. MCP LIVE: jcodemunch(52 tools)+jdocmunch(15 tools)=67 tools in governor. jdatamunch failed(transport error), disabled. MCP Server Phase 2 DONE: governor_server.go exposes tools via stdio/SSE. 3-Layer Memory DONE: service.go + migration 110 applied. Context Compaction DONE: compactor.go auto-summarizes sessions. Git Worktrees DONE: worktree.go for parallel agent isolation. tier0 rules: rule4(explicit migration workflow+GitHub link), rule6(no shortcuts). tier0 rules now 10 items incl post-task discipline.

CHROME CDP: port 9222, user-data-dir=~/.config/chrome-debug. BROWSER_CDP_URL env var.

TTS: edge-tts only (fast, free, no API key). Kokoro removed (9GB freed, too slow on x220). Audio cache: ~/.hermes/audio_cache/. VibesChatPanel SSE->Hermes API->edge-tts.

OLLAMA: installed v0.20.4, daemon stopped/disabled. Tested qwen3:4b+qwen3-vl:4b -- 2 tok/s on i5, too slow. Deleted both models. Cloud free tiers = primary path. Pull models only when viable. Disk 781GB free.

EMAIL: himalaya reads OK (app password "vibes"), use Python smtplib fallback for sending (himalaya raw send crashes on MIME parsing). Gmail browser needs manual login once, cookies to chrome-debug profile.

CONTEXT LAYER (.context/): Custom combined pipeline: lean-ctx (map.md) + jCodeMunch (index.db) + build-knowledge-db.py (knowledge.db, replaces jDocMunch). Full docs: docs/CONTEXT_KNOWLEDGE_LAYER.md. MCP ENABLED in governor: jcodemunch 52 tools + jdocmunch 15 tools = 67 live. tier0-static.md = rules source of truth. Post-task discipline rule in tier0: update CURRENT_STATE+WYNTK+TODO after significant work. USER: "prevention of gaps" -- systems must document themselves.

## USER PROFILE

PET PEEVE: Stop overcomplicating. SIMPLE DIRECT ACTION. Don't spawn extra models/processes when I can just DO it myself. Never dismiss local path fixes or config tweaks as "minor" -- if they break on another machine, they're not minor.

User vision: n8n-like visual config-driven orchestration (draw pipelines, not code them). Courier agents to free web AI tiers via Browser Use (self-hosted, open source). Chat URLs stored in task_runs for revision context. Visual QA agent checks apps before human review. MIT/Apache only. No Apple. Conservative subagent usage with GLM. May 1 = budget cliff.

WORKFLOW: User researches on personal laptop (emails, raindrop.io, Gemini chats, newsletters). Forwards links/articles to vibepilot email. Phone secondary. Dashboard primary, Telegram backup. No .env files, keys in Supabase vault.
AGENT SWAP VELOCITY: Uses Codex, OpenCode, Kimi, Kilo, Claude, Hermes -- swaps at lightspeed. Everything MUST be agent-independent. No Hermes-only dependencies. Knowledge must survive agent swap. Prefers building own tools over external deps. No multi-MCP chains.
BURNING CONSTRAINT: 20-46K tokens to boot up on repo files out of 70K context. Need compressed knowledge layer (queryable index, not raw files). Think before build -- analyze carefully first.
