# LLM API Pricing & Model Landscape — April 2026 Update

**Date:** 2026-04-08
**Source:** OpenRouter verified data, live rankings
**Supersedes:** llm-api-pricing-comparison-20260220.md

---

## Executive Summary

Massive shift since February. Three new model families with free tiers dominate the landscape. The courier agent strategy is more viable than ever.

**TOP 5 most-used models (OpenRouter weekly, April 8 2026):**

| Rank | Model | Weekly Tokens | Free Tier? | Context |
|------|-------|---------------|------------|---------|
| 1 | Qwen 3.6 Plus | 6.27T | YES | 1M |
| 2 | MiMo-V2-Pro (Xiaomi) | 1.76T | No | ? |
| 3 | Step 3.5 Flash (StepFun) | 1.25T | YES | ? |
| 4 | DeepSeek V3.2 | 1.2T | Web yes | 128K |
| 5 | Qwen 3.6 Plus Preview | 1.18T | YES | 1M |
| 6 | MiniMax M2.7 | 1.18T | No | 205K |
| 7 | Claude Sonnet 4.6 | 1.04T | Web yes | 200K |
| 9 | MiniMax M2.5 | 987B | YES | 197K |
| 10 | Gemini 3 Flash Preview | 977B | Web/API yes | 1M |

---

## New Models Since February

### Qwen 3.6 Plus (Released April 2, 2026)
- **What:** Hybrid linear attention + sparse MoE architecture
- **Context:** 1M tokens (65.5K output)
- **SWE-bench Verified:** 78.8
- **Key feature:** "Vibe coding" — excels at 3D scenes, games, repo-level problem solving
- **OpenRouter pricing:** $0.325/M input, $1.95/M output (under 256K); $1.30/$3.90 over 256K
- **FREE tier:** Yes, on OpenRouter (qwen/qwen3.6-plus:free)
- **Web chat:** chat.qwen.ai — email signup, no Chinese phone needed
- **119 languages, MCP support, tool use**
- **Why it matters:** #1 model by usage in its first week. Free. 1M context. Built for agentic coding — exactly what VibePilot needs.

### Qwen 3.5 Series (Released Feb-Mar 2026)
- **3.5-9B:** Multimodal (text+vision), $0.05/$0.15 per M tokens. Cheap multimodal option.
- **3.5-27B:** Dense, $0.1625/$1.30 per M tokens.
- **3.5-35B-A3B:** MoE, $0.1625/$1.30 per M tokens. Vision-language.

### MiniMax M2.7 (Released March 18, 2026)
- **What:** Designed for autonomous real-world productivity
- **Context:** 205K tokens
- **Benchmarks:** SWE-Pro 56.2%, TerminalBench 57.0%, GDPval-AA ELO 1495
- **OpenRouter pricing:** $0.30/M input, $1.20/M output
- **Key feature:** Multi-agent collaboration built in. Live debugging, root cause analysis, financial modeling
- **Why it matters:** Purpose-built for agent orchestration workflows. Directly relevant to VibePilot pipeline.

### MiniMax M2.5 (Released February 12, 2026)
- **Context:** 197K tokens
- **Benchmarks:** SWE-bench 80.2%, Multi-SWE 51.3%, BrowseComp 76.3%
- **OpenRouter pricing:** $0.118/M input, $0.99/M output
- **FREE tier:** Yes, on OpenRouter (minimax/minimax-m2.5:free)
- **Why it matters:** SWE-bench 80.2% beats M2.7 on coding. FREE. Cheaper API. Token-efficient.

### MiMo-V2-Pro (Xiaomi)
- **What:** Xiaomi's AI model, surging fast
- **OpenRouter:** #2 weekly with 1.76T tokens
- **Pricing/access:** Needs research. Available on OpenRouter.
- **Why it matters:** 60% growth, massive adoption. Capability details still unknown.

### Step 3.5 Flash (StepFun)
- **What:** StepFun's latest, free on OpenRouter
- **OpenRouter:** #3 weekly with 1.25T tokens, 13% growth
- **FREE tier:** Yes, on OpenRouter
- **Pricing/access:** Free. Web access details unknown.

---

## Updated Pricing Comparison

| Model | Input/M | Output/M | Context | Free? | Best For |
|-------|---------|----------|---------|-------|----------|
| DeepSeek V3.2 | $0.28 (cache $0.028) | $0.42 | 128K | Web | Cheapest quality API |
| MiniMax M2.5 | $0.118 | $0.99 | 197K | OR free | Coding, office docs |
| MiniMax M2.7 | $0.30 | $1.20 | 205K | No | Agent workflows |
| Qwen 3.5-9B | $0.05 | $0.15 | 256K | No | Cheap multimodal |
| Qwen 3.6 Plus | $0.325 | $1.95 | 1M | OR free | Agentic coding, long context |
| Gemini 2.5 Flash | $0.30 | $2.50 | 1M | API free | Multimodal, long context |
| GLM-5 | $1.00 | $3.20 | 200K | No | Coding, low hallucination |
| Claude Sonnet 4.6 | $3.00 | $15.00 | 200K | Web | Quality coding |
| Claude Opus 4.6 | $15.00 | $75.00 | 200K | No | NEVER default |

---

## VibePilot Routing Strategy (Updated April 2026)

### Free Tier Priority (Zero Cost)
1. Qwen 3.6 Plus free (OpenRouter) — coding, agentic tasks
2. MiniMax M2.5 free (OpenRouter) — coding, SWE-bench 80.2%
3. Step 3.5 Flash free (OpenRouter) — bulk tasks
4. DeepSeek web chat — unlimited volume
5. Copilot — unlimited sessions, GPT-4o
6. Gemini web — 100/day, 1M context

### Cheap API Backup ($0.05-0.30/M input)
1. DeepSeek V3.2 — $0.028/M cached
2. Qwen 3.5-9B — $0.05/M
3. MiniMax M2.5 — $0.118/M

### Quality API for Critical Steps ($0.30/M input)
1. MiniMax M2.7 — $0.30/M, agent-focused
2. Qwen 3.6 Plus — $0.325/M, 1M context

### NEVER Auto-Route
- Claude Opus ($15/M+)
- Claude Sonnet ($3/M+)
- GPT-4o ($2.50/M+)

---

## Research Gaps (Still Need Verification)
1. MiMo-V2-Pro — full capabilities, pricing, web access
2. Step 3.5 Flash — full capabilities, web access URL
3. MiniMax web access — does hailuoai.com work without Chinese phone now?
4. Qwen 3.6 Plus — multimodal capabilities (vision/audio)?
5. Gemma latest on Ollama — which size runs well on i5/16GB?

---

**File created: April 8, 2026**
**Data source: OpenRouter live rankings and model pages**
**Next update: When new models drop or pricing changes**
