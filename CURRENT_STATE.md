# VibePilot Current State - 2026-04-14

## Status: Infrastructure Optimized, Research Phase

### What's Running
- **Governator:** systemd user service, running since April 7, active
- **Cloudflared tunnel:** live at vibestribe.rocks, sacred (don't touch)
- **Hermes agent:** accessible via dashboard chat through tunnel
- **Chrome CDP:** port 9222 for browser automation
- **TTS:** edge-tts (fast, free, no changes needed)

### Hardware: ThinkPad X220
- Intel i5-2520M (no AVX2, no GPU)
- 16GB RAM (~10GB available)
- 781GB disk free
- Phone WiFi tethered

### What Changed This Session (April 14)
- **Ollama:** installed v0.20.4, daemon stopped/disabled. Tested qwen3:4b and qwen3-vl:4b -- too slow (2 tok/s) for real work. Cleaned out. Ready to pull models when landscape shifts.
- **Kokoro TTS:** removed (9GB freed). Edge-tts is better for this hardware.
- **Free model research:** verified 7 free API providers. Full rolodex in `research/2026-04-14-free-model-rolodex.md`.
- **GitHub PAT:** rotated (done in earlier session).

### Key Decisions
1. **No local models** -- x220 can't run useful inference. Cloud free tiers are the path.
2. **Edge-tts only** -- fastest free option, no reason to change.
3. **RAM for agents, not models** -- parallel agent sessions are the priority.
4. **Multiple free providers** -- cascade of Groq/Google/OpenRouter/SambaNova, never single-vendor dependency.
5. **Real usage decides spending** -- run tasks on free tiers first, data tells where $10 credit is worth it.

---

## Verified Free API Providers (April 2026)

| Provider | Card Needed | Best Free Models | Rate Limits |
|---|---|---|---|
| OpenRouter | NO | 24 free models, $0 cap | 50-1000 RPD |
| Groq | NO | qwen3-32b, llama-4-scout, gpt-oss | 30 RPM, 100-500K TPD |
| Google AI Studio | NO | Gemini 2.5 Flash, Gemma 4 | ~15 RPM |
| SambaNova | NO | DeepSeek-V3.1, Llama-4-Maverick | 20 RPD |
| NVIDIA NIM | NO | Nemotron 3 Super | Trial access |
| SiliconFlow | NO (real-name) | Qwen, GLM, DeepSeek | 1000-10000 RPM |
| HuggingFace | NO | Thousands of models | Varies |

**Status:** Only Google AI Studio key exists. Need to sign up for Groq, SambaNova, NVIDIA NIM.

---

## Repository State

**Branch:** `research-update-april2026` tracking origin
**Recent commits:**
- `9dd81ae9` - research: verified free model rolodex
- `8b06a2c3` - research: JourneyKits landscape analysis

**Key files:**
- `research/2026-04-14-free-model-rolodex.md` - Verified free providers + cascade plan
- `research/2026-04-08-journeykits-landscape-analysis.md` - 95-kit gap analysis
- `VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md` - Architecture bible (needs update)
- `governor/` - Go governor source
- `governor/config/` - JSON configs (models.json, connectors.json, routing.json, etc.)

**Dashboard:** https://vibeflow-dashboard.vercel.app/ (sacred, deployed from ~/vibeflow)

---

## On Disk (relevant)

| Path | Size | Purpose |
|---|---|---|
| ~/VibePilot/ | 165MB | Go governor + research |
| ~/vibeflow/ | 173MB | Dashboard (Vercel auto-deploy) |
| ~/vibepilot-server/ | 60KB | Restart scripts |
| ~/browser-use-env/ | 429MB | Browser Use (Playwright + Chrome CDP) |

**Stopped/disabled:**
- Ollama daemon (stopped, disabled, ready if needed)
- No local models pulled

---

## Next Steps

1. **Get free API keys** for Groq, SambaNova, NVIDIA NIM (user signing up)
2. **Build cascade into models.json** with verified providers
3. **Wire cascade into governor** routing logic
4. **Run real tasks** through cascade to learn what works
5. **Update VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md** -- still references old state

---

## How to Start Governor

```bash
# Check status
systemctl --user status vibepilot-governor

# View logs
journalctl --user -u vibepilot-governor -f

# Restart
systemctl --user restart vibepilot-governor

# Bootstrap credentials in:
# ~/.config/systemd/user/vibepilot-governor.service.d/override.conf
```

---

**Last Updated:** 2026-04-14
