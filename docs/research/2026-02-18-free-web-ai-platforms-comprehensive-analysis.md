# Free Web AI Platforms: Comprehensive Analysis

**Date:** 2026-02-18  
**Researcher:** Kimi (System Researcher)  
**Tag:** VET (Council review - affects routing strategy)  
**Priority:** CRITICAL  

---

## Executive Summary

Deep analysis of free tier web AI chat platforms for VibePilot courier routing. Covers ChatGPT, Claude, Gemini, DeepSeek, Perplexity, HuggingChat, and emerging platforms.

**Key Finding:** Most platforms have severe free tier limits (10-100 messages/day), requiring sophisticated multi-platform rotation and 80% threshold management.

---

## Platform Analysis

### 1. ChatGPT (chat.openai.com)

**Model Access:**
- GPT-4o (limited)
- GPT-4o-mini (unlimited fallback)

**Free Tier Limits:**
| Metric | Limit | Notes |
|--------|-------|-------|
| **Messages** | 10-50 per 5 hours | Rolling window, varies by demand |
| **Context Window** | ~16,000 tokens | ~12,000 words |
| **Rate** | 2-10 messages/hour average | Highly variable |
| **GPT-4o access** | Limited switches | Falls back to mini when exceeded |

**Practical Calculation:**
- 10 messages / 5 hours = 2 msg/hour average
- 5 hour cooldown is LONG
- Best for: High-quality reasoning tasks, complex code
- Worst for: High-volume work, rapid iterations

**VibePilot Strategy:**
- **Routing Flag:** W (web) for simple tasks, Q (internal) for complex
- **Use sparingly:** Only when quality justifies the 5-hour cooldown
- **Time dependency:** Check "off-peak" hours for higher limits

---

### 2. Claude (claude.ai)

**Model Access:**
- Claude Sonnet (limited)
- Claude Haiku (more generous)

**Free Tier Limits:**
| Metric | Limit | Notes |
|--------|-------|-------|
| **Messages** | ~40 per day | Daily reset (not rolling) |
| **File uploads** | Up to 30MB, 20 files/chat | Generous for document analysis |
| **Images** | Up to 8000x8000px | Good for vision tasks |
| **Context Window** | 200K tokens (Sonnet) | Excellent for large codebases |

**Practical Calculation:**
- 40 messages / 24 hours = ~1.7 msg/hour
- Daily reset at ~same time (varies by first use)
- More predictable than ChatGPT's rolling window

**VibePilot Strategy:**
- **Routing Flag:** W for document analysis, Q for complex reasoning
- **Best for:** Large context tasks (200K window), document understanding
- **Limitation:** 40/day burns fast on multi-step tasks

---

### 3. Gemini (gemini.google.com)

**Model Access:**
- Gemini Pro (limited)
- Gemini Flash (more generous)

**Free Tier Limits:**
| Metric | Limit | Notes |
|--------|-------|-------|
| **Queries** | 100 per day | Hard cap, resets midnight PT |
| **Rate limit** | ~4 per minute | Prevents burst usage |
| **Context Window** | 1M tokens (1,000,000!) | Largest available |
| **Code Assist** | 240 chat/day, 6K completions/day | Separate from main Gemini |

**Current Status:** 
- ⚠️ **QUOTA EXHAUSTED** in our testing
- May require credit card even for free tier now

**Practical Calculation:**
- 100 queries / 24 hours = ~4 per hour
- 1M context window is EXCEPTIONAL for large tasks
- Code Assist limits separate (good for dev tasks)

**VibePilot Strategy:**
- **Routing Flag:** W for massive context tasks (1M tokens!)
- **Best for:** Huge codebase analysis, long document processing
- **Current Issue:** Quota/credit requirements may block usage

---

### 4. DeepSeek (chat.deepseek.com)

**Model Access:**
- DeepSeek-V3 (free web access)
- DeepSeek-R1 (reasoning model, web access)

**Free Tier Limits:**
| Metric | Limit | Notes |
|--------|-------|-------|
| **Messages** | Unlimited? | No explicit daily cap found |
| **Length limit** | ~64K tokens per conversation | "Length limit reached" error |
| **Rate limiting** | Yes (unspecified) | Prevents abuse |
| **Context Window** | 128K tokens (V3 model) | Large but not massive |

**Key Constraint:**
- **"Length limit reached"** - forces new chat at ~64K tokens
- Context management required for long conversations
- Can't continue indefinitely in same thread

**Practical Calculation:**
- Appears to be most generous free tier
- No daily message count found
- 64K conversation limit requires task chunking

