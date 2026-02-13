# VibePilot Voice Interface — Architecture

## Overview

"Talk to Vibes" - Voice-first interaction with the VibePilot dashboard.

**User Experience:**
1. Click mic icon on dashboard
2. Ask any question
3. Vibes responds with current data from Supabase

**Example Queries:**
- "How are we doing on the social platform project?"
- "What are our 3 most successful models this week?"
- "How has our ROI improved this month?"
- "What tasks are currently escalated?"
- "Add a dashboard above the project dashboard"

---

## Architecture (Cost-Optimized for Free Tier)

```
┌─────────────────────────────────────────────────────────────────┐
│                        USER BROWSER                             │
│  ┌─────────────┐                                                │
│  │ Mic Button  │──▶ Record Audio (WebAudio API)                │
│  │ (Vibes)     │◀─── Play Response (WebAudio)                  │
│  └─────────────┘                                                │
└────────┬────────────────────────────────────────────────────────┘
         │
         │ Audio Blob (WebM/WAV)
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                  CLOUDFLARE WORKERS                             │
│  (Free tier: 100k requests/day)                                │
│                                                                 │
│  1. Receive audio blob                                          │
│  2. Forward to Deepgram (STT)                                   │
│  3. Receive transcript                                           │
│  4. Forward to LLM (DeepSeek/GLM)                               │
│  5. Query Supabase for real data                                │
│  6. Generate response                                            │
│  7. Forward text to Kokoro (TTS)                                 │
│  8. Return audio to browser                                      │
└─────────────────────────────────────────────────────────────────┘
         │
         │ API Calls
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    EXTERNAL SERVICES                            │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │  Deepgram   │  │  DeepSeek   │  │   Kokoro    │            │
│  │  (STT)      │  │  (LLM)      │  │   (TTS)     │            │
│  │             │  │             │  │             │            │
│  │ $0.0043/min │  │ $0.14/1M    │  │ Local/Cloud │            │
│  │ 200min free │  │  tokens     │  │ (cheapest)  │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│                                                                 │
│  ┌─────────────────────────────────────────────────┐           │
│  │              SUPABASE (Data Source)              │           │
│  │                                                  │           │
│  │  • tasks (current status)                        │           │
│  │  • task_runs (ROI data)                          │           │
│  │  • models (performance)                          │           │
│  │  • platforms (success rates)                     │           │
│  │  • projects (cumulative metrics)                 │           │
│  └─────────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────────┘
```

---

## Service Selection Rationale

| Service | Choice | Why | Cost |
|---------|--------|-----|------|
| **STT** | Deepgram | Best quality/price, 200min free | $0.0043/min after |
| **LLM** | DeepSeek/GLM | Already in use | $0.14/1M tokens |
| **TTS** | Kokoro | Cheapest, runs on CPU | ~$0 |
| **Edge** | Cloudflare Workers | 100k req/day free | Free tier |
| **State** | Supabase | Already in use | Free tier |

**Monthly Cost Estimate (Light Use):**
- 100 voice queries/day × 30 days = 3000 queries
- Avg 10 sec/query = 500 min audio → $2.15 (Deepgram)
- Avg 500 tokens/query = 1.5M tokens → $0.21 (DeepSeek)
- TTS: ~$0 (Kokoro efficient)
- **Total: ~$2.50/month**

---

## Cloudflare Worker Implementation

```javascript
// worker.js
export default {
  async fetch(request, env) {
    const formData = await request.formData();
    const audio = formData.get('audio');
    const query = formData.get('query'); // or transcribe from audio
    
    // 1. Transcribe (if audio)
    let transcript = query;
    if (audio && !query) {
      const deepgramRes = await fetch('https://api.deepgram.com/v1/listen', {
        method: 'POST',
        headers: { 'Authorization': `Token ${env.DEEPGRAM_KEY}` },
        body: audio
      });
      const dg = await deepgramRes.json();
      transcript = dg.results?.channels[0]?.alternatives[0]?.transcript;
    }
    
    // 2. Query Supabase for context
    const supabaseRes = await fetch(`${env.SUPABASE_URL}/rest/v1/rpc/vibes_query`, {
      headers: {
        'apikey': env.SUPABASE_KEY,
        'Content-Type': 'application/json'
      },
      method: 'POST',
      body: JSON.stringify({ question: transcript })
    });
    const context = await supabaseRes.json();
    
    // 3. Generate response with LLM
    const llmRes = await fetch('https://api.deepseek.com/v1/chat/completions', {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${env.DEEPSEEK_KEY}` },
      body: JSON.stringify({
        model: 'deepseek-chat',
        messages: [
          { role: 'system', content: VIBES_PROMPT },
          { role: 'user', content: `${transcript}\n\nCurrent Data:\n${JSON.stringify(context)}` }
        ]
      })
    });
    const llm = await llmRes.json();
    const response = llm.choices[0].message.content;
    
    // 4. Text-to-Speech (Kokoro)
    const ttsRes = await fetch('http://localhost:8888/tts', {
      method: 'POST',
      body: JSON.stringify({ text: response })
    });
    const audioResponse = await ttsRes.arrayBuffer();
    
    return new Response(audioResponse, {
      headers: { 'Content-Type': 'audio/wav' }
    });
  }
}

const VIBES_PROMPT = `You are Vibes, the voice interface for VibePilot - an AI execution engine.

You have access to real-time data about:
- Active tasks and their status
- Model and platform performance
- ROI metrics per project
- Efficiency improvements over time

Be concise, helpful, and data-driven. Respond naturally as if in conversation.
If asked about visual/UI changes, note that those require human approval.`;
```

---

## Supabase RPC for Vibes Queries

```sql
-- Add to schema
CREATE OR REPLACE FUNCTION vibes_query(p_question TEXT)
RETURNS JSONB AS $$
DECLARE
  v_result JSONB;
