[Interesting!] Thanks for sharing the X220 system reference. This gives me a clear picture of your current infrastructure and constraints. Based on the key insight about local LLM viability and courier optimization, I'll focus on enhancing the courier architecture for better resource efficiency on constrained hardware like the X220.

Here's what I found and recommend:

## Key Findings from X220 Reference
**Current Constraints:**
- Chrome-based couriers consume significant RAM (~500MB per headless instance + variable for renderers)
- Running >2-3 simultaneous `agent-browser` daemons causes memory pressure on X220's 16GB RAM
- Local LLMs (3B+) are unusable for interactive use (1-3 tokens/sec on CPU-only)
- Embedding models and lightweight STT/TTS (Whisper tiny, Kokoro) remain viable

**Key Opportunity:**
- "Unlimited web platforms handle 90% of courier work" - meaning most courier tasks don't require full browser automation
- Couriers can use direct API models instead of Chrome to save substantial RAM
- Only ~10% of platforms likely require actual browser interaction (those with complex UIs, CAPTCHAs, or non-standard auth)

## Recommended Courier Architecture Optimization
Based on your hardware constraints and the 90% API-compatible insight:

**Core Change:** Implement a hybrid courier system that:
1. **Default to API-first mode** for platforms supporting direct API access (saves ~400-500MB RAM per courier)
2. **Fall back to Chrome/CDP mode** only for platforms requiring browser interaction
3. **Add intelligent platform detection** to automatically choose the optimal mode
4. **Include RAM-aware scaling** to prevent over-subscription on constrained hardware

**Specific Features:**
- **Platform Capability Registry:** Maintain a list of platforms known to work via API (OpenAI, Anthropic, Groq, etc.) vs. those requiring browser
- **Dynamic Mode Switching:** Couriers check platform registry at runtime and select API or Chrome mode
- **Resource Monitoring:** Track per-courier RAM usage and auto-throttle if X220 memory >85% utilized
- **Fallback Queuing:** If API mode fails, seamlessly retry with Chrome mode (transparent to agent)
- **Metrics Dashboard:** Track mode usage, success rates, and RAM savings per platform

## Why This Approach Fits Your Vision
- **Addresses the RAM bottleneck:** Saves ~400-500MB per courier in API mode → allows 4-6 concurrent couriers instead of 2-3 on X220
- **Maintains compatibility:** No loss of functionality; Chrome fallback covers the 10% edge cases
- **Aligns with VibePilot principles:** 
  - Zero lock-in: Still works with any platform (API or web)
  - Modular: API courier and Chrome courier are swappable components
  - Production-grade: Includes monitoring, fallback, and resource safeguards
- **Leverages existing viable tech:** Uses your current faster-whisper (STT) and Edge TTS while avoiding non-viable local LLMs

## Does This Match Your System Optimization Goals?
This focuses purely on making your current courier architecture more efficient within X220 constraints—no new features, just smarter resource use of what you already have.

Would you like me to:
1. Proceed with generating the full PRD for this courier optimization?
2. Adjust the scope (e.g., include specific platform prioritization or add metrics collection details)?
3. Clarify any assumptions before I draft the PRD?

Just say "go ahead" or share any tweaks, and I'll create the PRD ready for your approval.