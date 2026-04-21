# VibePilot Current State
> Last updated: April 21, 2026

## System Status
- **Governor**: Stopped (intelligence overhaul committed, needs E2E test)
- **Supabase**: Live, project `kbhfepwpqztrrzwefskg.supabase.co`
- **GitHub**: `VibesTribe/VibePilot` (public, main branch)
- **Dashboard**: READ-ONLY React frontend, Supabase Realtime subscriptions (NEVER polls)
- **Webhooks**: `webhooks.vibestribe.rocks` (Cloudflare Tunnel) → Supabase

## Recent Changes (April 21, 2026)

### Governor Intelligence Overhaul (commit 57654556)
Four interdependent root causes fixed as one coherent unit:

1. **Fresh Code Map**: CodeMapConfig in system.json (path, cache_ttl_mins, refresh_on_startup). TTL-based cache replaces sync.Once. jcodemunch index_folder runs on startup via MCP.

2. **Config-Driven Agent Context**: context_policy field on agents.json. 4 policies: full_map (planner), file_tree (supervisor/council/etc), targeted (task_runner), none (watcher). Single buildAgentContext() method, no hardcoded switches. New agents get context from config.

3. **Task Packet Enrichment**: Planner requires Target Files per task. Parser extracts and sanitizes. Stored in tasks.result.target_files. At execution, only those files are read from disk and injected into executor prompt.

4. **Supervisor Plan Review**: 3 objective checks: file reference verification, PRD traceability, dependency cycle detection. No subjective limits.

### Design Document
Full design with rationale at `docs/designs/governor-intelligence-fix.md`.

### Consultant Agent + PRD Template (commit 7fbd059e)
Five-phase consultant process synthesized from 6 open-source spec-driven tools:
1. **Discovery**: Fluid conversation (not forms), natural questions one at a time
2. **Research & Architecture**: Tech stack, ADRs, patterns with rationale
3. **Structured Specification**: FR-XXX IDs, Given/When/Then scenarios, typed data contracts, P1/P2/P3 priorities
4. **Constitution Check**: Validate against project principles before PRD
5. **PRD Generation**: Machine-parseable output with self-critique loop (max 3 revisions)

PRD template at `config/templates/prd-template.md`. Every requirement traces to user intent = zero drift.
Consultant prompt at `config/prompts/consultant.md`.

### Pre-existing Fixes (committed earlier April 20-21)
- Plan review race condition: retry loop (3 attempts, 3s sleep) in runPlanReview()
- Stale lock cleanup in recovery.go
- Supervisor routing flag: ExcludeModels checks both test_failed_by: and exec_failed_by: prefixes
- routing_flag_reason set via REST PATCH after transition_task RPC

## Model Fleet (57 models)

| Provider | Active | Benched | Connector | Free Tier |
|----------|--------|---------|-----------|-----------|
| Groq | 7 | 0 | groq-api | Yes (rate-limited) |
| OpenRouter | 19 | 0 | openrouter-api | Yes ($0 credit, max spend limit set) |
| Google Gemini | 4 | 1 | gemini-api-courier/researcher/visual/general | Yes (4 projects, 60 RPM combined) |
| NVIDIA NIM | 3 | 0 | nvidia-api | Yes |
| Web (browser) | 16 | 0 | Various web connectors | Varies |
| **Total** | **49** | **8** | | |

### Benched Models
| Model | Status | Reason |
|-------|--------|--------|
| deepseek-chat | paused | Rate limits too aggressive |
| deepseek-reasoner | paused | Rate limits too aggressive |
| chatgpt-4o-mini | benched | No free API access |
| claude-sonnet | benched | No free API access |
| gemini-web | benched | Web-only, superseded by API |
| kimi-k2-instruct | benched | Available via NVIDIA NIM instead |
| minimax-m2.7 | benched | Unreliable |
| nvidia/nemotron-3-super-120b | benched | Dead model ID |

