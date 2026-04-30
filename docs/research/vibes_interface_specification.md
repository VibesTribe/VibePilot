# Vibes Interface Specification
## Mobile-First Conversational AI for VibePilot

**Date:** 2026-02-20  
**Status:** Research & Design Phase  
**Dashboard:** https://vibeflow-dashboard.vercel.app/ (already mobile-responsive ✓)

---

## Executive Summary

Vibes is the human-facing AI assistant for VibePilot - accessible from anywhere, especially mobile. The interface supports both **voice** (tap-and-talk) and **text** (chat) modes, giving the human complete flexibility in how they interact with the system.

**Key Innovation:** Instead of opening a terminal to check on projects, you simply open the dashboard on your phone and ask Vibes: *"How's the social platform project going?"* or *"What's our ROI this week?"*

---

## Current State Analysis

### What's Already Built ✅

| Component | Status | Location |
|-----------|--------|----------|
| Vibes Prompt | ✅ Complete | `config/prompts/vibes.md` |
| Voice Interface Architecture | ✅ Documented | `docs/voice_interface.md` |
| Dashboard Base | ✅ Mobile-responsive | `vibeflow/apps/dashboard/` |
| VibesMissionControl | ✅ Exists | `components/VibesMissionControl.tsx` |
| Voice Intent Detection | ✅ Basic | `voice/voice.ts` |
| Supabase Query Function | ⏳ Planned | `vibes_query()` RPC |

### Dashboard Mobile Status
- **Viewport:** Already configured for mobile (`width=device-width`)
- **Responsive Styles:** `@media` queries exist for breakpoints
- **Mobile Navigation:** `mission-mobile-nav` class exists
- **Touch-Friendly:** UI elements sized for touch interaction

**Verification:** https://vibeflow-dashboard.vercel.app/ loads and works on phone ✓

---

## User Experience Design

### Primary Interface: Sticky Header with Vibes

Vibes lives in the **top-left of a FIXED Mission Control header** that stays visible while scrolling through projects. This is the optimal design because:

- ✅ **Always visible** - Header stays fixed while content scrolls
- ✅ **Vibes always accessible** - No scrolling up to find it
- ✅ **Key data visible** - Status, tokens, ROI always in view
- ✅ **First thing you see** - Establishes Vibes as the primary interface
- ✅ **Consistent placement** - Same location on desktop and mobile

```
┌─────────────────────────────────────┐ ← Fixed Header (always visible)
│ 🤖 Vibes  Mission Control  [Tokens] │    
│     ↑         [Status]     [ROI]    │
│  Tap to                             │
│  talk                               │
├─────────────────────────────────────┤ ← Scrollable content starts here
│                                     │
│    [Project Slice 1]                │
│                                     │
│    [Project Slice 2]                │
│                                     │
│    [Project Slice 3...]             │
│    (scrolls while header            │
│     stays fixed)                    │
│                                     │
└─────────────────────────────────────┘
```

**Header Contents (Fixed):**
- **Left:** Vibes orb + "Text me" micro-label
- **Center:** Mission Control title + status summary
- **Right:** Key metrics (Tokens, ROI) + menu button

**Interaction:**
- **Tap Vibes icon** → Opens voice interface overlay
- **Tap "Text me"** (small text below orb) → Chat panel slides in
- **Pulsing glow** when Vibes has a proactive message
- **Badge counter** for unread notifications
- **Scroll content** → Header stays fixed at top

### Interaction Modes

#### Mode 1: Voice-First (Tap & Talk)

**Flow:**
```
1. Tap Vibes button → Voice interface opens
2. Hold to talk → Visual waveform animates
3. Release → Audio sent to processing
4. Vibes responds → Audio plays back
5. Optional: Transcript appears in chat history
```

**Visual States:**
- **Idle:** Mic icon, subtle pulse
- **Listening:** Expanding circular waveform, "Listening..."
- **Processing:** Spinner, "Thinking..."
- **Responding:** Speaker waves, "Vibes is speaking..."

#### Mode 2: Text Chat

**Access Methods:**
1. **"Text me" link** below Vibes icon in header
2. **Swipe up** on voice interface overlay
3. **Keyboard icon** in voice interface
4. **Long-press** Vibes icon (mobile shortcut to text)

**Chat Interface:**
```
┌─────────────────────────────────────┐
│  💬 Vibes                    [✕]   │  ← Header with close
├─────────────────────────────────────┤
│                                     │
│  Vibes: How can I help you today?  │
│                                     │
│  You: What's the ROI on Project X? │
│                                     │
│  Vibes: [Response with data]       │
│                                     │
├─────────────────────────────────────┤
│  [🎤] Type your message... [Send]  │  ← Input area
└─────────────────────────────────────┘
```

