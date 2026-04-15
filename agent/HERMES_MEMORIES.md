# Hermes Agent Memories
# Auto-backed up from Hermes memory store. Do not edit manually.
# Last updated: 2026-04-15 (evening)

## MEMORY

VP: Gov v2.0.0, main branch only. TWO copies: ~/VibePilot/ (dev) + ~/vibepilot/ (running). x220=server (wifi dead, USB tethered). 15,000+ lines Go, 60 files, 16 pkgs. WYNTK updated Apr 15. MCP LIVE: jcodemunch(52 tools)+jdocmunch(15 tools)=67 tools in governor. jdatamunch failed(transport error), disabled. MCP Server Phase 2 DONE. 3-Layer Memory DONE + migration 110. Context Compaction DONE. Git Worktrees DONE. tier0 rules 10 items incl post-task discipline.

CHROME CDP: Port 9222. Wrapper /usr/bin/google-chrome-stable has --remote-debugging-port=9222 + --user-data-dir=$HOME/.config/chrome-debug. Bind mount active. User auto-logged into Gmail/Gemini/Sheets. browser_navigate=Playwright HEADLESS (not real Chrome). For logged-in sites use Python Playwright connect_over_cdp + browser.contexts[0]. No --disable-blink-features (causes warning). Hermes restart needed to pick up BROWSER_CDP_URL from .env.

FREE API KEYS (all verified): Google AI Studio (primary) + Groq (fallback 1-2) + NVIDIA NIM 131 models (fallback 3-4, includes DeepSeek-v3.2/Llama-4/Qwen3-coder) + OpenRouter free-only (fallback 5-7). ZAI/GLM ends May 1. NOT FREE: SiliconFlow, SambaNova, Together AI. Governor config files (models.json/connectors.json/routing.json) still stale -- need updating with real free providers.

TTS: edge-tts only (fast, free). Kokoro removed (too slow). Cache: ~/.hermes/audio_cache/

OLLAMA: ABANDONED. x220 (AVX-only, no AVX2) max 6tok/s. Service disabled, no models. Cloud-only strategy.

EMAIL: himalaya reads OK (app password "vibes"), use Python smtplib fallback for sending (himalaya raw send crashes on MIME parsing). Gmail browser needs manual login once, cookies to chrome-debug profile.

CONTEXT LAYER: lean-ctx (map.md) + jCodeMunch (index.db) + build-knowledge-db.py (knowledge.db). MCP: jcodemunch 52 + jdocmunch 15 = 67 tools live. tier0 = rules source of truth. Post-task: update CURRENT_STATE+WYNTK+TODO.

§MEMORY-BACKUP: Hermes memories are backed up to GitHub (VibePilot repo, agent/HERMES_MEMORIES.md). Update that file whenever memories change.

## USER PROFILE

PET PEEVE: Stop overcomplicating. SIMPLE DIRECT ACTION. Don't spawn extra models/processes when I can just DO it myself. Never dismiss local path fixes or config tweaks as "minor" -- if they break on another machine, they're not minor.

User vision: n8n-like visual config-driven orchestration (draw pipelines, not code them). Courier agents to free web AI tiers via Browser Use (self-hosted, open source). Chat URLs stored in task_runs for revision context. Visual QA agent checks apps before human review. MIT/Apache only. No Apple. Conservative subagent usage with GLM. May 1 = budget cliff.

WORKFLOW: User researches on personal laptop (emails, raindrop.io, Gemini chats, newsletters). Forwards links/articles to vibepilot email. Phone secondary. Dashboard primary, Telegram backup. No .env files, keys in Supabase vault.
AGENT SWAP VELOCITY: Uses Codex, OpenCode, Kimi, Kilo, Claude, Hermes -- swaps at lightspeed. Everything MUST be agent-independent. No Hermes-only dependencies. Knowledge must survive agent swap. Prefers building own tools over external deps. No multi-MCP chains.
BURNING CONSTRAINT: 20-46K tokens to boot up on repo files out of 70K context. Need compressed knowledge layer (queryable index, not raw files). Think before build -- analyze carefully first.
