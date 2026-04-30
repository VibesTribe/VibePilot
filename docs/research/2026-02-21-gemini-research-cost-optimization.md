# Gemini Research: Cost Optimization & Architecture Alternatives

**Date:** 2026-02-21
**Source:** Gemini AI consultation session
**Context:** GCE costing $32/2-weeks, Kimi quota exhausted, only GLM-5 operational

---

## Executive Summary

Gemini validated VibePilot's architecture and provided cost optimization path:
1. Move browser to Remote CDP (save 800MB RAM)
2. Add Gemini CLI as second agent (free tier: 1000 req/day)
3. Downgrade GCE to e2-micro (free tier)
4. Use Cloudflare Workers for audio (zero host RAM)

---

## Key Recommendations

### 1. Remote CDP for Browser-Use (HIGH PRIORITY)

**Problem:** Browser-use requires ~800MB RAM to run Chromium
**Solution:** Connect to cloud browser via Chrome DevTools Protocol

```python
from browser_use import Agent, Browser, BrowserConfig

browser = Browser(
    config=BrowserConfig(
        cdp_url="wss://connect.browserbase.com?token=YOUR_FREE_TOKEN"
    )
)
```

**Providers with free tiers:**
- Browserbase
- Steel.dev

**Impact:** Moves browser rendering to cloud, GCE uses only ~50MB for JSON commands

### 2. Gemini CLI as Second Agent

**Why Gemini CLI (@google/gemini-cli):**
- 1,000 free requests/day
- Apache 2.0 licensed
- Native MCP support (GitHub, Supabase)
- Non-interactive mode for systemd services

**Usage pattern:**
```bash
# Check quota before running
gemini --stats

# Non-interactive task execution
gemini "Process the task in AGENTS.md using tools in SKILLS.json"
```

**Safety valve:** Cloudflare AI Gateway can auto-route to Groq/Llama if Gemini 429s

### 3. Cost Optimization Path

| Current | Optimized | Savings |
|---------|-----------|---------|
| GCE e2-standard (~$64/mo) | GCE e2-micro (free) | $64/mo |
| Local browser (800MB RAM) | Remote CDP (50MB) | 750MB RAM |
| Kimi subscription ($20/mo) | Gemini CLI (free) | $20/mo |

**Alternative hosting (if need more RAM):**
- Hetzner CAX11: $4/mo, 4GB RAM

### 4. Cloudflare Workers for Audio

**TTS Option:** Kokoro-82M on Cloudflare Workers
- 82M parameters = lightweight
- ONNX Runtime compatible
- $0 cost, zero host RAM

**STT Option:** Deepgram
- Industry standard for speed
- Dashboard sends audio directly to Deepgram
- Server only receives text

---

## Architecture: "No-Lock-In" Validation

Gemini confirmed VibePilot's design aligns with 2026 best practices:

### Our Current Architecture (Already Correct!)

| Component | VibePilot Implementation | Best Practice Match |
|-----------|-------------------------|---------------------|
| Agent definitions | `AGENTS.md` + JSON | ✅ OpenCode standard |
| Tool schemas | JSON in Supabase | ✅ Universal tool manifest |
| Provider switching | `access` table + RUNNER_REGISTRY | ✅ Provider-agnostic |
| State persistence | Supabase (GSON/JSON) | ✅ Hydratable state |
| Usage limits | UsageTracker with 80% threshold | ✅ Safety valve |

### What We Already Have

1. **Swappable LLMs:** `models` table + `access` table = provider-agnostic
2. **Markdown agents:** `AGENTS.md` for role definitions
3. **JSON tools:** Tool schemas in database
4. **Safety valve:** 80% usage threshold with cooldown

**Gemini's insight:** "Your Python attempts aren't bad, they're just Heavy."

---

## The "Claw" Frameworks (Research Only)

| Framework | Purpose | Consideration |
|-----------|---------|---------------|
| **ZeroClaw** | 3.4MB Rust binary, trait-based providers | v2 rewrite candidate |
| **IronClaw** | WASM sandboxing for credential safety | Security enhancement |
| **NanoClaw** | Specialized swarms, ephemeral containers | Agent orchestration pattern |

**Decision:** Not for current implementation. Python works, just needs optimization.

---

## Implementation Priority

### Immediate (This Week)
1. Test Gemini CLI as second agent
2. Research Remote CDP providers (Browserbase, Steel.dev)
3. Wait for Gemini API quota reset

### Near-Term
4. Implement Remote CDP for courier/browser tasks
5. Set up Cloudflare Worker for Kokoro TTS
6. Downgrade GCE to e2-micro

### Future (v2)
7. Consider Rust rewrite for core orchestrator
8. Add IronClaw-style credential sandboxing

---

## Technical Notes

### GSON Lenient Parsing
If using GSON/JSON parsing, enable lenient mode:
```java
Gson gson = new GsonBuilder().setLenient().create();
```
Prevents 90% of LLM-induced parsing failures.

### OpenCode Configuration
For Remote CDP, update `opencode.json`:
```json
{
  "browser": {
    "provider": "remote-cdp",
    "cdp_url": "wss://..."
  }
}
```

---

## Key Insight

> "Since you already have the systemd orchestrator, the question isn't whether to rebuild - it's how to optimize what works."

VibePilot's architecture is sound. The goal is:
1. Add more providers (Gemini CLI)
2. Move heavy operations off GCE (Remote CDP)
3. Reduce costs (e2-micro)