**Features:**
- Persistent chat history (stored in Supabase)
- Tap-to-speak button in text mode
- Rich responses (cards, charts, links)
- Pull to refresh

### Proactive Notifications

Vibes can reach out to the human:

```
┌─────────────────────────────────────┐
│  🤖 Vibes has an update            │  ← Push-style notification
│  "Daily briefing ready"    [View]  │
└─────────────────────────────────────┘
```

**Triggers:**
- Daily digest ready (morning)
- Task completed
- Credit/subscription alert
- Error requiring attention
- Council decision needed

---

## Technical Architecture

### High-Level Flow

```
┌─────────────┐     Voice/Text      ┌─────────────┐
│   Human     │◄──────Input────────►│  Dashboard  │
│  (Mobile)   │                     │  (Browser)  │
└──────┬──────┘                     └──────┬──────┘
       │                                   │
       │  ┌─────────────────────────────┐  │
       │  │      WebRTC (Voice)         │  │  ← Real-time audio streaming
       │  │      or HTTP (Text)         │  │
       │  └─────────────────────────────┘  │
       │                                   │
       └──────────┬────────────────────────┘
                  │
                  ▼
         ┌─────────────────┐
         │  Cloudflare     │  ← Edge processing (free tier: 100k req/day)
         │  Worker         │
         └────────┬────────┘
                  │
       ┌──────────┼──────────┐
       │          │          │
       ▼          ▼          ▼
┌──────────┐ ┌────────┐ ┌──────────┐
│ Deepgram │ │DeepSeek│ │  Kokoro  │
│   (STT)  │ │ (LLM)  │ │  (TTS)   │
└────┬─────┘ └────┬───┘ └────┬─────┘
     │            │          │
     └────────────┼──────────┘
                  │
                  ▼
         ┌─────────────────┐
         │    Supabase     │  ← Context & memory
         │  (vibes_query)  │
         └─────────────────┘
```

### Component Breakdown

#### 1. Frontend (Dashboard)

**New Files:**
```
apps/dashboard/
├── components/
│   ├── vibes/
│   │   ├── VibesHeaderButton.tsx    # Header-integrated Vibes button
│   │   ├── VibesVoiceModal.tsx      # Voice interface overlay
│   │   ├── VibesChatPanel.tsx       # Text chat interface (slide-out)
│   │   ├── VibesWaveform.tsx        # Audio visualization
│   │   └── VibesNotification.tsx    # Proactive message banner
│   └── MissionHeader.tsx            # Update to integrate Vibes
├── hooks/
│   └── useVibes.ts                  # Main Vibes interaction hook
├── lib/
│   └── vibesApi.ts                  # API client for Vibes backend
└── voice/
    └── VibesVoiceEngine.ts          # Web Audio + WebRTC management
```

**Key Features:**
- **Web Speech API** fallback for STT (free, built-in)
- **WebRTC** for real-time streaming (best quality)
- **Service Worker** for background notifications
- **LocalStorage** for chat persistence (offline support)

#### 2. Backend (Cloudflare Worker)

**Endpoints:**
```javascript
// POST /vibes/voice
// Audio stream in → Audio stream out

// POST /vibes/chat
// Text in → Text out (with optional audio)

// GET /vibes/history
// Fetch chat history

// POST /vibes/digest
// Trigger daily digest generation
```

**Processing Pipeline:**
```
1. Receive request (audio or text)
2. If audio: STT (Deepgram or Web Speech API)
3. Query Supabase for context (vibes_query RPC)
4. Generate response (DeepSeek/GLM with Vibes prompt)
5. If voice mode: TTS (Kokoro)
6. Return response (audio + text)
7. Log interaction to Supabase
```

#### 3. Supabase Schema Additions

