# Kimi API Vision Pricing & Limits Analysis

**Date:** 2026-02-18  
**Researcher:** Kimi (System Researcher)  
**Tag:** VET (Council review - affects routing/cost strategy)  
**Priority:** HIGH  

---

## Executive Summary

Kimi API supports vision (images + video) at competitive pricing. For dashboard/app testing use case (screenshots), Kimi K2.5 offers excellent value at $0.60/M input tokens with 256K context and vision capabilities.

**Key Finding:** For visual testing workflows, Kimi API is cost-competitive with alternatives while offering larger context windows.

---

## Kimi API Pricing (Vision + Text)

### Kimi K2.5 (Recommended for Vision Tasks)

**Capabilities:**
- ✅ **Vision:** Images + Video understanding
- ✅ **Context:** 256K tokens
- ✅ **Reasoning:** Deep thinking mode available
- ✅ **Tool Use:** ToolCalls, JSON Mode
- ✅ **Caching:** Automatic context caching

**Pricing (per 1 Million tokens):**

| Token Type | Price | Notes |
|------------|-------|-------|
| **Input (cache miss)** | $0.60 | First time seeing content |
| **Input (cache hit)** | $0.06 | 90% discount for repeated content |
| **Output** | $3.00 | Generated response |
| **Vision** | Same as text | No extra charge for images! |

**Context Length:**
- Standard: 256K tokens
- Long context: ~200K words equivalent
- Can handle multiple screenshots + code + conversation history

---

## Vision Token Calculation

### How Images Are Charged

**Dynamic token calculation based on:**
- Image resolution
- Format (png, jpeg, webp, gif)
- **Not flat rate** - higher res = more tokens

**Estimated costs for common screenshots:**

| Screenshot Type | Resolution | Tokens | Cost |
|----------------|------------|---------|------|
| **Small mobile** | 750×1334 | ~500-1K | $0.0003-0.0006 |
| **Standard desktop** | 1920×1080 | ~2K-4K | $0.0012-0.0024 |
| **Large 4K** | 3840×2160 | ~8K-15K | $0.0048-0.009 |
| **Video (1 sec)** | 1920×1080 | ~5K-10K | $0.003-0.006 |

**Note:** These are estimates. Use the `estimate tokens API` for exact calculation.

### Cost Example: Dashboard Testing

**Scenario:** Testing a dashboard UI
- 3 screenshots (1920×1080 each)
- Analysis prompt (500 tokens)
- Expected response (2,000 tokens)

**Calculation:**
```
Input:
  3 screenshots × 3,000 tokens = 9,000 tokens
  Text prompt                   = 500 tokens
  Total input                   = 9,500 tokens
  Cost: 9,500 × $0.60/M = $0.0057

Output:
  Response = 2,000 tokens
  Cost: 2,000 × $3.00/M = $0.006

Total per test: ~$0.012
```

**Volume scenarios:**
- 100 tests/day = $1.20/day = $36/month
- 1,000 tests/day = $12/day = $360/month

---

## Rate Limits (Scalable)

**Limits scale with account tier (cumulative recharge):**

| Tier | Min Deposit | Concurrent | RPM | TPM |
|------|-------------|------------|-----|-----|
| **Tier 1** | $10 | 50 | 200 | 200K |
| **Tier 2** | $50 | 100 | 500 | 500K |
| **Tier 3** | $100 | 200 | 1,000 | 1M |
| **Tier 4** | $500 | 500 | 5,000 | 5M |
| **Tier 5** | $3,000 | 1,000 | 10,000 | 10M |

**For dashboard testing:**
- Tier 1 ($10): 200 RPM = 3.3 requests/second
- More than enough for sequential testing
- Can handle parallel testing with higher tiers

---

## Comparison with Alternatives

### Vision API Pricing Comparison

| Provider | Model | Input | Output | Context | Vision |
|----------|-------|-------|--------|---------|--------|
| **Kimi** | K2.5 | $0.60/M | $3.00/M | 256K | ✅ |
| **OpenAI** | GPT-4o | $2.50/M | $10.00/M | 128K | ✅ |
| **OpenAI** | GPT-4o Mini | $0.15/M | $0.60/M | 128K | ✅ |
| **Google** | Gemini 1.5 Pro | $1.25/M | $5.00/M | 1M | ✅ |
| **Google** | Gemini Flash | $0.075/M | $0.30/M | 1M | ✅ |
| **Anthropic** | Claude 3.5 Sonnet | $3.00/M | $15.00/M | 200K | ✅ |
| **Anthropic** | Claude 3 Haiku | $0.25/M | $1.25/M | 200K | ✅ |

### Cost for Dashboard Screenshot Analysis

**Test case:** 3 screenshots + analysis (12K input tokens, 2K output)

| Provider | Model | Cost per Test | Cost per 1000 Tests |
|----------|-------|---------------|---------------------|
| **Kimi K2.5** | Best quality | $0.0132 | $13.20 |
| **GPT-4o Mini** | Budget | $0.0030 | $3.00 |
| **GPT-4o** | Premium | $0.0500 | $50.00 |
| **Gemini Flash** | Best value | $0.0015 | $1.50 |
| **Gemini Pro** | Large context | $0.0250 | $25.00 |
| **Claude Haiku** | Fast | $0.0055 | $5.50 |
| **Claude Sonnet** | Best quality | $0.0660 | $66.00 |

### Analysis