### Gemini 4-Project Setup
4 independent Google Cloud projects, each with own API key and free-tier quota:
| Project | Key | Model | Role | Rate Limit |
|---------|-----|-------|------|------------|
| Courier | GEMINI_COURIER_KEY | gemini-2.5-flash-lite | Stable workhorse | 15 RPM / 1000 RPD |
| Researcher | GEMINI_RESEARCHER_KEY | gemini-3.1-flash-lite-preview | Best intelligence | 15 RPM / 1500 RPD |
| Visual/Brain | GEMINI_VISUAL_TESTER_KEY | gemini-3-flash-preview | Code fixing, visual QA | 15 RPM / 1500 RPD |
| General | GEMINI_GENERAL_KEY | gemini-2.5-flash-lite | Legacy fallback | 15 RPM / 1500 RPD |

**Combined free capacity**: 60 RPM / ~5500 RPD, $0 cost.

## Connectors (26 total, 22 active)

### API Connectors (7 active)
| ID | Provider | Status | Notes |
|----|----------|--------|-------|
| groq-api | Groq | active | 7 models |
| openrouter-api | OpenRouter | active | 19 free models, $0 credit, max spend limit set |
| nvidia-api | NVIDIA | active | 3 models via NIM |
| gemini-api-courier | Google | active | Courier project |
| gemini-api-researcher | Google | active | Researcher project |
| gemini-api-visual | Google | active | Visual/Brain project |
| gemini-api-general | Google | active | General/fallback project |

### Web Connectors (15 active)
Browser-use connectors for courier agents. All verified working April 20, 2026 via live "hello" test.

| Connector | URL | Model Seen | Best For | Notes |
|-----------|-----|------------|----------|-------|
| chatgpt-web | chatgpt.com | GPT free tier | General | Google SSO |
| claude-web | claude.ai/new | Sonnet 4.6 | Coding, reasoning | Google SSO |
| gemini-web | gemini.google.com/app | Gemini 2.5 Pro | General, vision | Google SSO |
| deepseek-web | chat.deepseek.com | DeepSeek Instant | Coding, R1 reasoning | Google SSO |
| qwen-web | chat.qwen.ai | Qwen3.6-Plus | Coding, multilingual | Google SSO |
| mistral-web | chat.mistral.ai/chat | Mistral Large | Vision (Pixtral), coding | Google SSO |
| notegpt-web | notegpt.io/chat-deepseek | DeepSeek V3 | Quick queries | No auth, 3 free/day |
| kimi-web | kimi.com | K2.6 Instant | Agent tasks | Google SSO, agent swarm |
| huggingchat-web | huggingface.co/chat | Kimi-K2-Instruct | Open source, unlimited | No auth, MCP |
| aistudio-web | aistudio.google.com | Gemini 3.1 Pro Preview | Apps, design, tools | Google SSO, native tools |
| poe-web | poe.com | Multi-model | Prototyping, comparison | Google SSO, 3K pts/day |
| chatbox-web | app.chatbox.ai | GPT-4o mini | Quick GPT access | No auth, free |
| aizolo-web | chat.aizolo.com/new | Multi-model | Research, coding, fallback | Free tier limited, $9.90/mo 3M tokens |
| perplexity-web | perplexity.ai | Search + citations | **System Researcher** | Google SSO, 5 Pro/day, unlimited basic |

### Inactive (4)
opencode, claude-code, kimi, deepseek-api

## Vault (Supabase secrets_vault)
AES-GCM encrypted, PBKDF2 SHA256 100k iterations. ~15 keys stored:

| Vault Key | Purpose |
|-----------|---------|
| GROQ_API_KEY | Groq API access |
| OPENROUTER_API_KEY | OpenRouter ($0 credit account, max spend limit set) |
| GEMINI_COURIER_KEY | Courier project API |
| GEMINI_RESEARCHER_KEY | Researcher project API |
| GEMINI_VISUAL_TESTER_KEY | Visual/Brain project API |
| GEMINI_GENERAL_KEY | General project API |
| NVIDIA_API_KEY | NVIDIA NIM API |
| SUPABASE_URL | Database connection |
| SUPABASE_SERVICE_KEY | Service role access |
| SUPABASE_ANON_KEY | Public anon access |
| VAULT_ENCRYPTION_KEY | Master encryption key |

## Courier Agent Pipeline

### Architecture: GitHub Actions + Supabase Realtime (zero polling)

The courier dispatches browser-use tasks to GitHub Actions (free, zero local weight). Results come back via Supabase Realtime subscriptions -- never polls.