```sql
-- Chat history
CREATE TABLE vibes_conversations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL,  -- Anonymous or authenticated
  session_id TEXT NOT NULL,
  message_type TEXT CHECK (message_type IN ('human', 'vibes')),
  content TEXT NOT NULL,
  audio_url TEXT,  -- Optional: URL to stored audio
  context JSONB,   -- Snapshot of relevant data at time of message
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Digest preferences
CREATE TABLE vibes_preferences (
  user_id TEXT PRIMARY KEY,
  digest_enabled BOOLEAN DEFAULT TRUE,
  digest_time TIME DEFAULT '08:00',
  digest_frequency TEXT DEFAULT 'daily', -- daily, weekly
  voice_enabled BOOLEAN DEFAULT TRUE,
  proactive_alerts BOOLEAN DEFAULT TRUE,
  last_digest_at TIMESTAMPTZ
);

-- Enhanced vibes_query function
CREATE OR REPLACE FUNCTION vibes_query(
  p_user_id TEXT,
  p_question TEXT,
  p_context JSONB DEFAULT '{}'
)
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
  v_user_prefs JSONB;
BEGIN
  -- Get user preferences
  SELECT to_jsonb(vp.*) INTO v_user_prefs
  FROM vibes_preferences vp
  WHERE vp.user_id = p_user_id;

  -- Build comprehensive response context
  v_result := jsonb_build_object(
    -- Project status
    'active_projects', (
      SELECT jsonb_agg(jsonb_build_object(
        'id', p.id,
        'name', p.name,
        'status', p.status,
        'progress', p.progress_pct,
        'tasks_completed', (
          SELECT COUNT(*) FROM tasks t 
          WHERE t.project_id = p.id AND t.status = 'merged'
        ),
        'tasks_pending', (
          SELECT COUNT(*) FROM tasks t 
          WHERE t.project_id = p.id AND t.status = 'pending'
        )
      ))
      FROM projects p
      WHERE p.status = 'active'
    ),
    
    -- ROI summary (last 7 days)
    'roi_summary', (
      SELECT jsonb_build_object(
        'total_tasks', COUNT(*),
        'tokens_in', COALESCE(SUM(tokens_in), 0),
        'tokens_out', COALESCE(SUM(tokens_out), 0),
        'actual_cost_usd', COALESCE(SUM(actual_cost_usd), 0),
        'theoretical_cost_usd', COALESCE(SUM(theoretical_cost_usd), 0),
        'savings_usd', COALESCE(SUM(theoretical_cost_usd - actual_cost_usd), 0)
      )
      FROM task_runs
      WHERE completed_at > NOW() - INTERVAL '7 days'
    ),
    
    -- Platform health
    'platform_health', (
      SELECT jsonb_agg(jsonb_build_object(
        'id', m.id,
        'name', m.name,
        'status', m.status,
        'success_rate', m.success_rate,
        'tokens_used_24h', (
          SELECT COALESCE(SUM(tokens_in + tokens_out), 0)
          FROM task_runs tr
          WHERE tr.model_id = m.id 
          AND tr.completed_at > NOW() - INTERVAL '24 hours'
        )
      ))
      FROM models m
      WHERE m.status IN ('active', 'paused')
    ),
    
    -- Recent activity
    'recent_activity', (
      SELECT jsonb_agg(jsonb_build_object(
        'task_id', t.id,
        'title', t.title,
        'status', t.status,
        'updated_at', t.updated_at
      ))
      FROM tasks t
      ORDER BY t.updated_at DESC
      LIMIT 5
    ),
    
    -- Alerts requiring attention
    'alerts', (
      SELECT jsonb_agg(jsonb_build_object(
        'type', 'escalated_task',
        'task_id', t.id,
        'title', t.title,
        'reason', t.status_reason
      ))
      FROM tasks t
      WHERE t.status = 'escalated'
    ),
    
    -- User preferences
    'user_preferences', v_user_prefs,
    
    -- Query metadata
    'query_meta', jsonb_build_object(
      'timestamp', NOW(),
      'question', p_question
    )
  );
  
  RETURN v_result;
END;
$$ LANGUAGE plpgsql;
```

---

## Service Selection

### Speech-to-Text (STT)

| Option | Quality | Cost | Latency | Best For |
|--------|---------|------|---------|----------|
| **Web Speech API** | Good | Free | Medium | Fallback, simple queries |
| **Deepgram** | Excellent | $0.0043/min | Low | Primary, production |
| **OpenAI Whisper** | Excellent | $0.006/min | Higher | Batch processing |

**Recommendation:** Use Deepgram as primary, Web Speech API as fallback

### Text-to-Speech (TTS)

| Option | Quality | Cost | Speed | Best For |
|--------|---------|------|-------|----------|
| **Kokoro** | Good | ~$0 | Fast | Primary (self-hosted) |
| **ElevenLabs** | Excellent | $0.18/1K chars | Medium | Premium voice |
| **OpenAI TTS** | Good | $0.015/1K chars | Fast | Backup option |

**Recommendation:** Start with Kokoro (cheapest), upgrade to ElevenLabs if voice quality critical

### LLM for Vibes

| Option | Context | Cost | Reasoning | Best For |
|--------|---------|------|-----------|----------|
| **DeepSeek Chat** | 64K | $0.14/1M tok | Good | Primary (already in use) |
| **GLM-5** | 128K | Variable | Excellent | Complex analysis |
| **Kimi K2.5** | 200K | $20/mo sub | Excellent | Long context queries |

**Recommendation:** DeepSeek for cost-efficiency, GLM-5 for complex queries

---

## Implementation Phases

### Phase 1: Text Chat (MVP) - 1-2 days

**Goal:** Working text-based Vibes interface

**Tasks:**
1. ✅ Create `vibes_conversations` table
2. ✅ Create `vibes_query()` RPC function
3. ⏳ Add VibesButton to dashboard (floating action)
4. ⏳ Create VibesChatPanel component
5. ⏳ Deploy simple Cloudflare Worker (text only)
6. ⏳ Connect frontend to backend

