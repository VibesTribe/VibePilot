# Hermes Agent Memories
**Last updated:** Session April 14-15, 2026
**Purpose:** Backup of Hermes agent persistent memories. If agent config is wiped, restore from this file.

---

## VibePilot Project State

- Governor v2.0.0 rebuilt Apr 14, running on main branch. ONE branch: main. No feature branches.
- TWO copies synced: ~/VibePilot/ (dev) + ~/vibepilot/ (running). Both on same commit.
- Scripts are portable -- work on x220 (user "vibes") and other laptop (user "mjlockboxsocial").
- x220 = dedicated VibePilot server. WiFi chip dead (killed during repaste). USB tethered via phone.
- Codebase: 14,368 lines Go, 56 files, 13 internal packages + cmd handlers. Modular, not spaghetti.
- TWO TODO lists exist: ~/VibePilot/TODO.md (VP-specific, 11 items) AND vibeflow/TODO.md (master list for both repos + Hermes + dashboard, 17 items). These drift apart -- sync periodically.
- WYNTK doc updated Apr 14 -- architecture tree, knowledge layer, governor structure all current.

## Hardware & Network

- ThinkPad X220, i5-2520M Sandy Bridge (no AVX2, no GPU), 16GB RAM (~10GB usable), 781GB disk free
- Phone USB tethered (WiFi chip died during Arctic paste replacement)
- Planning ethernet hardwire + lid-closed headless mode
- Local inference tested: 2 tok/s on i5, unusable. Cloud free tiers = primary path.

## Chrome CDP Setup

- Port 9222, user-data-dir=~/.config/chrome-debug (NOT google-chrome -- CDP refuses default profile)
- Google cookies must be in chrome-debug profile. After copying cookies + Local State from google-chrome to chrome-debug, Gmail auto-logs in.
- Playwright may lose connection after gateway restart -- need fresh connect_over_cdp().
- BROWSER_CDP_URL env var controls CDP override.

## TTS

- edge-tts only (fast, free, no API key). Kokoro removed (9GB freed, too slow on x220).
- Audio cache: ~/.hermes/audio_cache/
- VibesChatPanel: SSE -> Hermes API -> edge-tts

## Ollama

- Installed v0.20.4, daemon stopped/disabled.
- Tested qwen3:4b and qwen3-vl:4b -- 2 tok/s on i5, too slow. Deleted both models.
- Pull models only when viable (e.g., tiny models for offline fallback).

## Email

- himalaya reads OK (Gmail app password "vibes"). Use Python smtplib fallback for sending (himalaya raw send crashes on MIME parsing).
- Gmail browser needs manual login once, cookies to chrome-debug profile.

## Context Layer (.context/)

- tier0-static.md = SINGLE SOURCE OF TRUTH for principles/rules/roles. Hand-crafted.
- knowledge.db (2.3MB SQLite): 24 clean rules from tier0, 30 prompts, 15 configs, 2972 doc sections.
- boot.md leads with Tier 0 rules. ~2,804 tokens.
- build-knowledge-db.py parses tier0-static.md (not scattered docs).
- Pre-commit hook rebuilds .context/ on every commit.
- Daily 2am cron syncs.
- Fresh recovery: clone repo -> tools/install.sh -> build.sh.

## Credentials & Services

- Sudo password: L0g0n
- Gmail: vibesagentai@gmail.com / L0g0nvibesagent (needs app password for CLI)
- Supabase: project qtpdzsinvifkgpxyxlaz, login vibesagentai@gmail.com / L0g0nvibepilot
- Keys in ~/.governor_env (Supabase service key + publishable key). Use `source ~/.governor_env` to access.
- The governor systemd service also has these in its Environment override.
- Legacy JWT dead. RLS on.
- GitHub secrets hold vault keys.
- Dashboard: https://vibeflow-dashboard.vercel.app/ -- SACRED, never modify.
- Tunnel: vibestribe.rocks via cloudflared, systemd user service, don't touch.
- Hermes API: port 8642, Gemini 2.5 Flash primary, OpenRouter fallback.
- Hermes Studio: port 3002, enhanced mode.

## User Profile

- Calls me "Vibes."
- Non-technical user. Linux is new. Doesn't code, merge, review code, or do anything technical.
- Resourceful problem-solver (resurrected X220, repasted CPU, installed Linux Mint from scratch).
- Broke -- spent $100 on GCE in 2 months, cancelled. X220 tethered via phone WiFi.
- OpenRouter subscription ends May 1 -- model future uncertain (budget cliff).
- Core values: modular, agnostic, flexible, cost-conscious, free-tier leverage.
- Everything must be recoverable from GitHub + Supabase alone.
- Hates spaghetti code/garbage.
- Wants me as sovereign overseer, reachable via dashboard chat (text/audio).
- Prefers: voice interaction, visual/oriented, config-driven philosophy.
- Vision: n8n-like visual config-driven orchestration (draw pipelines, not code them).

## Key Decisions (Settled)

- Work on main only -- no feature branches. One branch, one truth.
- Cloud free tiers are the path. Local models abandoned (too slow).
- Edge-tts is the TTS solution. Kokoro abandoned.
- Cloudflared tunnel is sacred -- vibestribe.rocks is live.
- Dashboard is sacred -- fix Go code, not dashboard.
- Pre-exec design preview for UI tasks (visual QA before code, not just after).
- Config over code. No hardcoding.

## Blocked / Waiting on User

- API keys: Need Groq (console.groq.com), SambaNova (cloud.sambanova.ai), SiliconFlow (cloud.siliconflow.cn) -- user signing up.
- Rotate GitHub PAT (current token missing read:org scope).
