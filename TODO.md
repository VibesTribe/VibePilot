# VibePilot TODO - 2026-04-15 (evening update)

## Critical (do first)

### 1. Free API keys -- DONE
Keys obtained, verified, and wired into Hermes fallback chain:
- Google AI Studio (Gemini) -- WORKING, primary
- Groq -- WORKING, fallback #1-2 (llama-3.1-8b-instant, compound)
- NVIDIA NIM -- VERIFIED, wired as fallback #3-4 (gemma-3-4b-it, llama-3.3-70b). 131 models available.
- OpenRouter -- WORKING, free-models-only (fallback #5-7)

Not free (removed from consideration):
- SiliconFlow: paid only, key needs credit deposit
- SambaNova: all models paid
- Together AI: credits only, then paid

**CRITICAL: ZAI/GLM subscription ends May 1.** The fallback chain above is the safety net. Do NOT let it lapse without testing every fallback.

### 2. Update governor config files for real providers
The governor reads models.json, connectors.json, routing.json but they're stale:
- **models.json**: has 10 models including GPT-4o, Claude, Kimi, paid DeepSeek -- none free
- **connectors.json**: has 15 connectors but no NVIDIA NIM, Groq has no base_url
- **routing.json**: current_strategy is still `kimi_priority` -- stale
- Need to: add NVIDIA NIM connector, add free models (Groq/NVIDIA/OpenRouter), replace kimi_priority with cascade strategy

### 3. Wire cascade into governor routing
The Go router needs to actually use the updated cascade config:
- Read routing strategy from `config/routing.json`
- Try primary provider, fallback on rate limit/error
- Track which models succeed/fail in Supabase for learning
- Rate limit tracking across all time windows (RPM, RPD, TPM, TPD)

---

## High Priority

### 4. Context Compaction -- DONE (April 15)

### 5. Git Worktrees -- DONE (April 15)

### 6. Test the pipeline end-to-end
The YAML pipeline is written but never tested against real governor:
- Queue a real task through Supabase
- Watch it flow through: plan -> supervisor -> execute -> review -> test -> merge
- Verify gitree branch management works with parallel tasks
- Fix whatever breaks (will be things)

### 7. Visual QA agent
- Wire browser-use to capture screenshots of dashboard after changes
- Compare to baseline, flag visual regressions
- Present to human for UI/UX yes/no
- This is the courier pattern applied to our own app

---

## Medium Priority

### 8. Daily landscape research cron
The researcher prompt exists (`prompts/daily_landscape_researcher.md`).
Need to wire it:
- Cron job that runs researcher daily
- Checks GitHub trending, HuggingFace new models, provider changelogs
- Posts findings to Supabase for Supervisor review
- Supervisor approves minor, escalates major to Council -> Human

### 9. LogAct patterns (from Meta research)
`research/2026-04-14-logact-agent-bus.md` -- maps directly to our architecture.
Adopt after pipeline is working end-to-end:
- Intent logging: record what agent PLANS to do before execution
- Safety voter: use a different cheap model to cross-check intent before execution
- Append-only task_events table in Supabase (currently we update rows in-place)
- Stupidity diagnosis: agent reads own failed output from log, rewrites

### 10. Pre-execution design preview (from orchestration research)
`research/2026-04-14-orchestration-comparison.md` -- Visual QA moved upstream.
For UI tasks, show human a design choice BEFORE writing code, not just after:
- Courier generates mockup/plan, presents to human
- Human picks direction, THEN agent writes code
- Skip for non-UI tasks (conditional pipeline stage)

### 11. JourneyKits implementation
95 kits scanned, 20 mapped to VibePilot gaps (`research/2026-04-08-journeykits-landscape-analysis.md`).
Need to go through them and decide which patterns to adopt.

---

## Lower Priority

### 12. Make .context/ hooks async
Post-checkout, post-merge, pre-commit hooks rebuild entire knowledge layer synchronously.
On x220 this causes command timeouts. Fix:
- Run rebuild in background (don't block git)
- Or skip if source files haven't changed since last build
- Or both

### 13. Add NVIDIA NIM to governor system.json
The governor MCP server is disabled by default (ready to enable for SSE port 8081).
NVIDIA NIM should also be added as a connector destination.

---

## Done (April 2026)

- [x] .context/ knowledge layer built (knowledge.db, boot.md, tier0)
- [x] Tier 0 hand-crafted rules (single source of truth for all docs)
- [x] All doc contradictions fixed (9 across 5 files)
- [x] YAML pipeline config (code-pipeline.yaml)
- [x] Gitree branch management (parallel agent git isolation)
- [x] Supabase schema (110 migrations, 4 version bumps)
- [x] Free model research (4 providers verified and working)
- [x] Free API keys obtained (Google, Groq, NVIDIA NIM, OpenRouter)
- [x] NVIDIA NIM tested and wired into Hermes fallback chain
- [x] JourneyKits landscape analysis (95 kits, 20 mapped)
- [x] GitHub PAT rotated
- [x] 25 stale files cleaned from repo root
- [x] research-update-april2026 merged into main (fast-forward, 29 commits)
- [x] research-considerations cherry-picked then deleted
- [x] Both disk copies synced on main
- [x] Scripts made portable (no hardcoded usernames)
- [x] Dead branches deleted locally and remotely
- [x] CURRENT_STATE.md rewritten with honest assessment
- [x] Rate limit research preserved as templates (Gemini 4-tier, DeepSeek, Kimi 5-tier)
- [x] LogAct + orchestration research analyzed and saved
- [x] Cloudflared tunnel live (vibestribe.rocks, dashboard chat works, sacred)
- [x] TTS working (edge-tts, fast, free, no API key needed)
- [x] Governor binary rebuilt from current main source (v2.0.0, DAG+MCP+gitree active)
- [x] MCP Client Phase 1 -- jcodemunch (52) + jdocmunch (15) = 67 tools connected
- [x] MCP Server Phase 2 -- governor exposes tools as MCP server (stdio + SSE)
- [x] 3-Layer Memory System -- short/mid/long-term tables in Supabase + Go service
- [x] Migration 110 applied to Supabase (memory_sessions, memory_project, memory_rules)
- [x] Context Compaction -- compactor.go auto-summarizes sessions
- [x] Git Worktrees -- worktree.go for parallel agent isolation
- [x] Tier0 rule 4 expanded -- explicit migration workflow with GitHub links
- [x] Tier0 rule 6 added -- no shortcuts, full work only
- [x] WYNTK updated (architecture tree, knowledge layer, governor structure, file paths)
- [x] Chrome CDP working (port 9222, bind mount, auto-login to Gmail/Gemini/Sheets)

## Not Viable (abandoned)

- [x] Ollama + local models -- ABANDONED. x220 (i5-2520M, AVX-only, no AVX2) maxes at ~6 tok/s. Cloud-only.
- [x] Kokoro TTS -- 9GB, too slow on x220. Edge-tts works fine.
- [x] SiliconFlow -- paid only, not free tier.
- [x] SambaNova -- all models paid ($0.10-$7.00/M tokens).

---

**Last Updated:** 2026-04-15 (evening)
