# VibePilot TODO - 2026-04-22 (updated)

## Critical (do next)

### 1. Fix webhook PRD path matching bug
isPRD() in governor/internal/webhooks/github.go matches `docs/prd/pending/` subfolder.
Should only match `docs/prd/*.md` directly (not nested paths).
Add `/pending/` to the exclusion list alongside `/processed/`.
This caused the knowledge graph spec (intentionally parked in pending) to trigger the planner.

### 2. Test the pipeline end-to-end with a SIMPLE task
NOT a full knowledge graph feature. Pick something small:
- A single new RPC function
- A dashboard text fix
- A config field addition
Run the full pipeline: PRD → planner → supervisor review → task creation → execution → task review → completion
This is THE proof point. Nothing else matters until this works once cleanly.

### 3. Test full fallback chain before May 1
ZAI/GLM subscription ends May 1. Before then:
- Force each fallback tier to activate (disable tiers above it temporarily)
- Verify Groq, NVIDIA NIM, OpenRouter all actually work through the governor router
- Time each tier's response to confirm latency is acceptable
- Document any that fail

### 4. Add governor systemd service
Governor currently running as manual process (PID 19288). If x220 reboots, governor is gone.
Need a systemd user service that auto-starts it.
Also: consider syncing runtime repo (~/vibepilot) with dev repo (~/VibePilot) — currently 2 separate clones.

---

## High Priority

### 5. Visual QA agent
- Wire browser-use to capture screenshots of dashboard after changes
- Compare to baseline, flag visual regressions
- Present to human for UI/UX yes/no
- This is the courier pattern applied to our own app

### 6. Daily landscape research cron
The researcher prompt exists (`prompts/daily_landscape_researcher.md`).
Need to wire it:
- Cron job that runs researcher daily
- Checks GitHub trending, HuggingFace new models, provider changelogs
- Posts findings to Supabase for Supervisor review
- Supervisor approves minor, escalates major to Council -> Human
- Approved findings now auto-sync to config + DB via ResearchActionApplier

### 7. Pre-execution design preview
For UI tasks, show human a design choice BEFORE writing code:
- Courier generates mockup/plan, presents to human
- Human picks direction, THEN agent writes code
- Skip for non-UI tasks (conditional pipeline stage)

### 8. Dashboard model management
Dashboard shows models but can't add/edit them. Need:
- "Add model" form → calls ResearchActionApplier via API
- "Edit model" → same path
- Dashboard becomes self-service, no Hermes required
- Any channel (dashboard, telegram, curl) hits same guaranteed-sync path

---

## Medium Priority

### 9. LogAct patterns (from Meta research)
`research/2026-04-14-logact-agent-bus.md` -- maps directly to our architecture.
Adopt after pipeline is working end-to-end:
- Intent logging: record what agent PLANS to do before execution
- Safety voter: use a different cheap model to cross-check intent before execution
- Append-only task_events table in Supabase (currently we update rows in-place)
- Stupidity diagnosis: agent reads own failed output from log, rewrites

### 10. JourneyKits implementation
95 kits scanned, 20 mapped to VibePilot gaps (`research/2026-04-08-journeykits-landscape-analysis.md`).
Need to go through them and decide which patterns to adopt.

### 11. Make .context/ hooks async
Post-checkout, post-merge, pre-commit hooks rebuild entire knowledge layer synchronously.
On x220 this causes command timeouts. Fix:
- Run rebuild in background (don't block git)
- Or skip if source files haven't changed since last build
- Or both

---

## Lower Priority

### 12. Enable governor MCP server
The governor can expose its tools as an MCP server (SSE on port 8081).
Currently disabled. Enable when there's a consumer for it.

### 13. Ethernet + headless setup
x220 currently tethered via phone WiFi. More stable:
- USB ethernet adapter for wired connection
- Configure headless boot (no display manager)
- Auto-start governor + cloudflared on boot

---

## Pending Specs (parked, not scheduled)

- `docs/pending/vibepilot-knowledge-graph-spec.md` — Full knowledge graph with PocketBase, dashboard viz, council review, research agent, bookmarklet. Complex. Do NOT schedule until simple pipeline test passes.

---

## Done (April 2026)

- [x] Governor intelligence overhaul (commit 57654556) -- context, routing, verification
- [x] Config-driven agent context policy (full_map/file_tree/targeted/none)
- [x] Task packet enrichment chain (planner → parser → storage → file injection)
- [x] Supervisor objective verification (file refs, PRD traceability, dep cycles)
- [x] Code map TTL cache + startup refresh via jcodemunch MCP
- [x] Plan review race condition fixed (retry loop + stale lock cleanup)
- [x] Supervisor routing flag feedback (exec_failed_by prefix + REST PATCH)
- [x] Hermes vibepilot-thinking skill (architectural reasoning framework)
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
- [x] Models config expanded to 57 models (correct rate limits, full connector coverage)
- [x] Connectors expanded to 26 (4 CLI, 7 API, 15 web)
- [x] NVIDIA NIM connector added (integrate.api.nvidia.com, OpenAI-compatible)
- [x] Groq connector fixed (active, all models, key in vault)
- [x] Gemini API connector fixed (active, 4 specialized keys, all in vault)
- [x] Deepseek API benched (out of credits, NVIDIA NIM deepseek-r1 as fallback)
- [x] Chrome CDP working (port 9222, auto-login Gmail/Gemini/Sheets)
- [x] Gmail app password updated in vault
- [x] Tier0 rules expanded (migration workflow, no shortcuts, post-task discipline)
- [x] Self-learning feedback loops wired across all 6 handlers
- [x] Supervisor model tracking in plan review, task review, research review
- [x] All legacy SelectDestination calls removed from handlers (SelectRouting only)
- [x] Courier system fully built (courier.go, courier_run.py, courier.yml, Supabase realtime)
- [x] Research→config+DB sync (ResearchActionApplier, deterministic, no LLM middleman)
- [x] 15 new web platforms added and wired (deepseek, qwen, mistral, notegpt, kimi, huggingchat, aistudio, poe, chatbox, aizolo, perplexity, chatgpt, claude, gemini-2.5-pro, gemini-3.1-pro-preview)
- [x] Consultant agent prompt and PRD template built (539 lines, tested with Knowledge Graph spec)
- [x] Config/DB sync verified — research pipeline keeps them in sync going forward

## Not Viable (abandoned)

- [x] Ollama + local models -- x220 too slow (6 tok/s max, AVX-only)
- [x] Kokoro TTS -- too slow on x220
- [x] SiliconFlow, SambaNova, Together AI -- paid only

---

**Last Updated:** 2026-04-22
