# VibePilot TODO - 2026-04-21 (updated)

## Critical (do next)

### 1. Test the pipeline end-to-end
THE blocker for everything else. The YAML pipeline is written but never tested against real governor:
- Queue a real task through Supabase
- Watch it flow through: plan -> supervisor -> execute -> review -> test -> merge
- Verify gitree branch management works with parallel tasks
- Fix whatever breaks (will be things)

### 2. Test full fallback chain before May 1
ZAI/GLM subscription ends May 1. Before then:
- Force each fallback tier to activate (disable tiers above it temporarily)
- Verify Groq, NVIDIA NIM, OpenRouter all actually work through the governor router
- Time each tier's response to confirm latency is acceptable
- Document any that fail

### 3. Reconcile config/models.json with Supabase DB
Config has 30 models, DB has 58. The ResearchActionApplier keeps them in sync going forward,
but existing orphans need cleaning:
- Verify which DB models are real vs stale
- Add missing models to config OR remove orphans from DB
- Then sync should be 1:1

---

## High Priority

### 4. Visual QA agent
- Wire browser-use to capture screenshots of dashboard after changes
- Compare to baseline, flag visual regressions
- Present to human for UI/UX yes/no
- This is the courier pattern applied to our own app

### 5. Daily landscape research cron
The researcher prompt exists (`prompts/daily_landscape_researcher.md`).
Need to wire it:
- Cron job that runs researcher daily
- Checks GitHub trending, HuggingFace new models, provider changelogs
- Posts findings to Supabase for Supervisor review
- Supervisor approves minor, escalates major to Council -> Human
- Approved findings now auto-sync to config + DB via ResearchActionApplier

### 6. Pre-execution design preview
For UI tasks, show human a design choice BEFORE writing code:
- Courier generates mockup/plan, presents to human
- Human picks direction, THEN agent writes code
- Skip for non-UI tasks (conditional pipeline stage)

### 7. Dashboard model management
Dashboard shows models but can't add/edit them. Need:
- "Add model" form → calls ResearchActionApplier via API
- "Edit model" → same path
- Dashboard becomes self-service, no Hermes required
- Any channel (dashboard, telegram, curl) hits same guaranteed-sync path

---

## Medium Priority

### 8. LogAct patterns (from Meta research)
`research/2026-04-14-logact-agent-bus.md` -- maps directly to our architecture.
Adopt after pipeline is working end-to-end:
- Intent logging: record what agent PLANS to do before execution
- Safety voter: use a different cheap model to cross-check intent before execution
- Append-only task_events table in Supabase (currently we update rows in-place)
- Stupidity diagnosis: agent reads own failed output from log, rewrites

### 9. JourneyKits implementation
95 kits scanned, 20 mapped to VibePilot gaps (`research/2026-04-08-journeykits-landscape-analysis.md`).
Need to go through them and decide which patterns to adopt.

### 10. Make .context/ hooks async
Post-checkout, post-merge, pre-commit hooks rebuild entire knowledge layer synchronously.
On x220 this causes command timeouts. Fix:
- Run rebuild in background (don't block git)
- Or skip if source files haven't changed since last build
- Or both

---

## Lower Priority

### 11. Enable governor MCP server
The governor can expose its tools as an MCP server (SSE on port 8081).
Currently disabled. Enable when there's a consumer for it.

### 12. Ethernet + headless setup
x220 currently tethered via phone WiFi. More stable:
- USB ethernet adapter for wired connection
- Configure headless boot (no display manager)
- Auto-start governor + cloudflared on boot

---

## Done (April 2026)

- [x] .context/ knowledge layer built (knowledge.db, boot.md, tier0)
- [x] Tier 0 hand-crafted rules (single source of truth for all docs)
- [x] All doc contradictions fixed (9 across 5 files)
- [x] YAML pipeline config (code-pipeline.yaml)
- [x] Gitree branch management (parallel agent git isolation)
- [x] Supabase schema (111 migrations, all deployed and verified)
- [x] Free model research (4 providers verified and working)
- [x] Free API keys obtained (Google, Groq, NVIDIA NIM, OpenRouter)
- [x] NVIDIA NIM tested and wired into Hermes fallback chain
- [x] JourneyKits landscape analysis (95 kits, 20 mapped)
- [x] GitHub PAT rotated (Apr 15)
- [x] 25 stale files cleaned from repo root
- [x] research-update-april2026 merged into main
- [x] Both disk copies synced on main
- [x] Scripts made portable (no hardcoded usernames)
- [x] Dead branches deleted locally and remotely
- [x] Rate limit research preserved as templates
- [x] LogAct + orchestration research analyzed and saved
- [x] Cloudflared tunnel live (vibestribe.rocks, sacred)
- [x] TTS working (edge-tts, fast, free)
- [x] Governor binary rebuilt with worktree wiring (Apr 15)
- [x] MCP Client -- jcodemunch (52) + jdocmunch (15) = 67 tools
- [x] MCP Server Phase 2 -- governor exposes tools via stdio/SSE
- [x] 3-Layer Memory System -- short/mid/long-term in Supabase
- [x] Migration 110 applied (memory tables)
- [x] Migration 111 applied (all RPCs, idempotent, verified live)
- [x] Context Compaction -- compactor.go auto-summarizes sessions
- [x] Worktrees WIRED into governor (handlers, shutdown cleanup, shadow merge, bootstrap)
- [x] Worktree strategic patterns (Gemini: shadow merge, env injection, branch naming)
- [x] Secrets vault fully stocked (10 keys, all encrypted, all fresh Apr 15)
- [x] Models config updated (16 models, correct rate limits, 3 NVIDIA NIM added)
- [x] Connectors wired (4 API active: gemini, groq, nvidia, claude-code + 7 web couriers)
- [x] NVIDIA NIM connector added (integrate.api.nvidia.com, OpenAI-compatible)
- [x] Groq connector fixed (active, all 4 models, key in vault)
- [x] Gemini API connector fixed (active, updated models, key in vault)
- [x] Deepseek API benched (out of credits, NVIDIA NIM deepseek-r1 as fallback)
- [x] Chrome CDP working (port 9222, auto-login Gmail/Gemini/Sheets)
- [x] Gmail app password updated in vault
- [x] Tier0 rules expanded (migration workflow, no shortcuts, post-task discipline)
- [x] Self-learning feedback loops wired across all 6 handlers (plan, council, task, testing, research, maint)
- [x] Supervisor model tracking in plan review, task review, research review
- [x] All legacy SelectDestination calls removed from handlers (SelectRouting only)
- [x] Courier system fully built (courier.go, courier_run.py, courier.yml, Supabase realtime)
- [x] Research→config+DB sync (ResearchActionApplier, deterministic, no LLM middleman)
- [x] 4 new web platforms added (kimi-ai, perplexity, poe, aizolo)
- [x] WYNTK and TODO updated to match verified system state (Apr 21)

## Not Viable (abandoned)

- [x] Ollama + local models -- x220 too slow (6 tok/s max, AVX-only)
- [x] Kokoro TTS -- too slow on x220
- [x] SiliconFlow, SambaNova, Together AI -- paid only

---

**Last Updated:** 2026-04-21
