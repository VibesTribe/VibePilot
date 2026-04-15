# Voice Interface: TTS/STT Options for VibePilot

**Date:** 2026-02-20  
**Purpose:** Voice stack options for "Talk to Vibes" feature  
**Status:** For review

---

## Voice Pipeline Overview

```
User Voice → [STT] → Text → [LLM] → Response → [TTS] → Voice Output
             Deepgram   DeepSeek/       Kokoro/
             or Whisper GLM/Kimi         F5-TTS
```

---

## 🎙️ STT (Speech-to-Text) Options

### 1. Deepgram (RECOMMENDED for Hetzner setup)
| Feature | Details |
|---------|---------|
| **Cost** | $0.0043/minute (200 min FREE tier) |
| **Free Tier** | 200 minutes/month |
| **Latency** | ~200-300ms |
| **Quality** | Excellent, best price/performance |
| **Hetzner 1 vCPU?** | ✅ Yes (API call, no local compute) |

**Monthly cost for 500 min (100 queries/day × 10 sec × 30 days):**
- First 200 min: FREE
- Remaining 300 min: 300 × $0.0043 = **~$1.30/month**

---

### 2. Self-Hosted Whisper (NOT recommended for 1 vCPU)
| Feature | Details |
|---------|---------|
| **Cost** | FREE (self-hosted) |
| **Hardware** | Needs 4GB+ RAM, GPU recommended |
| **Hetzner 1 vCPU?** | ❌ Too slow (5-10 sec transcription) |
| **Latency** | 5-10 seconds on CPU |
| **Best for** | Privacy-focused, high-resource setups |

---

### 3. Whisper API (OpenAI)
| Feature | Details |
|---------|---------|
| **Cost** | $0.006/minute |
| **No free tier** | Paid from first request |
| **Quality** | Very good, multilingual |

---

### 4. Google Speech-to-Text (Chirp)
| Feature | Details |
|---------|---------|
| **Cost** | ~$0.006/minute |
| **Languages** | 125+ |
| **Integration** | Good if using Gemini LLM |

---

## 🔊 TTS (Text-to-Speech) Options

### 1. Kokoro (RECOMMENDED - Self-Hosted)
| Feature | Details |
|---------|---------|
| **Cost** | ~$0 (self-hosted on Hetzner) |
| **Model size** | 82M parameters |
| **Speed** | 5x real-time on CPU |
| **Quality** | ELO 1,059 (#9 on leaderboard) |
| **Hetzner 1 vCPU?** | ✅ Yes, runs fine on CPU |
| **Languages** | English, French, Korean, Japanese, Mandarin |

**Hosting options:**
- Self-host on Hetzner: FREE (just VPS cost)
- Railway/Easypanel: Free tier available
- Kokoro Web (hosted): FREE at voice-generator.pages.dev

---

### 2. F5-TTS (Alternative - Voice Cloning)
| Feature | Details |
|---------|---------|
| **Cost** | FREE (open source) |
| **Model size** | 335M parameters |
| **Feature** | Zero-shot voice cloning (10 sec sample) |
| **Speed** | 0.15 real-time factor |
| **License** | MIT |
| **Best for** | Custom voice cloning, podcast generation |

---

### 3. Gemini TTS (Native)
| Feature | Details |
|---------|---------|
| **Cost** | Flash: $0.50 input / $10 output per 1M tokens |
| **Pros** | Native integration if using Gemini LLM |
| **Cons** | More expensive than Kokoro |

---

### 4. Inworld TTS (Best Quality)
| Feature | Details |
|---------|---------|
| **Cost** | $5-10 per 1M characters |
| **Quality** | #1 on Artificial Analysis (ELO 1,160) |
| **Latency** | <250ms |
| **Best for** | Production voice agents |
| **Too expensive?** | Maybe for bootstrap phase |

---

### 5. ElevenLabs (Industry Standard)
| Feature | Details |
|---------|---------|
| **Cost** | $103-206 per 1M characters |
| **Quality** | Excellent |
| **Verdict** | ❌ Too expensive for current budget |

---

## 💰 Recommended Voice Stack (Broke Founder Edition)

**For Hetzner CX11 (1 vCPU, 2GB RAM):**

| Component | Choice | Monthly Cost |
|-----------|--------|--------------|
| **STT** | Deepgram API | ~$1.30 (after free tier) |
| **LLM** | DeepSeek API | ~$0.21 |
| **TTS** | Kokoro (self-hosted) | $0 |
| **VPS** | Hetzner CX11 | €3.79 (~$4) |
| **TOTAL** | | **~$5.50/month** |

---

## 🚀 Even Cheaper Options

### Option A: Use All Free Tiers
| Component | Choice | Monthly Cost |
|-----------|--------|--------------|
| **STT** | Deepgram (200 min free) | $0 |
| **LLM** | Gemini 3 Flash (free tier) | $0 |
| **TTS** | Kokoro Web (hosted) | $0 |
| **VPS** | Hetzner CX11 | ~$4 |
| **TOTAL** | | **~$4/month** |

**Limitations:** Rate limits apply, may hit limits at scale

---

### Option B: Ultra-Minimal (Text-Only First)
| Component | Choice | Monthly Cost |
|-----------|--------|--------------|
| **STT** | Skip voice, text only | $0 |
| **LLM** | DeepSeek or Gemini | $0-0.21 |
| **TTS** | Skip voice, text only | $0 |
| **VPS** | Hetzner CX11 | ~$4 |
| **TOTAL** | | **~$4-4.20/month** |

**Strategy:** Add voice later when you have revenue

---

## 🔧 Implementation Notes

### Kokoro Self-Host on Hetzner
```bash
# One command deployment
docker run -d -p 8888:8888 \
  ghcr.io/eduardolat/kokoro-web:latest

# Test
curl http://localhost:8888/tts \
  -d '{"text":"Hello from VibePilot"}'
```

### Deepgram Integration
```javascript
// In Cloudflare Worker or Hetzner
const deepgramRes = await fetch('https://api.deepgram.com/v1/listen', {
  method: 'POST',
  headers: { 'Authorization': `Token ${env.DEEPGRAM_KEY}` },
  body: audioBlob
});
```

---

## 📝 Action Items

1. **Get Deepgram API key** (200 min free tier)
2. **Deploy Kokoro** on Hetzner (one Docker command)
3. **Test voice flow** end-to-end
4. **Monitor usage** - switch to paid Deepgram only after 200 min
5. **Keep text fallback** - voice should enhance, not replace UI

---

## Summary

| Setup | Monthly Cost | Voice Quality | Best For |
|-------|-------------|---------------|----------|
| **Full Voice** (Deepgram + Kokoro + DeepSeek) | ~$5.50 | Good | Production voice interface |
| **Free Tier Only** | ~$4 | Good | Testing, low volume |
| **Text-Only** | ~$4 | N/A | Bootstrap phase, add voice later |

**Recommendation:** Start with Deepgram (free tier) + Kokoro (self-hosted) + DeepSeek LLM. Total ~$5.50/month for full voice capability.

---

**File created for review by GLM-5 and human**