**Best Value for Dashboard Testing:**
1. **Gemini Flash** - $1.50/1K tests (1M context!)
2. **GPT-4o Mini** - $3.00/1K tests (reliable)
3. **Kimi K2.5** - $13.20/1K tests (excellent quality)
4. **Claude Haiku** - $5.50/1K tests (speed)

**Trade-offs:**
- **Gemini Flash:** Cheapest, huge context, but newer/less proven
- **GPT-4o Mini:** Reliable, good enough for most UI testing
- **Kimi K2.5:** Best reasoning, code understanding, mid-range price
- **Claude Sonnet:** Premium quality, expensive

---

## Browser Use + Vision API Strategy

### Recommended Architecture

```
Browser Use (Playwright)
    ↓
Takes screenshots of app/dashboard
    ↓
Vision API (Kimi/Gemini/Claude)
    ↓
Analyzes screenshots
    ↓
Returns: Issues found, suggestions, verification
    ↓
Supabase storage of results
```

### Cost-Optimized Routing

**Tier 1: Quick Checks (High Volume)**
- **Model:** Gemini Flash ($0.075/M input)
- **Use for:** Basic UI validation, element presence
- **Cost:** ~$0.0015 per screenshot analysis

**Tier 2: Standard Testing (Medium Volume)**
- **Model:** Kimi K2.5 ($0.60/M input)
- **Use for:** Functional testing, user flows
- **Cost:** ~$0.013 per screenshot analysis

**Tier 3: Complex Analysis (Low Volume)**
- **Model:** Claude 3.5 Sonnet ($3.00/M input)
- **Use for:** Complex bug investigation, architecture review
- **Cost:** ~$0.066 per screenshot analysis

### Caching Strategy

**Kimi's Automatic Caching:**
- 90% discount on repeated content
- Same dashboard layout = cached
- Only new/changed elements charged full price

**Example benefit:**
```
Test 1 (full analysis):     $0.013
Test 2 (same page, cached): $0.0013 (90% savings)
Test 3 (minor change):      $0.005 (partial cache hit)
```

---

## Implementation for VibePilot

### Configuration

```yaml
# vibepilot.yaml - Vision Testing Configuration
vision_testing:
  enabled: true
  providers:
    primary:
      name: "kimi-k2.5"
      input_cost: 0.60  # per 1M tokens
      output_cost: 3.00
      context: 256000
      supports_vision: true
      rpm: 200  # tier 1
      
    budget:
      name: "gemini-flash"
      input_cost: 0.075
      output_cost: 0.30
      context: 1000000
      supports_vision: true
      
    premium:
      name: "claude-3-5-sonnet"
      input_cost: 3.00
      output_cost: 15.00
      context: 200000
      supports_vision: true
      
  routing:
    quick_check: "gemini-flash"      # $0.0015/test
    standard: "kimi-k2.5"             # $0.013/test
    complex: "claude-3-5-sonnet"      # $0.066/test
    
  caching:
    enabled: true
    similar_threshold: 0.95  # 95% similarity = cache hit
    ttl_hours: 24
```

### Cost Projections

**Monthly Testing Scenarios:**

| Volume | Mix | Primary Model | Estimated Cost |
|--------|-----|---------------|----------------|
| **1,000 tests** | 70% quick, 30% standard | Gemini + Kimi | ~$20/month |
| **10,000 tests** | 70% quick, 30% standard | Gemini + Kimi | ~$200/month |
| **50,000 tests** | 80/15/5 split | All three | ~$800/month |

**Break-even Analysis:**
- Kimi CLI subscription: $19/month
- Kimi API for 1,500 vision tests: ~$19/month
- **Decision point:** >1,500 tests/month → API cheaper

---

## Recommendations

### For VibePilot Dashboard Testing

**Phase 1: Start with Kimi API**
- ✅ Best balance of quality and cost
- ✅ 256K context handles full dashboards
- ✅ Automatic caching saves money
- ✅ Supports video for interaction testing
- ✅ Scalable limits as volume grows

**Phase 2: Add Gemini Flash for Volume**
- When testing >10,000 screenshots/month
- Use Gemini Flash for quick validations
- Keep Kimi K2.5 for complex analysis
- 10x cost reduction on high-volume tasks

**Phase 3: Multi-Provider Redundancy**
- Implement all three providers
- Route by task complexity
- Failover if one provider down
- Optimize cost per task type

### Tag: VET

**Council Review Questions:**
1. Should we implement vision API for dashboard testing?
2. Which provider(s) to start with?
3. What's the monthly testing volume threshold?
4. How to handle caching across providers?
5. Privacy implications of sending screenshots to APIs?

### Next Steps

1. **SIMPLE:** Test Kimi API vision with sample dashboard
2. **SIMPLE:** Compare quality vs Gemini Flash
3. **VET:** Privacy review (screenshots contain UI data)
4. **SIMPLE:** Implement tiered routing
5. **VET:** Cost monitoring and alerting

---

## Summary Table

| Use Case | Best Model | Cost/Test | Monthly (1K tests) |
|----------|-----------|-----------|-------------------|
| **Quick validation** | Gemini Flash | $0.0015 | $1.50 |
| **Standard testing** | Kimi K2.5 | $0.013 | $13.20 |
| **Complex analysis** | Claude Sonnet | $0.066 | $66.00 |
| **Budget constraint** | GPT-4o Mini | $0.003 | $3.00 |

**Winner for VibePilot:** Kimi K2.5
- Reasonable cost ($13/1K tests)
- Excellent code understanding
- 256K context for large dashboards
- Automatic caching reduces costs
- Good balance of quality and price

---

*Analysis complete. Ready for Council review on vision API adoption strategy.*
