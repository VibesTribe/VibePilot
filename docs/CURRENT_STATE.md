# VibePilot Current State
> Last updated: April 20, 2026

## System Status
- **Governor**: Stopped (`systemctl --user stop vibepilot-governor`)
- **Supabase**: Live, project `kbhfepwpqztrrzwefskg.supabase.co`
- **GitHub**: `VibesTribe/VibePilot` (public, main branch)
- **Dashboard**: READ-ONLY React frontend, Supabase Realtime subscriptions (NEVER polls)
- **Webhooks**: `webhooks.vibestribe.rocks` (Cloudflare Tunnel) → Supabase

## Model Fleet (50 models)

| Provider | Active | Benched | Connector | Free Tier |
|----------|--------|---------|-----------|-----------|
| Groq | 7 | 0 | groq-api | Yes (rate-limited) |
| OpenRouter | 19 | 0 | openrouter-api | Yes ($0 credit, max spend limit set) |
| Google Gemini | 4 | 1 | gemini-api-courier/researcher/visual/general | Yes (4 projects, 60 RPM combined) |
| NVIDIA NIM | 3 | 0 | nvidia-api | Yes |
| Web (browser) | 9 | 0 | Various web connectors | N/A |
| **Total** | **42** | **8** | | |

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

## Connectors (19 total, 15 active)

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

### Web Connectors (8 active)
Browser-use connectors: chatgpt-web, claude-web, gemini-web, deepseek-web, qwen-web, mistral-web, notegpt-web, hermes

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

### Implementation Status
Steps 1-5 of 10 complete (committed, pushed to main):

| Step | Description | Status | Commit |
|------|-------------|--------|--------|
| 1 | Model capabilities + courier markers | Done | 833a94cf |
| 2 | Connector courier config + PlatformID/URL | Done | 833a94cf |
| 3 | CourierRunner scaffold | Done | 833a94cf |
| 4 | Vault threading to TaskHandler | Done | c2e94151 |
| 5 | Pipeline gap fixes (RPC, result format) | Done | c2e94151 |
| 6 | GitHub Actions workflow | Done | b0b55235 |
| 7 | Supabase realtime listener | Done | 57d9c237 |
| 8 | End-to-end test | Pending | - |
| 9 | Error handling + retry logic | Pending | - |
| 10 | Dashboard courier visibility | Pending | - |

### Courier Flow
```
Task created → Planner assigns → Router selects model
    → routing_flag="web" → CourierRunner builds packet
    → Vault decrypts API key → LLM API call
    → Result → task_runs (JSONB)
    → Tokens counted client-side → Cost calculated
    → transition_task → next stage
```

### Key Files
| File | Purpose |
|------|---------|
| governor/internal/connectors/courier.go | CourierRunner, API dispatch |
| governor/cmd/governor/handlers_task.go | Vault threading, deriveLLMKeyRef |
| scripts/courier_run.py | Python runner (GitHub Actions path) |
| .github/workflows/courier-dispatch.yml | GitHub Actions workflow |

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
| docs/plans/courier-implementation-plan.md | 10-step courier plan |
| docs/plans/strategic-optimization-plan.md | Full strategic plan |
| governor/config/models.json | 50 models, 42 active |
| governor/config/connectors.json | 19 connectors, 15 active |
| prompts/courier.md | Courier agent system prompt |
| prompts/orchestrator.md | Orchestrator system prompt |

## Hardware
- **Machine**: Lenovo x220, 16GB RAM, ~12GB free
- **OS**: Linux (user-level systemd services)
- **Local inference**: Too slow (2 tok/s tested). Cloud API only.
