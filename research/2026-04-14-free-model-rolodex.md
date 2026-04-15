# VibePilot Free Model Rolodex - April 2026

**Verified April 14, 2026 via direct API checks and provider documentation.**

## CRITICAL: Gemini Sheet Accuracy Check

From the Google Sheet "Free AI Models for Agentic Cycles":

| Model (Gemini listed) | Actually Exists? | Notes |
|---|---|---|
| Qwen 3.6 Plus | NOT on OpenRouter | Qwen3-Coder and qwen3-next-80b-a3b ARE free on OR |
| Nemotron 3 Super | YES | OpenRouter free, NVIDIA NIM free trial |
| Step 3.5 Flash | NOT found | StepFun not on any verified free provider |
| GLM 4.5 Air | YES | OpenRouter free (z-ai/glm-4.5-air:free) |
| DeepSeek R1 | NOT free on OpenRouter | Only paid. V3.1 is free on SambaNova |
| Arcee Trinity Mini | PARTIAL | trinity-large-preview:free on OR, but GOING AWAY April 22 |
| Llama 4 Scout | YES | Groq free (meta-llama/llama-4-scout-17b-16e-instruct) |
| GPT-OSS-20B | YES | Groq + OpenRouter free |
| MIMO V2 Pro | NOT found | MiniMax M2.5 IS free everywhere |
| Devstral 2 | NOT found on free tiers | |
| SiliconFlow | EXISTS | Free models require real-name verification |
| SambaNova | YES | Free tier, no credit card needed |

## Provider-by-Provider Breakdown

### 1. OpenRouter (openrouter.ai)
- **Credit card:** NOT required
- **Spending cap at $0:** YES (no payment method = $0 cap guaranteed)
- **Free models:** 24 total, ALL at $0/$0 pricing
- **Rate limits:** 50 req/day without credits; 1000 req/day with $10+ in credits
- **Gotcha:** Free models can be discontinued. Arcee Trinity going away April 22.
- **Base URL:** `https://openrouter.ai/api/v1`

**Top free models (by context length):**
| Model | Context | Modality | Notes |
|---|---|---|---|
| nvidia/nemotron-3-super-120b-a12b:free | 262K | text | Best reasoning, Programming #5 |
| google/gemma-4-31b-it:free | 262K | text+image+video | Google's latest, vision capable |
| google/gemma-4-26b-a4b-it:free | 262K | text+image+video | MoE variant, lighter |
| qwen/qwen3-next-80b-a3b-instruct:free | 262K | text | MoE 80B, strong coding |
| qwen/qwen3-coder:free | 262K | text | Coding specialist |
| nvidia/nemotron-3-nano-30b-a3b:free | 256K | text | Lightweight MoE |
| minimax/minimax-m2.5:free | 197K | text | Chinese frontier model |
| openai/gpt-oss-120b:free | 131K | text | OpenAI's open-weight flagship |
| z-ai/glm-4.5-air:free | 131K | text | Agent-centric, thinking mode |
| openai/gpt-oss-20b:free | 131K | text | Lighter OpenAI model |
| nousresearch/hermes-3-llama-3.1-405b:free | 131K | text | HUGE 405B model |
| meta-llama/llama-3.3-70b-instruct:free | 66K | text | Meta's workhorse |
| nvidia/nemotron-nano-12b-v2-vl:free | 128K | text+image+video | Vision capable |
| arcee-ai/trinity-large-preview:free | 131K | text | GOING AWAY APRIL 22 |

### 2. NVIDIA NIM / Build (build.nvidia.com)
- **Credit card:** NOT required for trial
- **Free tier:** Trial access, no payment needed
- **Rate limits:** Not publicly listed for free tier (appears generous)
- **Models:** Nemotron 3 Super, all Nemotron variants
- **Gotcha:** "Your input and output will be recorded" for trial. Not for production.
- **Base URL:** `https://integrate.api.nvidia.com`

### 3. Google AI Studio (aistudio.google.com)
- **Credit card:** NOT required
- **Spending cap:** Free tier is truly free, no charges possible
- **Rate limits:** ~15 RPM for free tier, daily token limits per model
- **Models:** Gemini 2.5 Flash, Gemini 3 Flash Preview, Gemma 4 31B IT, Gemma 3n E4B
- **Gotcha:** Our API key hit daily quota today. Free tier limits can be tight.
- **Base URL:** `https://generativelanguage.googleapis.com/v1beta`

### 4. Groq (groq.com)
- **Credit card:** NOT required for free tier
- **Spending cap at $0:** YES (free plan exists, no payment = no charges)
- **Rate limits (FREE PLAN):**

| Model | RPM | RPD | TPM | TPD |
|---|---|---|---|---|
| llama-3.1-8b-instant | 30 | 14.4K | 6K | 500K |
| llama-3.3-70b-versatile | 30 | 1K | 12K | 100K |
| meta-llama/llama-4-scout-17b-16e-instruct | 30 | 1K | 30K | 500K |
| openai/gpt-oss-120b | 30 | 1K | 8K | 200K |
| openai/gpt-oss-20b | 30 | 1K | 8K | 200K |
| qwen/qwen3-32b | 60 | 1K | 6K | 500K |
| moonshotai/kimi-k2-instruct | 60 | 1K | 10K | 300K |
| groq/compound | 30 | 250 | 70K | - |

- **Speed:** 200-560 tokens/sec (insanely fast)
- **Gotcha:** Daily token limits are strict. Good for fast routing, not long coding sessions.
- **Base URL:** `https://api.groq.com/openai/v1`

