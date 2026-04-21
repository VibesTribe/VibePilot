# VibePilot Current State
# AUTO-UPDATED: 2026-04-20 20:15 UTC
# RULE: Update this file after ANY change set. Resume from here, never from guesses.
# RULE: NEVER update from assumptions. ALWAYS verify against actual code/data first.

## Three Sources of Truth

1. **GitHub (code):** https://github.com/VibesTribe/VibePilot — pushed=real
2. **Supabase (data):** https://qtpdzsinvifkgpxyxlaz.supabase.co — in DB=real
3. **Dashboard (live):** https://vibeflow-dashboard.vercel.app/ — rendering=working

## System Status

- **Governor:** STOPPED + DISABLED (won't auto-start on boot)
- **Git:** main branch, clean, synced with origin
- **Dashboard:** Live, 0 tasks, 0 task_runs (clean slate)
- **Chrome CDP:** 127.0.0.1:9222

## Models — Config (models.json): 57 total

### Active API Models (33)
**Groq (7)** — key WORKS (needs User-Agent header)
- meta-llama/llama-4-scout-17b-16e-instruct, openai/gpt-oss-120b
- groq/compound, groq/compound-mini
- llama-3.3-70b-versatile, llama-3.1-8b-instant, qwen3-32b

**OpenRouter Paid (5)**
- google/gemma-4-31b-it, z-ai/glm-4.5-air, minimax/minimax-m2.5
- nvidia/nemotron-3-super-120b-a12b, nvidia/nemotron-3-super-120b

**OpenRouter Free (12)**
- google/gemma-4-26b-a4b-it:free, google/gemma-3-27b-it:free
- google/gemma-3-12b-it:free, google/gemma-3-4b-it:free
- google/gemma-3n-e2b-it:free, google/gemma-3n-e4b-it:free
- meta-llama/llama-3.2-3b-instruct:free, meta-llama/llama-3.3-70b-instruct:free
- nousresearch/hermes-3-llama-3.1-405b:free, qwen/qwen3-coder:free
- nvidia/nemotron-3-nano-30b-a3b:free, nvidia/nemotron-nano-12b-v2-vl:free
- openai/gpt-oss-20b:free

**NVIDIA NIM (3)** — key WORKS
- meta/llama-3.3-70b-instruct, moonshotai/kimi-k2-instruct
- nvidia/llama-3.1-nemotron-ultra-253b-v1 (response format issue, not auth)

**Gemini API (4 keys, all WORKING)** — 4 independent Google Cloud projects
- gemini-2.5-flash-lite (Courier key)
- gemini-3.1-flash-lite-preview (Researcher key)
- gemini-3-flash-preview (Visual Tester key)
- gemini-2.5-flash-lite (General key) — same model, different project/quota

**Other API (1)**
- bytedance/ui-tars-1.5-7b (courier vision model)

**Hermes/CLI (1)** — Z.AI subscription, ends May 1
- glm-5 (hermes interactive only, not pipeline-routable)

### Active Web Courier Models (16)
- gemini-2.5-pro, gemini-3.1-pro-preview-web (Google)
- deepseek-r1, deepseek-v3 (DeepSeek Web)
- qwen-2.5, qwen-3.6-plus (Qwen Web)
- mistral-large, codestral, pixtral (Mistral Le Chat)
- chatgpt-4o-mini-chatbox (Chatbox AI)
- perplexity-free (Perplexity)
- poe-mix (Poe)
- aizolo-mix (AiZolo)
- kimi-k2.6-instant, kimi-k2-instruct-0905-hf (Kimi AI)
**NOTE:** All need browser automation. None can execute tasks via API.

### Paused Models (2)
- deepseek-chat — out of credit
- deepseek-reasoner — out of credit

### Benched Models (6)
- chatgpt-4o-mini — Web-only, browser automation not built
- claude-sonnet — Web-only, no API key
- gemini-web — Web-only, browser automation not built
- kimi-k2-instruct — Benched from Groq, use NVIDIA version instead
- minimax-m2.7 — No API access, only via OpenRouter as m2.5
- nvidia/nemotron-3-super-120b — Duplicate of -a12b variant

## Config <-> Supabase Sync: VERIFIED IN SYNC (2026-04-20 20:15)

- 56 models in both config and DB
- 2 additional benched entries in DB only (gemini-2.5-flash, qwen-3 -- replaced)
- 0 status mismatches
- GAP 5 RESOLVED

## Connectors (26 total)

### Active API (7)
- Groq Cloud API — shared org 100K TPD tracked
- OpenRouter API — free tier, $0.50 credit
- NVIDIA NIM API — free tier
- Gemini API x4 (Courier/Researcher/Visual/General projects)

### Active Web (14)
- ChatGPT, Claude, Gemini, DeepSeek, Qwen, Mistral, NoteGPT, Kimi, HuggingChat, Google AI Studio, Poe, Chatbox, AiZolo, Perplexity

### Active CLI (1)
- Hermes Agent

### Inactive (4)
- OpenCode CLI, Claude Code CLI, Kimi CLI, DeepSeek API (out of credit)

## Secrets Vault (15 entries)

### Decrypted + Verified (10)
- GROQ_API_KEY — WORKS
- OPENROUTER_API_KEY — WORKS
- NVIDIA_API_KEY — WORKS
- GEMINI_COURIER_KEY — WORKS
- GEMINI_RESEARCHER_KEY — WORKS
- GEMINI_VISUAL_TESTER_KEY — WORKS
- GEMINI_GENERAL_KEY — WORKS
- GEMINI_API_KEY — WORKS (legacy single-key, still valid)
- GITHUB_TOKEN — Valid
- ZAI_API_KEY — Decrypted

### Can't Decrypt (4) — likely different encryption
- SUPABASE_SERVICE_KEY
- VIBEPILOT_GMAIL_EMAIL
- VIBEPILOT_GMAIL_PASSWORD
- webhook_secret

## Learning System — Committed State

### WIRED (recording data on every task lifecycle event)
| Handler | RecordUsage | RecordCompletion | recordSuccess/Failure | update_model_learning |
|---------|-------------|-----------------|----------------------|----------------------|
| handlers_task.go | YES | YES | YES | YES |
| handlers_plan.go | YES | YES | YES | YES |
| handlers_maint.go | — | — | YES | — |

### NOT WIRED (data leaks)
| Handler | Status | Impact |
|---------|--------|--------|
| Review rejection (fail case) | WIRED (recordFailure + recordModelLearning) | |
| Review needs_revision | WIRED (recordModelLearning) | |
| Review reroute | WIRED (recordModelLearning) | |
| Review council_review | NOT wired (no learning) | Low impact — rare case |
| Review timeout | NOT wired | Low impact — failure signal only |

**NOTE:** All review outcomes except council_review and timeout ARE wired.
Previous analysis was wrong -- the hooks were added but had a compile bug (undefined reviewStart).
Bug fixed in commit 1b1cc612.

### Learning RPCs Available in Supabase
- record_model_success(p_model_id, p_task_id, p_task_type, p_duration_seconds, p_tokens_used)
- record_model_failure(p_model_id, p_task_id, p_failure_type)
- update_model_learning(p_model_id, p_task_type, p_outcome, p_failure_class, p_failure_category, p_failure_detail)

### Persistence
- UsageTracker: LoadFromDatabase on startup, PersistToDatabase every 30s + shutdown
- ConnectorUsageTracker: shared connector limits (migration 126)
- PlatformUsageTracker: web platform limits (migration 126)

## Dashboard Status Model
- OK Ready — active, idle
- ~> Active — active, working on tasks
- ... Cooldown — paused with cooldown_expires_at timer
- $ Credit Needed — paused + status_reason contains "credit"
- ! Issue — everything else non-active (benched, deprecated, no key)
- **NOTHING is hidden** — benched models visible with reason

## Three Review Triggers (escalate to human)
1. Visual UI/UX approval after changes
2. System researcher suggestions after council review
3. API credit exhaustion (status=paused + reason contains "credit")

## Key Architecture Rules
- Models table = models we can route tasks to
- Platforms table = destinations where couriers go
- Connectors = how we reach the model (API keys, endpoints)
- One model can be reached via multiple connectors
- Pipeline picks best model FIRST, then picks best available connector
- Benched != invisible. Everything shows with truth about why.
- Token counting always on OUR side (prompt packets + outputs), never trust external counts
- Credentials live in Supabase vault. Period. No .env files.
- API test caveat: Groq needs User-Agent header or gets Cloudflare 1010

## Files Changed (last session commits on main)
- e6770a52 feat: research and document actual platform limits with sources
- d8edeb8e feat: add AiZolo + Perplexity, tag platforms with best_for roles
- 276454c9 feat: add 5 new web platforms, fix stale Qwen model names
- 0897340f feat: 4 independent Gemini projects = 4x free tier (60 RPM)
- 7c76aa21 models: expand OpenRouter free lineup to 19 active models
- c2e94151 fix: close 4 courier pipeline gaps found in audit
- b0b55235 feat: GitHub Actions courier workflow + browser-use script
