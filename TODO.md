     1|# VibePilot TODO - 2026-04-14 (evening)
     2|
     3|## Critical (do first)
     4|
     5|### 1. Get free API keys (user action)
     6|Currently only Google AI Studio key exists. These are free, no card needed:
     7|- Groq (fast inference: qwen3-32b, llama-4-scout) -- https://console.groq.com
     8|- SambaNova (DeepSeek-V3.1, Llama-4-Maverick) -- https://cloud.sambanova.ai
     9|- SiliconFlow (Qwen, GLM, DeepSeek) -- https://cloud.siliconflow.cn
    10|- Sign up, get keys, add to Supabase vault
    11|
    12|---
    13|
    14|## High Priority
    15|
    16|### 2. Build model cascade into config
    17|The free model rolodex is researched (`research/2026-04-14-free-model-rolodex.md`).
    18|Rate limit templates exist (`docs/rate_limits/` -- multi-tier RPM/TPM/RPD data).
    19|Need to wire it into the config layer:
    20|- Update `config/models.json` with verified free models + rate limits
    21|- Update `config/connectors.json` with new provider endpoints
    22|- Update `config/routing.json` -- replace stale `kimi_priority` with cascade strategy
    23|- Define fallback order: Groq -> Google AI Studio -> SambaNova -> OpenRouter free
    24|
    25|### 3. Wire cascade into governor routing
    26|The Go router needs to actually use the cascade config:
    27|- Read routing strategy from `config/routing.json`
    28|- Try primary provider, fallback on rate limit/error
    29|- Track which models succeed/fail in Supabase for learning
    30|- Rate limit tracking across all time windows (RPM, RPD, TPM, TPD)
    31|
### 4. Update WYNTK (VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md)
Done April 14. Architecture tree, knowledge layer, governor structure, file paths all current.
Still needs: clean stale `kimi_priority` reference from routing.json section.
    39|
    40|---
    41|
    42|## Medium Priority
    43|
    44|### 5. Test the pipeline end-to-end
    45|The YAML pipeline is written but never tested against real governor:
    46|- Queue a real task through Supabase
    47|- Watch it flow through: plan -> supervisor -> execute -> review -> test -> merge
    48|- Verify gitree branch management works with parallel tasks
    49|- Fix whatever breaks (will be things)
    50|
    51|### 6. Visual QA agent
    52|- Wire browser-use to capture screenshots of dashboard after changes
    53|- Compare to baseline, flag visual regressions
    54|- Present to human for UI/UX yes/no
    55|- This is the courier pattern applied to our own app
    56|
    57|### 7. Daily landscape research cron
    58|The researcher prompt exists (`prompts/daily_landscape_researcher.md`).
    59|Need to wire it:
    60|- Cron job that runs researcher daily
    61|- Checks GitHub trending, HuggingFace new models, provider changelogs
    62|- Posts findings to Supabase for Supervisor review
    63|- Supervisor approves minor, escalates major to Council -> Human
    64|
    65|### 8. LogAct patterns (from Meta research)
    66|`research/2026-04-14-logact-agent-bus.md` -- maps directly to our architecture.
    67|Adopt after pipeline is working end-to-end:
    68|- Intent logging: record what agent PLANS to do before execution (not just state transitions)
    69|- Safety voter: use a different cheap model to cross-check intent before execution
    70|- Append-only task_events table in Supabase (currently we update rows in-place)
    71|- Stupidity diagnosis: agent reads own failed output from log, rewrites
    72|
    73|### 9. Pre-execution design preview (from orchestration research)
    74|`research/2026-04-14-orchestration-comparison.md` -- Visual QA moved upstream.
    75|For UI tasks, show human a design choice BEFORE writing code, not just after:
    76|- Courier generates mockup/plan, presents to human
    77|- Human picks direction, THEN agent writes code
    78|- Skip for non-UI tasks (conditional pipeline stage)
    79|- Superpowers was the only approach to one-shot tasks using this pattern
    80|
    81|### 10. JourneyKits implementation
    82|95 kits scanned, 20 mapped to VibePilot gaps (`research/2026-04-08-journeykits-landscape-analysis.md`).
    83|Need to go through them and decide which patterns to adopt for courier agents, pipeline stages, etc.
    84|
    85|---
    86|
    87|## Lower Priority
    88|
    89|### 11. Make .context/ hooks async
    90|Post-checkout, post-merge, pre-commit hooks rebuild entire knowledge layer synchronously.
    91|On x220 this causes command timeouts. Fix:
    92|- Run rebuild in background (don't block git)
    93|- Or skip if source files haven't changed since last build
    94|- Or both
    95|
    96|---
    97|
    98|## Done (April 2026)
    99|
   100|- [x] .context/ knowledge layer built (knowledge.db, boot.md, tier0)
   101|- [x] Tier 0 hand-crafted rules (single source of truth for all docs)
   102|- [x] All doc contradictions fixed (9 across 5 files)
   103|- [x] YAML pipeline config (code-pipeline.yaml)
   104|- [x] Gitree branch management (parallel agent git isolation)
   105|- [x] Supabase schema (109 migrations, 4 version bumps)
   106|- [x] Free model research (7 providers verified)
   107|- [x] JourneyKits landscape analysis (95 kits, 20 mapped)
   108|- [x] GitHub PAT rotated
   109|- [x] 25 stale files cleaned from repo root
   110|- [x] research-update-april2026 merged into main (fast-forward, 29 commits)
   111|- [x] research-considerations cherry-picked (rate limits, research reports, scripts) then deleted
   112|- [x] Both disk copies synced on main
   113|- [x] Scripts made portable (no hardcoded usernames)
   114|- [x] Dead branches deleted locally and remotely
   115|- [x] CURRENT_STATE.md rewritten with honest assessment
   116|- [x] Rate limit research preserved as templates (Gemini 4-tier, DeepSeek, Kimi 5-tier)
   117|- [x] LogAct + orchestration research analyzed and saved
   118|- [x] Cloudflared tunnel live (vibestribe.rocks, dashboard chat works, sacred)
   119|- [x] TTS working (edge-tts, fast, free, no API key needed)
- [x] Governor binary rebuilt from current main source (v2.0.0, DAG+MCP+gitree active)
   120|
   121|## Not Viable (abandoned)
   122|
   123|- [x] Ollama + local models -- tested qwen3:4b and qwen3-vl:4b, 2 tok/s on i5, unusable. Models deleted, daemon disabled. Cloud free tiers are the path.
   124|- [x] Kokoro TTS -- 9GB, too slow on x220. Edge-tts works fine.
   125|
   126|---
   127|
   128|**Last Updated:** 2026-04-14
   129|