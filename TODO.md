# VibePilot TODO - 2026-04-14 (evening)

## Critical (do first)

### 1. Rebuild the governor binary
The running binary (Apr 11) was compiled from OLD main source. The Go source now on
main has newer code (DAG engine, MCP support, gitree improvements) that isn't compiled.
- `cd ~/vibepilot/governor && go build -o governor ./cmd/governor/`
- `systemctl --user restart vibepilot-governor`

### 2. Get free API keys (user action)
Currently only Google AI Studio key exists. These are free, no card needed:
- Groq (fast inference: qwen3-32b, llama-4-scout) -- https://console.groq.com
- SambaNova (DeepSeek-V3.1, Llama-4-Maverick) -- https://cloud.sambanova.ai
- SiliconFlow (Qwen, GLM, DeepSeek) -- https://cloud.siliconflow.cn
- Sign up, get keys, add to Supabase vault

---

## High Priority

### 3. Build model cascade into config
The free model rolodex is researched (`research/2026-04-14-free-model-rolodex.md`).
Rate limit templates exist (`docs/rate_limits/` -- multi-tier RPM/TPM/RPD data).
Need to wire it into the config layer:
- Update `config/models.json` with verified free models + rate limits
- Update `config/connectors.json` with new provider endpoints
- Update `config/routing.json` -- replace stale `kimi_priority` with cascade strategy
- Define fallback order: Groq -> Google AI Studio -> SambaNova -> OpenRouter free

### 4. Wire cascade into governor routing
The Go router needs to actually use the cascade config:
- Read routing strategy from `config/routing.json`
- Try primary provider, fallback on rate limit/error
- Track which models succeed/fail in Supabase for learning
- Rate limit tracking across all time windows (RPM, RPD, TPM, TPD)

### 5. Update WYNTK (VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md)
Still references old state in many sections:
- Architecture components list is outdated
- Config file paths may be stale
- Missing .context/ knowledge layer documentation
- Missing gitree / orphan branch documentation
- Missing pipeline YAML documentation

---

## Medium Priority

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
- Intent logging: record what agent PLANS to do before execution (not just state transitions)
- Safety voter: use a different cheap model to cross-check intent before execution
- Append-only task_events table in Supabase (currently we update rows in-place)
- Stupidity diagnosis: agent reads own failed output from log, rewrites

### 10. Pre-execution design preview (from orchestration research)
`research/2026-04-14-orchestration-comparison.md` -- Visual QA moved upstream.
For UI tasks, show human a design choice BEFORE writing code, not just after:
- Courier generates mockup/plan, presents to human
- Human picks direction, THEN agent writes code
- Skip for non-UI tasks (conditional pipeline stage)
- Superpowers was the only approach to one-shot tasks using this pattern

### 11. JourneyKits implementation
95 kits scanned, 20 mapped to VibePilot gaps (`research/2026-04-08-journeykits-landscape-analysis.md`).
Need to go through them and decide which patterns to adopt for courier agents, pipeline stages, etc.

---

## Lower Priority

### 12. Make .context/ hooks async
Post-checkout, post-merge, pre-commit hooks rebuild entire knowledge layer synchronously.
On x220 this causes command timeouts. Fix:
- Run rebuild in background (don't block git)
- Or skip if source files haven't changed since last build
- Or both

---

## Done (April 2026)

- [x] .context/ knowledge layer built (knowledge.db, boot.md, tier0)
- [x] Tier 0 hand-crafted rules (single source of truth for all docs)
- [x] All doc contradictions fixed (9 across 5 files)
- [x] YAML pipeline config (code-pipeline.yaml)
- [x] Gitree branch management (parallel agent git isolation)
- [x] Supabase schema (109 migrations, 4 version bumps)
- [x] Free model research (7 providers verified)
- [x] JourneyKits landscape analysis (95 kits, 20 mapped)
- [x] GitHub PAT rotated
- [x] 25 stale files cleaned from repo root
- [x] research-update-april2026 merged into main (fast-forward, 29 commits)
- [x] research-considerations cherry-picked (rate limits, research reports, scripts) then deleted
- [x] Both disk copies synced on main
- [x] Scripts made portable (no hardcoded usernames)
- [x] Dead branches deleted locally and remotely
- [x] CURRENT_STATE.md rewritten with honest assessment
- [x] Rate limit research preserved as templates (Gemini 4-tier, DeepSeek, Kimi 5-tier)
- [x] LogAct + orchestration research analyzed and saved
- [x] Cloudflared tunnel live (vibestribe.rocks, dashboard chat works, sacred)
- [x] TTS working (edge-tts, fast, free, no API key needed)

## Not Viable (abandoned)

- [x] Ollama + local models -- tested qwen3:4b and qwen3-vl:4b, 2 tok/s on i5, unusable. Models deleted, daemon disabled. Cloud free tiers are the path.
- [x] Kokoro TTS -- 9GB, too slow on x220. Edge-tts works fine.

---

**Last Updated:** 2026-04-14
