# LLM API Pricing Comparison for VibePilot

**Date:** 2026-02-20  
**Purpose:** Cost analysis for VibePilot LLM routing  
**Status:** For review

---

## Executive Summary

For VibePilot's voice interface (100 queries/day, ~500 tokens in/out per query):

| Option | Monthly Cost | Best For |
|--------|-------------|----------|
| **Gemini 3 Flash-Lite** | ~$0.15 | Cheapest, free tier available |
| **DeepSeek V3.2** | ~$0.21 | Lowest cost, good quality |
| **GLM-4.7** | ~$0.84 | Coding tasks |
| **Kimi K2.5** | ~$0.93 | Best reasoning quality |
| **GLM-5** | ~$1.05 | Open-source, low hallucination |
| **Current Stack** (GLM-5 + Kimi) | ~$2.00 | High quality, flexible |

---

## 🆕 New Releases (February 2026)

### GLM-5 (Released Feb 11, 2026)
- **Input:** $1.00/M tokens ($0.20 cached)
- **Output:** $3.20/M tokens
- **Context:** 200K tokens (128K output)
- **Architecture:** 744B params (40B active), MoE, MIT license
- **Hardware:** Trained on Huawei Ascend chips (zero NVIDIA)
- **Benchmarks:** 77.8% SWE-bench, lowest hallucination rate in industry

### Gemini 3.1 (Just Released)
| Model | Input | Output | Context | Notes |
|-------|-------|--------|---------|-------|
| Gemini 3 Pro | $2.00 ($4 long) | $12.00 ($18 long) | 1M | Premium quality |
| Gemini 3 Flash | $0.50 | $3.00 | 1M | Fast, multimodal |
| Gemini 3 Flash-Lite | $0.10 | $0.40 | 1M | **Cheapest option** |

---

## 📊 Full Comparison Table

| Provider | Model | Input | Output | Context | Free Tier |
|----------|-------|-------|--------|---------|-----------|
| **DeepSeek** | V3.2-Exp | $0.28 ($0.028 cache) | $0.42 | 128K | No |
| **Gemini** | 3 Flash-Lite | $0.10 | $0.40 | 1M | ✅ Yes |
| **Gemini** | 3 Flash | $0.50 | $3.00 | 1M | ✅ Yes |
| **Kimi** | K2.5 | $0.60 ($0.10 cache) | $2.50 | 256K | Limited |
| **GLM** | GLM-5 | $1.00 ($0.20 cache) | $3.20 | 200K | No |
| **GLM** | GLM-4.7 | $0.60 | $2.20 | 200K | No |
| **Gemini** | 3 Pro | $2.00 | $12.00 | 1M | No |
| **Gemini** | 2.5 Pro | $1.25 | $10.00 | 2M | No |
| **Gemini** | 2.5 Flash | $0.30 | $2.50 | 1M | ✅ Yes |

---

## 🎯 Recommendations

### Option 1: Maximum Savings
```
→ Gemini 3 Flash-Lite ($0.15/month)
  - FREE tier available
  - 1M context window
  - Multimodal capabilities
  - Google's infrastructure
```

### Option 2: Best Quality/Price
```
→ Keep: Kimi K2.5 for complex reasoning ($0.93/month)
→ Swap GLM-5 → Gemini 3 Flash-Lite ($0.15/month)
  Total: ~$1.08/month (50% savings)
```

### Option 3: Zero Vendor Lock-in (VibePilot Philosophy)
```
→ Keep: GLM-5 (MIT license, open weights) ($1.05/month)
→ Swap Kimi → DeepSeek ($0.21/month)
  Total: ~$1.26/month
  Both have open alternatives, swappable
```

### Option 4: Keep Current (If Budget Allows)
```
→ GLM-5 + Kimi K2.5 (~$2.00/month)
  - Best reasoning quality
  - GLM-5 has lowest hallucination rate
  - Both support complex agent tasks
```

---

## 🔑 API Key Status Check

| Provider | Status | Cost/Month |
|----------|--------|------------|
| DeepSeek | Need API key | ~$0.21 |
| Kimi (Moonshot) | Active | ~$0.93 |
| GLM (Zhipu) | Active | ~$1.05 |
| Gemini | Need API key | ~$0.15-0.88 |

**Action Items:**
1. Get Gemini API key (free tier available)
2. Get DeepSeek API key (cheapest paid option)
3. Keep Kimi and GLM as fallbacks

---

## Notes

- Gemini 3 Flash-Lite is now cheapest at $0.10/M input (beats DeepSeek)
- GLM-5 is pricier but has lowest hallucination rate + fully open-source
- Current stack costs ~$2/month, could cut to ~$0.30 with Gemini
- Context windows: Gemini 3.1 = 1M, GLM-5 = 200K, Kimi = 256K

---

**File created for review by GLM-5 and human**