**Success Criteria:**
- Can open dashboard on phone
- Tap "Text me" → chat opens
- Type question → get text response
- Response includes real data from Supabase

### Phase 2: Voice Interface - 2-3 days

**Goal:** Voice input/output working

**Tasks:**
1. ⏳ Add Web Speech API integration
2. ⏳ Create VibesVoiceModal component
3. ⏳ Implement audio recording/playback
4. ⏳ Add waveform visualization
5. ⏳ Integrate Deepgram STT
6. ⏳ Integrate Kokoro TTS

**Success Criteria:**
- Tap Vibes button → voice interface opens
- Hold to talk → Vibes responds with voice
- Works smoothly on mobile 4G

### Phase 3: Smart Features - 2-3 days

**Goal:** Proactive notifications, rich responses, memory

**Tasks:**
1. ⏳ Daily digest generation
2. ⏳ Proactive notification system
3. ⏳ Rich response cards (charts, links)
4. ⏳ Conversation memory/context
5. ⏳ Voice activity detection (VAD)

**Success Criteria:**
- Vibes proactively sends daily briefings
- Can ask follow-up questions with context
- Rich responses with visual elements

### Phase 4: Polish - Ongoing

**Goal:** Production-ready experience

**Tasks:**
1. ⏳ Offline support (queue messages)
2. ⏳ Push notifications (PWA)
3. ⏳ Voice customization (select voice)
4. ⏳ Multi-language support
5. ⏳ Usage analytics

---

## Mobile-Specific Considerations

### Performance

**Target Metrics:**
- First paint: < 1s on 4G
- Time to interactive: < 2s
- Voice response latency: < 3s
- Text response latency: < 1s

**Optimizations:**
- Lazy load Vibes components
- Pre-connect to WebRTC/Deepgram
- Audio compression (Opus codec)
- Debounce rapid interactions

### Accessibility

**Requirements:**
- Voice control for visually impaired
- Large touch targets (44px minimum)
- High contrast mode
- Screen reader support

### Battery & Data

**Optimizations:**
- Pause processing when tab not visible
- Compress audio streams
- Cache responses
- Low-data mode option

---

## Cost Estimates

### Monthly Usage Scenario
**100 interactions/day, avg 30 seconds each**

| Component | Quantity | Unit Cost | Monthly |
|-----------|----------|-----------|---------|
| Deepgram STT | 150 min | $0.0043/min | $0.65 |
| DeepSeek LLM | 150K tokens | $0.14/1M | $0.02 |
| Kokoro TTS | Self-hosted | $0 | $0 |
| Cloudflare | 3K requests | Free tier | $0 |
| Supabase | Minimal | Free tier | $0 |
| **Total** | | | **~$0.70/month** |

**At scale (1000 interactions/day): ~$7/month**

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| WebRTC not supported | Low | High | Fallback to HTTP polling |
| STT accuracy poor | Medium | Medium | Allow text correction |
| Mobile browser limits | Medium | Medium | Use native app wrapper (Capacitor) |
| Costs exceed budget | Low | Medium | Rate limiting, usage alerts |
| Latency too high | Medium | High | CDN edge deployment, caching |

---

## Next Steps

1. **Human Review** - Does this specification match your vision?
2. **Prioritize Phase** - Start with Phase 1 (text chat)?
3. **Technical Decisions** - Any preferences on STT/TTS services?
4. **Begin Implementation** - Create feature branch and start building

---

## Appendix: Example Conversations

### Scenario 1: Quick Status Check

**Human:** (taps Vibes button) "How's the social platform project?"

**Vibes:** (voice + text)
> "Social platform is 73% complete. 12 tasks done this week, 3 pending review. One issue: authentication module hit a rate limit. Kimi handled most tasks efficiently at $0.01 each. Want me to show details?"

### Scenario 2: ROI Analysis

**Human:** (text) "What's our ROI this month?"

**Vibes:** (rich card response)
```
📊 February ROI Summary

💰 Savings: $127.40
   (vs paying API rates)

🎯 Efficiency: 94% success rate
   156 tasks completed

🏆 Top Performers:
   1. Kimi CLI - $0.01/task
   2. DeepSeek API - $0.002/task
   
⚠️  Alert: Kimi subscription expires 
    March 15. Recommend renewal.
```

### Scenario 3: Proactive Briefing

**Vibes:** (notification banner)
> 🌅 Good morning! Daily briefing: 8 tasks completed yesterday. 2 require your review. DeepSeek credit at $1.20. Kimi subscription renews in 10 days. Tap for details.

---

**Document Version:** 1.0  
**Last Updated:** 2026-02-20  
**Next Review:** After human feedback