**VibePilot Strategy:**
- **Routing Flag:** W for high-volume work
- **Best for:** When other platforms hit limits
- **Note:** Requires credit card for API, web may be truly free

---

### 5. Perplexity (perplexity.ai)

**Model Access:**
- Multiple models via search
- Pro models (limited free access)

**Free Tier Limits:**
| Metric | Limit | Notes |
|--------|-------|-------|
| **Quick searches** | Unlimited | Best/Quick mode |
| **Pro searches** | 5 per day | Advanced features |
| **Deep searches** | ~1 per day | Replenishes slowly |
| **Citations** | Always included | Good for research tasks |

**Practical Calculation:**
- Unlimited quick searches = good for basic Q&A
- 5 Pro/day limits advanced reasoning
- Deep search (1/day) very limited

**VibePilot Strategy:**
- **Routing Flag:** W for research tasks
- **Best for:** Information retrieval, web search integration
- **Limitation:** Not suitable for code generation (search-focused)

---

### 6. HuggingChat (huggingface.co/chat)

**Model Access:**
- Multiple open-source models
- Dynamic model selection (Omni v2)

**Free Tier Limits:**
| Metric | Limit | Notes |
|--------|-------|-------|
| **Guest requests** | 10 per day | No signup required |
| **Registered users** | Higher limits | Exact number not published |
| **Inference credits** | Usage-based | PRO users get 20x more |
| **Models** | 115+ models from 15 providers | Excellent variety |

**Practical Calculation:**
- 10/day as guest = very limited
- Registration required for real usage
- Good model variety for different task types

**VibePilot Strategy:**
- **Routing Flag:** W for model diversity
- **Best for:** Testing different open-source models
- **Limitation:** Guest limits too low, requires accounts

---

### 7. Qwen (qwen.ai)

**Model Access:**
- Qwen2.5-Max
- Qwen-VL (vision)

**Free Tier Limits:**
| Metric | Limit | Notes |
|--------|-------|-------|
| **Web chat** | Appears unlimited | No explicit limits found |
| **Context Window** | 128K tokens | Standard large model |
| **Multimodal** | Images, video, docs | Strong vision capabilities |

**Status:** 
- Less documented than western platforms
- Appears generous but unclear actual limits
- Worth testing for VibePilot

**VibePilot Strategy:**
- **Routing Flag:** W (test for limits)
- **Best for:** Vision tasks, multimodal work
- **Risk:** Undocumented limits may surprise

---

## Comparative Analysis

### Request Limits Summary

| Platform | Daily Limit | Reset Type | Cooldown Strategy |
|----------|-------------|------------|-------------------|
| **ChatGPT** | 10-50 per 5h | Rolling window | 5 hours |
| **Claude** | ~40 per day | Daily | 24 hours |
| **Gemini** | 100 per day | Daily (midnight PT) | 24 hours |
| **DeepSeek** | Unlimited? | N/A | None apparent |
| **Perplexity** | 5 Pro + unlimited Quick | Mixed | Varies |
| **HuggingChat** | 10 (guest) | Daily | 24 hours |
| **Qwen** | Unknown? | Unknown | Unknown |

### Context Windows

| Platform | Context | Best For |
|----------|---------|----------|
| **Gemini** | 1M tokens | Massive documents, huge codebases |
| **Claude** | 200K tokens | Large codebases, long docs |
| **DeepSeek** | 128K tokens | Large context, unlimited volume |
| **Qwen** | 128K tokens | Large context, multimodal |
| **ChatGPT** | 16K tokens (free) | Standard tasks |
| **Claude Free** | 200K (but 40 msg/day) | Quality over quantity |

### Best Use Cases by Platform

| Platform | Best Tasks | Avoid |
|----------|-----------|-------|
| **ChatGPT** | Complex reasoning, GPT-4o quality needs | High volume, quick iterations |
| **Claude** | Document analysis, large context, coding | High frequency (40/day limit) |
| **Gemini** | Massive context (1M!), code completion | Currently quota-blocked |
| **DeepSeek** | High volume work, long conversations | When 64K conv limit breaks flow |
| **Perplexity** | Research, web search, citations | Code generation, creative writing |
| **HuggingChat** | Model comparison, open-source testing | Production workloads (10/day guest) |
| **Qwen** | Vision tasks, multimodal, China-friendly | When limits unknown |

---

## VibePilot Routing Strategy

### Tier 1: High-Volume Workhorses
**For bulk tasks, iterations, testing:**
- DeepSeek (unlimited?)
- Perplexity Quick (unlimited)

### Tier 2: Quality for Complex Tasks  
**For important, complex work:**
- Claude (40/day, 200K context)
- ChatGPT (10-50 per 5h, best reasoning)