BEGIN
  -- Detect query type and return relevant data
  v_result := jsonb_build_object(
    'active_tasks', (SELECT jsonb_agg(*) FROM tasks WHERE status NOT IN ('merged', 'pending')),
    'escalated', (SELECT jsonb_agg(*) FROM tasks WHERE status = 'escalated'),
    'top_models', (SELECT jsonb_agg(jsonb_build_object(
      'model', id, 
      'success_rate', success_rate,
      'tasks', total_tasks
    )) FROM models WHERE status = 'active' ORDER BY success_rate DESC LIMIT 3),
    'top_platforms', (SELECT jsonb_agg(jsonb_build_object(
      'platform', id,
      'success_rate', success_rate,
      'tasks', total_tasks
    )) FROM platforms WHERE status = 'active' ORDER BY success_rate DESC LIMIT 3),
    'roi_summary', (SELECT jsonb_build_object(
      'total_tasks', COUNT(*),
      'total_tokens', COALESCE(SUM(tokens_used), 0),
      'success_rate', AVG(CASE WHEN status = 'success' THEN 1 ELSE 0 END),
      'total_savings', COALESCE(SUM(theoretical_api_cost - actual_cost), 0)
    ) FROM task_runs WHERE completed_at > NOW() - INTERVAL '7 days')
  );
  
  RETURN v_result;
END;
$$ LANGUAGE plpgsql;
```

---

## Dashboard Integration

```tsx
// VibesButton.tsx
import { useState, useRef } from 'react';

export function VibesButton() {
  const [isRecording, setIsRecording] = useState(false);
  const [isProcessing, setIsProcessing] = useState(false);
  const mediaRecorder = useRef<MediaRecorder | null>(null);
  const chunks = useRef<Blob[]>([]);

  const startRecording = async () => {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    mediaRecorder.current = new MediaRecorder(stream);
    chunks.current = [];
    
    mediaRecorder.current.ondataavailable = (e) => chunks.current.push(e.data);
    mediaRecorder.current.onstop = handleSubmit;
    mediaRecorder.current.start();
    setIsRecording(true);
  };

  const stopRecording = () => {
    mediaRecorder.current?.stop();
    setIsRecording(false);
  };

  const handleSubmit = async () => {
    setIsProcessing(true);
    const blob = new Blob(chunks.current, { type: 'audio/webm' });
    const formData = new FormData();
    formData.append('audio', blob);

    const res = await fetch('/api/vibes', { method: 'POST', body: formData });
    const audio = await res.blob();
    
    // Play response
    const url = URL.createObjectURL(audio);
    new Audio(url).play();
    setIsProcessing(false);
  };

  return (
    <button 
      onClick={isRecording ? stopRecording : startRecording}
      disabled={isProcessing}
      className="vibes-button"
    >
      {isProcessing ? '⏳' : isRecording ? '🔴' : '🎤'}
    </button>
  );
}
```

---

## Daily Digest Feature

```sql
-- Store digest preferences
CREATE TABLE digest_settings (
  user_email TEXT PRIMARY KEY,
  enabled BOOLEAN DEFAULT TRUE,
  frequency TEXT DEFAULT 'daily', -- daily, weekly
  include_escalated BOOLEAN DEFAULT TRUE,
  include_roi_summary BOOLEAN DEFAULT TRUE,
  include_top_performers BOOLEAN DEFAULT TRUE,
  last_sent TIMESTAMPTZ
);

-- Generate digest
CREATE OR REPLACE FUNCTION generate_digest(p_email TEXT)
RETURNS JSONB AS $$
DECLARE
  v_digest JSONB;
BEGIN
  SELECT jsonb_build_object(
    'date', NOW()::DATE,
    'tasks_completed', (SELECT COUNT(*) FROM tasks WHERE status = 'merged' AND completed_at > NOW() - INTERVAL '24 hours'),
    'tasks_escalated', (SELECT jsonb_agg(*) FROM tasks WHERE status = 'escalated'),
    'tokens_used', (SELECT COALESCE(SUM(tokens_used), 0) FROM task_runs WHERE completed_at > NOW() - INTERVAL '24 hours'),
    'roi_today', (SELECT 
      COALESCE(SUM(theoretical_api_cost), 0) - COALESCE(SUM(actual_cost), 0)
      FROM task_runs WHERE completed_at > NOW() - INTERVAL '24 hours'
    ),
    'top_model', (SELECT id FROM models ORDER BY success_rate DESC LIMIT 1),
    'top_platform', (SELECT id FROM platforms ORDER BY success_rate DESC LIMIT 1)
  ) INTO v_digest;
  
  RETURN v_digest;
END;
$$ LANGUAGE plpgsql;
```

---

## Implementation Order

1. ✅ Add `vibes_query()` RPC to Supabase
2. ⏳ Deploy Cloudflare Worker
3. ⏳ Add VibesButton to dashboard
4. ⏳ Configure Deepgram account
5. ⏳ Set up Kokoro TTS locally/cloud
6. ⏳ Add digest settings table
7. ⏳ Create email worker for daily digest

---

## Cost Summary

| Component | Free Tier | Paid After |
|-----------|-----------|------------|
| Cloudflare Workers | 100k req/day | $5/mo |
| Deepgram STT | 200 min | $0.0043/min |
| DeepSeek LLM | Usage-based | $0.14/1M tok |
| Kokoro TTS | Unlimited | ~$0 |
| Supabase | 500MB | $25/mo |

**Estimated for 100 queries/day: ~$2.50/month**