### 5. SambaNova (sambanova.ai)
- **Credit card:** NOT required for free tier (no payment method = free tier)
- **Spending cap at $0:** YES (no payment = can't be charged)
- **Rate limits (FREE TIER):**

| Model | RPM | RPD | TPD |
|---|---|---|---|
| DeepSeek-V3.1 | 20 | 20 | 200K |
| Meta-Llama-3.3-70B-Instruct | 20 | 20 | 200K |
| gpt-oss-120b | 20 | 20 | 200K |
| Llama-4-Maverick (preview) | 20 | 20 | 200K |
| DeepSeek-V3.2 (preview) | 20 | 20 | 200K |

- **Gotcha:** Only 20 RPD on free tier! Very limited. Good as backup, not primary.
- **Gotcha:** MiniMax-M2.5 is production but NOT listed in free tier rate limits
- **Base URL:** `https://api.sambanova.ai/v1`

### 6. SiliconFlow (siliconflow.cn)
- **Credit card:** NOT required (but real-name verification IS required)
- **Free models:** Yes, many Chinese models (Qwen, GLM, DeepSeek)
- **Rate limits:** RPM 1000-10000, TPM 50000-5M for chat models
- **Gotcha:** Real-name verification required. Chinese company.
- **Base URL:** `https://api.siliconflow.cn/v1`

### 7. HuggingFace Serverless Inference
- **Credit card:** NOT required
- **Free tier:** Yes, thousands of models
- **Rate limits:** Varies by model, generally generous
- **Gotcha:** Slower than Groq/SambaNova. Some models may be cold-start.
- **Base URL:** `https://api-inference.huggingface.co/models/{model_id}`

### 8. Cloudflare Workers AI
- **Credit card:** NOT required for free tier
- **Free tier:** 10,000 "Neurons" per day
- **Models:** Llama 3.2 (1B/3B), smaller models
- **Gotcha:** Very small models only, limited usefulness for coding
- **Base URL:** `https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/`

### 9. Cerebras (cerebras.ai)
- **Status:** Free inference tier exists but needs verification
- **Speed claim:** Very fast (custom silicon)
- **Gotcha:** Uncertain free tier sustainability

## VibePilot Recommended Cascade

### Priority Order (maximum free, zero risk of charges):

```
TIER 1 - Primary (highest quality, good rate limits)
├── Google AI Studio (Gemini 2.5 Flash)     -- 15 RPM, best reasoning
├── OpenRouter (nemotron-3-super:free)       -- 50-1000 RPD, 262K ctx
└── OpenRouter (gpt-oss-120b:free)           -- 50-1000 RPD, OpenAI flagship

TIER 2 - Speed Specialists (fast routing/coding)
├── Groq (qwen/qwen3-32b)                   -- 60 RPM, 500K TPD, FAST
├── Groq (llama-4-scout)                     -- 30 RPM, 500K TPD, FAST
└── Groq (gpt-oss-20b)                       -- 30 RPM, 200K TPD, FAST

TIER 3 - Alternates (different providers, avoid single-vendor dependency)
├── SambaNova (DeepSeek-V3.1)               -- 20 RPD, reasoning king
├── SambaNova (Llama-4-Maverick)             -- 20 RPD, preview
├── OpenRouter (glm-4.5-air:free)            -- Agent-centric GLM
└── OpenRouter (minimax-m2.5:free)           -- Chinese frontier

TIER 4 - ABANDONED (local LLM not viable on x220)
└── See below: x220 (i5-2520M, AVX-only) maxes at ~6 tok/s with 1B models. Dead end.

TIER 5 - Last Resort
├── SiliconFlow (Qwen, GLM models)          -- Needs real-name verify
└── HuggingFace Serverless                   -- Slow but vast variety
```

### Phase-Based Routing (from Gemini sheet, updated with real models):
- **Router** (decide task type): Groq gpt-oss-20b (500 tok/s, 30 RPM)
- **Builder** (coding): OpenRouter qwen3-coder:free or glm-4.5-air:free
- **Validator** (reasoning): OpenRouter nemotron-3-super:free or Google Gemini 2.5 Flash
- **Cleanup** (summaries): OpenRouter minimax-m2.5:free

## Key Architecture Decisions

1. **OpenRouter is safe at $0** - no payment method = impossible to charge
2. **Groq is safe at $0** - free plan exists, no card needed
3. **SambaNova is safe at $0** - no payment method = free tier automatically
4. **Google AI Studio is safe** - free tier, no card on file
5. **Multi-provider is ESSENTIAL** - don't rely on any single one
6. **Local Ollama ABANDONED** - x220 (AVX-only, no AVX2) too weak. Even 1B models unreliable. Cloud-only + GitHub/Supabase DR.

## Gemini "Hallucination" Assessment

The Gemini sheet was **mostly accurate** but some model names were wrong:
- "Qwen 3.6 Plus" doesn't exist as such -- qwen3-coder and qwen3-next-80b-a3b DO exist
- "Step 3.5 Flash" not found anywhere -- may be future/hallucinated
- "Devstral 2" not on free tiers
- "MIMO V2 Pro" not found -- but MiniMax M2.5 IS available (Gemini confused the name)
- "Arcee Trinity Mini" -- only Trinity Large Preview exists, and it's GOING AWAY April 22

The strategic recommendations (cascade, phase routing, multi-key vault) were sound regardless of specific model name accuracy.

## Action Items for VibePilot

1. **Update models.json** with verified free model list
2. **Get API keys** for Groq, SambaNova, and NVIDIA NIM (all free, no card)
3. **Implement cascade logic** in Go governor: try T1 -> T2 -> T3 -> T4
4. **Store all keys in Supabase Vault** (already have infrastructure)
5. **Implement rate limit tracking** - respect each provider's limits
6. ~~Local Ollama setup~~ ABANDONED - x220 too weak for local inference