### Tier 3: Specialized Capabilities
**For specific needs:**
- Gemini (1M context - when available)
- Perplexity Pro (research with citations)
- Qwen (multimodal/vision)

### Tier 4: Fallback/Testing
**When others fail:**
- HuggingChat (model diversity)
- GPT-4o-mini (ChatGPT fallback)

---

## Critical Findings for VibePilot

### 1. The 80% Threshold Rule is ESSENTIAL

With these tight limits:
- ChatGPT: Alert at 8 messages (of 10)
- Claude: Alert at 32 messages (of 40)
- Gemini: Alert at 80 queries (of 100)

**Auto-rotation MUST trigger before limits to avoid downtime.**

### 2. Rolling vs Daily Resets

- **Rolling (ChatGPT):** Harder to predict, requires continuous tracking
- **Daily (Claude, Gemini):** Predictable, can plan around reset times
- **Strategy:** Mix both types so when one resets, another is available

### 3. Context Window Matters for Routing

- 1M tokens (Gemini) = Can handle entire large codebase
- 200K (Claude) = Large but manageable projects
- 16K (ChatGPT free) = Small tasks only

**Tasks >100K tokens MUST go to Gemini/Claude, no exceptions.**

### 4. The "Unlimited" Platforms

- **DeepSeek:** Appears unlimited but has 64K conversation limit
- **Perplexity Quick:** Truly unlimited but not for complex work
- **Reality:** "Unlimited" often means "generous but undefined"

### 5. Credit Card Walls

- **Gemini:** Now requiring credit card even for free tier
- **DeepSeek API:** Requires credit (but web appears free)
- **Trend:** Platforms tightening free access

---

## Recommendations

### For Implementation (VET Required)

1. **Aggressive 80% Thresholds**
   - All platforms enter cooldown at 80% usage
   - Prevents hitting hard limits
   - Ensures continuous availability

2. **Multi-Platform Rotation**
   - Never rely on single platform
   - Rotate through 4+ platforms continuously
   - Weight by success rate and cost

3. **Context-Aware Routing**
   - Tasks >100K tokens → Gemini/Claude only
   - Tasks 50-100K → Claude preferred
   - Tasks <50K → Any platform by availability

4. **Time-Based Optimization**
   - Track platform reset times
   - Schedule large batches after daily resets
   - Avoid ChatGPT 5-hour windows during peak work

5. **Fallback Cascade**
   ```
   Primary: DeepSeek (unlimited volume)
   Fallback 1: Perplexity Quick (unlimited simple)
   Fallback 2: Claude (quality, 40/day)
   Fallback 3: ChatGPT (best reasoning, 10-50/5h)
   Emergency: HuggingChat (diversity, 10/day)
   ```

### Research Gaps (Need Testing)

1. **DeepSeek actual limits** - Test high-volume usage
2. **Qwen free tier limits** - Unknown boundaries
3. **MiniMax web chat** - New platform, limits unclear
4. **Reset timing accuracy** - When exactly do daily limits reset?
5. **Cooldown recovery patterns** - How fast do platforms recover?

---

## Council Review Questions

**VET Tag - Architecture Impact:**

1. Should we implement **time-based scheduling** (work around daily resets)?
2. How do we handle **context window routing** (auto-detect token count)?
3. Should we **prioritize unlimited platforms** (DeepSeek) for bulk work?
4. What's the **credit card policy** (avoid platforms requiring them)?
5. How do we **test and validate** undocumented limits (DeepSeek, Qwen)?
6. Should we implement **predictive cooldown** (anticipate 5-hour windows)?

---

## Next Steps

1. **SIMPLE:** Update platform registry with these limits
2. **SIMPLE:** Implement 80% threshold alerts in orchestrator
3. **VET:** Design context-aware routing (token counting)
4. **SIMPLE:** Test DeepSeek high-volume claims
5. **SIMPLE:** Document Qwen and MiniMax limits
6. **VET:** Design time-based task scheduling

---

## Sources

- ChatGPT Help Center: Usage limits FAQ
- Reddit r/ClaudeAI: Usage limit discussions
- Google AI Developer docs: Gemini rate limits
- DeepSeek API docs: Rate limit notes
- Perplexity Help Center: Plan comparisons
- HuggingFace discussions: HuggingChat limits
- Various AI blogs and comparison sites (2024-2025)

---

**Tag: VET - Council Review Required**

This analysis affects core routing strategy, threshold management, and platform selection algorithms. Recommend Council review before implementation.

---

*Research complete. Ready for Council review and implementation planning.*
