---
name: universal-rate-limiter
description: "Hard safety net preventing rapid-fire hammering of any provider. Tracks calls per provider in a rolling 60s window, enforces max 10/min via sleep. Works at Hermes agent level and in connector configs."
version: 1.0
---

# Universal Rate Limiter (Provider-Level Safety Net)

## When to Use

- Any agent or system that makes API calls to LLM providers
- Prevents accidental rate limit hammering (15 calls in 1 minute to Gemini, etc.)
- Adds a universal floor: no provider gets more than ~10 requests per 60 seconds regardless of its listed limits
- Normal conversation pacing (1 call per 5-30s) is unaffected

## The Problem

Without a hard cap, a single session can blast 15+ requests to a single provider in under a minute. This triggers 429 rate limits, burning through the provider's RPM budget and freezing ALL subsequent requests (including legitimate ones from other processes) for the remainder of the minute.

Common scenarios:
- Visual agent restart sends 15 sequential screenshot analysis calls to Gemini → hits 15 RPM → everything blocked for 1 minute
- Agent loops through tool calls rapidly (read file → search → terminal → repeat) → same provider gets hit 10+ times in rapid succession
- Researcher scans 30 items → 30 sequential API calls in 2 minutes → provider blocks

## The Fix: Two-Layer Safety Net

### Layer 1: Hermes Agent (run_agent.py)

Added right after every successful API call in the core loop:

```python
# ── Universal rate limiter: max 10 calls per 60s per provider ──
_now = time.time()
_prov = self.provider or "unknown"
if not hasattr(self, '_call_timestamps'):
    self._call_timestamps = {}
if _prov not in self._call_timestamps:
    self._call_timestamps[_prov] = []
self._call_timestamps[_prov].append(_now)
# Keep only last 60s
self._call_timestamps[_prov] = [t for t in self._call_timestamps[_prov] if _now - t < 60]
_recent = len(self._call_timestamps[_prov])
if _recent >= 10:
    _oldest = self._call_timestamps[_prov][0]
    _wait = 60 - (_now - _oldest) + 1  # +1s buffer
    if _wait > 0 and _wait < 30:
        time.sleep(min(_wait, 6))  # max 6s sleep per enforcement
```

**How it works:**
- Maintains a rolling list of call timestamps per provider
- After each call, prunes entries older than 60s
- If 10+ calls exist in the window, calculates how long to wait before the oldest call drops out of the window
- Sleeps up to 6 seconds max per enforcement
- Does NOT affect normal conversation (1 call every 5-30s)

**Placement:** Immediately after the agent-session-logging-hook saves to DB, before the 80% threshold check. This ensures the rate limit is enforced BEFORE any alerting logic runs.

### Layer 2: Connectors.json (Pipeline Level)

Every connector in connectors.json should have a `min_interval_seconds` field:

```json
{
  "id": "gemini-api-general",
  "type": "api",
  "min_interval_seconds": 6,
  "limit_schema": [...]
}
```

This is a documentation/enforcement field that the governor's platform tracker should respect. It means: "no connector sends requests faster than one every 6 seconds regardless of its higher rate limits."

### Layer 3: Visual QA / Vision Models

Switch vision auxiliary to a provider with a SEPARATE rate limit pool from the primary text provider. This prevents vision tasks from consuming the text provider's RPM budget.

**Default pattern (bad):**
```yaml
vision:
  model: models/gemini-2.5-flash
  provider: gemini
```

**Fixed pattern (good):**
```yaml
vision:
  model: google/gemma-4-31b-it:free
  provider: openrouter
```

## Provider Rate Limits (Verified May 2026)

| Provider | RPM | RPD | Notes |
|----------|-----|-----|-------|
| Gemini (per key) | 15 | 1,500 | 4 keys = 60 RPM total. Shared across ALL Gemini models |
| OpenRouter (free, funded) | 20 per model | 1,000 | Funded = purchased 10+ credits. Separate pool per model |
| Groq (per model) | 30 | 1K-14K | Per-model, NOT org-wide. llama-3.3-70b: 1K RPD, 100K TPD |
| NVIDIA | 10 | 1,000 | No credit limit. Rate-limited only |
| DeepSeek API | server capacity | N/A | Credit-based ($), not RPM-limited |

## Implementation Notes

- The Hermes agent limiter uses an instance attribute (`_call_timestamps`) so it persists across a single session. A new session starts fresh.
- Sleep is capped at 6s to prevent pathological waits. If the limiter triggers repeatedly, the agent will naturally space calls to ~6-10s apart.
- The limiter only tracks the CURRENT session. If the session is long (90+ turns), the limiter ensures spacing throughout.
- For the pipeline, the governor's UsageTracker already has rate limit awareness via `CanMakeRequest`. The `min_interval_seconds` in connectors.json is an additional floor.

## Verification

```bash
# Check connector min_interval_seconds
grep "min_interval_seconds" ~/vibepilot/governor/config/connectors.json

# Verify Hermes rate limiter is installed
grep -A15 "Universal rate limiter" ~/.hermes/hermes-agent/run_agent.py

# Check that speed is throttled: rapid calls should space to 6s apart
```