```
Governor → router selects routing_flag="web"
        → CourierRunner.dispatch() sends repository_dispatch to GitHub
        → GitHub Actions spins up ubuntu-latest + browser-use + playwright
        → courier_run.py navigates to web platform, pastes prompt, extracts response
        → courier_run.py writes result to task_runs table via Supabase REST
        → Supabase Realtime fires UPDATE on task_runs
        → Governor realtime client receives EventCourierResult
        → CourierRunner.NotifyResult() delivers to waiting goroutine via channel
        → Task transitions to "review"
```

### Implementation Status

All core pipeline wired and committed. Verified by code audit.

| Component | Status | Commit | Detail |
|-----------|--------|--------|--------|
| Model capabilities + courier markers | Done | bc0197a7 | 11 models marked courier: true |
| Missing vision models added | Done | bc0197a7 | 4 vision models added to models.json |
| PlatformID/PlatformURL in RoutingResult | Done | e4e807ca | router.go carries destination info |
| Hardcoded RoutingFlag removed | Done | e4e807ca | Task runner passes "" (router decides) |
| CourierRunner on TaskHandler struct | Done | e4e807ca | Wired through main.go to TaskHandler |
| Web routing branch in executeTask | Done | e4e807ca | executeCourierTask() method added |
| GitHub Actions workflow | Done | b0b55235 | .github/workflows/courier_dispatch.yml |
| courier_run.py script | Done | b0b55235 | scripts/courier_run.py (browser-use) |
| Supabase Realtime (replaced polling) | Done | 57d9c237 | Zero-polling: channel-based waiters, task_runs subscription |
| EventCourierResult handler | Done | 57d9c237 | main.go wires realtime event to CourierRunner.NotifyResult |
| Pipeline gap fixes (5 gaps) | Done | c2e94151 | Vault threading, RPC params, task_runs columns, result format |
| Vault key derivation (deriveLLMKeyRef) | Done | c2e94151 | Maps connectorID → vault key name |
| Gemini 4-project connectors | Done | 0897340f, 3a16958c | 4 independent keys, correct models, all tested |

### E2E Pipeline Status
First E2E test attempted April 21, 2026 with "Greeting and Farewell Endpoints" PRD.
Pipeline reached Stage 2 (Planner) but failed at Supervisor review due to:
- Race condition in plan review (fixed)
- Supervisor rubber-stamping bad plans (fixed in intelligence overhaul)
- Planner over-engineering (fixed with code map context + supervisor verification)
- Task executor producing generic output (fixed with targeted file injection)

Intelligence overhaul (commit 57654556) addresses all root causes. Needs re-test.

## Budget
- **OpenRouter**: $0 credit account. Max spend limit configured for future-proofing. No payment added.
- **Groq**: Free tier
- **Gemini**: 4x free tier (no billing on any project)
- **NVIDIA NIM**: Free tier
- **GLM-5 (Hermes layer)**: Z.AI Pro subscription, ends May 1, 2026. NOT renewing at $90/3mo.
- **Total API cost**: $0/month (all free tiers)

## Key Architecture Docs
| Doc | Purpose |
|-----|---------|
| VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md | Essential context for any agent |
| docs/CURRENT_STATE.md | This file |
| docs/designs/governor-intelligence-fix.md | Intelligence overhaul design |
| docs/plans/strategic-optimization-plan.md | Full strategic plan |
| governor/config/models.json | 50 models, 42 active |
| governor/config/connectors.json | 19 connectors, 15 active |
| config/prompts/*.md | Agent prompts (planner, supervisor, council, etc.) |

## Hardware
- **Machine**: Lenovo x220, 16GB RAM, ~12GB free
- **OS**: Linux (user-level systemd services)
- **Local inference**: Too slow (2 tok/s tested). Cloud API only.

## Known Gaps (not yet addressed)
- Maintenance agent not wired (git write access disconnected)
- Module branches never created (merge has nowhere to go)
- Worktrees disabled (all tasks share same directory)
- Orchestrator is NOT an LLM call -- just hardcoded cascade in Go
- Dashboard reads mock data in some views, not all live Supabase
- Consultant agent not wired into pipeline (prompt and template exist, needs integration into governor flow)
- Periodic jcodemunch refresh (only runs on startup currently)
